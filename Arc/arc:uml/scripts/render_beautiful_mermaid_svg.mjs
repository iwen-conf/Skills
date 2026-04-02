#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

const DEFAULT_VERSION = process.env.BEAUTIFUL_MERMAID_VERSION || "1.1.3";
const DEFAULT_CACHE_DIR = path.join(
  process.env.XDG_CACHE_HOME || path.join(os.homedir(), ".cache"),
  "arc:uml",
  `beautiful-mermaid-${DEFAULT_VERSION}`,
);

const STRING_OPTIONS = new Map([
  ["--bg", "bg"],
  ["--fg", "fg"],
  ["--line", "line"],
  ["--accent", "accent"],
  ["--muted", "muted"],
  ["--surface", "surface"],
  ["--border", "border"],
  ["--font", "font"],
  ["--theme", "theme"],
]);

const NUMBER_OPTIONS = new Map([
  ["--padding", "padding"],
  ["--node-spacing", "nodeSpacing"],
  ["--layer-spacing", "layerSpacing"],
  ["--component-spacing", "componentSpacing"],
]);

const SUPPORTED_PREFIXES = [
  "graph",
  "flowchart",
  "stateDiagram-v2",
  "sequenceDiagram",
  "classDiagram",
  "erDiagram",
  "xychart-beta",
];

function printUsage() {
  console.log(`Usage:
  node Arc/arc:uml/scripts/render_beautiful_mermaid_svg.mjs --input <file.mmd> [--output <file.svg>]
  node Arc/arc:uml/scripts/render_beautiful_mermaid_svg.mjs --input-dir <dir> [--output-dir <dir>] [--skip-unsupported]

Options:
  --theme <name>                beautiful-mermaid preset theme name
  --bg/--fg/--line/--accent     Override renderer colors
  --muted/--surface/--border    Override secondary renderer colors
  --font <family>               Override font family
  --padding <px>                Canvas padding
  --node-spacing <px>           Horizontal sibling spacing
  --layer-spacing <px>          Vertical layer spacing
  --component-spacing <px>      Spacing between disconnected components
  --transparent                 Render transparent SVG background
  --cache-dir <dir>             Override local package cache directory
  --skip-unsupported            Skip Mermaid dialects unsupported by beautiful-mermaid
  --help                        Show this message`);
}

function parseArgs(argv) {
  const parsed = {
    cacheDir: DEFAULT_CACHE_DIR,
    output: null,
    outputDir: null,
    input: null,
    inputDir: null,
    renderOptions: {},
    skipUnsupported: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];

    const nextValue = () => {
      index += 1;
      if (index >= argv.length) {
        throw new Error(`Missing value for ${arg}`);
      }
      return argv[index];
    };

    if (arg === "--help" || arg === "-h") {
      printUsage();
      process.exit(0);
    }
    if (arg === "--input") {
      parsed.input = nextValue();
      continue;
    }
    if (arg === "--output") {
      parsed.output = nextValue();
      continue;
    }
    if (arg === "--input-dir") {
      parsed.inputDir = nextValue();
      continue;
    }
    if (arg === "--output-dir") {
      parsed.outputDir = nextValue();
      continue;
    }
    if (arg === "--cache-dir") {
      parsed.cacheDir = nextValue();
      continue;
    }
    if (arg === "--skip-unsupported") {
      parsed.skipUnsupported = true;
      continue;
    }
    if (arg === "--transparent") {
      parsed.renderOptions.transparent = true;
      continue;
    }
    if (STRING_OPTIONS.has(arg)) {
      parsed.renderOptions[STRING_OPTIONS.get(arg)] = nextValue();
      continue;
    }
    if (NUMBER_OPTIONS.has(arg)) {
      const value = Number(nextValue());
      if (Number.isNaN(value)) {
        throw new Error(`Expected a number for ${arg}`);
      }
      parsed.renderOptions[NUMBER_OPTIONS.get(arg)] = value;
      continue;
    }

    throw new Error(`Unknown argument: ${arg}`);
  }

  if (Boolean(parsed.input) === Boolean(parsed.inputDir)) {
    throw new Error("Provide exactly one of --input or --input-dir");
  }
  if (parsed.input && parsed.outputDir) {
    throw new Error("--output-dir can only be used with --input-dir");
  }
  if (parsed.inputDir && parsed.output) {
    throw new Error("--output can only be used with --input");
  }

  return parsed;
}

async function pathExists(target) {
  try {
    await fs.access(target);
    return true;
  } catch {
    return false;
  }
}

async function collectMmdFiles(rootDir) {
  const entries = await fs.readdir(rootDir, { withFileTypes: true });
  const files = [];

  for (const entry of entries.sort((left, right) => left.name.localeCompare(right.name))) {
    const target = path.join(rootDir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectMmdFiles(target)));
      continue;
    }
    if (entry.isFile() && target.endsWith(".mmd")) {
      files.push(target);
    }
  }

  return files;
}

