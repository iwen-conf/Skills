import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App.tsx";
import type { AppState } from "./App.tsx";
import { SnapshotError, parseSnapshot, readSnapshotText } from "./snapshot";

async function load(): Promise<AppState> {
  const embedded = document.getElementById("arc-snapshot");
  try {
    if (!embedded && import.meta.env.DEV) {
      const sample = await import("../dev/sample-snapshot.json");
      return { snapshot: parseSnapshot(JSON.parse(JSON.stringify(sample.default))) };
    }
    return { snapshot: readSnapshotText(embedded?.textContent ?? null) };
  } catch (error) {
    return {
      error:
        error instanceof SnapshotError
          ? error
          : new SnapshotError(
              "snapshot.load",
              error instanceof Error ? error.message : String(error),
            ),
    };
  }
}

void load().then((state) => {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App state={state} />
    </StrictMode>,
  );
});
