#!/usr/bin/env bash
set -euo pipefail

# Must run on bash 3.2: no associative arrays, mapfile, ${x,,} or ((x++)) under set -e, and arrays
# that may be empty are expanded as ${a[@]+"${a[@]}"} because of set -u.

export GIT_OPTIONAL_LOCKS=0 GIT_LITERAL_PATHSPECS=1

PROG=gate.sh
SHOW_MAX=20
TOOL_LINES=10

usage() {
  cat <<'EOF'
usage: gate.sh start [--task <slug>] [--force]
       gate.sh check [--task <slug>] [--max-files N] [--strict] [--scope <file>]

start   Record HEAD, `git status` entries and content hashes of files that are already dirty
        or untracked into .arc/artifacts/<slug>/baseline.json. Refuses to overwrite an
        existing baseline unless --force is given.
check   Check the change set since the baseline: placeholders, suppressions, format,
        lint/typecheck, affected tests, scope. Prints one line per check and writes the full
        report to .arc/artifacts/<slug>/reports/gate-<timestamp>.txt.

--task <slug>    artifact slug (default: current branch name, else "default")
--max-files N    scope WARN threshold (default 20)
--strict         run the full test suite of every detected stack
--scope <file>   one glob per line, # comments allowed; * also matches /, a trailing /
                 or a bare directory matches everything below it

Exit status: 0 no check failed, 1 a check failed or start refused, 2 usage or environment error.
The gate never stages, commits, stashes, formats or otherwise rewrites files.
EOF
}

die() {
  printf '%s: %s\n' "$PROG" "$*" >&2
  exit 2
}

IFS= read -r -d '' PY_HELPER <<'PY' || true
import hashlib
import json
import os
import re
import subprocess
import sys
import time

ART = ".arc/artifacts/"
DOCS = (".md", ".mdx", ".markdown", ".rst", ".adoc")
TS = (".ts", ".tsx", ".mts", ".cts")
BLOB_LIMIT = 4 << 20
HUNK = re.compile(rb"^@@ -[0-9,]+ \+([0-9]+)")
PLACEHOLDER = [(None, re.compile(p)) for p in (
    r"\bTODO\b",
    r"\bFIXME\b",
    r"\bXXX\b",
    r"\bTBD\b",
    r"\bpanic\(\s*\"(?i:not implemented|todo)\"\s*\)",
    r"\bthrow\s+new\s+Error\(\s*[\"'`](?i:not implemented|todo)[\"'`]\s*\)",
    r"\braise\s+NotImplementedError\b",
    r"\b(?:unimplemented|todo)!\s*\(",
    r"(?://|#|/\*|<!--)\s*(?:\.\.\.|\u2026)",
    r"(?i)(?://|#|/\*|<!--)\s*(?:\.\.\.\s*)?(?:existing code|rest of (?:the )?"
    r"(?:code|file|implementation|function|method|class|logic))\b",
)]
SUPPRESSION = [(None, re.compile(p)) for p in (
    r"//\s*nolint\b",
    r"\beslint-disable",
    r"@ts-(?:ignore|expect-error|nocheck)\b",
    r"#\s*type:\s*ignore\b",
    r"#\s*noqa\b",
    r"#\s*pyright:\s*ignore\b",
)] + [
    (TS, re.compile(r":\s*any\b")),
    (TS, re.compile(r"\bas\s+any\b")),
    ((".go",), re.compile(r"\binterface\s*\{\s*\}")),
]


