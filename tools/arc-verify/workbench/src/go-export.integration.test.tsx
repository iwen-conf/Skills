/// <reference types="node" />
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import process from "node:process";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";
import App from "./App";
import { isCampaignDiagnostic } from "./campaign";
import { displayValue } from "./contract";
import type { BusinessObservation } from "./contract";
import { actorsOf, branchOptions, factsOf, selectionContext } from "./explorer-model";
import { graphFor } from "./graph-model";
import { analyzeDirected } from "./layout";
import { occurrenceKey } from "./lineage";
import { buildDependencyMap } from "./lineage-map";
import { buildLineageBlockers } from "./lineage-blockers";
import { entityKey } from "./facts";
import type { UnifiedStepFact } from "./facts";
import { initialSelection, reduceSelection } from "./selection";
import type { Selection, SelectionAction } from "./selection";
import { readSnapshotText, SnapshotError } from "./snapshot";

/**
 * Optional integration against actual CLI exports, never the synthetic UI fixture.
 * ARC_WORKBENCH_SNAPSHOT_PATH=/absolute/export/snapshot.json (or its directory)
 * also accepts a JSON array of paths; ARC_WORKBENCH_SNAPSHOT_PATHS is an array alias.
 * Each supplied directory receives frontend-validation.json and ssr-verified.html.
 * These results cover the current reader/rendering, not target execution or RAW revalidation.
 */
function exportPaths(): string[] {
  const single = process.env.ARC_WORKBENCH_SNAPSHOT_PATH?.trim();
  const multiple = process.env.ARC_WORKBENCH_SNAPSHOT_PATHS?.trim();
  if (single && multiple)
    throw new Error("Set one ARC_WORKBENCH_SNAPSHOT_PATH or ARC_WORKBENCH_SNAPSHOT_PATHS value");
  const configured = single || multiple;
  if (!configured) return [];
  const paths: unknown = configured.startsWith("[") ? JSON.parse(configured) : [configured];
  if (
    !Array.isArray(paths) ||
    !paths.length ||
    paths.some((item) => typeof item !== "string" || !item.trim())
  )
    throw new Error("Snapshot path configuration must be a path or a nonempty JSON string array");
  return [
    ...new Set(
      (paths as string[]).map((path) => {
        const absolute = resolve(path);
        return statSync(absolute).isDirectory() ? join(absolute, "snapshot.json") : absolute;
      }),
    ),
  ];
}

const escaped = (value: string) =>
  value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
function visible(html: string, value: string | undefined, description = "source text") {
  if (value) expect(html.includes(escaped(value)), `SSR must retain ${description}`).toBe(true);
}
function valuesVisible(html: string, values?: Record<string, unknown>) {
  for (const [key, value] of Object.entries(values ?? {})) {
    visible(html, key, "observed attribute name");
    visible(html, displayValue(value), "exact observed JSON value");
  }
}
function observationVisible(html: string, observation?: BusinessObservation) {
  if (!observation) return;
  visible(html, observation.state, "observed state");
  valuesVisible(html, observation.values);
  for (const binding of Object.values(observation.bindings ?? {}))
    visible(html, binding, "observed subject binding");
  for (const sources of Object.values(observation.sources ?? {}))
    for (const source of sources) visible(html, source, "independent observation source");
}

