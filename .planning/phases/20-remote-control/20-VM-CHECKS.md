# Phase 20 VM checks (plan 20-09): remote control on macOS 27.2

Rig: `.planning/phases/20-remote-control/rig/p20-rig.sh` (builds on `p19-rig.sh`; `ax20.swift` and `pbset.js` ship to the guests). Office = portty-test (controlled Mac: Portty, TextEdit, the probe route), home = portty-test-2 (viewer: Safari through safaridriver, Chrome with DevTools on loopback). Every capture, input post, AX call, pasteboard access, launchctl and tccutil call ran inside a VM; the host only built, signed, compiled `ax20` and wrote evidence. Neither VM was stopped; no `tailscale down`; no host tailnet contact. Evidence in `evidence/` carries no tailnet address, name or token (addresses show as `100.x.x.N`, home paths as `~`).

Builds (this tree, Developer ID, hardened runtime, identifier app.portty.Portty, so the grants survive every swap):

| Build | Version | Tags | Used for |
|-------|---------|------|----------|
| T | 20.9.3 | `wailsassets vmdesktop` | every row below (carries the VM-only `POST /desktop/debug/vm20`) |
| U | 20.9.4 | `wailsassets` | left installed afterwards; answers 404 to the route (`evidence/TAGCHECK.txt`) |

The plan names the tags `wailsassets vmpreview vmdesktop`. `vmpreview` fakes Tailscale (media bind 127.0.0.1) and the desktop refuses to start on it (`media bind address is not a tailnet address`), so build T omits it, as 19-12's did. Builds 20.9.1 and 20.9.2 were built, deployed and replaced while finding this and the viewer fix below.

Two fixes the rows needed (code, not rig): the viewer page was covered by `index.html`'s opaque static splash, which swallowed every pointer event (commit f97d700e, see the SUMMARY); and the WebDriver rig works around Safari sending code `Unidentified` for a bare space, an idle-hidden control bar, and a Clipboard trigger a WebDriver click closes again (see Notes).