def git(root, args, ok=(0,)):
    proc = subprocess.run(["git"] + list(args), cwd=root, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if proc.returncode not in ok:
        err = proc.stderr.decode("utf-8", "replace").strip()
        sys.stderr.write("gate.sh: git %s failed: %s\n" % (args[0], err))
        sys.exit(2)
    return proc.stdout


def zsplit(data):
    return [os.fsdecode(item) for item in data.split(b"\0") if item]


def read_list(path):
    with open(path, "rb") as fh:
        return zsplit(fh.read())


def write_list(path, items):
    with open(path, "wb") as fh:
        for item in items:
            fh.write(os.fsencode(item) + b"\0")


def file_state(path):
    if os.path.islink(path):
        return "link:" + hashlib.sha256(os.fsencode(os.readlink(path))).hexdigest(), None
    if os.path.isdir(path):
        return "dir", None
    if not os.path.isfile(path):
        return "missing", None
    digest = hashlib.sha256()
    data = None
    with open(path, "rb") as fh:
        if os.fstat(fh.fileno()).st_size <= BLOB_LIMIT:
            data = fh.read()
            digest.update(data)
        else:
            for chunk in iter(lambda: fh.read(1 << 20), b""):
                digest.update(chunk)
    return "sha256:" + digest.hexdigest(), data


def load_baseline(path):
    if not path:
        return {}
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
        if not isinstance(data.get("files"), dict) or not isinstance(data.get("head"), str):
            raise ValueError("missing head/files")
    except (OSError, ValueError, AttributeError) as exc:
        sys.stderr.write("gate.sh: unreadable baseline %s: %s\n" % (path, exc))
        sys.exit(2)
    return data


def snapshot(root, artdir, head, branch, task, out):
    fields = git(root, ["status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames"]).split(b"\0")
    entries, i = [], 0
    while i < len(fields):
        field = fields[i]
        i += 1
        if len(field) < 4:
            continue
        entry = {"xy": field[:2].decode("ascii", "replace"), "path": os.fsdecode(field[3:])}
        if "R" in entry["xy"] or "C" in entry["xy"]:
            entry["orig"] = os.fsdecode(fields[i])
            i += 1
        if not entry["path"].startswith(ART):
            entries.append(entry)
    files = {}
    for entry in entries:
        for path in (entry["path"], entry.get("orig")):
            if not path or path in files or path.startswith(ART):
                continue
            state, data = file_state(os.path.join(root, path))
            blob = None
            if data is not None:
                blob = "baseline-blobs/" + state.split(":", 1)[1]
                target = os.path.join(artdir, blob)
                if not os.path.exists(target):
                    os.makedirs(os.path.dirname(target), exist_ok=True)
                    with open(target, "wb") as fh:
                        fh.write(data)
            files[path] = {"hash": state, "blob": blob}
    record = {
        "version": 1,
        "task": task,
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "head": head,
        "branch": branch or None,
        "status": entries,
        "files": files,
    }
    tmp = out + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(record, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, out)
    print(len(files))


def head(baseline):
    print(load_baseline(baseline)["head"])


def changeset(root, base, files, untracked):
    paths = set(zsplit(git(root, ["diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", base, "--"])))
    paths.update(untracked)
    paths.update(files)
    keep = []
    for path in sorted(paths):
        if path.startswith(ART):
            continue
        if path in files and file_state(os.path.join(root, path))[0] == files[path].get("hash"):
            continue
        keep.append(path)
    return keep


def added_lines(diff):
    lines, number, in_hunk = [], 0, False
    for raw in diff.split(b"\n"):
        match = HUNK.match(raw)
        if match:
            number, in_hunk = int(match.group(1)), True
        elif raw.startswith(b"diff --git "):
            in_hunk = False
        elif in_hunk and raw.startswith(b"+"):
            lines.append((number, raw[1:]))
            number += 1
    return lines


def analyze(root, base, baseline, outdir):
    files = load_baseline(baseline).get("files", {})
    blobdir = os.path.dirname(baseline) if baseline else ""
    untracked = set(zsplit(git(root, ["ls-files", "--others", "--exclude-standard", "-z"])))
    changed = changeset(root, base, files, untracked)
    write_list(os.path.join(outdir, "changed"), changed)
    found = {"placeholders": [], "suppressions": []}
    diff_opts = ["-U0", "--no-color", "--no-ext-diff", "--no-textconv"]
    for path in changed:
        full = os.path.join(root, path)
        if os.path.islink(full) or not os.path.isfile(full) or path.lower().endswith(DOCS):
            continue
        with open(full, "rb") as fh:
            if b"\0" in fh.read(8000):
                continue
        blob = (files.get(path) or {}).get("blob")
        if blob and os.path.isfile(os.path.join(blobdir, blob)):
            diff = git(root, ["diff", "--no-index"] + diff_opts + ["--", os.path.join(blobdir, blob), full], ok=(0, 1))
            lines = added_lines(diff)
        elif path in untracked:
            with open(full, "rb") as fh:
                lines = list(enumerate(fh.read().split(b"\n"), 1))
        else:
            lines = added_lines(git(root, ["diff"] + diff_opts + [base, "--", path]))
        ext = os.path.splitext(path)[1].lower()
        label = path.replace("\n", "\\n")
        for number, raw in lines:
            text = raw.decode("utf-8", "replace")
            for key, rules in (("placeholders", PLACEHOLDER), ("suppressions", SUPPRESSION)):
                for exts, rule in rules:
                    if exts and ext not in exts:
                        continue
                    match = rule.search(text)
                    if match:
                        found[key].append("%s:%d: %s" % (label, number, match.group(0).strip()[:60]))
                        break
    for key in found:
        with open(os.path.join(outdir, key), "wb") as fh:
            for line in found[key]:
                fh.write(line.encode("utf-8", "surrogateescape") + b"\n")


def pytests(root, changed, out):
    def is_test(name):
        return name.startswith("test_") or name.endswith("_test.py")

    paths = [p for p in read_list(changed) if p.endswith(".py")]
    selected = {p for p in paths if is_test(os.path.basename(p)) and os.path.isfile(os.path.join(root, p))}
    wanted = set()
    for path in paths:
        name = os.path.basename(path)
        if not is_test(name) and name not in ("__init__.py", "conftest.py"):
            wanted.update(("test_" + name, name[:-3] + "_test.py"))
    if wanted:
        for path in zsplit(git(root, ["ls-files", "-z", "--cached", "--others", "--exclude-standard"])):
            if os.path.basename(path) in wanted and os.path.isfile(os.path.join(root, path)):
                selected.add(path)
    write_list(out, sorted(selected))


COMMANDS = {"snapshot": snapshot, "head": head, "analyze": analyze, "pytests": pytests}
COMMANDS[sys.argv[1]](*sys.argv[2:])
PY

py() { python3 -c "$PY_HELPER" "$@"; }

say() { printf '%s\n' "$*"; }
rec() { printf '%s\n' "$*" >>"$TMP/summary"; }
out() {
  say "$@"
  rec "$@"
}

emit() {
  local text="$1: $2"
  if [ $# -ge 3 ]; then text="$text($3)"; fi
  out "$text"
  case $2 in
    FAIL) FAIL_N=$((FAIL_N + 1)) ;;
    NOT_RUN) NOT_RUN_N=$((NOT_RUN_N + 1)) ;;
    WARN) WARN_N=$((WARN_N + 1)) ;;
  esac
}

