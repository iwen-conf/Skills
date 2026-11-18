import { readFileSync } from "node:fs";
import { describe, expect, it } from "vite-plus/test";

/** This guards source-derived ETA geometry, not screenshot/pixel acceptance. */
const stylesheet = readFileSync(new URL("./eta-board.css", import.meta.url), "utf8");
const workspace = readFileSync(new URL("./App.css", import.meta.url), "utf8");

describe("ETA source-derived visual geometry baseline", () => {
  it("uses ETA's 3.6px spacing unit rather than default Tailwind 4px", () => {
    expect(stylesheet).toContain("--eta-gap:36px");
    expect(stylesheet).toContain("--eta-pad:12.6px");
    expect(stylesheet).toContain("--eta-row-gap:9px");
    expect(stylesheet).toContain("--eta-card-px:9px");
    expect(stylesheet).toContain("--eta-card-py:7.2px");
    expect(stylesheet).toContain("grid-auto-columns:minmax(208px,256px)");
  });
  it("switches at ETA's 768px breakpoint and does not compress SVG wires on phones", () => {
    expect(stylesheet).toContain("@media (max-width:767px)");
    expect(stylesheet).toContain("@media (min-width:768px)");
    expect(stylesheet).toContain(".eta-board-wide { display:none; }");
    expect(stylesheet).toContain(".eta-board-phone { display:grid;");
  });
  it("enables a full-width plan while keeping business facts one click away", () => {
    expect(workspace).toContain(".workspace.eta-plan-workspace");
    expect(workspace).toContain("grid-template-columns:minmax(0,1fr)");
    expect(workspace).toContain(".eta-plan-wide-toggle");
  });
  it("does not silently import ETA's proprietary mixed font into Arc", () => {
    expect(stylesheet).toContain('"Eta Sans"');
    expect(stylesheet).not.toMatch(/url\(.*\.woff2/i);
  });
});
