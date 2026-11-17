import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { branchOptions } from "./explorer-model";
import { entityKey } from "./facts";
import { graphFor } from "./graph-model";
import { mirrorRaw, recordedFixture } from "./state-overlay-fixtures";
import { readSnapshotText } from "./snapshot";

function concurrentCreation() {
  const { snapshot, run, selection } = recordedFixture();
  const fact = run.facts![0];
  const original = fact.entities![0];
  const entities = ["A", "B"].map((suffix) => ({
    ...original,
    alias: `created-${suffix}`,
    id: `new-order-${suffix}`,
    source_step: fact.seq,
    source_field: "response:/id",
  }));
  fact.before_entities = [];
  fact.entities = entities;
  const first = fact.business_expectations![0];
  const second = structuredClone(first);
  second.action_index = 1;
  second.request!.action_index = 1;
  second.request!.request_sha256 = "second-entity-request";
  second.comparison!.reason = "unrelated-entity-B-comparison";
  fact.business_expectations!.push(second);
  fact.requests!.push(second.request!);
  fact.actions!.push({
    ...structuredClone(fact.actions![0]),
    action_index: 1,
    request_sha256: second.request!.request_sha256,
    response_values: { entity: "unrelated-entity-B-response" },
  });
  fact.oracles!.push({ ...structuredClone(fact.oracles![0]), action_index: 1 });
  fact.extractions = entities.map((entity, index) => ({
    name: entity.alias,
    source: "response",
    path: "/id",
    value: entity.id,
    sha256: `captured-created-${index}`,
    entity_kind: entity.kind,
    entity_alias: entity.alias,
    owner_actor: entity.owner_actor,
    owner_principal: entity.owner_principal,
    step: fact.seq,
    source_actor: first.actor,
    action_index: index,
  }));
  mirrorRaw(snapshot, run, fact);
  return { snapshot: readSnapshotText(JSON.stringify(snapshot)), selection, entities };
}

describe("independent selection-to-fact scope audit", () => {
  it("keeps the fact inspector in the same entity scope as the observed graph", () => {
    const { snapshot, selection, entities } = concurrentCreation();
    const selected = { ...selection, entity: entityKey(entities[0]) };
    const graph = graphFor(snapshot, selected, branchOptions(snapshot), snapshot.runs[0]);
    expect(
      graph.stateOverlay?.occurrences
        .filter((item) => item.step === "1")
        .map((item) => item.action_index),
    ).toEqual([0]);
    const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={selected} />);
    const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
    expect(panel).toContain("synthetic unique branch");
    expect(panel).not.toContain("unrelated-entity-B-comparison");
    expect(panel).not.toContain("unrelated-entity-B-response");
  });

  it("does not restore an occurrence for a different entity into the selected entity's inspector", () => {
    const { snapshot, selection, entities } = concurrentCreation();
    const graph = graphFor(snapshot, selection, branchOptions(snapshot), snapshot.runs[0]);
    const other = graph.stateOverlay!.occurrences.find(
      (item) => item.step === "1" && item.action_index === 1,
    )!;
    expect(other.entity_keys).toContain(entityKey(entities[1]));
    expect(other.entity_keys).not.toContain(entityKey(entities[0]));
    const selected = { ...selection, entity: entityKey(entities[0]), occurrence: other.occurrence };
    const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={selected} />);
    const panel = html.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
    expect(panel).not.toContain("unrelated-entity-B-comparison");
    expect(panel).not.toContain("unrelated-entity-B-response");
  });
});
