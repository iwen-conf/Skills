import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import sample from "../dev/sample-snapshot.json";
import App from "./App";
import { branchOptions } from "./explorer-model";
import { readSnapshotText, SnapshotError } from "./snapshot";
import type { Snapshot, SnapshotEvidence } from "./types";

const fixture = () => structuredClone(sample) as unknown as Snapshot;
const read = (snapshot: Snapshot) => readSnapshotText(JSON.stringify(snapshot));
const action = (snapshot: Snapshot) =>
  snapshot.runs[0].evidence.find((item) => item.phase === "action")!;

function renderRun(snapshot: Snapshot) {
  const run = snapshot.runs[0];
  const branch = branchOptions(snapshot).find(
    (item) => item.legacy && run.trace.task.endsWith(`/${item.legacy.id}`),
  )!;
  return renderToStaticMarkup(
    <App
      state={{ snapshot }}
      initialContext={{ branch: branch.id, run: run.trace.run, step: "1" }}
    />,
  );
}

describe("RAW summary occurrence identities", () => {
  it("keeps the actual legacy Go export readable with setup -1 and an omitted action phase", () => {
    const snapshot = read(fixture());
    const run = snapshot.runs[0];
    const setup = run.evidence.find((item) => item.phase === "setup")!;
    expect(setup.step).toBe(0);
    expect(setup.data!.step).toBe(-1);
    expect(action(snapshot).data!.phase).toBeUndefined();
    const html = renderRun(snapshot);
    expect(html).toContain(action(snapshot).sha256);
  });

  it("refuses another run's complete RAW entry before it can appear under the selected run", () => {
    const snapshot = fixture();
    const foreign = snapshot.runs[1].evidence.find((item) => item.phase === "action")!;
    snapshot.runs[0].evidence = [foreign];
    expect(() => renderRun(read(snapshot))).toThrow(SnapshotError);
  });

  it.each([
    [
      "RAW run",
      (entry: SnapshotEvidence) => {
        entry.data!.run = "another-run";
      },
    ],
    [
      "RAW step",
      (entry: SnapshotEvidence) => {
        entry.data!.step = 2;
      },
    ],
    [
      "RAW phase",
      (entry: SnapshotEvidence) => {
        entry.data!.phase = "fixture";
      },
    ],
    [
      "RAW graph",
      (entry: SnapshotEvidence) => {
        entry.data!.identity.graph = "another-model";
      },
    ],
    [
      "RAW task",
      (entry: SnapshotEvidence) => {
        entry.data!.identity.task = "another-task";
      },
    ],
  ] as const)(
    "rejects a conflicting %s even when its surrounding entry claims the selected run",
    (_name, mutate) => {
      const snapshot = fixture();
      mutate(action(snapshot));
      expect(() => read(snapshot)).toThrow(SnapshotError);
    },
  );

  it("refuses two RAW entries claiming the same occurrence instead of rendering both as that step", () => {
    const snapshot = fixture();
    snapshot.runs[0].evidence.push(structuredClone(action(snapshot)));
    expect(() => read(snapshot)).toThrow(SnapshotError);
  });

  it("keeps an exported verification error visible without requiring invented RAW data", () => {
    const snapshot = fixture();
    const run = snapshot.runs[0];
    run.evidence = [
      { run: run.trace.run, phase: "action", step: 1, error: "recorded-RAW-read-failure" },
    ];
    run.evidence_status = "BLOCKED";
    const html = renderRun(read(snapshot));
    expect(html).toContain("recorded-RAW-read-failure");
    expect(html).toContain("导出时校验失败");
  });
});
