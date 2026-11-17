import type { ContractBranch } from "./contract";
import { entityKey } from "./facts";
import type { UnifiedStepFact } from "./facts";
import type { SelectionContext, SelectionRun, SelectionStep } from "./selection";
import type { Snapshot, SnapshotRun, TestDefinition } from "./types";

export interface BranchOption {
  id: string;
  label: string;
  action: string;
  flow: string;
  contract?: ContractBranch;
  legacy?: TestDefinition;
  unresolved?: boolean;
}
const unique = (items: (string | undefined)[]) => [
  ...new Set(items.filter((value): value is string => Boolean(value))),
];

export function branchOptions(snapshot: Snapshot): BranchOption[] {
  const names = new Map(snapshot.model.data.nodes.map((node) => [node.id, node.name]));
  const branches: BranchOption[] = (snapshot.model.data.business_contract?.branches ?? []).map(
    (branch) => ({
      id: branch.id,
      label: `${names.get(branch.action) ?? branch.action} · ${branch.id}`,
      action: branch.action,
      flow: branch.flow ?? "",
      contract: branch,
    }),
  );
  // Old test obligations remain inspectable in their own namespace. They are not new contract branches.
  const tests = new Map((snapshot.model.data.tests ?? []).map((test) => [test.id, test]));
  for (const obligation of snapshot.views.verification.obligations)
    tests.set(obligation.definition.id, obligation.definition);
  for (const test of tests.values()) {
    const action =
      snapshot.model.data.rules.find((rule) => rule.id === test.rule)?.action ??
      test.steps?.[0] ??
      "";
    branches.push({
      id: `legacy:${test.id}`,
      label: `旧测试 · ${test.name || test.id}`,
      action,
      flow: test.flow ?? "",
      legacy: test,
    });
  }
  const ids = new Set(branches.map((branch) => branch.id));
  if (
    snapshot.runs.some((run) => {
      const linked = [run.branch_id, ...factsOf(snapshot, run).flatMap(factBranches)].filter(
        Boolean,
      );
      return (
        !linked.some((id) => ids.has(id!)) &&
        !branches.some(
          (branch) =>
            branch.legacy &&
            (run.trace.task === branch.legacy.id ||
              run.trace.task.endsWith(`/${branch.legacy.id}`)),
        )
      );
    })
  )
    branches.push({
      id: "unresolved:records",
      label: "未关联分支的运行 · 诊断",
      action: "",
      flow: "",
      unresolved: true,
    });
  return branches;
}

export function factsOf(snapshot: Snapshot, run: SnapshotRun): UnifiedStepFact[] {
  return (run.facts ?? [])
    .filter((fact) => fact.run === run.trace.run && fact.graph === snapshot.meta.graph)
    .sort((a, b) => a.seq - b.seq);
}
export function factBranches(fact?: UnifiedStepFact): string[] {
  return unique(
    (fact?.business_expectations ?? []).flatMap((item) => [
      item.decision.frozen?.branch,
      item.decision.branch,
    ]),
  );
}
export function actorsOf(fact?: UnifiedStepFact): string[] {
  return unique([
    fact?.actor,
    ...(fact?.actions ?? []).map((action) =>
      typeof action.actor === "string" ? action.actor : undefined,
    ),
    ...(fact?.business_expectations ?? []).map((item) => item.actor),
    ...(fact?.oracles ?? []).map((item) => item.actor),
    ...(fact?.requests ?? []).map((item) => item.actor),
  ]);
}
export function runSteps(snapshot: Snapshot, run: SnapshotRun): SelectionStep[] {
  const facts = factsOf(snapshot, run);
  const seqs = unique([
    ...run.trace.steps.map((step) => step.step),
    ...facts.filter((fact) => fact.seq > 0).map((fact) => String(fact.seq)),
  ]).sort((a, b) => Number(a) - Number(b));
  return seqs.map((id) => {
    const fact = facts.find((item) => String(item.seq) === id);
    const step = run.trace.steps.find((item) => item.step === id);
    const actors = actorsOf(fact);
    return {
      id,
      node: fact?.node ?? step?.node ?? "",
      actors,
      actorNodes: Object.fromEntries(
        actors.map((actor) => [
          actor,
          unique([
            ...(actor === fact?.actor ? [fact?.node] : []),
            ...(fact?.requests ?? [])
              .filter((item) => item.actor === actor)
              .map((item) => item.node),
            ...(fact?.actions ?? [])
              .filter((item) => item.actor === actor)
              .map((item) => item.node),
            ...(fact?.business_expectations ?? [])
              .filter((item) => item.actor === actor)
              .map((item) => item.decision.action),
          ]),
        ]),
      ),
      actorBranches: Object.fromEntries(
        actors.map((actor) => [
          actor,
          unique(
            (fact?.business_expectations ?? [])
              .filter((item) => item.actor === actor)
              .flatMap((item) => [item.decision.frozen?.branch, item.decision.branch]),
          ),
        ]),
      ),
      entities: (fact?.entities ?? []).map(entityKey),
      branches: factBranches(fact).length
        ? factBranches(fact)
        : run.branch_id
          ? [run.branch_id]
          : [],
    };
  });
}

export function selectionContext(
  snapshot: Snapshot,
  branches = branchOptions(snapshot),
): SelectionContext {
  const runs: SelectionRun[] = snapshot.runs.map((run) => {
    const catalog = snapshot.catalog.runs.find((item) => item.id === run.trace.run);
    const round = snapshot.catalog.rounds.find((item) => item.id === run.trace.round);
    const steps = runSteps(snapshot, run);
    const explicit = unique([run.branch_id, ...steps.flatMap((step) => step.branches ?? [])]);
    const legacy = explicit.length
      ? undefined
      : branches.find(
          (branch) =>
            branch.legacy &&
            (run.trace.task === branch.legacy.id ||
              run.trace.task.endsWith(`/${branch.legacy.id}`)),
        );
    const known = explicit.filter((id) => branches.some((branch) => branch.id === id));
    const attached = known.length ? known : legacy ? [legacy.id] : ["unresolved:records"];
    return {
      id: run.trace.run,
      branch: attached.length === 1 ? attached[0] : "",
      branches: attached,
      flow: run.flow_id ?? (legacy?.flow || ""),
      environment: run.environment ?? catalog?.environment ?? round?.environment ?? "",
      steps: steps.map((step) => ({
        ...step,
        branches: step.branches?.length
          ? step.branches
          : legacy
            ? [legacy.id]
            : ["unresolved:records"],
      })),
    };
  });
  return {
    model: snapshot.meta.graph,
    environments: unique([snapshot.meta.environment, ...runs.map((run) => run.environment)]),
    branches: branches.map((branch) => ({
      id: branch.id,
      flow: branch.flow,
      action: branch.action,
    })),
    runs,
  };
}
