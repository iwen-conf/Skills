---
name: perfetto-trace-analysis
description: Analyzes Perfetto traces to find the root cause of latency, memory, or
  jank issues in Android apps. Use when the user provides a Perfetto trace file and
  asks any question, ongoing investigation, or open-ended request to analyze its contents.
license: Complete terms in LICENSE.txt
metadata:
  requires:
    skills: [perfetto-sql]
  author: Google LLC
  last-updated: '2026-05-14'
  keywords:
  - Perfetto
  - trace analysis
  - Android performance
  - debugging
  - profiling
  - jank
  - bottleneck
  - SQL
---

## Evidence Gate

MUST load the evidence-first root-cause repair gate in
[`docs/execution-truth.md`](../../docs/execution-truth.md). Treat a long slice, metric outlier, or
user suspicion as a signal rather than a confirmed defect; keep facts and hypotheses separate,
follow the dependency chain to the owning boundary, and recommend a repair only when the trace
evidence and expected app behavior confirm the cause.

## Resources

- **Domain Hints:** Reference files for specific performance areas: [`CPU`](references/hints_cpu.md), [`Graphics`](references/hints_graphics.md), [`I/O`](references/hints_io.md), [`IPC`](references/hints_ipc.md), [`Memory`](references/hints_memory.md), [`Power`](references/hints_power.md). These files each contain multiple expert-vetted, powerful trace analysis techniques to steer and aid in the analysis.
- **Perfetto SQL Reference:** Reference guidelines for translating intents into valid queries are located in [the SQL reference](references/sql.md). You must read this reference and follow its Execution Protocol for all SQL generation.

## Setup Phase (Mandatory)

1. **Initialize Scratchpad (Chain of Evidence):**
   - Maintain your working memory in a local scratchpad file located in an independent temporary work directory for read-only requests; use the trace directory only when that output location is authorized.
   - Name the file using the trace's filename appended with `_analysis.md` (e.g., `[trace_filename]_analysis.md`). Before creating it, check if a file with that name already exists using host-approved exact-path discovery---to avoid biasing your investigation, DO NOT read the file's contents to check for its existence. If it does, append an incrementing version number (e.g., `_v2.md`, `_v3.md`) until you find an available filename. You MUST hardcode this exact filename in all subsequent tool calls.
   - Use this scratchpad STRICTLY to log verified facts: timestamps, slice names, thread IDs (utid/tid), and thread states.
   - DO NOT write preliminary hypotheses or premature conclusions in the scratchpad. It is a strict Chain of Evidence.
2. **Review Domain Hints:** Load only the domain hints implicated by the user question or observed evidence; expand when a dependency crosses that domain.
3. **Review SQL Reference:** Read the SQL reference in [`references/sql.md`](references/sql.md) and follow its Execution Protocol for all SQL generation. Do not guess schemas.
4. **Target Resolution:** If the user's request is broad (e.g., "why is the app slow?") and doesn't specify a package name:
   - Execute a query to identify the active application: `sql INCLUDE
     PERFETTO MODULE android.startup.startups; SELECT package FROM
     android_startups;`
   - If multiple packages are returned, ask the user to choose one. Save the chosen `package_name` to your scratchpad.

## Investigation Protocol

Bound investigation by the user question, package and observation window. Stop with a confirmed chain or an explicit evidence gap when required events are absent or further queries add no evidence.

### 1. Formulate Hypothesis

- **Prioritization:** Form hypotheses using information from: user prompt \> "Domain Hints" ([`CPU`](references/hints_cpu.md), [`Graphics`](references/hints_graphics.md), [`I/O`](references/hints_io.md), [`IPC`](references/hints_ipc.md), [`Memory`](references/hints_memory.md), [`Power`](references/hints_power.md)) \> general knowledge. Be sure to leverage these "Domain Hints" as they are expert-vetted analysis techniques.
- **Source Attribution:** Explicitly mention the source of your hypothesis (e.g., "Based on hints_io.md...").
- **Focus Constraint:** Focus on the primary bottleneck. Avoid investigating deep into binder transactions unless the user explicitly asks for it or there is no other obvious bottleneck.
- **State Reasoning:** Briefly state your reasoning based on previous findings *before* generating a new query.

### 2. Plan and Collect Data

- **Metrics First:** Start with a high-level view using trace metrics before diving into custom SQL (e.g., `"$TRACE_PROCESSOR" --run-metrics
  android_startup`).
- **Broad to Narrow:** Begin with broad queries using minimal filters. Favor fuzzy matching (e.g., `GLOB '*abc*'`) over exact matching.
- **Overlapping Time:** When filtering by time, you MUST check for events that overlap with the target time range (e.g., `start1 < end2 AND start2 < end1`) to ensure you don't miss slices that span across the boundaries.

### 3. Analyze and Drill Down (Depth-First)

- **Evidentiary Rigor:** Do not draw conclusions without explicit data.
- **Wall Time vs. CPU Time:** Do not assume a long-running slice is actively computing. You MUST query the `thread_state` table for the exact timestamp window of suspicious slices to verify if the thread was `Running`, `Runnable` (waiting for CPU), or `Sleeping`/`Uninterruptible Sleep` (blocked).
- **Follow Dependencies:** If a thread is blocked/waiting, you MUST find what it is waiting *for* (Binder, Lock, I/O, etc.). Cross process boundaries if necessary. If blocker events are missing, report the wait and missing evidence; do not invent a definitive root cause.

### 4. Coverage and stopping

Check related sibling bottlenecks within the requested window when evidence warrants it. A system-wide review is a separate requested scope, not a prerequisite for every latency question. A disproven hypothesis can justify another bounded query; missing instrumentation, unavailable tools or repeated queries without new evidence require an explicit partial/blocked result.

## Final Report

Report the target/window, verified dependency chain, CPU versus wall time, query evidence, unresolved hypotheses and missing events. State what was not covered and link the scratchpad. A conclusion must not claim no other bottlenecks exist outside the observed scope.
