---
phase: quick-261116-c9f
plan: 01
subsystem: frontend/sidebar
tags: [sidebar, ui, macos, nav]
status: complete
requires: []
provides:
  - "Sidebar onNewSession prop (header '+' and empty-group New Session row)"
  - "Other Macs host rows with Reachable/Not reachable status dot and nested sessions"
  - "peerView keeps reachable peers with zero sessions"
affects:
  - frontend/src/App.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/style.css
  - frontend/src/lib/sidebarPeers.ts
tech-stack:
  added: []
  patterns:
    - "Hover-reveal header action via opacity (stays in tab order, focus-within reveals)"
    - "Status dot encodes state by shape (filled vs hollow ring), with a text line carrying it too"
key-files:
  created: []
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/style.css
    - frontend/src/lib/sidebarPeers.ts
    - frontend/src/components/__tests__/Sidebar.test.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/lib/__tests__/sidebarPeers.test.ts
    - frontend/src/components/__tests__/App.nav.test.tsx
    - frontend/src/components/__tests__/style.contrast.test.ts
    - TESTING.md
decisions:
  - "Header '+' tooltip is 'New Session (⌘T)' to match the toolbar '+'; 'New Session ⌘T' is the visible text of the empty-group row"
  - "Nested peer session rows use CommandLineIcon; the host row carries ComputerDesktopIcon"
  - "Supersedes four 02-UI-SPEC Sidebar rules at the user's request (Hover: None, per-peer headers, reachable-empty Hidden, 'No sessions' copy); 02-UI-SPEC.md left untouched"
metrics:
  duration: "~5 min"
  completed: 2026-11-16
actuals:
  tokens: 10900
  tasks: 3
  commits: 3
plan_head_before: 13199140816858644cdc72d011c07456ffb76954
plan_head_after: b501b8f8875e803ba5a07553a10fc90ab125ae25
---

# Quick 261116-c9f Plan 01: Sidebar "native, alive" refresh Summary

The sidebar now has a This Mac "+" and an accent "New Session ⌘T" row when the group is empty, both wired to the existing New Session dialog. It also has Heroicons glyphs on the group headers, a hover fill on rows, and one Other Macs section of host rows. Each host row has a filled or hollow reachability dot, its sessions are nested beneath it, and an idle peer reads "Nothing shared". The footer is a compact 28px line with the full text in its tooltip.

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 (tracer) | New Session path (App prop, header "+", empty row, CSS), header glyphs, row hover (R1, R2, R3, R6) | a607785e |
| 2 | Other Macs host rows with status dot and nested sessions, idle peers kept, compact footer (R4, R5) | 45574955 |
| 3 | App.nav wiring gate, accent contrast proof, TESTING.md (note, NAV-01/02 rows, LOOK-01 contrast row, M-69) | b501b8f8 |

## Verification

- RED before GREEN for Tasks 1 and 2: 5 and 10 tests failed first, then passed after the implementation.
- Tracer gate: Task 1's `<verify>` was re-run end-to-end and passed, including both grep gates.
- Final gate: `pnpm exec tsc --noEmit` is clean. Full vitest run: 91 files, 1173 tests, all passing (baseline was 1162). `bash tests/check-traceability-paths.sh` reports 93 paths OK. The M-69, Quick 261116-c9f and contrast-row greps all match.
- Test counts from `git ls-files` are unchanged: Go 108, vitest 91, Playwright 7.
- No other consumer of the old markup exists (`sidebar-group-peer-*` ids, "No sessions", `.sidebar__dot`).
- Not done here: a visual check of hover, reveal, layout and vibrancy. The orchestrator runs it in the Tart VM, per TESTING.md M-69.

## Deviations from Plan

None. The plan was executed as written.

## Threat Flags

None. Peer hostnames and session names are rendered only as React text and attribute values. The ids come from the loop index (T-c9f-01).

## Self-Check: PASSED

- All 10 modified files exist.
- Commits a607785e, 45574955 and b501b8f8 are ancestors of HEAD on gsd/phase-03-portty-brand.
