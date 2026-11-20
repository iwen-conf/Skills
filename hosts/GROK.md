# Grok Build Host Adapter (Zero-Jump Checklist)

1. Run `arc-wave status` and `git status --short` first; resume active scope instead of restarting.
2. Edit only files inside `write_scope`; add zero unrequested packages or wrapper layers.
3. UI changes: strip slogans, helper subtitles, and stacked card borders/shadows—fix spacing hierarchy, `44pt/44px` hit targets, `tabular-nums`, `>= 4.5:1` Light/Dark contrast, and real `<button>`/`<a>`/`<input>` controls.
4. Run `bash ~/.grok/skills/arc-code/scripts/gate.sh check --task <slug>` before completion.