| Row | Requirement | Steps (short) | Verdict | Evidence |
|-----|-------------|---------------|---------|----------|
| E1 | DCTL-02 | `textedit-new` (empty document, SIGKILL and state folders cleared first), WebDriver click on the stage over TextEdit's text area, `safari-type "Portty 20"` (capitals as Shift down, letter, Shift up; the space as the WebDriver Space key) | PASS | `evidence/E1.txt`: document reads `Portty 20` (chars=9). Counters `btn` +2, `move` +1, `key` +20. The whole path ran: Safari key events, input data channel, daemon gate, CGEvent posts from the launchd daemon, TextEdit |
| E2 | DCTL-01 | AX frame of the first character (`charrect 0 1` = 105,180 8x14), normalized point (107/1280, 187/720) clicked through WebDriver, then `X` | PASS | `evidence/E2.txt`: after the click the selection is at location 0 (it was 9), then the document reads `XPortty 20` |
| C1 | DCTL-03 | probe chord MetaLeft+Space, then Escape, then MetaLeft+Tab; then the viewer's Send Keys menu, Cmd+Space | PASS | `evidence/C1.txt`: Cmd+Space opens the system search panel (on 27.2 it is a window owned by `Siri`, layer 23, 640x64, the frontmost app stays TextEdit) and Escape closes it; Cmd+Tab changes the frontmost app TextEdit to Finder; the Send Keys menu's Cmd+Space opens the same panel. Posts from the launchd daemon work (A3) |
| C2 | DCTL-03 | Send Keys, Lock Screen, confirm, through WebDriver; guest polls GET /desktop every 0.2 s; type `abc` through the viewer; probe key while locked; AX read of loginwindow's text field; `p19 unlock` | PASS | `evidence/C2.txt`: click at guest epoch ...846.68, `locked` true seen at ...847.62 (under 1 s, the poll step is 0.4 s). While locked the viewer shows `Locked` and sent nothing for `abc` (daemon `key` counter unchanged at 132; the viewer pauses itself). A probe key while locked counted `locked:1`, `key` unchanged. loginwindow's text field holds 0 characters after both. Unlock restored `streaming` (A4) |
| T1 | DCTL-02 | 1024 bytes (48 x e-acute, n-tilde, u-diaeresis, grinning face, woman technologist with ZWJ, plus 16 x) through probe text into an empty TextEdit | PASS | `evidence/T1.txt`: sha1 of the string and of TextEdit's text both `b3ac815e91fe`, utf8 1024 bytes (A6). First attempt differed in one byte: TextEdit's own autocapitalisation upper-cased the first letter (`e` became `E`); `textedit-new` now turns that default off, the run shown is the second |
| S1 | DCTL-01 | 120 lines in TextEdit, Cmd+Up, pointer over the text, natural scrolling on then off (`defaults write -g com.apple.swipescrolldirection`), probe scroll CG dy -240 and +240, vertical scroll bar read through AX | OBSERVED | `evidence/S1.txt`: CG dy -240 (the browser's positive dy, scroll down) moves the content up and the bar from 0 to 0.179 with natural scrolling on and with it off; CG +240 returns it to 0. The preference does not re-invert posted pixel scrolls (A7). The sign rule holds; no code changed |
| W1 | DCTL-05 | Finder frontmost, probe raise on TextEdit's window id; minimize it (AX button), probe minimized, probe raise; `front` Finder and TextEdit alternately with two probe focus reads each | PASS | `evidence/W1.txt` (second run, see X3 for the first): raise from the launchd daemon confirmed by rung 1 (Accessibility) in 210 ms with Finder frontmost; the window then reads minimized true, raise restored it in 621 ms (rung 1), on screen and frontmost; the AX focused-app read followed all four app switches with the right pid both times (no stale value). Rung 1 was enough every time, 2 and 3 never needed (A1, A2) |
| W2 | DCTL-05 | window share of TextEdit's document (probe select), click inside through the viewer, Fonts panel (Cmd+T) moved over the document, click on the panel, click on an uncovered part, Cmd+Tab | PASS | `evidence/W2.txt`: click inside lands (selection 5 to 0, `btn` +2); click on the panel while the panel had focus was dropped as `raising` (no post), the same click once the document had focus was dropped as `covered` (`btn` unchanged, selection unchanged); an uncovered click landed (selection 0 to 5); Cmd+Tab counted `window_chord:1` and TextEdit stayed frontmost. A click outside the window cannot be made through the viewer (its picture is the window), so `outside` stays covered by the unit tests |
| W3 | DESK-05 | Save sheet (Cmd+S) in a window share and the pop-up of X1, read from the window server | NOT RUN | `evidence/W3.txt`, `evidence/X1.txt`: the decoded frame could not be read. Safari's `drawImage` of the video throws `The object is in an invalid state` in the VM (WebKit has no IOSurface here, as with screenshots) and Chrome's DevTools needs a websocket client the guests lack. Measured instead: the Save sheet is its own window (id 3184, 390x218, layer 0) beside the shared window (id 3178), and the pop-up is its own window (id 3086, layer 101). Apple's WWDC22 note (research R10) is that a single-window capture excludes such windows; this run does not confirm or refute it. Moves to an M-NN item |
| D1 | DESK-06 | Safari and Chrome viewers open, probe select display, window, display, window, each polled 3 times after each switch; daemon log | PASS | `evidence/D1.txt`: decoded size 2560x1440 (display) and 1280x840 (window) in Safari, width 2560 and 1280 in Chrome, `framesDecoded` rising in both after every switch, status `streaming` in every sample, viewer count stayed 2, the log has one `viewer joined` (Chrome opening) and four `surface switched`, no second join for either viewer, no Reconnecting (A11) |
| D2 | DESK-06 | display share, TextEdit untouched 12 s, probe select the window; Safari's `resize` event time against the select request time; Chrome title samples at 0.1 s | PASS | `evidence/D2.txt`: select sent at ...008.70 (guest epoch), Safari's first frame of the new size at ...009.54 (0.84 s; the clocks agree within 0.3 s and the select call itself took 0.64 s), Chrome's title read `fw=1280` at ...009.53 (its debug title ticks once a second). The still-frame fallback did not fire for the idle document (the caret blink supplied a frame); it fired once earlier, selecting a hidden 500x500 helper window (`screencap: switch used a still frame`, in the first W2 attempt that was discarded) (A12) |
| P1 | DCTL-06 | probe behavior; pb-set `probe text`, probe pbread with nobody touching the VM; probe pbwrite; a concealed item read; windows listed before and after each | OBSERVED | `evidence/P1.txt`: the launchd daemon's `accessBehavior` is `alwaysAllow`; reading text answered ok in 6 ms, writing in under 1 ms, reading a concealed item answered concealed with no content; no new window of any app appeared at any point (no alert), the daemon kept serving. The `waiting_approval` path was therefore never reached in a VM (it is covered by `TestClipboard_WaitingApproval`) (A10, R14) |
| P2 | DCTL-06 | office clipboard `from mac`, Copy from Mac through the viewer; home clipboard `to mac`, Paste to Mac; office clipboard concealed `secret`, Copy from Mac | PASS | `evidence/P2.txt`: daemon log `clipboard outcome=ok bytes=8`, then `ok bytes=6`, then `concealed bytes=0`. The menu and its items were activated by JavaScript `click()` (see Notes), which gives Safari no user activation, so both directions used the viewer's fallback popovers: the Copy popover appeared (`Your browser didn't let Portty write to your clipboard...`), the Paste popover took `to mac` and Send to Mac left the office clipboard reading `to mac` (`Sent to the Mac's clipboard; press ⌘V`). The concealed item showed `The Mac's clipboard holds a hidden item (password manager); not copied` and the home clipboard kept its marker. Safari writing the clipboard from a real click was not measured (M-NN) |
| H1 | DCTL-02 | click the text area through the viewer, Shift down with no up, close the Safari session; daemon log; probe-typed z with no viewer; reopen the viewer and type a | PASS | `evidence/H1.txt`: the viewer's summary shows `hb:10` (heartbeats while it held a key) and `key:3`, then the viewer left; a letter posted next came out `z`, and `a` through the new viewer made the document `za`: Shift was released. (The first attempt's click had hit a still-open menu overlay of the previous rows and is not used) |
| A1 | CONTEXT [Permissions] | `tccutil reset Accessibility app.portty.Portty` in the office VM, then pointer moves through the viewer | PASS | `evidence/A1.txt`: `{sr:true, ax:false}` straight after the reset and no prompt; after the second and third moves `control_input` was `needs_accessibility` (`effect_miss:3`) and the viewer showed `Control needs Accessibility. On the Mac, allow Portty in Privacy & Security.` |
| A2 | CONTEXT [Permissions] | Accessibility granted again in System Settings (automated, see Task 2 below) with the viewer open and idle; poll every 0.5 s | PASS | `evidence/A2.txt`, `evidence/A2-regrant.txt`: `ax` true seen at ...361.1, `control_input` back to `ok` at ...365.1 (the poll prints only changes), at most 4 s after the grant, with no viewer action; the viewer page lost its chip (`evidence/P3.txt`) |
| P3 | DCTL-06 | after the re-grant: probe pbread, a viewer Copy from Mac, windows listed; Privacy & Security searched for a paste setting | PASS | `evidence/P3.txt`: probe read ok (7 bytes), viewer Copy from Mac `outcome=ok bytes=7`, no alert window; an Accessibility-tree search of the Privacy & Security pane for `paste`, `clipboard` and `pasteboard` found nothing, so there is no System Settings path to record |
| X1 | extra (20-04) | window share, Typeface pop-up opened with AX press, click on its Menlo item through the viewer | OBSERVED | `evidence/X1.txt`: the click was dropped as `covered` (counter 1 to 2, no post), the typeface stayed Helvetica and the menu stayed open. A menu, pop-up or sheet of the shared app counts as covering, so none of it can be clicked in a window share. A same-app carve-out is needed if pop-ups must work |
| X2 | extra (20-06) | status read at the first poll after each viewer opens | OBSERVED | `evidence/D1.txt` and the Safari open in `evidence/H1.txt`: Safari read `streaming` with `framesDecoded` 0 at its second poll, Chrome read `streaming` at `fd=2` on its first; the 20-06 rule (picture needs the track and `connected`) never showed `streaming` ahead of a working media path in either engine. `connectionState` itself is not exposed by the debug hook, so "connected before the first frame" is inferred, not read |
| X3 | extra (20-04) | AX system-wide focused-app read during the first W1 run | OBSERVED | `evidence/W1.txt` first run was discarded and is described here: for about ten minutes after the C2 lock and unlock, T1, S1 and the natural-scroll toggles, `probe focus` answered `reading the focused window failed (code -25204)`, `Raise` answered -90203 after 1.8 s and `Minimized` -90202, and a trusted `ax20 sysfocus` run over ssh got -25204 too (reading the frontmost app's own focused window worked at the same time). Roughly six minutes and a fresh TextEdit later every read succeeded. Cause not found. The gate failed closed throughout (the raise even brought the window forward but could not confirm focus) |

## Notes on the rig

- Safari's WebDriver sends code `Unidentified` for a bare space (the daemon counted `unknown_code`); `safari-type` sends the WebDriver Space key (``) and Shift around capitals.
- The control bar hides when the pointer idles, and a WebDriver click on a hidden control lands on the stage and posts to the Mac; the Clipboard trigger (open on click) is closed again by the click event of a WebDriver element click. P2 and P3 therefore open the menu and pick its items with `click()` from JavaScript. Send Keys worked with real WebDriver clicks.
- TextEdit restores an earlier session's text after `pkill`; `textedit-new` kills with SIGKILL and removes its saved-state folders, and switches TextEdit's autocapitalisation off.
- Cmd+Space on 27.2 opens a window owned by `Siri` at layer 23, not a `Spotlight` one, and it is not in the daemon's surface list; `ax20 cgwin` lists it.
- The re-grant (A2) needed System Settings only: the switch row for Portty stays in the Accessibility list after `tccutil reset`, a click on it asks for the administrator once (a sheet with a password field, typed inside the VM), and a later click in the same session needed no sheet.
- `GET /desktop/permissions` sometimes timed out at 2 s during A2 (two empty answers in 70 polls) while the daemon was otherwise answering.

## Closure

Row verdicts: E1 PASS, E2 PASS, C1 PASS, C2 PASS, T1 PASS, S1 OBSERVED, W1 PASS, W2 PASS, W3 NOT RUN, D1 PASS, D2 PASS, P1 OBSERVED, P2 PASS, H1 PASS, A1 PASS, A2 PASS, P3 PASS; extras X1 OBSERVED, X2 OBSERVED, X3 OBSERVED. The blocking rows E1, C2, W2, D1, P2 and H1 all passed.

Research assumptions:

| Assumption | Answer | Row |
|------------|--------|-----|
| A1 raise from the launchd daemon | holds: rung 1 (Accessibility) raised and restored a minimized window | W1 |
| A2 live AX focused-app read | holds in the second run (four switches, no stale value); failed with -25204 for about ten minutes in the first run | W1, X3 |
| A3 Cmd+Tab and Cmd+Space posted by the daemon | hold (Cmd+Space opens the Siri-owned panel on 27.2) | C1 |
| A4 Ctrl+Cmd+Q posted as key events locks | holds, under 1 s | C2 |
| A5 posts into password fields with Secure Event Input | not measured: the lock gate stops all input while locked, which is what C2 shows | C2 |
| A6 20 UTF-16 units per event | holds: a 1024-byte string of accents, emoji and a ZWJ sequence arrived exactly | T1 |
| A7 natural scrolling does not re-invert posted scrolls | holds: same direction on and off | S1 |
| A8 Caps Lock through alpha-shift | not run | none |
| A9 `kCGEventSourceUserData` survives (Phase 22) | not run | none |
| A10 NSPasteboard alert from a launchd daemon | no alert for reads or writes; `accessBehavior` `alwaysAllow` | P1, P3 |
| A11 in-band resolution change | holds in Safari and Chrome (Wails web view and iPhone not run) | D1 |
| A12 first frame after a switch onto a still surface | a frame arrived within 0.84 s; the still-frame fallback fired once (hidden helper window) and not for an idle document | D2 |
| A13 sheets in a window share | decoded frame not read; the sheet is a separate window | W3 |
| A14 cheap window frame fetch | not measured | none |
| A15 double-click interval | not run | none |

Rows 20-11 turns into M-NN items (NOT RUN and OBSERVED):

- W3: sheet, Open and Save panel and pop-up menu in the decoded window frame (needs a Chrome DevTools client or a real Mac).
- X1: pop-ups of the shared app are unclickable in a window share; decide on a same-app carve-out.
- X3: the AX system-wide focus read failing with -25204 for a while on 27.2 (window share would drop every press during it); watch for it on the real Macs.
- P1: the `waiting_approval` path with a real paste alert (never reached in a VM).
- P2: Safari writing the clipboard from a real click on Copy from Mac, and Safari's own Paste prompt for Paste to Mac.
- S1: scroll direction on a real trackpad Mac with natural scrolling on and off.
- A5, A8, A9 and the Wails web view and iPhone halves of A11.

Findings for the user:

- The /desktop/ viewer shipped with an opaque full-window splash over the picture (fixed in f97d700e). Phase 19's real-media rows read `__desktopStats` only, so none could have seen it.
- The VM-only route and its two files are in the repo behind the `vmdesktop` tag; the installed build is the untagged U.

PASTE_ROW: none

The decision: P1 saw no paste-privacy alert for the launchd daemon reading types, reading text or writing text, `accessBehavior` reports `alwaysAllow`, P3 repeated the reads after the re-grant with the same result, and System Settings lists no Paste from Other Apps setting for Portty. There is no alert wording and no System Settings path, so 20-10 builds no Paste from Other Apps row.
