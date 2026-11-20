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

## Milestone: v1.1 — Hardening

**Shipped:** 2026-11-19
**Phases:** 2 (7 Hardening, 8 Debt Cleanup) | **Plans:** 11

### What Was Built
- Bounded alt-screen memory, intact large pastes everywhere, faithful query-free replays, delta rejoin on tab switch.
- The launch blip found and fixed; fast daemon failure; safe upgrades (no missing daemon, consent before ending sessions).
- Hermetic, deterministic tests: HOME isolation, a tailnet guard, race fixes, nothing skipped.

### What Worked
- Code review with probe overlays found real bugs every round: a session freeze from binary output, OSC 8 injection into viewers, an upgrade leaving no daemon.
- VM checks after every fix round caught what unit tests can't: a no-argument launch crash, the CPU pinned by a paste, the root cause of the login-time blip.
- The user's firewall alerts exposed tests probing the real tailnet.

### What Was Inefficient
- Fix rounds introduced regressions twice (wrap keying by line number; the lock-wait readiness check). Each needed another review round.
- Worktrees always started from an old commit, and every executor needed a fast-forward step at startup.
- A planning commit made on the phase branch while agents ran forced cherry-picks instead of fast-forwards.

### Key Lessons
- Never let a test binary reach the network: guard the choke points, not the call sites.
- Anything touching `main()`'s argument routing needs a real launch check; unit tests don't run main.
- "Ready" must mean the right version, not just answering.
- An upgrade path is part of reliability: test it with live sessions.
