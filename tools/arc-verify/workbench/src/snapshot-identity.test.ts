import { describe, expect, it } from "vite-plus/test";
import { contractFixture } from "./test-fixtures";
import { readSnapshotText, SnapshotError } from "./snapshot";

describe("captured identity collection boundaries", () => {
  it.each(["participants", "entities", "before_entities", "sessions", "before_sessions"])(
    "refuses non-object or incomplete entries in %s",
    (field) => {
      for (const entry of [null, 7, "account-A", {}, []]) {
        const snapshot = contractFixture();
        Object.assign(snapshot.runs[0].facts![0], { [field]: [entry] });
        expect(() => readSnapshotText(JSON.stringify(snapshot))).toThrow(SnapshotError);
      }
    },
  );

  it("preserves optional historical identity collections without manufacturing missing bindings", () => {
    const snapshot = contractFixture();
    const first = snapshot.runs[0].facts![0];
    delete first.before_entities;
    delete first.before_sessions;
    const result = readSnapshotText(JSON.stringify(snapshot));
    expect(result.runs[0].facts![0].before_entities).toBeUndefined();
    expect(result.runs[0].facts![0].before_sessions).toBeUndefined();
    expect(result.runs[0].facts![0].participants).toEqual(first.participants);
  });
});
