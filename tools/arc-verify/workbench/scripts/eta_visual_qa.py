"""Browser verification for the actual Arc React plan; ETA app itself is not rendered here."""
from __future__ import annotations
import json
import subprocess
import time
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent.parent
ARTIFACTS = ROOT / ".arc-visual-qa"
URL = "http://127.0.0.1:5174/eta-browser-fixture.html"
VIEWPORTS = [(1440, 900), (1024, 768), (768, 900), (767, 900), (390, 844)]


def wait_for_server(process: subprocess.Popen[bytes]) -> None:
    for _ in range(100):
        if process.poll() is not None:
            raise RuntimeError("Vite+ exited before browser fixture could start")
        try:
            with urllib.request.urlopen(URL, timeout=1) as response:
                if response.status == 200:
                    return
        except OSError:
            time.sleep(0.2)
    raise RuntimeError("Vite+ did not serve the fixture")


def main() -> None:
    ARTIFACTS.mkdir(exist_ok=True)
    server = subprocess.Popen(
        [str(ROOT / "node_modules/.bin/vp"), "dev", "--host", "127.0.0.1", "--port", "5174"],
        cwd=ROOT,
        stdout=(ARTIFACTS / "vite-server.log").open("wb"),
        stderr=subprocess.STDOUT,
    )
    measurements = []
    try:
        wait_for_server(server)
        with sync_playwright() as driver:
            browser = driver.chromium.launch(headless=True)
            try:
                for width, height in VIEWPORTS:
                    page = browser.new_page(viewport={"width": width, "height": height}, device_scale_factor=1)
                    page_errors: list[str] = []
                    page.on("pageerror", lambda error: page_errors.append(str(error)))
                    page.goto(URL, wait_until="networkidle")
                    page.locator('[data-eta-group="refund-plan"]').wait_for()
                    if width >= 768:
                        page.wait_for_function("""() =>
                            [...document.querySelectorAll('.eta-board-wide [data-eta-edge]')]
                              .every((edge) => !!edge.querySelector('path[d]'))
                        """)
                    metrics = page.evaluate("""() => {
                      const wide = document.querySelector(".eta-board-wide");
                      const mobile = document.querySelector(".eta-board-phone");
                      const shown = inner => getComputedStyle(inner).display !== "none";
                      const node = document.querySelector(shown(wide) ? ".eta-board-wide [data-plan-node]" : ".eta-board-phone [data-plan-node]");
                      const style = getComputedStyle(node);
                      const rect = node.getBoundingClientRect();
                      const columns = document.querySelector(".eta-board-columns");
                      return {
                        wideVisible: shown(wide),
                        mobileVisible: shown(mobile),
                        firstCardWidth: rect.width,
                        firstCardHeight: rect.height,
                        cardPaddingX: parseFloat(style.paddingLeft),
                        cardPaddingY: parseFloat(style.paddingTop),
                        columnGap: parseFloat(getComputedStyle(columns).columnGap),
                        squareCorners: getComputedStyle(node).borderRadius === "0px",
                        edgeCount: document.querySelectorAll(".eta-board-wire").length,
                        drawnEdgeCount: [...document.querySelectorAll('.eta-board-wide [data-eta-edge]')]
                          .filter((edge) => !!edge.querySelector('path[d]')).length,
                        horizontalOverflow: document.documentElement.scrollWidth > innerWidth + 1,
                      };
                    }""")
                    assert metrics["wideVisible"] == (width >= 768), metrics
                    assert metrics["mobileVisible"] == (width < 768), metrics
                    assert abs(metrics["cardPaddingX"] - 9) <= 1, metrics
                    assert abs(metrics["cardPaddingY"] - 7.2) <= 1, metrics
                    assert abs(metrics["columnGap"] - 36) <= 1, metrics
                    assert metrics["squareCorners"], metrics
                    if width >= 768:
                        assert 208 <= metrics["firstCardWidth"] <= 256, metrics
                    assert metrics["edgeCount"] == 2, metrics
                    if width >= 768:
                        assert metrics["drawnEdgeCount"] == 2, metrics
                    else:
                        assert not metrics["horizontalOverflow"], metrics
                    selector = '.eta-board-phone [data-plan-node="issue-refund"] button' if width < 768 else '.eta-board-wide [data-plan-node="issue-refund"] button'
                    page.locator(selector).click()
                    assert page.locator("#eta-selected-node").text_content() == "issue-refund"
                    page.locator(selector).focus()
                    page.locator(selector).press("Enter")
                    assert page.locator("#eta-selected-node").text_content() == "issue-refund"
                    assert not page_errors, "Browser runtime errors: " + "; ".join(page_errors)
                    page.screenshot(path=str(ARTIFACTS / f"arc-{width}x{height}.png"), full_page=True)
                    measurements.append({"viewport": [width, height], **metrics, "selection": "PASS"})
                    page.close()
            finally:
                browser.close()
    finally:
        server.terminate()
        try:
            server.wait(timeout=8)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
    (ARTIFACTS / "report.json").write_text(json.dumps({
        "scope": "Rendered Arc fixture in Chromium; actual running ETA visual diff NOT_EVALUATED",
        "eta_reference": "0xfe-project/eta@6b62da886b225391a157c7486e93b85aef73a17c",
        "measurements": measurements,
        "pixel_parity_against_running_eta": "NOT_EVALUATED",
        "reason": "No independently captured matched-font, matched-fixture ETA application screenshot",
    }, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    main()
