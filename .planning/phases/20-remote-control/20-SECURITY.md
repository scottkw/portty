---
phase: 20
name: remote-control
audited: 2026-11-26
auditor: gsd-security-auditor
asvs_level: 1
block_on: high
verdict: OPEN_THREATS
threats_total: 52
threats_closed: 51
threats_open: 1
threats_open_nonblocking: 0
unregistered_flags: 1
---

# Phase 20: Remote Control, Security Audit

Scope: the `<threat_model>` registers of 20-01..20-11-PLAN.md (T-20-01..T-20-51, plus T-20-SC, which all eleven plans declare and is counted once here), and 20-CONTEXT.md "Input security boundaries" and "Clipboard and permissions". The audit read code at HEAD of `gsd/phase-17-desktop-feasibility-spike`, limited to the Phase 20 file list. It did not run Portty, capture, input, AX or pasteboard on the host. It ran unit tests only: `go test ./internal/desktop ./internal/screencap` passed, `go test -run Desktop ./internal/daemon` passed, the named threat tests passed in verbose mode, and 8 vitest files (410 tests) passed. It also ran `tests/check-brand.sh` (rc 0), `tests/check-traceability-paths.sh` (OK, 295 paths) and an untagged `go build ./internal/daemon` (0 vmdesktop files).

Runtime evidence comes from 20-VM-CHECKS.md. Its blocking rows E1, C2, W2, D1, P2 and H1 passed. W2 was recorded before commit 78ea4fc8 (see T-20-21).

## Threat Verification

