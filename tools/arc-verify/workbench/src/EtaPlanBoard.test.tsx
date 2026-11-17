import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import EtaPlanBoard from "./EtaPlanBoard";
import { etaReadPlan } from "./eta-read-plan";
import type { CanvasGraph } from "./graph-model";

const graph: CanvasGraph = {
  nodes: [
    { id: "setup", subject: "setup", kind: "preparation", name: "建立前置状态", status: "NOT_OBSERVED" },
    { id: "ready", subject: "ready", kind: "condition", name: "订单已确认", status: "SATISFIED" },
    { id: "refund", subject: "refund", kind: "target", name: "发起退款", status: "NOT_OBSERVED" },
  ],
  edges: [
    { id:"provider",from:"setup",to:"ready",kind:"candidate_provider",label:"候选提供",alternative:true },
    { id:"required",from:"ready",to:"refund",kind:"requires",label:"必要依赖" },
  ],
  caption:"synthetic fixture, no real gateway calls",
  empty:"none",
};
const groups = [
  { id:"prepare",label:"前态准备",members:["setup","ready"] },
  { id:"refund",label:"退款分支",members:["refund"] },
];
describe("ETA WidePlan/PhonePlan UI parity", () => {
  it("renders desktop step columns, curved-wire layer and mobile step stack", () => {
    const html = renderToStaticMarkup(
      <EtaPlanBoard graph={graph} groups={groups} selected={new Set()} onSelectNode={() => {}} />,
    );
    expect(html).toContain('aria-label="ETA式业务依赖计划"');
    expect(html).toContain('class="eta-board-wide"');
    expect(html).toContain('class="eta-board-phone"');
    expect(html).toContain('class="eta-board-columns"');
    expect(html).toContain('class="eta-board-wires"');
    expect(html).toContain('data-eta-group="prepare"');
    expect(html).toContain('data-eta-group="refund"');
    expect(html).toContain('data-eta-phone-step="1"');
    expect(html).toContain('data-eta-status="OBSERVED"');
    expect(html).toContain("跨业务组依赖");
    expect(html).toContain("候选/OR 来源");
    expect(html).toContain('data-alternative="true"');
    expect(html).not.toContain('data-handle=');
    expect(html).toContain("业务证据");
    expect(html).not.toContain("业务 PASS · 已完成");
  });
  it("preserves a true dependent depth, ignoring candidate provider as required work", () => {
    const plan = etaReadPlan(graph);
    expect(plan.levels.get("setup")).toBe(0);
    expect(plan.levels.get("ready")).toBe(0);
    expect(plan.levels.get("refund")).toBe(1);
    expect(plan.critical).toEqual(["ready", "refund"]);
  });
  it("refuses to silently truncate giant graphs", () => {
    const many: CanvasGraph = {
      ...graph,
      nodes: Array.from({length:801},(_,i)=>({
        id:"n" + i, subject:"n" + i, kind:"condition", name:"Node " + i,
      })),
      edges:[],
    };
    const html=renderToStaticMarkup(<EtaPlanBoard
      graph={many} groups={[]} selected={new Set()} onSelectNode={() => {}} />);
    expect(html).toContain("801");
    expect(html).toContain("完整分层目录");
    expect(html).not.toContain('class="eta-board-wide"');
  });
});
