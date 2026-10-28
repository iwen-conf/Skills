# dsh-arc-policy

Arc Policy Runtime for DeepSeek Harness. Prompt inject plus `tools/pre-execute`
deny, both delegated to `plugins/arc-policy/hooks/hook.py`. A tool call is denied when the
engine cannot answer (missing, crashed, timed out after 30 s, invalid reply). After the call runs,
the adapter reports it as `PostToolUse` with the same `tool_use_id`; a failed report is logged and
leaves the tool result unchanged. The plugin sends one
session id per harness process and exports `ARC_POLICY_AGENT=1`, so the Git hook gates commits
the harness's shells make. The harness has no turn-end hook wired here; delivery relies on the
commit gate and the Git hook.

Does not replace `dsh-force-arc-idx` (repository search still goes through arc-idx).

## Install (desktop profile)

```bash
ln -sfn /Users/iluwen/Documents/Code/Skills/plugins/arc-policy/hosts/dsh \
  ~/.dsh/plugins/dsh-arc-policy
```

`~/.dsh/profiles/desktop/package.json` dependency, as a link:

```json
"dsh-arc-policy": "link:../../plugins/dsh-arc-policy"
```

Use `link:`, not `file:`. pnpm copies a `file:` directory into `node_modules`, and the copy cannot
find `hooks/hook.py`, so every tool call is denied with `arc-policy engine unavailable: missing …`.
Before 2026-10-24 the adapter allowed tool calls in that state, so the gate did nothing.

Then:

```bash
cd ~/.dsh/profiles/desktop && pnpm install
```

`cordis.patch.yml` insert:

```yaml
- insert:
    - id: arc-policy
      name: dsh-arc-policy
      config:
        enabled: true
        injectPrompt: true
```

Restart DeepSeek Harness.

## Config

| field | default | meaning |
| --- | --- | --- |
| `enabled` | `true` | master switch |
| `injectPrompt` | `true` | add a short system-prompt reminder |
