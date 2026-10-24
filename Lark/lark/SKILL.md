---
name: lark
version: 1.0.0
description: >-
  飞书 / Lark work through lark-cli: docs and wiki (/docx/, /wiki/), Drive files, native Markdown files, sheets
  (/sheets/), Base / 多维表格 (/base/), slides (/slides/), whiteboards and diagrams (画板, 流程图), IM messages and
  groups, mail, calendar and meeting rooms, video meetings, minutes and transcripts (妙记, 会议纪要), tasks, OKR,
  approvals, attendance, contacts, real-time events, Miaoda apps, raw OpenAPI, custom lark-cli skills, daily
  agenda and meeting-summary reports, and the project workspace (.lark.json, 飞书项目空间). doubao.com URLs route by
  path like Feishu ones. Not for local Markdown or PRDs (use arc:plan or research-write).
metadata:
  requires:
    bins: ["lark-cli"]
  library:
    - lark-shared
    - lark-doc
    - lark-wiki
    - lark-drive
    - lark-markdown
    - lark-sheets
    - lark-base
    - lark-slides
    - lark-whiteboard
    - lark-im
    - lark-mail
    - lark-calendar
    - lark-meeting
    - lark-task
    - lark-okr
    - lark-approval
    - lark-attendance
    - lark-contact
    - lark-event
    - lark-apps
    - lark-openapi-explorer
    - lark-skill-maker
    - lark-workflow-standup-report
    - lark-workflow-meeting-summary
    - lark-workflow-project-docs
    - lark-minutes
    - lark-note
    - lark-vc
    - lark-vc-agent
---

# lark

One entry for every Feishu/Lark capability. Pick exactly one library entry per step from the table, read it,
and follow it; read a second entry only when the request moves to another product.

## Session preflight (once)

1. `lark-cli auth status`. Missing config, unknown identity, missing scope or expired login → read
   [lark-shared](../lark-shared/SKILL.md) and its auth reference; valid authorization → continue.
2. Choose `--as user` (default: the signed-in user's own resources) or `--as bot` (bot-owned resources)
   explicitly per call; never switch identity silently to hide a permission error.
3. Success means exit code 0 and `ok == true` in the JSON envelope; never test `code == 0`. CLI success is not
   business success: poll async jobs to their final state, check every batch item and page.
4. Writes, deletes and local-file inputs follow lark-shared's safety reference; loading this skill authorizes
   no publishing, sending or deletion.

## Route

| Request or URL path | Entry |
|---|---|
| `/docx/`, `/wiki/` document content: read, create, edit, images, mind notes | [lark-doc](../lark-doc/SKILL.md) |
| Knowledge spaces, members, node tree, move or copy nodes | [lark-wiki](../lark-wiki/SKILL.md) |
| Drive files and folders, upload/download, permissions, comments, import Word/Excel/Markdown | [lark-drive](../lark-drive/SKILL.md) |
| Native `.md` files stored in Drive (view, upload, edit, diff) | [lark-markdown](../lark-markdown/SKILL.md) |
| `/sheets/` spreadsheets: cells, formulas, charts, pivots, financial models | [lark-sheets](../lark-sheets/SKILL.md) |
| `/base/` Base / 多维表格 / bitable: tables, fields, records, views, dashboards, workflows | [lark-base](../lark-base/SKILL.md) |
| `/slides/` presentations | [lark-slides](../lark-slides/SKILL.md) |
| Whiteboards, UML, flowcharts, sequence and architecture diagrams inside docs | [lark-whiteboard](../lark-whiteboard/SKILL.md) |
| Messages, groups, chat history, cards and card callbacks, urgent notices | [lark-im](../lark-im/SKILL.md) |
| Email: draft, send, reply, search, folders, rules | [lark-mail](../lark-mail/SKILL.md) |
| Calendar events, attendees, free/busy, meeting rooms | [lark-calendar](../lark-calendar/SKILL.md) |
| Past or live video meetings, minutes, transcripts, notes (old names lark-minutes, lark-note, lark-vc, lark-vc-agent) | [lark-meeting](../lark-meeting/SKILL.md) |
| Tasks, task lists, task agents | [lark-task](../lark-task/SKILL.md) |
| OKR cycles, objectives, key results | [lark-okr](../lark-okr/SKILL.md) |
| Approval inbox, instances, start a native approval | [lark-approval](../lark-approval/SKILL.md) |
| Own attendance records | [lark-attendance](../lark-attendance/SKILL.md) |
| Resolve a person or bot to an open_id, or the reverse | [lark-contact](../lark-contact/SKILL.md) |
| Subscribe to or consume real-time events (NDJSON) | [lark-event](../lark-event/SKILL.md) |
| Miaoda (妙搭) apps only when the app or its link exists | [lark-apps](../lark-apps/SKILL.md) |
| Agenda plus open tasks for a day or week | [lark-workflow-standup-report](../lark-workflow-standup-report/SKILL.md) |
| Meeting notes over a period into a report | [lark-workflow-meeting-summary](../lark-workflow-meeting-summary/SKILL.md) |
| Project workspace: create, refresh or sync, `.lark.json` | [lark-workflow-project-docs](../lark-workflow-project-docs/SKILL.md) |
| Wrap Lark operations into a custom lark-cli skill | [lark-skill-maker](../lark-skill-maker/SKILL.md) |
| Nothing above covers it: find a raw OpenAPI | [lark-openapi-explorer](../lark-openapi-explorer/SKILL.md) |

The URL path decides the entry, not the domain. A request that reads one product and writes another runs the
read entry first, then the write entry.

## Failure handling

| Failure | Action |
|---|---|
| Auth or scope error | lark-shared auth reference; one `auth login`, then report |
| Platform error code | The entry's error table; retry the same call at most once |
| No row matches | Check the neighbouring rows, then lark-openapi-explorer; still none → report the gap |

## Completion

- [ ] One entry chosen per step and followed.
- [ ] Every write reports the returned token, ID or URL; partial, blocked and not-run items listed.
