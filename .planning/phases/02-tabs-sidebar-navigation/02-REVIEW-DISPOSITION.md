---
phase: 02
review: 02-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "Esc cancels a confirm while its action is in flight, bypassing the disabled Cancel button"
  - id: WR-02
    severity: warning
    disposition: open
    title: "Tray \"focus session\" does nothing for a detached session (regression from detach-only close)"
  - id: WR-03
    severity: warning
    disposition: open
    title: "The stored Appearance is never applied after \"Retry Connection\""
  - id: WR-04
    severity: warning
    disposition: open
    title: "Changing the Appearance quickly can leave the stored, native and on-screen values out of sync"
  - id: WR-05
    severity: warning
    disposition: open
    title: "`domReady` waits on `startupDone` with no bound, and the daemon client has no timeout"
  - id: WR-06
    severity: warning
    disposition: open
    title: "The session poll can bring back an exited or ended session, and clicking it opens a tab that never closes"
  - id: IN-01
    severity: info
    disposition: open
    title: "A failed rename reverts silently, and the poll can flicker the name"
  - id: IN-02
    severity: info
    disposition: open
    title: "Menu shortcuts act behind open dialogs"
  - id: IN-03
    severity: info
    disposition: open
    title: "Dead Hub-era code and stale comments left behind"
  - id: IN-04
    severity: info
    disposition: open
    title: "Chat history files and aliases.json are orphaned on disk for upgraded users"
  - id: IN-05
    severity: info
    disposition: open
    title: "`OpenTailscale` finds `open` through PATH"
  - id: IN-06
    severity: info
    disposition: open
    title: "Impossible-state guard: `@available(macOS 10.14, *)`"
  - id: IN-07
    severity: info
    disposition: open
    title: "Unused dependency and unhandled promise rejections"
  - id: IN-08
    severity: info
    disposition: open
    title: "Settings shows \"Automatic\" when the Appearance load fails, and Automatic cannot be re-saved"
  - id: IN-09
    severity: info
    disposition: open
    title: "The danger cue on the Full-access segment is visual only"
  - id: IN-10
    severity: info
    disposition: open
    title: "Inherited fresh-install plugin settings bug (already logged)"
---

# Phase 02 review disposition

One row per finding in 02-REVIEW.md, defaulting to `open`.

