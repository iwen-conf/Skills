import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { branchOptions, selectionContext } from "./explorer-model";
import { initialSelection, matchingRuns, reduceSelection } from "./selection";
import { readSnapshotText } from "./snapshot";
import { contractFixture } from "./test-fixtures";

function partiallyAssociatedRun() {
  const snapshot = contractFixture();
  const run = snapshot.runs[0];
  run.trace.steps = run.trace.steps.slice(0, 2);
  run.facts = run.facts!.slice(0, 2);
  // snapshotRun exports the unique *frozen* branch even when a later decision
  // could not freeze. A run-level identity cannot fill in that missing fact.
  run.branch_id = "B-CREATE";
  run.trace.status = "BLOCKED";
  run.trace.first_divergence = "2";
  const missing = run.facts[1];
  missing.verdict = "BLOCKED";
  missing.reason = "second-operation-missing-independent-principal";
  missing.business_expectations = [
    {
      actor: "merchant",
      action_index: 1,
      decision: {
        status: "BLOCKED",
        action: "disable",
        reason: missing.reason,
        matches: [],
        unknown: ["merchant.principal"],
      },
    },
  ];
  run.trace.steps[1].verdict = missing.verdict;
  run.trace.steps[1].decision = missing.reason;
  return readSnapshotText(JSON.stringify(snapshot));
}

