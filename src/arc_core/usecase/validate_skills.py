from __future__ import annotations
import re
from pathlib import Path
from ..domain.validation import validate_root_cause_gate, validate_text
from ..infrastructure.markdown import parse_frontmatter
from ..domain.skill import (
    ARC_LOAD_SCENARIOS,
    ARC_REFERENCE_MAX_TOKENS,
    ARC_SHARED_DIR,
    SOURCE_SKILL_PATHS,
    VERTICAL_SKILL_ROOTS,
    estimate_tokens,
    get_namespace_dir,
)
from ..domain.engineering import (
    validate_line_budget,
    validate_relative_links,
    validate_retired_names,
    validate_token_budget,
)
from ..domain.triggers import load_trigger_corpus

def validate_file(path: Path, root: Path | None = None) -> tuple[list[str], list[str]]:
    text = path.read_text(encoding="utf-8")
    return validate_text(text, str(path), root=root, skill_path=path)

def validate_repo_policies(root: Path) -> list[str]:
    errors: list[str] = []
    workflows_dir = root / ".github" / "workflows"
    if workflows_dir.exists():
        workflow_files = sorted(
            path.relative_to(root)
            for path in workflows_dir.rglob("*")
            if path.is_file()
        )
        if workflow_files:
            listed = ", ".join(str(path) for path in workflow_files[:5])
            if len(workflow_files) > 5:
                listed += ", ..."
            errors.append(
                "repository policy violation: GitHub Actions workflows are not allowed in this skills repository; "
                f"remove {listed}"
            )
        else:
            errors.append(
                "repository policy violation: empty .github/workflows directory is not allowed in this skills repository"
            )
    corpus = load_trigger_corpus(root)
    skill_names: set[str] = set()
    for path in collect_skill_files(root):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        name = str(frontmatter.get("name", "") or "")
        if not name:
            continue
        skill_names.add(name)
        if name not in corpus:
            errors.append(f"trigger corpus missing skill {name}")
    extra = sorted(set(corpus) - skill_names)
    if extra:
        errors.append(f"trigger corpus has unknown skills: {', '.join(extra)}")
    for name, entry in corpus.items():
        terms = [str(term).strip() for term in entry.get("must_contain", []) if str(term).strip()]
        positives = entry.get("positive", [])
        negatives = entry.get("negative", [])
        if len(terms) < 3:
            errors.append(f"trigger corpus {name} needs at least 3 must_contain terms")
        if len(positives) < 8:
            errors.append(f"trigger corpus {name} needs at least 8 positive utterances")
        if len(negatives) < 3:
            errors.append(f"trigger corpus {name} needs at least 3 negative utterances")

    seen_names: dict[str, Path] = {}
    for path in [*collect_skill_files(root), *collect_vertical_skill_files(root), *collect_source_skill_files(root)]:
        frontmatter, error = parse_frontmatter(path.read_text(encoding="utf-8"))
        if error:
            continue
        name = str(frontmatter.get("name", "") or "")
        if not name:
            continue
        previous = seen_names.get(name)
        if previous is not None:
            errors.append(f"duplicate skill name {name}: {previous} and {path}")
        else:
            seen_names[name] = path
    errors.extend(validate_arc_layout(root))
    errors.extend(validate_load_scenarios(root))
    return errors


def validate_load_scenarios(root: Path) -> list[str]:
    """Typical Arc tasks must stay under their summed load ceiling."""
    if not (root / "Arc").is_dir():
        return []
    errors: list[str] = []
    for name, (ceiling, paths) in ARC_LOAD_SCENARIOS.items():
        missing = [path for path in paths if not (root / path).is_file()]
        if missing:
            errors.append(f"load scenario {name}: missing {', '.join(missing)}")
            continue
        total = sum(estimate_tokens((root / path).read_text(encoding="utf-8")) for path in paths)
        if total > ceiling:
            errors.append(f"load scenario {name}: {total} estimated tokens exceeds {ceiling}")
    return errors


