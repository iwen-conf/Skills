import react from "@vitejs/plugin-react";
import { defineConfig, lazyPlugins } from "vite-plus";
import type { Plugin } from "vite-plus";

// Browsers refuse module scripts from file:// (opaque origin), so the static snapshot
// ships one classic script; the CLI embeds the data and a CSP of default-src 'none'.
function classicScript(): Plugin {
  return {
    name: "arc-classic-script",
    apply: "build",
    transformIndexHtml: (html) =>
      html
        .replace(/<script type="module" crossorigin src=/g, "<script defer src=")
        .replace(/ crossorigin/g, ""),
  };
}

// https://vite.dev/config/
export default defineConfig({
  base: "./",
  build: {
    modulePreload: false,
    cssCodeSplit: false,
    rollupOptions: { output: { format: "iife" } },
  },
  fmt: {},
  lint: {
    plugins: ["react", "typescript", "oxc"],
    rules: {
      "react/rules-of-hooks": "error",
      "react/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
        },
      ],
      "vite-plus/prefer-vite-plus-imports": "error",
    },
    options: {
      typeAware: true,
      typeCheck: true,
    },
    jsPlugins: [
      {
        name: "vite-plus",
        specifier: "vite-plus/oxlint-plugin",
      },
    ],
  },
  plugins: lazyPlugins(() => [react(), classicScript()]),
});
