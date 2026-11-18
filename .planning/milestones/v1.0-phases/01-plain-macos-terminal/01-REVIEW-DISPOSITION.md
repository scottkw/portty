---
phase: 01
review: 01-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "The daemon auto-starts a LAN-mode web server whenever Tailscale is off at launch, so the Amendment 3 gate never applies on the default path"
  - id: WR-02
    severity: warning
    disposition: open
    title: "With Tailscale connected but no HTTPS certificates, the gated Start button still starts a LAN-only server"
  - id: WR-03
    severity: warning
    disposition: open
    title: "Already-defended-elsewhere: init/retryInit `KillSession` of stopped sessions duplicates the Go watcher (carried forward from iteration 4 WR-02, deliberately not fixed)"
  - id: IN-01
    severity: info
    disposition: open
    title: "Redundant `h.Connected &&` after the new guard"
  - id: IN-02
    severity: info
    disposition: open
    title: "The NET-01 sentence is a duplicated string literal in Go, kept in sync with the frontend only by a comment"
  - id: IN-03
    severity: info
    disposition: open
    title: "`TestStartWebServerLocalModeFallback` asserts nothing and depends on the host's Tailscale"
  - id: IN-04
    severity: info
    disposition: open
    title: "The Amendment 3 frontend tests do not cover \"Stop stays available\" or the pending-check state"
  - id: IN-05
    severity: info
    disposition: open
    title: "Stale comments that now contradict the code (carried forward)"
  - id: IN-06
    severity: info
    disposition: open
    title: "`QuitGUIOnly` counts active sessions by `Status != \"stopped\"`, a value `Status` never takes (carried forward)"
  - id: IN-07
    severity: info
    disposition: open
    title: "The Linux Playwright fixture has no exit detection (carried forward)"
  - id: IN-08
    severity: info
    disposition: open
    title: "The traceability parser drops the second of two adjacent path cells (carried forward)"
  - id: IN-09
    severity: info
    disposition: open
    title: "The `GetSessionQRCode` binding has no Go method (carried forward)"
  - id: IN-10
    severity: info
    disposition: open
    title: "`daemonErr` is written and read without synchronisation (carried forward, pre-existing)"
open: 13
total: 13
recorded: 2026-11-15T15:46:43.174Z
---

# Phase 01: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |
| IN-03 | info | open | - |
| IN-04 | info | open | - |
| IN-05 | info | open | - |
| IN-06 | info | open | - |
| IN-07 | info | open | - |
| IN-08 | info | open | - |
| IN-09 | info | open | - |
| IN-10 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
