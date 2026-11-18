import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import App from "./App";
import { SnapshotError, parseSnapshot } from "./snapshot";
import { contractFixture } from "./test-fixtures";

describe("offline business verification workbench", () => {
  it("shows four coordinated areas and requires a branch before showing any run facts", () => {
    const snapshot = contractFixture();
    const html = renderToStaticMarkup(<App state={{ snapshot }} />);
    for (const label of [
      "能力树与分支导航",
      "准备依赖",
      "业务状态",
      "运行轨迹",
      "分支与事实面板",
      "执行轨迹",
      "先选择条件与预期",
      "先选择业务分支",
    ])
      expect(html).toContain(label);
    expect(html).toContain(snapshot.meta.graph);
    expect(html).toContain("静态快照");
    expect(html).not.toContain("principal-A");
    expect(html).not.toContain("principal-B-private-marker");
    expect(html).toMatch(/aria-label="运行"[^>]*disabled/);
  });
  it("opens a full-width ETA plan by default and leaves the evidence workbench accessible", () => {
    const html = renderToStaticMarkup(<App state={{ snapshot: contractFixture() }} initialView="boundaries" />);
    expect(html).toContain('class="workspace eta-plan-workspace"');
    expect(html).toContain('aria-label="返回包含事实面板的四区工作台"');
    expect(html).toContain("显示事实面板");
    expect(html).toContain('aria-label="画布视图"');
  });

  it("shows declared predicates and expectations before there is an execution", () => {
    const html = renderToStaticMarkup(
      <App state={{ snapshot: contractFixture() }} initialContext={{ branch: "B-CALLBACK" }} />,
    );
    expect(html).toContain("merchant.enabled");
    expect(html).toContain("order.inflight");
    expect(html).toContain("order.settlements");
    expect(html).toContain("确定性单一结果");
    expect(html).toContain("尚未选择运行");
    expect(html).not.toContain("principal-A");
  });
  it("keeps flow-independent branches selectable in the same flow scope as the capability rail", () => {
    const snapshot = contractFixture();
    delete snapshot.model.data.business_contract!.branches[2].flow;
    const html = renderToStaticMarkup(
      <App state={{ snapshot }} initialContext={{ flow: "F-PAY" }} />,
    );
    const options = html.match(/<select aria-label="业务分支"[^>]*>([\s\S]*?)<\/select>/)?.[1];
    expect(options).toBeDefined();
    expect(options).toContain('value="B-CALLBACK"');
    expect(html).toContain("<code>B-CALLBACK</code>");
  });
  it("renders the selected step's frozen expectation, actual result, subject and database evidence without another run's identity", () => {
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot: contractFixture() }}
        initialContext={{ branch: "B-CALLBACK", run: "synthetic-run-A", step: "3" }}
      />,
    );
    for (const text of [
      "principal-A",
      "order-principal-A",
      "merchant-enabled",
      "false",
      "独立判据与 DB 回读",
      "会话状态",
      "失效",
      "异步等待",
      "5000",
      "冻结前置条件与实际结果",
      "isolated-settlement-count",
      "synthetic invariant divergence retained",
      "首次偏离",
      "B-CALLBACK",
    ])
      expect(html).toContain(text);
    expect(html).not.toContain("principal-B-private-marker");
    expect(html).not.toContain("order-principal-B-private-marker");
  });
  it("keeps failed evidence separate from a recorded business result and preserves errors", () => {
    const snapshot = contractFixture();
    snapshot.runs[0].evidence_status = "BLOCKED";
    snapshot.runs[0].facts![0].evidence_status = "BLOCKED";
    snapshot.runs[0].evidence_errors = ["synthetic hash mismatch"];
    snapshot.runs[0].evidence = [
      { run: "synthetic-run-A", phase: "action", step: 1, error: "synthetic hash mismatch" },
    ];
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{ branch: "B-CREATE", run: "synthetic-run-A", step: "1" }}
      />,
    );
    expect(html).toContain("不能据此点绿业务");
    expect(html).toContain("导出时校验失败");
    expect(html).toContain("synthetic hash mismatch");
  });
  it("keeps legacy traces reviewable without claiming the new business contract", () => {
    const snapshot = parseSnapshot(structuredClone(sample));
    const run = snapshot.runs[0];
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{ branch: "legacy:R1/POSITIVE", run: run.trace.run }}
      />,
    );
    expect(html).toContain("旧格式测试可回看");
    expect(html).toContain("未导出统一 step_facts");
    expect(html).toContain("历史格式或身份不完整");
    expect(html).toContain(run.trace.run);
  });
  it("preserves bounded coverage gaps and sourced model diagnostics", () => {
    const snapshot = contractFixture();
    snapshot.views.boundaries.diagnostics = [
      {
        code: "DEPENDENCY_CYCLE",
        severity: "error",
        subjects: ["P-A", "P-B"],
        reason: "synthetic preparation cycle",
      },
    ];
    const html = renderToStaticMarkup(<App state={{ snapshot }} />);
    for (const text of [
      "UNSPECIFIED",
      "synthetic missing branch retained",
      "覆盖不完整",
      "DEPENDENCY_CYCLE",
      "severity-error",
      "P-A",
      "synthetic preparation cycle",
    ])
      expect(html).toContain(text);
  });
  it("exposes no target write control and does not render fallback business data on a broken snapshot", () => {
    const html = renderToStaticMarkup(<App state={{ snapshot: contractFixture() }} />);
    expect(html).not.toContain("<form");
    expect(html).not.toContain("<textarea");
    expect(html).not.toContain('type="submit"');
    const failure = renderToStaticMarkup(
      <App state={{ error: new SnapshotError("snapshot.facts", "run identity mismatch") }} />,
    );
    expect(failure).toContain("run identity mismatch");
    expect(failure).toContain("snapshot.facts");
    expect(failure).not.toContain("synthetic-run-A");
  });
});
