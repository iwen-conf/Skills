import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { branchOptions, selectionContext } from "./explorer-model";
import { entityKey } from "./facts";
import { initialSelection, reduceSelection } from "./selection";
import { readSnapshotText, SnapshotError } from "./snapshot";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";
import { contractFixture } from "./test-fixtures";
import type { Snapshot } from "./types";

const readFixture = (snapshot: Snapshot) => readSnapshotText(JSON.stringify(snapshot));

describe("recorded scope and historical identity stay attached to displayed facts", () => {
  it("opens a model-only snapshot with Go's zero proof context without claiming a node proof", () => {
    const snapshot = contractFixture();
    // analyzeModel leaves ProofContext at its Go zero value until a round is selected.
    // Keep the actual export shape: view.graph identifies the model, context does not.
    snapshot.meta.round = "";
    snapshot.runs = [];
    snapshot.catalog.rounds = [];
    snapshot.catalog.runs = [];
    snapshot.business_campaigns = [];
    snapshot.contract_execution = undefined;
    for (const view of Object.values(snapshot.views)) {
      view.round = "";
      view.context = {
        graph: "",
        graph_sha256: "",
        environment: "",
        target: "",
        source_commit: "",
        source_sha256: "",
        config_sha256: "",
        source_root: "",
        config_path: "",
      };
      view.verdict = "NOT_RUN";
      view.nodes.forEach((node) => {
        node.status = "NOT_RUN";
        node.passed = 0;
        if (node.readiness) node.readiness.verified = false;
      });
    }
    const parsed = readFixture(snapshot);
    const html = renderToStaticMarkup(
      <App state={{ snapshot: parsed }} initialContext={{ branch: "B-CREATE" }} />,
    );
    expect(html).toContain("节点汇总未绑定验证轮次");
    expect(html).toContain("节点汇总不适用于当前选择");
    expect(html).not.toMatch(/<small>节点汇总<\/small><span class="status status-pass"/);
  });

  it.each(["environment", "round"] as const)(
    "does not display the export's node PASS as the selected %s's summary",
    (scope) => {
      const snapshot = contractFixture();
      const node = snapshot.views.capabilities.nodes.find((item) => item.id === "create")!;
      node.status = "PASS";
      node.reason = "only-the-exported-scope-was-verified";
      node.passed = node.required;
      if (node.readiness) node.readiness.verified = true;
      const other = snapshot.runs[1];
      other.environment = scope === "environment" ? "other-environment" : snapshot.meta.environment;
      other.trace.round = "other-round";
      const catalog = snapshot.catalog.runs.find((item) => item.id === other.trace.run)!;
      catalog.environment = other.environment!;
      catalog.round = other.trace.round;
      snapshot.catalog.rounds.push({
        ...snapshot.catalog.rounds[0],
        id: other.trace.round,
        environment: other.environment!,
      });
      const parsed = readFixture(snapshot);
      const original = renderToStaticMarkup(
        <App state={{ snapshot: parsed }} initialContext={{ branch: "B-CREATE" }} />,
      );
      expect(original).toContain("only-the-exported-scope-was-verified");
      expect(original).toMatch(/<small>节点汇总<\/small><span class="status status-pass"/);
      const html = renderToStaticMarkup(
        <App
          state={{ snapshot: parsed }}
          initialContext={{
            environment: other.environment,
            branch: "B-CREATE",
            run: other.trace.run,
          }}
        />,
      );
      const nodeSummary =
        html.match(/<p class="panel-status"><small>节点汇总<\/small>(.*?)<\/p>/)?.[1] ?? "";
      expect(nodeSummary).not.toContain("status-pass");
      expect(html).toContain("节点汇总不适用于当前选择");
    },
  );

  it.each([
    [
      "model id",
      (s: Snapshot) => {
        s.model.data.id = "another-model-version";
      },
    ],
    [
      "model digest",
      (s: Snapshot) => {
        s.model.sha256 = "another-model-digest";
      },
    ],
    [
      "projection graph",
      (s: Snapshot) => {
        s.views.capabilities.graph = "another-model-version";
      },
    ],
    [
      "projection context graph",
      (s: Snapshot) => {
        s.views.capabilities.context.graph = "another-model-version";
      },
    ],
    [
      "projection context digest",
      (s: Snapshot) => {
        s.views.capabilities.context.graph_sha256 = "another-model-digest";
      },
    ],
  ] as const)("refuses %s from another model before rendering recorded facts", (_name, mutate) => {
    const snapshot = contractFixture();
    mutate(snapshot);
    expect(() => readFixture(snapshot)).toThrow(SnapshotError);
  });

  it("refuses a claimed verification round whose proof context omits its model identity", () => {
    const snapshot = contractFixture();
    snapshot.views.capabilities.context.graph = "";
    snapshot.views.capabilities.context.graph_sha256 = "";
    expect(() => readFixture(snapshot)).toThrow(SnapshotError);
  });

  it("keeps a failing identity-change step selectable by its before-only entity", () => {
    const snapshot = contractFixture();
    const run = snapshot.runs[0];
    const fact = run.facts![1];
    const before = { ...fact.entities![0] };
    fact.before_entities = [before];
    fact.entities = [{ ...before, id: "wrong-replacement-entity" }];
    fact.verdict = "FAIL";
    fact.reason = "recorded-entity-identity-changed";
    fact.business_expectations![0].comparison!.status = "FAIL";
    fact.business_expectations![0].comparison!.reason = fact.reason;
    run.facts = [fact];
    run.trace.steps = [{ ...run.trace.steps[1], verdict: "FAIL", decision: fact.reason }];
    run.trace.first_divergence = "2";
    const parsed = readFixture(snapshot);
    const context = selectionContext(parsed);
    let state = reduceSelection(
      initialSelection(parsed.meta.graph, parsed.meta.environment),
      { type: "branch", value: "B-DISABLE" },
      context,
    );
    state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
    state = reduceSelection(state, { type: "entity", value: entityKey(before) }, context);
    expect(state.entity).toBe(entityKey(before));
    expect(state.step).toBe("2");
    const html = renderToStaticMarkup(<App state={{ snapshot: parsed }} initialContext={state} />);
    expect(html).toContain("recorded-entity-identity-changed");
    expect(html).toContain("动作后未记录此身份");
  });

  it.each([{ kind: "refund" }, { owner_actor: "secondary" }, { channel_account_id: "channel-B" }])(
    "does not merge captured namespaces or owners with equal display IDs: %j",
    (patch) => {
      const snapshot = contractFixture();
      const fact = snapshot.runs[0].facts![0];
      const before = { ...fact.entities![0], channel_account_id: "channel-A" };
      const after = { ...before, ...patch };
      fact.before_entities = [before];
      fact.entities = [after];
      fact.participants!.push({ ...fact.participants![0], alias: "secondary" });
      const parsed = readFixture(snapshot);
      const context = selectionContext(parsed);
      const step = context.runs.find((run) => run.id === fact.run)!.steps[0];
      expect(entityKey(before)).not.toBe(entityKey(after));
      expect(new Set(step.entities)).toEqual(new Set([entityKey(before), entityKey(after)]));
    },
  );

  it("safely leaves an old serialized entity selector empty instead of binding it to another identity", () => {
    const snapshot = readFixture(contractFixture());
    const run = snapshot.runs[0];
    const entity = run.facts![0].entities![0];
    const context = selectionContext(snapshot);
    let state = reduceSelection(
      initialSelection(snapshot.meta.graph, snapshot.meta.environment),
      { type: "branch", value: "B-CREATE" },
      context,
    );
    state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
    const oldKey = JSON.stringify([entity.alias, entity.id, entity.owner_principal]);
    state = reduceSelection(state, { type: "entity", value: oldKey }, context);
    expect(state.entity).toBe("");
    expect(state.run).toBe(run.trace.run);
    expect(state.step).toBe("1");
  });

  it("shows backend unresolved alternatives without changing a proven action's PASS or inventing one outcome", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const record = fact.business_expectations![0];
    const contract = snapshot.model.data.business_contract!;
    const branch = contract.branches[0];
    contract.subjects.push({
      id: "provider",
      kind: "external",
      requires_binding: false,
      attributes: [{ name: "unobserved_settlement", type: "boolean" }],
    });
    branch.expected = {
      mode: "allowed_outcomes",
      reason: "Synthetic allowed outcomes preserve an independently unobserved alternative",
      outcomes: [
        { ...branch.expected.outcomes[0], id: "known" },
        {
          id: "possible",
          denied: false,
          predicate: {
            op: "eq",
            left: { ref: { subject: "provider", attribute: "unobserved_settlement" } },
            right: { literal: { type: "boolean", value: true } },
          },
        },
      ],
    };
    record.decision.frozen!.expected = structuredClone(branch.expected);
    record.comparison = {
      status: "PASS",
      reason: "current-action-proved-with-unresolved-alternative",
      branch: "B-CREATE",
      outcome: "",
      checks: [
        {
          outcome: "known",
          state_passed: true,
          denied_passed: true,
          predicate: { value: "TRUE" },
        },
        {
          outcome: "possible",
          denied_passed: true,
          predicate: { value: "UNKNOWN", unknown: ["provider.unobserved_settlement"] },
        },
      ],
      unknown: ["provider.unobserved_settlement"],
      next_allowed: [],
    };
    mirrorRaw(snapshot, run, fact);
    const parsed = readFixture(snapshot);
    const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
    expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot: parsed }}
        initialContext={{ branch: "B-CREATE", run: snapshot.runs[0].trace.run, step: "1" }}
      />,
    );
    expect(html).toContain("provider.unobserved_settlement");
    expect(html).toContain("未记录唯一结果身份");
    const comparison = html
      .split('<p class="panel-status">')
      .map((panel) => panel.split("</p>")[0])
      .find((panel) => panel.includes("current-action-proved-with-unresolved-alternative"));
    expect(comparison).toContain("status-pass");
    expect(comparison).not.toContain("status-fail");
    expect(html).toContain("无合法后继（显式空集合）");
  });
});
