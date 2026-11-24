#!/usr/bin/env bash
# Zero-venv installer & verifier for the 6-skill architecture across Claude Code, Codex, and Grok Build.
# Installs ONLY the 6 core skills (+ agent-browser / lanhu-design if present on machine) and archives
# all retired/redundant skills safely into ~/.agents/skills-backup-<timestamp>.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILES="$ROOT/hosts/host_profiles.json"

usage() {
  cat <<'EOF'
usage: sync.sh [--check] [--prune-extra]
  (no args)      Sync the 6 skills across claude, codex, grok, and gemini (retains third-party user skills)
  --prune-extra  Also archive non-core / unlisted third-party skills into ~/.agents/skills-backup-<timestamp>
  --check        Verify YAML frontmatter, openai.yaml bounds, line budgets, links, script syntax/exec bits, and host mounts
EOF
}

PRUNE_EXTRA=0

check_all() {
  python3 - "$ROOT" "$PROFILES" <<'PY'
import json, os, py_compile, re, subprocess, sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
profiles = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
pool = Path(profiles["source_root"]).expanduser()
link_re = re.compile(r"(?:`|\[.*?\]\()(\.\.?/[A-Za-z0-9_./-]+|(?:scripts|references)/[A-Za-z0-9_./-]+|~/.agents/skills/[A-Za-z0-9_./-]+)(?:`|\))")
errors: list[str] = []


def build_inline_bundle(src_dir: Path, name: str, managed: set[str]) -> str:
    chunks = [(src_dir / "SKILL.md").read_text(encoding="utf-8").rstrip()]
    if name in managed:
        chunks.append((root / "skills" / "_shared" / "contract.md").read_text(encoding="utf-8").rstrip())
    if name in {"arc-code", "arc-audit"}:
        chunks.append((root / "skills" / "_shared" / "ui-system.md").read_text(encoding="utf-8").rstrip())
        chunks.append((root / "skills" / "_shared" / "debugging.md").read_text(encoding="utf-8").rstrip())
        chunks.append((root / "skills" / "_shared" / "go-conventions.md").read_text(encoding="utf-8").rstrip())
    if name in {"arc-plan", "arc-code", "arc-audit"}:
        chunks.append((root / "skills" / "_shared" / "web-frontend.md").read_text(encoding="utf-8").rstrip())
    if name == "check":
        chunks.append((root / "skills" / "_shared" / "cnb.md").read_text(encoding="utf-8").rstrip())
    if name == "arc-verify":
        catalog = src_dir / "references" / "code-testing-catalog.md"
        if catalog.is_file():
            chunks.append(catalog.read_text(encoding="utf-8").rstrip())
    cheat = src_dir / "references" / "cheatsheet.md"
    if cheat.is_file():
        chunks.append(cheat.read_text(encoding="utf-8").rstrip())
    return "\n\n---\n\n".join(chunks) + "\n"


def check_ref(base_dir: Path, ref: str) -> bool:
    if ref.startswith("~/.agents/skills/"):
        sub = ref[len("~/.agents/skills/"):]
        return (root / "skills" / sub).exists() or (pool / sub).exists()
    return (base_dir / ref).exists()


def discover_linked_roots(pool_dir: Path, excluded: set[Path]) -> list[Path]:
    home = Path.home()
    try:
        shared_target = (pool_dir / "_shared").resolve()
    except OSError:
        return []
    found_roots: set[Path] = set()
    for pat in (".*/skills", ".*/*/skills", ".config/*/skills", ".config/*/*/skills"):
        for p in home.glob(pat):
            if not p.is_dir():
                continue
            if any(part.startswith("skills-backup") or part.startswith("skills.bak") for part in p.parts):
                continue
            try:
                rp = p.resolve()
                sh = p / "_shared"
                if rp in excluded or not sh.is_symlink() or sh.resolve() != shared_target:
                    continue
            except OSError:
                continue
            found_roots.add(p)
    return sorted(found_roots)


expected = set(profiles["skills"])
skill_files = sorted(root.glob("skills/*/SKILL.md"))
found = {sf.parent.name for sf in skill_files}
if found != expected:
    errors.append(f"expected exactly 6 skills {sorted(expected)}, found {sorted(found)}")

for sf in skill_files:
    text = sf.read_text(encoding="utf-8")
    lines = text.splitlines()
    if len(lines) > 80:
        errors.append(f"{sf.relative_to(root)}: exceeds 80-line budget ({len(lines)} lines)")
    if not (lines and lines[0].strip() == "---" and "---" in [l.strip() for l in lines[1:10]]):
        errors.append(f"{sf.relative_to(root)}: invalid YAML frontmatter")
    else:
        end_fm = [i for i, l in enumerate(lines[1:10], 1) if l.strip() == "---"][0]
        fm_body = "\n".join(lines[1:end_fm])
        desc_m = re.search(r"^description:\s*(.+)$", fm_body, re.M)
        if desc_m and ("<" in desc_m.group(1) or ">" in desc_m.group(1)):
            errors.append(f"{sf.relative_to(root)}: frontmatter description contains '<' or '>'")
    m = re.search(r"^name:\s*([a-z0-9-]+)\s*$", text, re.M)
    if not m or m.group(1) != sf.parent.name:
        errors.append(f"{sf.relative_to(root)}: name mismatch for '{sf.parent.name}'")
    if not re.search(r"^version:\s*\d+\.\d+\.\d+\s*$", text, re.M):
        errors.append(f"{sf.relative_to(root)}: missing semver version")
    oai = sf.parent / "agents" / "openai.yaml"
    if not oai.is_file():
        errors.append(f"{sf.relative_to(root)}: missing agents/openai.yaml")
    else:
        sd = re.search(r'^\s*short_description:\s*"([^"]*)"\s*$', oai.read_text(encoding="utf-8"), re.M)
        if not sd or not (25 <= len(sd.group(1)) <= 64):
            got = len(sd.group(1)) if sd else 0
            errors.append(f"{oai.relative_to(root)}: short_description length {got} must be 25..64")
    for rel_ref in set(link_re.findall(text)):
        if not check_ref(sf.parent, rel_ref):
            errors.append(f"{sf.relative_to(root)}: broken relative reference '{rel_ref}'")

for shared_doc in (
    "contract.md", "handoff.md", "ui-system.md", "web-frontend.md", "debugging.md", "api-contract.md",
    "go-conventions.md", "android.md", "harmonyos.md", "apple-native.md", "cnb.md"
):
    if not (root / "skills" / "_shared" / shared_doc).is_file():
        errors.append(f"missing shared spec: skills/_shared/{shared_doc}")

for sc in (
    "scripts/sync.sh",
    "scripts/install_arc.sh",
    "scripts/arc-wave.py",
    "skills/arc-code/scripts/gate.sh",
    "skills/arc-audit/scripts/security-scan.py",
    "skills/arc-plan/scripts/check_closure.py",
    "skills/arc-plan/scripts/github_health.py",
    "tools/arc/smoke.sh",
):
    sp = root / sc
    if not sp.is_file() or not os.access(sp, os.X_OK):
        errors.append(f"missing or non-executable script: {sc}")
        continue
    if sp.suffix == ".py":
        try:
            py_compile.compile(str(sp), doraise=True)
        except py_compile.PyCompileError as exc:
            errors.append(f"python syntax error in {sc}: {exc.msg}")
    elif sp.suffix == ".sh":
        res = subprocess.run(["bash", "-n", str(sp)], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if res.returncode != 0:
            errors.append(f"bash syntax error in {sc}: {res.stderr.strip()}")

extra_checked = 0
if pool.is_dir():
    configured_roots = {pool.resolve()}
    for host, hspec in profiles["hosts"].items():
        hroot = Path(hspec["root"]).expanduser()
        if not hroot.is_dir() and not hroot.parent.is_dir():
            continue
        configured_roots.add(hroot.resolve())
        ignore = set(hspec.get("ignore", []))
        if not (hroot / "_shared" / "contract.md").is_file():
            errors.append(f"{host}: _shared/contract.md unreachable in {hroot}")
        if not (hroot / "_shared" / "web-frontend.md").is_file():
            errors.append(f"{host}: _shared/web-frontend.md unreachable in {hroot}")
        for name in sorted(expected):
            sdir = hroot / name
            if not (sdir / "SKILL.md").is_file():
                errors.append(f"{host}: missing installed skill '{name}'")
                continue
            if hspec.get("mode") == "inline_bundle":
                expected_bundle = build_inline_bundle(root / "skills" / name, name, expected)
                actual_bundle = (sdir / "SKILL.md").read_text(encoding="utf-8")
                if actual_bundle != expected_bundle:
                    errors.append(f"{host}/{name}: inline_bundle SKILL.md out of sync with repository (run scripts/sync.sh)")
            for rel_ref in set(link_re.findall((sdir / "SKILL.md").read_text(encoding="utf-8"))):
                if not check_ref(sdir, rel_ref):
                    errors.append(f"{host}/{name}: broken installed path '{rel_ref}'")
        if hroot.is_dir():
            retired = [
                p.name for p in hroot.iterdir()
                if not p.name.startswith(".") and p.name not in ignore and (p.name.startswith("arc:") or (p.is_symlink() and not p.exists()))
            ]
            if retired:
                errors.append(f"{host}: retired or broken skills still mounted: {retired}")

    for eroot in discover_linked_roots(pool, configured_roots):
        extra_checked += 1
        if not (eroot / "_shared" / "web-frontend.md").is_file():
            errors.append(f"{eroot}: _shared/web-frontend.md unreachable")
        for name in sorted(expected):
            if not (eroot / name / "SKILL.md").is_file():
                errors.append(f"{eroot}: missing installed skill '{name}'")
        retired = [
            p.name for p in eroot.iterdir()
            if not p.name.startswith(".") and (p.name.startswith("arc:") or (p.is_symlink() and not p.exists()))
        ]
        if retired:
            errors.append(f"{eroot}: retired or broken skills still mounted: {retired}")

if errors:
    print("\n".join(f"FAIL: {e}" for e in errors), file=sys.stderr)
    sys.exit(1)
print(f"OK: validated 6 lean skills, openai.yaml bounds, 11 shared stack specs, 8 scripts, host mounts, and {extra_checked} linked agent roots.")
PY
}

while [ $# -gt 0 ]; do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --check)   check_all; exit 0 ;;
    --prune-extra) PRUNE_EXTRA=1; shift ;;
    *) usage >&2; exit 2 ;;
  esac
