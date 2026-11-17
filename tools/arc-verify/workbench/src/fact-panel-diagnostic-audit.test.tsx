import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";
import { readSnapshotText } from "./snapshot";

describe("independent fact-inspector diagnosis audit", () => {
  it("shows the scoped evidence diagnostic when an original PASS cannot support an Actual path", () => {
    const { snapshot, run, selection } = recordedFixture();
    const fact = run.facts![0];
    const comparison = fact.business_expectations![0].comparison!;
    comparison.reason = "original-contradictory-backend-comparison-retained";
    comparison.checks[0].predicate.value = "FALSE";
    mirrorRaw(snapshot, run, fact);
    const parsed = readSnapshotText(JSON.stringify(snapshot));
    const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
    const diagnostic = graph.stateOverlay!.diagnostics.find(
      (item) => item.step === "1" && item.actor === "merchant" && item.action_index === 0,
    );
    expect(diagnostic).toBeDefined();
    expect(graph.edges.some((edge) => edge.actual?.step === "1")).toBe(false);
    const html = renderToStaticMarkup(
      <App state={{ snapshot: parsed }} initialContext={selection} />,
    );
    const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
    expect(panel).toContain(comparison.reason);
    expect(panel).toContain(diagnostic!.reason);
    expect(panel).toContain("status-blocked");
    expect(panel).toMatch(/原始记录|原记录|后端记录/);
  });

  it("keeps a consistent completed observation available as PASS", () => {
    const { snapshot, selection } = recordedFixture();
    const parsed = readSnapshotText(JSON.stringify(snapshot));
    const graph = graphFor(parsed, selection, branchOptions(parsed), parsed.runs[0]);
    expect(graph.edges.find((edge) => edge.actual?.step === "1")?.actual?.verdict).toBe("PASS");
    const html = renderToStaticMarkup(
      <App state={{ snapshot: parsed }} initialContext={selection} />,
    );
    const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
    expect(panel).toContain("synthetic unique branch");
    expect(panel).toContain("status-pass");
    expect(panel).not.toContain("status-blocked");
  });
});
