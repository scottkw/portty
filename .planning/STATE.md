---
gsd_state_version: '1.0'
status: planning
progress:
  total_phases: 5
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-11-14)

**Core value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.
**Current focus:** Phase 1 - Plain macOS Terminal

## Current Position

Phase: 1 of 5 (Plain macOS Terminal)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-11-14 — Roadmap created (5 phases, 25/25 v1 requirements mapped)

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: Strip-down before rebrand, so the rename (about 179 files reference "agenthub" today) touches the smallest tree
- [Roadmap]: Navigation before rebrand, so help and in-app text are rewritten once against the final UI
- [Roadmap]: NAV-02 (peer sessions in sidebar) ships with Hub removal so GUI remote attach never regresses (it currently lives in Hub remote cards)
- [Roadmap]: Overnight survival is its own phase so the overnight-length test doesn't hold up same-day share verification

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: Chat panel, Share modal, Funnel risk panel, and web-share session view live under `frontend/src/components/Hub/`. Hub removal must move them, not delete them. Remote-peer polling in `App.tsx` only runs while the Hub tab is active.
- [Phase 3]: Peer discovery probes port 7443 for AgentHub's "capability required" marker. If AgentHub still runs on either Mac, Portty may treat it as a peer or collide on the port. Decide on the port and marker during the rebrand. Both Macs need the rebranded build at the same time.
- [Phase 3]: NOTF-01 (v1: update check points at Portty's feed) overlaps DIST-02 (v2: auto-update pulls from Portty's feed). The v1 check must cope with no Portty release existing yet.
- [Phase 4]: Remote attach today uses a join-code/capability handshake (`RemoteJoinCodeModal`). SHAR-01's "one action, no separate toggles" must define how peers get capabilities without weakening the capability model.
- [Phase 5]: The office Mac must not sleep overnight. Keep-awake (CONV-01) is v2, so the overnight test needs manual `caffeinate`/Energy settings. The home Mac sleeping overnight needs reconnect-on-wake.

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| *(none)* | | | | |

## Session Continuity

Last session: 2026-11-14
Stopped at: Roadmap and STATE created; awaiting roadmap approval
Resume file: None
