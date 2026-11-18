import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import EtaPlanBoard from "./EtaPlanBoard";
import type { CanvasGraph } from "./graph-model";
import "./index.css";
import "./eta-board.css";

/** Synthetic visual fixture. This page never loads a production Arc ledger. */
const graph: CanvasGraph = {
  nodes: [
    { id: "create-order", subject: "create-order", kind: "preparation", name: "起草退款接口", status: "SATISFIED" },
    { id: "validate-payment", subject: "validate-payment", kind: "condition", name: "补全支付条件", status: "NOT_OBSERVED" },
    { id: "issue-refund", subject: "issue-refund", kind: "target", name: "发起退款", status: "BLOCKED" },
  ],
  edges: [
    { id: "from-create", from: "create-order", to: "issue-refund", kind: "requires", label: "必要前置" },
    { id: "from-valid", from: "validate-payment", to: "issue-refund", kind: "requires", label: "必要前置" },
  ],
  empty: "none", caption: "synthetic fixture; no product execution",
};
const groups = [{ id: "refund-plan", label: "退款流程", members: graph.nodes.map((node) => node.id) }];

function Fixture() {
  const [selected, setSelected] = useState("");
  return (
    <main className="eta-source-fixture">
      <EtaPlanBoard
        graph={graph}
        groups={groups}
        selected={selected ? new Set([selected]) : new Set<string>()}
        onSelectNode={(node) => setSelected(node.id)}
      />
      <output id="eta-selected-node" aria-live="polite">{selected}</output>
    </main>
  );
}
createRoot(document.getElementById("root")!).render(
  <StrictMode><Fixture /></StrictMode>,
);
