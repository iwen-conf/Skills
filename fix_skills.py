import os
import re
from pathlib import Path
import shutil

ROOT = Path(".")

# 1. Update schemas/skill.schema.json
schema_path = ROOT / "schemas" / "skill.schema.json"
if schema_path.exists():
    content = schema_path.read_text()
    content = content.replace('"pattern": "^[a-z0-9:-]+$"', '"pattern": "^[a-z0-9-]+$"')
    schema_path.write_text(content)

# 2. Rename directories
arc_dir = ROOT / "Arc"
if arc_dir.exists():
    for d in list(arc_dir.glob("arc:*")):
        if d.is_dir():
            new_name = d.name.replace(":", "-")
            new_path = d.parent / new_name
            print(f"Renaming {d} to {new_path}")
            # Use git mv if possible
            os.system(f'git mv "{d}" "{new_path}"')

# 3. Replace text in all files
for ext in ["*.md", "*.py", "*.json", "*.html", "*.yaml", "*.yml", "*.sh", "*.go"]:
    for file_path in ROOT.rglob(ext):
        # skip .git, node_modules, etc
        if ".git/" in str(file_path) or "node_modules" in str(file_path) or ".venv" in str(file_path):
            continue
        if file_path.is_file():
            try:
                content = file_path.read_text(encoding="utf-8")
                new_content = content
                
                # Fix YAML frontmatter quotes and colon: `name: arc-xxx` -> `name: arc-xxx`
                new_content = re.sub(r'name:\s*"arc:([^"]+)"', r'name: arc-\1', new_content)
                new_content = re.sub(r"name:\s*'arc:([^']+)'", r'name: arc-\1', new_content)
                new_content = re.sub(r'name:\s*arc:([a-zA-Z0-9-]+)', r'name: arc-\1', new_content)
                
                # Replace other arc: references to arc- references
                # Be careful not to replace `arc: ` if it's a normal sentence, but `arc-something` is very specific
                new_content = re.sub(r'arc:([a-z0-9]+)', r'arc-\1', new_content)
                
                if new_content != content:
                    file_path.write_text(new_content, encoding="utf-8")
                    print(f"Updated {file_path}")
            except Exception as e:
                print(f"Could not process {file_path}: {e}")

