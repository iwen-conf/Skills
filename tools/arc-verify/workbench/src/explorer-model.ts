import type { ContractBranch } from "./contract";
import { entityKey, factEntities } from "./facts";
import type { BusinessExpectationObservation, UnifiedStepFact } from "./facts";
import type { Selection, SelectionContext, SelectionRun, SelectionStep } from "./selection";
import type { Projection, Snapshot, SnapshotRun, TestDefinition } from "./types";

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
const unresolvedBranch = "unresolved:records";

/** An omitted branch flow is unrestricted, as in the kernel's branch matcher. */
export const branchAppliesToFlow = (branch: { flow?: string }, flow: string) =>
  !flow || !branch.flow || branch.flow === flow;

export function executionCoverageScope(snapshot: Snapshot, selection: Selection) {
  const round = snapshot.contract_execution?.round ?? "";
  const environment =
    snapshot.catalog.rounds.find((item) => item.id === round && item.graph === snapshot.meta.graph)
      ?.environment ??
    snapshot.meta.environment ??
    "";
  const run = snapshot.runs.find((item) => item.trace.run === selection.run);
  const campaign = snapshot.business_campaigns?.find((item) => item.id === selection.campaign);
  const reasons: string[] = [];
  if (!round) reasons.push("分支覆盖未绑定已验证轮次");
  if (selection.environment && selection.environment !== environment)
    reasons.push("所选环境不属于此覆盖汇总");
  if (run && run.trace.round !== round) reasons.push("所选运行不属于此覆盖轮次");
  if (campaign && campaign.round !== round) reasons.push("所选 Campaign 不属于此覆盖轮次");
  if (selection.variant) reasons.push("轮次分支汇总不能作为所选输入变体的判定");
  return {
    round,
    environment,
    label: `环境 ${environment || "未声明"} / 轮次 ${round || "未指定"}`,
    appliesToSelection: reasons.length === 0,
    reason: reasons.join("；"),
  };
}

/** Node aggregates belong to the exported view, not whichever run is currently selected. */
export function projectionScope(snapshot: Snapshot, selection: Selection, projection: Projection) {
  const environment = projection.environment || projection.context.environment;
  const round = projection.round ?? "";
  const run = snapshot.runs.find((item) => item.trace.run === selection.run);
  const campaign = snapshot.business_campaigns?.find((item) => item.id === selection.campaign);
  const reasons: string[] = [];
  if (!round) reasons.push("节点汇总未绑定验证轮次");
  if (selection.environment && selection.environment !== environment)
    reasons.push("所选环境不同于汇总环境");
  if (run && run.trace.round !== round) reasons.push("所选运行不同于汇总轮次");
  if (campaign && campaign.round !== round) reasons.push("所选 Campaign 不同于汇总轮次");
  if (selection.variant) reasons.push("节点汇总不能作为输入变体的判定");
  return {
    label: `环境 ${environment || "未声明"} / 轮次 ${round || "未指定"}`,
    appliesToSelection: reasons.length === 0,
    reason: reasons.join("；"),
  };
}

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
  if (
    snapshot.runs.some((run) =>
      runAssociations(snapshot, run, branches).attached.includes(unresolvedBranch),
    )
  )
    branches.push({
      id: unresolvedBranch,
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
/** An explicit decision without a branch is a fact, not a missing run default. */
function decisionBranches(records: BusinessExpectationObservation[] = []): string[] {
  return unique(
    records.flatMap((item) => {
      const ids = unique([item.decision.frozen?.branch, item.decision.branch]);
      return ids.length ? ids : [unresolvedBranch];
    }),
  );
}
function recordedActorAlias(fact?: UnifiedStepFact): string | undefined {
  const participants = fact?.participants ?? [];
  const consistent = (participant: (typeof participants)[number]) =>
    (!fact?.principal || participant.principal === fact.principal) &&
    (!fact?.account || participant.account === fact.account);
  const aliases = participants.filter((participant) => participant.alias === fact?.actor);
  if (aliases.length)
    return aliases.length === 1 && consistent(aliases[0]) ? aliases[0].alias : undefined;
  if (!fact?.principal && !fact?.account) return undefined;
  const identities = participants.filter(consistent);
  return identities.length === 1 ? identities[0].alias : undefined;
}
/** A captured freeze without outbound actions cannot identify every aborted batch member. */
export function hasUnexecutedFreeze(fact?: UnifiedStepFact): boolean {
  return Boolean(
    fact &&
    ["BLOCKED", "UNKNOWN"].includes(fact.verdict) &&
    !fact.actions?.length &&
    fact.business_expectations?.some((item) => item.decision.frozen && !item.comparison),
  );
}
export function actorsOf(fact?: UnifiedStepFact): string[] {
  const observed = unique([
    ...(fact?.actions ?? []).map((action) =>
      typeof action.actor === "string" ? action.actor : undefined,
    ),
    ...(fact?.business_expectations ?? []).map((item) => item.actor),
    ...(fact?.oracles ?? []).map((item) => item.actor),
    ...(fact?.requests ?? []).map((item) => item.actor),
    recordedActorAlias(fact),
  ]);
  if (observed.length) return observed;
  const participants = unique((fact?.participants ?? []).map((item) => item.alias));
  return participants.length ? participants : unique([fact?.actor]);
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
    const recordedBranches = decisionBranches(fact?.business_expectations);
    // A typed action can fail its independent before-read before freeze runs.
    // Its persisted fact is explicit absence, unlike an old export with no facts.
    const typedFact = Boolean(snapshot.model.data.business_contract && fact && fact.seq > 0);
    const incompleteBatch = typedFact && hasUnexecutedFreeze(fact);
    const missingTypedDecision =
      typedFact &&
      (incompleteBatch ||
        !recordedBranches.length ||
        (fact?.node &&
          !fact.business_expectations?.some((item) => item.decision.action === fact.node)));
    const actorBranches = Object.fromEntries(
      actors.map((actor) => {
        const branches = decisionBranches(
          (fact?.business_expectations ?? []).filter((item) => item.actor === actor),
        );
        return [
          actor,
          typedFact && (!branches.length || incompleteBatch)
            ? unique([...branches, unresolvedBranch])
            : branches,
        ];
      }),
    );
    const stepBranches = unique([
      ...recordedBranches,
      ...Object.values(actorBranches).flat(),
      ...(missingTypedDecision ? [unresolvedBranch] : []),
    ]);
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
      actorBranches,
      entities: factEntities(fact).map(entityKey),
      branches: stepBranches.length ? stepBranches : run.branch_id ? [run.branch_id] : [],
    };
  });
}