describe("unassociated decisions within a partially verified run", () => {
  it.each(["omitted", "empty"] as const)(
    "keeps a typed action fact diagnostic when a before-oracle failure leaves decisions %s",
    (representation) => {
      const snapshot = partiallyAssociatedRun();
      const run = snapshot.runs[0];
      const fact = run.facts![1];
      // This shape was reproduced through execute's real pre-action Oracle 404
      // path and snapshotRun: positive sequence fact, no outbound second action,
      // no frozen decision, while run.branch_id retains the first frozen branch.
      fact.source_kind = "sequence";
      // The native fact stores the aggregate role here; the participant alias
      // remains in participants, even when no actor action has been captured.
      fact.actor = fact.participants![0].role;
      fact.observed_state = "UNKNOWN";
      fact.reason = "before action: HTTP adapter status 404 is outside the declared contract";
      fact.actions = [];
      fact.oracles = [];
      fact.conditions = [];
      fact.business_expectations = representation === "empty" ? [] : undefined;
      run.trace.steps[1].observed = fact.observed_state;
      run.trace.steps[1].decision = fact.reason;
      const parsed = readSnapshotText(JSON.stringify(snapshot));
      const context = selectionContext(parsed);
      expect(context.runs[0].steps[1].branches).toEqual(["unresolved:records"]);
      let state = reduceSelection(
        initialSelection(parsed.meta.graph, parsed.meta.environment),
        { type: "branch", value: "B-CREATE" },
        context,
      );
      state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
      state = reduceSelection(state, { type: "step", value: "2" }, context);
      expect(state).toMatchObject({ branch: "unresolved:records", node: "disable", step: "2" });
      state = reduceSelection(state, { type: "actor", value: "merchant" }, context);
      expect(state.branch).toBe("unresolved:records");
      const html = renderToStaticMarkup(
        <App state={{ snapshot: parsed }} initialContext={state} />,
      );
      const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
      expect(panel).toContain(fact.reason);
      expect(panel).toContain("分支未解析 · 运行诊断");
      expect(panel).not.toContain("预期操作与结果");
    },
  );

  it("retains the recorded run association for old exports without unified step facts", () => {
    const snapshot = partiallyAssociatedRun();
    snapshot.runs[0].facts = undefined;
    const context = selectionContext(readSnapshotText(JSON.stringify(snapshot)));
    expect(context.runs[0].steps.map((step) => step.branches)).toEqual([
      ["B-CREATE"],
      ["B-CREATE"],
    ]);
  });

  it("keeps an explicit missing branch separate from the run's only frozen branch", () => {
    const snapshot = partiallyAssociatedRun();
    const context = selectionContext(snapshot);
    expect(context.runs[0].steps[0].branches).toEqual(["B-CREATE"]);
    expect(context.runs[0].steps[1].branches).toEqual(["unresolved:records"]);
  });

  it("offers the mixed run in both its proven branch and diagnostic scope", () => {
    const snapshot = partiallyAssociatedRun();
    const branches = branchOptions(snapshot);
    expect(branches.some((branch) => branch.unresolved)).toBe(true);
    const context = selectionContext(snapshot, branches);
    const initial = initialSelection(snapshot.meta.graph, snapshot.meta.environment);
    for (const branch of ["B-CREATE", "unresolved:records"]) {
      const selected = reduceSelection(initial, { type: "branch", value: branch }, context);
      expect(matchingRuns(context, selected).map((run) => run.id)).toContain(
        snapshot.runs[0].trace.run,
      );
    }
  });

  it("switches the selected step to diagnostics without showing the prior branch's expected result", () => {
    const snapshot = partiallyAssociatedRun();
    const context = selectionContext(snapshot);
    let state = reduceSelection(
      initialSelection(snapshot.meta.graph, snapshot.meta.environment),
      { type: "branch", value: "B-CREATE" },
      context,
    );
    state = reduceSelection(state, { type: "run", value: snapshot.runs[0].trace.run }, context);
    state = reduceSelection(state, { type: "step", value: "2" }, context);
    expect(state).toMatchObject({
      branch: "unresolved:records",
      run: snapshot.runs[0].trace.run,
      step: "2",
      node: "disable",
    });
    const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={state} />);
    const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
    expect(panel).toContain("second-operation-missing-independent-principal");
    expect(panel).toContain("分支未解析 · 运行诊断");
    expect(panel).not.toContain("预期操作与结果");
  });

  it("keeps a missing actor decision diagnostic beside another actor's known branch in one step", () => {
    const snapshot = partiallyAssociatedRun();
    const run = snapshot.runs[0];
    const known = run.facts![0];
    const missing = structuredClone(run.facts![1].business_expectations![0]);
    missing.actor = "channel";
    missing.action_index = 2;
    known.business_expectations!.push(missing);
    known.participants!.push({
      alias: "channel",
      role: "CHANNEL",
      account: "isolated-channel",
      principal: "isolated-channel-principal",
      sha256: "synthetic",
    });
    run.facts = [known];
    run.trace.steps = [run.trace.steps[0]];
    const parsed = readSnapshotText(JSON.stringify(snapshot));
    const context = selectionContext(parsed);
    expect(new Set(context.runs[0].steps[0].branches)).toEqual(
      new Set(["B-CREATE", "unresolved:records"]),
    );
    let state = reduceSelection(
      initialSelection(parsed.meta.graph, parsed.meta.environment),
      { type: "branch", value: "B-CREATE" },
      context,
    );
    state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
    state = reduceSelection(state, { type: "actor", value: "channel" }, context);
    expect(state).toMatchObject({
      branch: "unresolved:records",
      run: run.trace.run,
      step: "1",
      node: "disable",
      actor: "channel",
    });
    state = reduceSelection(state, { type: "actor", value: "merchant" }, context);
    expect(state).toMatchObject({ branch: "B-CREATE", step: "1", node: "create" });
  });

  it.each(["channel", "CHANNEL"])(
    "does not borrow another actor's frozen branch when %s fails before its own freeze",
    (recordedActor) => {
      const snapshot = partiallyAssociatedRun();
      const run = snapshot.runs[0];
      const fact = run.facts![0];
      // execute reads and freezes each batch member in turn. The second
      // before-read can fail after the first decision, before any batch action.
      fact.actor = recordedActor;
      fact.account = "channel-account";
      fact.principal = "channel-principal";
      fact.node = "disable";
      fact.verdict = "BLOCKED";
      fact.reason = "before action: second batch actor Oracle unavailable";
      fact.actions = [];
      fact.participants!.push({
        alias: "channel",
        role: "CHANNEL",
        account: "channel-account",
        principal: "channel-principal",
        sha256: "synthetic",
      });
      run.facts = [fact];
      run.trace.steps = [
        { ...run.trace.steps[0], node: fact.node, verdict: fact.verdict, decision: fact.reason },
      ];
      const parsed = readSnapshotText(JSON.stringify(snapshot));
      const context = selectionContext(parsed);
      const step = context.runs[0].steps[0];
      expect(step.actors).toContain("channel");
      expect(step.actorBranches?.channel).toEqual(["unresolved:records"]);
      expect(step.branches).toContain("unresolved:records");
      let state = reduceSelection(
        initialSelection(parsed.meta.graph, parsed.meta.environment),
        { type: "branch", value: "B-CREATE" },
        context,
      );
      state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
      state = reduceSelection(state, { type: "actor", value: "channel" }, context);
      expect(state).toMatchObject({
        branch: "unresolved:records",
        run: run.trace.run,
        step: "1",
        actor: "channel",
      });
      const html = renderToStaticMarkup(
        <App state={{ snapshot: parsed }} initialContext={state} />,
      );
      const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
      expect(panel).toContain(fact.reason);
      expect(panel).toContain("分支未解析 · 运行诊断");
      expect(panel).not.toContain("预期操作与结果");
      expect(panel).not.toContain("synthetic unique branch");
    },
  );

  it("keeps an unmatched typed action diagnostic without guessing its actor from conflicting identity fields", () => {
    const snapshot = partiallyAssociatedRun();
    const fact = snapshot.runs[0].facts![0];
    fact.actor = "CHANNEL";
    fact.node = "disable";
    fact.principal = "channel-principal";
    fact.account = "different-account";
    fact.participants!.push({
      alias: "channel",
      role: "CHANNEL",
      account: "channel-account",
      principal: "channel-principal",
      sha256: "synthetic",
    });
    const context = selectionContext(readSnapshotText(JSON.stringify(snapshot)));
    const step = context.runs[0].steps[0];
    expect(step.actors).toEqual(["merchant"]);
    expect(step.branches).toContain("unresolved:records");
  });

  it.each(["create", "disable"])(
    "keeps a partially frozen, unexecuted batch diagnostic when its final action is %s",
    (node) => {
      const snapshot = partiallyAssociatedRun();
      const run = snapshot.runs[0];
      const fact = run.facts![0];
      fact.node = node;
      fact.verdict = "BLOCKED";
      fact.reason = "before action: later batch member Oracle unavailable";
      fact.actions = [];
      const captured = fact.business_expectations![0];
      captured.after = undefined;
      captured.comparison = undefined;
      run.facts = [fact];
      run.trace.steps = [
        { ...run.trace.steps[0], node, verdict: fact.verdict, decision: fact.reason },
      ];
      const parsed = readSnapshotText(JSON.stringify(snapshot));
      const context = selectionContext(parsed);
      expect(context.runs[0].steps[0].branches).toContain("unresolved:records");
      expect(context.runs[0].steps[0].actorBranches?.merchant).toContain("unresolved:records");
      let state = reduceSelection(
        initialSelection(parsed.meta.graph, parsed.meta.environment),
        { type: "branch", value: "B-CREATE" },
        context,
      );
      state = reduceSelection(state, { type: "run", value: run.trace.run }, context);
      expect(state.branch).toBe("unresolved:records");
      state = reduceSelection(state, { type: "actor", value: "merchant" }, context);
      expect(state.branch).toBe("unresolved:records");
      const html = renderToStaticMarkup(
        <App state={{ snapshot: parsed }} initialContext={state} />,
      );
      const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
      expect(panel).toContain(fact.reason);
      expect(panel).toContain("分支未解析 · 运行诊断");
      expect(panel).toContain("执行前冻结的期望");
      expect(panel).toContain(captured.decision.frozen!.sha256);
      expect(panel).toContain("失败主体保持未知");
      expect(panel).not.toContain("预期操作与结果");
    },
  );
});
