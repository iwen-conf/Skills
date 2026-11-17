import { isJSONObject } from "./json-values";
import { sameLineageJSON } from "./lineage-schema";

type Fields = Record<string, unknown>;

/** Check that a derived display retains its scoped facts; do not re-evaluate a business rule. */
export function checkLineageReading(
  reading: Fields,
  node: Fields,
  observed: Fields | undefined,
  fact: Fields,
  overlay: Fields,
  request: Fields | undefined,
  bad: (message: string) => never,
) {
  if (reading.status === "BLOCKED") return;
  const truth = isJSONObject(reading.evaluation) ? reading.evaluation.value : undefined;
  if (
    (["SATISFIED", "READY"].includes(String(reading.status)) && truth !== "TRUE") ||
    (reading.status === "UNSATISFIED" && truth !== "FALSE")
  )
    bad(`节点 ${String(node.id)} 的状态与记录的谓词求值矛盾`);
  if (!observed || !isJSONObject(observed.values)) bad("可验证观察没有对应的实际前态");
  const canonical = observed as Fields;
  const actuals = canonical.values as Fields;
  const bindings = isJSONObject(canonical.bindings) ? canonical.bindings : {};
  const origins = isJSONObject(canonical.sources) ? canonical.sources : {};
  if (reading.observed_state !== undefined && reading.observed_state !== canonical.state)
    bad("派生前态与同作用域的实际状态不一致");
  const references = Array.isArray(node.references) ? (node.references as Fields[]) : [];
  const values = Array.isArray(reading.values) ? (reading.values as Fields[]) : [];
  if (references.length !== values.length) bad("派生观察丢失属性引用的实际记录");
  const seen = new Set<string>();
  for (const value of values) {
    const key = `${String(value.subject)}.${String(value.attribute)}`;
    const reference = references.find(
      (ref) => ref.subject === value.subject && ref.attribute === value.attribute,
    );
    if (!reference || seen.has(key) || reference.subject_kind !== value.subject_kind)
      bad("派生观察的主体、属性或种类与静态系谱不一致");
    seen.add(key);
    if ((value.binding ?? "") !== (bindings[String(value.subject)] ?? ""))
      bad(`属性 ${key} 的主体绑定被删除或替换`);
    const present = Object.hasOwn(actuals, key);
    if (
      present !== Object.hasOwn(value, "actual") ||
      (present && !sameLineageJSON(actuals[key], value.actual))
    )
      bad(`属性 ${key} 的实际值与原始前态不一致`);
    const sources = value.sources as Fields[];
    const ids = Array.isArray(origins[key]) ? (origins[key] as string[]) : [];
    if (!present) {
      if (sources.length) bad(`未观察的属性 ${key} 不能凭空拥有来源`);
      continue;
    }
    if (
      !ids.length ||
      sources.length !== ids.length ||
      new Set(sources.map((source) => source.oracle)).size !== ids.length
    )
      bad(`属性 ${key} 的观察来源被删除、重复或补写`);
    for (const source of sources) {
      if (!ids.includes(String(source.oracle))) bad(`属性 ${key} 含其他作用域的 Oracle`);
      if (overlay.phase === "preparation") {
        if (source.kind !== "preparation_readback" || source.sha256 !== overlay.evidence_sha256)
          bad("准备观察的来源不能冒充目标动作 Oracle");
      } else if (source.oracle === "request-capture") {
        if (
          source.kind !== "request_capture" ||
          !request ||
          request.actor !== overlay.actor ||
          request.action_index !== overlay.action_index ||
          source.sha256 !== request.request_sha256 ||
          !isJSONObject(request.values) ||
          !sameLineageJSON(request.values[key], value.actual)
        )
          bad(`属性 ${key} 的请求来源与当前动作留档不一致`);
      } else {
        const matches = (Array.isArray(fact.oracles) ? (fact.oracles as Fields[]) : []).filter(
          (oracle) =>
            oracle.actor === overlay.actor &&
            (oracle.action_index ?? 0) === overlay.action_index &&
            oracle.id === source.oracle,
        );
        const oracle = matches[0];
        if (
          matches.length !== 1 ||
          source.kind !== "oracle" ||
          source.sha256 !== oracle.before_sha256 ||
          source.boundary !== oracle.boundary ||
          !isJSONObject(oracle.before) ||
          !isJSONObject(oracle.before.values) ||
          !sameLineageJSON(oracle.before.values[key], value.actual)
        )
          bad(`属性 ${key} 没有同主体、同动作和同摘要的独立回读`);
      }
    }
  }
}
