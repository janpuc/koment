package metrics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return recorder.Body.String()
}

func TestEveryStatusIsReportedIncludingZero(t *testing.T) {
	m := New()
	m.ObserveRepository(map[anchor.Status]int{anchor.StatusOK: 3}, 2, time.Millisecond)

	body := scrape(t, m)
	for _, status := range []string{"ok", "moved", "drifted", "orphaned"} {
		want := `koment_annotations{status="` + status + `"}`
		if !strings.Contains(body, want) {
			t.Errorf("missing %s — a status that drops out looks like a scrape gap, not a return to zero", want)
		}
	}
	if !strings.Contains(body, `koment_annotations{status="ok"} 3`) {
		t.Error("ok count did not make it through")
	}
	if !strings.Contains(body, `koment_annotations{status="drifted"} 0`) {
		t.Error("drifted must be reported as zero rather than omitted")
	}
	if !strings.Contains(body, "koment_files_annotated 2") {
		t.Error("file count did not make it through")
	}
}

func TestDriftFallingBackToZeroIsVisible(t *testing.T) {
	m := New()
	m.ObserveRepository(map[anchor.Status]int{anchor.StatusDrifted: 2}, 1, time.Millisecond)
	if !strings.Contains(scrape(t, m), `koment_annotations{status="drifted"} 2`) {
		t.Fatal("drift was not recorded")
	}

	m.ObserveRepository(map[anchor.Status]int{anchor.StatusOK: 3}, 1, time.Millisecond)
	if !strings.Contains(scrape(t, m), `koment_annotations{status="drifted"} 0`) {
		t.Error("a fixed drift must read as zero, not as the previous value")
	}
}

func TestServedAnnotationsAreCountedByStatus(t *testing.T) {
	m := New()
	m.ObserveServed(anchor.StatusOK)
	m.ObserveServed(anchor.StatusDrifted)
	m.ObserveServed(anchor.StatusDrifted)

	body := scrape(t, m)
	if !strings.Contains(body, `koment_mcp_annotations_served_total{status="drifted"} 2`) {
		t.Error("the metric that matters most — agents handed stale annotations — was not recorded")
	}
}

func TestToolAndHTTPCallsAreRecorded(t *testing.T) {
	m := New()
	m.ObserveMCPCall("koment_get", "ok", 5*time.Millisecond)
	m.ObserveHTTP("ui", http.StatusNotFound, time.Millisecond)

	body := scrape(t, m)
	for _, want := range []string{
		`koment_mcp_calls_total{outcome="ok",tool="koment_get"} 1`,
		`koment_http_requests_total{code="404",handler="ui"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestDiscardRecordsNothingAndDoesNotPanic(t *testing.T) {
	var recorder Recorder = Discard{}
	recorder.ObserveRepository(map[anchor.Status]int{anchor.StatusOK: 1}, 1, time.Second)
	recorder.ObserveHTTP("ui", 200, time.Second)
	recorder.ObserveMCPCall("koment_get", "ok", time.Second)
	recorder.ObserveServed(anchor.StatusDrifted)
}

func TestInstrumentCapturesTheStatusCode(t *testing.T) {
	m := New()
	handler := Instrument(m, "test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(scrape(t, m), `koment_http_requests_total{code="418",handler="test"} 1`) {
		t.Error("the status code was not captured")
	}
}

func TestInstrumentDefaultsToOKWhenNothingWroteAHeader(t *testing.T) {
	m := New()
	handler := Instrument(m, "test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(scrape(t, m), `koment_http_requests_total{code="200",handler="test"} 1`) {
		t.Error("an implicit 200 should be recorded as 200")
	}
}

func TestSweepCountsTheRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	annotations := store.Open(root)
	excerpt := "func main() {}"
	if err := annotations.Save(&store.Record{
		Version: store.RecordVersion,
		File:    "main.go",
		Annotations: []store.Annotation{{
			ID:            "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:         store.ScopeExcerpt,
			Excerpt:       excerpt,
			ExcerptSHA256: store.ExcerptSHA256(excerpt),
			LastSeenLine:  3,
			Kind:          store.KindWhy,
			Body:          "Entry point only.",
			Created:       store.Date{Time: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	m := New()
	if err := Sweep(annotations, m); err != nil {
		t.Fatal(err)
	}

	body := scrape(t, m)
	if !strings.Contains(body, `koment_annotations{status="ok"} 1`) {
		t.Errorf("sweep did not count the annotation:\n%s", body)
	}
	if !strings.Contains(body, "koment_files_annotated 1") {
		t.Error("sweep did not count the file")
	}
}
