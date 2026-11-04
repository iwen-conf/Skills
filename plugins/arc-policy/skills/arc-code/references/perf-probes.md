# Stage Timing Probes

Probe mode adds low-cost stage timers to a slow request path, by default a Go/Gin server-rendered
(SSR) page. Stage timing answers "which stage is slow", not "which function, allocation,
goroutine or lock is slow".

## Rules

- Stage timing is not a profiler. CPU hotspots, GC, heap, allocation → `pprof`; goroutine
  scheduling, blocking, lock contention → `go tool trace`. Frontend performance is out of scope.
- No timing-based optimization without a reproduced baseline.
- No code patch from a slow-signal guess: confirm the slow stage and its root cause first; until
  then the work stays observational ([repair.md](repair.md) §6).
- No hand-formatted JSON and no default `log.Printf` prefix: use the project's structured logger
  or the slog JSON helper below.
- No metrics stack, tracing platform or pprof setup for a focused probe unless the user asks.
- No secrets, tokens, raw authorization headers, full payloads or sensitive personal data in
  timing logs.

## Workflow

1. Capture the baseline: the request (command or URL), number of runs, and per-request or
   p50/p95 latency. Save it to `.arc/artifacts/<task-slug>/logs/perf-baseline.txt`.
2. Inspect the router, Gin handlers, service calls, template renderer, logger convention, request
   ID middleware and existing tests.
3. Split the path into stages: handler entry (`TOTAL`), data fetch, view-model build, template
   render, response write.
4. Put the helper next to the owning package unless the repository already has an observability
   helper package.
5. Emit through the project's structured logger when one exists (for example zap), with the same
   four keys: `event`, `request_id`, `stage`, `cost_ms`. Otherwise use the helper below.
6. Use the request ID when available. IF none exists THEN emit an empty `request_id` and report the
   correlation limit; adding request-ID middleware is a separate scope decision.
7. Run focused tests or local requests, compare with the baseline, then change one thing at a time
   and re-measure after each change.
8. Report where the logs appear, how to read them, and that this is coarse timing; recommend
   `pprof` or `go tool trace` only for function-level or runtime questions.

## Minimal Trace Helper

For Go 1.21+ without an established structured logger. Create the logger once at composition with
`slog.New(slog.NewJSONHandler(os.Stderr, nil))`; older Go projects use their established JSON
logger. Do not change global log flags.

```go
package tracing

import (
    "log/slog"
    "time"
)

type Trace struct {
    logger *slog.Logger
    reqID string
    stage string
    start time.Time
}

func NewTrace(logger *slog.Logger, reqID, stage string) *Trace {
    return &Trace{logger: logger, reqID: reqID, stage: stage, start: time.Now()}
}

func (t *Trace) Done() {
    t.logger.Info("ssr_trace", "event", "ssr_trace", "request_id", t.reqID,
        "stage", t.stage, "cost_ms", time.Since(t.start).Milliseconds())
}
```

The JSON handler escapes quotes and newlines in `stage` and `request_id`, so every emitted line is
one complete JSON object.

## SSR Stage Template

Adapt names to the repository:

```go
func GetArticle(c *gin.Context) {
	reqID := c.GetString("req_id") // empty when no request ID is available
	trace := NewTrace(logger, reqID, "TOTAL")
	defer trace.Done()

	t1 := NewTrace(logger, reqID, "FETCH")
	data, err := service.GetData(c.Request.Context())
	t1.Done()
	if err != nil {
		// Return using the repo's existing error response pattern.
		return
	}

	t2 := NewTrace(logger, reqID, "BUILD_VIEW")
	vm := buildViewModel(data)
	t2.Done()

	t3 := NewTrace(logger, reqID, "RENDER_TEMPLATE")
	html, err := renderTemplate(vm)
	t3.Done()
	if err != nil {
		// Return using the repo's existing error response pattern.
		return
	}

	t4 := NewTrace(logger, reqID, "WRITE_RESPONSE")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	t4.Done()
}
```

Use `defer` only for the total timer or for stages with several returns. For short linear stages
call `Done()` right after the operation so the measured range is obvious.

## Output Rules

One log line per stage:

```json
{"event":"ssr_trace","request_id":"abc","stage":"FETCH","cost_ms":12}
{"event":"ssr_trace","request_id":"abc","stage":"BUILD_VIEW","cost_ms":3}
{"event":"ssr_trace","request_id":"abc","stage":"RENDER_TEMPLATE","cost_ms":45}
{"event":"ssr_trace","request_id":"abc","stage":"TOTAL","cost_ms":67}
```

- Stage names are stable: `TOTAL`, `FETCH`, `DB_QUERY`, `RPC_CALL`, `BUILD_VIEW`,
  `RENDER_TEMPLATE`, `WRITE_RESPONSE`.
- Add route, handler, tenant or user fields only when they are safe and needed for the question.

## Escalation

When stage timing has isolated a stage but not the cause inside it, recommend the next tool in
the report; set it up only on user request:

- CPU or allocation inside a stage: `go test -bench . -cpuprofile cpu.out -memprofile mem.out`,
  then `go tool pprof`.
- Live service: `net/http/pprof` on an internal, access-controlled port only.
- Scheduling, blocking or lock waits: `go test -trace trace.out`, then `go tool trace`.

## Review Checklist

- [ ] Baseline recorded before any change; after-numbers use the same request and run count.
- [ ] Logs go through the project logger, or the injected slog JSON logger; every line is valid
      JSON with `event`, `request_id`, `stage`, `cost_ms`.
- [ ] Missing request IDs are reported as a limitation, never invented.
- [ ] The path is split into at least fetch, build view, render template and total.
- [ ] Error paths still emit the total timing and never write the response twice.
- [ ] Probes are removed, or kept behind a log level when the task asks to keep them.
- [ ] The report states this is coarse timing and names `pprof` / `go tool trace` only for deeper
      diagnosis.