| Threat ID | Category | Sev | Disp | Mitigation | Evidence (file:line) | Status |
|---|---|---|---|---|---|---|
| T-20-01 | Spoofing | high | mitigate | Only a reliable, ordered `input` channel from a viewer that declared caps input 1 is kept, one per viewer, on the /desktop/ws session that was already authenticated (Origin, same-user, token, all before Accept) | internal/desktop/input.go:283-296; internal/desktop/manager.go:587,634; internal/webserver/desktop_routes.go:153-154,172-195 | CLOSED |
| T-20-02 | Tampering | high | mitigate | 4096-byte cap checked before decode, UTF-8 check, closed kind set, per-kind validation, clamp01, closed key table; SCTP max message 8192; 8 invalid golden lines | input.go:34,120-166,708-712; internal/desktop/peer.go:64; internal/screencap/keytable.go; internal/desktop/testdata/input-v1.jsonl | CLOSED |
| T-20-03 | DoS | medium | mitigate | OnMessage only parses and enqueues without blocking; a single input goroutine calls the bridge; per-viewer FIFO capped at 1024 | input.go:299-316,329-354,376-391 | CLOSED |
| T-20-04 | EoP | high | mitigate | Every press, move, wheel and text message passes inputLocked(). Locked() caches for 250 ms and returns true on a read error. Releases skip the gate | internal/desktop/gates.go:16-21,52; internal/desktop/state.go:116-130; input.go:632-653,713-728 | CLOSED |
| T-20-05 | Tampering | high | mitigate | @try/@catch in every ObjC entry point (input 7/7, window 5/6, pasteboard 3/4). Go recover in runInput, OnOpen, OnMessage, OnClose, raise and clip-read goroutines, and inputTick | internal/screencap/input_darwin.m, window_darwin.m, pasteboard_darwin.m; input.go:300,305,320,398,434; internal/desktop/window.go:254,265,283-288; internal/desktop/clipboard.go:159 | CLOSED |
| T-20-06 | Tampering | high | mitigate | Every posting bridge returns ErrRefusedInTest under testing.Testing(); tests use a fake bridge | internal/screencap/input_darwin.go:63,75,90,102 | CLOSED |
| T-20-07 | Info disc. | medium | mitigate | Logs carry counters keyed by kind and reason, release counts and viewer ID prefixes only. No key code, text, coordinate or title is logged | input.go:803,875,879,909; every slog call in Phase 20 Go files was reviewed | CLOSED |
| T-20-08 | Tampering (avail.) | high | mitigate | Held input is released on channel close, on leave and on Close (releaseAllViewers waits for the queued releases to post, up to 1 s). Releases bypass every gate and the full queue | input.go:321,848-880,885-911; manager.go:826,1071 | CLOSED |
| T-20-09 | DoS | medium | mitigate | Token bucket of 400/s with burst 200 per viewer; drops are counted; only presses that passed every other check spend a token; releases are never rate-limited | internal/desktop/input_map.go:15-16; gates.go:24-42,66-68 | CLOSED |
| T-20-10 | EoP | high | mitigate | Locked() is checked synchronously on each press, and presses are also refused for 3 s after a Ctrl+Cmd+Q chord (optimistic lock); TestInput_OptimisticLockOnChord passes | input_map.go:20; input.go:739-744; gates.go:20 | CLOSED |
| T-20-11 | Tampering (avail.) | high | mitigate | Release-all on lock and after 3 s of silence (dead-man), on channel close, leave, Close, the release message and a surface switch; modifiers are reconciled from the mask | input.go:396-421,499,758-780; input_map.go:18; internal/desktop/surface.go:325 | CLOSED |
| T-20-12 | Info disc. | medium | mitigate | Same as T-20-07: no code, text or coordinate appears in any log call | see T-20-07 | CLOSED |
| T-20-13 | Spoofing | medium | mitigate | Every press checks the viewer's RightsControl (runtime state, never a token claim) | gates.go:50; internal/desktop/viewer.go:15-29 | CLOSED |
| T-20-14 | Repudiation | low | mitigate | EventMarker is set in kCGEventSourceUserData by the single pcPost helper, and on the watchdog nudge too | input_darwin.m:33-37 (sole CGEventPost there); internal/screencap/sys_darwin.m:141; input_darwin.go:28 | CLOSED |
| T-20-15 | Tampering | high | mitigate | The epoch is bumped before the switch, on fallback and resize, and on reset; positional input with a stale epoch is dropped; held input is released on every switch | surface.go:170,323-325,440-472; gates.go:54 | CLOSED |
| T-20-16 | EoP | medium | mitigate | Listing and selecting surfaces check RightsSurface per viewer; TestSurface_SelectForbiddenAndBusy passes | surface.go:225,269 | CLOSED |
| T-20-17 | Info disc. | medium | mitigate | Titles go only to viewers on the wire; logs carry kind and viewer count; the switch error is the SCK localizedDescription, never a title | surface.go:231,290,336,339; internal/screencap/media_darwin.go:476 | CLOSED |
| T-20-18 | DoS | medium | mitigate | One switch at a time (busy); native steps bounded at 5 s; list bounded at 5 s in desktop and 15 s in native; 1 s poll; a failed switch keeps the old surface | surface.go:47,51,250-274,330-340; media_darwin.go:471-481; media_darwin.m:620,640,649,675 | CLOSED |
| T-20-19 | Tampering | high | mitigate | ListSurfaces, Start and the switch refuse under testing.Testing() | media_darwin.go:220,240,439,478 | CLOSED |
| T-20-20 | Info disc./Tampering | high | mitigate | Each press reads focus live from AX (system-wide focused app, not NSWorkspace) and checks both pid and frame. If focus is elsewhere, the window is raised and the press dropped. Fails closed on a nil bridge or any read error. Raise and NilBridge tests pass | window.go:144-147,155-191; internal/screencap/window_darwin.m:8,31-33,67-75 | CLOSED |
| T-20-21 | Tampering | high | mitigate | Topmost-window hit test per press: "a covering panel or banner gets nothing" | window.go:215-221. **Gap:** sameAppPopup (window.go:229-238, commit 78ea4fc8, after the VM run) lets a click through to any same-pid window with layer != 0, including floating panels at layer 3, or to a same-pid layer-0 window inside the shared frame. See the carve-out assessment | **OPEN (blocking)** |
| T-20-22 | EoP | high | mitigate | Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q are dropped daemon-side in window shares; held modifiers and the wire mask both count; TestWindow_ChordsDropped passes; VM row W2 counted window_chord | gates.go:60-64,74-95 | CLOSED |
| T-20-23 | DoS | medium | mitigate | AX messaging timeout of 0.25 s; the raise runs on its own goroutine, one at a time, with a 300 ms raise_failed signal; presses drop while raising | window_darwin.m:24,33,75,133; window.go:27,175-179,244-281 | CLOSED |
| T-20-24 | Tampering | high | mitigate | Every WindowBridge method refuses under testing.Testing() | internal/screencap/window_darwin.go:56,70,84,101,124 | CLOSED |
| T-20-25 | Info disc. | high | mitigate | Types are checked before any content read: Concealed, Transient and the 1Password type answer concealed; TestClipboardTracer_CopyFromMac passes; VM P2 logged `concealed bytes=0` | internal/screencap/pasteboard_darwin.m:55-60 (before stringForType at :65) | CLOSED |
| T-20-26 | Info disc. | high | mitigate | Reads happen only on clip_get and writes only on a completed clip_put; changeCount appears nowhere in internal/ or frontend/src; Cmd+C and Cmd+V stay keys | clipboard.go:136,206-226; manager.go:704-707 | CLOSED |
| T-20-27 | DoS | high | mitigate | alwaysDeny (3) is answered without reading; the read is bounded at 2 s on a separate goroutine; one read in flight per process; TestClipboard_WaitingApproval passes | pasteboard_darwin.m:46-49; clipboard.go:41,148-171 | CLOSED |
| T-20-28 | Tampering/DoS | medium | mitigate | Strict checks on xid, seq, total and len; 16 KiB chunks; the 1 MiB cap is refused before buffering; UTF-8 is validated; the transfer is discarded on any error, a gate failure or a leave | clipboard.go:35-37,237-273,209,214; manager.go:827 | CLOSED |
| T-20-29 | EoP | medium | mitigate | RightsClipboard is checked per action, and the action fails while locked | clipboard.go:115-121,141,213 | CLOSED |
| T-20-30 | Info disc. | medium | mitigate | Logs carry the outcome and byte count only | clipboard.go:109 | CLOSED |
| T-20-31 | Tampering | high | mitigate | Key listeners sit only on the stage's hidden input; there is no window or document keydown listener; the release chord always exits; blur releases | frontend/src/lib/desktopInput.ts:299-306,362-364,400-419 | CLOSED |
| T-20-32 | Tampering (avail.) | high | mitigate | Cmd chord is sent as a tap; forwarded map; release on blur, hidden, pagehide, channel close and View Only; 1 s heartbeat while anything is held | desktopInput.ts:190-203,317-322,377-397,422-425; frontend/src/components/DesktopView.tsx:442 | CLOSED |
| T-20-33 | Tampering | medium | mitigate | Only known enum values and finite or integer numbers are taken from control, cursor and surface messages; text is rendered as React text | frontend/src/lib/desktopClient.ts:625-634,668,674-701,907-924 | CLOSED |
| T-20-34 | Info disc. | low | mitigate | guardPasteInsert on the hidden input | desktopInput.ts:2,416 | CLOSED |
| T-20-35 | Tampering | medium | mitigate | No dangerouslySetInnerHTML or innerHTML in any Desktop* component, ContextMenu, desktopClient or desktopInput | grep across the Phase 20 frontend files: 0 hits | CLOSED |
| T-20-36 | DoS (Mac avail.) | high | mitigate | Lock This Mac confirm with initialFocus cancel ("Don't Lock"); greyed in window shares and while locked; the chord is never sent in a window share; the daemon also drops it | frontend/src/components/DesktopSendKeysMenu.tsx:43-63,80-97; RegenerateKeyModal.tsx:55; DesktopControlBar.tsx:67,544-545; gates.go:91-92 | CLOSED |
| T-20-37 | EoP | medium | mitigate | Send Keys greyed with a caption in window shares; the daemon-side drop is the real gate (T-20-22) | DesktopSendKeysMenu.tsx:43-63 | CLOSED |
| T-20-38 | Tampering | low | mitigate | The release chord moves focus to the Control segment; the hidden bar reveals on :focus-within and stays reachable by Tab | DesktopView.tsx:422-426; frontend/src/style.css:2771 | CLOSED |
| T-20-39 | Info disc. | high | mitigate | readText and write run only in the Copy from Mac and Paste to Mac menu items' onSelect; no polling; no copy or paste listener | DesktopControlBar.tsx:295-296; DesktopView.tsx:639-680,697-719,881-882 | CLOSED |
| T-20-40 | Info disc. | medium | mitigate | Text stays in memory or the open popover only; there is no console.* in DesktopView, DesktopClipboardPopover, DesktopControlBar, desktopClient or desktopInput | grep: 0 hits | CLOSED |
| T-20-41 | Tampering | medium | mitigate | xid, seq, total and len are checked; gaps and duplicates fail; the 1 MiB cap is checked before sending; chunks are 16 KiB or less | desktopClient.ts:705-740; DesktopView.tsx:686-687 | CLOSED |
| T-20-42 | Tampering | medium | mitigate | Clipboard text is set only as a textarea value | frontend/src/components/DesktopClipboardPopover.tsx:120-126,167-178 | CLOSED |
| T-20-43 | EoP | high | mitigate | `//go:build vmdesktop` is on both probe files; the daemon probe file registers the route in its own init() through extraDesktopRoutes, which is empty in release builds; check-brand.sh fails if the tag appears in .github, build.sh or scripts (rc 0); the untagged build has 0 vmdesktop files; the route is on the 0600 daemon socket; the installed build U answered 404 (evidence/TAGCHECK.txt) | internal/desktop/probe_vmdesktop.go:1; internal/daemon/desktop_probe_vmdesktop.go:1,17-21; internal/daemon/desktop_api.go:132-146; tests/check-brand.sh:72-74; internal/daemon/ipc.go:14 | CLOSED |
| T-20-44 | Tampering | high | mitigate | require_vm and require_vm2 guards (16 uses); every input, AX, pasteboard, tccutil and launchctl call goes through the VM ssh helper | .planning/phases/20-remote-control/rig/p20-rig.sh:6,62-66,153-183 | CLOSED |
| T-20-45 | Info disc. | medium | mitigate | Evidence and 20-VM-CHECKS.md grepped for 100.64/10 addresses, `.ts.net`, `/Users/ken` and long `cap=` tokens: 0 hits | .planning/phases/20-remote-control/evidence/, 20-VM-CHECKS.md | CLOSED |
| T-20-46 | DoS | medium | accept | Accepted: VM loss on stop; the plan never stops or suspends a VM and uses pf | Accepted Risks Log below; 20-09-PLAN.md threat_model | CLOSED |
| T-20-47 | Repudiation (consent) | medium | mitigate | Settings Row 1, Row 3 and the setup sheet say the link can control the Mac; tests pin the strings | frontend/src/components/SettingsSheet.tsx:93,369; DesktopSetupSheet.tsx:111; __tests__/SettingsSheet.desktop.test.tsx, DesktopSetupSheet.test.tsx (passing) | CLOSED |
| T-20-48 | Tampering | medium | mitigate | Only the fixed kinds "screen" and "accessibility" are accepted; "paste" stays refused (PASTE_ROW: none); openSystemURL refuses in a test binary | app_desktop.go:67-77; app.go:488-494 | CLOSED |
| T-20-49 | Info disc. | low | accept | Accepted: the shared window's title appears in status only over the 0600 daemon socket on the user's own Mac | Accepted Risks Log below; 20-10-PLAN.md threat_model | CLOSED |
| T-20-50 | Info disc. | medium | mitigate | Phase 20's TESTING.md additions (commits 943f483e, ce91e3c1) contain no home path, tailnet address, ts.net name or token; check-brand.sh rc 0 | TESTING.md (Phase 20 note line 36, rows ~1000, M-180..M-193) | CLOSED |
| T-20-51 | Tampering | low | mitigate | Every Phase 20 test file has a TESTING.md row; check-traceability-paths.sh reports OK (295). peerDesktop.test.ts is a 21-05 file and is out of scope | TESTING.md Section 2 and 4 | CLOSED |
| T-20-SC | Tampering (supply chain) | high | mitigate | No package added: none of the 55 Phase 20 commits touches go.mod, go.sum, frontend/package.json or pnpm-lock.yaml | git log of Phase 20 commits | CLOSED |

