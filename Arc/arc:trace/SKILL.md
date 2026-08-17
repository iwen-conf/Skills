---
name: arc:trace
version: 1.0.0
description: >
  Adds low-cost fmt/time or log.Printf timing probes to Go Gin SSR request paths. Use when
  the user says 加日志, 计时, 慢请求, Gin SSR, tracing, 探针, or request timing. Not for pprof,
  heap, or generic observability platforms.
---
# Go Gin SSR fmt Tracing

## Overview

Use this skill to add coarse performance timing to Go/Gin SSR code paths with minimal dependencies. Treat `fmt + time` as a low-cost instrumentation method, not as a profiler: it helps answer "which stage is slow", not "which function, allocation, goroutine, or lock is slow".

## Intent Router

| When | Load |
|---|---|
| Add Gin SSR stage timing | this SKILL.md Workflow and helper |
| CPU/heap/goroutine detail | `pprof` / `go tool trace`, not this skill |
| Frontend performance | `arc:frontend` / `web-perf` |
| Failure diagnosis beyond timing | `arc:fix` |

## Red Lines

```text
NO PPROF COSPLAY — THIS IS STAGE TIMING, NOT A PROFILER.
NO fmt.Println FOR TIMING — USE log.Printf / stderr / PROJECT LOGGER.
NO BROAD OBSERVABILITY STACK FOR A FOCUSED SSR PROBE.
NO SECRET OR FULL PAYLOAD IN TIMING LOGS.
```

## When to Use

Use this skill when the user asks to:

1. Add quick timing logs to a Go/Gin server-rendered page or template path.
2. Diagnose slow SSR requests when full profiling is not yet available or would be too invasive.
3. Split request latency into handler, data fetch, view-model build, template render, and response write stages.
4. Produce temporary, AI-readable timing logs for a focused performance investigation.

Skip this skill when the user needs CPU, heap, allocation, goroutine scheduling, blocking, or lock analysis. Use `pprof` or `go tool trace` for those deeper runtime questions.

## Positioning

Use stage timing for:

- Quickly locating the slow segment in a request.
- Splitting SSR latency across handler, data fetch, view-model build, template render, and response write.
- Temporary production debugging when pprof is unavailable.
- Producing structured logs that another agent or script can analyze.

Do not present this as sufficient for:

- CPU hotspot analysis.
- GC, heap, or allocation diagnosis.
- Goroutine scheduling, blocking, or lock contention.
- Precise function-level profiling.

Escalate to `pprof` for CPU/heap detail and to `go tool trace` for goroutine scheduling and runtime behavior.

## Workflow

1. Inspect the existing Go project before editing: router, Gin handlers, service calls, template renderer, logger convention, request ID middleware, and tests.
2. Identify the SSR path and split it into these stages: handler entry, data fetch, view-model build, template render, and response write.
3. Add a tiny trace helper close to the owning package unless the repo already has an observability helper package.
4. Prefer structured output with stable keys. Use the project's logger if one exists; otherwise use `log.Printf` or `fmt.Fprintf(os.Stderr, ...)`, not plain `fmt.Println`.
5. Include a request ID when available. If the project has no request ID, add Gin middleware only when that is inside the task scope.
6. Keep probes temporary or clearly scoped. Avoid broad refactors, new metrics stacks, or pprof setup unless the user asks.
7. Run focused tests or a local request when possible, then explain where the logs appear and how to interpret them.

## Minimal Trace Helper

Use a helper like this when the project has no existing trace/log helper:

```go
type Trace struct {
	name  string
	start time.Time
}

func NewTrace(name string) *Trace {
	return &Trace{
		name:  name,
		start: time.Now(),
	}
}

func (t *Trace) Done() {
	log.Printf("[TRACE] stage=%s cost_ms=%d", t.name, time.Since(t.start).Milliseconds())
}
```

For AI-readable logs, prefer JSON lines:

```go
func (t *Trace) Done() {
	log.Printf(`{"event":"ssr_trace","stage":"%s","cost_ms":%d}`,
		t.name,
		time.Since(t.start).Milliseconds(),
	)
}
```

If stage names can contain quotes or user-controlled values, use `encoding/json` or a structured logger instead of formatting JSON by hand.

## Request ID

When Gin context already contains a request ID, thread it into each trace:

```go
func NewTraceWithID(reqID, stage string) *Trace {
	return NewTrace(fmt.Sprintf("[%s] %s", reqID, stage))
}
```

For cleaner structured output, keep request ID as a separate key:

```go
type Trace struct {
	reqID string
	stage string
	start time.Time
}

func NewTrace(reqID, stage string) *Trace {
	return &Trace{reqID: reqID, stage: stage, start: time.Now()}
}

func (t *Trace) Done() {
	log.Printf(`{"event":"ssr_trace","request_id":"%s","stage":"%s","cost_ms":%d}`,
		t.reqID,
		t.stage,
		time.Since(t.start).Milliseconds(),
	)
}
```

If request IDs are missing and middleware is in scope:

```go
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("req_id", uuid.NewString())
		c.Next()
	}
}
```

## SSR Stage Template

Instrument the standard SSR path like this, adapting names to the repo:

```go
func GetArticle(c *gin.Context) {
	reqID, _ := c.Get("req_id")
	trace := NewTrace(fmt.Sprint(reqID), "TOTAL")
	defer trace.Done()

	t1 := NewTrace(fmt.Sprint(reqID), "FETCH")
	data, err := service.GetData(c.Request.Context())
	t1.Done()
	if err != nil {
		// Return using the repo's existing error response pattern.
		return
	}

	t2 := NewTrace(fmt.Sprint(reqID), "BUILD_VIEW")
	vm := buildViewModel(data)
	t2.Done()

	t3 := NewTrace(fmt.Sprint(reqID), "RENDER_TEMPLATE")
	html, err := renderTemplate(vm)
	t3.Done()
	if err != nil {
		// Return using the repo's existing error response pattern.
		return
	}

	t4 := NewTrace(fmt.Sprint(reqID), "WRITE_RESPONSE")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	t4.Done()
}
```

Use `defer` only for the total request timer or for stages with multiple returns. For short linear stages, call `Done()` immediately after the operation so the measured range is obvious.

## Output Rules

Prefer one log line per stage:

```json
{"event":"ssr_trace","request_id":"abc","stage":"FETCH","cost_ms":12}
{"event":"ssr_trace","request_id":"abc","stage":"BUILD_VIEW","cost_ms":3}
{"event":"ssr_trace","request_id":"abc","stage":"RENDER_TEMPLATE","cost_ms":45}
{"event":"ssr_trace","request_id":"abc","stage":"TOTAL","cost_ms":67}
```

Use stable stage names such as `TOTAL`, `FETCH`, `DB_QUERY`, `RPC_CALL`, `BUILD_VIEW`, `RENDER_TEMPLATE`, and `WRITE_RESPONSE`. Include route, handler, tenant, or user only when safe and useful; never log secrets, tokens, raw authorization headers, full payloads, or sensitive personal data.

## Review Checklist

- The helper imports only what is needed, usually `log` or `fmt`, `time`, and possibly `os`.
- Logs go through the existing project logger when one is established.
- Every trace line can be correlated by request ID.
- SSR is split at least into fetch, build view, render template, and total.
- Error paths still emit total timing and do not double-write responses.
- JSON output is valid or uses the project's structured logger instead.
- The final response tells the user that this is coarse timing and suggests pprof or trace only for deeper function/runtime diagnosis.
