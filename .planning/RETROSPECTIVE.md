# Retrospective

## Milestone: v1.0 — Portty v1.0

**Shipped:** 2026-11-18
**Phases:** 6 | **Plans:** 61 | **Commits since fork:** ~560 (2026-11-14 → 2026-11-18)

### What Was Built
- AgentHub stripped to a plain macOS terminal (login zsh, no agent CLIs, file browser, Windows/Linux).
- Tabs + source-list sidebar, system light/dark, native macOS sheets and toolbar; chat removed.
- Full Portty rebrand with a CI brand gate; DMG-only releases.
- One Share switch: same-account tailnet peers plus a tailnet web link; `portty attach host:session`.
- Overnight survival: resumable replay, heartbeats, wake/network detection, reconnect loops in GUI, browser and CLI, daemon single-instance lock and auto-respawn.
- GitHub-release updater with digest, Team ID, signing-identifier and notarization checks; reveal in Finder.

### What Worked
- Two Tart VMs on the real tailnet as stand-ins for the two Macs: the resilience matrix (R1-R9) found R6 (no daemon respawn) that unit tests could not.
- Code review → fix → re-review loops caught real security issues (Phase 6 WR-01: same-team DMG / downgrade swap).
- Fake GitHub feed behind a build tag (vmpreview) let the whole update UX run in VMs with no real release.
- Chesterton's Fence with full fork history made deletions safe.

### What Was Inefficient
- Host load from parallel VMs and agents produced recurring test flakes (SharePopover Copy, initialGrid, LongHistoryJoiners) that cost reruns.
- Daemon tests reading the host's real settings.json/caches went unnoticed until Phase 4/6.
- Human checks needing two physical Macs piled up across Phases 2-5 and were all deferred.

### Patterns Established
- TESTING.md as the regression home: suite manifest, traceability paths gate, M-NN manual items.
- Worktree-isolated executors, fast-forward merge onto stacked phase branches.
- VM evidence files (05-VM-MATRIX.md, 06-VM-CHECKS.md) recorded per check with PASS/FAIL and evidence.

### Key Lessons
- Verify identity, not just signer: a shared Developer ID team needs an identifier check.
- Anything that must recover unattended (daemon, viewers) needs an explicit respawn path tested by killing it.
- Isolate HOME in tests that touch app paths from day one.

### Cost Observations
- Model mix: mostly Opus (orchestrator, executors, reviewers).
- Notable: VM checks were the slowest steps (60-110 min each) but the highest-value evidence.

## Cross-Milestone Trends

| Milestone | Phases | Plans | Days | Deferred human checks |
|-----------|--------|-------|------|-----------------------|
| v1.0 | 6 | 61 | 4 | ~15 (two-Mac + first release) |