## Open Threats

### T-20-21 (high, blocking): the same-app carve-out widens "covered target"

Declared mitigation (20-04): "Topmost-window hit test per press; a covering panel or banner gets nothing; TestWindow_TopmostHitDrops."

**What still holds at HEAD.** A click or scroll in a window share passes only when all of these are true:

- the viewer has RightsControl;
- the Mac is not locked;
- the epoch is current;
- Accessibility is present;
- a live AX read says the shared app is focused and its focused window has the shared frame (window.go:186-191);
- the point is inside the freshly read frame (window.go:196-199);
- the topmost window there has the shared app's pid (window.go:230).

Another app's windows still drop as `covered`. That includes password managers, Notification Center banners and the Spotlight panel. The tests "other app, normal layer" and "other app, menu layer" pin this. Keys and text do not use the hit test: they depend on the AX focus check, so the carve-out does not affect them.

**What widened.** `sameAppPopup` (window.go:229-238, commit 78ea4fc8, added after the VM run) treats any same-pid window with `Layer != 0` as the app's own pop-up. That covers more than menus (layer 101):

- floating panels and palettes (layer 3);
- modal panels (layer 8);
- utility windows;
- any same-pid layer-0 window lying wholly inside the shared frame.

The VM evidence shows the effect. evidence/W2.txt:1 records TextEdit's Fonts panel as `layer=3`. W2 recorded "the same click once the document had focus was dropped as covered". At HEAD that same click would be posted to the Fonts panel. The W2 PASS predates the change and is stale for this case.

