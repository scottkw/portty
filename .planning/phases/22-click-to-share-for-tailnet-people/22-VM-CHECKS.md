# Phase 22 VM checks (plan 22-12): click-to-share on macOS 27.2

Rig: `.planning/phases/22-click-to-share-for-tailnet-people/rig/p22-rig.sh` (builds on `p20-rig.sh` and `p19-rig.sh`). Office = portty-test (the shared Mac: Portty GUI and daemon, TextEdit, Terminal, the probe route), guest = portty-test-2 (the person who opens the guest link: Safari through safaridriver). Every capture, input post, AX call, pasteboard access and launchctl call runs inside a VM through ssh; the host only compiles, copies and writes scrubbed evidence. Nothing runs Portty, capture, an input tap or injection on the host, and nothing contacts the host's tailscaled or a 100.x address. Neither VM is stopped.

## Criteria (written before any row ran)

A row is PASS only when every numbered condition holds. PARTIAL means a named condition could not be observed and the reason is recorded (the observed conditions held). FAIL means a condition was observed not to hold. NOT RUN means the row could not be driven at all, with the reason. UNVERIFIED is used only for rows that need a second Tailscale account.

- **G-01 tracer (DSHR-03, DSHR-04, DSHR-06).**
  1. The share is started from the Portty GUI on portty-test (AX presses, no daemon call), and the daemon then reports an active share with a link (link never printed).
  2. The guest link opened in portty-test-2's Safari shows text that contains `Waiting for` and `to let you in`, with no picture decoded (`framesDecoded` 0 or absent).
  3. The Admit sheet is in Portty's AX tree on portty-test: title `Someone Wants to Watch Your Desktop`, buttons `Deny` and `Admit`.
  4. After the Admit button is pressed through AX, the guest page reaches a moving picture (`framesDecoded` strictly rising across two reads at least 2 s apart) and its text contains `View only`.
  5. `Stop Sharing` pressed from the tray menu (AX) makes the guest page text contain `Sharing has ended` within 5 s, and the daemon reports no active share.
- **G-02 grant then click (DSHR-04, DCTL-04).**
  1. While view-only, a guest click and typed text change nothing in an empty TextEdit document and the input counters show them dropped (not posted).
  2. After `Allow control` is pressed on that guest's row in the Share Desktop sheet (AX), the guest page text contains `You have control`.
  3. A guest click on the TextEdit text area and typed text `Portty 22` makes TextEdit's document read exactly that, and the posted counters (`btn`, `key`) rise.
- **G-03 revoke mid-key (DSHR-05).**
  1. A controlling guest holds the A key for at least 4 s; the office OS key state (`keystate`) reads `A` held before the revoke.
  2. `Disconnect` pressed on the guest's row (AX) makes the office key state read `none` within 2 s, and the guest page text contains `You were disconnected`.
  3. The audit file holds a `revoked` event for that guest.
- **G-04 Stop with two guests (DSHR-05).**
  1. Two guest viewers (two Safari windows in portty-test-2) are admitted and both have a rising `framesDecoded`.
  2. `Stop Sharing` from the tray: the audit `share_stop` event and both guests' `leave` events fall within 2 s of one another, and both guest pages then show `Sharing has ended`.
  3. The old link answers the static link page (no viewer can join) after the stop.
- **G-05 physical wins (DCTL-04).**
  1. A controlling guest's typed text posts, and the `physical` counter does not move while only Portty's own (tagged) posts happen.
  2. One untagged software click (`~/ptest/click`, an honest stand-in for hardware, RESEARCH A.5) is followed by guest typing within 1 s: that typing is dropped, the `physical` counter rises, nothing is typed.
  3. The guest page text contains `Someone is using this Mac` during that time.
  4. After the lease passes (3 s idle) the guest's typing posts again.
- **G-06 Secure Event Input banner (DSHR-06, DCTL-04).**
  1. Terminal is frontmost with Secure Keyboard Entry switched on through its menu, a guest holds control: Portty's AX tree gains the banner text starting `Typing at this Mac isn't detected` within 5 s.
  2. A string typed by the guest still arrives in Terminal (remote keys keep posting).
  3. With Secure Keyboard Entry off the banner is gone within 5 s.
- **G-07 notification during capture (DSHR-06, UI backstop).**
  1. With a guest streaming and TextEdit frontmost (Portty hidden), a second knock is raised; the notification database and Notification Center's AX tree are read for the `wants to see your desktop` notification and the outcome is recorded as delivered or suppressed.
  2. Either way the Admit sheet is reachable (Portty's AX tree holds the sheet) and the first guest's picture keeps moving.
- **G-08 tray line and icon variant (DSHR-06).**
  1. With one guest connected, the tray menu (AX) lists `Desktop shared with 1 person` with `Stop Sharing` directly under it; with nobody connected it lists `Desktop shared with nobody yet`.
  2. The menu-bar icon differs from its not-sharing form (menu-bar crop of the VM compared pixel by pixel before and after); legibility in light and dark is looked at in the VM and the real-Mac item stays owner-run.
- **G-09 GUI quit ends the share (DSHR-05, DSHR-06).**
  1. With a guest watching, the Portty GUI process alone is terminated (SIGTERM). Within 3 s the audit file gains `share_stop` with reason `gui_gone` and the guest page text contains `Sharing has ended`.
  2. The daemon keeps running and reports no active share.
- **G-10 audit file (DSHR-07).**
  1. `desktop-audit.jsonl` exists with mode 0600.
  2. It holds, from the rows above, at least: `share_start`, `admit_request`, `admitted`, `join`, `control_granted`, `control_revoked` or `revoked`, `leave`, `share_stop` with reasons `share_stopped` and `gui_gone`.
  3. No line contains `cap=` or a token-shaped string (three base64url segments or 40 or more base64url characters).
- **G-11 window-share key focus, Phase 20 M-05 carried in (DCTL-05, DSHR-04).**
  1. TextEdit's single window is shared and a guest controls it; a non-activating panel (Spotlight) is opened locally by a software key event.
  2. Guest keys sent while the panel holds keyboard focus are dropped: the `window_key_focus` counter rises, nothing is typed into the shared window or the panel.
  3. After the panel is closed the guest's keys reach the shared window.
- **SC1 (DSHR-03 live two-account proof).** A second Tailscale user opens the guest link, waits at Admit, watches view-only after Admit and gets the own-devices refusal on a terminal link from the same device. Needs a second Tailscale account on the test tailnet. Not available: recorded UNVERIFIED, never PASS.
- **AR-21-01 (own-account peer filter).** Needs a node from another Tailscale account online. Recorded UNVERIFIED with SC1.
- **Appendix B (real-Mac physical input).** Recorded PARTIAL by reference to `evidence/appendix-b-real-mac.txt`; not re-run.
