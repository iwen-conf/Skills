# Web E2E: start, recon, act, assert

Drive the real web app through its UI and judge the business outcome. The four UI checks and
runner commands live in [stacks.md](stacks.md); this file covers starting the app, finding
elements and writing journeys whose oracle is not the page itself.

## 1. Choose the path

| Situation | Path |
|---|---|
| Static HTML | read the file for selectors, open it via `file://`, script directly |
| Playwright config with `webServer` | run the project's suite; it starts the app |
| App not running, no `webServer` | start it through `with_server.py` (§2) |
| User-supplied running URL | recon, then script; record the URL and label the build unverified unless the deployment shows it |

## 2. Start the app

```bash
python3 <skill-dir>/scripts/with_server.py \
  --server "npm run dev -- --port 5173" --port 5173 \
  --server "cd api && go run ./cmd/api" --port 8080 --ready-url http://localhost:8080/healthz \
  --timeout 120 --log-dir .arc/artifacts/<task-slug>/logs -- npx playwright test e2e/checkout.spec.ts
```

- Ready means every port accepts connections and every `--ready-url` answers 2xx/3xx; never a
  fixed sleep. Use the project's real start commands and test-environment config.
- A port that is already listening is refused (exit 125): a server this run did not start may
  serve another build. `--reuse-running` only when the user names that server.
- Exit 125 (port taken, server exited, deadline passed) is `BLOCKED` with the server log tail,
  never a product `FAIL`. Other exit codes are the test command's own.
- Seed data through the project's API, fixtures or test tenant; never production accounts.

## 3. Recon before act

In a dynamic app never guess selectors from source alone.

1. Open the page and wait for its ready signal: a heading, a test id or the response it depends
   on (`page.waitForResponse`); `networkidle` only for apps without one.
2. Capture the accessibility tree, a screenshot, console errors and failed requests:
   `agent-browser open <url>`, `snapshot -i` (refs such as `@e3`), `screenshot <path>`,
   `console`, `errors`, `network requests`; or Playwright `locator.ariaSnapshot()`.
3. Pick locators from what was observed: role and accessible name, label, test id. Never CSS
   positions or copy fragments that change with wording.
4. Act one step at a time; snapshot again after the DOM changes (dialog, route, list reload).
5. Turn the explored path into a scripted test. The exploration itself is not evidence.

```python
from playwright.sync_api import sync_playwright  # one-off recon script

with sync_playwright() as p:
    page = p.chromium.launch().new_page()
    errors = []
    page.on("console", lambda m: errors.append(m.text) if m.type == "error" else None)
    page.goto("http://localhost:5173/cart")
    page.get_by_role("heading", name="Cart").wait_for()
    print(page.locator("main").aria_snapshot())
    page.screenshot(path=".arc/artifacts/<task-slug>/logs/cart.png", full_page=True)
    print(errors)
```

## 4. Journeys and oracles

The UI is an observer, not the oracle. Each journey asserts:

- the business end state through an independent channel: API read, test-database query, the
  captured message to an external-service double;
- the state after reload, and the forbidden effect absent: a double submit creates one order; a
  denied role sees no data and the direct API call is rejected too;
- the four UI checks on the screens involved.

Add a case when the risk applies: double click or submit; back, refresh or reopen mid-flow; one
slow or failing API (`page.route` delays or fails it) with a visible, recoverable error; session
expiry; a permission-denied deep link; both sides of every input limit; two tabs editing the same
record; locale, time zone and currency; upload limits ([API cases](api-contract-tests.md)).

```ts
type Order = { id: string; total: string };

test('VIP checkout creates exactly one order at the discounted total', async ({ page, request }) => {
  const mine = async (): Promise<Order[]> => (await (await request.get('/api/orders?mine=1')).json()).items;
  const before = await mine();
  await page.goto('/cart');
  await page.getByRole('button', { name: 'Place order' }).dblclick();
  await expect(page.getByRole('heading', { name: 'Order confirmed' })).toBeVisible();
  const created = (await mine()).filter((o) => !before.some((b) => b.id === o.id));
  expect(created).toHaveLength(1);
  expect(created[0].total).toBe('90.00'); // 100.00 list price × VIP 10% off, from the pricing rule
});
```

## 5. Playwright practice

- Web-first assertions (`await expect(locator).toBeVisible()`) wait with a deadline; never
  `page.waitForTimeout`.
- Log in once in a setup project and reuse `storageState`; one state per role for permission
  journeys.
- `npx playwright test --trace retain-on-failure`; keep `trace.zip`, screenshots, console and
  server logs under `.arc/artifacts/<task-slug>/logs/`.
- Flake check: `--repeat-each=20 --retries=0`; CI uses `--fail-on-flaky-tests`.
- `page.route` stubs only third parties outside the claim, never the API whose behavior is claimed.

## 6. Verdicts

| Observation | Verdict |
|---|---|
| Journey completes, independent oracle matches, UI checks hold | `PASS` |
| Wrong end state, duplicate effect, permission leak, failed UI check | `FAIL` + repair packet with the trace |
| `with_server.py` exit 125, browser or test account missing | `BLOCKED: <missing item>` |
| Exploration only, nothing scripted | scripted claim `NOT_RUN`; findings are leads |