list_capped() {
  local n=0 line
  while IFS= read -r line; do
    line=${line#./}
    n=$((n + 1))
    if [ $n -le $SHOW_MAX ]; then out "  $line"; else rec "  $line"; fi
  done <"$1"
  if [ $n -gt $SHOW_MAX ]; then say "  +$((n - SHOW_MAX)) more"; fi
}

rel() { printf '%s' "${1#"$ROOT"/}"; }

rel_dir() {
  case $1 in
    "$ROOT" | "$ROOT/.") printf . ;;
    *) rel "$1" ;;
  esac
}

short() {
  case $1 in
    '' | none) printf none ;;
    *) printf '%s' "${1:0:12}" ;;
  esac
}

sfx() {
  if [ "$1" = . ]; then SFX=; else SFX="@$1"; fi
}

current_head() { git rev-parse -q --verify 'HEAD^{commit}' 2>/dev/null || true; }

empty_tree() { git hash-object -t tree /dev/null; }

have() { command -v "$1" >/dev/null 2>&1; }

run_tool() {
  local dir=$1
  shift
  if (cd "$dir" && "$@") >"$TMP/out" 2>&1 </dev/null; then RC=0; else RC=$?; fi
  {
    printf '\n$ (cd %s &&' "$(rel_dir "$dir")"
    printf ' %q' "$@"
    printf ') -> exit %s\n' "$RC"
    cat "$TMP/out"
  } >>"$TMP/details"
}

show_output() {
  local n=0 line total
  total=$(grep -c '[^[:space:]]' "$TMP/out" || true)
  while [ $n -lt $TOOL_LINES ] && IFS= read -r line; do
    case $line in *[![:space:]]*) ;; *) continue ;; esac
    say "    $line"
    n=$((n + 1))
  done <"$TMP/out"
  if [ "$total" -gt $TOOL_LINES ]; then say "    (+$((total - TOOL_LINES)) more lines in the report)"; fi
}

tool_verdict() {
  if [ "$RC" -eq 0 ]; then
    emit "$1" PASS
  elif [ "$RC" -eq 127 ]; then
    emit "$1" NOT_RUN "$2: command not found (exit 127)"
    show_output
  else
    emit "$1" FAIL "$2 exit $RC"
    show_output
  fi
}

is_kind() {
  case $1 in
    go) case $2 in *.go) return 0 ;; esac ;;
    gomod) case $2 in *.go | go.mod | */go.mod | go.sum | */go.sum) return 0 ;; esac ;;
    js) case $2 in *.js | *.jsx | *.mjs | *.cjs | *.ts | *.tsx | *.mts | *.cts | *.vue | *.svelte) return 0 ;; esac ;;
    tsproj)
      case $2 in
        *.js | *.jsx | *.mjs | *.cjs | *.ts | *.tsx | *.mts | *.cts | *.vue | *.svelte) return 0 ;;
        tsconfig*.json | */tsconfig*.json | package.json | */package.json) return 0 ;;
      esac
      ;;
    web)
      case $2 in
        *.js | *.jsx | *.mjs | *.cjs | *.ts | *.tsx | *.mts | *.cts | *.vue | *.svelte) return 0 ;;
        *.css | *.scss | *.less | *.html) return 0 ;;
      esac
      ;;
    py) case $2 in *.py | *.pyi) return 0 ;; esac ;;
    rs) case $2 in *.rs) return 0 ;; esac ;;
    rsproj) case $2 in *.rs | Cargo.toml | */Cargo.toml | Cargo.lock | */Cargo.lock) return 0 ;; esac ;;
  esac
  return 1
}

