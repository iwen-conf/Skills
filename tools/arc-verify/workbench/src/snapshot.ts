import type { BusinessView, Snapshot, Verdict } from "./types";
import { isJSONObject, parseSnapshotJSON } from "./json-values";
import { checkCampaigns } from "./campaign-validation";

export const businessViews: BusinessView[] = [
  "overview",
  "capabilities",
  "boundaries",
  "flows",
  "verification",
  "impact",
];
const verdicts = new Set<string>([
  "PASS",
  "PARTIAL",
  "FAIL",
  "NOT_RUN",
  "BLOCKED",
  "STALE",
  "FLAKY",
  "EXCLUDED",
] satisfies Verdict[]);

/** A snapshot that cannot be shown; `stage` names where reading stopped. */
export class SnapshotError extends Error {
  readonly stage: string;
  constructor(stage: string, message: string) {
    super(message);
    this.stage = stage;
  }
}

type Fields = Record<string, unknown>;

function fields(value: unknown): value is Fields {
  return isJSONObject(value);
}

function fail(stage: string, message: string): never {
  throw new SnapshotError(stage, message);
}

/** Reads the JSON the CLI embedded in `<script type="application/json" id="arc-snapshot">`. */
export function readSnapshotText(text: string | null): Snapshot {
  if (text === null || text.trim() === "") {
    fail(
      "snapshot.missing",
      "页面里没有内嵌快照（#arc-snapshot）；请用 arc verify snapshot 重新导出",
    );
  }
  let value: unknown;
  try {
    value = parseSnapshotJSON(text);
  } catch (error) {
    fail("snapshot.parse", error instanceof Error ? error.message : String(error));
  }
  return parseSnapshot(value);
}

