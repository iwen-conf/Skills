import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import { SnapshotError, parseSnapshot, readSnapshotText } from "./snapshot";
import { contractFixture } from "./test-fixtures";

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
  it("refuses cross-run or cross-model facts instead of leaking another account into the panel", () => {
    const wrongRun = contractFixture();
    wrongRun.runs[0].facts![0].run = "synthetic-run-B";
    expect(stageOf(() => parseSnapshot(wrongRun))).toBe("snapshot.facts");
    const wrongModel = contractFixture();
    wrongModel.runs[0].facts![0].graph = "other-model";
    expect(stageOf(() => parseSnapshot(wrongModel))).toBe("snapshot.facts");
    const duplicate = contractFixture();
    duplicate.runs[0].facts!.push(duplicate.runs[0].facts![0]);
    expect(stageOf(() => parseSnapshot(duplicate))).toBe("snapshot.facts");
  });
  it("validates new contract structure while keeping old snapshots supported", () => {
    expect(parseSnapshot(contractFixture()).model.data.business_contract?.branches).toHaveLength(3);
    const damaged = contractFixture();
    damaged.model.data.business_contract!.branches[0].expected.mode = "guess" as "deterministic";
    expect(stageOf(() => parseSnapshot(damaged))).toBe("snapshot.contract");
    expect(parseSnapshot(JSON.parse(sampleText)).model.data.business_contract).toBeUndefined();
  });
  it.each([undefined, null, [], [""], [7]])(
    "refuses malformed typed prestates before state projection: %j",
    (fromStates) => {
      const snapshot = contractFixture();
      Object.assign(snapshot.model.data.business_contract!.branches[0], {
        from_states: fromStates,
      });
      expect(stageOf(() => parseSnapshot(snapshot))).toBe("snapshot.contract");
    },
  );
  it("rejects damaged preparation and captured-response structures at the facts boundary", () => {
    const preparation = contractFixture();
    Object.assign(preparation.runs[0].facts![0], {
      preparation: { status: "PASS", plans: [], steps: [], readbacks: "missing" },
    });
    expect(stageOf(() => parseSnapshot(preparation))).toBe("snapshot.facts");
    const response = contractFixture();
    Object.assign(response.runs[0].facts![0], {
      actions: [{ response_values: ["not an object"] }],
    });
    expect(stageOf(() => parseSnapshot(response))).toBe("snapshot.facts");
  });
  it("accepts the native setup and recovery sequence identities without accepting negative action steps", () => {
    const snapshot = contractFixture();
    const first = snapshot.runs[0].facts![0];
    snapshot.runs[0].facts!.unshift({
      source_kind: "setup",
      run: first.run,
      graph: first.graph,
      seq: -1,
      verdict: "PASS",
    });
    snapshot.runs[0].facts!.push({
      source_kind: "recovery",
      run: first.run,
      graph: first.graph,
      seq: 0,
      verdict: "PASS",
      recovery_mode: "checkpoint",
    });
    expect(parseSnapshot(snapshot).runs[0].facts).toHaveLength(5);
    snapshot.runs[0].facts![0].source_kind = "execute";
    expect(stageOf(() => parseSnapshot(snapshot))).toBe("snapshot.facts");
  });
});
