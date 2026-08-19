---
name: android-intent-security
description: Best practices for Android Intent security. Use this skill when auditing
  AndroidManifest.xml component configurations or source code handling incoming Intents,
  PendingIntents, exported services, receivers, or ContentProviders to prevent redirection,
  privilege escalation, and unauthorized access. Do not use for network, web, or
  host-to-server security.
license: Complete terms in LICENSE.txt
metadata:
  author: Google LLC
  last-updated: '2026-06-25'
  keywords:
  - Android
  - Intent
  - Redirection
  - PendingIntent
  - ContentProvider
  - Service
  - Signature
  - Verification
---

# Android Intent Security

Use the focused guide for concrete security rules and code/configuration patterns. Load it after
confirming the component type and inbound trust boundary; do not apply every pattern to an
unrelated Android feature.

## Intent Router

| User intent | Load |
|---|---|
| Nested Intent or Intent redirection | [`references/intent-security-guide.md`](references/intent-security-guide.md): Intent handling/redirection logic and safe redirection patterns |
| PendingIntent mutability | [`references/intent-security-guide.md`](references/intent-security-guide.md): PendingIntent security logic and secure creation |
| Exported component or signature permission | [`references/intent-security-guide.md`](references/intent-security-guide.md): routing comparison, custom signature permission, and service caller verification |
| ContentProvider exposure/query safety | [`references/intent-security-guide.md`](references/intent-security-guide.md): provider security logic and secure configuration/query patterns |
| onCreate/onNewIntent lifecycle | [`references/intent-security-guide.md`](references/intent-security-guide.md): safe onNewIntent lifecycle verification |
| Reporting a hardening change | [`references/intent-security-guide.md`](references/intent-security-guide.md), Reporting guidelines |

## Red Lines

```text
DO NOT launch an untrusted nested Intent without validating its target and allowed data.
DO NOT create an implicit mutable PendingIntent.
DO NOT expose a component or provider without checking exported status and caller permissions.
DO NOT weaken a failing security assertion or claim safety from a checklist alone.
```

- Confirm the manifest/source behavior and caller trust boundary before changing security code.
- Preserve the existing security contract and verify both the original entry point and analogous exported components.
- For a suspected vulnerability or hardening fix, load the evidence-first root-cause repair gate in [`docs/execution-truth.md`](../../docs/execution-truth.md) before editing; distinguish a scanner signal from a confirmed reachable defect.
