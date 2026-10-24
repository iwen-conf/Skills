# Read methods

## Source fetch and local extraction

The installed skill contains [fetch.sh](../scripts/fetch.sh) and its local
[extractor](../scripts/fetch_local.py). Resolve `<skill-dir>` to the directory
containing this skill's SKILL.md; do not use a repository-provided helper with
the same name.

```bash
bash "<skill-dir>/scripts/fetch.sh" "{url}"
```

By default the local extractor connects directly to the source site and ignores
implicit system proxies. An explicitly chosen network proxy is separate from a
third-party extraction service. If the user's existing network configuration
requires one, pass it explicitly without displaying its credentials:

```bash
bash "<skill-dir>/scripts/fetch.sh" "{url}" "{network_proxy_url}"
```

The Python helper also accepts `--network-proxy`. Both paths retain normal TLS
certificate verification. Neither enables defuddle.md or r.jina.ai.

Only when the user has opted into third-party extraction for a public URL:

```bash
bash "<skill-dir>/scripts/fetch.sh" --use-proxy "{url}"
```

Read the structured stderr for each attempted tier. A public-page fetch failure
does not itself grant consent for third-party extraction. Do not send internal,
authenticated, or sensitive URLs to extraction services or reader plugins.
Inspect the actual output for a login, consent, or paywall shell before treating
it as source content. Missing optional extraction packages are a quality limit,
not a reason to install packages or change global configuration automatically.

## GitHub

Use raw source for public files and the existing authenticated `gh` client for
private material. Preserve the requested revision when reading a file:

```bash
curl -fL "https://raw.githubusercontent.com/{owner}/{repo}/{revision}/{path}"
gh api "repos/{owner}/{repo}/contents/{path}?ref={revision}" --jq '.content' | base64 -d
```

Use the built-in fetcher for public page fallback. Keep private content on its
authenticated path. Once a repository is local, discovery follows `arc-idx`.

## PDF

Download a remote PDF from its source into a session temporary directory, then
use an installed local extractor:

```bash
READ_PDF_DIR=$(mktemp -d)
curl -fL "{pdf_url}" -o "$READ_PDF_DIR/input.pdf"
pdftotext -layout "$READ_PDF_DIR/input.pdf" -
```

Use an installed `marker_single` when document layout matters or `pypdf` when
available. If no extractor is available, report the limitation; do not invent
text. A third-party PDF reader follows the same explicit opt-in boundary as
other extraction services. Local PDF paths never become public upload authority.

## Feishu / Lark

Load the installed `lark-doc` skill for documents and `lark-wiki` for wiki node
resolution, then follow their existing CLI and authorization workflow. Reuse
the user's configured account. Do not create a separate app-credential flow,
install another CLI, or request app secrets just to read a link. If access is
unavailable, report the actual authorization error and continue unrelated work.
Convert the retrieved blocks or document text to the requested output; raw API
JSON is not the final reading result.

## WeChat and JavaScript-rendered pages

Try source fetch first. If readable content requires a browser, load and follow
the installed `agent-browser` skill; desktop UI interaction must also follow
`agent-computer-use`. Use the user's allowed session and report authentication
limits. Do not launch custom Playwright scripts, install browser runtimes, use
Safari AppleScript, or bypass the configured browser tooling. Third-party
reader fallback still requires the explicit opt-in described above.
