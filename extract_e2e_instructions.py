import re
from pathlib import Path

def extract_section(file_path: Path, heading_start: str, ref_file_name: str, link_text: str):
    content = file_path.read_text(encoding="utf-8")
    
    lines = content.split('\n')
    start_idx = -1
    for i, line in enumerate(lines):
        if line.startswith(heading_start):
            start_idx = i
            break
            
    if start_idx == -1:
        print(f"Section {heading_start} not found in {file_path}")
        return
        
    end_idx = len(lines)
    for i in range(start_idx + 1, len(lines)):
        if lines[i].startswith("## ") and not lines[i].startswith("### "):
            # Ignore false positives inside code blocks
            in_code_block = False
            for j in range(start_idx, i):
                if lines[j].startswith("```"):
                    in_code_block = not in_code_block
            if not in_code_block:
                end_idx = i
                break
            
    section_lines = lines[start_idx:end_idx]
    section_content = '\n'.join(section_lines)
    
    ref_dir = file_path.parent / "references"
    ref_dir.mkdir(exist_ok=True)
    ref_path = ref_dir / ref_file_name
    
    if not section_lines[0].startswith("# "):
        ref_content = f"# {section_lines[0].strip('# ')}\n\n" + '\n'.join(section_lines[1:])
    else:
        ref_content = section_content
        
    ref_path.write_text(ref_content, encoding="utf-8")
    
    replacement = f"{lines[start_idx]}\n\nThis section has been moved to a reference file to reduce context bloat.\n\n👉 **Please see [{link_text}](references/{ref_file_name}) for the full details.**\n"
    
    new_content = '\n'.join(lines[:start_idx]) + '\n' + replacement + '\n' + '\n'.join(lines[end_idx:])
    
    file_path.write_text(new_content, encoding="utf-8")
    print(f"Extracted {heading_start} from {file_path} to {ref_path}")

extract_section(Path("Arc/arc-e2e/SKILL.md"), "## **Instructions (execution process)**", "execution-instructions.md", "Detailed Execution Instructions")
