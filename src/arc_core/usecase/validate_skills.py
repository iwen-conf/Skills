from __future__ import annotations
from pathlib import Path
from ..domain.validation import validate_text
from ..infrastructure.markdown import parse_frontmatter
from ..domain.skill import is_arc_skill, get_namespace_dir
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
    return errors

def collect_skill_files(root: Path) -> list[Path]:
    search_root = root / "Arc" if (root / "Arc").is_dir() else root
    collected: list[Path] = []
    for path in sorted(search_root.rglob("SKILL.md")):
        text = path.read_text(encoding="utf-8")
        frontmatter, error = parse_frontmatter(text)
        if error:
            continue
        skill_name = frontmatter.get("name", "")
        if is_arc_skill(skill_name):
            collected.append(path)
    return collected

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
    all_errors: list[str] = validate_repo_policies(root)
    all_warnings: list[str] = []
    for path in skill_files:
        errors, warnings = validate_file(path, root=root)
        all_errors.extend(errors)
        all_warnings.extend(warnings)
    return all_errors, all_warnings, len(skill_files)
