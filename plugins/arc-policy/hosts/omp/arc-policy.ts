/**
 * Arc Policy Runtime for OMP / PI.
 *
 * Delegates deny/allow to hooks/hook.py so OMP uses the same engine as
 * Claude, Codex, Grok, Cursor, and DeepSeek Harness.
 *
 * Same-user extensions are process control, not a security boundary.
 */
import { spawnSync } from "node:child_process";
import { existsSync, realpathSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

type HookResult = Record<string, unknown> | null;

// One id per agent process: the engine records which repositories this session touched.
const SESSION_ID = `omp-${process.pid}-${Date.now().toString(36)}`;
// Shells the agent spawns inherit this, so the Git-level commit gate recognizes the agent.
process.env.ARC_POLICY_AGENT ??= "1";

function pluginRoot(): string {
  const here = dirname(realpathSync(fileURLToPath(import.meta.url)));
  return join(here, "..", "..");
}

/** The engine's JSON reply, null for no reply, or an Error when the engine could not answer. */
function runHook(
  payload: Record<string, unknown>,
  cwd: string,
): HookResult | Error {
  const root = pluginRoot();
  const hook = join(root, "hooks", "hook.py");
  if (!existsSync(hook)) {
    return new Error(`missing ${hook}`);
  }
  const result = spawnSync("python3", [hook], {
    input: JSON.stringify({ ...payload, session_id: SESSION_ID }),
    encoding: "utf8",
    timeout: 30_000,
    cwd,
    env: {
      ...process.env,
      PLUGIN_ROOT: root,
      OMP_PLUGIN_ROOT: root,
      CLAUDE_PLUGIN_ROOT: root,
    },
  });
  if (result.error) {
    return result.error;
  }
  if (result.status !== 0) {
    return new Error(
      `hook exited ${result.status ?? result.signal}: ${(result.stderr || "").trim()}`,
    );
  }
  const text = (result.stdout || "").trim();
  if (!text) {
    return null;
  }
  try {
    const parsed: unknown = JSON.parse(text);
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : new Error(`hook replied with non-object JSON: ${text}`);
  } catch {
    return new Error(`hook replied with invalid JSON: ${text}`);
  }
}

function reply(result: HookResult | Error): HookResult {
  return result instanceof Error ? null : result;
}

function denyReason(out: HookResult): string | null {
  if (!out) {
    return null;
  }
  const spec = out.hookSpecificOutput;
  if (spec && typeof spec === "object") {
    const body = spec as Record<string, unknown>;
    if (body.permissionDecision === "deny") {
      return String(body.permissionDecisionReason || "arc-policy deny");
    }
  }
  if (out.permission === "deny") {
    return String(out.user_message || out.agent_message || "arc-policy deny");
  }
  if (out.decision === "block") {
    return String(out.reason || out.followup_message || "arc-policy block");
  }
  return null;
}

function sessionText(out: HookResult): string | null {
  if (!out) {
    return null;
  }
  if (typeof out.systemMessage === "string" && out.systemMessage) {
    return out.systemMessage;
  }
  if (typeof out.additional_context === "string" && out.additional_context) {
    return out.additional_context;
  }
  const spec = out.hookSpecificOutput;
  if (spec && typeof spec === "object") {
    const text = (spec as Record<string, unknown>).additionalContext;
    if (typeof text === "string" && text) {
      return text;
    }
  }
  return null;
}

type PromptOpts = {
  promptGuidelines?: string[];
};

type Context = {
  cwd: string;
  ui?: { notify: (text: string, kind: "warning") => unknown };
};

type Pi = {
  on: (
    event: string,
    handler: (event: Record<string, unknown>, ctx: Context) => unknown,
  ) => void;
};

function assistantText(messages: unknown): string {
  if (!Array.isArray(messages)) return "";
  for (let i = messages.length - 1; i >= 0; i--) {
    const value: unknown = messages[i];
    if (!value || typeof value !== "object") continue;
    const message = value as Record<string, unknown>;
    if (message.role !== "assistant") continue;
    if (typeof message.content === "string") return message.content;
    if (!Array.isArray(message.content)) return "";
    return message.content
      .flatMap((value: unknown) => {
        if (!value || typeof value !== "object") return [];
        const part = value as Record<string, unknown>;
        return part.type === "text" && typeof part.text === "string"
          ? [part.text]
          : [];
      })
      .join("\n");
  }
  return "";
}

function stopFeedback(
  event: Record<string, unknown>,
  ctx: Context,
  messages: unknown,
  stopHookActive: boolean,
): string | null {
  const out = reply(
    runHook(
      {
        hook_event_name: "Stop",
        reason: "end_turn",
        last_assistant_message: assistantText(messages),
        stop_hook_active: stopHookActive,
        cwd: ctx.cwd,
      },
      ctx.cwd,
    ),
  );
  if (typeof out?.systemMessage === "string") {
    ctx.ui?.notify(out.systemMessage, "warning");
  }
  const reason = denyReason(out);
  if (reason) return reason;
  const spec = out?.hookSpecificOutput;
  if (spec && typeof spec === "object") {
    const body = spec as Record<string, unknown>;
    if (body.hookEventName === "Stop" && typeof body.additionalContext === "string") {
      return body.additionalContext || null;
    }
  }
  return null;
}

export default function (pi: Pi) {
  let sessionContext: string | null = null;
  let stopHookActive = false;

  pi.on("before_agent_start", (event) => {
    stopHookActive = false;
    const opts = event.systemPromptOptions as PromptOpts | undefined;
    if (!opts) {
      return;
    }
    const extra =
      "Arc Policy Runtime is active. Before editing source: arc-policy task begin --skill <arc-*> in the repository that owns the files (arc-policy --cwd <repo> from a parent folder). Before git commit/push or delivery: arc-policy task verify. Never route a denied edit through the shell.";
    opts.promptGuidelines = [
      ...new Set([
        ...(opts.promptGuidelines ?? []),
        extra,
        ...(sessionContext ? [sessionContext] : []),
      ]),
    ];
  });

  pi.on("session_start", (_event, ctx) => {
    stopHookActive = false;
    sessionContext = sessionText(
      reply(runHook({ hook_event_name: "SessionStart", cwd: ctx.cwd }, ctx.cwd)),
    );
  });

  pi.on("tool_call", (event, ctx) => {
    const toolName = String(event.toolName || "");
    const input =
      event.input && typeof event.input === "object"
        ? (event.input as Record<string, unknown>)
        : {};
    const cwd = ctx.cwd;
    const out = runHook(
      {
        hook_event_name: "PreToolUse",
        tool_name: toolName,
        tool_input: input,
        cwd,
      },
      cwd,
    );
    // An engine that cannot answer must not open the gate.
    if (out instanceof Error) {
      return {
        block: true,
        reason: `arc-policy engine unavailable: ${out.message}; verification remains pending`,
      };
    }
    const reason = denyReason(out);
    if (reason) {
      return { block: true, reason };
    }
    return undefined;
  });

  // PI and OMP expose different actionable boundaries; agent_end is notification-only.
  pi.on("agent_before_settle", (event, ctx) => {
    if (event.outcome !== "completed") return;
    const context = event.context as Record<string, unknown> | undefined;
    if (context?.canContinue === false) return;
    const reason = stopFeedback(
      event,
      ctx,
      context?.contextMessages,
      stopHookActive,
    );
    if (reason) {
      stopHookActive = true;
      return {
        continue: true,
        entries: [
          ...(Array.isArray(event.entries) ? event.entries : []),
          {
            type: "custom_message",
            customType: "arc-policy",
            content: reason,
            display: false,
          },
        ],
      };
    }
    return undefined;
  });

  pi.on("session_stop", (event, ctx) => {
    const reason = stopFeedback(
      event,
      ctx,
      event.last_assistant_message
        ? [event.last_assistant_message]
        : event.messages,
      event.stop_hook_active === true,
    );
    return reason ? { continue: true, additionalContext: reason } : undefined;
  });
}
