import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { lineageFixture } from "./lineage-fixtures";
import { occurrenceKey } from "./lineage";
import { initialSelection, reduceSelection } from "./selection";
import type { SelectionContext } from "./selection";

const context: SelectionContext = {
  model: "synthetic-map",
  environments: ["isolated"],
  branches: [
    { id: "A", action: "create", flow: "payment" },
    { id: "B", action: "confirm", flow: "payment" },
  ],
  runs: [
    {
      id: "run-A",
      branch: "A",
      branches: ["A", "B"],
      environment: "isolated",
      flow: "payment",
      steps: [
        {
          id: "1",
          node: "create",
          actors: ["merchant", "channel"],
          entities: ["entity-A"],
          branches: ["A", "B"],
          actorBranches: { merchant: ["A"], channel: ["B"] },
          actorNodes: { merchant: ["create"], channel: ["confirm"] },
        },
      ],
    },
  ],
};

function selected() {
  const branch = reduceSelection(
    initialSelection(context.model, "isolated"),
    { type: "branch", value: "A" },
    context,
  );
  return reduceSelection(branch, { type: "run", value: "run-A" }, context);
}

const occurrence = {
  run: "run-A",
  step: "1",
  actor: "channel",
  action_index: 1,
  branch: "B",
  occurrence: JSON.stringify(["run-A", 1, "channel", 1, "before", "B", null]),
  entity_keys: ["entity-A"],
};

describe("business map and actual path navigation", () => {
  it("opens the model as a shared declaration map without borrowing a run's identities", () => {
    const html = renderToStaticMarkup(<App state={{ snapshot: lineageFixture() }} />);
    expect(html).toContain('aria-label="系谱范围"');
    expect(html).toContain("共享依赖与影响分支");
    expect(html).toContain("全模型声明");
    expect(html).not.toContain("principal-A");
    expect(html).not.toContain("principal-B-private-marker");
  });

  it("retains exact run selection while inspecting declarations and returns to branch scope on a branch choice", () => {
    const state = selected();
    const model = reduceSelection(state, { type: "dependencyScope", value: "model" }, context);
    expect(model).toMatchObject({
      dependencyScope: "model",
      canvas: "dependencies",
      branch: "A",
      run: "run-A",
      step: "1",
      node: "",
      focus: "all",
    });
    const branch = reduceSelection(model, { type: "branch", value: "B" }, context);
    expect(branch).toMatchObject({ dependencyScope: "branch", branch: "B", run: "" });
  });

  it("keeps evidence damage visible in the selected branch's blocking explanation", () => {
    const snapshot = lineageFixture();
    snapshot.runs[0].evidence_status = "BLOCKED";
    snapshot.runs[0].evidence_errors = ["synthetic RAW checksum mismatch"];
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{
          branch: "B-CALLBACK",
          run: "synthetic-run-A",
          step: "3",
          canvas: "dependencies",
        }}
      />,
    );
    expect(html).toContain('aria-label="前态阻断解释"');
    expect(html).toContain("证据阻断");
    expect(html).toContain("synthetic RAW checksum mismatch");
    expect(html).not.toContain("本次观察未发现前态阻断");
  });

  it("selects one observed concurrent action atomically without losing the run or entity", () => {
    const state = reduceSelection(selected(), { type: "entity", value: "entity-A" }, context);
    const next = reduceSelection(state, { type: "observed", value: occurrence }, context);
    expect(next).toMatchObject({
      run: "run-A",
      branch: "B",
      step: "1",
      actor: "channel",
      node: "confirm",
      entity: "entity-A",
      occurrence: occurrence.occurrence,
      canvas: "states",
      focus: "all",
    });
  });

  it("shows only the selected action occurrence's response and Oracle when inspecting a state edge", () => {
    const snapshot = lineageFixture();
    const run = snapshot.runs[0];
    const fact = run.facts![2];
    const first = fact.business_expectations![0];
    fact.business_expectations!.push({ ...structuredClone(first), action_index: 1 });
    fact.actions = [0, 1].map((action_index) => ({
      action_index,
      node: first.decision.frozen!.action,
      actor: first.actor,
      principal: "principal-A",
      request_sha256: `synthetic-request-${action_index}`,
      response_sha256: `synthetic-response-${action_index}`,
      response_values: {
        receipt: action_index === 0 ? "selected-action-receipt" : "sibling-action-receipt",
      },
      exit_code: 0,
      denied: false,
      started: "2026-11-17T00:00:00Z",
      completed: "2026-11-17T00:00:01Z",
    }));
    fact.oracles = [0, 1].map((action_index) => ({
      ...structuredClone(fact.oracles![0]),
      action_index,
      actor: first.actor,
      id: action_index === 0 ? "selected-action-oracle" : "sibling-action-oracle",
    }));
    const overlay = run.business_lineage![0];
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{
          branch: "B-CALLBACK",
          run: run.trace.run,
          actor: first.actor,
          step: "3",
          occurrence: occurrenceKey(overlay),
          canvas: "states",
        }}
      />,
    );
    expect(html).toContain("selected-action-receipt");
    expect(html).toContain("selected-action-oracle");
    expect(html).not.toContain("sibling-action-receipt");
    expect(html).not.toContain("sibling-action-oracle");
  });

  it("rejects an observed-path jump from a different run, actor, branch or entity", () => {
    const state = reduceSelection(selected(), { type: "entity", value: "entity-A" }, context);
    for (const wrong of [
      { ...occurrence, run: "run-B" },
      { ...occurrence, actor: "stranger" },
      { ...occurrence, branch: "A" },
      { ...occurrence, entity_keys: ["entity-B"] },
      {
        ...occurrence,
        occurrence: JSON.stringify(["run-A", 2, "channel", 1, "before", "B", null]),
      },
    ])
      expect(reduceSelection(state, { type: "observed", value: wrong }, context)).toEqual(state);
  });

  it("limits blocking-path focus to dependency views and clears it on state navigation", () => {
    const state = { ...selected(), canvas: "dependencies" as const };
    const blocked = reduceSelection(state, { type: "focus", value: "blocked" }, context);
    expect(blocked.focus).toBe("blocked");
    expect(reduceSelection(blocked, { type: "canvas", value: "states" }, context).focus).toBe(
      "all",
    );
    expect(reduceSelection(selected(), { type: "focus", value: "blocked" }, context)).toEqual(
      selected(),
    );
  });
});
