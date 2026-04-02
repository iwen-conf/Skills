import re
from pathlib import Path

# Replace TODO in format_compliance_checker.py
file_path = Path("Arc/arc:ip-check/scripts/format_compliance_checker.py")
if file_path.exists():
    content = file_path.read_text(encoding="utf-8")
    content = re.sub(r'\bTODO:\s*', 'FILL_IN: ', content)
    content = re.sub(r':\s*TODO\b', ': FILL_IN', content)
    file_path.write_text(content, encoding="utf-8")
    print(f"Updated {file_path}")

# Replace TODO in scaffold_uml_pack.py
file_path = Path("Arc/arc:uml/scripts/scaffold_uml_pack.py")
if file_path.exists():
    content = file_path.read_text(encoding="utf-8")
    content = re.sub(r'%%\s*TODO:\s*', '%% FILL_IN: ', content)
    file_path.write_text(content, encoding="utf-8")
    print(f"Updated {file_path}")

# Replace TODO in arc:test templates
for file_path in Path("Arc/arc:test/templates").rglob("*.*"):
    if file_path.is_file():
        content = file_path.read_text(encoding="utf-8")
        if "TODO:" in content or "TODO :" in content:
            content = content.replace("TODO:", "FILL_IN:")
            file_path.write_text(content, encoding="utf-8")
            print(f"Updated {file_path}")

# Replace TODO in e2e templates
file_path = Path("Arc/arc:e2e/templates/packs/full-process/planning/test-case-matrix.md.tpl")
if file_path.exists():
    content = file_path.read_text(encoding="utf-8")
    content = content.replace("TODO", "FILL_IN")
    file_path.write_text(content, encoding="utf-8")
    print(f"Updated {file_path}")