nearest_marker() {
  local p=$1 d m
  shift
  case $p in */*) d=${p%/*} ;; *) d=. ;; esac
  NEAREST=
  while :; do
    for m in "$@"; do
      if [ -e "$ROOT/$d/$m" ]; then
        NEAREST=$d
        return 0
      fi
    done
    if [ "$d" = . ]; then return 0; fi
    case $d in */*) d=${d%/*} ;; *) d=. ;; esac
  done
}

add_root() {
  local r
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    if [ "$r" = "$1" ]; then return 0; fi
  done
  U_ROOTS+=("$1")
}

group_by_marker() {
  local kind=$1 include_root=$2 fallback=$3 f m
  shift 3
  G_FILES=() G_ROOTS=() U_ROOTS=()
  if [ "$include_root" = 1 ]; then
    for m in "$@"; do
      if [ -e "$ROOT/$m" ]; then
        U_ROOTS=(.)
        break
      fi
    done
  fi
  for f in ${CHANGED[@]+"${CHANGED[@]}"}; do
    is_kind "$kind" "$f" || continue
    nearest_marker "$f" "$@"
    if [ -z "$NEAREST" ]; then NEAREST=$fallback; fi
    if [ -z "$NEAREST" ]; then continue; fi
    G_FILES+=("$f")
    G_ROOTS+=("$NEAREST")
    add_root "$NEAREST"
  done
}

files_for_root() {
  local i=0 relpath
  R_FILES=() R_EXIST=()
  while [ $i -lt ${#G_FILES[@]} ]; do
    if [ "${G_ROOTS[$i]}" = "$1" ]; then
      relpath=${G_FILES[$i]}
      if [ "$1" != . ]; then relpath=${relpath#"$1"/}; fi
      R_FILES+=("$relpath")
      if [ -f "$ROOT/${G_FILES[$i]}" ] && [ ! -L "$ROOT/${G_FILES[$i]}" ]; then R_EXIST+=("./$relpath"); fi
    fi
    i=$((i + 1))
  done
}

find_node_bin() {
  local d=$1
  while :; do
    if [ -e "$ROOT/$d/node_modules/.bin/$2" ]; then return 0; fi
    if [ "$d" = . ]; then return 1; fi
    case $d in */*) d=${d%/*} ;; *) d=. ;; esac
  done
}

find_py_tool() {
  TOOL=
  if [ -x "$ROOT/$2/.venv/bin/$1" ]; then
    TOOL=$ROOT/$2/.venv/bin/$1
  elif [ -x "$ROOT/.venv/bin/$1" ]; then
    TOOL=$ROOT/.venv/bin/$1
  else
    TOOL=$(command -v "$1" 2>/dev/null || true)
  fi
  [ -n "$TOOL" ]
}

find_pytest() {
  if [ -x "$ROOT/.venv/bin/python" ] && "$ROOT/.venv/bin/python" -c 'import pytest' >/dev/null 2>&1; then
    PYTEST=("$ROOT/.venv/bin/python" -m pytest)
    return 0
  fi
  if have pytest; then
    PYTEST=(pytest)
    return 0
  fi
  return 1
}

dir_has_go() {
  local x
  for x in "$1"/*.go; do
    if [ -f "$x" ]; then return 0; fi
  done
  return 1
}

add_pkg() {
  local p
  for p in ${PKGS[@]+"${PKGS[@]}"}; do
    if [ "$p" = "$1" ]; then return 0; fi
  done
  PKGS+=("$1")
}

go_packages() {
  local f d
  PKGS=()
  for f in ${R_FILES[@]+"${R_FILES[@]}"}; do
    case $f in
      go.mod | go.sum)
        PKGS=(./...)
        return 0
        ;;
    esac
    case /$f in */testdata/* | */vendor/* | */_* | */.*) continue ;; esac
    case $f in */*) d=./${f%/*} ;; *) d=. ;; esac
    if dir_has_go "$ROOT/$1/$d"; then add_pkg "$d"; fi
  done
}

check_hits() {
  local n
  n=$(wc -l <"$2" | tr -d ' ')
  if [ "$n" -eq 0 ]; then
    emit "$1" PASS
    return 0
  fi
  if [ "$n" -eq 1 ]; then emit "$1" FAIL "1 hit"; else emit "$1" FAIL "$n hits"; fi
  list_capped "$2"
}

check_format() {
  local any=0 r n f
  group_by_marker go 0 .
  files_for_root .
  GO_FILES=()
  for f in ${R_EXIST[@]+"${R_EXIST[@]}"}; do
    case $f in */testdata/* | ./vendor/* | */vendor/*) continue ;; esac
    GO_FILES+=("$f")
  done
  if [ ${#GO_FILES[@]} -gt 0 ]; then
    any=1
    if ! have gofmt; then
      emit "format(go)" NOT_RUN "gofmt not installed"
    else
      run_tool "$ROOT" gofmt -l "${GO_FILES[@]}"
      if [ "$RC" -ne 0 ]; then
        tool_verdict "format(go)" gofmt
      elif [ -s "$TMP/out" ]; then
        n=$(wc -l <"$TMP/out" | tr -d ' ')
        emit "format(go)" FAIL "$n file(s) not gofmt-formatted"
        list_capped "$TMP/out"
      else
        emit "format(go)" PASS
      fi
    fi
  fi

  group_by_marker web 0 . package.json
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    files_for_root "$r"
    [ ${#R_EXIST[@]} -gt 0 ] || continue
    any=1
    sfx "$r"
    if ! find_node_bin "$r" prettier; then
      emit "format(web$SFX)" NOT_RUN "prettier not installed"
    elif ! have npx; then
      emit "format(web$SFX)" NOT_RUN "npx not installed"
    else
      run_tool "$ROOT/$r" npx --no-install prettier --check "${R_EXIST[@]}"
      tool_verdict "format(web$SFX)" prettier
    fi
  done

  group_by_marker py 0 .
  files_for_root .
  if [ ${#R_EXIST[@]} -gt 0 ]; then
    any=1
    if find_py_tool ruff .; then
      run_tool "$ROOT" "$TOOL" format --check --no-cache "${R_EXIST[@]}"
      tool_verdict "format(python)" "ruff format"
    else
      emit "format(python)" NOT_RUN "ruff not installed"
    fi
  fi

  group_by_marker rs 0 "" Cargo.toml
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    files_for_root "$r"
    [ ${#R_EXIST[@]} -gt 0 ] || continue
    any=1
    sfx "$r"
    if ! have cargo; then
      emit "format(rust$SFX)" NOT_RUN "cargo not installed"
    elif ! cargo fmt --version >/dev/null 2>&1; then
      emit "format(rust$SFX)" NOT_RUN "rustfmt not installed"
    else
      run_tool "$ROOT/$r" cargo fmt --check
      tool_verdict "format(rust$SFX)" "cargo fmt"
    fi
  done

  if [ $any -eq 0 ]; then emit format NOT_RUN "no changed files with a known formatter"; fi
}

pick_typechecker() {
  local prefer=mypy
  if [ -f "$ROOT/$1/pyrightconfig.json" ] || grep -q '^\[tool\.pyright\]' "$ROOT/$1/pyproject.toml" 2>/dev/null; then
    prefer=pyright
  fi
  for TOOL_NAME in "$prefer" mypy pyright; do
    if find_py_tool "$TOOL_NAME" "$1"; then return 0; fi
  done
  return 1
}

check_lint() {
  local any=0 r
  group_by_marker gomod 1 "" go.mod
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    go_packages "$r"
    if [ ${#PKGS[@]} -eq 0 ]; then
      emit "lint(go$SFX)" NOT_RUN "no changed Go packages"
    elif ! have go; then
      emit "lint(go$SFX)" NOT_RUN "go not installed"
    else
      run_tool "$ROOT/$r" go vet "${PKGS[@]}"
      tool_verdict "lint(go$SFX)" "go vet"
    fi
  done

  group_by_marker tsproj 1 "" tsconfig.json
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ ${#R_FILES[@]} -eq 0 ]; then
      emit "lint(ts$SFX)" NOT_RUN "no changed TS/JS files"
    elif ! find_node_bin "$r" tsc; then
      emit "lint(ts$SFX)" NOT_RUN "typescript not installed locally"
    elif ! have npx; then
      emit "lint(ts$SFX)" NOT_RUN "npx not installed"
    else
      run_tool "$ROOT/$r" npx --no-install tsc --noEmit
      tool_verdict "lint(ts$SFX)" tsc
    fi
  done

  group_by_marker py 1 "" pyproject.toml setup.cfg
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ ${#R_EXIST[@]} -eq 0 ]; then
      emit "lint(python$SFX)" NOT_RUN "no changed Python files"
    elif ! pick_typechecker "$r"; then
      emit "lint(python$SFX)" NOT_RUN "mypy/pyright not installed"
    elif [ "$TOOL_NAME" = mypy ]; then
      run_tool "$ROOT/$r" "$TOOL" --cache-dir=/dev/null "${R_EXIST[@]}"
      tool_verdict "lint(python$SFX)" mypy
    else
      run_tool "$ROOT/$r" "$TOOL" "${R_EXIST[@]}"
      tool_verdict "lint(python$SFX)" pyright
    fi
  done

  group_by_marker rsproj 1 "" Cargo.toml
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ ${#R_FILES[@]} -eq 0 ]; then
      emit "lint(rust$SFX)" NOT_RUN "no changed Rust files"
    elif ! have cargo; then
      emit "lint(rust$SFX)" NOT_RUN "cargo not installed"
    elif ! cargo clippy --version >/dev/null 2>&1; then
      emit "lint(rust$SFX)" NOT_RUN "clippy not installed"
    else
      run_tool "$ROOT/$r" cargo clippy --quiet -- -D warnings
      tool_verdict "lint(rust$SFX)" "cargo clippy"
    fi
  done

  if [ $any -eq 0 ]; then emit lint NOT_RUN "no supported stack detected"; fi
}

js_verdict() {
  if [ "$RC" -eq 0 ] && grep -Eq 'No test files found|No tests found' "$TMP/out"; then
    emit "tests(js$SFX)" NOT_RUN "no related tests found"
  else
    tool_verdict "tests(js$SFX)" "$1"
  fi
}

check_tests() {
  local any=0 r m f
  group_by_marker gomod 1 "" go.mod
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ "$STRICT" -eq 1 ]; then PKGS=(./...); else go_packages "$r"; fi
    if [ ${#PKGS[@]} -eq 0 ]; then
      emit "tests(go$SFX)" NOT_RUN "no changed Go packages"
    elif ! have go; then
      emit "tests(go$SFX)" NOT_RUN "go not installed"
    else
      run_tool "$ROOT/$r" go test "${PKGS[@]}"
      if [ "$RC" -eq 0 ] && ! grep -q '^ok' "$TMP/out"; then
        emit "tests(go$SFX)" NOT_RUN "no Go test files in affected packages"
      else
        tool_verdict "tests(go$SFX)" "go test"
      fi
    fi
  done

  group_by_marker js 1 "" package.json
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ "$STRICT" -ne 1 ] && [ ${#R_EXIST[@]} -eq 0 ]; then
      emit "tests(js$SFX)" NOT_RUN "no changed JS/TS files"
    elif ! have npx; then
      emit "tests(js$SFX)" NOT_RUN "npx not installed"
    elif find_node_bin "$r" vitest; then
      if [ "$STRICT" -eq 1 ]; then
        run_tool "$ROOT/$r" env CI=true npx --no-install vitest run
      else
        run_tool "$ROOT/$r" env CI=true npx --no-install vitest related --run --passWithNoTests "${R_EXIST[@]}"
      fi
      js_verdict vitest
    elif find_node_bin "$r" jest; then
      if [ "$STRICT" -eq 1 ]; then
        run_tool "$ROOT/$r" env CI=true npx --no-install jest --ci
      else
        run_tool "$ROOT/$r" env CI=true npx --no-install jest --ci --passWithNoTests --findRelatedTests "${R_EXIST[@]}"
      fi
      js_verdict jest
    elif [ "$STRICT" -eq 1 ] && grep -q '"test"[[:space:]]*:' "$ROOT/$r/package.json"; then
      run_tool "$ROOT/$r" env CI=true npm test
      tool_verdict "tests(js$SFX)" "npm test"
    else
      emit "tests(js$SFX)" NOT_RUN "no local vitest or jest"
    fi
  done

  group_by_marker py 0 .
  m=${#U_ROOTS[@]}
  for f in pyproject.toml setup.cfg setup.py pytest.ini tox.ini; do
    if [ -e "$ROOT/$f" ]; then m=$((m + 1)); fi
  done
  if [ "$m" -gt 0 ]; then
    any=1
    TARGETS=()
    if [ "$STRICT" -ne 1 ] && [ ${#U_ROOTS[@]} -gt 0 ]; then
      py pytests "$ROOT" "$TMP/changed" "$TMP/pytests"
      while IFS= read -r -d '' f; do TARGETS+=("./$f"); done <"$TMP/pytests"
    fi
    if [ "$STRICT" -ne 1 ] && [ ${#U_ROOTS[@]} -eq 0 ]; then
      emit "tests(python)" NOT_RUN "no changed Python files"
    elif [ "$STRICT" -ne 1 ] && [ ${#TARGETS[@]} -eq 0 ]; then
      emit "tests(python)" NOT_RUN "no tests found for changed Python files"
    elif ! find_pytest; then
      emit "tests(python)" NOT_RUN "pytest not installed"
    else
      run_tool "$ROOT" env PYTHONDONTWRITEBYTECODE=1 "${PYTEST[@]}" -q -p no:cacheprovider ${TARGETS[@]+"${TARGETS[@]}"}
      if [ "$RC" -eq 5 ]; then
        emit "tests(python)" NOT_RUN "no tests collected"
      else
        tool_verdict "tests(python)" pytest
      fi
    fi
  fi

  group_by_marker rsproj 1 "" Cargo.toml
  for r in ${U_ROOTS[@]+"${U_ROOTS[@]}"}; do
    any=1
    sfx "$r"
    files_for_root "$r"
    if [ "$STRICT" -ne 1 ] && [ ${#R_FILES[@]} -eq 0 ]; then
      emit "tests(rust$SFX)" NOT_RUN "no changed Rust files"
    elif ! have cargo; then
      emit "tests(rust$SFX)" NOT_RUN "cargo not installed"
    else
      run_tool "$ROOT/$r" cargo test --quiet
      tool_verdict "tests(rust$SFX)" "cargo test"
    fi
  done

  if [ $any -eq 1 ]; then return 0; fi
  if [ -f "$ROOT/Makefile" ] && grep -q '^test:' "$ROOT/Makefile"; then
    if [ "$STRICT" -ne 1 ]; then
      emit "tests(make)" NOT_RUN "make test is a full suite; run with --strict"
    elif ! have make; then
      emit "tests(make)" NOT_RUN "make not installed"
    else
      run_tool "$ROOT" make test
      tool_verdict "tests(make)" "make test"
    fi
  else
    emit tests NOT_RUN "no test runner detected"
  fi
}

load_scope() {
  local line
  SCOPE_GLOBS=()
  while IFS= read -r line || [ -n "$line" ]; do
    line=${line%$'\r'}
    line=${line#"${line%%[![:space:]]*}"}
    line=${line%"${line##*[![:space:]]}"}
    case $line in '' | '#'*) continue ;; esac
    SCOPE_GLOBS+=("${line#./}")
  done <"$SCOPE_FILE"
}

in_scope() {
  local g
  for g in ${SCOPE_GLOBS[@]+"${SCOPE_GLOBS[@]}"}; do
    case $g in */) g=$g* ;; esac
    # Scope globs must stay unquoted so they match as patterns.
    # shellcheck disable=SC2254
    case $1 in $g | $g/*) return 0 ;; esac
  done
  return 1
}

check_scope() {
  local n=${#CHANGED[@]} f joined= i=0
  if [ -n "$SCOPE_FILE" ]; then
    load_scope
    OUTSIDE=()
    for f in ${CHANGED[@]+"${CHANGED[@]}"}; do
      if ! in_scope "$f"; then OUTSIDE+=("$f"); fi
    done
    if [ ${#OUTSIDE[@]} -gt 0 ]; then
      for f in "${OUTSIDE[@]}"; do
        i=$((i + 1))
        if [ $i -gt $SHOW_MAX ]; then
          joined="$joined, +$((${#OUTSIDE[@]} - SHOW_MAX)) more"
          break
        fi
        if [ -z "$joined" ]; then joined=$f; else joined="$joined, $f"; fi
      done
      emit scope FAIL "outside scope: $joined"
      if [ ${#OUTSIDE[@]} -gt $SHOW_MAX ]; then
        for f in "${OUTSIDE[@]}"; do rec "  $f"; done
      fi
      return 0
    fi
  fi
  if [ "$n" -gt "$MAX_FILES" ]; then
    emit scope WARN "$n files > $MAX_FILES"
    printf '%s\n' "${CHANGED[@]}" >"$TMP/list"
    list_capped "$TMP/list"
  else
    emit scope PASS
  fi
}

write_report() {
  local dir=$ARTDIR/reports stamp i=1 f
  mkdir -p "$dir"
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  REPORT=$dir/gate-$stamp.txt
  while [ -e "$REPORT" ]; do
    i=$((i + 1))
    REPORT=$dir/gate-$stamp-$i.txt
  done
  {
    printf '# arc gate report %s\n' "$stamp"
    printf '# options: max-files=%s strict=%s scope=%s\n' "$MAX_FILES" "$STRICT" "${SCOPE_FILE:-none}"
    cat "$TMP/summary"
    printf '# summary: %s FAIL, %s NOT_RUN, %s WARN\n' "$FAIL_N" "$NOT_RUN_N" "$WARN_N"
    printf '\n## changed files (%s)\n' "${#CHANGED[@]}"
    for f in ${CHANGED[@]+"${CHANGED[@]}"}; do printf '%s\n' "$f"; done
    printf '\n## tool output\n'
    cat "$TMP/details"
  } >"$REPORT"
}

resolve_slug() {
  local raw=$TASK
  if [ -z "$raw" ]; then raw=$(git symbolic-ref --short -q HEAD 2>/dev/null || true); fi
  SLUG=$(printf '%s' "$raw" | LC_ALL=C tr -c 'A-Za-z0-9._-' '-')
  SLUG=${SLUG#"${SLUG%%[!.-]*}"}
  if [ -z "$SLUG" ]; then SLUG=default; fi
}

cmd_start() {
  local head branch count
  if [ -e "$BASELINE" ] && [ "$FORCE" -ne 1 ]; then
    printf '%s: baseline already exists: %s (use --force to overwrite)\n' "$PROG" "$(rel "$BASELINE")" >&2
    exit 1
  fi
  head=$(current_head)
  if [ -z "$head" ]; then head=none; fi
  branch=$(git symbolic-ref --short -q HEAD 2>/dev/null || true)
  mkdir -p "$ARTDIR"
  count=$(py snapshot "$ROOT" "$ARTDIR" "$head" "$branch" "$SLUG" "$BASELINE")
  say "baseline: recorded $(rel "$BASELINE") (head $(short "$head"), $count pre-existing dirty or untracked file(s))"
}

cmd_check() {
  local head base bl_arg= bl_head note= mode=affected f
  FAIL_N=0 NOT_RUN_N=0 WARN_N=0
  : >"$TMP/summary"
  : >"$TMP/details"
  head=$(current_head)
  if [ -f "$BASELINE" ]; then
    bl_arg=$BASELINE
    bl_head=$(py head "$BASELINE")
    if [ "$bl_head" = none ]; then
      base=$(empty_tree)
    elif git cat-file -e "$bl_head^{commit}" 2>/dev/null; then
      base=$bl_head
    else
      base=${head:-$(empty_tree)}
      note="baseline head $(short "$bl_head") not found; using HEAD"
    fi
  else
    base=${head:-$(empty_tree)}
    note="no baseline; using HEAD"
  fi
  if [ "$STRICT" -eq 1 ]; then mode=strict; fi

  py analyze "$ROOT" "$base" "$bl_arg" "$TMP"
  CHANGED=()
  while IFS= read -r -d '' f; do CHANGED+=("$f"); done <"$TMP/changed"

  out "# gate check: task=$SLUG base=$(short "$base") changed=${#CHANGED[@]} mode=$mode"
  if [ -z "$note" ]; then emit baseline PASS; else emit baseline NOT_RUN "$note"; fi
  check_hits placeholders "$TMP/placeholders"
  check_hits suppressions "$TMP/suppressions"
  check_format
  check_lint
  check_tests
  check_scope

  write_report
  say "# summary: $FAIL_N FAIL, $NOT_RUN_N NOT_RUN, $WARN_N WARN"
  say "# report: $(rel "$REPORT")"
  if [ "$FAIL_N" -gt 0 ]; then exit 1; fi
}

main() {
  if [ $# -eq 0 ]; then
    usage >&2
    exit 2
  fi
  CMD=$1
  shift
  case $CMD in
    -h | --help | help)
      usage
      exit 0
      ;;
    start | check) ;;
    *) die "unknown command: $CMD (try --help)" ;;
  esac
  TASK= FORCE=0 STRICT=0 MAX_FILES=20 SCOPE_FILE=
  while [ $# -gt 0 ]; do
    case $1 in
      --task)
        [ $# -ge 2 ] || die "--task needs a value"
        TASK=$2
        shift 2
        ;;
      --task=*)
        TASK=${1#*=}
        shift
        ;;
      --force)
        [ "$CMD" = start ] || die "--force is only valid for start"
        FORCE=1
        shift
        ;;
      --strict)
        [ "$CMD" = check ] || die "--strict is only valid for check"
        STRICT=1
        shift
        ;;
      --max-files)
        [ "$CMD" = check ] || die "--max-files is only valid for check"
        [ $# -ge 2 ] || die "--max-files needs a value"
        MAX_FILES=$2
        shift 2
        ;;
      --scope)
        [ "$CMD" = check ] || die "--scope is only valid for check"
        [ $# -ge 2 ] || die "--scope needs a file"
        SCOPE_FILE=$2
        shift 2
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *) die "unknown option: $1 (try --help)" ;;
    esac
  done
  case $MAX_FILES in '' | *[!0-9]*) die "--max-files needs a non-negative integer" ;; esac
  have git || die "git not found on PATH"
  have python3 || die "python3 is required for baseline and diff parsing but was not found on PATH"
  ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "not inside a git work tree"
  [ -n "$ROOT" ] || die "not inside a git work tree"
  if [ -n "$SCOPE_FILE" ]; then
    case $SCOPE_FILE in /*) ;; *) SCOPE_FILE=$PWD/$SCOPE_FILE ;; esac
    [ -f "$SCOPE_FILE" ] || die "scope file not found: $SCOPE_FILE"
  fi
  cd "$ROOT"
  resolve_slug
  ARTDIR=$ROOT/.arc/artifacts/$SLUG
  BASELINE=$ARTDIR/baseline.json
  TMP=$(mktemp -d "${TMPDIR:-/tmp}/arc-gate.XXXXXX")
  trap 'rm -rf "$TMP"' EXIT
  "cmd_$CMD"
}

main "$@"
