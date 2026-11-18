# Phase 5 VM Matrix

Shared evidence file for the 05-09, 05-12, 05-13 and 05-14 gap plans. Each row is one check that Claude ran in the Tart VMs. Result is exactly `PASS` or `FAIL`. No share links or cap values go in this file.

- **Date:** 2026-11-18 (VM clock, UTC)
- **Build:** real app (not vmpreview), `wails build -platform darwin/arm64 -tags wailsassets`, commit `d7c6382b`, deployed to both VMs
- **VM roles:**
  - `portty-test`: office role, tailnet name `portty-vm-office` (100.91.251.68)
  - `portty-test-2`: home role, tailnet name `portty-vm-home` (100.112.33.35)
  - Both run the system tailscaled on the same tailnet and list each other in `tailscale status`.
- **How it was observed:** in both VMs, WebKit cannot paint into `screencapture` right now. Portty's window and a plain Safari page both capture blank, and the log shows `IOSurface creation failed` from WebContent. This is a VM graphics limit and a VM reboot did not fix it. So GUI state was read from the macOS Accessibility tree of the running app with `~/ptest/axdump app.portty.Portty`, which is the live DOM as WebKit exposes it (roles, DOM ids, classes, titles, frames). Clicks and keys used the click helper (real CGEvents).

## Feature checks

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| V-09 | PASS | double-click opens one tab; Cmd+W keeps sidebar and tab bar; reopen works | Home VM, office session `vmoffice` shared from the office VM. Double-click on its sidebar row (`click dclick`, 80 ms apart, while the connect is still in flight): the AX tree shows exactly one tab `portty-vm-office › vmoffice` and one `tabpanel-__websession__<id>` with a live xterm. The office daemon log shows one web viewer join for it. Cmd+W (`click key 13 cmd`): the sidebar nav (This Mac, Other Macs, the `vmoffice` row), the `Open sessions` tab list and the `No Open Sessions` empty state are all present, and the app pid did not change. One click on the row reopens one tab. Close plus double-click again: one tab again. No `[ErrorBoundary]` log lines, and no fallback copy shown |