function runAssociations(snapshot: Snapshot, run: SnapshotRun, branches: BranchOption[]) {
  const rawSteps = runSteps(snapshot, run);
  const explicit = unique([run.branch_id, ...rawSteps.flatMap((step) => step.branches ?? [])]);
  const legacy = explicit.length
    ? undefined
    : branches.find(
        (branch) =>
          branch.legacy &&
          (run.trace.task === branch.legacy.id || run.trace.task.endsWith(`/${branch.legacy.id}`)),
      );
  const known = new Set(branches.map((branch) => branch.id));
  const identify = (ids: string[]) =>
    unique(ids.map((id) => (known.has(id) ? id : unresolvedBranch)));
  const steps = rawSteps.map((step) => ({
    ...step,
    branches: step.branches?.length ? identify(step.branches) : [legacy?.id ?? unresolvedBranch],
    actorBranches: Object.fromEntries(
      Object.entries(step.actorBranches ?? {}).map(([actor, ids]) => [actor, identify(ids)]),
    ),
  }));
  const attached = unique([...identify(explicit), ...steps.flatMap((step) => step.branches)]);
  return { steps, legacy, attached: attached.length ? attached : [legacy?.id ?? unresolvedBranch] };
}

export function selectionContext(
  snapshot: Snapshot,
  branches = branchOptions(snapshot),
): SelectionContext {
  const runs: SelectionRun[] = snapshot.runs.map((run) => {
    const catalog = snapshot.catalog.runs.find((item) => item.id === run.trace.run);
    const round = snapshot.catalog.rounds.find((item) => item.id === run.trace.round);
    const { steps, legacy, attached } = runAssociations(snapshot, run, branches);
    return {
      id: run.trace.run,
      branch: attached.length === 1 ? attached[0] : "",
      branches: attached,
      flow: run.flow_id ?? (legacy?.flow || ""),
      environment: run.environment ?? catalog?.environment ?? round?.environment ?? "",
      round: run.trace.round,
      runner: run.runner_id,
      steps,
    };
  });
  return {
    model: snapshot.meta.graph,
    environments: unique([
      snapshot.meta.environment,
      ...runs.map((run) => run.environment),
      ...(snapshot.business_campaigns ?? []).map((campaign) => campaign.environment),
    ]),
    branches: branches.map((branch) => ({
      id: branch.id,
      flow: branch.flow,
      action: branch.action,
    })),
    runs,
    campaigns: (snapshot.business_campaigns ?? []).map((campaign) => ({
      id: campaign.id,
      environment: campaign.environment,
      flow: campaign.flow,
      round: campaign.round,
      variants: campaign.variants ?? [],
      visits: campaign.visits ?? [],
    })),
  };
}
