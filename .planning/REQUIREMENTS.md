# Requirements: Portty

**Defined:** 2026-11-18
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## v1.1 Requirements

Hardening only: each requirement closes a v1.0 todo. No new features.

### Reliability

- [ ] **HARD-01**: A full-screen app (vim, less, a TUI) scrolling 20,000+ lines in the alternate screen keeps the daemon's heap under a fixed bound; main-screen history is not lost when the emulator is rebuilt. (todo: alt-screen-scrollback-growth)
- [ ] **HARD-02**: Pasting more than 32 KiB into a local tab, a peer tab, a browser view or `portty attach` delivers every byte to the shell, in order; the 32 KiB per-frame input limit stays as the DoS guard. (todo: split-large-pastes)
- [ ] **HARD-03**: A viewer joining a session whose history holds CSI 14/16/18 t, XTGETTCAP, DECRQSS, DA3, OSC 4 palette queries or OSC 52 clipboard reads answers none of them into the shared shell; the host hub and the daemon engine strip the same set. (todo: strip-more-terminal-queries-from-replay)
- [ ] **HARD-04**: Launching the GUI with a healthy daemon makes no respawn attempt and does not count toward the 3-attempt daemon-error streak. (todo: spurious-respawn-at-launch)
- [ ] **HARD-05**: Switching back to a remote tab rejoins with a delta replay from its last offset instead of a full replay, with no duplicated or missing output. (todo: tab-switch-full-replay)

### Test hygiene

- [ ] **HARD-06**: The daemon test suite never reads or writes the host's real Portty config dir (settings.json, socket); TestAPIAppearance_GetPatch and TestAPITerminalFont_GetPatch pass on a developer Mac with non-default settings, and the `-skip` is removed from TESTING.md and every gate. (todos: isolate-daemon-settings-tests, daemon-tests-read-host-settings)
- [ ] **HARD-07**: TestHub_TwoClientsFanOut passes reliably under the full `go test -race -short ./...` run (race found and fixed, or the test made deterministic). (todo: relay-fanout-flake)
- [ ] **HARD-08**: The multi-line prompt resize artifact (stray prompt top lines in scrollback when narrowing the window) is documented as a known limit with its cause. (todo: document-multi-line-prompt-resize-artifacts)

## Future Requirements

- Clicking the "update available" notification shows Portty (needs a notarized build to observe; after the first real release).
- Overnight two-Mac test (VERF-01, M-84) and M-85; first real release checks (M-93).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Any new user-facing feature | v1.1 is hardening only |
| Fixing the multi-line prompt resize artifact itself | Caused by zsh redrawing a wrapped prompt; documenting is the agreed scope |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| HARD-01 | Phase 7 | Pending |
| HARD-02 | Phase 7 | Pending |
| HARD-03 | Phase 7 | Pending |
| HARD-04 | Phase 7 | Pending |
| HARD-05 | Phase 7 | Pending |
| HARD-06 | Phase 7 | Pending |
| HARD-07 | Phase 7 | Pending |
| HARD-08 | Phase 7 | Pending |

**Coverage:**
- v1.1 requirements: 8 total
- Mapped to phases: 8
- Unmapped: 0 ✓

---
*Requirements defined: 2026-11-18*
*Last updated: 2026-11-18 after v1.1 roadmap creation (all 8 → Phase 7)*
