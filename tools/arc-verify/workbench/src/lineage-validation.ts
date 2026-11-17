import { isJSONObject } from "./json-values";
import { lineagePredicateReferences, sameLineageJSON } from "./lineage-schema";
import { checkLineageReading } from "./lineage-evidence";

type Fields = Record<string, unknown>;
type Failure = (stage: string, message: string) => never;
const kinds = new Set(["target", "state", "condition", "logic", "preparation", "input_data", "expected", "assertion", "invariant"]);
const edgeKinds = new Set(["requires", "logical_operand", "depends_on", "candidate_provider", "reads", "expects", "verifies"]);
const statuses = new Set(["SATISFIED", "UNSATISFIED", "NOT_OBSERVED", "BLOCKED", "READY"]);

/** Validate identity and structure; predicates and evidence are evaluated by the Go exporter. */
export function checkLineage(root: Fields, fail: Failure) {
  const bad = (message: string): never => fail("snapshot.lineage", `业务系谱：${message}`);
  const object = (value: unknown, name: string): Fields => {
    if (!isJSONObject(value)) bad(`${name} 不是对象`);
    return value as Fields;
  };
  const array = (value: unknown, name: string): unknown[] => {
    if (!Array.isArray(value)) bad(`${name} 不是数组`);
    return value as unknown[];
  };
  const text = (value: unknown, name: string): string => {
    if (typeof value !== "string" || !value) bad(`${name} 缺少身份`);
    return value as string;
  };
  const runs = array(root.runs, "运行列表").map((run) => object(run, "运行"));
  if (root.business_lineage === undefined) {
    if (runs.some((run) => Array.isArray(run.business_lineage) && run.business_lineage.length))
      bad("观察没有对应的版本化系谱");
    return;
  }
  const lineage = object(root.business_lineage, "模型系谱");
  if (lineage.version !== 1 && lineage.version !== 2) bad("不支持的系谱版本");
  const hash = text(lineage.contract_sha256, "契约指纹");
  const model = object(object(root.model, "模型").data, "模型数据");
  const contract = object(model.business_contract, "业务契约");
  const definitions = array(contract.branches, "契约分支").map((entry) => object(entry, "分支"));
  const subjects = array(contract.subjects, "业务主体").map((entry) => object(entry, "主体"));
  const branches = new Map<string, Map<string, Fields>>();
  let nodeCount = 0;
  let edgeCount = 0;
  for (const entry of array(lineage.branches, "分支系谱")) {
    const branch = object(entry, "分支系谱");
    const id = text(branch.branch, "分支 ID");
    const definition = definitions.find((item) => item.id === id);
    if (
      !definition ||
      branches.has(id) ||
      branch.action !== definition.action ||
      (branch.flow ?? "") !== (definition.flow ?? "")
    )
      bad(`分支 ${id} 身份不一致或重复`);
    const nodes = new Map<string, Fields>();
    for (const item of array(branch.nodes, `${id} 节点`)) {
      const node = object(item, "节点");
      const nodeID = text(node.id, "节点 ID");
      if (nodes.has(nodeID) || node.branch !== id || !kinds.has(String(node.kind)))
        bad(`分支 ${id} 含重复、未知类型或跨分支节点 ${nodeID}`);
      text(node.name, `节点 ${nodeID} 名称`);
      if (typeof node.source_ref !== "string") bad(`节点 ${nodeID} 缺少来源字段`);
      const expectedReferences =
        node.kind === "target"
          ? lineagePredicateReferences(definition!.when, bad)
          : node.kind === "input_data"
            ? new Set((Array.isArray(node.references) ? node.references : []).map((item) =>
                isJSONObject(item) ? String(item.subject) + "." + String(item.attribute) : "",
              ))
          : ["expected", "assertion", "invariant"].includes(String(node.kind))
            ? new Set<string>()
          : node.predicate !== undefined
            ? lineagePredicateReferences(node.predicate, bad)
            : new Set<string>();
      if (["condition", "logic"].includes(String(node.kind)) && node.predicate === undefined)
        bad(`节点 ${nodeID} 缺少属性级条件`);
      for (const item of node.effects === undefined ? [] : array(node.effects, "声明效果")) {
        const effect = object(item, "声明效果");
        expectedReferences.add(
          `${text(effect.subject, "效果主体")}.${text(effect.attribute, "效果属性")}`,
        );
      }
      if (
        node.from_states !== undefined &&
        array(node.from_states, "前态集合").some((state) => typeof state !== "string")
      )
        bad(`节点 ${nodeID} 前态不完整`);
      const refKeys = new Set<string>();
      for (const ref of node.references === undefined ? [] : array(node.references, "属性引用")) {
        const value = object(ref, "属性引用");
        for (const key of ["subject", "attribute", "subject_kind"]) text(value[key], key);
        const key = `${String(value.subject)}.${String(value.attribute)}`;
        const subject = subjects.find((item) => item.id === value.subject);
        if (
          refKeys.has(key) ||
          !expectedReferences.has(key) ||
          !subject ||
          subject.kind !== value.subject_kind ||
          !Array.isArray(subject.attributes) ||
          !subject.attributes.some(
            (attribute: unknown) => isJSONObject(attribute) && attribute.name === value.attribute,
          )
        )
          bad(`节点 ${nodeID} 的属性引用不属于对应契约主体`);
        refKeys.add(key);
      }
      if (refKeys.size !== expectedReferences.size)
        bad(`节点 ${nodeID} 丢失完整条件所引用的主体属性`);
      if (node.kind === "input_data") {
        if (refKeys.size !== 1) bad(`输入数据节点 ${nodeID} 必须定位单一属性`);
        const attributeKey = [...refKeys][0];
        const expectedID = "data:" + encodeURIComponent(id) + ":" + encodeURIComponent(attributeKey);
        const [owner, field] = attributeKey.split(".");
        const subject = subjects.find((item) => item.id === owner);
        const attribute = (Array.isArray(subject?.attributes) ? subject.attributes : [])
          .find((item: unknown) => isJSONObject(item) && item.name === field) as Fields | undefined;
        if (lineage.version === 2 && (nodeID !== expectedID || !attribute ||
          (node.references as Fields[])[0].source_ref !== attribute.source_ref ||
          node.source_ref !== (attribute.source_ref ?? "")))
          bad(`输入数据节点 ${nodeID} 的来源或身份与声明属性不一致`);
      }
      if (["expected", "assertion", "invariant"].includes(String(node.kind)) && refKeys.size !== 0)
        bad(`执行后验证节点 ${nodeID} 不能伪装为执行前的观测`);
      nodes.set(nodeID, node);
      nodeCount++;
    }
    const target = nodes.get(text(branch.target, "目标节点"));
    if (
      target?.kind !== "target" ||
      target.action !== branch.action ||
      [...nodes.values()].filter((node) => node.kind === "target").length !== 1
    )
      bad(`分支 ${id} 没有唯一对应的目标节点`);
    const edgeIDs = new Set<string>();
    const edges: Fields[] = [];
    for (const item of array(branch.edges, `${id} 关系`)) {
      const edge = object(item, "依赖关系");
      const edgeID = text(edge.id, "关系 ID");
      if (
        edgeIDs.has(edgeID) ||
        !edgeKinds.has(String(edge.kind)) ||
        !nodes.has(String(edge.from)) ||
        !nodes.has(String(edge.to))
      )
        bad(`分支 ${id} 的关系 ${edgeID} 重复或缺少端点`);
      if (typeof edge.source_ref !== "string") bad(`关系 ${edgeID} 缺少来源字段`);
      if (edge.kind === "candidate_provider" && edge.alternative !== true)
        bad(`候选来源 ${edgeID} 被误标为必经依赖`);
      edgeIDs.add(edgeID);
      edges.push(edge);
      edgeCount++;
    }
    const requirements = edges
      .filter((edge) => edge.kind === "requires" && edge.to === branch.target)
      .map((edge) => nodes.get(String(edge.from))!);
    if (
      !requirements.some(
        (node) =>
          node.kind === "state" && sameLineageJSON(node.from_states, definition!.from_states),
      ) ||
      !requirements.some(
        (node) =>
          ["condition", "logic"].includes(String(node.kind)) &&
          sameLineageJSON(node.predicate, definition!.when),
      )
    )
      bad(`分支 ${id} 的系谱遗漏原契约前态或根 Guard`);
    for (const node of nodes.values()) {
      if (node.kind !== "logic") continue;
      const predicate = object(node.predicate, "逻辑条件");
      const children = edges
        .filter((edge) => edge.kind === "logical_operand" && edge.to === node.id)
        .map((edge) => nodes.get(String(edge.from))!.predicate);
      const args = array(predicate.args, "逻辑子项");
      for (const arg of args) {
        const index = children.findIndex((child) => sameLineageJSON(child, arg));
        if (index < 0) bad("系谱丢失逻辑组合的子条件");
        children.splice(index, 1);
      }
      if (children.length) bad("系谱包含不属于当前逻辑组合的子条件");
    }
    if (lineage.version === 2) {
      // Reject partial lineage projections: every leaf operand must have
      // an explicit, typed data origin, and every expected outcome its node.
      for (const node of nodes.values()) {
        if (node.kind !== "condition") continue;
        for (const reference of Array.isArray(node.references) ? node.references : []) {
          const incoming = edges.filter((edge) => edge.kind === "reads" && edge.to === node.id)
            .map((edge) => nodes.get(String(edge.from)));
          if (!incoming.some((origin) => origin?.kind === "input_data" &&
            Array.isArray(origin.references) &&
            origin.references.some((ref: unknown) => isJSONObject(ref) &&
              ref.subject === reference.subject && ref.attribute === reference.attribute)))
            bad(`节点 ${String(node.id)} 丢失属性数据来源`);
        }
      }
      const outcomes = array(object(definition.expected, "预期契约").outcomes, "预期结果");
      const exported = edges.filter((edge) => edge.kind === "expects" && edge.from === branch.target);
      if (exported.length !== outcomes.length ||
        exported.some((edge) => nodes.get(String(edge.to))?.kind !== "expected"))
        bad(`分支 ${id} 没有完整导出预期结果依赖`);
      const identities = new Set(exported.map((edge) => String(edge.to)));
      let requiredChecks = 0;
      const hasVerification = (source: string, target: string, sourceRef: string) =>
        edges.some((edge) => edge.kind === "verifies" && edge.from === source &&
          edge.to === target && edge.source_ref === sourceRef);
      for (const outcome of outcomes) {
        const declared = object(outcome, "预期结果");
        const expectedID = "branch:" + encodeURIComponent(id) + ":outcome:" + encodeURIComponent(text(declared.id, "结果ID"));
        const expected = nodes.get(expectedID);
        if (!identities.has(expectedID) || !expected ||
          !sameLineageJSON(expected.predicate, declared.predicate) ||
          expected.source_ref !== definition!.source_ref)
          bad(`分支 ${id} 缺少或篡改预期结果 ${String(declared.id)}`);
        for (const raw of array(declared.checks ?? [], "结果断言")) {
          const check = object(raw, "结果断言");
          const checkID = expectedID + ":check:" + encodeURIComponent(text(check.id, "断言ID"));
          const node = nodes.get(checkID);
          if (!node || node.kind !== "assertion" ||
              node.source_ref !== check.source_ref ||
              !sameLineageJSON(node.predicate, check.predicate) ||
              !hasVerification(expectedID, checkID, text(check.source_ref, "断言来源")))
            bad(`分支 ${id} 未完整导出结果 Oracle 断言 ${String(check.id)}`);
          requiredChecks++;
        }
      }
      for (const raw of array(object(definition.expected, "预期契约").common_invariants ?? [], "共同不变量")) {
        const check = object(raw, "共同不变量");
        const checkID = "branch:" + encodeURIComponent(id) + ":invariant:" + encodeURIComponent(text(check.id, "不变量ID"));
        const node = nodes.get(checkID);
        if (!node || node.kind !== "invariant" ||
            node.source_ref !== check.source_ref ||
            !sameLineageJSON(node.predicate, check.predicate) ||
            !hasVerification(branch.target as string, checkID, text(check.source_ref, "不变量来源")))
          bad(`分支 ${id} 未完整导出共同不变量 ${String(check.id)}`);
        requiredChecks++;
      }
      if (edges.filter((edge) => edge.kind === "verifies").length !== requiredChecks ||
          [...nodes.values()].filter((node) => ["assertion", "invariant"].includes(String(node.kind))).length !== requiredChecks)
        bad(`分支 ${id} 的执行后验证义务数量不一致`);
      for (const edge of edges) {
        if (edge.kind === "reads" &&
          (nodes.get(String(edge.from))?.kind !== "input_data" ||
           nodes.get(String(edge.to))?.kind !== "condition") ||
          edge.kind === "expects" &&
          (edge.from !== branch.target || nodes.get(String(edge.to))?.kind !== "expected"))
          bad(`分支 ${id} 含不合法的资料或预期结果关系`);
      }
    }
    branches.set(id, nodes);
  }
  if (nodeCount > 65536 || edgeCount > 131072) bad("系谱超过导出预算，拒绝静默截断");
  if (branches.size !== definitions.length) bad("静态系谱没有完整覆盖契约声明的全部分支");
  for (const run of runs) {
    if (run.business_lineage === undefined) continue;
    const runID = text(object(run.trace, "运行轨迹").run, "运行 ID");
    const occurrences = new Set<string>();
    for (const item of array(run.business_lineage, `${runID} 观察`)) {
      const overlay = object(item, "观察作用域");
      const branch = text(overlay.branch, "观察分支");
      const nodes = branches.get(branch);
      const preparation = overlay.phase === "preparation";
      if (
        !nodes ||
        overlay.run !== runID ||
        !Number.isInteger(overlay.step) ||
        (preparation ? overlay.step !== -1 : Number(overlay.step) < 1) ||
        !Number.isInteger(overlay.action_index) ||
        Number(overlay.action_index) < 0 ||
        !["before", "preparation"].includes(String(overlay.phase))
      )
        bad(`运行 ${runID} 的观察跨运行、缺少步骤或动作发生身份`);
      if (
        preparation &&
        (!Number.isInteger(overlay.readback_index) ||
          Number(overlay.readback_index) < 0 ||
          overlay.action_index !== 0 ||
          typeof overlay.readback_phase !== "string")
      )
        bad(`运行 ${runID} 的准备观察缺少回读身份`);
      const actor = text(overlay.actor, "观察主体");
      text(overlay.evidence_status, "证据状态");
      const key = JSON.stringify([
        branch,
        overlay.step,
        actor,
        overlay.action_index,
        overlay.phase,
        overlay.readback_index,
      ]);
      if (occurrences.has(key)) bad(`运行 ${runID} 有重复的观察发生记录`);
      occurrences.add(key);
      const facts = Array.isArray(run.facts) ? run.facts : [];
      const fact = facts.find(
        (candidate) => isJSONObject(candidate) && candidate.seq === overlay.step,
      );
      if (!isJSONObject(fact)) bad(`运行 ${runID} 的观察没有对应步骤事实`);
      let observed: Fields | undefined;
      let request: Fields | undefined;
      if (overlay.evidence_sha256 !== undefined && overlay.evidence_sha256 !== fact.evidence_sha256)
        bad(`运行 ${runID} 的发生记录摘要与步骤事实不一致`);
      if (preparation) {
        if (
          fact.source_kind !== "setup" ||
          !isJSONObject(fact.preparation) ||
          fact.preparation.contract_sha256 !== hash
        )
          bad(`运行 ${runID} 的准备观察没有同版本准备账本`);
        const execution = fact.preparation as Fields;
        const readbacks = array(execution.readbacks, "准备回读").filter(
          (entry) =>
            isJSONObject(entry) &&
            entry.index === overlay.readback_index &&
            entry.phase === overlay.readback_phase,
        );
        const participants = array(fact.participants, "准备主体").filter(
          (entry) => isJSONObject(entry) && entry.alias === actor,
        );
        if (readbacks.length !== 1 || participants.length !== 1)
          bad(`运行 ${runID} 的准备回读序号、阶段或主体没有唯一原始记录`);
        const readback = object(readbacks[0], "准备回读");
        if (isJSONObject(readback.observation)) observed = readback.observation;
        else if (overlay.diagnostic !== true) bad("缺少回读观察的准备记录只能保留阻断诊断");
      } else {
        const expectations: unknown[] = Array.isArray(fact.business_expectations)
          ? fact.business_expectations
          : [];
        const decision = expectations.find(
          (candidate) =>
            isJSONObject(candidate) &&
            candidate.actor === actor &&
            candidate.action_index === overlay.action_index &&
            isJSONObject(candidate.decision) &&
            isJSONObject(candidate.decision.frozen) &&
            candidate.decision.frozen.branch === branch,
        );
        if (
          !isJSONObject(decision) ||
          !isJSONObject(decision.decision) ||
          !isJSONObject(decision.decision.frozen) ||
          decision.decision.frozen.contract_sha256 !== hash
        )
          bad(`运行 ${runID} 的观察与冻结分支或契约版本不一致`);
        const recorded = decision as Fields;
        const frozen = object(object(recorded.decision, "冻结决策").frozen, "冻结期望");
        observed = object(frozen.before, "冻结前态");
        if (isJSONObject(recorded.request)) request = recorded.request;
      }
      const seen = new Set<string>();
      for (const item of array(overlay.nodes, "节点观察")) {
        const reading = object(item, "节点观察");
        const id = text(reading.node, "观察节点");
        if (
          !nodes!.has(id) ||
          seen.has(id) ||
          !statuses.has(String(reading.status)) ||
          (reading.status === "READY" && (preparation || nodes!.get(id)?.kind !== "target"))
        )
          bad(`运行 ${runID} 的观察含未知、重复节点或错误状态`);
        if (overlay.diagnostic === true && reading.status !== "BLOCKED")
          bad(`运行 ${runID} 的诊断观察被误标为可验证就绪`);
        seen.add(id);
        for (const item of reading.values === undefined ? [] : array(reading.values, "观察值")) {
          const value = object(item, "观察值");
          for (const key of ["subject", "attribute", "subject_kind"]) text(value[key], key);
          if (value.entity_ref !== undefined) {
            const entity = object(value.entity_ref, "实体引用");
            for (const key of ["alias", "kind", "id"]) text(entity[key], `实体 ${key}`);
            if (
              entity.id !== value.binding ||
              !Number.isInteger(entity.source_step) ||
              ["owner_actor", "owner_principal", "source_field"].some(
                (key) => typeof entity[key] !== "string",
              ) ||
              (entity.channel_account_id !== undefined &&
                typeof entity.channel_account_id !== "string")
            )
              bad("实体引用缺少完整命名空间、所有权或来源身份");
            if (
              !preparation &&
              !(
                Array.isArray(fact.before_entities) &&
                fact.before_entities.some((original: unknown) => sameLineageJSON(original, entity))
              )
            )
              bad("实体引用不属于该动作记录的前态绑定，不能借用后态实体");
          }
          for (const item of array(value.sources, "观察来源")) {
            const source = object(item, "观察来源");
            text(source.oracle, "Oracle 身份");
            if (
              !["oracle", "request_capture", "preparation_readback"].includes(String(source.kind))
            )
              bad("观察来源种类无法区分独立 Oracle 与请求留档");
            if (
              source.kind === "preparation_readback" &&
              (!preparation ||
                !overlay.evidence_sha256 ||
                source.sha256 !== overlay.evidence_sha256)
            )
              bad("准备回读来源与对应 setup RAW 摘要不一致");
          }
        }
        checkLineageReading(reading, nodes!.get(id)!, observed, fact, overlay, request, bad);
      }
      if (seen.size !== nodes!.size) bad(`运行 ${runID} 的节点观察被截断，不能隐藏未满足条件`);
    }
  }
}
