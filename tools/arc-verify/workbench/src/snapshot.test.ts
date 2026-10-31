import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import { SnapshotError, parseSnapshot, readSnapshotText } from "./snapshot";

const sampleText = JSON.stringify(sample);

function stageOf(action: () => unknown): string {
  try {
    action();
  } catch (error) {
    if (error instanceof SnapshotError) return error.stage;
    throw error;
  }
  throw new Error("expected a snapshot error");
}

describe("embedded snapshot boundary", () => {
  it("reads the CLI-exported sample with all six views and its runs", () => {
    const snapshot = parseSnapshot(JSON.parse(sampleText));
    expect(Object.keys(snapshot.views).sort()).toEqual([
      "boundaries",
      "capabilities",
      "flows",
      "impact",
      "overview",
      "verification",
    ]);
    expect(snapshot.runs.length).toBe(2);
    expect(snapshot.meta.graph).toBe("course-v2");
  });
  it("fails visibly when the page has no snapshot", () => {
    expect(stageOf(() => readSnapshotText(null))).toBe("snapshot.missing");
    expect(stageOf(() => readSnapshotText("   "))).toBe("snapshot.missing");
  });
  it("fails visibly on broken JSON", () => {
    expect(stageOf(() => readSnapshotText('{"meta":'))).toBe("snapshot.parse");
  });
  it("rejects a snapshot that would hide or invent results", () => {
    const base = JSON.parse(sampleText);
    const withoutView = JSON.parse(sampleText);
    delete withoutView.views.flows;
    expect(stageOf(() => parseSnapshot(withoutView))).toBe("snapshot.views");
    const guessed = JSON.parse(sampleText);
    guessed.views.overview.verdict = "GUESS_PASS";
    expect(stageOf(() => parseSnapshot(guessed))).toBe("snapshot.views");
    const duplicated = JSON.parse(sampleText);
    duplicated.views.capabilities.nodes.push(duplicated.views.capabilities.nodes[0]);
    expect(stageOf(() => parseSnapshot(duplicated))).toBe("snapshot.views");
    const unknownStatus = JSON.parse(sampleText);
    unknownStatus.views.verification.obligations[0].status = "LIKELY";
    expect(stageOf(() => parseSnapshot(unknownStatus))).toBe("snapshot.views");
    expect(stageOf(() => parseSnapshot({ ...base, meta: { ...base.meta, graph: 7 } }))).toBe(
      "snapshot.meta",
    );
    expect(stageOf(() => parseSnapshot({ ...base, runs: null }))).toBe("snapshot.runs");
  });
});