/** Checks the parts the page relies on; a damaged snapshot is refused, never repaired. */
export function parseSnapshot(value: unknown): Snapshot {
  if (!fields(value)) fail("snapshot.root", "快照不是 JSON 对象");
  const meta = value.meta;
  if (
    !fields(meta) ||
    typeof meta.generated !== "string" ||
    typeof meta.graph !== "string" ||
    typeof meta.graph_sha256 !== "string" ||
    typeof meta.version !== "string" ||
    !Array.isArray(meta.argv)
  ) {
    fail("snapshot.meta", "快照缺少生成时间、图版本、hash、版本或导出命令");
  }
  const graphID = meta.graph;
  if (!fields(value.model) || !fields(value.model.data) || typeof value.model.sha256 !== "string") {
    fail("snapshot.model", "快照缺少版本化模型");
  }
  // Compare exported identities only. Content hashes and RAW authenticity are
  // checked by the CLI; the static reader does not re-verify those proofs.
  if (value.model.data.id !== meta.graph || value.model.sha256 !== meta.graph_sha256)
    fail("snapshot.model", "模型声明的身份或指纹与快照元数据不一致；拒绝混用历史事实");
  if (
    !Array.isArray(value.model.data.nodes) ||
    !Array.isArray(value.model.data.edges) ||
    !Array.isArray(value.model.data.rules)
  )
    fail("snapshot.model", "模型缺少节点、关系或规则");
  if (value.model.data.business_contract !== undefined)
    checkContract(value.model.data.business_contract);
  if (!fields(value.catalog)) fail("snapshot.catalog", "快照缺少账本目录");
  const views = value.views;
  if (!fields(views)) fail("snapshot.views", "快照缺少视图");
  for (const view of businessViews)
    checkProjection(view, views[view], meta.graph, meta.graph_sha256);
  if (!Array.isArray(value.runs)) fail("snapshot.runs", "快照的运行列表不是数组");
  value.runs.forEach((run, index) => {
    if (
      !fields(run) ||
      !fields(run.trace) ||
      typeof run.trace.run !== "string" ||
      !Array.isArray(run.trace.steps) ||
      !Array.isArray(run.evidence)
    ) {
      fail("snapshot.runs", `第 ${index + 1} 个运行缺少回放或证据`);
    }
    checkRunEvidence(run.evidence, run.trace.run, run.trace.task, graphID);
    if (run.facts !== undefined) {
      if (!Array.isArray(run.facts)) fail("snapshot.facts", `运行 ${run.trace.run} 的事实不是数组`);
      const seen = new Set<number>();
      for (const fact of run.facts) {
        if (
          !fields(fact) ||
          fact.run !== run.trace.run ||
          fact.graph !== meta.graph ||
          !Number.isInteger(fact.seq) ||
          Number(fact.seq) < -1 ||
          (fact.seq === -1 && fact.source_kind !== "setup")
        )
          fail(
            "snapshot.facts",
            `运行 ${run.trace.run} 包含身份不匹配的事实；拒绝跨运行或跨模型展示`,
          );
        if (seen.has(Number(fact.seq)))
          fail("snapshot.facts", `运行 ${run.trace.run} 的步骤 ${String(fact.seq)} 有重复事实`);
        seen.add(Number(fact.seq));
        for (const key of [
          "participants",
          "entities",
          "sessions",
          "before_entities",
          "before_sessions",
          "extractions",
          "conditions",
          "waits",
          "oracles",
          "actions",
          "requests",
          "assertions",
          "business_expectations",
        ]) {
          if (fact[key] !== undefined && !Array.isArray(fact[key]))
            fail("snapshot.facts", `步骤 ${String(fact.seq)} 的 ${key} 不是数组`);
        }
        for (const item of (fact.business_expectations ?? []) as unknown[]) {
          if (
            !fields(item) ||
            !fields(item.decision) ||
            typeof item.decision.status !== "string" ||
            !Array.isArray(item.decision.matches)
          )
            fail("snapshot.facts", `步骤 ${String(fact.seq)} 缺少确定性决策身份`);
          if (item.request !== undefined) checkRequest(item.request);
          if (item.after !== undefined) checkObservation(item.after);
          if (item.decision.frozen !== undefined) {
            if (
              !fields(item.decision.frozen) ||
              !fields(item.decision.frozen.before) ||
              !fields(item.decision.frozen.before.values) ||
              typeof item.decision.frozen.branch !== "string"
            )
              fail("snapshot.facts", `步骤 ${String(fact.seq)} 的冻结期望不完整`);
            checkExpectation(item.decision.frozen.expected, "snapshot.facts");
            checkObservation(item.decision.frozen.before);
          }
        }
        for (const oracle of (fact.oracles ?? []) as unknown[]) {
          if (!fields(oracle)) fail("snapshot.facts", "独立判据不是对象");
          for (const key of ["before", "after"]) {
            checkObservation(oracle[key]);
            const reading = oracle[key] as Fields;
            if (reading.conditions !== null && !fields(reading.conditions))
              fail("snapshot.facts", "独立判据的条件不是字段对象");
          }
        }
        for (const action of (fact.actions ?? []) as unknown[]) {
          if (
            !fields(action) ||
            (action.response_values !== undefined && !fields(action.response_values))
          )
            fail("snapshot.facts", `步骤 ${String(fact.seq)} 的动作响应不是字段对象`);
        }
        for (const request of (fact.requests ?? []) as unknown[]) checkRequest(request);
        if (fact.preparation !== undefined) checkPreparation(fact.preparation);
      }
    }
  });
  if (value.business_campaigns !== undefined)
    checkCampaigns(value.business_campaigns, meta.graph, fail);
  return value as unknown as Snapshot;
}

