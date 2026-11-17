import type { BusinessExpectationObservation, UnifiedStepFact } from "./facts";
import type {
  BusinessAssertionResult,
  BusinessComparison,
  BusinessExpectedCheck,
  FrozenBusinessExpectation,
} from "./contract";
import { entityKey } from "./facts";
import { isJSONObject } from "./json-values";
import { sameLineageJSON } from "./lineage-schema";
import type { Snapshot, SnapshotRun } from "./types";

const present = (value: unknown): value is string =>
  typeof value === "string" && value.trim().length > 0;
const actionIndex = (item: { action_index?: number }) => item.action_index ?? 0;

function recordedChecksMatch(
  expected: BusinessExpectedCheck[] = [],
  actual: BusinessAssertionResult[] = [],
): boolean {
  return (
    Array.isArray(expected) &&
    Array.isArray(actual) &&
    expected.length === actual.length &&
    new Set(actual.map((check) => check?.id)).size === actual.length &&
    expected.every(
      (check) =>
        isJSONObject(check) &&
        actual.some(
          (recorded) =>
            recorded?.id === check.id &&
            recorded.kind === check.kind &&
            recorded.oracle === check.oracle &&
            recorded.source_ref === check.source_ref &&
            ["TRUE", "FALSE", "UNKNOWN"].includes(recorded.evaluation?.value),
        ),
    )
  );
}

/** Checks captured verdict completeness; business predicates remain the execution engine's responsibility. */
function passingComparisonDiagnostic(
  frozen: FrozenBusinessExpectation,
  comparison: BusinessComparison,
): string | undefined {
  const expected = frozen.expected;
  if (
    !expected.outcomes.length ||
    comparison.checks.length !== expected.outcomes.length ||
    new Set(comparison.checks.map((check) => check.outcome)).size !== comparison.checks.length ||
    !recordedChecksMatch(expected.common_invariants, comparison.invariant_checks) ||
    (comparison.invariant_checks ?? []).some((check) => check.evaluation.value !== "TRUE")
  )
    return "通过记录缺少完整且成立的冻结结果检查或共同不变量。";
  const passed = new Set<string>();
  for (const check of comparison.checks) {
    const outcome = expected.outcomes.find((item) => item.id === check.outcome);
    if (
      !outcome ||
      !isJSONObject(check.predicate) ||
      !["TRUE", "FALSE", "UNKNOWN"].includes(check.predicate.value) ||
      (outcome.state && typeof check.state_passed !== "boolean") ||
      (outcome.denied !== undefined && typeof check.denied_passed !== "boolean") ||
      !recordedChecksMatch(outcome.checks, check.assertions)
    )
      return "通过记录没有逐项保存冻结结果所要求的状态、响应或独立断言。";
    if (
      check.predicate.value === "TRUE" &&
      (!outcome.state || check.state_passed === true) &&
      (outcome.denied === undefined || check.denied_passed === true) &&
      (check.assertions ?? []).every((assertion) => assertion.evaluation.value === "TRUE")
    )
      passed.add(outcome.id);
  }
  if (!passed.size || (comparison.outcome && !passed.has(comparison.outcome)))
    return "通过标签与已记录的逐项比较矛盾；没有完整成立的冻结允许结果。";
  return undefined;
}

