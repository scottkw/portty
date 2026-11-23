---
created: 2026-11-23
area: cli
source: Phase 15 M-153 VM run
---
`portty kill <name>` fails with `status 500: kill session: session not found` while `portty rename <name>` and `portty share <name>` accept the same name; killing by full session id works. Seen in the office VM on build v1.2.0-vm16. Make kill resolve names the way share/rename do, and add a test.
