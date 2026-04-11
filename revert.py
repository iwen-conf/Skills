import os
import re
from pathlib import Path

ROOT = Path(".")

# 1. Gather all skills
arc_dir = ROOT / "Arc"
skills = []
if arc_dir.exists():
    for d in arc_dir.iterdir():
        if d.is_dir() and d.name.startswith("arc-"):
            skills.append(d.name[4:])

print("Skills found:", skills)

# 2. Update schema
schema_path = ROOT / "schemas" / "skill.schema.json"
if schema_path.exists():
    content = schema_path.read_text()
    if '"pattern": "^[a-z0-9-]+$"' in content:
        content = content.replace('"pattern": "^[a-z0-9-]+$"', '"pattern": "^[a-z0-9:-]+$"')
        schema_path.write_text(content)

# 3. Replace text in all files
for ext in ["*.md", "*.py", "*.json", "*.html", "*.yaml", "*.yml", "*.sh", "*.go"]:
    for file_path in ROOT.rglob(ext):
        if ".git/" in str(file_path) or "node_modules" in str(file_path) or ".venv" in str(file_path):
            continue
        if file_path.name in ["revert.py", "fix_skills.py"]:
            continue
        if file_path.is_file():
            try:
                content = file_path.read_text(encoding="utf-8")
                new_content = content
                
                for skill in skills:
                    old_str = f"arc-{skill}"
                    new_str = f"arc:{skill}"
                    new_content = new_content.replace(old_str, new_str)
                
                if new_content != content:
                    file_path.write_text(new_content, encoding="utf-8")
                    print(f"Updated {file_path}")
            except Exception as e:
                print(f"Could not process {file_path}: {e}")

# 4. Rename directories
for skill in skills:
    old_dir = arc_dir / f"arc-{skill}"
    new_dir = arc_dir / f"arc:{skill}"
    if old_dir.exists():
        os.system(f'git mv "{old_dir}" "{new_dir}" || mv "{old_dir}" "{new_dir}"')
        print(f"Renamed {old_dir} to {new_dir}")