/** Match already revalidated RAW to its display mirror; this does not verify RAW bytes in a browser. */
export function stateFactDiagnostic(
  snapshot: Snapshot,
  run: SnapshotRun,
  fact: UnifiedStepFact,
): string | undefined {
  if (
    fact.run !== run.trace.run ||
    fact.graph !== snapshot.meta.graph ||
    fact.evidence_status !== "PASS" ||
    (fact.evidence_version ?? 0) < 2
  )
    return "步骤身份或独立证据未通过校验，不能叠加实际状态。";
  const entries = run.evidence.filter(
    (entry) => entry.run === run.trace.run && entry.phase === "action" && entry.step === fact.seq,
  );
  const entry = entries[0];
  const raw = entry?.data;
  if (
    entries.length !== 1 ||
    entry.error ||
    !present(entry.sha256) ||
    entry.sha256 !== fact.evidence_sha256 ||
    !raw ||
    raw.run !== fact.run ||
    raw.step !== fact.seq ||
    (raw.phase && raw.phase !== "action") ||
    raw.identity.graph !== snapshot.meta.graph ||
    raw.identity.graph_sha256 !== snapshot.meta.graph_sha256 ||
    snapshot.model.sha256 !== snapshot.meta.graph_sha256 ||
    raw.identity.task !== run.trace.task ||
    raw.identity.environment !== run.environment ||
    !isJSONObject(raw.model)
  )
    return "步骤没有同运行、同模型、同环境和同摘要的 RAW 捕获。";
  const mirrorFields = [
    "participants",
    "oracles",
    "actions",
    "requests",
    "business_expectations",
    "before_entities",
    "entities",
    "extractions",
  ] as const;
  for (const field of mirrorFields) {
    const entries = fact[field] ?? [];
    if (!Array.isArray(entries) || entries.some((entry) => !isJSONObject(entry)))
      return `步骤 ${field} 包含不完整的身份或观察记录。`;
    if (!sameLineageJSON(fact[field] ?? [], raw.model[field] ?? []))
      return `步骤 ${field} 与原始捕获不一致，不能采用被替换的实际状态。`;
  }
  const traces = run.trace.steps.filter((trace) => trace.step === String(fact.seq));
  if (
    traces.length !== 1 ||
    traces[0].sha256 !== entry.sha256 ||
    (traces[0].evidence_status !== undefined && traces[0].evidence_status !== "PASS")
  )
    return "步骤回放与独立证据的摘要或身份不一致。";
  return undefined;
}

