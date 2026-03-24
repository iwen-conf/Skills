# **Output Schema (log specification)**


The output of this Skill is divided into two categories:

1) **Real-time log (stdout)**: used to watch while running.
2) **Placement report (artifacts)**: used for delivery, playback, and reproduction. **Must generate**.

The standardized Schema and templates are given below.

### **0. Run Report (Mandatory, report.md)**

`report.md` must contain the following chapters (the order is recommended to be fixed to facilitate diff and machine parsing):

```markdown