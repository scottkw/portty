# Roadmap: Portty

## Milestones

- ✅ **v1.0 Portty v1.0** — Phases 1-6 (shipped 2026-11-18)
- 🚧 **v1.1 Hardening** — Phase 7 (in progress)

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

- [ ] **Phase 7: Hardening** - Bounded alt-screen memory, intact large pastes, a query-free catch-up replay, no spurious respawn, delta rejoin on tab switch, and a full test suite that passes with nothing skipped

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
**Plans**: 5 plans
Plans:
- [ ] 07-01-PLAN.md — Relay replay hardening: shared query strip (HARD-03), alt-screen exit rebuild with heap soak (HARD-01), deterministic fan-out test (HARD-07)
- [ ] 07-02-PLAN.md — Peer tab delta rejoin: snapshot + offset across unmount, no hidden sockets (HARD-05)
- [ ] 07-03-PLAN.md — Launch respawn gate (HARD-04), daemon test isolation with no -skip (HARD-06), README Known Issues (HARD-08)
- [ ] 07-04-PLAN.md — Large pastes split into frames under the 32 KiB guard: frontend, CLI, server guard test (HARD-02)
- [ ] 07-05-PLAN.md — TESTING.md reconciliation, full gate with nothing skipped, Tart VM spot check (all HARD)
**UI hint**: no

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 7. Hardening | v1.1 | 0/0 | Not started | - |