export function stateOccurrenceDiagnostic(
  snapshot: Snapshot,
  fact: UnifiedStepFact,
  item: BusinessExpectationObservation,
): string | undefined {
  if (
    (fact.business_expectations ?? []).filter(
      (candidate) => candidate.actor === item.actor && candidate.action_index === item.action_index,
    ).length !== 1
  )
    return "同账号、同动作发生序号没有唯一冻结记录。";
  const frozen = item.decision.frozen;
  if (
    !present(item.actor) ||
    !Number.isInteger(item.action_index) ||
    item.action_index < 0 ||
    item.decision.status !== "READY" ||
    !frozen ||
    !present(frozen.sha256) ||
    !present(frozen.before_sha256) ||
    !present(frozen.contract_sha256) ||
    item.decision.branch !== frozen.branch ||
    item.decision.action !== frozen.action ||
    !sameLineageJSON(item.decision.matches, [frozen.branch])
  )
    return "动作没有唯一且完整的执行前冻结分支。";
  const contract = snapshot.model.data.business_contract;
  const branches = contract?.branches.filter((branch) => branch.id === frozen.branch) ?? [];
  const branch = branches[0];
  const contractHash =
    snapshot.business_lineage?.contract_sha256 ?? snapshot.contract_analysis?.contract_sha256;
  if (
    branches.length !== 1 ||
    !branch.confirmed ||
    !snapshot.model.approved ||
    snapshot.model.rules[branch.rule] !== true ||
    frozen.version !== contract?.version ||
    (contractHash && contractHash !== frozen.contract_sha256) ||
    branch.action !== frozen.action ||
    branch.rule !== frozen.rule ||
    (branch.flow && branch.flow !== frozen.flow) ||
    (frozen.flow ?? "") !== (frozen.before.flow ?? "") ||
    (branch.transition ?? "") !== (frozen.transition ?? "") ||
    !sameLineageJSON(branch.expected, frozen.expected) ||
    !present(frozen.before.state) ||
    !branch.from_states.includes(frozen.before.state)
  )
    return "冻结分支与当前已确认契约、版本或前态不一致。";
  if (!item.after || !present(item.after.state)) return "没有独立读取的实际后态，保留未完成诊断。";
  const comparison = item.comparison;
  if (
    !comparison ||
    comparison.branch !== frozen.branch ||
    !["PASS", "FAIL", "BLOCKED"].includes(comparison.status) ||
    !Array.isArray(comparison.checks) ||
    (comparison.outcome &&
      !frozen.expected.outcomes.some((outcome) => outcome.id === comparison.outcome))
  )
    return "缺少对应冻结分支的实际比较记录。";
  for (const check of comparison.checks) {
    if (!isJSONObject(check)) return "实际比较包含不完整的状态断言。";
    const expected = frozen.expected.outcomes.find((outcome) => outcome.id === check.outcome);
    if (!expected) return "实际比较引用了冻结分支以外的预期结果。";
    const expectedState = expected.state === "UNCHANGED" ? frozen.before.state : expected.state;
    if (
      expectedState &&
      typeof check.state_passed === "boolean" &&
      check.state_passed !== (expectedState === item.after.state)
    )
      return "记录的状态断言与冻结预期和实际后态矛盾，不能据此展示通过。";
  }
  if (comparison.status === "PASS") {
    if ((item.after.flow ?? "") !== (frozen.before.flow ?? ""))
      return "通过记录的实际业务流程与冻结前态不一致，不能跨流程延续身份。";
    for (const subject of contract!.subjects) {
      const beforeID = frozen.before.bindings?.[subject.id] ?? "";
      if (beforeID && item.after.bindings?.[subject.id] !== beforeID)
        return `通过记录未独立确认已绑定主体 ${subject.id} 的身份连续性。`;
    }
    const reason = passingComparisonDiagnostic(frozen, comparison);
    if (reason) return reason;
  }
  const participants = (fact.participants ?? []).filter(
    (participant) => participant.alias === item.actor,
  );
  const actions = (fact.actions ?? []).filter(
    (action) => action.actor === item.actor && actionIndex(action) === item.action_index,
  );
  const requests = (fact.requests ?? []).filter(
    (request) => request.actor === item.actor && request.action_index === item.action_index,
  );
  const participant = participants[0];
  const action = actions[0];
  const request = requests[0];
  if (
    participants.length !== 1 ||
    !present(participant.principal) ||
    !present(participant.account) ||
    actions.length !== 1 ||
    requests.length !== 1 ||
    action.node !== frozen.action ||
    action.principal !== participant.principal ||
    request.node !== frozen.action ||
    action.request_sha256 !== request.request_sha256 ||
    !present(action.request_sha256) ||
    !present(action.response_sha256) ||
    !present(action.started) ||
    !present(action.completed) ||
    (item.request != null && !sameLineageJSON(request, item.request)) ||
    frozen.before.bindings?.[item.actor] !== participant.principal
  )
    return "账号、动作发生序号、请求捕获或执行收据没有唯一对应关系。";
  for (const check of comparison.checks) {
    const expected = frozen.expected.outcomes.find((outcome) => outcome.id === check.outcome)!;
    if (
      expected.denied !== undefined &&
      typeof check.denied_passed === "boolean" &&
      check.denied_passed !== (expected.denied === action.denied)
    )
      return "记录的响应拒绝断言与冻结预期和真实动作收据矛盾。";
  }
  const rules = snapshot.model.data.rules.filter((rule) => rule.id === frozen.rule);
  const primary = (fact.oracles ?? []).filter(
    (oracle) =>
      oracle.actor === item.actor &&
      actionIndex(oracle) === item.action_index &&
      oracle.id === rules[0]?.oracle,
  );
  const oracle = primary[0];
  if (
    rules.length !== 1 ||
    primary.length !== 1 ||
    !present(oracle.boundary) ||
    !present(oracle.before_sha256) ||
    !present(oracle.after_sha256) ||
    oracle.before.principal !== participant.principal ||
    oracle.after.principal !== participant.principal ||
    oracle.before.state !== frozen.before.state ||
    oracle.after.state !== item.after.state
  )
    return "前后态没有同账号、同动作发生序号的主 Oracle 独立观察，或其状态与记录冲突。";
  return undefined;
}

