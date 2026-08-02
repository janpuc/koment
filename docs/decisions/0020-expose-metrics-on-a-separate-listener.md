# 0020 — Expose metrics, on a separate listener

Date: 2026-08-02
Status: Accepted

## Context

koment now runs as a long-lived process in a cluster. Nothing about it is
observable: whether anyone is reading, whether agents are being handed stale
annotations, whether the drift gate is quietly getting worse over time.

The last of those is the interesting one. `koment check` answers "is anything
drifted right now" as an exit code, in CI, at one moment. It cannot answer "has
drift been climbing for three weeks", which is the question that tells you a
codebase is outgrowing its annotations. That is a time series, and a time series
needs a metrics endpoint.

Prometheus is the only realistic target given where this runs, which makes
`github.com/prometheus/client_golang` the dependency. It is large — it pulls
`prometheus/common`, `prometheus/procfs`, `golang/protobuf` and more — and
AGENTS.md §10 sets a high bar.

It clears the bar for a reason worth stating: the alternative is not "a smaller
metrics library", it is hand-writing an exposition-format encoder and a
concurrency-safe registry. That is a well-known way to produce subtly wrong
counters, and metrics that are subtly wrong are worse than none — the same
argument this whole project makes about annotations.

## Decision

Instrument koment with `prometheus/client_golang`, and serve metrics on a
**listener of their own**:

```
koment ui  --listen 0.0.0.0:8080 --metrics 0.0.0.0:9090
koment mcp --http   0.0.0.0:8080 --metrics 0.0.0.0:9090
```

Metrics are off unless `--metrics` is given. The endpoint is never mounted on
the serving listener.

That separation is the substantive part of this decision. koment's serving
surface is unauthenticated by design (ADR 0011), and is expected to sit behind
an ingress. Metrics must not inherit that exposure: `/metrics` leaks repository
paths through label values and is exactly the sort of thing that ends up
public when it shares a port with the UI. A separate address means the ingress
publishes 8080 and never 9090, which is also the convention every Kubernetes
scrape configuration already assumes.

What is measured:

| metric | type | answers |
|---|---|---|
| `koment_annotations` by `status` | gauge | is drift growing? |
| `koment_files_annotated` | gauge | is the store growing? |
| `koment_resolve_duration_seconds` | histogram | is resolution getting slow? |
| `koment_http_requests_total` by `handler`, `code` | counter | is anyone reading? |
| `koment_http_request_duration_seconds` by `handler` | histogram | is it slow? |
| `koment_mcp_calls_total` by `tool`, `outcome` | counter | are agents using it? |
| `koment_mcp_call_duration_seconds` by `tool` | histogram | are they waiting? |
| `koment_mcp_annotations_served_total` by `status` | counter | **how often is an agent handed a stale annotation?** |

The last one is the metric this project should care about most. A high
`drifted` rate there means agents are routinely reading history as though it
were current, which is the failure koment exists to prevent, happening in
production rather than in CI.

Labels never carry a file path, an annotation id or a body. Cardinality stays
bounded by the four statuses, two tools and a handful of handlers.

## Consequences

- Drift becomes a trend rather than a boolean, which is the thing CI cannot
  give you.
- A large dependency enters the graph — the first that is neither the YAML
  parser nor the MCP SDK. **Only `internal/metrics` touches a Prometheus API.**
  The servers import that package for its `Recorder` interface and take a
  no-op `Discard` when metrics are off, so they never name a Prometheus type
  and removing the dependency is deleting one package plus a handful of
  interface calls. That is weaker than the confinement ADR 0007 bought for the
  MCP SDK, and it is stated plainly rather than dressed up.
- **Metrics are opt-in and separately addressed**, so the default posture is
  unchanged and no existing deployment starts exposing anything.
- Two listeners means two ports to configure, and someone will publish the wrong
  one. The chart makes 9090 a distinct named port that the ingress template
  cannot target.
- The gauges describe the repository being served, so they are only meaningful
  for a long-lived process. `koment check` in CI stays an exit code and gets no
  instrumentation.

## Alternatives rejected

- **`/metrics` on the serving listener.** One port, no new flag, and every
  tutorial does it. Rejected because that listener is unauthenticated and
  ingress-facing by design; the first person to publish koment would publish
  their repository's shape with it.
- **OpenTelemetry instead.** More general, vendor-neutral, and it would give
  traces as well. Rejected as a much heavier dependency and a collector to run,
  for a tool whose entire operational question is "how much drift, and is anyone
  reading" — which is four gauges and three counters.
- **`expvar` from the standard library.** Zero dependencies, and it was the
  tempting answer given §10. Rejected because nothing scrapes it: it would need
  a translation layer to be useful, which is the encoder this ADR declined to
  hand-write, plus a JSON format nobody's dashboards read.
- **Logs instead of metrics, aggregated downstream.** No new dependency and no
  new port. Rejected because deriving "drifted annotations over time" from log
  lines needs a log pipeline that is a much bigger dependency than a client
  library, and it is the wrong shape for a gauge.
- **Per-annotation or per-file labels.** Far more useful for finding *which*
  annotation is drifting. Rejected on cardinality: a repository with thousands
  of annotations would produce thousands of series, and `koment check` already
  answers "which one" exactly and for free.
