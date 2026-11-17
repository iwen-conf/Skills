import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { branchOptions, selectionContext } from "./explorer-model";
import { canvasSelectionAnchors } from "./graph-focus";
import { graphFor } from "./graph-model";
import { lineageFixture } from "./lineage-fixtures";
import { lineageSelection, occurrenceKey } from "./lineage";
import { entityKey } from "./facts";
import { initialSelection, reduceSelection } from "./selection";
import { parseSnapshot } from "./snapshot";

function context() {
  const snapshot = lineageFixture();
  const state = {
    ...initialSelection(snapshot.meta.graph, snapshot.meta.environment),
    flow: "F-PAY",
    branch: "B-CALLBACK",
    node: "callback",
    canvas: "dependencies" as const,
  };
  return { snapshot, state, branches: branchOptions(snapshot) };
}

function preparationOnly() {
  const snapshot = lineageFixture();
  const run = snapshot.runs[0];
  const fact = structuredClone(run.facts![0]);
  fact.seq = -1;
  fact.source_kind = "setup";
  fact.node = "setup";
  fact.evidence_sha256 = "synthetic-setup";
  fact.business_expectations = [];
  fact.preparation = {
    status: "BLOCKED",
    reason: "synthetic preparation refusal",
    contract_sha256: "synthetic-contract",
    initial_observed: { state: "disabled", values: { "merchant.enabled": true } },
    final_observed: { state: "disabled", values: { "merchant.enabled": true } },
    plans: [],
    steps: [],
    readbacks: [{ index: 0, phase: "initial", observation: { state: "disabled", values: {} } }],
    observation_reads: 1,
    explored_states: 0,
    parallel_executed: false,
    target_ready: false,
  };
  run.facts = [fact];
  run.trace.steps = [];
  run.trace.status = "BLOCKED";
  const overlay = run.business_lineage[0];
  Object.assign(overlay, {
    phase: "preparation",
    step: -1,
    readback_index: 0,
    readback_phase: "initial",
    diagnostic: true,
    evidence_sha256: "synthetic-setup",
    reason: "synthetic preparation refusal",
  });
  overlay.nodes.forEach((node) => {
    node.status = "BLOCKED";
  });
  return snapshot;
}