/** Present the action verdict conservatively within its recorded step verdict. */
export function stateOccurrenceVerdict(
  fact: UnifiedStepFact,
  item: BusinessExpectationObservation,
): { verdict: "PASS" | "FAIL" | "BLOCKED"; reason: string } {
  const comparison = item.comparison!;
  let verdict = comparison.status as "PASS" | "FAIL" | "BLOCKED";
  let reason = comparison.reason;
  if (fact.verdict === "FAIL" && verdict !== "FAIL") {
    verdict = fact.business_expectations!.length === 1 ? "FAIL" : "BLOCKED";
    reason +=
      verdict === "FAIL"
        ? "；同一步存在独立断言失败。"
        : "；同一步失败尚未归属到此并发动作，不能记为通过。";
  } else if (fact.verdict !== "PASS" && verdict === "PASS") {
    verdict = "BLOCKED";
    reason += `；步骤记载为 ${fact.verdict}，完整执行义务尚未成立，不能记为通过。`;
  }
  return { verdict, reason };
}

/** Full captured identity is required; an after rebinding cannot rewrite the before entity. */
export function stateOccurrenceEntityKeys(
  run: SnapshotRun,
  fact: UnifiedStepFact,
  item: BusinessExpectationObservation,
): string[] {
  const frozen = item.decision.frozen;
  if (!frozen) return [];
  const before = fact.before_entities ?? [];
  const bindings = frozen.before.bindings ?? {};
  const references = (run.business_lineage ?? [])
    .filter(
      (overlay) =>
        overlay.phase === "before" &&
        overlay.run === run.trace.run &&
        overlay.step === fact.seq &&
        overlay.actor === item.actor &&
        overlay.action_index === item.action_index &&
        overlay.branch === frozen.branch &&
        overlay.evidence_status === "PASS" &&
        !overlay.diagnostic,
    )
    .flatMap((overlay) => overlay.nodes.flatMap((node) => node.values ?? []));
  const keys = before
    .filter((entity) => {
      if (
        !entity.alias ||
        !entity.kind ||
        !entity.id ||
        !entity.owner_actor ||
        !entity.owner_principal
      )
        return false;
      if (before.filter((candidate) => candidate.alias === entity.alias).length !== 1) return false;
      const scoped = references.filter((value) => value.entity_ref);
      return (
        scoped.some(
          (value) =>
            sameLineageJSON(value.entity_ref, entity) &&
            value.binding === entity.id &&
            bindings[value.subject] === entity.id,
        ) ||
        (!scoped.some((value) => value.subject === entity.alias) &&
          bindings[entity.alias] === entity.id)
      );
    })
    .map(entityKey);
  for (const entity of fact.entities ?? []) {
    if (before.some((previous) => previous.alias === entity.alias)) continue;
    const captures = (fact.extractions ?? []).filter(
      (capture) =>
        capture.source_actor === item.actor &&
        actionIndex(capture) === item.action_index &&
        capture.step === fact.seq &&
        capture.entity_alias === entity.alias &&
        capture.entity_kind === entity.kind &&
        capture.value === entity.id &&
        capture.owner_actor === entity.owner_actor &&
        capture.owner_principal === entity.owner_principal &&
        (capture.channel_account_id ?? "") === (entity.channel_account_id ?? "") &&
        present(capture.sha256),
    );
    if (captures.length === 1) keys.push(entityKey(entity));
  }
  return [...new Set(keys)].sort();
}
