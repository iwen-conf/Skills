# tester-army/e2e: execute a bounded UI journey

Adapted from the upstream [skill](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/skills/e2e/SKILL.md)
and implementation at `232ce75ab4b68ded93525162e1121bf973792ea8`, checked 2026-11-07,
[Apache-2.0](upstream/tester-army-e2e-LICENSE); [upstream NOTICE](upstream/tester-army-e2e-NOTICE) retained.
Reviewed versions: `e2e` 0.23.0, `@e2e-dev/web` 0.15.5, `@e2e-dev/mobile` 0.12.6;
Node `^22.22.3 || >=24.8.0`. Pre-1.0 minor releases can change API/config.

This is a client execution reference within the current verify mode. Whole-system requests
stay graph mode; use the selected card's RULE, actor, conditions and oracle. The SDK can execute
actions, but cannot approve rules or supply independent evidence by declaring success.

## 1. Bind the installed project

- Locate the actual `e2e.config.ts/.mts/.js/.mjs`, `tests` globs, package/lock versions and existing
  scripts with code-intel. Prefer the project's entry; `vpx e2e` below assumes its declared local
  CLI is installed. Do not fetch the latest package or add a framework to this Skills repository.
- Read the matching installed guide with `vpx e2e guide writing-tests`, `running`, `agent` or
  `debugging`; full docs ship in `node_modules/e2e/docs`. Recheck changed APIs against the actual
  installed version. Use the project's Vite+/bun workflow for authorized setup.
- Reuse the approved app/environment, deployed build, account aliases and fixture recipes.
  `targets[].engine` and `targets[].app` name the real target; there is no top-level `app` key.
  Start through [web-e2e](web-e2e.md) or the existing app command, with one owner and readiness
  checks; the CLI/config must not launch the same app twice.
- Locator-only tests need no model: do not acquire `agent`. Built-in agent steps need an explicitly bound
  `agents.<name>.model` and its authorized provider credentials. No model is inferred from an
  `E2E_MODEL` variable; acquiring the built-in agent without one stops with `MODEL_UNAVAILABLE`.
  This SDK configuration belongs to the executor, not a new Arc model service or handshake.

## 2. Write while observing; check after each goal

1. Start on the screen under test. Prepare unrelated data through the existing API/fixtures;
   cover the creation UI only when creation is the claim. Read back setup as the intended actor.
2. Capture `screen.tree()`, `app.screenshot()` and the observed accessible names before acting.
   Add each exact step and its contract-derived assertion as the path is explored. Live inspection
   produces a test candidate; execute the saved test before claiming scripted coverage.
3. Use exact `screen` actions for required input values, rejection attempts and guarded transitions.
   Use `agent.act` for one bounded goal, with real values in `params`, followed immediately by a
   deterministic locator assertion. Two independently checked outcomes require two goals.
