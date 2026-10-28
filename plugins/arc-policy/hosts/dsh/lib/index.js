/**
 * dsh-arc-policy
 *
 * Delegates DeepSeek Harness tool pre-execute to the Arc Policy Python engine
 * (hooks/hook.py). Prompt inject reminds the model to run arc-policy CLI.
 *
 * Same-user plugins are process control, not a security boundary.
 */

import { spawnSync } from 'node:child_process'
import { existsSync, realpathSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

// Shells the harness spawns inherit this, so the Git-level commit gate recognizes the agent.
process.env.ARC_POLICY_AGENT ??= '1'

export const name = 'dsh-arc-policy'
export const inject = ['tools']

const POLICY_PROMPT = `
ARC POLICY RUNTIME (dsh-arc-policy)

This session is gated by the Arc Policy Runtime. Domain work still belongs to
one Arc skill per phase (arc-explore, arc-plan, arc-code, arc-verify, arc-audit, arc-vet).

1. Before the first source edit, in the repository that owns the files
   (from a parent folder that is not a repository: arc-policy --cwd <repo> ...):

   arc-policy task begin --skill <arc-code|arc-explore|arc-plan|arc-verify|arc-audit|arc-vet>

2. Write source with host tools after begin. Repository search still goes through
   arc-idx (dsh-force-arc-idx).

3. Before git commit / git push, schema migrations, or claiming delivery:

   arc-policy task verify

The harness denies source edits with no active task in the edited file's
repository, git commit/push without a matching receipt, git --no-verify, and
common schema-migration CLIs. Never route a denied edit through the shell.

Passing a hook is not proof of skill execution. Only CLI receipts count.
`.trim()

function pluginRoot() {
  const here = dirname(realpathSync(fileURLToPath(import.meta.url)))
  return join(here, '..', '..', '..')
}

function asRecord(value) {
  if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim().startsWith('{')) {
    try {
      const parsed = JSON.parse(value)
      if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
        return parsed
      }
    } catch {
      // not JSON
    }
  }
  return undefined
}

// One id per harness process: the engine records which repositories this session touched.
const SESSION_ID = `dsh-${process.pid}-${Date.now().toString(36)}`

/** The engine's JSON reply or null; throws when the engine could not answer. */
function runHook(payload) {
  const root = pluginRoot()
  const hook = join(root, 'hooks', 'hook.py')
  if (!existsSync(hook)) {
    throw new Error(`missing ${hook}`)
  }
  const result = spawnSync('python3', [hook], {
    input: JSON.stringify({ ...payload, session_id: SESSION_ID }),
    encoding: 'utf8',
    timeout: 30_000,
    cwd: typeof payload.cwd === 'string' ? payload.cwd : process.cwd(),
    env: {
      ...process.env,
      PLUGIN_ROOT: root,
      CLAUDE_PLUGIN_ROOT: root,
    },
  })
  if (result.error) {
    throw result.error
  }
  if (result.status !== 0) {
    throw new Error(`hook exited ${result.status ?? result.signal}: ${(result.stderr || '').trim()}`)
  }
  const text = (result.stdout || '').trim()
  if (!text) {
    return null
  }
  const parsed = JSON.parse(text)
  if (!parsed || typeof parsed !== 'object') {
    throw new Error(`hook replied with non-object JSON: ${text}`)
  }
  return parsed
}

function denyReason(out) {
  if (!out) {
    return null
  }
  const spec = out.hookSpecificOutput
  if (spec && spec.permissionDecision === 'deny') {
    return String(spec.permissionDecisionReason || 'arc-policy deny')
  }
  if (out.permission === 'deny') {
    return String(out.user_message || out.agent_message || 'arc-policy deny')
  }
  return null
}

/**
 * @param {import('@deepseek-ai/cordis').Context} ctx
 * @param {{ enabled?: boolean, injectPrompt?: boolean, promptOrder?: number }} [config]
 */
export function apply(ctx, config = {}) {
  const enabled = config.enabled !== false
  const injectPrompt = enabled && config.injectPrompt !== false

  if (injectPrompt) {
    ctx.inject(['systemPrompt'], (scope) => {
      try {
        const sp = scope.systemPrompt
        if (typeof sp?.section !== 'function') {
          scope.logger?.warn?.(
            'dsh-arc-policy: systemPrompt.section missing; runtime deny-gate remains active',
          )
          return
        }
        let order = Number.isFinite(config.promptOrder) ? Number(config.promptOrder) : 91
        sp.section({
          name: 'dsh-arc-policy',
          order,
          text: POLICY_PROMPT,
        })
      } catch (err) {
        scope.logger?.warn?.(`dsh-arc-policy: systemPrompt inject failed: ${String(err)}`)
      }
    })
  }

  ctx.on('tools/pre-execute', async (exec, next) => {
    if (!enabled) {
      return next()
    }
    const toolName = typeof exec?.name === 'string' ? exec.name : ''
    try {
      const rawArgs = exec?.arguments
      const args =
        asRecord(rawArgs)
        ?? (typeof rawArgs === 'string' && rawArgs.trim() ? { command: rawArgs } : {})
      const reason = denyReason(
        runHook({
          hook_event_name: 'PreToolUse',
          tool_name: toolName,
          tool_input: args,
          cwd: process.cwd(),
        }),
      )
      if (reason) {
        return {
          kind: 'deny',
          reason,
        }
      }
    } catch (err) {
      // An engine that cannot answer must not open the gate.
      return {
        kind: 'deny',
        reason: `arc-policy engine unavailable: ${String(err)}; verification remains pending`,
      }
    }
    return next()
  })
}