done

chmod +x \
  "$ROOT/scripts/sync.sh" \
  "$ROOT/scripts/install_arc.sh" \
  "$ROOT/scripts/arc-wave.py" \
  "$ROOT/skills/arc-code/scripts/gate.sh" \
  "$ROOT/skills/arc-audit/scripts/security-scan.py" \
  "$ROOT/skills/arc-plan/scripts/check_closure.py" \
  "$ROOT/skills/arc-plan/scripts/github_health.py" \
  "$ROOT/tools/arc/smoke.sh"

python3 - "$ROOT" "$PROFILES" "$PRUNE_EXTRA" <<'PY'
import datetime as dt, json, os, shutil, sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
profiles = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
prune_extra = sys.argv[3] == "1"
pool = Path(profiles["source_root"]).expanduser()
bin_dir = Path.home() / ".local" / "bin"
pool.mkdir(parents=True, exist_ok=True)
bin_dir.mkdir(parents=True, exist_ok=True)

stamp = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
backup_root = Path.home() / ".agents" / f"skills-backup-{stamp}-{os.getpid()}"

managed = set(profiles["skills"])
allowed_ext = set(profiles.get("allowed_external", []))
keep_in_pool = managed | allowed_ext | {"_shared"}


def build_inline_bundle(src_dir: Path, name: str) -> str:
    chunks = [(src_dir / "SKILL.md").read_text(encoding="utf-8").rstrip()]
    if name in managed:
        chunks.append((root / "skills" / "_shared" / "contract.md").read_text(encoding="utf-8").rstrip())
    if name in {"arc-code", "arc-audit"}:
        chunks.append((root / "skills" / "_shared" / "ui-system.md").read_text(encoding="utf-8").rstrip())
        chunks.append((root / "skills" / "_shared" / "debugging.md").read_text(encoding="utf-8").rstrip())
        chunks.append((root / "skills" / "_shared" / "go-conventions.md").read_text(encoding="utf-8").rstrip())
    if name in {"arc-plan", "arc-code", "arc-audit"}:
        chunks.append((root / "skills" / "_shared" / "web-frontend.md").read_text(encoding="utf-8").rstrip())
    if name == "check":
        chunks.append((root / "skills" / "_shared" / "cnb.md").read_text(encoding="utf-8").rstrip())
    if name == "arc-verify":
        catalog = src_dir / "references" / "code-testing-catalog.md"
        if catalog.is_file():
            chunks.append(catalog.read_text(encoding="utf-8").rstrip())
    cheat = src_dir / "references" / "cheatsheet.md"
    if cheat.is_file():
        chunks.append(cheat.read_text(encoding="utf-8").rstrip())
    return "\n\n---\n\n".join(chunks) + "\n"


