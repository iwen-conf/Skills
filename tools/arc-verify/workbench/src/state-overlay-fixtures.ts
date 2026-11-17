import type { UnifiedStepFact } from "./facts";
import { initialSelection } from "./selection";
import type { Selection } from "./selection";
import { contractFixture } from "./test-fixtures";
import type { Snapshot, SnapshotRun } from "./types";

export function mirrorRaw(snapshot: Snapshot, run: SnapshotRun, fact: UnifiedStepFact) {
  const model = structuredClone({
    evidence_version: fact.evidence_version,
    participants: fact.participants,
    before_entities: fact.before_entities,
    entities: fact.entities,
    oracles: fact.oracles,
    actions: fact.actions,
    requests: fact.requests,
    business_expectations: fact.business_expectations,
    extractions: fact.extractions,
  });
  run.evidence = run.evidence.filter((entry) => entry.step !== fact.seq);
  run.evidence.push({
    run: run.trace.run,
    phase: "action",
    step: fact.seq,
    sha256: fact.evidence_sha256,
    data: {
      run: run.trace.run,
      step: fact.seq,
      identity: {
        graph: snapshot.meta.graph,
        graph_sha256: snapshot.meta.graph_sha256,
        task: run.trace.task,
        environment: run.environment,
      },
      expected: fact.expected_state!,
      observed: fact.observed_state!,
      preconditions: {},
      denied: false,
      oracle: "isolated-db",
      runner: "synthetic-overlay-fixture",
      exit_code: 0,
      model,
    },
  });
  const trace = run.trace.steps.find((entry) => entry.step === String(fact.seq))!;
  trace.sha256 = fact.evidence_sha256!;
  trace.evidence_status = "PASS";
  trace.observation = structuredClone(model);
}

/** Synthetic projection records with explicit receipts, never a product execution claim. */
export function recordedFixture() {
  const snapshot = contractFixture();
  snapshot.contract_analysis!.contract_sha256 = "synthetic-contract";
  for (const run of snapshot.runs) {
    run.trace.evidence_status = "PASS";
    for (const fact of run.facts!) {
      fact.source_kind = "execute";
      fact.evidence_sha256 = `synthetic-raw-${run.trace.run}-${fact.seq}`;
      fact.before_entities = structuredClone(fact.entities);
      const recorded = fact.business_expectations![0];
      const frozen = recorded.decision.frozen!;
      recorded.after!.bindings = { ...frozen.before.bindings };
      for (const observation of [frozen.before, recorded.after!])
        observation.sources = Object.fromEntries(
          Object.keys(observation.values).map((key) => [key, ["isolated-db"]]),
        );
      const request = {
        actor: recorded.actor,
        action_index: recorded.action_index,
        node: frozen.action,
        template_sha256: "synthetic-template",
        request_sha256: `synthetic-request-${fact.seq}`,
        values: {},
      };
      recorded.request = request;
      fact.requests = [request];
      fact.actions = [
        {
          ...request,
          principal: fact.principal!,
          response_sha256: `synthetic-response-${fact.seq}`,
          exit_code: 0,
          denied: false,
          started: "2026-11-17T00:00:00Z",
          completed: "2026-11-17T00:00:01Z",
        },
      ];
      mirrorRaw(snapshot, run, fact);
    }
  }
  const run = snapshot.runs[0];
  const selection: Selection = {
    ...initialSelection(snapshot.meta.graph, run.environment),
    canvas: "states",
    flow: "F-PAY",
    branch: "B-CREATE",
    run: run.trace.run,
    step: "1",
    actor: "merchant",
    node: "create",
  };
  return { snapshot, run, selection };
}