/** Check occurrence identities only; RAW bytes and digests remain the CLI's proof boundary. */
function checkRunEvidence(entries: unknown[], run: string, task: unknown, graph: string) {
  const occurrences = new Set<string>();
  for (const entry of entries) {
    if (
      !fields(entry) ||
      entry.run !== run ||
      typeof entry.phase !== "string" ||
      !["action", "setup", "fixture"].includes(entry.phase) ||
      typeof entry.step !== "number" ||
      !Number.isInteger(entry.step) ||
      (entry.phase === "action" ? entry.step < 1 : entry.step !== 0)
    )
      fail("snapshot.evidence", `运行 ${run} 的 RAW 摘要身份不完整或属于其他运行`);
    const occurrence = `${entry.phase}/${entry.step}`;
    if (occurrences.has(occurrence))
      fail("snapshot.evidence", `运行 ${run} 的 RAW 摘要 ${occurrence} 重复`);
    occurrences.add(occurrence);
    if (entry.data !== undefined) {
      const raw = entry.data;
      const expectedStep = entry.phase === "setup" ? -1 : entry.step;
      if (
        !fields(raw) ||
        raw.run !== run ||
        raw.step !== expectedStep ||
        (raw.phase === undefined || raw.phase === "" ? "action" : raw.phase) !== entry.phase ||
        !fields(raw.identity) ||
        (raw.identity.graph !== undefined &&
          raw.identity.graph !== "" &&
          raw.identity.graph !== graph) ||
        (typeof task === "string" &&
          raw.identity.task !== undefined &&
          raw.identity.task !== "" &&
          raw.identity.task !== task)
      )
        fail("snapshot.evidence", `运行 ${run} 的 RAW 内容身份与 ${occurrence} 摘要不一致`);
    } else if (typeof entry.error !== "string" || !entry.error.trim()) {
      fail("snapshot.evidence", `运行 ${run} 的 RAW 摘要 ${occurrence} 缺少数据或读取失败原因`);
    }
  }
}

function checkRequest(value: unknown) {
  if (
    !fields(value) ||
    typeof value.actor !== "string" ||
    !Number.isInteger(value.action_index) ||
    typeof value.node !== "string" ||
    typeof value.request_sha256 !== "string" ||
    (value.values !== null && !fields(value.values))
  )
    fail("snapshot.facts", "请求捕获缺少动作身份或输入字段对象");
}

function checkObservation(value: unknown) {
  if (!fields(value) || (value.values !== null && !fields(value.values)))
    fail("snapshot.facts", "业务观察缺少属性字段对象");
  if (
    value.bindings !== undefined &&
    (!fields(value.bindings) ||
      Object.values(value.bindings).some((item) => typeof item !== "string"))
  )
    fail("snapshot.facts", "业务观察的主体绑定非法");
  if (
    value.sources !== undefined &&
    (!fields(value.sources) ||
      Object.values(value.sources).some(
        (item) => !Array.isArray(item) || item.some((source) => typeof source !== "string"),
      ))
  )
    fail("snapshot.facts", "业务观察的属性来源非法");
}

function checkPreparation(value: unknown) {
  if (
    !fields(value) ||
    typeof value.status !== "string" ||
    !Array.isArray(value.plans) ||
    !Array.isArray(value.steps) ||
    !Array.isArray(value.readbacks)
  )
    fail("snapshot.facts", "准备记录缺少计划、执行步骤或独立回读");
  checkObservation(value.initial_observed);
  checkObservation(value.final_observed);
  for (const plan of value.plans) {
    if (!fields(plan) || !Array.isArray(plan.layers)) fail("snapshot.facts", "准备计划缺少依赖层");
    for (const layer of plan.layers) {
      if (!fields(layer) || !Array.isArray(layer.steps))
        fail("snapshot.facts", "准备依赖层缺少步骤");
      for (const step of layer.steps)
        if (!fields(step) || !Array.isArray(step.predicted_effects))
          fail("snapshot.facts", "准备计划缺少预计效果");
    }
  }
  for (const step of value.steps) {
    if (
      !fields(step) ||
      !fields(step.planned) ||
      !fields(step.receipt) ||
      !fields(step.receipt.action) ||
      !fields(step.requirements) ||
      !fields(step.ensures)
    )
      fail("snapshot.facts", "实际准备步骤缺少动作捕获或前后判据");
    checkObservation(step.before);
    if (step.after !== undefined) checkObservation(step.after);
  }
  for (const readback of value.readbacks) {
    if (!fields(readback) || typeof readback.phase !== "string")
      fail("snapshot.facts", "准备回读缺少阶段身份");
    if (readback.observation !== undefined) checkObservation(readback.observation);
  }
}