def move_to_backup(entry: Path, bucket: str) -> None:
    dest = backup_root / bucket
    dest.mkdir(parents=True, exist_ok=True)
    target = dest / entry.name
    if target.exists():
        shutil.rmtree(target) if target.is_dir() and not target.is_symlink() else target.unlink()
    shutil.move(str(entry), str(target))


def force_symlink(src: Path, dst: Path) -> None:
    if dst.is_symlink() or dst.is_file():
        dst.unlink()
    elif dst.is_dir():
        move_to_backup(dst, "replaced")
    dst.symlink_to(src)


def should_archive(entry: Path, allowed: set[str]) -> bool:
    if entry.name.startswith("arc:") or (entry.is_symlink() and not entry.exists()):
        return True
    return prune_extra and entry.name not in allowed


# 1. Link declared external targets (e.g., lanhu-design) if present on disk
for ext_name, ext_path_str in profiles.get("external_targets", {}).items():
    ext_path = Path(ext_path_str).expanduser()
    if ext_name in allowed_ext and (ext_path / "SKILL.md").is_file() and not (pool / ext_name).exists():
        force_symlink(ext_path, pool / ext_name)

# 2. Archive retired skills (or unlisted extras when --prune-extra is set)
for entry in sorted(pool.iterdir()):
    if entry.name.startswith("."):
        continue
    if should_archive(entry, keep_in_pool):
        move_to_backup(entry, "pool")