The window-share capture is `initWithDesktopIndependentWindow` (internal/screencap/media_darwin.m:417) and sets no child-window option. So these panels are most likely not in the viewer's picture: VM row W3 was NOT RUN and M-185 is still pending. The viewer sees the shared window's own content and clicks on it, and the click lands on an unseen window, which is the exact vector T-20-21 names.

No test pins a same-app floating panel. M-186 rechecks a pop-up menu and a Finder window, not a layer-3 panel.

**Assessment.** For Phase 20 the widening has low practical impact:

- viewers are own-account only (RightsOwnAccount, manager.go:588);
- they already hold RightsSurface, so they could switch to a display share and click the panel anyway;
- the click stays inside one app the user already controls.

The window-share boundary matters more later. 20-CONTEXT.md marks it "costly" to reverse, and Phase 22 guests inherit it. The change was made after verification, without a register entry or a re-check of the floating-panel case. As declared, the mitigation is therefore not fully present.

**To close (choose one).**

- (a) Narrow `sameAppPopup` to pop-up-menu levels (layer >= 101, the kCGPopUpMenuWindowLevel family) plus sheets attached to the shared window. Add a TestWindow_SameAppPopupAllowed case: "same app, floating panel (layer 3) over the window drops as covered". Then re-run W2's panel click and M-186.
- (b) The owner records an accepted risk below ("same-app windows over a shared window may receive clicks; own-account viewers only; re-review before Phase 22 grants window shares to guests") and runs M-185 and M-186.

