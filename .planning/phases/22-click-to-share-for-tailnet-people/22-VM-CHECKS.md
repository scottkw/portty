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

## User checkpoint

Orchestrator decision on the user's behalf (2026-11-27). The user was not asked: the orchestrator decided these answers when it dispatched this plan, and this section records them as it gave them.

What the rig read before the decision applied: the Phase 22 build (V, 22.12.0) is installed on portty-test and its daemon holds both permissions (`{"sr":true,"ax":true}`, re-read after G-01); portty-test-2 runs the Phase 21 build and needs no Phase 22 build because it only opens the guest link in Safari; the G-01 result is PASS (see Results). Both VMs answered over ssh without being started or stopped.

1. **VM permissions.** Handled inside the VMs; nothing needed granting (Accessibility and Screen Recording already held on portty-test). Answer recorded: "permissions ok, handled by the executor inside the VMs (admin account), as earlier phases did".
2. **SC1.** Answer: "skip SC1". SC1 is recorded UNVERIFIED with the reason "no second Tailscale user", never PASS, and becomes an owner-run M-NN row in 22-13. AR-21-01 (own-account peer filter) needs the same second account and is recorded UNVERIFIED with it.
3. **Real-Mac items.** Answer: "defer" for all five; each becomes an owner-run M-NN row in 22-13:

| Real-Mac item | Answer |
|---------------|--------|
| Tray icon variant beside the other tray icons, in a light and a dark menu bar | defer |
| The Admit sheet arriving while typing in a terminal (no stray answer, the guard feels right) | defer |
| A right-to-left or look-alike display name on the real Admit sheet | defer |
| Someone is using this Mac appearing for a controlling guest while the owner types at the Mac | defer |
| Warp check (a remote pointer warp must not eat local input) and the Secure Keyboard Entry check (banner shows on the Mac while remote keys still post) | defer |

## Build under test

| Build | Version | Tags | Used for |
|-------|---------|------|----------|
| V | 22.12.0 | `wailsassets vmdesktop` | every row (HEAD `dfa1a208` of this phase branch, Developer ID, hardened runtime, identifier `app.portty.Portty`; carries the VM-only `POST /desktop/debug/vm20` probe route) |

Built with `p22-rig.sh build22` (a scratch `git archive` of HEAD outside the repo; compile only on the host), installed in the office with the Phase 19 `deploy` and started with `start-gui`. The office daemon held Screen Recording and Accessibility (`GET /desktop/permissions` answered `{"sr":true,"ax":true}`) and the Certificate Transparency notice was already acknowledged, so the Share Desktop sheet starts a share by itself on open. Both VMs are on the owner's real tailnet; the rows only ever connect the two VMs to each other.

## Results