4. Check the durable/external result through the project's separate API/DB/message observer,
   forbidden effects absent, then reload/reopen and check again ([web-e2e §4](web-e2e.md#4-journeys-and-oracles)).
   `agent.assert`, `agent.waitFor`, `agent.extract` and an action summary are observations, not
   independent oracles. Exact UI assertions also remain necessary.

The following is only the UI half of an illustrative cart journey; bind the actual project's
route, names, expected values and independent readback before it can prove a business RULE:

```ts
import { test } from '@e2e-dev/web';
import { expect } from 'e2e';

test('adds one item to the cart', async ({ app, agent, screen }) => {
  await app.open('/cart');
  await expect(screen.getByRole('heading', 'Cart')).toBeVisible();
  await agent.act('add one item to the cart');
  await expect(screen.getByRole('status')).toHaveText('1 item');
});
```

- Import platform fixtures from `@e2e-dev/web` (`browser`) or `@e2e-dev/mobile` (`device`);
  `expect`, `credentials` and `secrets` come from `e2e`. Exact actions include `tap`, `fill`,
  `doubleTap`; locator matchers poll with a deadline. Reads such as `count()`/`isVisible()` do not
  wait for a value to settle. No fixed sleep or timeout increase to hide a failure.
- Names/text match the whole normalized string, case-sensitive by default, unlike Playwright.
  Two matches are ambiguous: scope to the observed container, never pick `first()` to hide it.
  Before a negative assertion prove the screen, actor and relevant control are readable using
  positive checks. Zero matches, an unsupported role or a cold/blank/crashed mobile screen can
  otherwise pass an absence assertion. Independently verify the server rejected the operation.
- `test.extend` can own setup/teardown. Use unique run data and verify both; a fresh browser context
  or saved session resets no server data. Web `test.setup` saves a role-specific session only after
  successful sign-in is asserted. Mobile does not support Web saved sessions.
- Start with one worker and zero retries. Inspect test/group overrides, which can override CLI
  retries/trace/video. Parallelize only when each worker/actor has independent data, sessions and
  a verified cleanup path; `serial` shares state and does not establish business isolation.

## 3. Select and run once; retain every attempt

Set `E2E_CONFIG`, `E2E_TARGET`, `E2E_CASE` (actual `file:line`) and `E2E_OUTPUT` from the bound
project/card. The output must be a new directory under `.arc/artifacts/<task-slug>/logs/` for
this run/attempt. After editing, recheck the test's registration line so it selects the same case.
Use the card's bounded execution deadline and the project's configured action/assertion budgets.

```bash
E2E_TELEMETRY_DISABLED=1 vpx e2e list "$E2E_CASE" \
  --config "$E2E_CONFIG" --target "$E2E_TARGET" --reporter json

E2E_TELEMETRY_DISABLED=1 vpx e2e run "$E2E_CASE" \
  --config "$E2E_CONFIG" --target "$E2E_TARGET" \
  --workers 1 --retries 0 --no-cache --trace on \
  --reporter list,junit,markdown --output "$E2E_OUTPUT"
```

- `list` starts no app, engine or workers, but imports project config/tests: it is not a sandbox.
  Compare its `pairs[]`/`disposition` (`run`, `skip`, `filtered`) with planned cases/targets/actors.
  Selection flags can omit pairs completely; compare against the card, not just this list.
  `test.only`, `test.skip`, `test.fail`, capability skips and AI filters never remove a required
  obligation or turn a known business failure into acceptance.
- `run` writes `report.json` and per-test `results/<test>/trace.md`, then exits. Use this one-shot
  entry rather than `init`, watch, interactive debug or a report/MCP service for card execution.
  Plain `e2e explore`/live MCP findings stay candidates until a reproducer and independent oracle run.
- Save CLI command/exit, selection, config/build/engine/model identity, all results/attempts,
  step/cache outcomes, failure screen, screenshots and application/console/network/cleanup logs.
  Use required trace settings on tests too; a test-level override can remove the CLI trace.
- Never reuse default `.e2e/results`: a new run clears results, while a preflight failure can leave
  the old report. Check artifact creation and identity for this attempt; a timestamp or file's
  presence alone cannot bind it to the candidate, account or environment. Preserve evidence before
  another run and let the existing adapter reference it by hash in RAW.

## 4. Cache and healing are not acceptance gates

- Establish a fresh `--no-cache` run on a verified fixture. It disables replay, not agent/model
  calls. Model/config changes require fresh execution evidence, not a cached success.
- A later locator matcher can confirm an action recording; plain-value `expect` and `expect.poll`
  do not. Even a verified recording is just a replay recipe, not durable-state or provenance proof.
- For authorized comparable replay, `--strict-cache` prevents live handoff of an existing stale
  recording. It does not prohibit all model calls: new steps, gaps, truncated recordings or store
  read errors can still run live; `agent.assert`/`waitFor`/`extract` always do. There is no global
  `--no-healing` flag. Audit actual step outcomes and model calls.
- A stale replay may already have executed a prefix of writes. Read actual state before retry;
  retain the first contracted deviation as FAIL and recover only in a linked new task/run.
  Cache repair/healing cannot rewrite a RULE, erase a failure or certify recovered state. Prefer
  exact steps when acceptance must stop at a guard or the first forbidden action.
- Keep cache, environment and actor namespaces separate. Cache identity normalizes dynamic
  route IDs and does not include every model/engine-version change. Validate substituted IDs and
  ownership through the oracle; `unique()`/`app.identity` cannot establish environment isolation.
  Treat unsigned cache input as executable test input; use only reviewed, controlled recordings.

## 5. Mobile and secret boundaries

- Use the existing target-native runner or an explicitly configured mobile engine/device provider.
  Follow [android-execution](_shared/docs/android-execution.md) for the project's remote device;
  no local emulator installation/start follows from choosing this SDK.
- Upstream exposes `DeviceProvider.acquire/release` and a remote `DeviceLease.daemon`
  (`baseUrl`, `authToken`). This is an integration point, not verified redroid compatibility.
  Missing supported provider/device makes that mobile claim BLOCKED; browser evidence is separate.
- Verify app/package/build, device lease and actor before every attempt. `app.open()` relaunches
  the pinned mobile app; omitting it can inherit the previous test's screen. Relaunching does not
  restore server state, network or fold settings. Check independent recovery and release results.
- Passwords use `credentials.user(alias).password`; other secrets use `secrets.get(alias)`.
  Pass opaque handles to `fill`/`agent.act` params; values stay in credential references, never
  instructions, test code or RAW. Reuse an approved model/data boundary for screenshots and trees.
- Upstream has no origin allowlist: an agent can follow an external link and fill a step secret
  there. Bind the authorized destination through existing network/target controls and exact
  actions for secret-bearing steps. Disabling telemetry does not restrict model input or actions.
- Secure input masking does not redact all screenshots; video is unmasked, app command logs are
  raw, and ordinary typed inputs enter the cache. Keep these local/restricted, inspect and redact
  before sharing; no automatic PR-comment/artifact upload integration follows from this guide.

## 6. Judge the card, then repair from evidence

| Observation | Arc verdict/action |
|---|---|
| All planned cases ran; contract assertions, independent readback and recovery hold | PASS for those claims only |
| Independently observed contracted deviation | FAIL; retain first RAW/attempt and repair packet |
| Pass on retry or inconsistent comparable attempts | FLAKY; retain every attempt, no replacement PASS |
| Required case skipped, filtered or excluded | NOT_RUN with reason |
| Contract violation accepted by `test.fail` | FAIL when independently confirmed; preserve the violation |
| Missing/stale report, engine/model/account/oracle/cleanup failure, unknown state | BLOCKED with evidence; independent claims may continue |

The CLI's exit 0 includes `passed`, `flaky` and `skipped`. Inspect the complete report, run errors,
attempts and missing obligations. Classify harness errors separately; an independently observed
contracted liveness breach can be FAIL, while an observer timeout alone is BLOCKED.

Read the failure block (expected/observed, line, URL/app/screen, nearby nodes), trace and logs
before driving the app again. Check all remaining steps that the evidence covers; repair a locator
only from observed semantics, and change expectations only for an explicitly changed contract.
After independently reconstructing the initial state, rerun the selected case and affected sibling
claims. A passed diagnostic/healed run does not replace the first FAIL.

For graph work the existing command adapter invokes the CLI and independent readback; it must
bind task/round/build/config/env/actor/fixture and attach actual step observations in the existing
RAW contract ([v2 execution](business-graph-v2.md)). `report.json` is not a runner manifest or
automatically valid RAW. This guide adds no adapter or trusted Runner implementation; `finish`,
`trace` and `gate` cannot prove an unexecuted UI/mobile journey.

Sources at the reviewed commit: [test semantics](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/skills/e2e/references/writing-tests.md),
[CLI](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/docs/reference/cli.mdx),
[output/config](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/docs/reference/config.mdx),
[cache](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/docs/cache.mdx),
[exit codes](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/packages/e2e/src/cli/exit-codes.ts),
[mobile](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/docs/mobile.mdx),
[security](https://github.com/tester-army/e2e/blob/232ce75ab4b68ded93525162e1121bf973792ea8/docs/security.mdx).
