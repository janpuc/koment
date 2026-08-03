// Package metrics reports how a long-lived koment is doing: how much drift the
// repository carries, whether anyone is reading, and how often an agent is
// handed an annotation that no longer applies.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/store"
)

const (
	namespace     = "koment"
	shutdownGrace = 5 * time.Second
	headerTimeout = 10 * time.Second
)

// Recorder is what the servers depend on, so that neither imports Prometheus
// and a build without metrics costs nothing (ADR 0020).
type Recorder interface {
	ObserveRepository(resolved map[anchor.Status]int, files int, took time.Duration)
	ObserveHTTP(handler string, code int, took time.Duration)
	ObserveMCPCall(tool, outcome string, took time.Duration)
	ObserveServed(status anchor.Status)
}

// Discard satisfies Recorder without measuring anything, which is what every
// server uses unless --metrics was given.
type Discard struct{}

func (Discard) ObserveRepository(map[anchor.Status]int, int, time.Duration) {}
func (Discard) ObserveHTTP(string, int, time.Duration)                      {}
func (Discard) ObserveMCPCall(string, string, time.Duration)                {}
func (Discard) ObserveServed(anchor.Status)                                 {}

type Metrics struct {
	registry *prometheus.Registry

	annotations     *prometheus.GaugeVec
	files           prometheus.Gauge
	resolveDuration prometheus.Histogram

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	mcpCalls    *prometheus.CounterVec
	mcpDuration *prometheus.HistogramVec
	mcpServed   *prometheus.CounterVec
}

func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),

		annotations: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "annotations",
			Help:      "Annotations in the served repository, by resolution status.",
		}, []string{"status"}),

		files: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "files_annotated",
			Help:      "Source files carrying at least one annotation.",
		}),

		resolveDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "resolve_duration_seconds",
			Help:      "Time to resolve every annotation in the repository.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 4, 8),
		}),

		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "HTTP requests served.",
		}, []string{"handler", "code"}),

		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "Time to serve an HTTP request.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"handler"}),

		mcpCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mcp_calls_total",
			Help:      "MCP tool calls, by tool and outcome.",
		}, []string{"tool", "outcome"}),

		mcpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "mcp_call_duration_seconds",
			Help:      "Time to answer an MCP tool call.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"tool"}),

		// The metric this project should watch: a rising drifted rate means
		// agents are being handed history as though it described the code.
		mcpServed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mcp_annotations_served_total",
			Help:      "Annotations handed to an agent, by resolution status.",
		}, []string{"status"}),
	}

	m.registry.MustRegister(
		m.annotations, m.files, m.resolveDuration,
		m.httpRequests, m.httpDuration,
		m.mcpCalls, m.mcpDuration, m.mcpServed,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// ObserveRepository publishes a full sweep. Every status is set on every sweep,
// including the zero ones: leaving a status unset would make a drift that was
// fixed look like a scrape gap rather than a return to zero.
func (m *Metrics) ObserveRepository(resolved map[anchor.Status]int, files int, took time.Duration) {
	for _, status := range []anchor.Status{
		anchor.StatusOK, anchor.StatusMoved, anchor.StatusAmbiguous, anchor.StatusDrifted, anchor.StatusOrphaned,
	} {
		m.annotations.WithLabelValues(string(status)).Set(float64(resolved[status]))
	}
	m.files.Set(float64(files))
	m.resolveDuration.Observe(took.Seconds())
}

func (m *Metrics) ObserveHTTP(handler string, code int, took time.Duration) {
	m.httpRequests.WithLabelValues(handler, fmt.Sprint(code)).Inc()
	m.httpDuration.WithLabelValues(handler).Observe(took.Seconds())
}

func (m *Metrics) ObserveMCPCall(tool, outcome string, took time.Duration) {
	m.mcpCalls.WithLabelValues(tool, outcome).Inc()
	m.mcpDuration.WithLabelValues(tool).Observe(took.Seconds())
}

func (m *Metrics) ObserveServed(status anchor.Status) {
	m.mcpServed.WithLabelValues(string(status)).Inc()
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Serve runs the metrics endpoint on its own listener. It is never mounted on
// the serving listener, which is unauthenticated and ingress-facing (ADR 0020).
func (m *Metrics) Serve(ctx context.Context, address string, stderr io.Writer) error {
	resolved, err := listen.Address(address)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.Handler())
	server := &http.Server{Handler: mux, ReadHeaderTimeout: headerTimeout}

	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("listening for metrics on %s: %w", resolved, err)
	}
	fmt.Fprintf(stderr, "koment: metrics on http://%s/metrics\n", listener.Addr())

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(timeout); err != nil {
			fmt.Fprintf(stderr, "koment: shutting down metrics: %v\n", err)
		}
	}()

	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Sweep resolves the whole repository so the gauges describe it. Called on a
// schedule rather than per request, because a sweep is O(annotations) and a
// scrape must not be able to drive load.
func Sweep(annotations *store.Store, recorder Recorder) error {
	started := time.Now()
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return err
	}

	counts := map[anchor.Status]int{}
	for _, file := range files {
		resolutions, err := anchor.ResolveStored(annotations, file)
		if err != nil {
			return err
		}
		for _, resolution := range resolutions {
			counts[resolution.Status]++
		}
	}

	recorder.ObserveRepository(counts, len(files), time.Since(started))
	return nil
}
