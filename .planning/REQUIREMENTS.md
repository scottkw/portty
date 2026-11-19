# Requirements: Portty

**Defined:** 2026-11-18
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## v1.1 Requirements

Hardening only: each requirement closes a v1.0 todo. No new features.

### Reliability

- [x] **HARD-01**: A full-screen app (vim, less, a TUI) scrolling 20,000+ lines in the alternate screen keeps the daemon's heap under a fixed bound; main-screen history is not lost when the emulator is rebuilt. (todo: alt-screen-scrollback-growth)
- [x] **HARD-02**: Pasting more than 32 KiB into a local tab, a peer tab, a browser view or `portty attach` delivers every byte to the shell, in order; the 32 KiB per-frame input limit stays as the DoS guard. (todo: split-large-pastes)
- [x] **HARD-03**: A viewer joining a session whose history holds CSI 14/16/18 t, XTGETTCAP, DECRQSS, DA3, OSC 4 palette queries or OSC 52 clipboard reads answers none of them into the shared shell; the host hub and the daemon engine strip the same set. (todo: strip-more-terminal-queries-from-replay)
- [x] **HARD-04**: Launching the GUI with a healthy daemon makes no respawn attempt and does not count toward the 3-attempt daemon-error streak. (todo: spurious-respawn-at-launch)
- [x] **HARD-05**: Switching back to a remote tab rejoins with a delta replay from its last offset instead of a full replay, with no duplicated or missing output. (todo: tab-switch-full-replay)

### Test hygiene

- [x] **HARD-06**: The daemon test suite never reads or writes the host's real Portty config dir (settings.json, socket); TestAPIAppearance_GetPatch and TestAPITerminalFont_GetPatch pass on a developer Mac with non-default settings, and the `-skip` is removed from TESTING.md and every gate. (todos: isolate-daemon-settings-tests, daemon-tests-read-host-settings)
- [x] **HARD-07**: TestHub_TwoClientsFanOut passes reliably under the full `go test -race -short ./...` run (race found and fixed, or the test made deterministic). (todo: relay-fanout-flake)
- [x] **HARD-08**: The multi-line prompt resize artifact (stray prompt top lines in scrollback when narrowing the window) is documented as a known limit with its cause. (todo: document-multi-line-prompt-resize-artifacts)

### Debt cleanup (from the v1.1 audit)

- [ ] **DEBT-01**: While a full-screen app stays in the alternate screen, the daemon heap per session stays under a small fixed bound that does not scale with 10,000 lines × width (today ~128 MiB at 100 columns, ~270 MiB at 220). The alt screen and the main-screen history stay intact for viewers.
- [ ] **DEBT-02**: A soft-wrapped long line comes back as one wrapped line, not two hard lines, in history replay and after a rebuild.
- [ ] **DEBT-03**: Rendered replays and the alt-exit rebuild restore DEC modes before the screen text (IN-06), keep a G1 line-drawing charset shifted in with SO, and keep an open OSC 8 hyperlink (IN-08).
- [ ] **DEBT-04**: The peer-tab snapshot store is bounded by bytes, drops a tab's entry when the tab closes, is keyed by host plus session, never leaks an xterm if the flush callback doesn't fire, and a quick switch back to the tab still restores the snapshot rather than falling back to a full replay (IN-02, IN-03, W1, W2).
- [ ] **DEBT-05**: If the socket drops partway through a paste that was split into frames, the user is told the paste was cut short. Unsent frames are never silently lost, and never sent twice (IN-05).
- [ ] **DEBT-06**: The cause of the HARD-04 launch blip is found and fixed at its root (the grace stays as a safety net), or a cold launch in the VM shows the blip can't be reproduced, and the log diagnostics that would catch it on the real Mac are recorded.
- [ ] **DEBT-07**: The Phase 7 bookkeeping is complete: `requirements_completed` in the 07 SUMMARY frontmatter, and a SECURITY.md from `/gsd-secure-phase 7`.

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
| HARD-01 | Phase 7 | Complete |
| HARD-02 | Phase 7 | Complete |
| HARD-03 | Phase 7 | Complete |
| HARD-04 | Phase 7 | Complete |
| HARD-05 | Phase 7 | Complete |
| HARD-06 | Phase 7 | Complete |
| HARD-07 | Phase 7 | Complete |
| HARD-08 | Phase 7 | Complete |
| DEBT-01 | Phase 8 | Pending |
| DEBT-02 | Phase 8 | Pending |
| DEBT-03 | Phase 8 | Pending |
| DEBT-04 | Phase 8 | Pending |
| DEBT-05 | Phase 8 | Pending |
| DEBT-06 | Phase 8 | Pending |
| DEBT-07 | Phase 8 | Pending |

**Coverage:**
- v1.1 requirements: 15 total
- Mapped to phases: 15
- Unmapped: 0 ✓

---
*Requirements defined: 2026-11-18*
*Last updated: 2026-11-18 after v1.1 roadmap creation (all 8 → Phase 7)*