## Carve-out assessment (requested): does sameAppPopup widen the window-share boundary acceptably?

- **Cross-app boundary (20-CONTEXT "a password manager or another app they cannot see"):** intact. The pid must match, keys stay gated by live AX focus, and other apps' menus and panels still drop.
- **Same app, menus and pop-ups (layer >= 101) and sheets inside the frame:** acceptable for Phase 20 own-account viewers. This is the X1 usability fix the owner asked for. M-186 must confirm it on a real Mac.
- **Same app, floating, utility and modal panels (layers 3 and 8):** not covered by the stated intent ("menu or pop-up layer above 0"). This contradicts the W2 result and is probably invisible to the viewer. It is not acceptable without either narrowing or an explicit owner-accepted risk. This is the open item above.

## Accepted Risks Log

| Threat ID | Risk | Rationale | Source |
|---|---|---|---|
| T-20-46 | A Tart VM stopped for hours drops off the tailnet and needs re-keying and a Tailnet Lock signature | The rig never stops or suspends either VM; network loss is simulated with pf, not `tailscale down` | 20-09-PLAN.md threat_model (accept) |
| T-20-49 | The title of the window the user chose to share appears in the GUI and tray status | Only on the user's own Mac, over the 0600 daemon socket; no new endpoint | 20-10-PLAN.md threat_model (accept) |

## Unregistered Flags

All eleven SUMMARY.md `## Threat Flags` sections say "none beyond the register".

| Flag | Source | Mapping |
|---|---|---|
| Post-VM same-app carve-out (78ea4fc8) changes what a window share permits | 20-VM-CHECKS.md X1 follow-up; git log; not in any SUMMARY Threat Flags section | Widens T-20-21 (open above) |

## Advisory (not counted, not blocking)

- **System chords in window shares.** Only the three DCTL-03 chords are filtered. Other system shortcuts are posted while the shared app has focus: Mission Control and Spaces (Ctrl+arrows), Cmd+Opt+Esc, and the screenshot chords. Each can open system UI, but the next press then meets the live-focus raise-and-drop and the covered gates, so it cannot drive another app. Revisit before Phase 22 guests.
- **AX focus-read failure (VM X3).** A -25204 failure for about ten minutes makes every window-share press drop. That fails closed, so it is an availability issue only; watch for it on the real Macs.
- **Releases bypass the window gate by design (T-20-08, T-20-11).** A key-up can reach a different focused app. This is harmless and intended, since it prevents stuck keys.
- **waiting_approval with a real paste alert (VM P1).** Never reached in a VM, where accessBehavior is alwaysAllow. The unit test TestClipboard_WaitingApproval covers it.
