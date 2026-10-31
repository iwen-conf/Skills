import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import { parseSnapshot } from "./snapshot";
import { drillRoot, evidenceRun, visibleNodes } from "./views";

const snapshot = parseSnapshot(JSON.parse(JSON.stringify(sample)));
const ids = (nodes: { id: string }[]) => nodes.map((node) => node.id).sort();

describe("drill-down and filters", () => {
  it("narrows a hierarchical view to the selected subtree", () => {
    expect(drillRoot(snapshot.views.capabilities, "enrollment")).toBe("enrollment");
    expect(ids(visibleNodes(snapshot.views.capabilities, { root: "enrollment" }))).toEqual([
      "enrollment",
      "enrollment.submit",
    ]);
  });
  it("keeps the whole view when the entity has no subtree there", () => {
    expect(drillRoot(snapshot.views.flows, "enrollment.submit")).toBe("");
    expect(visibleNodes(snapshot.views.flows, {}).length).toBe(snapshot.views.flows.nodes.length);
  });
  it("filters by domain and by the role a capability serves", () => {
    expect(ids(visibleNodes(snapshot.views.capabilities, { domain: "enrollment" }))).toEqual([
      "enrollment",
      "enrollment.submit",
    ]);
    expect(ids(visibleNodes(snapshot.views.capabilities, { role: "STUDENT" }))).toEqual([
      "STUDENT",
      "enrollment",
      "enrollment.submit",
    ]);
    expect(visibleNodes(snapshot.views.capabilities, { role: "ADMIN" })).toEqual([]);
  });
  it("filters the displayed nodes without touching the denominator", () => {
    const view = snapshot.views.verification;
    const before = JSON.stringify(view.coverage);
    const flaky = visibleNodes(view, { status: "FLAKY" });
    expect(flaky.length).toBeGreaterThan(0);
    expect(flaky.every((node) => node.status === "FLAKY")).toBe(true);
    expect(JSON.stringify(view.coverage)).toBe(before);
  });
  it("opens the first FAIL with first-divergence instead of the latest retry", () => {
    const obligation = snapshot.views.verification.obligations.find(
      (item) => item.definition.id === "R1/POSITIVE",
    );
    const failed = snapshot.runs.find(
      (run) => run.trace.status === "FAIL" && Boolean(run.trace.first_divergence),
    );
    expect(obligation).toBeTruthy();
    expect(failed).toBeTruthy();
    expect(obligation?.run).not.toBe(failed?.trace.run);
    expect(evidenceRun(obligation!, snapshot.runs)).toBe(failed?.trace.run);
  });
});