| Row | Requirement | Steps (short) | Verdict | Evidence |
|-----|-------------|---------------|---------|----------|
| G-01 | DSHR-03, DSHR-04, DSHR-06 | tray item Share Desktop… (AX press) starts the share in the GUI; the link goes by ssh pipe to a 0600 file on portty-test-2; Safari (safaridriver) opens it; Admit sheet read from Portty's AX tree; Admit pressed through AX; picture and View only chip read from the page; Stop Sharing pressed in the tray (AX) | PASS | `evidence/G-01.txt` |
| G-02 | DSHR-04, DCTL-04 | empty TextEdit, view-only guest clicks and types `zz`; `Allow control` switch pressed on the guest's row in the Share Desktop sheet (AX, role AXSwitch named `Allow control <name>`); guest clicks the text area and types `Portty 22` through WebDriver | PASS | `evidence/G-02.txt`: view-only: counters empty before and after (nothing posted, the page sends nothing without the right) and the document stays empty (`chars=0`); after the switch the daemon reports `control:true` and the page shows `You have control` at the first poll; the document reads exactly `Portty 22` (`chars=9`), counters `btn` 2, `key` 20, `move` 1 |
| G-03 | DSHR-05 | controlling guest holds A (WebDriver key down, 100 s pause); a poller inside the office reads the OS key state every 0.1 s (`keystate`); `Disconnect <name>` pressed on the guest's row (AX), timed inside the office | PASS | `evidence/G-03.txt`: the poller saw `KeyA` held from epoch ...331.76; AXPress started at ...378.43; the key state went back to `none` at ...378.57 (0.14 s after the press began) with the guest's own key up still 50 s away; the page text says `You were disconnected` and `You can't rejoin while this share is on`; the daemon reports 0 viewers; the audit file holds `revoked` and `leave` (reason `revoked`) for the guest. An earlier run of this row read the key state too late (rig latency under host load) and showed `none`; it was discarded and the poller added |
| G-04 | DSHR-05 | two guests on different tailnet nodes (the guest VM's Safari and the office's own Safari, see X-1), both streaming; Stop Sharing from the tray (AX), then twice through the same `DELETE /desktop/share` route timed to the millisecond inside the office | PARTIAL | `evidence/G-04.txt`: tray run: the audit shows `share_stop` and the office guest's leave at 19:37:48 and the guest VM's leave at 19:37:55, 6.9 s later (host load average read 55 a few minutes afterwards). The two timed repeats: DELETE answered in 0.25 s and 0.24 s, the two leaves came at +0.16 s and +1.51 s, and at +0.21 s and +0.99 s (daemon log, ms). Both pages then read `Sharing has ended`, the daemon reported no share and the old link answered `This link no longer works`. The criterion (both leaves within 2 s) held in two of three measurements; the 6.9 s straggler is reported as an open item for DSHR-05, not explained (see Notes) |
| G-05 | DCTL-04 | guest types `ab` (Portty's own tagged posts); then the guest types `c` 40 times, 250 ms apart, while one untagged `~/ptest/click` (CGEvent at the HID tap, the software stand-in for hardware) is fired 4 s in; an in-page MutationObserver logs the chip; after 4 s idle the guest types `d` | PASS | `evidence/G-05.txt`: tagged posts only: `physical` 2 to 2 (delta 0), `key` +4, document `ab`. With the stand-in click: `physical` 2 to 4 (delta 2), `key` +76 of 80 messages, the document holds 38 of the 40 `c`; the observer saw `Someone is using this Mac` on the guest page (n 1). After the lease passed: `d` posted (`key` +2, document ends `d`). The first attempt of this row used a wrong click-tool argument (the stand-in click never fired) and was discarded; the rig was fixed (`~/ptest/click click X Y`) and the whole row re-run |
| G-06 | DSHR-06, DCTL-04 | Terminal frontmost, Secure Keyboard Entry switched on through Terminal's menu (AX), the share's guest holds control; the banner read from Portty's AX tree (Share Desktop sheet closed: an open sheet makes the rest of the window inert in the tree); guest types `echo p22ske`; Secure Keyboard Entry off | PASS | `evidence/G-06.txt`: banner `Typing at this Mac isn't detected right now…` seen 1.38 s after the toggle began (polled every 0.25 s inside the office), `secureInput:true, anyController:true` from the daemon; the guest's typing arrived in Terminal (`key` +22, the string found once in Terminal's text); banner gone 1.56 s after Secure Keyboard Entry went off. The first run of this row polled with the sheet still open and saw nothing: the tree showed only the sheet, not the banner behind it (recorded in the Notes) |
| G-07 | DSHR-06 | new share, guest VM streaming, TextEdit frontmost (Portty not the active app); the office's own Safari (another node) knocks, is admitted and leaves; after each event the notification database (`usernoted` db, p19 `notif-list`) and Notification Center's AX tree are read; the first guest's picture is checked live each time (a local key press changes the screen first) | PASS | `evidence/G-07.txt`: all three notifications were delivered while the display was captured: `portty.desktop-admit` (`<login> wants to see your desktop.`) on the knock, `portty.desktop-join` on the join, `portty.desktop-leave` on the leave (records 77, 78, 79). Nothing was suppressed. The Admit sheet was in Portty's AX tree on the knock, and the first guest's `framesDecoded` rose at every stage (534 to 635, 1267 to 1369, 1825 to 1917, 2319 to 2428). Banners are transient and were not caught by the AX read (the records are the evidence of delivery). Two windows of one device on one share did not work as a second guest: X-1 |
| G-08 | DSHR-06 | tray menu through AX (`status item` menu, items and enabled state) with no share, sharing with nobody, one guest connected; menu-bar crop (`screencapture -R`) of the status item each time, compared with `cmp` and looked at | PASS | `evidence/G-08.txt`, `evidence/G-08-normal.png`, `G-08-nobody.png`, `G-08-sharing.png`: no share: `Share Desktop…`; sharing with nobody: `Desktop shared with nobody yet` with `Stop Sharing` directly under it and the icon unchanged (identical crop to normal); one guest: `Desktop shared with 1 person`, `Stop Sharing` directly under it, and the status item changes from the `P_` glyph to the `P` with a filled dot (the crops differ; the sharing crop also holds macOS's own capture indicator, so the glyph was confirmed by looking at the image). Light menu bar only: the dark variant is the deferred real-Mac item |
| G-09 | DSHR-05, DSHR-06 | new share with a guest streaming; SIGTERM to the Portty GUI process only, timed inside the office (daemon share state polled every 0.2 s); GUI started again afterwards | PASS | `evidence/G-09.txt`: the daemon reported no active share 0.51 s after the SIGTERM; the audit file holds `share_stop` (reason `gui_gone`) and the guest's `leave` (reason `gui_gone`) one second apart; the guest page reads `Sharing has ended`; the daemon kept running and answered on its socket; no GUI process remained. Restarting the GUI needed `open -n` (see Notes) |
| G-10 | DSHR-07 | the office's `desktop-audit.jsonl` after every row, read inside the office | PASS | `evidence/G-10.txt`: mode 600; 147 lines; `share_start` 12, `admit_request` 17, `admitted` 16, `join` 40, `control_granted` 5, `control_revoked` 2, `revoked` 5 (reasons `revoked` and `media_identity_mismatch`), `leave` 40, `share_stop` 12 with reasons `share_stopped` (11) and `gui_gone` (1); leaves with reason `viewer_closed` 26; no capability parameter and no token-shaped string (three base64url segments or 40 or more base64url characters) in any line |
| G-11 | DCTL-05, DSHR-04 | TextEdit window shared to a controlling guest (probe `select` of the document window, picture 1280x840); Spotlight opened locally by an untagged Cmd+Space, TextEdit's own Fonts panel opened by an untagged Cmd+T; the guest types `zz` while each panel holds keyboard focus; counters read before and after; keys sent again after Spotlight closed | PARTIAL | `evidence/G-11.txt`: baseline `k1` typed into the window; with Spotlight open (a `Siri` panel at layer 23) and with the Fonts panel open (a TextEdit panel at layer 3) the guest's keys were dropped, no `key` counted and nothing typed into the window (document unchanged), and the guest sees the notice that the window could not be brought to the front; after Spotlight closed the guest's `k2` posted (`key` +4, document `k1k2`). The drops were counted as `raising` (the Phase 20 frontmost check), not as `window_key_focus`, so the 22-05 `window_key_focus` path itself was not reached by either panel on macOS 27.2 (counter stayed 0). Panel contents could not be read (no Accessibility target for the `Siri` panel), so "nothing typed into the panel" rests on no key being posted. The case the fix is for, a non-activating panel that leaves the shared app as the focused application, needs a real Alfred or Raycast style panel: owner-run M-NN item |
| SC1 | DSHR-03 | second Tailscale user opens the guest link, waits at Admit, watches view-only, gets the own-devices refusal on a terminal link from the same device | UNVERIFIED | not run: "skip SC1" (orchestrator's decision at the checkpoint); there is no second Tailscale user on the test tailnet. Never PASS. DSHR-03's live two-account proof stays UNVERIFIED; owner-run M-NN row in 22-13. What the rows above did show is the guest flow between two nodes of the same account (G-01, G-02, G-04, G-07) |
| AR-21-01 | Phase 21 carried in | a node from another Tailscale account online must not appear as a Desktop row or terminal peer and a hand-typed connect must be refused as unknown host | UNVERIFIED | not run: it needs a node from another account; recorded with SC1, owner-run item. The unit tests of 22-10 cover the filter |
| Appendix B | DCTL-04 | real-Mac physical-input check (RESEARCH Appendix B), 2026-11-25 | PARTIAL | by reference to `evidence/appendix-b-real-mac.txt`: untagged hardware keys and mouse reach a listen-only tap under Accessibility only, with no Input Monitoring prompt, and carry no Portty tag. Not covered there: the warp check (a remote pointer warp must not eat local input) and the Secure Keyboard Entry case on real hardware; both deferred at the checkpoint to an owner-run real-Mac item. The covered part was not re-run; G-05 and G-06 show the VM equivalents with a software stand-in |

## Notes

- **The stand-in is software, not hardware.** G-05 posts its untagged click with `CGEvent` at the HID tap (`~/ptest/click`), RESEARCH A.5's honest stand-in. It proves the tap sees untagged events while a real controller is connected and that the arbiter drops the guest's input; Appendix B is what showed real hardware events look the same.
- **An open sheet hides the rest of the window from Accessibility.** With the Share Desktop sheet open, Portty's AX tree holds only the sheet (the banners and sidebar behind it are inert). G-06's first run polled for the banner with the sheet still open and found nothing; the sheet was closed and the banner appeared 1.38 s after Secure Keyboard Entry went on. Anyone reading banners by AX must close the sheet first.
- **Two windows of one device are not two guests.** X-1 (`evidence/X-same-node-windows.txt`): a second Safari window on the same link made the daemon log a join and a leave every 1 to 3 s (reason `viewer_closed`, the page closing its own connection) with one viewer at a time, and the notifications arrived coalesced as `10 viewers joined` and `16 viewers left`. Cause not investigated. G-04 and G-07 use the office VM's own Safari (its own tailnet node) as the second guest, which works: it knocks as a separate node and streams beside the first guest.
- **A joining node already admitted in this share is re-admitted without a knock** (observed while preparing G-07): the second guest's join came with no Admit sheet. G-07 therefore starts a new share before the knock.
- **Stop with two guests: one 6.9 s straggler** (G-04). In the tray run the office guest left with the `share_stop` and the guest VM's leave followed 6.9 s later; the two timed repeats, with the AX menu out of the way, were 0.2 s and 1.5 s, and 0.2 s and 1.0 s. The host's load average read 36 to 55 when checked during the session (VM calls took seconds each, and the office VM's load average read 26 once), so the straggler may be load. It is reported as an open item for DSHR-05 (a re-measure on a quiet machine, or an owner-run row with two real devices), not as a pass.
- **The picture stalled once on a long-lived viewer.** After many minutes with several joins and leaves, one guest page kept `status streaming` while `bytesReceived` stopped rising (G-07's first attempt: `framesDecoded` 881 for minutes); reloading the page gave a working picture at once. The page showed no `Reconnecting`. Not reproduced in the final runs and not investigated; recorded as an observation (heavy host load again).
- **Relaunching the GUI after it was killed:** `open /Applications/Portty.app` answered -600 while the daemon survived, because LaunchServices keeps the daemon process as the running Portty app; `open -n` started the GUI. A person double-clicking Portty after a GUI crash may therefore see nothing happen. Observed once (G-09), not investigated.
- **Spotlight on macOS 27.2 is not the M-05 case.** The `Siri` panel takes the focused application, so the Phase 20 frontmost check already drops the keys (`raising`, the guest sees the "could not bring the window to the front" notice). G-11 is PARTIAL for that reason; the unit tests of 22-05 cover `window_key_focus`.
- **Rows were re-run, not fixed in code.** No product code changed in this plan. Row scripts, the rig and the evidence were corrected as rig bugs surfaced (click tool argument, bash 3.2 quoting, a toggle switch pressed twice, an event-less static screen): each discarded run is named in its row.

## Owner-run items for 22-13 (M-NN candidates)

1. SC1: a second Tailscale user opens the guest link, waits at Admit, watches view-only after Admit, and gets the own-devices refusal on a terminal link from the same device (UNVERIFIED here).
2. AR-21-01: a node shared in from another Tailscale account is not offered as a Desktop row or terminal peer and a hand-typed connect is refused as unknown host (UNVERIFIED here).
3. Tray icon variant beside the other tray icons in a light and a dark menu bar (deferred; the VM showed the light bar only).
4. The Admit sheet arriving while typing in a real terminal: a stray Return, Space or Esc answers nobody, and a click within about 600 ms of the sheet appearing does nothing (deferred).
5. A right-to-left or look-alike display name on the real Admit sheet, login name still visible (deferred).
6. "Someone is using this Mac" appearing for a controlling guest while the owner types at the Mac with real hardware (deferred; G-05 used a software stand-in).
7. Warp check: with a guest controlling, move the real mouse right after a remote pointer move and confirm the local move registers; and the Secure Keyboard Entry check on real hardware: the banner shows while remote keys still post (deferred; Appendix B PARTIAL).
8. M-05 with a real non-activating panel (Alfred or Raycast style): guest keys are dropped as `window_key_focus` and nothing types into the panel (G-11 PARTIAL).
9. Stop with two real guests: both leave within 2 s on an unloaded machine (G-04 PARTIAL, one 6.9 s straggler under load).
10. Admit sheet above the Share Desktop and Settings sheets, Portty window forward on a knock; Share stopped banner after killing the daemon during a share; Quit GUI Only while sharing (from the 22-09 and 22-11 summaries; not driven here).
11. A restrictive tailnet access rule blocks the guest and the page shows "Can't reach this Mac" with the access-rules hint: NOT RUN (no restrictive policy is available on the test tailnet).

## Closure

Row verdicts: G-01 PASS, G-02 PASS, G-03 PASS, G-04 PARTIAL, G-05 PASS, G-06 PASS, G-07 PASS, G-08 PASS, G-09 PASS, G-10 PASS, G-11 PARTIAL; SC1 UNVERIFIED; AR-21-01 UNVERIFIED; Appendix B PARTIAL. No FAIL: DSHR-03's live two-account proof is unverified rather than failed, and the G-04 straggler is an open item for DSHR-05 with the evidence above.