# 3. Link _shared and the 6 skills into ~/.agents/skills
force_symlink(root / "skills" / "_shared", pool / "_shared")
for name in profiles["skills"]:
    force_symlink(root / "skills" / name, pool / name)

active_ext = sorted(
    name for name in allowed_ext
    if (pool / name / "SKILL.md").is_file()
)
all_active = [*profiles["skills"], *active_ext]

# 4. Sync configured hosts cleanly
configured_roots = {pool.resolve()}
synced_hosts: list[str] = []
for host, hspec in profiles["hosts"].items():
    hroot = Path(hspec["root"]).expanduser()
    if (host == "gemini" or hspec.get("optional")) and not hroot.parent.is_dir():
        continue
    hroot.mkdir(parents=True, exist_ok=True)
    configured_roots.add(hroot.resolve())
    synced_hosts.append(host)
    ignore = set(hspec.get("ignore", []))

    for entry in sorted(hroot.iterdir()):
        if entry.name.startswith(".") or entry.name in ignore:
            continue
        if should_archive(entry, set(all_active) | {"_shared"}):
            move_to_backup(entry, host)

    force_symlink(pool / "_shared", hroot / "_shared")

    if hspec["mode"] == "inline_bundle":
        for name in all_active:
            src_dir = pool / name
            dest_dir = hroot / name
            if dest_dir.is_symlink():
                dest_dir.unlink()
            elif dest_dir.is_dir():
                shutil.rmtree(dest_dir)
            dest_dir.mkdir(parents=True, exist_ok=True)

            (dest_dir / "SKILL.md").write_text(build_inline_bundle(src_dir, name), encoding="utf-8")

            for sub in ("scripts", "references", "agents"):
                if (src_dir / sub).is_dir():
                    force_symlink(src_dir / sub, dest_dir / sub)
    else:
        for name in all_active:
            force_symlink(pool / name, hroot / name)

# 5. Sync all other existing local agent skill roots already linked to pool/_shared
home = Path.home()
shared_target = (pool / "_shared").resolve()
extra_roots: set[Path] = set()
for pat in (".*/skills", ".*/*/skills", ".config/*/skills", ".config/*/*/skills"):
    for p in home.glob(pat):
        if not p.is_dir():
            continue
        if any(part.startswith("skills-backup") or part.startswith("skills.bak") for part in p.parts):
            continue
        try:
            rp = p.resolve()
            sh = p / "_shared"
            if rp in configured_roots or not sh.is_symlink() or sh.resolve() != shared_target:
                continue
        except OSError:
            continue
        extra_roots.add(p)

extra_synced = 0
for eroot in sorted(extra_roots):
    extra_synced += 1
    bucket = eroot.relative_to(home).as_posix().replace("/", "_")
    for entry in sorted(eroot.iterdir()):
        if entry.name.startswith("."):
            continue
        if should_archive(entry, set(all_active) | {"_shared"}):
            move_to_backup(entry, bucket)
    force_symlink(pool / "_shared", eroot / "_shared")
    for name in all_active:
        force_symlink(pool / name, eroot / name)

force_symlink(root / "scripts" / "arc-wave.py", bin_dir / "arc-wave")
print(
    f"Synced 6 core skills + {len(active_ext)} external tool skills "
    f"({', '.join(active_ext) or 'none'}) across {len(synced_hosts)} configured hosts "
    f"({', '.join(synced_hosts)}) and {extra_synced} additional linked agent roots."
)
if backup_root.exists():
    print(f"Archived retired/extra skills safely to {backup_root}")
PY

check_all
