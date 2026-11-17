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
import { actorsOf, factsOf, selectionContext } from "./explorer-model";
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
      snapshot.runs.length + (snapshot.business_campaigns?.length ?? 0),
      "Provide a CLI export with actual ledger records, not a model-only or synthetic fallback",
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