describe("branch-centered dependency genealogy", () => {
  it("explains a model-only branch with its target, pre-state and complete logical condition tree", () => {
    const { snapshot, state, branches } = context();
    const graph = graphFor(snapshot, state, branches);
    expect(graph.nodes.map((node) => node.id)).toEqual(
      expect.arrayContaining([
        "target",
        "pre-state",
        "guard",
        "account-condition",
        "order-condition",
      ]),
    );
    expect([...canvasSelectionAnchors(graph, state)]).toEqual(["target"]);
    expect(graph.nodes.every((node) => node.status === "NOT_OBSERVED")).toBe(true);
    expect(graph.edges.filter((edge) => edge.kind === "candidate_provider")).toHaveLength(2);
  });

  it("provides a ranked readable directory and names alternative providers without a run", () => {
    const { snapshot, state } = context();
    const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={state} />);
    expect(html).toContain('aria-label="依赖系谱分层目录"');
    expect(html).toContain("候选来源");
    expect(html).toContain("尚无实际前态观察");
    expect(html).toContain("停用账号 · 入口 A");
    expect(html).not.toContain("principal-A");
  });

  it("overlays only the selected run step and actor while preserving declaration-only providers", () => {
    const { snapshot, state, branches } = context();
    const run = snapshot.runs[0];
    const graph = graphFor(
      snapshot,
      { ...state, run: run.trace.run, step: "3", actor: "merchant" },
      branches,
      run,
    );
    expect(graph.nodes.find((node) => node.id === "account-condition")?.status).toBe("SATISFIED");
    expect(graph.nodes.find((node) => node.id === "provider-a")?.status).toBe("NOT_OBSERVED");
    const unobserved = graphFor(
      snapshot,
      { ...state, run: run.trace.run, step: "2", actor: "merchant" },
      branches,
      run,
    );
    expect(unobserved.nodes.every((node) => node.status === "NOT_OBSERVED")).toBe(true);
  });

  it("never colors readiness from a recorded result when source evidence is blocked", () => {
    const { snapshot, state, branches } = context();
    const run = snapshot.runs[0];
    run.evidence_status = "BLOCKED";
    const graph = graphFor(
      snapshot,
      { ...state, run: run.trace.run, step: "3", actor: "merchant" },
      branches,
      run,
    );
    expect(graph.nodes.find((node) => node.id === "target")?.status).toBe("BLOCKED");
    expect(graph.nodes.some((node) => node.status === "SATISFIED" || node.status === "READY")).toBe(
      false,
    );
  });

  it("does not merge two occurrences of the same Oracle and actor within one step", () => {
    const { snapshot, state, branches } = context();
    const run = snapshot.runs[0];
    run.business_lineage.push({ ...structuredClone(run.business_lineage[0]), action_index: 1 });
    const graph = graphFor(
      snapshot,
      { ...state, run: run.trace.run, step: "3", actor: "merchant" },
      branches,
      run,
    );
    expect(graph.nodes.every((node) => node.status === "NOT_OBSERVED")).toBe(true);
    expect(graph.caption).toContain("选择动作发生记录");
  });

  it("keeps the detail panel on the selected action occurrence instead of showing a sibling freeze", () => {
    const { snapshot, state } = context();
    const run = snapshot.runs[0];
    const other = structuredClone(run.facts![2].business_expectations![0]);
    other.action_index = 1;
    other.decision.reason = "other-action-private-marker";
    run.facts![2].business_expectations!.push(other);
    run.business_lineage.push({ ...structuredClone(run.business_lineage[0]), action_index: 1 });
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{
          ...state,
          run: run.trace.run,
          step: "3",
          actor: "merchant",
          occurrence: occurrenceKey(run.business_lineage[0]),
        }}
      />,
    );
    expect(html).toContain("synthetic unique branch");
    expect(html).not.toContain("other-action-private-marker");
  });

  it("restores an exact condition selection without losing the observation step", () => {
    const { snapshot, state } = context();
    const run = snapshot.runs[0];
    run.business_lineage[0].nodes.find(
      (node) => node.node === "account-condition",
    )!.values![0].sources[0].oracle = "selected-condition-source-marker";
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{
          ...state,
          run: run.trace.run,
          step: "3",
          actor: "merchant",
          node: "account-condition",
          occurrence: occurrenceKey(run.business_lineage[0]),
        }}
      />,
    );
    expect(html).toContain("selected-condition-source-marker");
  });

  it("rejects erased actual values, bindings and sources behind a satisfied condition", () => {
    const snapshot = lineageFixture();
    const value = snapshot.runs[0].business_lineage[0].nodes.find(
      (node) => node.node === "account-condition",
    )!.values![0];
    delete value.actual;
    delete value.binding;
    value.sources = [];
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("rejects positive status paired with a false recorded predicate without recomputing business rules", () => {
    const snapshot = lineageFixture();
    snapshot.runs[0].business_lineage[0].nodes.find(
      (node) => node.node === "account-condition",
    )!.evaluation!.value = "FALSE";
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("rejects malformed nested predicates before the graph renderer can dereference them", () => {
    const snapshot = lineageFixture();
    Object.assign(
      snapshot.business_lineage.branches[0].nodes.find((node) => node.id === "guard")!,
      { predicate: { op: "and", args: {} } },
    );
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("rejects a mutually truncated graph and overlay that retain only a READY target", () => {
    const snapshot = lineageFixture();
    const graph = snapshot.business_lineage.branches[0];
    graph.nodes = graph.nodes.filter((node) => node.kind === "target");
    graph.edges = [];
    snapshot.runs[0].business_lineage[0].nodes = snapshot.runs[0].business_lineage[0].nodes.filter(
      (node) => node.node === "target",
    );
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("does not transfer a before observation to another full entity identity sharing the same bare ID", () => {
    const { snapshot, state } = context();
    const run = snapshot.runs[0];
    const fact = run.facts![2];
    const before = fact.entities![0];
    fact.before_entities = [before];
    const other = { ...before, owner_actor: "other", owner_principal: "principal-B" };
    fact.entities = [other];
    Object.assign(
      run.business_lineage[0].nodes.find((node) => node.node === "order-condition")!.values![0],
      { entity_ref: before },
    );
    const selected = {
      ...state,
      run: run.trace.run,
      step: "3",
      actor: "merchant",
      entity: entityKey(other),
    };
    expect(lineageSelection(snapshot, selected, run).overlay).toBeUndefined();
  });

  it("accepts an explicit subject-to-entity identity even when its alias differs from the contract subject", () => {
    const { snapshot, state } = context();
    const run = snapshot.runs[0];
    const entity = { ...run.facts![2].entities![0], alias: "captured-order" };
    run.facts![2].before_entities = [entity];
    run.facts![2].entities = [entity];
    Object.assign(
      run.business_lineage[0].nodes.find((node) => node.node === "order-condition")!.values![0],
      { entity_ref: entity },
    );
    const selected = {
      ...state,
      run: run.trace.run,
      step: "3",
      actor: "merchant",
      entity: entityKey(entity),
    };
    expect(lineageSelection(snapshot, selected, run).overlay).toBe(run.business_lineage[0]);
  });

  it("rejects an entity reference whose full identity is absent from the recorded before bindings", () => {
    const snapshot = lineageFixture();
    const run = snapshot.runs[0];
    const entity = run.facts![2].entities![0];
    run.facts![2].before_entities = [entity];
    Object.assign(
      run.business_lineage[0].nodes.find((node) => node.node === "order-condition")!.values![0],
      {
        entity_ref: { ...entity, owner_principal: "foreign-owner" },
      },
    );
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("keeps a true OR guard valid when its other operand is explicitly unobserved", () => {
    const snapshot = lineageFixture();
    const run = snapshot.runs[0];
    snapshot.model.data.business_contract!.branches[2].when.op = "or";
    const before = run.facts![2].business_expectations![0].decision.frozen!.before;
    delete before.values["order.inflight"];
    delete before.sources!["order.inflight"];
    delete run.facts![2].oracles![0].before.values["order.inflight"];
    for (const reading of run.business_lineage[0].nodes) {
      for (const value of reading.values ?? [])
        if (value.subject === "order") {
          delete value.actual;
          value.sources = [];
        }
      if (reading.node === "order-condition") {
        reading.status = "NOT_OBSERVED";
        reading.evaluation = { value: "UNKNOWN" };
      }
    }
    expect(() => parseSnapshot(snapshot)).not.toThrow();
    expect(run.business_lineage[0].nodes.find((node) => node.node === "target")?.status).toBe(
      "READY",
    );
  });

  it("keeps a false NOT operand as a valid explanation of a true root guard", () => {
    const snapshot = lineageFixture();
    const run = snapshot.runs[0];
    const branch = snapshot.model.data.business_contract!.branches[2];
    const child = structuredClone(branch.when.args![0]);
    child.right!.literal!.value = true;
    branch.when = { op: "not", args: [child] };
    const graph = snapshot.business_lineage.branches[0];
    const retained = new Set(["target", "pre-state", "guard", "account-condition"]);
    graph.nodes = graph.nodes.filter((node) => retained.has(node.id));
    graph.edges = graph.edges.filter((edge) => retained.has(edge.from) && retained.has(edge.to));
    for (const node of graph.nodes) {
      node.references = node.references?.filter((ref) => ref.subject === "merchant");
      if (node.id === "guard") node.predicate = branch.when;
      if (node.id === "account-condition") node.predicate = child;
    }
    run.business_lineage[0].nodes = run.business_lineage[0].nodes.filter((node) =>
      retained.has(node.node),
    );
    for (const reading of run.business_lineage[0].nodes) {
      reading.values = reading.values?.filter((value) => value.subject === "merchant");
      if (reading.node === "account-condition") {
        reading.status = "UNSATISFIED";
        reading.evaluation = { value: "FALSE" };
      }
    }
    expect(() => parseSnapshot(snapshot)).not.toThrow();
    const state = {
      ...initialSelection(snapshot.meta.graph),
      branch: branch.id,
      run: run.trace.run,
      step: "3",
      actor: "merchant",
      canvas: "dependencies" as const,
    };
    const projected = graphFor(snapshot, state, branchOptions(snapshot), run);
    expect(projected.nodes.find((node) => node.id === "target")?.status).toBe("READY");
    expect(projected.nodes.find((node) => node.id === "account-condition")?.status).toBe(
      "UNSATISFIED",
    );
  });

  it("rejects malformed genealogy rather than silently omitting a missing prerequisite", () => {
    const { snapshot } = context();
    snapshot.business_lineage.branches[0].edges[0].from = "missing-precondition";
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("rejects observation overlays belonging to a different run or unknown condition", () => {
    const { snapshot } = context();
    snapshot.runs[0].business_lineage[0].run = "synthetic-run-B";
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
    const next = lineageFixture();
    next.runs[0].business_lineage[0].nodes[0].node = "foreign-node";
    expect(() => parseSnapshot(next)).toThrow(/系谱|lineage/);
  });

  it("refuses an incomplete condition ledger even when its retained target says READY", () => {
    const snapshot = lineageFixture();
    snapshot.runs[0].business_lineage[0].nodes = snapshot.runs[0].business_lineage[0].nodes.filter(
      (node) => node.node !== "order-condition",
    );
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("refuses a static genealogy that omits a declared business branch", () => {
    const snapshot = lineageFixture();
    snapshot.business_lineage.branches.pop();
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });

  it("keeps the same occurrence selected when inspecting a dependency condition", () => {
    const { snapshot, state, branches } = context();
    const selected = { ...state, run: snapshot.runs[0].trace.run, step: "3", actor: "merchant" };
    const next = reduceSelection(
      selected,
      { type: "lineage", value: "account-condition" },
      selectionContext(snapshot, branches),
    );
    expect(next.node).toBe("account-condition");
    expect(next.step).toBe("3");
    expect(next.branch).toBe("B-CALLBACK");
    expect(next.actor).toBe("merchant");
  });

  it("keeps an explicitly targeted preparation failure selectable before any business freeze", () => {
    const snapshot = preparationOnly();
    const context = selectionContext(snapshot);
    const selected = reduceSelection(
      initialSelection(snapshot.meta.graph),
      { type: "branch", value: "B-CALLBACK" },
      context,
    );
    const run = reduceSelection(
      selected,
      { type: "run", value: snapshot.runs[0].trace.run },
      context,
    );
    expect(run.run).toBe(snapshot.runs[0].trace.run);
    expect(run.step).toBe("-1");
    expect(run.branch).toBe("B-CALLBACK");
  });

  it("accepts scoped preparation diagnostics and retains their first observed values without target readiness", () => {
    const snapshot = preparationOnly();
    expect(() => parseSnapshot(snapshot)).not.toThrow();
    const state = {
      ...initialSelection(snapshot.meta.graph),
      branch: "B-CALLBACK",
      run: snapshot.runs[0].trace.run,
      step: "-1",
      actor: "merchant",
      canvas: "dependencies" as const,
    };
    const graph = graphFor(snapshot, state, branchOptions(snapshot), snapshot.runs[0]);
    expect(graph.nodes.find((node) => node.id === "account-condition")?.status).toBe("BLOCKED");
    expect(graph.nodes.some((node) => node.status === "READY")).toBe(false);
    expect(graph.caption).toContain("synthetic preparation refusal");
  });

  it("rejects preparation observations outside their recorded readback and participant scope", () => {
    const snapshot = preparationOnly();
    Object.assign(snapshot.runs[0].business_lineage[0], {
      readback_index: 999,
      readback_phase: "post_action",
      actor: "unrecorded-other-account",
    });
    expect(() => parseSnapshot(snapshot)).toThrow(/系谱|lineage/);
  });
});
