---
created: 2026-11-23
area: cli
source: Phase 15 M-153 VM run
---
`Portty --version` is not a known subcommand and appears to start a GUI instance, which hung over SSH (`Portty version` works). Treat `--version`/`-v` as `version` (or reject unknown flags with usage) so a CLI-only call never launches the GUI.
