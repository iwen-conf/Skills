#!/usr/bin/env bash
# Zero-venv installer & verifier for the 6-skill architecture across Claude Code, Codex, and Grok Build.
# Installs ONLY the 6 core skills (+ agent-browser / lanhu-design if present on machine) and archives
# all retired/redundant skills safely into ~/.agents/skills-backup-<timestamp>.
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILES="$ROOT/hosts/host_profiles.json"

usage() {
  cat <<'EOF'
usage: sync.sh [--check]
  (no args)   Sync the 6 skills across claude, codex, grok and archive retired skills to backup
  --check     Verify YAML frontmatter, line budgets, relative links, executable scripts, and the 3 host roots
EOF
}

check_all() {
  python3 - "$ROOT" "$PROFILES" <<'PY'
import json, os, re, sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
profiles = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
pool = Path(profiles["source_root"]).expanduser()
link_re = re.compile(r"(?:`|\[.*?\]\()(\.\.?/[A-Za-z0-9_./-]+|(?:scripts|references)/[A-Za-z0-9_./-]+)(?:`|\))")
errors: list[str] = []

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
    m = re.search(r"^name:\s*([a-z0-9-]+)\s*$", text, re.M)
    if not m or m.group(1) != sf.parent.name:
        errors.append(f"{sf.relative_to(root)}: name mismatch for '{sf.parent.name}'")
    if not re.search(r"^version:\s*\d+\.\d+\.\d+\s*$", text, re.M):
        errors.append(f"{sf.relative_to(root)}: missing semver version")
    if not (sf.parent / "agents" / "openai.yaml").is_file():
        errors.append(f"{sf.relative_to(root)}: missing agents/openai.yaml")
    for rel_ref in set(link_re.findall(text)):
        if not (sf.parent / rel_ref).exists():
            errors.append(f"{sf.relative_to(root)}: broken relative reference '{rel_ref}'")

for shared_doc in (
    "contract.md", "handoff.md", "ui-system.md", "api-contract.md",
    "go-conventions.md", "android.md", "harmonyos.md", "apple-native.md", "cnb.md"
):
    if not (root / "skills" / "_shared" / shared_doc).is_file():
        errors.append(f"missing shared spec: skills/_shared/{shared_doc}")

for sc in (
    "scripts/sync.sh",
    "scripts/arc-wave.py",
    "skills/arc-code/scripts/gate.sh",
    "skills/arc-audit/scripts/security-scan.py",
    "skills/arc-plan/scripts/check_closure.py",
    "skills/arc-plan/scripts/github_health.py",
):
    sp = root / sc
    if not sp.is_file() or not os.access(sp, os.X_OK):
        errors.append(f"missing or non-executable script: {sc}")

if pool.is_dir():
    allowed = expected | set(profiles.get("allowed_external", [])) | {"_shared"}
    for host, hspec in profiles["hosts"].items():
        hroot = Path(hspec["root"]).expanduser()
        ignore = set(hspec.get("ignore", []))
        if not (hroot / "_shared" / "contract.md").is_file():
            errors.append(f"{host}: _shared/contract.md unreachable in {hroot}")
        for name in sorted(expected):
            sdir = hroot / name
            if not (sdir / "SKILL.md").is_file():
                errors.append(f"{host}: missing installed skill '{name}'")
                continue
            for rel_ref in set(link_re.findall((sdir / "SKILL.md").read_text(encoding="utf-8"))):
                if not (sdir / rel_ref).exists():
                    errors.append(f"{host}/{name}: broken installed path '{rel_ref}'")
        if hroot.is_dir():
            leaked = [
                p.name for p in hroot.iterdir()
                if not p.name.startswith(".") and p.name not in ignore and p.name not in allowed
            ]
            if leaked:
                errors.append(f"{host}: unexpected extra skills still mounted: {leaked}")

if errors:
    print("\n".join(f"FAIL: {e}" for e in errors), file=sys.stderr)
    sys.exit(1)
print("OK: validated 6 lean skills, 9 shared stack specs, 6 deterministic scripts, and clean 3-host mounts.")
PY
}

case "${1:-}" in
  --help|-h) usage; exit 0 ;;
  --check)   check_all; exit 0 ;;
  "") ;;
  *) usage >&2; exit 2 ;;
