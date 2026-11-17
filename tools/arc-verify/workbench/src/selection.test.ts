import { describe, expect, it } from "vite-plus/test";
import { initialSelection, matchingRuns, reduceSelection } from "./selection";
import type { SelectionContext } from "./selection";

const context: SelectionContext = {
  model: "payments-v3",
  environments: ["isolated", "other"],
  branches: [
    { id: "active", action: "create", flow: "payment" },
    { id: "disabled", action: "callback", flow: "payment" },
  ],
  runs: [
    {
      id: "run-A",
      branch: "active",
      flow: "payment",
      environment: "isolated",
      steps: [
        { id: "1", node: "create", actors: ["merchant-A"], entities: ["order-A"] },
        { id: "2", node: "callback", actors: ["channel"], entities: ["order-A"] },
      ],
    },
    {
      id: "run-B",
      branch: "active",
      flow: "payment",
      environment: "isolated",
      steps: [{ id: "1", node: "create", actors: ["merchant-B"], entities: ["order-B"] }],
    },
    { id: "run-other", branch: "active", flow: "payment", environment: "other", steps: [] },
    { id: "run-disabled", branch: "disabled", flow: "payment", environment: "isolated", steps: [] },
  ],
};
const reduce = (
  state: ReturnType<typeof initialSelection>,
  type: "branch" | "run" | "step" | "node" | "actor" | "entity" | "environment" | "flow",
  value: string,
) => reduceSelection(state, { type, value }, context);
const selected = () =>
  reduce(reduce(initialSelection(context.model, "isolated"), "branch", "active"), "run", "run-A");

describe("one business selection context", () => {
  it("requires an explicit branch before any run and excludes other branches or environments", () => {
    const state = initialSelection(context.model, "isolated");
    expect(matchingRuns(context, state)).toEqual([]);
    expect(reduce(state, "run", "run-A")).toEqual(state);
    expect(matchingRuns(context, reduce(state, "branch", "active")).map((run) => run.id)).toEqual([
      "run-A",
      "run-B",
    ]);
    expect(reduce(selected(), "run", "run-disabled").run).toBe("run-A");
  });
  it("locates nodes from steps and steps from nodes in the same run", () => {
    expect(reduce(selected(), "step", "2")).toMatchObject({
      node: "callback",
      step: "2",
      run: "run-A",
    });
    expect(reduce(selected(), "node", "callback")).toMatchObject({
      node: "callback",
      step: "2",
      run: "run-A",
    });
    expect(reduce(selected(), "node", "unexecuted")).toMatchObject({
      node: "unexecuted",
      step: "",
    });
  });
  it("never carries an account or entity filter into another run", () => {
    let state = reduce(reduce(selected(), "actor", "merchant-A"), "entity", "order-A");
    state = reduce(state, "run", "run-B");
    expect(state).toMatchObject({ run: "run-B", actor: "", entity: "", step: "1" });
    expect(reduce(state, "actor", "merchant-A")).toEqual(state);
    expect(reduce(state, "entity", "order-A")).toEqual(state);
  });
  it("clears evidence on branch, flow and environment changes", () => {
    for (const [type, value] of [
      ["branch", "disabled"],
      ["flow", "refund"],
      ["environment", "other"],
    ] as const) {
      expect(reduce(selected(), type, value)).toMatchObject({
        run: "",
        step: "",
        actor: "",
        entity: "",
      });
    }
  });
  it("filters steps by the selected participant without silently displaying another actor", () => {
    const state = reduce(selected(), "actor", "channel");
    expect(state).toMatchObject({ node: "callback", step: "2" });
    expect(reduce(state, "step", "1")).toEqual(state);
    expect(reduce(state, "node", "create").step).toBe("");
  });
  it("follows persisted step-to-branch associations within one continuous run", () => {
    const continuous: SelectionContext = structuredClone(context);
    continuous.runs[0].branches = ["active", "disabled"];
    continuous.runs[0].steps[0].branches = ["active"];
    continuous.runs[0].steps[1].branches = ["disabled"];
    const state = reduceSelection(selected(), { type: "step", value: "2" }, continuous);
    expect(state).toMatchObject({ branch: "disabled", run: "run-A", step: "2", node: "callback" });
    expect(matchingRuns(continuous, state).map((run) => run.id)).toContain("run-A");
  });
  it("selects the recorded actor's node and branch when one step contains multiple actions", () => {
    const multi = structuredClone(context);
    multi.runs[0].branches = ["active", "disabled"];
    multi.runs[0].steps[0] = {
      id: "1",
      node: "create",
      actors: ["merchant-A", "channel"],
      entities: ["order-A"],
      branches: ["active", "disabled"],
      actorNodes: { "merchant-A": ["create"], channel: ["callback"] },
      actorBranches: { "merchant-A": ["active"], channel: ["disabled"] },
    };
    const state = reduceSelection(selected(), { type: "actor", value: "channel" }, multi);
    expect(state).toMatchObject({
      run: "run-A",
      step: "1",
      actor: "channel",
      branch: "disabled",
      node: "callback",
    });
    expect(reduceSelection(state, { type: "node", value: "create" }, multi).step).toBe("");
    expect(reduceSelection(state, { type: "actor", value: "merchant-A" }, multi)).toMatchObject({
      run: "run-A",
      step: "1",
      branch: "active",
      node: "create",
    });
  });
});
