import type { BusinessView, Snapshot, Verdict } from "./types";

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
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function fail(stage: string, message: string): never {
  throw new SnapshotError(stage, message);
}

/** Reads the JSON the CLI embedded in `<script type="application/json" id="arc-snapshot">`. */
export function readSnapshotText(text: string | null): Snapshot {
  if (text === null || text.trim() === "") {
    fail(
      "snapshot.missing",
      "页面里没有内嵌快照（#arc-snapshot）；请用 arc-verify snapshot 重新导出",
    );
  }
  let value: unknown;
  try {
    value = JSON.parse(text);
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
  if (!fields(value.model) || !fields(value.model.data) || typeof value.model.sha256 !== "string") {
    fail("snapshot.model", "快照缺少版本化模型");
  }
  if (!fields(value.catalog)) fail("snapshot.catalog", "快照缺少账本目录");
  const views = value.views;
  if (!fields(views)) fail("snapshot.views", "快照缺少视图");
  for (const view of businessViews) checkProjection(view, views[view]);
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
  });
  return value as unknown as Snapshot;
}

function checkProjection(view: BusinessView, value: unknown) {
  if (!fields(value)) fail("snapshot.views", `缺少视图 ${view}`);
  if (value.view !== view) fail("snapshot.views", `视图 ${view} 的内容属于 ${String(value.view)}`);
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