def validate_arc_layout(root: Path) -> list[str]:
    """Arc/ holds only skill packages and the undiscoverable shared layer."""
    arc_root = root / "Arc"
    if not arc_root.is_dir():
        return []
    errors: list[str] = []
    for entry in sorted(arc_root.iterdir()):
        if not entry.is_dir() or entry.name.startswith("."):
            continue
        if entry.name == ARC_SHARED_DIR:
            if any(entry.rglob("SKILL.md")):
                errors.append(f"{entry}: shared layer must not contain SKILL.md")
            for path in sorted(entry.rglob("*.md")):
                text = path.read_text(encoding="utf-8")
                errors.extend(validate_line_budget(text, str(path), is_skill_md=False))
                errors.extend(validate_token_budget(text, str(path), ARC_REFERENCE_MAX_TOKENS, "shared reference"))
                errors.extend(validate_relative_links(text, str(path), path.parent))
                errors.extend(validate_retired_names(text, str(path)))
        elif not (entry / "SKILL.md").is_file():
            errors.append(f"{entry}: Arc directories must be skill packages or {ARC_SHARED_DIR}/")
    return errors

def collect_skill_files(root: Path) -> list[Path]:
    # 先按包布局收集，再验证内容，避免损坏的 frontmatter 让入口从门禁中消失。
    if (root / "Arc").is_dir():
        return sorted(path for path in (root / "Arc").glob("*/SKILL.md") if path.is_file())
    return sorted(path for path in root.glob("arc:*/SKILL.md") if path.is_file())


def collect_vertical_skill_files(root: Path) -> list[Path]:
    """Collect bundled vertical entrypoints without applying the Arc router contract."""
    files: list[Path] = []
    for directory in VERTICAL_SKILL_ROOTS:
        search_root = root / directory
        if not search_root.is_dir():
            continue
        files.extend(sorted(path for path in search_root.glob("*/SKILL.md") if path.is_file()))
    return files


def collect_library_members(root: Path) -> set[str]:
    """Source skills a router bundles as undiscoverable library entries (metadata.library)."""
    members: set[str] = set()
    for path in [*collect_skill_files(root), *collect_vertical_skill_files(root)]:
        frontmatter, error = parse_frontmatter(path.read_text(encoding="utf-8"))
        if error:
            continue
        library = (frontmatter.get("metadata") or {}).get("library") or []
        if isinstance(library, list):
            members.update(str(name) for name in library)
    return members


def collect_source_skill_files(root: Path) -> list[Path]:
    return [root / path for path in SOURCE_SKILL_PATHS if (root / path).is_file()]


def validate_vertical_skill_files(root: Path) -> list[str]:
    return validate_basic_skill_files(collect_vertical_skill_files(root))


def validate_basic_skill_files(paths: list[Path]) -> list[str]:
    errors: list[str] = []
    for path in paths:
        label = str(path)
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            errors.append(f"{label}: {error}")
            continue
        name = str(frontmatter.get("name", "") or "")
        description = str(frontmatter.get("description", "") or "")
        if not name:
            errors.append(f"{label}: missing frontmatter name")
        elif not re.fullmatch(r"[a-z0-9-]+", name):
            errors.append(f"{label}: vertical skill name contains unsupported characters")
        if not description:
            errors.append(f"{label}: missing frontmatter description")
        errors.extend(validate_root_cause_gate(text, name, label))
        errors.extend(validate_line_budget(text, label, is_skill_md=True))
        errors.extend(validate_relative_links(text, label, path.parent))
    return errors

def find_skill_file(root: Path, skill_name: str) -> Path | None:
    namespace_dir = get_namespace_dir(skill_name)
    candidates: list[Path] = []
    if namespace_dir:
        candidates.append(root / namespace_dir / skill_name / "SKILL.md")
    candidates.append(root / skill_name / "SKILL.md")

    seen: set[Path] = set()
    for candidate in candidates:
        if candidate in seen:
            continue
        seen.add(candidate)
        if candidate.exists():
            return candidate

    for path in collect_skill_files(root):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        if frontmatter.get("name", "") == skill_name:
            return path
    return None

def run_validation(root: Path) -> tuple[list[str], list[str], int]:
    skill_files = collect_skill_files(root)
    vertical_skill_files = collect_vertical_skill_files(root)
    source_skill_files = collect_source_skill_files(root)
    all_errors: list[str] = validate_repo_policies(root)
    all_errors.extend(validate_vertical_skill_files(root))
    all_errors.extend(validate_basic_skill_files(source_skill_files))
    all_warnings: list[str] = []
    for path in skill_files:
        errors, warnings = validate_file(path, root=root)
        all_errors.extend(errors)
        all_warnings.extend(warnings)
    return all_errors, all_warnings, len(skill_files) + len(vertical_skill_files) + len(source_skill_files)
