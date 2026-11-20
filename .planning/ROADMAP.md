# Roadmap: Portty

## Milestones

- ✅ **v1.0 Portty v1.0** — Phases 1-6 (shipped 2026-11-18)
- 🚧 **v1.1 Hardening** — Phases 7-8 (in progress)

## Phases

<details>
<summary>✅ v1.0 Portty v1.0 (Phases 1-6) — SHIPPED 2026-11-18</summary>

- [x] Phase 1: Plain macOS Terminal (12/12 plans) — completed 2026-11-15
- [x] Phase 2: Tabs + Sidebar Navigation (13/13 plans) — completed 2026-11-16 (two-Mac UAT deferred)
- [x] Phase 3: Portty Brand (6/6 plans) — completed 2026-11-16 (M-70/M-73/M-74 deferred)
- [x] Phase 4: One-Click Share (10/10 plans) — completed 2026-11-17 (live two-Mac repro deferred)
- [x] Phase 5: Overnight Survival (16/16 plans) — completed 2026-11-18 (M-84/M-85, VERF-01 deferred)
- [x] Phase 6: Release Updates (4/4 plans) — completed 2026-11-18 (M-93 first real release deferred)

Full details: `.planning/milestones/v1.0-ROADMAP.md`. Audit: `.planning/milestones/v1.0-MILESTONE-AUDIT.md`.

</details>

### 🚧 v1.1 Hardening

**Milestone Goal:** Fix the reliability and test-hygiene debt left from v1.0, with no new features. Each requirement closes a v1.0 todo.

- [x] **Phase 7: Hardening** - Bounded alt-screen memory, intact large pastes, a query-free catch-up replay, no spurious respawn, delta rejoin on tab switch, and a full test suite that passes with nothing skipped (completed 2026-11-18)
- [x] **Phase 8: Debt Cleanup** - Close the v1.1 audit's tech debt: a small alt-screen heap bound, wrapped lines kept in replay, full replay and rebuild fidelity, a bounded and leak-free peer snapshot store, honest partial pastes, the launch-blip root cause, and Phase 7 bookkeeping (completed 2026-11-19)

## Phase Details

### Phase 7: Hardening
**Goal**: The v1.0 reliability and test-hygiene debt is closed. The daemon's memory stays bounded under full-screen apps. The shared shell receives exactly the bytes users type or paste, and nothing a joining terminal answers. Launch and tab switches do no wasted recovery work. The full test suite passes on a developer Mac with nothing skipped.
**Depends on**: Phase 6
**Requirements**: HARD-01, HARD-02, HARD-03, HARD-04, HARD-05, HARD-06, HARD-07, HARD-08
**Success Criteria** (what must be TRUE):
  1. A full-screen app (vim, less, a TUI) that scrolls 20,000+ lines in the alternate screen keeps the daemon heap under a fixed bound, proven by a heap soak test like `TestHistory_HeapBoundAfterBurst`. A viewer who joins afterwards still gets the main-screen history from before the app started.
  2. The shared shell receives exactly what users send. A paste over 32 KiB into a local tab, a peer tab, a browser view or `portty attach` arrives complete and in order, and a single input frame over 32 KiB is still dropped. A viewer joining a session whose history holds CSI 14/16/18 t, XTGETTCAP, DECRQSS, DA3, OSC 4 palette queries or OSC 52 clipboard reads types nothing into the shell. The host hub and the daemon engine strip the same set, with one test per sequence.
  3. Recovery paths do no wasted work. Launching the GUI with a healthy daemon logs no respawn attempt and leaves the 3-attempt daemon-error streak at zero. Switching back to a remote tab rejoins with a delta replay from its last offset: no full redraw, no duplicated or missing output.
  4. On a developer Mac with non-default Portty settings, `go test -race -short ./...` and Vitest pass with no `-skip` in TESTING.md or any gate. The daemon tests never read or write the real Portty config dir (settings.json, socket). TestHub_TwoClientsFanOut passes in repeated full `-race` runs because its race is fixed or the test is deterministic.
  5. A user who sees stray prompt lines after narrowing the window finds a Known issues entry in the user docs. It gives the cause (zsh redrawing a wrapped multi-line prompt on each resize) and the remedies: `clear` or Ctrl-L, or a single-line prompt via `p10k configure`.
