import { isJSONObject } from "./json-values";

type Fields = Record<string, unknown>;
const object = isJSONObject;
const list = (value: unknown) => value === null || Array.isArray(value);
const strings = (value: unknown) =>
  value === null || (Array.isArray(value) && value.every((item) => typeof item === "string"));
const whole = (value: unknown) =>
  typeof value === "number" && Number.isSafeInteger(value) && value >= 0;

/** Structural and identity checks only. Proofs and eligibility are supplied by the Go verifier. */
export function checkCampaigns(
  value: unknown,
  graph: string,
  fail: (stage: string, message: string) => never,
) {
  function stop(reason: string): never {
    return fail("snapshot.campaigns", reason);
  }
  if (!Array.isArray(value)) stop("Campaign 列表不是数组");
  const observation = (entry: unknown) => {
    if (!object(entry) || (entry.values !== null && !object(entry.values)))
      stop("Campaign 观察缺少属性对象");
    for (const key of ["bindings", "sources"])
      if (entry[key] !== undefined && !object(entry[key])) stop(`Campaign 观察的 ${key} 不是对象`);
    if (
      object(entry.bindings) &&
      Object.values(entry.bindings).some((item) => typeof item !== "string")
    )
      stop("Campaign 主体绑定不是字符串");
    if (
      object(entry.sources) &&
      Object.values(entry.sources).some(
        (item) => !Array.isArray(item) || item.some((source) => typeof source !== "string"),
      )
    )
      stop("Campaign 属性来源不是字符串列表");
  };
  const request = (entry: unknown) => {
    if (
      !object(entry) ||
      typeof entry.actor !== "string" ||
      !whole(entry.action_index) ||
      typeof entry.node !== "string" ||
      typeof entry.request_sha256 !== "string" ||
      (entry.values !== null && !object(entry.values))
    )
      stop("候选请求捕获缺少动作身份或输入字段");
  };
  const ids = new Set<string>();
  for (const campaign of value as unknown[]) {
    if (
      !object(campaign) ||
      campaign.version !== 1 ||
      campaign.graph !== graph ||
      typeof campaign.id !== "string" ||
      ids.has(campaign.id)
    )
      stop("Campaign 模型/版本/身份不匹配或重复");
    ids.add(campaign.id);
    for (const key of [
      "flow",
      "environment",
      "round",
      "status",
      "reason",
      "evidence_status",
      "history_sha256",
      "created",
      "updated",
    ])
      if (typeof campaign[key] !== "string") stop(`Campaign ${campaign.id} 缺少 ${key}`);
    if (!list(campaign.variants) || !list(campaign.frontier) || !list(campaign.visits))
      stop(`Campaign ${campaign.id} 缺少变体、候选或子运行列表`);
    if (campaign.dispatches !== undefined && !list(campaign.dispatches))
      stop(`Campaign ${campaign.id} 的派发意图不是列表`);
    for (const intent of (campaign.dispatches ?? []) as unknown[])
      if (
        !object(intent) ||
        !whole(intent.sequence) ||
        typeof intent.run !== "string" ||
        typeof intent.sha256 !== "string"
      )
        stop(`Campaign ${campaign.id} 的派发意图缺少身份`);
    if (
      campaign.status === "BLOCKED" &&
      campaign.evidence_status === "BLOCKED" &&
      !(campaign.variants as unknown[] | null)?.length &&
      !(campaign.frontier as unknown[] | null)?.length &&
      !(campaign.visits as unknown[] | null)?.length
    ) {
      // The Go checked loader deliberately withholds untrusted execution details.
      // The UI renders only the trusted row scope and diagnostic reason for this case.
      continue;
    }
    if (!whole(campaign.revision) || !object(campaign.identity) || !object(campaign.request))
      stop(`Campaign ${campaign.id} 缺少冻结身份或请求`);
    for (const key of ["graph", "environment"])
      if (campaign.identity[key] !== campaign[key])
        stop(`Campaign ${campaign.id} 的身份 ${key} 不匹配`);
    for (const key of ["graph", "environment", "flow", "round"])
      if (campaign.request[key] !== campaign[key])
        stop(`Campaign ${campaign.id} 的请求 ${key} 不匹配`);
    if (!Array.isArray(campaign.variants))
      stop(`Campaign ${campaign.id} 缺少变体、候选或子运行列表`);
    if (!object(campaign.budget) || !object(campaign.spent) || !object(campaign.coverage))
      stop(`Campaign ${campaign.id} 缺少预算、消耗或覆盖范围`);
    for (const key of ["max_actions", "max_states", "deadline_ms"])
      if (!whole(campaign.budget[key])) stop(`Campaign ${campaign.id} 的预算 ${key} 非法`);
    for (const key of [
      "actions",
      "reserved_actions",
      "dispatches",
      "target_actions",
      "preparation_actions",
      "recovery_actions",
      "states",
      "observations",
      "elapsed_ms",
    ])
      if (!whole(campaign.spent[key])) stop(`Campaign ${campaign.id} 的消耗 ${key} 非法`);
    for (const key of [
      "declared_branches",
      "bound_branches",
      "selected_variants",
      "attempted_variants",
      "passed_variants",
    ])
      if (!whole(campaign.coverage[key])) stop(`Campaign ${campaign.id} 的覆盖计数 ${key} 非法`);
    for (const key of ["selected_variants_covered", "full_business_coverage"])
      if (typeof campaign.coverage[key] !== "boolean")
        stop(`Campaign ${campaign.id} 缺少覆盖范围标记 ${key}`);
    if (
      !strings(campaign.coverage.unmapped_branches) ||
      (campaign.next_allowed !== undefined && !strings(campaign.next_allowed))
    )
      stop(`Campaign ${campaign.id} 的分支或后续动作列表非法`);
    const variants = new Map<string, Fields>();
    for (const variant of campaign.variants) {
      if (
        !object(variant) ||
        typeof variant.id !== "string" ||
        variants.has(variant.id) ||
        typeof variant.branch !== "string" ||
        typeof variant.runner !== "string" ||
        !whole(variant.visits) ||
        !whole(variant.passes)
      )
        stop(`Campaign ${campaign.id} 的输入变体缺少身份或重复`);
      variants.set(variant.id, variant);
    }
    const checkLink = (entry: Fields) => {
      const variant = variants.get(String(entry.variant_id));
      if (
        !variant ||
        variant.branch !== entry.branch ||
        variant.runner !== entry.runner ||
        typeof entry.run !== "string"
      )
        stop(`Campaign ${String(campaign.id)} 的变体/分支/Runner 关联不一致`);
    };
    const frontier = new Set<string>();
    for (const candidate of (campaign.frontier ?? []) as unknown[]) {
      if (
        !object(candidate) ||
        typeof candidate.variant_id !== "string" ||
        frontier.has(candidate.variant_id) ||
        typeof candidate.eligibility !== "string" ||
        typeof candidate.reason !== "string" ||
        typeof candidate.selected !== "boolean" ||
        typeof candidate.next_allowed !== "boolean"
      )
        stop(`Campaign ${campaign.id} 的候选身份或资格记录非法`);
      frontier.add(candidate.variant_id);
      checkLink(candidate);
      if (candidate.before !== undefined) observation(candidate.before);
      if (candidate.preparation_before !== undefined) observation(candidate.preparation_before);
      if (candidate.input !== undefined) request(candidate.input);
      for (const key of ["observation", "preparation_observation"])
        if (candidate[key] !== undefined) {
          const observed = candidate[key];
          if (!object(observed) || !list(observed.participants)) stop("候选独立观察的主体列表非法");
          for (const field of ["node", "round", "runner_sha256", "code_sha256"])
            if (typeof observed[field] !== "string") stop("候选独立观察缺少模型/运行身份");
          for (const participant of (observed.participants ?? []) as unknown[])
            if (
              !object(participant) ||
              typeof participant.alias !== "string" ||
              typeof participant.account !== "string" ||
              typeof participant.principal !== "string"
            )
              stop("候选独立观察缺少主体身份");
        }
      if (
        candidate.decision !== undefined &&
        (!object(candidate.decision) ||
          typeof candidate.decision.status !== "string" ||
          !strings(candidate.decision.matches))
      )
        stop("候选决策记录非法");
      if (candidate.preparation !== undefined) {
        const plan = candidate.preparation;
        if (
          !object(plan) ||
          typeof plan.status !== "string" ||
          !list(plan.layers) ||
          typeof plan.requires_readback !== "boolean"
        )
          stop("候选准备计划缺少依赖层或回读要求");
        for (const layer of (plan.layers ?? []) as unknown[])
          if (!object(layer) || !list(layer.steps)) stop("候选准备依赖层没有步骤列表");
      }
    }
    const sequences = new Set<number>();
    const runs = new Set<string>();
    for (const visit of (campaign.visits ?? []) as unknown[]) {
      if (
        !object(visit) ||
        !whole(visit.sequence) ||
        sequences.has(Number(visit.sequence)) ||
        typeof visit.run !== "string" ||
        runs.has(visit.run) ||
        typeof visit.status !== "string" ||
        typeof visit.reason !== "string" ||
        typeof visit.dispatch_sha256 !== "string"
      )
        stop(`Campaign ${campaign.id} 的连续子运行身份非法或重复`);
      sequences.add(Number(visit.sequence));
      runs.add(visit.run);
      checkLink(visit);
      if (visit.before !== undefined) observation(visit.before);
      if (visit.after !== undefined) observation(visit.after);
      if (visit.entities !== undefined && !Array.isArray(visit.entities))
        stop("Campaign 子运行的实体不是数组");
      if (
        visit.recovery !== undefined &&
        (!object(visit.recovery) ||
          typeof visit.recovery.verdict !== "string" ||
          typeof visit.recovery.safe !== "boolean")
      )
        stop("Campaign 子运行的恢复记录非法");
    }
    if (campaign.last_actual !== undefined) observation(campaign.last_actual);
  }
}
