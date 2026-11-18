---
phase: 02
review: 02-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: fixed
    title: "Esc cancels a confirm while its action is in flight"
  - id: WR-02
    severity: warning
    disposition: fixed
    title: "Tray focus does nothing for a detached session"
  - id: WR-03
    severity: warning
    disposition: fixed
    title: "Stored Appearance not applied after Retry Connection"
  - id: WR-04
    severity: warning
    disposition: fixed
    title: "Fast Appearance changes desync stored/native/on-screen"
  - id: WR-05
    severity: warning
    disposition: fixed
    title: "domReady waits unbounded; daemon client no timeout"
  - id: WR-06
    severity: warning
    disposition: fixed
    title: "Session poll resurrects exited sessions"
  - id: WR-07
    severity: warning
    disposition: fixed
    title: "Startup wait timeout leaves slow-start app broken"
  - id: WR-08
    severity: warning
    disposition: fixed
    title: "Quit blocked during slow startup by awaitStartup gates"
  - id: IN-01
    severity: info
    disposition: deferred
    title: "Failed rename reverts silently"
    note: "minor UX; revisit with Phase 4 share redesign"
  - id: IN-02
    severity: info
    disposition: deferred
    title: "Menu shortcuts act behind open dialogs"
    note: "menu shortcuts behind dialogs; low impact"
  - id: IN-03
    severity: info
    disposition: deferred
    title: "Dead Hub-era code and stale comments"
    note: "dead tail routes kept per plan as later cut candidates (Chesterton)"
  - id: IN-04
    severity: info
    disposition: deferred
    title: "Chat history/aliases orphaned on disk"
    note: "deleting users' chat files on disk is data removal; needs user decision"
  - id: IN-05
    severity: info
    disposition: fixed
    title: "OpenTailscale finds open via PATH"
  - id: IN-06
    severity: info
    disposition: deferred
    title: "Impossible-state @available guard"
    note: "necessity: harmless @available guard"
  - id: IN-07
    severity: info
    disposition: fixed
    title: "Unused dep and unhandled promise rejections"
  - id: IN-08
    severity: info
    disposition: deferred
    title: "Settings shows Automatic when appearance load fails"
    note: "edge case when appearance load fails"
  - id: IN-09
    severity: info
    disposition: fixed
    title: "Full-access danger cue visual only"
  - id: IN-10
    severity: info
    disposition: fixed
    title: "Inherited fresh-install plugin defaults bug"
  - id: IN-11
    severity: info
    disposition: fixed
    title: "IN-07 error messages never clear"
  - id: IN-12
    severity: info
    disposition: deferred
    title: "Race fixes tested by source matching only"
    note: "race fixes covered by source tests; native checks done in VM"
  - id: IN-13
    severity: info
    disposition: fixed
    title: "Misleading comments in app.go"
  - id: IN-14
    severity: info
    disposition: deferred
    title: "Start-minimized ignored when startup >5s"
    note: "start-minimized ignored on >5s startup; minor"
  - id: IN-15
    severity: info
    disposition: fixed
    title: "90s bound comment inaccurate"
---

# Phase 02 review disposition

Three review iterations plus a targeted WR-08 fix (b9041abc). All warnings fixed; deferred info items carry a reason.
Fixes VM-verified: WR-02 (tray opens tab), WR-06 (ended session stays gone), IN-10 (fresh-install plugin defaults), normal launch after WR-05/07 startup rework.