**Plans**: 5/5 plans complete
Plans:
- [x] 07-01-PLAN.md — Relay replay hardening: shared query strip (HARD-03), alt-screen exit rebuild with heap soak (HARD-01), deterministic fan-out test (HARD-07)
- [x] 07-02-PLAN.md — Peer tab delta rejoin: snapshot + offset across unmount, no hidden sockets (HARD-05)
- [x] 07-03-PLAN.md — Launch respawn gate (HARD-04), daemon test isolation with no -skip (HARD-06), README Known Issues (HARD-08)
- [x] 07-04-PLAN.md — Large pastes split into frames under the 32 KiB guard: frontend, CLI, server guard test (HARD-02)
- [x] 07-05-PLAN.md — TESTING.md reconciliation, full gate with nothing skipped, Tart VM spot check (all HARD)
**UI hint**: no

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 7. Hardening | v1.1 | 5/5 | Complete    | 2026-11-18 |

### Phase 8: Debt Cleanup
**Goal**: The tech debt recorded in the v1.1 audit is closed, so v1.1 ships with no known deferred items in its own code.
**Depends on**: Phase 7
**Requirements**: DEBT-01, DEBT-02, DEBT-03, DEBT-04, DEBT-05, DEBT-06, DEBT-07, DEBT-08
**Success Criteria** (what must be TRUE):
  1. A full-screen app scrolling 20,000+ full-width lines in the alternate screen keeps each session's daemon heap growth under a small fixed bound that does not grow with width, proven by a soak test at 80 and 220 columns and a VM check. The alt screen and the main-screen history stay correct for viewers who join.
  2. Replays are faithful. A soft-wrapped long line replays as one wrapped line. Rendered replays and the alt-exit rebuild restore modes before text, keep SO/G1 line drawing and an open OSC 8 link, and the rebuild fuzz test stays at 0 mismatches.
  3. The peer-tab snapshot store is bounded by bytes, frees a closed tab's entry, is keyed by host plus session, never leaks a terminal, and a quick switch back still gets a delta rejoin.
  4. A paste cut short by a dropped socket tells the user so. No frame is sent twice, and none is silently lost.
  5. The HARD-04 launch blip has a root-cause fix, or a documented VM non-reproduction with diagnostics in place. The Phase 7 SUMMARYs carry `requirements_completed`, and Phase 7 has a SECURITY.md.
  6. Upgrading Portty never silently ends live sessions. With sessions running, a version-mismatch restart needs consent: a GUI dialog, or the CLI's `--restart-daemon`. Declining keeps the old daemon and its sessions working. Proven by tests and a VM upgrade run.
**Plans**: 6/6 plans complete
Plans:
- [x] 08-01-PLAN.md — Alt screen keeps no scrollback, with soak at 80/220 columns (DEBT-01); soft wraps recorded as vt makes them and replayed without CRLF (DEBT-02)
- [x] 08-02-PLAN.md — Mode restores before the text and the history, ?7 after (IN-06); rebuild keeps SO/G1 and OSC 8 links (IN-08); extended fuzz at 0 mismatches (DEBT-03)
- [x] 08-03-PLAN.md — Peer snapshot store keyed by host plus session, 8 MiB budget, dropped on tab close, flush timeout, quick-switch wait (DEBT-04); partial-paste notice with no resend (DEBT-05)
- [x] 08-04-PLAN.md — Five cold launches after VM reboots, root-cause fix or documented non-reproduction (DEBT-06); 07 SUMMARY requirements-completed and 07-SECURITY.md (DEBT-07)
- [x] 08-05-PLAN.md — TESTING.md reconciliation, full gate once with nothing skipped, Tart VM checks V-01..V-06 in 08-VM-CHECKS.md (all DEBT)
- [x] 08-06-PLAN.md — Upgrade guard: consent before a version-mismatch restart ends live sessions (DEBT-08)
**UI hint**: no
