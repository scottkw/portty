---
created: 2026-11-23
area: cli
source: Phase 15 M-153 VM run
---
`portty kill <name>` fails with `status 500: kill session: session not found` while `portty rename <name>` and `portty share <name>` accept the same name; killing by full session id works. Seen in the office VM on build v1.2.0-vm16. Make kill resolve names the way share/rename do, and add a test.

## Resolution

Resolved in Phase 16 plan 16-01 (commit e6ec4050). `cmdKill` and `cmdRename` now resolve through `lookupSession` (exact name, exact ID, unique ID prefix); a duplicate name is refused and nothing is killed. Rename had the same bug (the daemon looks the path value up as an ID only). Tests: `TestCmdKill_ByName`, `TestCmdRename_ByName`; live check TESTING.md M-156.
