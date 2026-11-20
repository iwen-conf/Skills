# CNB (cnb.cool) Hard-Spec Cheatsheet

## 1. Runtime & Security Contract

- Verify `CNB_API_ENDPOINT` (default `https://api.cnb.cool`), `CNB_TOKEN`, `CNB_REPO_SLUG`, and target `CNB_ISSUE_IID` or `CNB_PULL_REQUEST_IID` before write calls. Never print tokens or send credentials to untrusted hosts.
- Pass external text via structured argv, stdin, or verified file flags—never interpolate raw PR/issue text into shell commands. Quote `` `@npc` `` in backticks when mentioning without summoning.

## 2. `cnb` CLI & PR Line Review

- **Issues**: `cnb issues get | list-comments | comment --body <text> | close | open | add-labels | add-assignees | upload-file | upload-image`
- **Pulls**: `cnb pulls get | list-files | list-commits | list-comments | comment --body <text> | check-status | get-ci-logs [--sn N] | get-ci-timing`
- **Inline Review**: Anchor line comments strictly to valid right-side diff lines; report JSON status as `passed` (only when `coverage.complete == true`), `partial`, `needs_modification`, or `critical`.

## 3. `.cnb.yml` Pipeline Schema

- **Triggers**: `<branch>` -> `push`, `commit.add`, `pull_request`, `pull_request.target`, `pull_request.mergeable`, `tag_push`, `web_trigger`, `api_trigger`, `"crontab: 30 5 * * *"`; `$` -> `issue.open`, `pull_request.comment@npc`.
- **Stage Keys**: `runner.cpus` (RAM = `cpus * 2G`, default `8`c/`16`G), `docker.image` or `docker.build` (`dockerfile`, `by`, `versionBy`), `services: [docker]`, `imports: [https://cnb.cool/<org>/<secret-repo>/-/blob/main/secrets.yml]`, `lock`, `stages`.