function checkPredicate(value: unknown, depth = 0) {
  if (!fields(value) || typeof value.op !== "string" || depth > 128)
    fail("snapshot.contract", "契约谓词结构非法或嵌套过深");
  if (value.args !== undefined) {
    if (!Array.isArray(value.args)) fail("snapshot.contract", "谓词 args 不是数组");
    value.args.forEach((arg) => checkPredicate(arg, depth + 1));
  }
}
function checkExpectation(value: unknown, stage: string) {
  if (
    !fields(value) ||
    !["deterministic", "allowed_outcomes"].includes(String(value.mode)) ||
    !Array.isArray(value.outcomes)
  )
    fail(stage, "缺少声明的预期结果集合");
  for (const outcome of value.outcomes) {
    if (!fields(outcome) || typeof outcome.id !== "string") fail(stage, "预期结果缺少身份");
    checkPredicate(outcome.predicate);
  }
}
function checkContract(value: unknown) {
  if (
    !fields(value) ||
    value.version !== 1 ||
    !Array.isArray(value.subjects) ||
    !Array.isArray(value.branches)
  )
    fail("snapshot.contract", "业务契约缺少版本、主体或分支");
  const ids = new Set<string>();
  for (const branch of value.branches) {
    if (
      !fields(branch) ||
      typeof branch.id !== "string" ||
      typeof branch.action !== "string" ||
      !Array.isArray(branch.from_states) ||
      branch.from_states.length === 0 ||
      branch.from_states.some((state) => typeof state !== "string" || state === "") ||
      ids.has(branch.id)
    )
      fail("snapshot.contract", "业务分支身份或前态非法，或分支身份重复");
    ids.add(branch.id);
    checkPredicate(branch.when);
    checkExpectation(branch.expected, "snapshot.contract");
  }
  if (value.preparation !== undefined && !Array.isArray(value.preparation))
    fail("snapshot.contract", "准备依赖不是数组");
}

function checkProjection(view: BusinessView, value: unknown, graph: string, graphSHA: string) {
  if (!fields(value)) fail("snapshot.views", `缺少视图 ${view}`);
  if (value.view !== view) fail("snapshot.views", `视图 ${view} 的内容属于 ${String(value.view)}`);
  // A model-only export has a zero-valued ProofContext until a round is selected.
  // Accept absent proof identity, but never a conflicting declared identity.
  if (
    value.graph !== graph ||
    !fields(value.context) ||
    typeof value.context.graph !== "string" ||
    typeof value.context.graph_sha256 !== "string" ||
    (value.context.graph !== "" && value.context.graph !== graph) ||
    (value.context.graph_sha256 !== "" && value.context.graph_sha256 !== graphSHA)
  )
    fail("snapshot.views", `视图 ${view} 声明的模型身份或指纹与快照元数据不一致`);
  if (value.round && (!value.context.graph || !value.context.graph_sha256))
    fail("snapshot.views", `视图 ${view} 声明了验证轮次，但缺少该轮次的模型身份`);
  if (!verdicts.has(String(value.verdict)))
    fail("snapshot.views", `视图 ${view} 的结论 ${String(value.verdict)} 不是合法状态`);
  if (
    !Array.isArray(value.nodes) ||
    !Array.isArray(value.edges) ||
    !Array.isArray(value.obligations) ||
    !Array.isArray(value.diagnostics)
  ) {
    fail("snapshot.views", `视图 ${view} 缺少节点、关系、义务或诊断`);
  }
  const coverage = value.coverage;
  if (!fields(coverage)) fail("snapshot.views", `视图 ${view} 缺少覆盖统计`);
  for (const name of ["obligations", "capabilities", "behaviors", "branches", "invariants"]) {
    const measure = coverage[name];
    if (
      !fields(measure) ||
      typeof measure.total !== "number" ||
      typeof measure.passed !== "number"
    ) {
      fail("snapshot.views", `视图 ${view} 的覆盖统计 ${name} 不完整`);
    }
  }
  const seen = new Set<string>();
  for (const node of value.nodes) {
    if (!fields(node) || typeof node.id !== "string" || !verdicts.has(String(node.status))) {
      fail("snapshot.views", `视图 ${view} 有缺少身份或状态非法的节点`);
    }
    if (seen.has(node.id)) fail("snapshot.views", `视图 ${view} 的节点 ${node.id} 重复`);
    seen.add(node.id);
  }
  for (const obligation of value.obligations) {
    if (
      !fields(obligation) ||
      !fields(obligation.definition) ||
      !verdicts.has(String(obligation.status))
    ) {
      fail("snapshot.views", `视图 ${view} 有状态非法的义务`);
    }
  }
}
