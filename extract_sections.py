import re
from pathlib import Path

def extract_section(file_path: Path, heading_start: str, ref_file_name: str, link_text: str):
    content = file_path.read_text(encoding="utf-8")
    
    # Find the start of the section
    lines = content.split('\n')
    start_idx = -1
    for i, line in enumerate(lines):
        if line.startswith(heading_start):
            start_idx = i
            break
            
    if start_idx == -1:
        print(f"Section {heading_start} not found in {file_path}")
        return
        
    # Find the end of the section (next heading of same or higher level)
    # The heading starts with "## ", so we look for "^## "
    end_idx = len(lines)
    for i in range(start_idx + 1, len(lines)):
        if lines[i].startswith("## ") or lines[i].startswith("# "):
            end_idx = i
            break
            
    # Extract the block
    section_lines = lines[start_idx:end_idx]
    section_content = '\n'.join(section_lines)
    
    # Determine the reference path
    ref_dir = file_path.parent / "references"
    ref_dir.mkdir(exist_ok=True)
    ref_path = ref_dir / ref_file_name
    
    # Add a title to the reference file if it doesn't have one
    if not section_lines[0].startswith("# "):
        ref_content = f"# {section_lines[0].strip('# ')}\n\n" + '\n'.join(section_lines[1:])
    else:
        ref_content = section_content
        
    ref_path.write_text(ref_content, encoding="utf-8")
    
    # Replace the section with a link
    replacement = f"{lines[start_idx]}\n\nThis section has been moved to a reference file to reduce context bloat.\n\n👉 **Please see [{link_text}](references/{ref_file_name}) for the full details.**\n"
    
    new_content = '\n'.join(lines[:start_idx]) + '\n' + replacement + '\n' + '\n'.join(lines[end_idx:])
    
    file_path.write_text(new_content, encoding="utf-8")
    print(f"Extracted {heading_start} from {file_path} to {ref_path}")

# arc:e2e
extract_section(Path("Arc/arc:e2e/SKILL.md"), "## **Output Schema", "output-schema.md", "Output Schema and Log Specifications")

# arc:ip-check
extract_section(Path("Arc/arc:ip-check/SKILL.md"), "## Instructions", "execution-instructions.md", "Detailed Execution Instructions")

# arc:ip-draft
extract_section(Path("Arc/arc:ip-draft/SKILL.md"), "## Instructions", "execution-instructions.md", "Detailed Execution Instructions")