function validateExport(path: string) {
  const directory = dirname(path);
  const suffix = basename(path) === "snapshot.json" ? "" : `${basename(path, ".json")}-`;
  const reportPath = join(directory, `${suffix}frontend-validation.json`);
  const ssrPath = join(directory, `${suffix}ssr-verified.html`);
  const scope =
    "Actual supplied Go export -> production readSnapshotText -> unified selection -> SSR. No target execution, RAW revalidation, product E2E claim or browser acceptance.";
  writeFileSync(reportPath, JSON.stringify({ status: "RUNNING", scope }, null, 2));
  writeFileSync(
    ssrPath,
    '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>验证进行中</title><p>本次 SSR 验证尚未完成，请查看 frontend-validation.json。</p></html>',
  );
  try {
    const source = readFileSync(path, "utf8");
    const snapshot = readSnapshotText(source);
    expect(
      snapshot.runs.length +
        (snapshot.business_campaigns?.length ?? 0) +
        (snapshot.business_lineage?.branches.length ?? 0),
      "Provide a CLI export with ledger records or a versioned model genealogy",
    ).toBeGreaterThan(0);
    const context = selectionContext(snapshot);
    const reduce = (state: Selection, action: SelectionAction) =>
      reduceSelection(state, action, context);
    const pages = new Map<string, string>();
    let renderedViews = 0;
    let preparationRuns = 0;
    let preparationActions = 0;
    let preparationReadbacks = 0;
    let frozenComparisons = 0;
    let unassociatedStepViews = 0;
    let requestCaptures = 0;
    let entitySelections = 0;
    let linkedChildren = 0;
    let unexportedChildren = 0;
    const runReports: Record<string, unknown>[] = [];
    const campaignReports: Record<string, unknown>[] = [];
    const lineageReports: Record<string, unknown>[] = [];
    const lineageScopeReports: Record<string, unknown>[] = [];
    const blockerReports: Record<string, unknown>[] = [];
    const statePathReports: Record<string, unknown>[] = [];
    let observedSelections = 0;
    let observedEntitySelections = 0;
    let blockedPreparationsWithoutTarget = 0;
    const render = (selection: Selection, representative?: string) => {
      const html = renderToStaticMarkup(<App state={{ snapshot }} initialContext={selection} />);
      for (const label of ["能力树与分支导航", "分支与事实面板", "执行轨迹", "离线只读快照"])
        visible(html, label, "workbench region");
      renderedViews++;
      if (representative && !pages.has(representative)) pages.set(representative, html);
      return html;
    };
    const selectRun = (runId: string, fact?: UnifiedStepFact, actor = "") => {
      const run = context.runs.find((item) => item.id === runId)!;
      expect(run, "every exported run must have a selectable identity").toBeDefined();
      const step = run.steps.find((item) => item.id === String(fact?.seq));
      const branch =
        (actor ? step?.actorBranches?.[actor]?.[0] : undefined) ||
        step?.branches?.[0] ||
        run.branches?.[0] ||
        run.branch;
      let selection = initialSelection(snapshot.meta.graph, run.environment);
      selection = reduce(selection, { type: "branch", value: branch });
      selection = reduce(selection, { type: "run", value: runId });
      if (actor) selection = reduce(selection, { type: "actor", value: actor });
      if (step) selection = reduce(selection, { type: "step", value: step.id });
      expect(selection.run, "a run must not resolve to a neighboring run").toBe(runId);
      if (step) expect(selection.step).toBe(step.id);
      if (actor) expect(selection.actor).toBe(actor);
      return selection;
    };

    const sharedMap = buildDependencyMap(snapshot);
    let sharedMapReport: Record<string, unknown> = { status: "NOT_PRESENT" };
    if (snapshot.business_lineage?.branches.length) {
      expect(sharedMap.status, sharedMap.reason).toBe("READY");
      const selected = reduce(initialSelection(snapshot.meta.graph), {
        type: "dependencyScope",
        value: "model",
      });
      const graph = graphFor(snapshot, selected, branchOptions(snapshot));
      expect(graph).toEqual(sharedMap.graph);
      expect(graph.stateOverlay).toBeUndefined();
      expect(graph.lineage).toBeUndefined();
      expect(graph.nodes.every((node) => node.status === "NOT_OBSERVED" && !node.actual)).toBe(
        true,
      );
      expect(graph.edges.every((edge) => !edge.actual && edge.kind !== "actual_transition")).toBe(
        true,
      );
      const projectedBySource = new Map<string, string>();
      for (const [id, members] of sharedMap.members) {
        expect(graph.nodes.some((node) => node.id === id)).toBe(true);
        expect(members.length).toBeGreaterThan(0);
        for (const member of members) {
          const key = JSON.stringify([member.branch, member.node]);
          expect(projectedBySource.has(key), "an original node has exactly one projection").toBe(
            false,
          );
          projectedBySource.set(key, id);
          const original = snapshot.business_lineage.branches
            .find((branch) => branch.branch === member.branch)
            ?.nodes.find((node) => node.id === member.node);
          expect(member.declaration, "a shared member retains its exact declaration").toEqual(
            original,
          );
          expect(member.source_ref).toBe(original?.source_ref);
        }
      }
      const edgeSources: string[] = [];
      for (const [id, members] of sharedMap.edgeMembers) {
        const edge = graph.edges.find((item) => item.id === id)!;
        expect(edge).toBeDefined();
        expect(members.length).toBeGreaterThan(0);
        for (const member of members) {
          const original = snapshot.business_lineage.branches
            .find((branch) => branch.branch === member.branch)
            ?.edges.find((item) => item.id === member.edge);
          expect(original).toBeDefined();
          expect(member.declaration).toEqual(original);
          expect(member.source_ref).toBe(original!.source_ref);
          expect(edge.kind).toBe(original!.kind);
          expect(edge.from).toBe(
            projectedBySource.get(JSON.stringify([member.branch, original!.from])),
          );
          expect(edge.to).toBe(
            projectedBySource.get(JSON.stringify([member.branch, original!.to])),
          );
          if (original!.alternative) expect(edge.alternative).toBe(true);
          edgeSources.push(JSON.stringify([member.branch, member.edge]));
        }
      }
      expect([...projectedBySource.keys()].sort()).toEqual(
        snapshot.business_lineage.branches
          .flatMap((branch) => branch.nodes.map((node) => JSON.stringify([branch.branch, node.id])))
          .sort(),
      );
      expect(edgeSources.sort()).toEqual(
        snapshot.business_lineage.branches
          .flatMap((branch) => branch.edges.map((edge) => JSON.stringify([branch.branch, edge.id])))
          .sort(),
      );
      expect(graph.groups!.flatMap((group) => group.members).sort()).toEqual(
        graph.nodes.map((node) => node.id).sort(),
      );
      const html = render(selected, "Shared dependency model");
      visible(html, "共享依赖与影响分支", "global dependency inspector");
      visible(html, "未叠加任何运行结果", "declarations do not borrow Actual");
      for (const group of graph.groups!) {
        visible(html, group.label, "declared capability grouping");
        expect(html).toContain(`data-plan-group="${escaped(group.id)}"`);
        for (const id of group.members) expect(html).toContain(`data-plan-node="${escaped(id)}"`);
      }
      const shared = [...sharedMap.members].filter(([, members]) => members.length > 1);
      for (const [id, members] of shared) {
        const nodeHTML = render(
          reduce(selected, { type: "lineage", value: id }),
          pages.has("Shared dependency provenance") ? undefined : "Shared dependency provenance",
        );
        visible(nodeHTML, "原始分支节点与规则来源", "expandable source provenance");
        for (const member of members) {
          visible(nodeHTML, member.branch, "shared node source branch");
          visible(nodeHTML, member.node, "shared node original identity");
          visible(nodeHTML, member.source_ref, "shared node rule source");
        }
      }
      const analysis = analyzeDirected(graph.nodes, graph.edges);
      expect(analysis.cycles).toEqual([]);
      sharedMapReport = {
        status: "EXERCISED",
        branches: snapshot.business_lineage.branches.length,
        nodes: graph.nodes.length,
        original_nodes: projectedBySource.size,
        shared_nodes: shared.length,
        edges: graph.edges.length,
        original_edges: edgeSources.length,
        groups: graph.groups,
        source_members_preserved: true,
        source_relations_preserved: true,
        actual_occurrences: 0,
      };
    }

    for (const lineage of snapshot.business_lineage?.branches ?? []) {
      let selected = reduce(initialSelection(snapshot.meta.graph), {
        type: "branch",
        value: lineage.branch,
      });
      selected = reduce(selected, { type: "canvas", value: "dependencies" });
      expect(selected.dependencyScope).toBe("branch");
      const graph = graphFor(snapshot, selected, branchOptions(snapshot));
      expect(graph.lineage?.graph).toBe(lineage);
      expect(graph.nodes.map((node) => node.id)).toEqual(lineage.nodes.map((node) => node.id));
      expect(graph.nodes.every((node) => node.status === "NOT_OBSERVED")).toBe(true);
      expect(graph.groups).toBeUndefined();
      expect(buildLineageBlockers(graph.lineage!).status).toBe("UNOBSERVED");
      const analysis = analyzeDirected(graph.nodes, graph.edges);
      const html = render(selected, pages.has("Model genealogy") ? undefined : "Model genealogy");
      visible(html, "依赖系谱分层目录", "complete dependency directory");
      visible(html, "尚无实际前态观察", "model-only observation boundary");
      lineageReports.push({
        branch: lineage.branch,
        nodes: graph.nodes.length,
        edges: graph.edges.length,
        layers: Math.max(0, ...analysis.levels.values()) + 1,
        cycle_components: analysis.cycles.length,
        candidate_providers: graph.edges.filter((edge) => edge.kind === "candidate_provider")
          .length,
        observed_nodes: 0,
        explicit_branch_scope: selected.dependencyScope,
        dense_directory_default: graph.nodes.length > 16,
      });
    }

    for (const run of snapshot.runs) {
      const facts = factsOf(snapshot, run);
      const html = render(selectRun(run.trace.run), `Run ${run.trace.run}`);
      visible(html, run.trace.run, "selected run identity");
      visible(html, run.environment, "run environment");
      visible(html, run.runner_id, "verified runner identity");
      visible(
        html,
        run.evidence_status ?? "LEGACY",
        "evidence status, separate from stored result",
      );
      const preparations = facts
        .filter((fact) => fact.preparation)
        .map((fact) => fact.preparation!);
      if (preparations.length) preparationRuns++;
      for (const preparation of preparations) {
        visible(html, "本次运行准备", "actual preparation panel");
        visible(html, "准备独立回读记录", "preparation readback evidence");
        valuesVisible(html, preparation.initial_observed.values);
        valuesVisible(html, preparation.final_observed.values);
        preparationActions += preparation.steps.length;
        preparationReadbacks += preparation.readbacks.length;
        expect(
          preparation.readbacks.length,
          "actual preparation must retain independent observations",
        ).toBeGreaterThan(0);
        for (const step of preparation.steps) {
          visible(html, step.planned.id, "executed preparation identity");
          visible(html, step.receipt.action.request_sha256, "actual preparation request hash");
          valuesVisible(html, step.before.values);
          valuesVisible(html, step.after?.values);
        }
        for (const readback of preparation.readbacks) {
          visible(html, readback.phase, "independent readback phase");
          valuesVisible(html, readback.observation?.values);
        }
      }
      for (const fact of facts.filter((item) => item.seq > 0)) {
        const actors = actorsOf(fact);
        for (const actor of actors.length ? actors : [""]) {
          const selected = selectRun(run.trace.run, fact, actor);
          const factHTML = render(selected);
          if (snapshot.model.data.business_contract && !fact.business_expectations?.length) {
            expect(
              selected.branch,
              "a pre-freeze failure must not inherit another step's branch",
            ).toBe("unresolved:records");
            visible(factHTML, "分支未解析 · 运行诊断", "missing frozen branch diagnostic");
            const panel = factHTML.match(/<aside class="fact-panel"[\s\S]*?<\/aside>/)?.[0] ?? "";
            expect(panel).not.toContain("预期操作与结果");
            unassociatedStepViews++;
          }
          visible(factHTML, `${run.trace.run} / 第 ${fact.seq} 步`, "step/run scope");
          const participant = fact.participants?.find((item) => item.alias === actor);
          visible(factHTML, participant?.principal, "selected participant identity");
          for (const request of (fact.requests ?? []).filter(
            (item) => !actor || item.actor === actor,
          )) {
            requestCaptures++;
            visible(factHTML, request.request_sha256, "captured request hash");
            valuesVisible(factHTML, request.values ?? undefined);
          }
          for (const expectation of (fact.business_expectations ?? []).filter(
            (item) =>
              (!actor || item.actor === actor) &&
              (!item.decision.frozen ||
                item.decision.frozen.branch === selected.branch ||
                selected.branch === "unresolved:records"),
          )) {
            if (!expectation.decision.frozen) continue;
            frozenComparisons++;
            visible(factHTML, "冻结前置条件与实际结果", "Expected vs Actual table");
            visible(factHTML, expectation.decision.frozen.sha256, "durable frozen identity");
            valuesVisible(factHTML, expectation.decision.frozen.before.values);
            valuesVisible(factHTML, expectation.after?.values);
            visible(factHTML, expectation.comparison?.status, "backend comparison result");
          }
        }
        for (const entity of fact.entities ?? []) {
          const owner = actors.includes(entity.owner_actor) ? entity.owner_actor : "";
          const selected = reduce(selectRun(run.trace.run, fact, owner), {
            type: "entity",
            value: entityKey(entity),
          });
          expect(selected).toMatchObject({
            run: run.trace.run,
            step: String(fact.seq),
            entity: entityKey(entity),
          });
          const entityHTML = render(selected);
          visible(entityHTML, entity.id, "selected stable entity ID");
          visible(entityHTML, entity.owner_principal, "selected entity owner");
          entitySelections++;
        }
      }
      for (const overlay of run.business_lineage ?? []) {
        const fact = facts.find((item) => item.seq === overlay.step);
        let selected = selectRun(run.trace.run, fact, overlay.actor);
        if (selected.branch !== overlay.branch) {
          selected = reduce(selected, { type: "branch", value: overlay.branch });
          selected = reduce(selected, { type: "run", value: run.trace.run });
          selected = reduce(selected, { type: "step", value: String(overlay.step) });
          selected = reduce(selected, { type: "actor", value: overlay.actor });
        }
        selected = reduce(selected, { type: "canvas", value: "dependencies" });
        selected = reduce(selected, { type: "occurrence", value: occurrenceKey(overlay) });
        expect(selected).toMatchObject({
          run: run.trace.run,
          branch: overlay.branch,
          step: String(overlay.step),
          actor: overlay.actor,
        });
        const graph = graphFor(snapshot, selected, branchOptions(snapshot), run);
        expect(graph.lineage?.overlay).toBe(overlay);
        expect(graph.nodes).toHaveLength(overlay.nodes.length);
        const qualified =
          !overlay.diagnostic &&
          [
            overlay.evidence_status,
            run.evidence_status,
            fact?.evidence_status ?? run.evidence_status,
          ].every((value) => value === "PASS");
        for (const reading of overlay.nodes)
          expect(graph.nodes.find((node) => node.id === reading.node)?.status).toBe(
            qualified ? reading.status : "BLOCKED",
          );
        const reading = overlay.nodes.find((node) => node.values?.length);
        if (reading) selected = reduce(selected, { type: "lineage", value: reading.node });
        const html = render(selected, `Genealogy ${run.trace.run} / ${overlay.phase}`);
        visible(html, overlay.run, "lineage occurrence run");
        visible(html, overlay.actor, "lineage occurrence actor");
        for (const value of reading?.values ?? []) {
          visible(html, `${value.subject}.${value.attribute}`, "selected lineage property");
          visible(html, displayValue(value.actual), "selected lineage actual value");
          for (const source of value.sources) {
            visible(html, source.oracle, "scoped lineage observation source");
            visible(html, source.sha256, "scoped source digest");
          }
        }
        if (overlay.phase === "preparation")
          expect(graph.nodes.some((node) => node.status === "READY")).toBe(false);
        const blockers = buildLineageBlockers(graph.lineage!);
        expect(blockers.scope).toMatchObject({
          branch: overlay.branch,
          run: overlay.run,
          step: overlay.step,
          actor: overlay.actor,
          action_index: overlay.action_index,
          phase: overlay.phase,
        });
        if (overlay.diagnostic) {
          expect(blockers.status).toBe("DIAGNOSTIC");
          expect(blockers.issues.length).toBeGreaterThan(0);
          visible(html, blockers.reason, "evidence blockage remains explicit");
        }
        blockerReports.push({
          ...blockers.scope,
          status: blockers.status,
          reason: blockers.reason,
          issues: blockers.issues,
        });
        lineageScopeReports.push({
          run: run.trace.run,
          branch: overlay.branch,
          step: overlay.step,
          action_index: overlay.action_index,
          actor: overlay.actor,
          phase: overlay.phase,
          readback_index: overlay.readback_index,
          readback_phase: overlay.readback_phase,
          diagnostic: overlay.diagnostic ?? false,
          evidence_status: overlay.evidence_status,
          nodes: overlay.nodes.length,
        });
      }
      let stateSelection = selectRun(run.trace.run);
      stateSelection = reduce(stateSelection, { type: "canvas", value: "states" });
      const expectedGraph = graphFor(snapshot, stateSelection, branchOptions(snapshot));
      const stateGraph = graphFor(snapshot, stateSelection, branchOptions(snapshot), run);
      expect(stateGraph.stateOverlay).toBeDefined();
      expect(stateGraph.edges.filter((edge) => !edge.actual)).toEqual(expectedGraph.edges);
      const actualEdges = stateGraph.edges.filter((edge) => edge.kind === "actual_transition");
      expect(actualEdges.map((edge) => edge.actual)).toEqual(stateGraph.stateOverlay!.occurrences);
      const sourceActions = facts.flatMap((fact) =>
        fact.seq > 0
          ? (fact.business_expectations ?? [])
              .filter(
                (item) =>
                  !stateSelection.flow ||
                  !item.decision.frozen?.flow ||
                  item.decision.frozen.flow === stateSelection.flow,
              )
              .map((item) => ({ fact, item }))
          : [],
      );
      const recordedStates = sourceActions.filter(
        ({ fact, item }) =>
          run.evidence_status === "PASS" &&
          fact.evidence_status === "PASS" &&
          item.decision.frozen?.before.state &&
          item.after?.state &&
          item.comparison,
      );
      for (const { fact, item } of recordedStates) {
        const matching = actualEdges.filter(
          (edge) =>
            edge.actual?.step === String(fact.seq) &&
            edge.actual.actor === item.actor &&
            edge.actual.action_index === item.action_index,
        );
        expect(matching, JSON.stringify(stateGraph.stateOverlay!.diagnostics)).toHaveLength(1);
      }
      for (const edge of actualEdges) {
        const actual = edge.actual!;
        const sources = sourceActions.filter(
          ({ fact, item }) =>
            String(fact.seq) === actual.step &&
            item.actor === actual.actor &&
            item.action_index === actual.action_index,
        );
        expect(sources, "Actual is backed by one scoped action in this run").toHaveLength(1);
        const { fact, item } = sources[0];
        const frozen = item.decision.frozen!;
        expect(actual).toMatchObject({
          run: run.trace.run,
          step: String(fact.seq),
          actor: item.actor,
          action_index: item.action_index,
          branch: frozen.branch,
          action: frozen.action,
          from_state: frozen.before.state,
          to_state: item.after!.state,
          evidence_sha256: fact.evidence_sha256,
        });
        expect(actual.occurrence).toBe(
          JSON.stringify([
            run.trace.run,
            fact.seq,
            item.actor,
            item.action_index,
            "before",
            frozen.branch,
            null,
          ]),
        );
        expect(stateGraph.nodes.find((node) => node.id === edge.from)?.subject).toBe(
          frozen.before.state,
        );
        expect(stateGraph.nodes.find((node) => node.id === edge.to)?.subject).toBe(
          item.after!.state,
        );
        if (fact.verdict === "PASS") expect(actual.verdict).toBe(item.comparison!.status);
        if (item.comparison!.status === "FAIL") expect(actual.verdict).toBe("FAIL");
        const selected = reduce(stateSelection, { type: "observed", value: actual });
        expect(selected).toMatchObject({
          model: snapshot.meta.graph,
          environment: run.environment,
          run: run.trace.run,
          step: String(fact.seq),
          actor: item.actor,
          branch: frozen.branch,
          node: frozen.action,
          occurrence: actual.occurrence,
          canvas: "states",
          dependencyScope: "branch",
        });
        const actualHTML = render(selected, `Actual state path ${run.trace.run}`);
        visible(actualHTML, "本次运行的实际状态路径", "observed state path inspector");
        visible(actualHTML, frozen.sha256, "selected immutable Expected");
        visible(
          actualHTML,
          `${frozen.before.state} → ${item.after!.state}`,
          "independent Actual path",
        );
        visible(actualHTML, `${run.trace.run} / 第 ${fact.seq} 步`, "selected step identity");
        observedSelections++;
        for (const entity of fact.before_entities ?? []) {
          if (frozen.before.bindings?.[entity.alias] !== entity.id) continue;
          const key = entityKey(entity);
          expect(actual.entity_keys).toContain(key);
          const scoped = reduce(selected, { type: "entity", value: key });
          const located = reduce(scoped, { type: "observed", value: actual });
          expect(located.entity, "an observed edge preserves full entity identity").toBe(key);
          expect(located.occurrence).toBe(actual.occurrence);
          const graph = graphFor(snapshot, located, branchOptions(snapshot), run);
          expect(graph.stateOverlay!.occurrences.some((item) => item.id === actual.id)).toBe(true);
          observedEntitySelections++;
        }
        for (const other of snapshot.runs.filter((item) => item.trace.run !== run.trace.run)) {
          const otherSelection = selectRun(other.trace.run);
          expect(reduce(otherSelection, { type: "observed", value: actual })).toEqual(
            otherSelection,
          );
        }
      }
      const blockedPreparation = preparations.some(
        (preparation) => preparation.status === "BLOCKED",
      );
      if (blockedPreparation && !facts.some((fact) => fact.seq > 0)) {
        expect(actualEdges).toEqual([]);
        expect(stateGraph.nodes.some((node) => node.actual)).toBe(false);
        expect(stateGraph.stateOverlay!.diagnostics.length).toBeGreaterThan(0);
        const diagnosticHTML = render(stateSelection, `Blocked target ${run.trace.run}`);
        for (const diagnostic of stateGraph.stateOverlay!.diagnostics)
          visible(
            diagnosticHTML,
            diagnostic.reason,
            "blocked preparation creates no target Actual",
          );
        blockedPreparationsWithoutTarget++;
      }
      if (snapshot.business_lineage?.branches.length) {
        const globalSelection = reduce(stateSelection, { type: "dependencyScope", value: "model" });
        const globalGraph = graphFor(snapshot, globalSelection, branchOptions(snapshot), run);
        expect(globalGraph).toEqual(sharedMap.graph);
        expect(globalGraph.stateOverlay).toBeUndefined();
        expect(globalGraph.nodes.some((node) => node.actual || node.reading)).toBe(false);
        const globalHTML = render(globalSelection);
        visible(globalHTML, "未叠加任何运行结果", "global declaration view with a selected run");
      }
      statePathReports.push({
        run: run.trace.run,
        source_state_observations: recordedStates.length,
        actual_edges: actualEdges.length,
        expected_edges_unchanged: true,
        occurrences: stateGraph.stateOverlay!.occurrences,
        diagnostics: stateGraph.stateOverlay!.diagnostics,
        blocked_preparation: blockedPreparation,
      });
      if (run.trace.recovery?.mode === "checkpoint") {
        visible(html, "现场检查点", "checkpoint boundary");
        visible(html, "不表示环境已恢复", "checkpoint is not restore");
      }
      runReports.push({
        run: run.trace.run,
        environment: run.environment,
        round: run.trace.round,
        runner: run.runner_id,
        evidence_status: run.evidence_status ?? "LEGACY",
        action_steps: facts.filter((fact) => fact.seq > 0).length,
        preparation_records: preparations.length,
        recovery_mode: run.trace.recovery?.mode ?? "LEGACY_UNRECORDED",
      });
    }

    for (const campaign of snapshot.business_campaigns ?? []) {
      const state = reduce(initialSelection(snapshot.meta.graph), {
        type: "campaign",
        value: campaign.id,
      });
      expect(state).toMatchObject({
        campaign: campaign.id,
        environment: campaign.environment,
        flow: campaign.flow,
        run: "",
        actor: "",
        entity: "",
      });
      const html = render(state, `Campaign ${campaign.id}`);
      visible(html, campaign.id, "campaign identity");
      if (isCampaignDiagnostic(campaign)) {
        visible(html, campaign.reason, "checked ledger corruption diagnostic");
        visible(html, "预算消耗和覆盖结论均不可用", "withheld evidence is not zero-value success");
        campaignReports.push({
          campaign: campaign.id,
          evidence_status: campaign.evidence_status,
          status: campaign.status,
          details: "WITHHELD_BY_CHECKED_LOADER",
        });
        continue;
      }
      visible(html, campaign.status, "execution stop reason");
      visible(html, "全业务覆盖标记", "coverage scope boundary");
      visible(html, "已选变体覆盖不升级为全业务证明", "selected scope is not product E2E");
      if (campaign.last_actual) {
        visible(html, "最近目标动作的实际后态", "target After is not live current state");
        observationVisible(html, campaign.last_actual);
      }
      for (const candidate of campaign.frontier ?? []) {
        const selected = reduce(state, { type: "variant", value: candidate.variant_id });
        expect(selected).toMatchObject({
          variant: candidate.variant_id,
          branch: candidate.branch,
          run: "",
          actor: "",
          entity: "",
        });
        const candidateHTML = render(selected);
        visible(candidateHTML, candidate.eligibility, "recorded candidate eligibility");
        visible(candidateHTML, candidate.reason, "candidate eligibility reason");
        observationVisible(candidateHTML, candidate.before);
        if (candidate.preparation) {
          visible(candidateHTML, "这是预计准备路径", "preparation plan is not execution");
          observationVisible(candidateHTML, candidate.preparation_before);
        }
        if (candidate.input) valuesVisible(candidateHTML, candidate.input.values ?? undefined);
      }
      let previous = state;
      for (const visit of campaign.visits ?? []) {
        visible(html, visit.run, "persisted child-run link");
        visible(html, visit.runner, "distinct child runner variant");
        if (campaign.dispatches?.length)
          expect(
            campaign.dispatches.some(
              (intent) =>
                intent.sequence === visit.sequence &&
                intent.run === visit.run &&
                intent.sha256 === visit.dispatch_sha256,
            ),
            "completed visits must retain their explicit pre-action intent reference",
          ).toBe(true);
        const run = snapshot.runs.find((item) => item.trace.run === visit.run);
        const selected = reduce(previous, { type: "visit", value: visit.run });
        if (!run) {
          unexportedChildren++;
          visible(
            html,
            "子运行未导出或范围身份不匹配",
            "missing child export, not a synthesized run",
          );
          continue;
        }
        expect(selected).toMatchObject({
          campaign: campaign.id,
          variant: visit.variant_id,
          run: visit.run,
          actor: "",
          entity: "",
        });
        expect(run.environment).toBe(campaign.environment);
        expect(run.trace.round).toBe(campaign.round);
        if (run.evidence_status === "PASS") expect(run.runner_id).toBe(visit.runner);
        linkedChildren++;
        const childHTML = render(selected, `Campaign ${campaign.id} / Child ${visit.run}`);
        visible(childHTML, visit.run, "selected child identity");
        visible(childHTML, "所选子运行目标动作的实际前态", "child target prestate");
        visible(childHTML, "所选子运行目标动作的实际后态", "child target After");
        observationVisible(childHTML, visit.before);
        observationVisible(childHTML, visit.after);
        if (visit.recovery?.mode === "restore")
          visible(childHTML, "恢复边界", "restore breaks assumed entity continuity");
        previous = selected;
        const step = context.runs
          .find((item) => item.id === visit.run)
          ?.steps.find((item) => item.id === selected.step);
        if (step?.actors[0]) previous = reduce(previous, { type: "actor", value: step.actors[0] });
        if (step?.entities[0])
          previous = reduce(previous, { type: "entity", value: step.entities[0] });
      }
      for (const intent of campaign.dispatches ?? [])
        if (!(campaign.visits ?? []).some((visit) => visit.run === intent.run))
          expect(reduce(state, { type: "visit", value: intent.run })).toEqual(state);
      campaignReports.push({
        campaign: campaign.id,
        environment: campaign.environment,
        round: campaign.round,
        status: campaign.status,
        evidence_status: campaign.evidence_status,
        variants: campaign.variants?.length ?? 0,
        child_visits: campaign.visits?.length ?? 0,
        pre_action_intents: campaign.dispatches?.length ?? 0,
        selected_variants_covered: campaign.coverage.selected_variants_covered,
        full_business_coverage: campaign.coverage.full_business_coverage,
      });
    }
    const assetDirectory = join(directory, "assets");
    const css = existsSync(assetDirectory)
      ? readdirSync(assetDirectory)
          .filter((name) => name.endsWith(".css"))
          .map((name) => `<link rel="stylesheet" href="./assets/${escaped(name)}">`)
          .join("")
      : "";
    const sections = [...pages.entries()]
      .map(
        ([title, html], index) =>
          `<section id="validation-${index + 1}"><h2>${escaped(title)}</h2>${html}</section>`,
      )
      .join("\n");
    writeFileSync(
      ssrPath,
      `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Arc Go 导出 → SSR 集成验证</title>${css}</head><body><p>当前生产解析器的 SSR 结构输出；不是浏览器交互验收，不执行目标系统、不重新判定业务。</p>${sections}</body></html>`,
    );
    const builtAssets = existsSync("dist/assets") ? readdirSync("dist/assets") : [];
    const assetsMatch =
      builtAssets.length && existsSync(assetDirectory)
        ? builtAssets.every(
            (name) =>
              existsSync(join(assetDirectory, name)) &&
              readFileSync(join("dist/assets", name)).equals(
                readFileSync(join(assetDirectory, name)),
              ),
          )
        : null;
    writeFileSync(
      reportPath,
      JSON.stringify(
        {
          version: 1,
          status: "PASS",
          scope,
          validated_at: new Date().toISOString(),
          snapshot_sha256: createHash("sha256").update(source).digest("hex"),
          graph: snapshot.meta.graph,
          parser: "PASS",
          unified_selection: "PASS",
          ssr: "PASS",
          browser: "NOT_RUN",
          raw_revalidated_by_frontend: false,
          rendered_views: renderedViews,
          representative_ssr_views: pages.size,
          assets_match_current_dist: assetsMatch,
          genealogy: {
            model_only: snapshot.runs.length === 0,
            shared_model: sharedMapReport,
            branches: lineageReports,
            observed_occurrences: lineageScopeReports,
            blocker_diagnostics: blockerReports,
            business_execution_claim: "NOT_INFERRED_FROM_FRONTEND_VALIDATION",
          },
          state_paths: {
            status: statePathReports.length ? "EXERCISED" : "NOT_PRESENT",
            runs: statePathReports,
            observed_selections: observedSelections,
            exact_entity_selections: observedEntitySelections,
            blocked_preparations_without_target: blockedPreparationsWithoutTarget,
          },
          preparation: {
            status: preparationRuns ? "EXERCISED" : "NOT_PRESENT",
            runs: preparationRuns,
            actual_actions: preparationActions,
            independent_readbacks: preparationReadbacks,
          },
          expected_actual: {
            status: frozenComparisons ? "EXERCISED" : "NOT_PRESENT",
            frozen_decisions: frozenComparisons,
            unassociated_step_views: unassociatedStepViews,
            request_captures: requestCaptures,
          },
          entity_selection: {
            status: entitySelections ? "EXERCISED" : "NOT_PRESENT",
            selections: entitySelections,
          },
          campaign: {
            status: campaignReports.length ? "EXERCISED" : "NOT_PRESENT",
            linked_children: linkedChildren,
            unexported_children: unexportedChildren,
          },
          runs: runReports,
          campaigns: campaignReports,
        },
        null,
        2,
      ),
    );
  } catch (error) {
    writeFileSync(
      reportPath,
      JSON.stringify(
        {
          version: 1,
          status: "FAIL",
          scope,
          stage: error instanceof SnapshotError ? error.stage : "integration.assertion",
          reason: "See test output; no successful integration result is recorded.",
        },
        null,
        2,
      ),
    );
    writeFileSync(
      ssrPath,
      '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>SSR 验证失败</title><p>本次集成验证失败，不保留先前成功页面作为本次证据。请查看测试输出。</p></html>',
    );
    throw error;
  }
}

const paths = exportPaths();
describe("optional real Go export -> production reader -> SSR integration", () => {
  if (!paths.length)
    it.skip("SKIP: set ARC_WORKBENCH_SNAPSHOT_PATH; no real export is supplied and no synthetic fallback is used", () =>
      undefined);
  for (const path of paths)
    it(`validates supplied export ${path}`, () => validateExport(path), 30000);
});