function detectHeader(source) {
  for (const rawLine of source.split(/\r?\n/u)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("%%")) {
      continue;
    }
    return line;
  }
  return "";
}

function isSupportedDialect(source) {
  const header = detectHeader(source);
  return SUPPORTED_PREFIXES.some((prefix) => header.startsWith(prefix));
}

function resolveOutputPath(inputPath, outputPath, outputDir, inputDir) {
  if (outputPath) {
    return path.resolve(outputPath);
  }

  if (outputDir && inputDir) {
    const relative = path.relative(path.resolve(inputDir), path.resolve(inputPath));
    return path.join(path.resolve(outputDir), relative.replace(/\.mmd$/u, ".svg"));
  }

  return path.resolve(inputPath.replace(/\.mmd$/u, ".svg"));
}

function installBeautifulMermaid(cacheDir) {
  const result = spawnSync(
    "npm",
    [
      "install",
      "--no-save",
      "--ignore-scripts",
      "--fund=false",
      "--audit=false",
      `beautiful-mermaid@${DEFAULT_VERSION}`,
    ],
    {
      cwd: cacheDir,
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  if (result.status !== 0) {
    throw new Error(
      [
        `Failed to install beautiful-mermaid@${DEFAULT_VERSION} into ${cacheDir}.`,
        result.stderr?.trim(),
        result.stdout?.trim(),
      ]
        .filter(Boolean)
        .join("\n"),
    );
  }
}

async function loadBeautifulMermaid(cacheDir) {
  await fs.mkdir(cacheDir, { recursive: true });
  const entry = path.join(cacheDir, "node_modules", "beautiful-mermaid", "dist", "index.js");

  if (!(await pathExists(entry))) {
    installBeautifulMermaid(cacheDir);
  }

  return import(pathToFileURL(entry).href);
}

function buildRenderOptions(moduleExports, rawOptions) {
  const options = { ...rawOptions };
  const themeName = options.theme;
  delete options.theme;

  if (!themeName) {
    return options;
  }

  const theme = moduleExports.THEMES?.[themeName];
  if (!theme) {
    const available = Object.keys(moduleExports.THEMES || {}).sort().join(", ");
    throw new Error(`Unknown beautiful-mermaid theme "${themeName}". Available themes: ${available}`);
  }

  return { ...theme, ...options };
}

function relativeToCwd(target) {
  return path.relative(process.cwd(), target) || ".";
}

async function main() {
  const parsed = parseArgs(process.argv.slice(2));
  const inputFiles = parsed.input
    ? [path.resolve(parsed.input)]
    : await collectMmdFiles(path.resolve(parsed.inputDir));

  if (inputFiles.length === 0) {
    throw new Error("No .mmd files found to render");
  }

  const workItems = [];
  const skipped = [];

  for (const inputFile of inputFiles) {
    const source = await fs.readFile(inputFile, "utf8");
    if (!isSupportedDialect(source)) {
      const header = detectHeader(source) || "<empty>";
      if (parsed.skipUnsupported) {
        skipped.push({ inputFile, reason: `unsupported Mermaid header: ${header}` });
        continue;
      }
      throw new Error(`Unsupported Mermaid header in ${inputFile}: ${header}`);
    }

    workItems.push({
      inputFile,
      outputFile: resolveOutputPath(inputFile, parsed.output, parsed.outputDir, parsed.inputDir),
      source,
    });
  }

  if (workItems.length === 0) {
    console.log("No renderable Mermaid files found.");
    for (const item of skipped) {
      console.log(`- skipped ${relativeToCwd(item.inputFile)} (${item.reason})`);
    }
    return;
  }

  const moduleExports = await loadBeautifulMermaid(parsed.cacheDir);
  const renderOptions = buildRenderOptions(moduleExports, parsed.renderOptions);
  const render = moduleExports.renderMermaidSVG;

  if (typeof render !== "function") {
    throw new Error("beautiful-mermaid did not expose renderMermaidSVG");
  }

  const rendered = [];

  for (const item of workItems) {
    const svg = render(item.source, renderOptions);
    await fs.mkdir(path.dirname(item.outputFile), { recursive: true });
    await fs.writeFile(item.outputFile, svg, "utf8");
    rendered.push(item);
  }

  console.log(`Rendered ${rendered.length} SVG file(s) with beautiful-mermaid@${DEFAULT_VERSION}.`);
  for (const item of rendered) {
    console.log(`- ${relativeToCwd(item.inputFile)} -> ${relativeToCwd(item.outputFile)}`);
  }
  for (const item of skipped) {
    console.log(`- skipped ${relativeToCwd(item.inputFile)} (${item.reason})`);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
