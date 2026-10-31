import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import App from "./App";
import { SnapshotError, parseSnapshot } from "./snapshot";

const snapshot = parseSnapshot(JSON.parse(JSON.stringify(sample)));

describe("static workbench", () => {
  it("makes every workspace and view reachable from the snapshot", () => {
    const html = renderToStaticMarkup(<App state={{ snapshot }} />);
    for (const label of [
      "业务建模",
      "业务验证",
      "业务分析",
      "系统业务",
      "业务能力",
      "业务边界",
      "业务流程",
      "测试验证",
      "业务影响",
    ]) {
      expect(html).toContain(label);
    }
    expect(html).toContain(snapshot.meta.graph);
    expect(html).toContain(snapshot.meta.generated);
  });
  it("offers no write action", () => {
    const html = [
      ...(
        ["overview", "capabilities", "boundaries", "flows", "verification", "impact"] as const
      ).map((view) => renderToStaticMarkup(<App state={{ snapshot }} initialView={view} />)),
      renderToStaticMarkup(
        <App state={{ error: new SnapshotError("snapshot.parse", "broken") }} />,
      ),
    ].join("");
    const buttons = [...html.matchAll(/<button[^>]*>(.*?)<\/button>/g)].map((match) =>
      match[1].replace(/<[^>]+>/g, ""),
    );
    expect(buttons.length).toBeGreaterThan(0);
    for (const label of buttons) {
      expect(label).not.toMatch(/导入|批准|规划|执行|重试|删除|保存|提交/);
    }
    for (const markup of ["<form", "<textarea", 'type="submit"']) {
      expect(html).not.toContain(markup);
    }
  });
  it("shows evidence that failed re-verification as a failure, never as valid", () => {
    const tampered = parseSnapshot(JSON.parse(JSON.stringify(sample)));
    tampered.runs[0].evidence = tampered.runs[0].evidence.map((entry) =>
      entry.phase === "fixture"
        ? { run: entry.run, phase: entry.phase, step: entry.step, error: "RAW sha256 mismatch" }
        : entry,
    );
    const html = renderToStaticMarkup(<App state={{ snapshot: tampered }} />);
    expect(html).not.toContain("RAW sha256 mismatch");
    const verification = renderToStaticMarkup(
      <App state={{ snapshot: tampered }} initialView="verification" />,
    );
    expect(verification).toContain("夹具恢复：导出时校验失败");
    expect(verification).toContain("RAW sha256 mismatch");
  });
  it("shows sourced diagnostics with their subjects", () => {
    const diagnosed = parseSnapshot(JSON.parse(JSON.stringify(sample)));
    diagnosed.views.boundaries.diagnostics = [
      {
        code: "OWNERSHIP_CONFLICT",
        severity: "error",
        subjects: ["enrollment.submit", "course.boundary", "audit.boundary"],
        reason:
          "multiple boundaries own the same business subject without a sourced sharing declaration",
      },
      {
        code: "DEPENDENCY_CYCLE",
        severity: "warning",
        subjects: ["enrollment.submit", "enrollment.review"],
        reason: "business dependencies are cyclic",
      },
    ];
    const html = renderToStaticMarkup(
      <App state={{ snapshot: diagnosed }} initialView="boundaries" />,
    );
    for (const text of [
      "OWNERSHIP_CONFLICT",
      "severity-error",
      "audit.boundary",
      "DEPENDENCY_CYCLE",
      "severity-warning",
      "business dependencies are cyclic",
    ]) {
      expect(html).toContain(text);
    }
  });
  it("shows a contract change with its propagation reasons and tests to rerun", () => {
    const changed = parseSnapshot(JSON.parse(JSON.stringify(sample)));
    changed.impact = {
      graph: "course-v2",
      previous: "course-v1",
      changed: ["enrollment.submit"],
      affected: ["R1"],
      tests: ["R1/POSITIVE"],
      reasons: { R1: ["implements enrollment.submit"] },
      scope: "typed dependency propagation",
    };
    const html = renderToStaticMarkup(<App state={{ snapshot: changed }} initialView="impact" />);
    for (const text of [
      "course-v1",
      "enrollment.submit",
      "R1/POSITIVE",
      "implements enrollment.submit",
      "待复验测试",
    ]) {
      expect(html).toContain(text);
    }
    const unchanged = renderToStaticMarkup(<App state={{ snapshot }} initialView="impact" />);
    expect(unchanged).toContain("没有可比较的差异");
  });
  it("shows ledger exploration coverage without greening a capability by explore count", () => {
    const explored = parseSnapshot(JSON.parse(JSON.stringify(sample)));
    explored.exploration = {
      "enrollment.submit": {
        explored_branches: ["repeat:enroll"],
        unexplored_branches: ["skip_precondition:enroll"],
        unspecified: ["UNDECLARED_TRANSITION:force"],
        confirmed_fails: ["enrollment.submit historic FAIL"],
        current_pass: [],
        tested: true,
        pending_recovery: ["X-blocked"],
        stop_reasons: ["budget_actions"],
        budget_exhausted: true,
        evidence_missing: "oracle timeout",
      },
    };
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot: explored }}
        initialView="capabilities"
        initialSelected="enrollment.submit"
      />,
    );
    for (const text of [
      "探索与验证",
      "已测试",
      "尚未完整验证",
      "repeat:enroll",
      "skip_precondition:enroll",
      "UNDECLARED_TRANSITION:force",
      "enrollment.submit historic FAIL",
      "X-blocked",
      "预算耗尽",
      "oracle timeout",
      "当前有效 PASS",
      "尚未完整验证",
      "结果不稳定",
      "AI 边界候选",
      "已执行路径",
      "未执行 Frontier",
    ]) {
      expect(html).toContain(text);
    }
  });
  it("shows the failing stage instead of sample data when the snapshot is broken", () => {
    const html = renderToStaticMarkup(
      <App
        state={{ error: new SnapshotError("snapshot.parse", "Unexpected end of JSON input") }}
      />,
    );
    expect(html).toContain("snapshot.parse");
    expect(html).toContain("Unexpected end of JSON input");
    expect(html).not.toContain(snapshot.meta.graph);
  });
});
