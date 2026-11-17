import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { campaignFixture } from "./campaign-fixture";
import { executionCoverageScope, selectionContext } from "./explorer-model";
import { initialSelection, matchingRuns, reduceSelection } from "./selection";
import type { SelectionAction } from "./selection";
import { readSnapshotText, parseSnapshot } from "./snapshot";

function selectionFixture() {
  const snapshot = campaignFixture();
  const context = selectionContext(snapshot);
  const reduce = (state: ReturnType<typeof initialSelection>, action: SelectionAction) =>
    reduceSelection(state, action, context);
  const selected = reduce(initialSelection(snapshot.meta.graph, snapshot.meta.environment), {
    type: "campaign",
    value: "synthetic-campaign-A",
  });
  return { snapshot, context, reduce, selected };
}

describe("bounded campaign selection and evidence", () => {
  it("keeps pre-action dispatch intents out of completed child-run navigation", () => {
    const snapshot = campaignFixture();
    snapshot.business_campaigns![0].dispatches = [
      { sequence: 3, run: "uncompleted-intent-only", sha256: "synthetic-intent-sha" },
    ];
    const context = selectionContext(parseSnapshot(snapshot));
    const state = reduceSelection(
      initialSelection(snapshot.meta.graph),
      { type: "campaign", value: "synthetic-campaign-A" },
      context,
    );
    expect(
      reduceSelection(state, { type: "visit", value: "uncompleted-intent-only" }, context),
    ).toEqual(state);
    const html = renderToStaticMarkup(
      <App state={{ snapshot }} initialContext={{ campaign: state.campaign }} />,
    );
    expect(html).toContain("派发意图不代表执行完成");
    expect(html).not.toContain("定位子运行 uncompleted-intent-only");
  });
  it("labels target-after readings separately from restore boundaries and current probes", () => {
    const snapshot = campaignFixture();
    snapshot.business_campaigns![0].visits![0].recovery = {
      mode: "restore",
      verdict: "PASS",
      reason: "synthetic restore",
      state: "start",
      safe: true,
      actions: [],
    };
    const html = renderToStaticMarkup(
      <App state={{ snapshot }} initialContext={{ campaign: "synthetic-campaign-A" }} />,
    );
    for (const text of [
      "最近目标动作的实际后态",
      "当前状态仍以后续独立 probe 为准",
      "恢复边界",
      "处理后观察 start",
      "不推断同一实体连续",
    ])
      expect(html).toContain(text);
    expect(html).not.toContain("Campaign 最近记录的实际状态");
  });
  it("keeps distinct input variants on one branch and requires branch/variant before a run", () => {
    const { context, reduce, selected } = selectionFixture();
    expect(selected).toMatchObject({ branch: "", run: "", campaign: "synthetic-campaign-A" });
    expect(reduce(selected, { type: "run", value: "synthetic-run-A" })).toEqual(selected);
    const variant = reduce(selected, { type: "variant", value: "V-100" });
    expect(variant).toMatchObject({ branch: "B-CREATE", variant: "V-100", run: "" });
    expect(matchingRuns(context, variant).map((run) => run.id)).toEqual(["synthetic-run-A"]);
    expect(reduce(variant, { type: "run", value: "synthetic-run-B" })).toEqual(variant);
    expect(reduce(variant, { type: "run", value: "synthetic-reserved-0" })).toEqual(variant);
  });
  it("follows an explicit campaign child link and clears account/entity context", () => {
    const { context, reduce, selected } = selectionFixture();
    let state = reduce(selected, { type: "visit", value: "synthetic-run-A" });
    state = reduce(state, { type: "actor", value: "merchant" });
    state = reduce(state, { type: "entity", value: context.runs[0].steps[0].entities[0] });
    expect(state.actor).toBe("merchant");
    state = reduce(state, { type: "visit", value: "synthetic-run-B" });
    expect(state).toMatchObject({
      campaign: "synthetic-campaign-A",
      variant: "V-200",
      branch: "B-CREATE",
      run: "synthetic-run-B",
      actor: "",
      entity: "",
      step: "1",
    });
    expect(reduce(state, { type: "environment", value: "other-ui-environment" })).toMatchObject({
      campaign: "",
      variant: "",
      run: "",
      actor: "",
      entity: "",
    });
  });
  it("refuses a campaign link whose exported child has another runner or round", () => {
    const { context, selected } = selectionFixture();
    for (const patch of [{ runner: "other-runner" }, { round: "other-round" }]) {
      const damaged = structuredClone(context);
      Object.assign(damaged.runs[0], patch);
      const result = reduceSelection(
        selected,
        { type: "visit", value: "synthetic-run-A" },
        damaged,
      );
      expect(result.run).toBe("");
    }
  });
  it("validates scope, variant links, duplicate visits and numeric budgets without producing eligibility", () => {
    const original = campaignFixture();
    expect(parseSnapshot(original)).toBe(original);
    const mutations = [
      (snapshot: typeof original) => {
        snapshot.business_campaigns![0].graph = "other-model";
      },
      (snapshot: typeof original) => {
        snapshot.business_campaigns![0].request.environment = "other-environment";
      },
      (snapshot: typeof original) => {
        snapshot.business_campaigns![0].frontier![0].runner = "wrong-variant-runner";
      },
      (snapshot: typeof original) => {
        snapshot.business_campaigns![0].visits![1].sequence = 1;
      },
      (snapshot: typeof original) => {
        snapshot.business_campaigns![0].budget.max_actions = -1;
      },
    ];
    for (const mutate of mutations) {
      const snapshot = structuredClone(original);
      mutate(snapshot);
      expect(() => parseSnapshot(snapshot)).toThrow();
    }
    expect(original.business_campaigns![0].frontier![0].eligibility).toBe("READY");
  });
  it("keeps completed selected variants separate from full business coverage and neutral stop status", () => {
    const snapshot = campaignFixture();
    const campaign = snapshot.business_campaigns![0];
    campaign.status = "COMPLETED_IN_SCOPE";
    campaign.coverage.selected_variants_covered = true;
    const html = renderToStaticMarkup(
      <App state={{ snapshot }} initialContext={{ campaign: campaign.id }} />,
    );
    for (const text of [
      "COMPLETED_IN_SCOPE",
      "已完成所选范围",
      "全业务覆盖标记",
      "false",
      "已选变体覆盖不升级为全业务证明",
      "动作尝试",
      "预留",
      "未绑定输入变体的分支",
      "B-DISABLE",
      "Campaign → 连续子运行",
      "next_allowed",
    ])
      expect(html).toContain(text);
    expect(html).toMatch(/class="campaign-stop">执行状态 <strong>COMPLETED_IN_SCOPE<\/strong>/);
    expect(html).not.toContain("other-campaign-private-marker");
  });
  it("retains a checked BLOCKED ledger stub without blanking other campaigns or rendering fake zero statistics", () => {
    const snapshot = campaignFixture();
    const stub = structuredClone(snapshot.business_campaigns![0]);
    stub.id = "synthetic-damaged-ledger";
    stub.status = "BLOCKED";
    stub.evidence_status = "BLOCKED";
    stub.reason = "synthetic campaign body hash mismatch";
    stub.variants = null;
    stub.frontier = null;
    stub.visits = null;
    stub.identity.graph = "";
    stub.identity.environment = "";
    stub.request.graph = "";
    stub.request.environment = "";
    snapshot.business_campaigns!.push(stub);
    const parsed = parseSnapshot(snapshot);
    const html = renderToStaticMarkup(
      <App state={{ snapshot: parsed }} initialContext={{ campaign: stub.id }} />,
    );
    expect(html).toContain("synthetic campaign body hash mismatch");
    expect(html).toContain("预算消耗和覆盖结论均不可用");
    expect(html).toContain("不能据此判断是否曾执行过动作");
    expect(html).not.toContain("动作尝试");
    expect(html).not.toContain("已完成所选范围");
    expect(parsed.business_campaigns).toHaveLength(3);
    const healthy = renderToStaticMarkup(
      <App state={{ snapshot: parsed }} initialContext={{ campaign: "synthetic-campaign-A" }} />,
    );
    expect(healthy).toContain("动作尝试");
  });
  it("does not apply round-level branch coverage to a selected variant or another campaign round", () => {
    const { snapshot, reduce, selected } = selectionFixture();
    snapshot.contract_execution = {
      status: "PASS",
      round: snapshot.meta.round,
      total: 1,
      passed: 1,
      failed: 0,
      blocked: 0,
      not_run: 0,
      branches: [],
      reason: "synthetic aggregate only",
    };
    const variant = reduce(selected, { type: "variant", value: "V-200" });
    expect(executionCoverageScope(snapshot, variant).reason).toContain("所选输入变体");
    snapshot.business_campaigns![0].round = "older-campaign-round";
    expect(executionCoverageScope(snapshot, selected).reason).toContain(
      "Campaign 不属于此覆盖轮次",
    );
  });
  it("shows actual candidate input and preparation as a plan, never as an executed reserved run", () => {
    const snapshot = campaignFixture();
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{ campaign: "synthetic-campaign-A", variant: "V-CALLBACK" }}
      />,
    );
    for (const text of [
      "候选回读的实际前态",
      "候选的实际请求输入",
      "candidate-private-marker",
      "需要准备与回读",
      "synthetic dependency path",
      "requires_readback=true",
      "准备规划前的独立观察",
      "order.exists",
      "synthetic-preparation-db",
      "预计 order.inflight",
      "不证明已执行",
    ])
      expect(html).toContain(text);
    expect(html).not.toContain("所选子运行目标动作的实际后态");
    expect(html).toContain("NOT_RUN 不等于规则通过");
  });
  it("shows only the selected child's actual values in the detail panel and retains other visits as labeled history", () => {
    const snapshot = campaignFixture();
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{
          campaign: "synthetic-campaign-A",
          variant: "V-100",
          run: "synthetic-run-A",
        }}
      />,
    );
    for (const text of [
      "所选子运行目标动作的实际前态",
      "所选子运行目标动作的实际后态",
      "synthetic-create-100",
      "principal-A",
      "checkpoint",
      "不表示恢复到了初态",
      "synthetic first divergence retained",
      "synthetic-run-B",
    ])
      expect(html).toContain(text);
    for (const hidden of [
      "candidate-private-marker",
      "latest-not-selected-run",
      "principal-B-private-marker",
      "other-campaign-private-marker",
    ])
      expect(html).not.toContain(hidden);
  });
  it("shows missing child exports and null Go slices without inferring an execution", () => {
    const snapshot = campaignFixture();
    snapshot.runs = [];
    let html = renderToStaticMarkup(
      <App
        state={{ snapshot: parseSnapshot(snapshot) }}
        initialContext={{ campaign: "synthetic-campaign-A" }}
      />,
    );
    expect(html).toContain("NOT_EXPORTED");
    expect(html).toContain("子运行未导出或范围身份不匹配");
    snapshot.business_campaigns![0].frontier = null;
    snapshot.business_campaigns![0].visits = null;
    snapshot.business_campaigns![0].coverage.unmapped_branches = null;
    snapshot.business_campaigns![0].next_allowed = [];
    html = renderToStaticMarkup(
      <App
        state={{ snapshot: parseSnapshot(snapshot) }}
        initialContext={{ campaign: "synthetic-campaign-A" }}
      />,
    );
    expect(html).toContain("没有已导出的子运行记录");
    expect(html).toContain("未记录额外后续限制");
  });
  it("carries exact JSON i64 tokens from the text parser into candidate actual displays", () => {
    const fixture = campaignFixture();
    fixture.business_campaigns![0].frontier![0].input!.values!["request.amount"] = "__RAW_I64__";
    const text = JSON.stringify(fixture).replace('"__RAW_I64__"', "9223372036854775807");
    const snapshot = readSnapshotText(text);
    const html = renderToStaticMarkup(
      <App
        state={{ snapshot }}
        initialContext={{ campaign: "synthetic-campaign-A", variant: "V-100" }}
      />,
    );
    expect(html).toContain("9223372036854775807");
    expect(html).not.toContain("9223372036854776000");
    expect(snapshot.business_campaigns![0].frontier![0].eligibility).toBe("READY");
  });
});
