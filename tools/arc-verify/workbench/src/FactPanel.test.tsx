import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import type { ActionObservation, PreparationExecution } from "./facts";
import { parseSnapshot } from "./snapshot";
import { contractFixture } from "./test-fixtures";

const capture: ActionObservation = {
  node: "create",
  actor: "merchant",
  principal: "principal-A",
  request_sha256: "synthetic-request",
  response_sha256: "synthetic-response",
  response_values: {
    "transport.status_code": 201,
    "response.order_id": "synthetic-captured-order",
  },
  status_code: 201,
  exit_code: 0,
  denied: false,
  started: "2026-11-17T00:00:00Z",
  completed: "2026-11-17T00:00:01Z",
};
const observed = { values: { "merchant.enabled": false } };
const ready = { values: { "merchant.enabled": true } };
const planned = {
  id: "synthetic-enable",
  action: "create",
  requires: { op: "true" },
  ensures: { op: "true" },
  predicted_effects: [{ subject: "merchant", attribute: "enabled", value: true }],
  source_ref: "synthetic UI fixture",
};
const preparation: PreparationExecution = {
  status: "READY",
  reason: "synthetic independent readiness",
  contract_sha256: "synthetic-contract",
  initial_observed: observed,
  final_observed: ready,
  plans: [
    {
      status: "PLANNED",
      reason: "synthetic prediction",
      layers: [{ index: 0, steps: [planned] }],
      requires_readback: true,
      blocked: [],
      gaps: [],
    },
  ],
  steps: [
    {
      index: 0,
      before_read: 0,
      after_read: 1,
      planned,
      before: observed,
      requirements: { value: "TRUE" },
      dependencies: {},
      receipt: { action: capture },
      after: ready,
      ensures: { value: "TRUE" },
      status: "PASS",
      reason: "synthetic actual readback",
    },
  ],
  readbacks: [
    { index: 0, phase: "initial", observation: observed },
    { index: 1, phase: "after", observation: ready },
  ],
  observation_reads: 2,
  explored_states: 2,
  parallel_executed: false,
  target_ready: true,
};

describe("new execution facts remain inspectable", () => {
  it("renders captured action values, setup preparation readbacks and checkpoint semantics", () => {
    const snapshot = contractFixture();
    const run = snapshot.runs[0];
    run.facts![0].actions = [capture];
    run.facts![0].requests = [
      {
        actor: "merchant",
        action_index: 0,
        node: "create",
        template_sha256: "synthetic-template",
        request_sha256: "synthetic-request",
        values: { "request.amount": "100.00", "request.currency": "synthetic-CNY" },
      },
    ];
    run.facts![0].business_expectations![0].request = run.facts![0].requests[0];
    run.facts!.unshift({
      source_kind: "setup",
      run: run.trace.run,
      seq: -1,
      graph: snapshot.meta.graph,
      verdict: "PASS",
      evidence_status: "PASS",
      evidence_version: 2,
      preparation,
    });
    run.trace.recovery = {
      mode: "checkpoint",
      verdict: "PASS",
      reason: "synthetic preserved state",
      state: "pending",
      safe: true,
      actions: [],
    };
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot: parseSnapshot(snapshot) }}
        initialContext={{ branch: "B-CREATE", run: run.trace.run, step: "1" }}
      />,
    );
    for (const text of [
      "实际操作与响应",
      "request-capture",
      "request.amount",
      "100.00",
      "synthetic-CNY",
      "transport.status_code",
      "201",
      "synthetic-captured-order",
      "本次运行准备",
      "准备计划历史",
      "预计",
      "实际执行的准备步骤",
      "synthetic actual readback",
      "准备独立回读记录",
      "现场检查点",
      "不表示环境已恢复",
      "跨运行自动继承变量尚未完成",
    ])
      expect(html).toContain(text);
    expect(html).not.toContain("principal-B-private-marker");
    expect(html).toContain("动作 0");
  });

  it("uses the selected actor's identity and decision within a shared step", () => {
    const snapshot = contractFixture();
    const run = snapshot.runs[0];
    const fact = run.facts![0];
    fact.participants!.push({
      alias: "channel",
      role: "CHANNEL",
      account: "channel-account-only",
      principal: "channel-principal-only",
      sha256: "synthetic",
    });
    fact.actions = [
      capture,
      {
        ...capture,
        node: "callback",
        actor: "channel",
        principal: "channel-principal-only",
        response_values: { "response.channel_marker": "channel-response-only" },
      },
    ];
    fact.business_expectations!.push({
      actor: "channel",
      action_index: 1,
      decision: {
        status: "UNKNOWN",
        reason: "synthetic missing channel condition",
        action: "callback",
        branch: "B-CALLBACK",
        matches: [],
        unknown: ["channel.ready"],
      },
    });
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{ branch: "B-CREATE", run: run.trace.run, step: "1", actor: "channel" }}
      />,
    );
    const scope = html.match(/<div class="scope-card">([\s\S]*?)<\/div>/)?.[1];
    expect(scope).toContain("channel-account-only");
    expect(scope).toContain("channel-principal-only");
    expect(scope).not.toContain("principal-A");
    expect(html).toContain("synthetic missing channel condition");
    expect(html).toContain("channel-response-only");
    expect(html).not.toContain("synthetic-captured-order");
    expect(html).not.toContain("synthetic unique branch");
  });
});