esac

chmod +x \
  "$ROOT/scripts/sync.sh" \
  "$ROOT/scripts/arc-wave.py" \
  "$ROOT/skills/arc-code/scripts/gate.sh" \
  "$ROOT/skills/arc-audit/scripts/security-scan.py" \
  "$ROOT/skills/arc-plan/scripts/check_closure.py" \
  "$ROOT/skills/arc-plan/scripts/github_health.py"

python3 - "$ROOT" "$PROFILES" <<'PY'
import datetime as dt, json, shutil, sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
profiles = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
pool = Path(profiles["source_root"]).expanduser()
bin_dir = Path.home() / ".local" / "bin"
pool.mkdir(parents=True, exist_ok=True)
bin_dir.mkdir(parents=True, exist_ok=True)

stamp = dt.datetime.now().strftime("%Y%m%d-%H%M%S")
backup_root = Path.home() / ".agents" / f"skills-backup-{stamp}"

managed = set(profiles["skills"])
allowed_ext = set(profiles.get("allowed_external", []))
keep_in_pool = managed | allowed_ext | {"_shared"}

def move_to_backup(entry: Path, bucket: str) -> None:
    dest = backup_root / bucket
    dest.mkdir(parents=True, exist_ok=True)
    shutil.move(str(entry), str(dest / entry.name))

def force_symlink(src: Path, dst: Path) -> None:
    if dst.is_symlink() or dst.is_file():
        dst.unlink()
    elif dst.is_dir():
        move_to_backup(dst, "replaced")
    dst.symlink_to(src)

# 1. Link declared external targets (e.g., lanhu-design) if present on disk
for ext_name, ext_path_str in profiles.get("external_targets", {}).items():
    ext_path = Path(ext_path_str).expanduser()
    if ext_name in allowed_ext and (ext_path / "SKILL.md").is_file() and not (pool / ext_name).exists():
        force_symlink(ext_path, pool / ext_name)

# 2. Archive everything outside the 6 repo skills + allowed external skills
for entry in sorted(pool.iterdir()):
    if entry.name.startswith("."):
        continue
    if entry.name not in keep_in_pool:
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

# 4. Sync the 3 official hosts cleanly
for host, hspec in profiles["hosts"].items():
    hroot = Path(hspec["root"]).expanduser()
    hroot.mkdir(parents=True, exist_ok=True)
    ignore = set(hspec.get("ignore", []))

    for entry in sorted(hroot.iterdir()):
        if entry.name.startswith(".") or entry.name in ignore:
            continue
        if entry.name not in set(all_active) | {"_shared"}:
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

            chunks = [(src_dir / "SKILL.md").read_text(encoding="utf-8").rstrip()]
            if name in managed:
                chunks.append((root / "skills" / "_shared" / "contract.md").read_text(encoding="utf-8").rstrip())
            if name in {"arc-code", "arc-audit"}:
                chunks.append((root / "skills" / "_shared" / "ui-system.md").read_text(encoding="utf-8").rstrip())
                chunks.append((root / "skills" / "_shared" / "go-conventions.md").read_text(encoding="utf-8").rstrip())
            if name == "check":
                chunks.append((root / "skills" / "_shared" / "cnb.md").read_text(encoding="utf-8").rstrip())
            cheat = src_dir / "references" / "cheatsheet.md"
            if cheat.is_file():
                chunks.append(cheat.read_text(encoding="utf-8").rstrip())
            (dest_dir / "SKILL.md").write_text("\n\n---\n\n".join(chunks) + "\n", encoding="utf-8")

            for sub in ("scripts", "references", "agents"):
                if (src_dir / sub).is_dir():
                    force_symlink(src_dir / sub, dest_dir / sub)
    else:
        for name in all_active:
            force_symlink(pool / name, hroot / name)

force_symlink(root / "scripts" / "arc-wave.py", bin_dir / "arc-wave")
print(f"Synced 6 core skills + {len(active_ext)} external tool skills ({', '.join(active_ext) or 'none'}) across claude, codex, grok.")
if backup_root.exists():
    print(f"Archived retired/extra skills safely to {backup_root}")
PY

check_all
