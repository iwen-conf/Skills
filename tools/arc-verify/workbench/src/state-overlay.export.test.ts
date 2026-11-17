/// <reference types="node" />
import { readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import process from "node:process";
import { describe, expect, it } from "vite-plus/test";
import { branchOptions } from "./explorer-model";
import { graphFor } from "./graph-model";
import { entityKey } from "./facts";
import { initialSelection } from "./selection";
import { readSnapshotText } from "./snapshot";

const supplied = process.env.ARC_WORKBENCH_SNAPSHOT_PATHS?.trim();
const parsed: unknown = supplied ? JSON.parse(supplied) : [];
if (!Array.isArray(parsed) || parsed.some((value) => typeof value !== "string"))
  throw new Error("ARC_WORKBENCH_SNAPSHOT_PATHS must be an array of actual CLI export paths");
const paths: string[] = parsed;

describe("Actual state projection of supplied CLI exports", () => {
  it.skipIf(!paths.length)(
    "preserves the exporter’s captured state identities and evidence boundaries",
    () => {
      let capturedActions = 0;
      let observedEntities = 0;
      for (const path of paths) {
        const absolute = resolve(path);
        const snapshot = readSnapshotText(
          readFileSync(
            statSync(absolute).isDirectory() ? join(absolute, "snapshot.json") : absolute,
            "utf8",
          ),
        );
        const branches = branchOptions(snapshot);
        for (const run of snapshot.runs) {
          const decisions = (run.facts ?? []).flatMap((fact) =>
            (fact.business_expectations ?? []).map((item) => ({ fact, item })),
          );
          const branch =
            decisions.find(({ item }) => item.decision.frozen)?.item.decision.frozen?.branch ??
            run.branch_id ??
            "unresolved:records";
          const selection = {
            ...initialSelection(snapshot.meta.graph, run.environment),
            branch,
            run: run.trace.run,
            canvas: "states" as const,
          };
          const graph = graphFor(snapshot, selection, branches, run);
          expect(graph.stateOverlay).toBeDefined();
          for (const { fact, item } of decisions) {
            const frozen = item.decision.frozen;
            if (
              !frozen ||
              !item.after?.state ||
              fact.evidence_status !== "PASS" ||
              run.evidence_status !== "PASS"
            )
              continue;
            const actual = graph.stateOverlay!.occurrences.find(
              (occurrence) =>
                occurrence.step === String(fact.seq) &&
                occurrence.actor === item.actor &&
                occurrence.action_index === item.action_index,
            );
            expect(actual, JSON.stringify(graph.stateOverlay!.diagnostics)).toBeDefined();
            expect(actual).toMatchObject({
              run: run.trace.run,
              branch: frozen.branch,
              from_state: frozen.before.state,
              to_state: item.after.state,
              evidence_sha256: fact.evidence_sha256,
            });
            capturedActions++;
            for (const entity of fact.before_entities ?? []) {
              if (frozen.before.bindings?.[entity.alias] !== entity.id) continue;
              const scoped = graphFor(
                snapshot,
                { ...selection, actor: item.actor, entity: entityKey(entity) },
                branches,
                run,
              );
              expect(
                scoped.stateOverlay!.occurrences.some((occurrence) => occurrence.id === actual!.id),
                JSON.stringify(scoped.stateOverlay!.diagnostics),
              ).toBe(true);
              observedEntities++;
            }
          }
          if (!decisions.some(({ item }) => item.decision.frozen && item.after?.state))
            expect(graph.stateOverlay!.occurrences).toEqual([]);
        }
      }
      expect(
        capturedActions,
        "Supply exports containing actual independently captured tool actions",
      ).toBeGreaterThan(0);
      expect(
        observedEntities,
        "Include a same-entity campaign export to verify exact identity filtering",
      ).toBeGreaterThan(0);
    },
  );
});
