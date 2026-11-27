# Phase 20: Remote Control - Research

**Researched:** 2026-11-25
**Domain:** macOS input injection (CGEvent), ScreenCaptureKit surface switching, AX window raise, NSPasteboard, browser keyboard/IME/clipboard capture, pion data channels
**Confidence:** MEDIUM (stack and wire design HIGH; macOS behaviours that no Phase 17 row measured are LOW until a VM row confirms them, see the risk section)

<user_constraints>
## User Constraints (from 20-CONTEXT.md)

### Locked Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the six hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

**Input wire protocol**
- Pointer and key events travel on one reliable, ordered WebRTC data channel per viewer labelled `input`, carrying small JSON messages `{t, ...}`, with the input protocol version negotiated once in the signalling hello (`input: 1`); pointer moves are coalesced to one per animation frame on the viewer. Low-rate, bulk or stateful messages (surface list and select, clipboard) stay on the versioned /desktop/ws signalling socket. The data channel is proven in every viewer engine (R13, R14), avoids head-of-line blocking behind signalling, and ordered delivery keeps move-before-click correct. Reversibility: costly.
- A viewer sends pointer positions as normalized `x, y` in 0..1 of the currently shared surface (display or window), plus the surface `epoch` it last received; the daemon maps to global points itself (display origin including negative origins, backing scale, window frame) and drops input carrying a stale epoch. Buttons are down/up events with a button index, the daemon computes click count (double and triple click) from timing and distance, and scroll is pixel deltas (dx, dy) after the viewer converts line and page modes. Viewers never need to know Retina scale, display layout or window position. Reversibility: costly.
- Keyboard input uses two message kinds: `key` with the W3C `KeyboardEvent.code` (layout-independent physical key), down/up, a repeat flag and a modifier snapshot, which the daemon maps to a macOS virtual key code through a closed table (unknown codes dropped) so shortcuts work and the Mac's own layout picks characters; and `text` with a composed Unicode string (IME commit, emoji, dictation, iPhone soft keyboard) typed with CGEventKeyboardSetUnicodeString. Send-keys items and the Ctrl→Cmd swap are built in the viewer from ordinary `key` messages, so the protocol has no special actions. Reversibility: costly.

**Input security boundaries**
- While CGSSessionScreenIsLocked is set, the daemon drops all pointer and key input from every viewer, and viewers show Locked with control greyed out; Phase 23 opens a dedicated, own-account-only unlock path. Reversibility: costly.
- In a window share, keys and clicks are posted only while the shared window's app is frontmost and the window is its main window; the first input after focus moved elsewhere raises the window (activate the app, AXRaise the window) and that triggering event is dropped; pointer moves are clamped to the window frame and clicks outside it are dropped. Reversibility: costly.

**Clipboard and permissions**
- The daemon reads NSPasteboard only when a viewer clicks Copy from Mac, checks item types first and skips concealed and transient items before reading content. A VM row first measures whether macOS 27.2 shows the paste-privacy alert for a launchd daemon read (and that a launchd Aqua agent can read NSPasteboard at all). If the alert appears, the Phase 19 permissions sheet gains a Paste from Other Apps row, and a read that would prompt with nobody there returns Waiting for approval at the Mac instead of hanging. Reversibility: costly.

**Decided autonomously**
- [Packages] CGEvent, AX raise and NSPasteboard bridges as cgo ObjC in internal/screencap (input_darwin.m, pasteboard_darwin.m); per-viewer input state, gates and surface selection in internal/desktop; frontend lib/desktopInput.ts beside desktopClient.ts
- [Posting] CGEventPost at kCGHIDEventTap from one dedicated CGEventSource, every event tagged in kCGEventSourceUserData with a Portty marker so Phase 22 can tell remote from physical input
- [Posting] Dragged event types while a button is held; moves warped to the mapped point; one input goroutine per share serialises posts
- [Mapping] Global points from SCDisplay/CGDisplayBounds (negative origins and scale handled) or the SCWindow frame refreshed on move and resize; mapping rebuilt on display reconfiguration
- [Keys] Hand-written closed table of W3C codes to kVK constants (ANSI plus ISO/JIS extras); modifiers posted as their own key events with flags from the daemon's held set; browser repeat keydowns forwarded with the autorepeat field
- [Stuck keys] Per-viewer held key and button sets; release-all on viewer blur, visibilitychange hidden, pagehide, data-channel close, peer failure, revoke, surface switch and lock; dead-man: while anything is held the viewer sends a 1 s heartbeat and the daemon releases all after 3 s of silence
- [Limits] Data-channel messages up to 4 KiB, text messages up to 1 KiB each (viewer splits), a per-viewer rate cap with drop counters; clipboard up to 1 MiB text sent in chunks of at most 32 KiB to keep the HARD-02 WebSocket read limit
- [Logging] Only counts and event kinds; never key codes, text or clipboard contents
- [Permissions] Because AX preflights are cached per process (R08), the daemon checks the effect of a posted move (fresh CGEvent location) and pushes a Control needs Accessibility state to viewers and the GUI
- [Surface] select message on /desktop/ws, SCStream updateContentFilter on the running stream, encoder reconfigure for the new size, forced IDR, epoch bump pushed to viewers, held input released; no renegotiation
- [Surface] Not persisted; every capture start uses the main display and desktop.json is unchanged
- [Window] NSRunningApplication activate plus AXRaise on the AX window matched by title and frame (no private _AXUIElementGetWindow)
- [Clipboard] public.utf8-plain-text only; skip items carrying org.nspasteboard.ConcealedType or org.nspasteboard.TransientType; writes from viewers set a plain string
- [Arbitration] No arbitration in Phase 20; events interleave per viewer with separate held sets; DCTL-04 adds the 2 s hold in Phase 22
- [Dependencies] None: system frameworks via cgo and vanilla browser APIs
- [Testing] Pure-Go tests behind a fake poster for code mapping, coordinate mapping, epoch drop, release-all, lock gate, window confinement and limits; loopback data-channel integration tests reusing the Phase 19 synthetic source; VM rows for real CGEvent posting, window raise and the pasteboard alert check; real-Mac M-NN rows for Retina plus external display mapping, IME and emoji typing, and clipboard on the office Mac; TESTING.md updated per the standing rule

### Claude's Discretion
- Slim auto-hiding top bar with Display/Window picker, Send keys menu, Copy from Mac, Paste to Mac, a Ctrl→Cmd toggle and a View only/Control toggle.
- Control starts on for own-account viewers; keyboard captured only while the video area has focus (click to focus).
- Display/window picker lives in the viewer, not the host GUI; host Settings Desktop group and tray state line show what is shared.
- Window list shows on-screen normal windows as "App — Title", including Portty's own.
- When a shared window closes or its display disappears, fall back to the main display with the notice "The shared window closed; showing the main display".
- Ctrl→Cmd swap off by default, toggled in the bar, remembered per browser.
- Send keys menu lists Cmd+Tab, Cmd+Space and Lock Screen (Ctrl+Cmd+Q chord) only.
- Copy from Mac writes to the viewer's clipboard with a "Copied from Mac" toast, shows the text in a selectable box when the browser refuses the write, and says "The Mac's clipboard holds a hidden item (password manager); not copied" for a concealed item.
- Paste to Mac sets the Mac's clipboard and toasts "Sent to the Mac's clipboard; press ⌘V", without pressing Cmd+V itself.
- Cmd+C and Cmd+V typed in the viewer go to the remote Mac as keys and never sync clipboards.
- Over the video the local pointer is a small dot, with the real cursor visible in the frames.
- A Fullscreen button enters fullscreen and, in Chrome, requests Keyboard Lock; Safari gets fullscreen only.
- While locked, the bar shows Locked and the control items are greyed out.

### Deferred Ideas (OUT OF SCOPE)
- Control arbitration (last typist holds about 2 s, physical input wins): DCTL-04, Phase 22.
- Per-person control and clipboard grants, guest view-only, and a host-side surface picker for click-to-share: Phase 22.
- Portty app Desktop tab with Cmd+W/Q/C forwarding, and iPhone trackpad mode, long-press right-click, two-finger scroll and soft keyboard: Phase 21.
- The guided unlock flow and lock-on-disconnect when the session started locked: Phase 23.
- Images and rich text on the clipboard; file transfer stays out of scope.
- A data-channel cursor (REQUIREMENTS Future).
- Persisting the chosen display or window across captures.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DCTL-01 | Viewer with control can move, click, scroll and drag; pointer lands right on Retina | Normalized mapping in global points (no backing scale needed for CGEvent), post `kCGEventMouseMoved` not CGWarp, `*Dragged` types, scroll sign inversion, viewer-timestamp click counts, letterbox-aware normalization (Patterns 1, 2, 5) |
| DCTL-02 | Type incl. shortcuts and non-ASCII; no stuck keys after blur/disconnect | `key`/`text` split, hidden-textarea IME capture, Safari keyCode 229 quirk, Cmd-held missing keyup quirk, release-all exempt from gates, dead-man (Patterns 3, 4, 6; Pitfalls 1-5) |
| DCTL-03 | Send-keys menu (Cmd+Tab, Cmd+Space, Lock Screen), optional Ctrl→Cmd swap | Chords as ordinary key messages; HID-tap posting evidence; window-share and lock-gate interplay; swap computed at keydown (Risk R8, R9; Pitfall 6) |
| DCTL-05 | Window share: raise when controlled, clicks confined | Layered raise (cooperative-activation caveat), AX focused-app check instead of NSWorkspace, frame refresh at button-down, optional topmost-window hit test (Risk R1, R2; Pattern 7) |
| DCTL-06 | Explicit-only clipboard; concealed never synced | Type check before read, `org.nspasteboard.*` strings, accessBehavior measurement, chunk size vs read limit, Safari ClipboardItem promise (Pattern 8; Risk R4, R14) |
| DESK-05 | Sharer picks whole display or single window | SCContentFilter display vs desktopIndependentWindow; window list filtering; popups not captured in window mode (Pattern 9; Risk R10) |
| DESK-06 | Switch displays mid-session without reconnect | `updateConfiguration` then `updateContentFilter` on the running stream (Apple sample), VT session recreate, epoch, in-band resolution change UNMEASURED (Risk R5) |
</phase_requirements>

## Summary

Phase 20 is almost entirely system-framework work behind decisions the user already accepted: CGEvent posting from the in-daemon capture host (Accessibility attribution proven GO in R08-B/C), NSPasteboard in a launchd agent, and `SCStream` filter updates. No new library is needed (CONTEXT; pion v4.2.29 is a Phase 19 dependency and is not added here). The locked wire protocol is sound, and every one of its hard parts has a known shape. The research found no finding that invalidates a locked decision outright, but it found **fifteen places where the plan as written will misbehave or rests on an unmeasured assumption**, listed in the next section. The most consequential: `NSRunningApplication activate` is a request that macOS 14+ may refuse from a background process (so the window-share raise needs a layered, verified, fail-closed design), "warping" the pointer generates no events, the 32 KiB clipboard chunk cannot fit under a 32768-byte WebSocket read limit once wrapped in JSON, and Mac browsers drop `keyup` for any key pressed while Cmd is held (a stuck-key source the daemon-side dead-man only papers over).

Two things Phase 17 did **not** measure are on the critical path for DESK-06: whether browsers (Chrome, Safari, Wails WKWebView, iPhone) decode an in-band resolution change at an IDR with no renegotiation, and whether `SCStream` delivers a first frame after `updateContentFilter` onto a *still* surface (R22 shows still screens deliver nothing). Both need VM rows early; the plan should put them in Wave 0 before the switch UI is built. Phase 19 is designed but not built, so several seams Phase 20 depends on (listed below) are not yet settled.

**Primary recommendation:** Build the daemon input path as a small, fully fake-poster-testable core in `internal/desktop` (gates, held sets, mapping, epoch, rate cap, chunking) with a thin ObjC poster in `internal/screencap`; make **releases exempt from every gate**; make the window raise a layered, verified, fail-closed step; and front-load three VM measurement rows (resolution change in-band across engines, first frame after filter switch on a still surface, pasteboard alert and types-read behaviour from a launchd agent).

## RISKS AND CONTRADICTIONS WITH 20-CONTEXT.md (for the orchestrator)

Severity: **H** = plan as written fails or is unsafe; **M** = plan works with a specified change; **L** = note. "Tag" is the provenance of the underlying fact.

| # | Sev | CONTEXT decision touched | Finding | Recommended resolution | Tag |
|---|-----|--------------------------|---------|------------------------|-----|
| R1 | H | [Window] "NSRunningApplication activate plus AXRaise" | Since macOS 14, activation is cooperative: Apple says it is "a request instead of a command" and the system "dynamically determines whether to grant". A launchd daemon is never the active app, so it cannot yield, and a developer thread reports the call "silently fails" from a non-frontmost process. Apple's doc: "NSWorkspace automatically handles this for you when opening URLs or applications." | Raise as a ladder, each rung verified by reading focus back: (1) `AXUIElementSetAttributeValue(app, kAXFrontmostAttribute, true)` plus `kAXRaiseAction` on the window and `kAXMainAttribute` true, (2) `NSWorkspace openApplicationAtURL:configuration:` with `activates = YES` on the running app's `bundleURL` (the LaunchServices path `open -a` uses), (3) `NSRunningApplication activate`. If focus cannot be confirmed within ~300 ms, input stays dropped (fail closed) and viewers get a "Could not bring the window to the front" state. Add a VM row proving rung 1 or 2 works from the launchd daemon. | [CITED: developer.apple.com/documentation/appkit/passing-control-from-one-app-to-another-with-cooperative-activation] rungs 1-2 efficacy: [ASSUMED] |
| R2 | H | [Window] frontmost check | `NSWorkspace.frontmostApplication` and `NSRunningApplication` values do not refresh without a main run-loop pass; the Go daemon has no AppKit run loop. A stale "frontmost" would let keys through to the wrong app (the exact PITFALLS 11 hazard). | Check focus live over AX: `AXUIElementCopyAttributeValue(AXUIElementCreateSystemWide(), kAXFocusedApplicationAttribute)`, compare pid, then focused/main window. Do not use NSWorkspace for the gate. | [CITED: Apple docs via search: NSRunningApplication does not update outside a run-loop pass]; AX live read [ASSUMED] |
| R3 | H | [Posting] "moves warped to the mapped point" | `CGWarpMouseCursorPosition` moves the cursor "without generating events" (SDK header). Apps get no mouseMoved, so hover, tooltips, drag tracking and window-server hit-testing break. | Post `kCGEventMouseMoved` (or the `*Dragged` type) with `CGEventPost` at the HID tap, as the spike's `spk_mouse_nudge` does; also set `kCGMouseEventDeltaX/Y`. Never use warp for normal moves. | [VERIFIED: MacOSX27.2.sdk CoreGraphics CGRemoteOperation.h:219-221 "Move the mouse cursor to the desired position in global display coordinates without generating events."] |
| R4 | H | [Limits] clipboard "chunks of at most 32 KiB to keep the HARD-02 WebSocket read limit" | The share server keeps coder/websocket's 32768-byte read limit ("its input DoS guard; senders split around it"). A chunk of 32 KiB of text wrapped in a JSON envelope (and JSON escaping of quotes, newlines, control characters) exceeds 32768 bytes and the socket closes. | Cap by **serialized frame length**, not text length: Paste-to-Mac chunks of at most 16 KiB of text (worst-case JSON escaping stays under 32768 for typical text, and the sender measures the encoded frame and splits again if over 30000). Or `conn.SetReadLimit` on the /desktop/ws conn only, leaving the pinned terminal-ws limit alone. The Copy-from-Mac direction is bounded by the browser, not by this limit. | [VERIFIED: internal/webserver/input_limit_test.go:14-15 "The share server keeps coder/websocket's 32768-byte default read limit as its input DoS guard; senders split around it (HARD-02)."] |
| R5 | H | [Surface] "SPS/PPS on every IDR lets a switch change resolution in-band with no renegotiation" | Sourced only from research ARCHITECTURE (marked LOW there). The Phase 17 matrix has no row for a mid-stream resolution change, and none for `updateContentFilter`. R22 shows a still screen delivers no frames, so after a switch onto an idle display or window SCK may deliver nothing and the viewer stays on the old surface's last frame. A VideoToolbox compression session also cannot change its dimensions in place (must be recreated). | Wave 0 VM rows, before UI work: (a) switch 2560x1440 to a 1280x800 window and back, decoded and rendered in Chrome, Safari, the Wails shell and iPhone Safari without renegotiation; (b) frames arrive after `updateContentFilter` onto a still surface within ~1 s. If (b) fails, the switch must also re-present the last frame for the new surface (cached frame plus forced IDR, as Phase 19 does for joins) or nudge SCK. If (a) fails in an engine, fall back to renegotiation for that engine only (the offer/answer path already exists). Drop frames whose dimensions differ from the active encoder session until both `updateConfiguration` and `updateContentFilter` completion handlers have returned. | Apple sample ordering [CITED: developer.apple.com/documentation/screencapturekit/capturing-screen-content-in-macos]; in-band decode [ASSUMED]; still-surface frame [ASSUMED] |
| R6 | M | [Keys] code-based keys; success criterion 2 "arrives as typed" | With `key` + `code` only, a viewer on a different layout than the Mac (Dvorak, AZERTY, QWERTZ) types different characters than they pressed, and a viewer with Caps Lock on types lowercase (browsers emit no reliable Caps Lock key events). | Keep the accepted design (it is the right base), but add to every `key` message from day one: `k` (the `KeyboardEvent.key` the viewer produced) and `caps` (`getModifierState('CapsLock')`) in the modifier snapshot, and have the daemon set the alpha-shift flag from `caps`. Both are additive under `input: 1`, free now, and costly to retrofit later. Add an M-NN row for a non-US viewer layout. Ask the user (Open Question 1) whether a layout-aware printable-key fallback is wanted; do not adopt one silently. | [VERIFIED: 20-CONTEXT "Keyboard input uses two message kinds"]; caps handling [ASSUMED] |
| R7 | M | [Stuck keys] "no key stays held" | On macOS, Chrome, Firefox (Bugzilla 1299553) and native apps get **no keyup for a non-modifier key pressed while Cmd is held**. For a Mac viewer every Cmd chord strands a key down in the viewer's own tracking; the daemon dead-man only clears it 3 s later. | Viewer rule: for a keydown with `metaKey` true on a non-modifier key, emit down then up as a tap (and for `repeat` keydowns, emit down and re-arm a ~120 ms synthetic up); on Meta keyup, emit up for every key pressed while Meta was down. Only forward a keyup whose down was forwarded (keep a per-code map). Safari behaviour unconfirmed; test all three. | [CITED: bugzilla.mozilla.org/show_bug.cgi?id=1299553]; Safari [ASSUMED] |
| R8 | M | [Lock] "daemon drops all pointer and key input" while locked; Send keys includes Lock Screen | (a) If the gate also drops key-**ups** and release-all posts, the Ctrl+Cmd+Q chord's own releases are dropped and modifiers stay down in the HID state at the lock screen. (b) A lock-state **poll** has a lag window in which a fast viewer's keystrokes (including a typed password) reach loginwindow before the gate closes; R16 proved posted keys do reach loginwindow. (c) After Lock Screen, no viewer can unlock until Phase 23, so a casual click on Send keys > Lock Screen locks the unattended office Mac until someone returns. | (a) Gates (lock, epoch, window, rate cap) apply to presses and moves only; **a release of something in the daemon's own held set always posts**. (b) Check lock state synchronously on every key/button press with a ~250 ms cache, and set `locked` optimistically the moment the daemon posts the Ctrl+Cmd+Q chord. (c) Keep Lock Screen but put it behind a confirm dialog ("The Mac stays locked until you unlock it at the Mac"), or hide it until Phase 23 (Open Question 2). | [VERIFIED: 17-SPIKE-MATRIX R16-A/R16-B rows: keys posted from the capture-host process reached loginwindow and unlocked the guest]; polling cost [ASSUMED] |
| R9 | M | [Window] frontmost gate vs [Send keys] chords | In a window share, Cmd+Tab and Cmd+Space move focus off the shared app, so the very next input is dropped and the window is re-raised, undoing the chord; the Lock Screen chord's first event may be consumed as the "raise" trigger. | In a window share, disable Cmd+Tab and Cmd+Space in the menu and hide Lock Screen (a window share must not grant more than a window). The gate stays unchanged. | [ASSUMED] (logical consequence of the locked gate) |
| R10 | L | [Window] "window list shows on-screen normal windows" | Apple's WWDC22 session says a single-window capture includes no child, pop-up or other windows of the app. Menus, popovers and popup buttons are therefore invisible to the viewer, and clicks on them are outside the window frame (dropped). Sheets are unconfirmed. | Add a one-line hint in the picker ("Menus and pop-ups do not appear in a window share; share the display instead"). Add a VM check for sheets and Open/Save panels. | [CITED: developer.apple.com/videos/play/wwdc2022/10155] (via search summary) |
| R11 | M | [Input] "daemon computes click count from timing and distance" | Computing double-click timing from **arrival** time on the daemon makes it jitter-dependent (59 ms median data-channel RTT over DERP in R13, more under load); two clicks 150 ms apart can arrive 40 or 300 ms apart and a double click fails or a slow click double-fires. | Add `ts` (viewer `performance.now()` ms) to button down/up messages and compute the interval from viewer timestamps and the distance from normalized coordinates scaled to points; compare to the system double-click interval (`NSEvent.doubleClickInterval` is run-loop free, or use a constant 500 ms [ASSUMED]). Still daemon-computed, so no viewer needs click count. | [VERIFIED: 20-CONTEXT:20 "the daemon computes click count ... from timing and distance"]; RTT [VERIFIED: 20-CONTEXT:119 "R13: data-channel RTT median 59 ms"] |
| R12 | M | [Packages] input on a pion data channel | pion calls `OnMessage` **synchronously inside the channel read loop** (`datachannel.go` readLoop calls `d.onMessage`), so a handler that blocks on a CGEvent post, an AX call or a full queue stalls all reads on that channel. The default max SCTP message size is 1073741823 bytes, so the 4 KiB cap is not enforced by the stack. | The handler only parses and does a non-blocking send to a bounded per-share queue; moves are latest-wins (replace the pending move), presses and releases are never dropped. Set `SettingEngine.SetSCTPMaxMessageSize` to a small bound (for example 8192) so an oversized message closes the channel (pion closes it on an over-limit message). | [VERIFIED: pion/webrtc v4.2.29 datachannel.go:409-428 readLoop calls `d.onMessage(DataChannelMessage{...})` inline; constants.go:28 `defaultMaxSCTPMessageSize = 1073741823`; datachannel.go:436-442 closes on an over-limit message] |
| R13 | M | [Window] AX raise on the input goroutine | AX calls to a hung or beachballing target app block until the messaging timeout; with "one input goroutine per share" every viewer's input freezes behind it. | `AXUIElementSetMessagingTimeout(systemWide, 0.25)` globally in the bridge, and run raise/focus work off the post path with a bounded result wait (input stays dropped while the raise is pending). | [VERIFIED: AXUIElement.h:387-398 AXUIElementSetMessagingTimeout "Sets the timeout value used in the accessibility API."; kAXErrorCannotComplete when an app does not return in time, AXUIElement.h:317-320] |
| R14 | M | [Clipboard] pasteboard alert measurement | Apple's `NSPasteboard.accessBehavior` (macOS 15.4+) reports whether a programmatic read is always allowed, never allowed or prompts, but current shipping state is unclear: one report says the feature is absent on macOS 26 unless enabled by defaults; another says the developer-preview default is deprecated and prompts were not seen on 27.2 beta 2. Whether reading **types** (needed for the concealed check) alerts is undocumented. | The VM row should record three things: `accessBehavior` value, whether `types` reads alert, and whether `stringForType:` alerts, each from the launchd daemon with nobody at the VM. Use `accessBehavior` at runtime where available to refuse a read that would prompt (return Waiting for approval) rather than provoke the alert. | [CITED: mjtsai.com/blog/2025/05/12/pasteboard-privacy-preview-in-macos-15-4] state on 27.2: [ASSUMED] |
| R15 | L | [Discretion] Chrome Keyboard Lock | Chrome's doc says that as of Chrome 130 requesting a keyboard lock also requires permission, and the API needs JS-initiated fullscreen and has a two-second Esc-hold exit. | Handle a rejected `navigator.keyboard.lock()` quietly (fullscreen still works); show a one-line hint. | [CITED: developer.chrome.com/docs/capabilities/web-apis/keyboard-lock] |

### Phase 19 design details Phase 20 depends on that are not settled (the code does not exist yet)

1. **Capture interface shape.** Phase 20 needs, beyond start/stop: `SelectSurface(display|window)`, an event for the actual encoded width/height per frame, and a synthetic source that can change size mid-test. 19-CONTEXT only promises "a synthetic frame source behind the capture interface". Phase 19's plan must include surface selection and a size-change path or Phase 20 reworks it.
2. **Encoder re-creation.** 19-CONTEXT fixes "VideoToolbox ... fixed target bitrate" and the main display only. A runtime encoder re-create (new dimensions, forced IDR, SPS/PPS emitted) is a Phase 20 addition to an encoder that Phase 19 may build as create-once.
3. **Offer shape.** Phase 19's viewer creates a non-trickle offer. The `input` data channel must be created on the viewer **before** `createOffer` (otherwise the SDP has no application m-line). The signalling hello versioning (`input: 1`) is a 19 message-shape detail not yet fixed.
4. **Per-viewer identity and rights struct** in `internal/desktop` (held sets, epoch last-seen, rate bucket live there) is not designed.
5. **Lock-state source**: 19 polls `CGSSessionScreenIsLocked`; Phase 20's gate wants a synchronous read with a short cache (R8). The state package must expose it as a function, not only as a pushed enum.
6. **Video layout rect in DesktopView.** Normalization is against the letterboxed video content rectangle, so DesktopView must expose the displayed video rect (and recompute on the `<video>` element's `resize` event and window resize).
7. **TESTING.md numbering.** The last recorded item is M-164 and Phase 19 will add its own rows; Phase 20's M-NN numbers must continue after Phase 19's last one (unknown today).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Capture pointer/key/IME/clipboard-intent events | Browser / Client | — | Only the viewer sees DOM events and the user's gestures |
| Normalize coordinates against the letterboxed video rect | Browser / Client | — | The viewer owns the layout; the daemon must never know viewer geometry |
| Surface (display/window) enumeration and selection | API / Backend (daemon, `internal/desktop` + `internal/screencap`) | Browser (picker UI) | SCK and AX are only reachable in-daemon (FEAS-02) |
| Coordinate mapping to global points, click counting, epoch, gates, held sets | API / Backend (daemon) | — | Security boundary: every check must live where the post happens |
| CGEvent posting, AX raise/focus, NSPasteboard | API / Backend (daemon native bridge) | — | TCC identity is the daemon's bundle ID |
| Input transport | Share server data channel `input` | /desktop/ws (low-rate, stateful) | Locked decision |
| Clipboard text transfer | /desktop/ws chunks | Browser clipboard APIs | Locked: explicit, never background |
| Lock/permission state | API / Backend | Browser (display only) | Failing closed must not depend on the viewer |
| Release-all dead-man | API / Backend | Browser (heartbeat, blur events) | The daemon must recover even when the viewer vanishes |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| CoreGraphics CGEvent (system) | macOS 14+ (SDK 27.2 read) | Mouse, scroll, keyboard posting | Apple's own header tells "remote control programs" to use a private event source; STACK.md already chose it |
| ScreenCaptureKit `SCStream` (system) | macOS 14+ | Display/window filter, live `updateContentFilter` | Apple sample applies `updateConfiguration` then `updateContentFilter` to a running stream |
| ApplicationServices AX (system) | macOS 14+ | Focus check, raise, window frame/main-window | Accessibility is already granted for CGEvent (R08) |
| AppKit NSPasteboard / NSWorkspace (system) | macOS 14+ | Clipboard text, app activation fallback | No dependency |
| pion/webrtc v4 | v4.2.29 (Phase 19 adds it; not added here) | `input` data channel | [VERIFIED: spike/desktop/go.mod "github.com/pion/webrtc/v4 v4.2.29"; module cache v4@v4.2.29] |
| coder/websocket | v1.8.15 (existing) | /desktop/ws signalling and clipboard chunks | [VERIFIED: go.mod:14 "github.com/coder/websocket v1.8.15"] |
| Browser: KeyboardEvent, Composition/`beforeinput`, Pointer Events, Clipboard API, Fullscreen, Keyboard Lock | n/a | Viewer capture | Vanilla, per CONTEXT |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| vitest | ^4.1.0 (existing) | Frontend unit tests for `desktopInput.ts` | Closed-table, release-all, IME/Cmd quirks logic [VERIFIED: frontend/package.json devDependencies "vitest": "^4.1.0"] |
| Go `testing` + fake poster seam | existing | Gates, mapping, epoch, limits | Per CONTEXT [Testing] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Private `CGEventSource` for posting | HIDSystemState/CombinedSession source | Apple header recommends `kCGEventSourceStatePrivate` for "remote control programs"; the other states merge physical modifier state, so a physical key held at the Mac leaks flags into remote events. Always also set flags explicitly per event. Phase 22 idle detection via `CGEventSourceSecondsSinceLastEventType` may or may not count private-source events (unknown, flag for Phase 22). |
| robotgo / go-vgo | none | Rejected by STACK.md ("a few hundred lines of ObjC beat a cgo dependency tree"); CONTEXT [Dependencies] None |

**Installation:** none. No package is added by this phase (the frontend and Go modules are unchanged).

**Version verification:** pion version confirmed from `spike/desktop/go.mod` and the local module cache directory `~/go/pkg/mod/github.com/pion/webrtc/v4@v4.2.29`; the root module does not contain pion yet (Phase 19 adds it). SDK headers were read from `/Library/Developer/CommandLineTools/SDKs/MacOSX27.2.sdk` (macOS 27.2.1 host, `sw_vers`).

## Package Legitimacy Audit

No external package is installed by this phase. pion/webrtc v4.2.29 and its companions are a Phase 19 addition that was vetted in the Phase 17 spike (`spike/desktop/go.mod`); they are not re-recommended here, so no `gsd-tools package-legitimacy check` run was needed.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none new) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 VIEWER (browser: DesktopView.tsx + lib/desktopInput.ts)
   DOM events ─► hidden <textarea> focus target (keydown/keyup, compositionend, beforeinput, paste guard)
        │             │ key{code,down,repeat,mods,caps,k,ts}      text{s}
        │             ▼
        │      per-code "forwarded down" map, Cmd-chord tap rule, Ctrl↔Cmd swap at keydown,
        │      pointer: normalize to letterboxed video rect → {x,y,epoch}, 1 move/rAF, bufferedAmount guard,
        │      blur / hidden / pagehide ─► release  ;  held ─► heartbeat 1 s
        ▼
  WebRTC data channel `input` (reliable, ordered, JSON ≤ 4 KiB)          /desktop/ws (WSS, JSON, 32768-byte read limit)
        │                                                                    │ select, surfaces, epoch, state, clip chunks
        ▼                                                                    ▼
 DAEMON internal/desktop ───────────────────────────────────────────────────────────────
   OnMessage: parse + non-blocking enqueue (moves latest-wins)   ◄── never block pion's read loop
   per-viewer: held keys/buttons, last epoch, rate bucket, heartbeat deadline (dead-man 3 s)
   gates on PRESS/MOVE only:  epoch ▸ lock (sync, cached ≤250 ms) ▸ window (AX focus live, raise ladder, clamp, topmost-hit) ▸ rate
   RELEASES of held items always pass (release-all on blur/close/failure/revoke/switch/lock)
        │ one input goroutine per share (serialises posts)
        ▼
 internal/screencap ObjC bridge (@try around calls)
   input_darwin.m: private CGEventSource, userData marker, CGEventPost(kCGHIDEventTap)
                   AX: focused app/window, raise ladder, frame, messaging timeout
   pasteboard_darwin.m: types check ▸ skip Concealed/Transient ▸ read utf8-plain-text ; write string
   SCStream: updateConfiguration ▸ updateContentFilter ▸ VT session recreate ▸ forced IDR ▸ epoch++
        ▼
   macOS HID/WindowServer ─► focused app          SCK frames ─► encoder ─► shared track ─► every viewer
```

### Recommended Project Structure
```
internal/screencap/
├── input_darwin.m        # CGEvent source+post, AX focus/raise/frame, key/mouse/scroll/text posting
├── input_darwin.go       # cgo shim, exported Go API, @try wrappers
├── pasteboard_darwin.m   # types check, read text, write string
├── input_stub.go         # non-darwin / testing stubs
└── keytable.go           # W3C code -> kVK (closed table), generated/asserted against Events.h list
internal/desktop/
├── input.go              # message types, per-viewer state, gates, held sets, rate bucket, dead-man
├── input_map.go          # normalized -> global points (display / window), click counting
├── input_queue.go        # bounded queue, latest-wins moves
├── surface.go            # select, epoch, fallback-to-main-display, window list filter
├── clipboard.go          # chunking, concealed result, size cap, state
└── *_test.go             # fake poster, fake AX, fake pasteboard
frontend/src/lib/
├── desktopInput.ts       # capture, IME, Cmd-chord rule, swap, release-all, heartbeat, coalescing
├── desktopKeymap.ts      # (optional) closed client-side code allow-list shared by tests
└── desktopClient.ts      # (Phase 19) gains data channel + surface/clipboard messages
```

### Pattern 1: Pointer mapping in points (no backing scale in the post path)
**What:** CGEvent mouse locations are global display coordinates **in points**, top-left origin of the main display, negative for displays left/above. `SCDisplay.width/height` are "in points". So `global = origin + norm * size_in_points` for a display and `window.origin + norm * window.size` for a window. Backing scale only matters for capture size, not for posting. Rebuild on display reconfiguration; round to the nearest point; clamp to the surface.
**When to use:** every pointer message after the epoch check.
**Source:** [CITED: developer.apple.com/documentation/screencapturekit/scdisplay: "The width of the display in points."]; "global display coordinates" [VERIFIED: CGRemoteOperation.h:219-220]; top-left origin and negative origins [ASSUMED, matches STACK.md and R08 spike behaviour on the main display].
```objc
// Skeleton only; constants come from the cited headers.
CGPoint p = CGPointMake(surf.origin.x + nx * surf.size.width,
                        surf.origin.y + ny * surf.size.height);
CGEventRef ev = CGEventCreateMouseEvent(src, type, p, button);   // type: Moved / LeftDragged / ...
CGEventSetIntegerValueField(ev, kCGMouseEventDeltaX, dx);        // deltas help apps that read them
CGEventSetIntegerValueField(ev, kCGEventSourceUserData, kPorttyMarker); // marker value is Portty's own constant
CGEventPost(kCGHIDEventTap, ev);
```
The spike posted `CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, CGPointMake(p.x + 1, p.y), kCGMouseButtonLeft)` with `CGEventPost(kCGHIDEventTap, move)` and it moved the cursor on the VM [VERIFIED: spike/desktop/sys_darwin.m:67-69]. Viewer side: normalize against the **displayed video content rectangle** (the letterbox bars are not part of the surface); clicks in the bars are clamped or ignored, and the rect must be recomputed on `<video>` `resize` and window resize. After a size or aspect change the output dimensions must keep the surface aspect (round to even for H.264), otherwise SCK letterboxes inside the frame and normalization is off by the padding; use the frame's `contentRect` attachment to verify ([CITED: Apple capture sample "Retrieve the content rectangle, scale, and scale factor"]).

### Pattern 2: Buttons, drag, scroll
**What:** Button down/up use the matching event types with `kCGMouseEventButtonNumber`; while a button is held, moves are posted as the `*Dragged` type. Map browser buttons (0 left, 1 middle, 2 right, 3 back, 4 forward) to CG buttons (left 0, right 1, other 2+). Click count goes in `kCGMouseEventClickState` ("1 single, 2 double, 3 triple") [VERIFIED: CGEventTypes.h:145-148]. Scroll: `CGEventCreateScrollWheelEvent2(src, kCGScrollEventUnitPixel, 2, wheel1, wheel2, 0)` where pixel units "will produce an event that most applications interpret as a smooth scrolling event" [VERIFIED: CGEvent.h comment on `CGEventCreateScrollWheelEvent`]; also set `kCGScrollWheelEventIsContinuous` [VERIFIED present: CGEventTypes.h:374 `kCGScrollWheelEventIsContinuous = 88`]. **Sign:** browser `deltaY > 0` means scroll down; CG wheel1 positive means scroll up, so negate (and likewise `deltaX`). Whether the Mac's "natural scrolling" re-inverts synthetic HID-tap events is unmeasured: add a VM row, and do not compensate until measured. Convert line/page deltas to pixels in the viewer (`deltaMode`), clamp each message.
**Tag:** sign convention and natural-scroll behaviour [ASSUMED]; rest verified as cited.

### Pattern 3: Keyboard — key path
**What:** Daemon maps `KeyboardEvent.code` through a closed table to kVK; unknown codes are dropped (matches `ctrlMap.ts` precedent: "a key with no entry is dropped or sent as typed, never guessed"). Modifiers are posted as their own key events with flags set explicitly per event from the held set (CreateKeyboardEvent docs: "All keystrokes needed to generate a character must be entered, including SHIFT, CONTROL, OPTION, and COMMAND keys" [VERIFIED: CGEvent.h]). Autorepeat keydowns set `kCGKeyboardEventAutorepeat` [VERIFIED present: CGEventTypes.h:178 `kCGKeyboardEventAutorepeat = 8`]. Use a **private** event source: Apple's header says "Very specialized applications such as remote control programs may want to generate and track event source state independent of other processes. These programs should use the `kCGEventSourceStatePrivate` value" [VERIFIED: CGEventSource.h:41-46].
**Verified kVK values** (quoted verbatim from `Events.h` in the 27.2 SDK; the table must be built from these, including ISO/JIS extras):
`kVK_ANSI_A = 0x00; kVK_ANSI_S = 0x01; kVK_ANSI_D = 0x02; kVK_ANSI_F = 0x03; kVK_ANSI_H = 0x04; kVK_ANSI_G = 0x05; kVK_ANSI_Z = 0x06; kVK_ANSI_X = 0x07; kVK_ANSI_C = 0x08; kVK_ANSI_V = 0x09; kVK_ANSI_B = 0x0B; kVK_ANSI_Q = 0x0C; kVK_ANSI_W = 0x0D; kVK_ANSI_E = 0x0E; kVK_ANSI_R = 0x0F; kVK_ANSI_Y = 0x10; kVK_ANSI_T = 0x11; kVK_ANSI_1 = 0x12; kVK_ANSI_2 = 0x13; kVK_ANSI_3 = 0x14; kVK_ANSI_4 = 0x15; kVK_ANSI_6 = 0x16; kVK_ANSI_5 = 0x17; kVK_ANSI_Equal = 0x18; kVK_ANSI_9 = 0x19; kVK_ANSI_7 = 0x1A; kVK_ANSI_Minus = 0x1B; kVK_ANSI_8 = 0x1C; kVK_ANSI_0 = 0x1D; kVK_ANSI_RightBracket = 0x1E; kVK_ANSI_O = 0x1F; kVK_ANSI_U = 0x20; kVK_ANSI_LeftBracket = 0x21; kVK_ANSI_I = 0x22; kVK_ANSI_P = 0x23; kVK_ANSI_L = 0x25; kVK_ANSI_J = 0x26; kVK_ANSI_Quote = 0x27; kVK_ANSI_K = 0x28; kVK_ANSI_Semicolon = 0x29; kVK_ANSI_Backslash = 0x2A; kVK_ANSI_Comma = 0x2B; kVK_ANSI_Slash = 0x2C; kVK_ANSI_N = 0x2D; kVK_ANSI_M = 0x2E; kVK_ANSI_Period = 0x2F; kVK_ANSI_Grave = 0x32; kVK_Return = 0x24; kVK_Tab = 0x30; kVK_Space = 0x31; kVK_Delete = 0x33; kVK_Escape = 0x35; kVK_RightCommand = 0x36; kVK_Command = 0x37; kVK_Shift = 0x38; kVK_CapsLock = 0x39; kVK_Option = 0x3A; kVK_Control = 0x3B; kVK_RightShift = 0x3C; kVK_RightOption = 0x3D; kVK_RightControl = 0x3E; kVK_Function = 0x3F; kVK_Help = 0x72; kVK_Home = 0x73; kVK_PageUp = 0x74; kVK_ForwardDelete = 0x75; kVK_End = 0x77; kVK_PageDown = 0x79; kVK_LeftArrow = 0x7B; kVK_RightArrow = 0x7C; kVK_DownArrow = 0x7D; kVK_UpArrow = 0x7E; kVK_ISO_Section = 0x0A; kVK_JIS_Yen = 0x5D; kVK_JIS_Underscore = 0x5E; kVK_JIS_Eisu = 0x66; kVK_JIS_Kana = 0x68;` plus keypad (`kVK_ANSI_Keypad0 = 0x52` .. `kVK_ANSI_Keypad9 = 0x5C`, `kVK_ANSI_KeypadEnter = 0x4C`) and `kVK_F1 = 0x7A .. kVK_F20 = 0x5A` as in the same header [VERIFIED: MacOSX27.2.sdk HIToolbox Events.h:205-328]. The W3C side (`KeyA`, `Digit1`, `IntlBackslash`, `IntlYen`, `IntlRo`, `Lang1`, `Lang2`, `NumpadEnter`, ...) is the UI Events code list and must be written out by hand; add a test that asserts every kVK in the table is unique and every viewer-allowed code resolves.
**Not representable from a browser:** the Fn/Globe key, media keys. Drop; document.
**Tag:** kVK values VERIFIED as quoted; W3C-code spellings [ASSUMED from the UI Events spec].

### Pattern 4: Keyboard — text path (IME, emoji, dictation, dead keys)
**What:** `CGEventCreateKeyboardEvent(src, 0, down)` plus `CGEventKeyboardSetUnicodeString(ev, n, utf16)` for both down and up, with flags explicitly set to 0 (so no stale modifier state leaks), then post. Apple's header warns "application frameworks may ignore the Unicode string in a keyboard event and do their own translation based on the virtual keycode" [VERIFIED: CGEvent.h:197-203], so the text path is for composed text only, never for shortcuts; bulk text goes through the explicit Paste to Mac. **Chunk to at most 20 UTF-16 units per event** without splitting surrogate pairs or ZWJ sequences (the 20 limit is long-standing practice but not stated in the SDK header: [ASSUMED]; a VM test with a 1 KiB emoji/accent string must pass). Daemon enforces the 1 KiB cap and validates UTF-8.
**Viewer capture:** keyboard events must land on a focused editable element for IME, emoji picker and dictation to work. Use a visually hidden `<textarea autocapitalize=off autocorrect=off spellcheck=false>` focused on click of the video; send `text` from `compositionend` (and from `beforeinput` for `insertText` that did not come from a forwarded `key`), then clear the textarea. While `event.isComposing || event.keyCode === 229 || event.key === 'Dead'`, suppress `key` forwarding: Safari fires `compositionend` **before** the committing keydown, so `isComposing` is already false on it and only `keyCode === 229` is reliable [CITED: bugs.webkit.org/show_bug.cgi?id=165004; developer.chrome.com/docs/modern-web-guidance/guides/forms/ime-safe-enter-submit]. Cancel the textarea's paste insertion (the repo's own lesson: `guardPasteInsert` in `frontend/src/lib/pasteGuard.ts`, "the browser then inserts the whole paste into that textarea ... 20 MB: ~100% CPU").
**Dead-key double-typing:** if a dead key (Option+E then E) is forwarded as `key` events **and** later arrives as composed `text`, the character is typed twice. Rule: once a composition starts, suppress `key` for the keys in it; forward only the `text`.

### Pattern 5: Click counting from viewer timestamps (R11)
Viewer adds `ts` to button messages; the daemon keeps `(lastDownTs, lastPoint, count)` per viewer and increments when `Δts <= interval` and the point distance (in points) is within a small radius, else resets to 1. Count never exceeds 3 for the posted event field. A pure function, table-testable with jittered arrival times.

### Pattern 6: Held state, release-all, dead-man, and which gates apply to releases
**What:** Per viewer: `heldKeys map[vk]struct{}`, `heldButtons`, `lastEpoch`, `deadline`. `releaseAll()` posts key-ups (modifiers last) and button-ups for the held set at the last mapped point (or current cursor location), then clears the set. Triggers per CONTEXT plus: surface switch, lock, revoke, `OnClose`, `PeerConnectionStateFailed`. **Releases bypass the lock gate, epoch gate, window gate and rate cap**; only presses and moves are gated. A button-up whose point falls outside a window share is posted at the clamped point, never dropped (a dropped up leaves the button stuck). The dead-man timer is armed whenever the held set is non-empty and re-armed by any message from that viewer; 3 s of silence runs `releaseAll`. Move gating drops only moves.
**Counters only** are logged (CONTEXT [Logging]): events by kind, drops by reason.

### Pattern 7: Window share — focus ladder, frame freshness, hit testing
**What:** Before posting a press in a window share: (1) read focus live over AX (R2); if the shared app/window is not frontmost+main, run the raise ladder (R1) off the post path, drop the triggering event, and keep dropping until the check passes or ~300 ms elapse (then surface the failure state); (2) fetch the window frame fresh at every button-down and key-focus check, and poll it ~5 Hz for moves (`CGWindowListCreateDescriptionFromArray` on the single window ID is cheap [ASSUMED]; matching the AX window to the SCWindow uses frame within a ±2 pt tolerance first and title only as a tiebreak, because terminal titles change constantly and the private `_AXUIElementGetWindow` is excluded by CONTEXT); (3) clamp moves to the frame, drop presses outside it; (4) **recommended addition, not in CONTEXT:** on a press, verify the topmost on-screen window at the target point belongs to the shared window (`CGWindowListCopyWindowInfo` with on-screen ordering), otherwise a floating panel, notification banner or other app sitting on top of the shared window's rectangle receives the click ("clicks never land outside it" holds only for the frame, not for what overlaps it). Needs Screen Recording for owner names, already granted. Unminimize (`kAXMinimizedAttribute` false) in the raise path. Window close or display loss falls back to the main display per CONTEXT (`SCStream` delegate `didStopWithError` plus a 1 s poll of the shareable list; do not rely on `CGDisplayRegisterReconfigurationCallback`, which needs a run loop on the registering thread).
**Tag:** AX/CG details [ASSUMED] until a VM row; fail-closed behaviour [VERIFIED: 20-CONTEXT decision].

### Pattern 8: Clipboard
**Copy from Mac (explicit):** read the item **types first**; if any of `org.nspasteboard.ConcealedType` or `org.nspasteboard.TransientType` is present return `concealed` (no content read); otherwise read `public.utf8-plain-text` only, cap at 1 MiB, send in chunks (R4). `org.nspasteboard.AutoGeneratedType` marks content the user did not copy; the nspasteboard.org guidance says it "may still be shown as the current clipboard content if it isn't also marked transient", so it is not skipped by CONTEXT's rule [CITED: nspasteboard.org; the three strings quoted verbatim from nspasteboard.org: `org.nspasteboard.TransientType`, `org.nspasteboard.ConcealedType`, `org.nspasteboard.AutoGeneratedType`]. Also skip `com.agilebits.onepassword` (1Password's proprietary confidential type, listed on nspasteboard.org) as defence in depth [CITED: nspasteboard.org]. Never poll `changeCount`; there is no background path. Use `accessBehavior` and the measured alert behaviour (R14). A read that would prompt with nobody there returns Waiting for approval, never hangs (bound the read with a timeout on a separate goroutine).
**Paste to Mac (explicit):** `clearContents` then `setString:forType:NSPasteboardTypeString`. Writes are not subject to the read alert [ASSUMED]. Size cap 1 MiB; reassemble chunks with a total-length and sequence check; discard a partial transfer on disconnect.
**Viewer, Copy side:** the round trip to the daemon puts an `await` between the click and `writeText`. Safari rejects clipboard writes outside a user-gesture stack ("after an async call, the call to the write() method is not seen as part of a user gesture anymore") [CITED: bugs.webkit.org/show_bug.cgi?id=244812]. Call `navigator.clipboard.write([new ClipboardItem({'text/plain': promiseOfBlob})])` **synchronously in the click handler** with a promise that resolves when the daemon answers; keep `writeText` for Chrome and the selectable-text fallback box CONTEXT already specifies. **Paste side:** `navigator.clipboard.readText()` prompts in Chrome and shows a Paste bubble in Safari; offer a small popover with a focused textarea as the fallback (the user pastes locally, which exposes `clipboardData` without a prompt) and then Send.

### Pattern 9: Surface list, select, and switch
**What:** `SCShareableContent.getExcludingDesktopWindows(false, onScreenWindowsOnly: true)` yields `SCDisplay`/`SCWindow`/apps [CITED: Apple capture sample]. Filter windows to `windowLayer == 0`, on screen, with an owning application, a non-trivial size, and a non-empty title or app name (an unfiltered list returns many "windows no ordinary user would consider a window", per Apple forum 756134 results). Display filter: `SCContentFilter(display:excludingApplications:exceptingWindows:)`; window filter: `SCContentFilter(desktopIndependentWindow:)` [CITED: Apple sample]. Switch on a **running** stream: build the new configuration (width/height computed from the new surface: points times scale for a display, `Int(window.frame.width) * 2` style for a window per the Apple sample, capped to the 2560 long edge) then `try await stream.updateConfiguration(cfg)` then `try await stream.updateContentFilter(filter)`; "further changes to its configuration and content filter don't require restarting it" [CITED: Apple sample]. Then recreate the VT session if dimensions changed, force an IDR with SPS/PPS, bump `epoch`, release all held input, push `{epoch, surface, w, h}` on /desktop/ws. Frames in flight between the two updates can have either size; gate on dimensions (R5). Display IDs: use `CGDirectDisplayID` plus a label (resolution and index; `NSScreen.localizedName` is run-loop sensitive, so label from CGDisplay APIs). Epoch bumps on surface change, window **resize**, display reconfiguration; **not** on window move (normalized coordinates are surface-relative and the frame is refreshed at post time).
**Race safety (by construction):** the viewer learns the new epoch over /desktop/ws while input flows on the data channel; any input sent with the old epoch is dropped by the daemon, so there is no window in which input lands on the wrong surface. Presses and moves drop; releases still post (Pattern 6).

### Anti-Patterns to Avoid
- **`CGWarpMouseCursorPosition` for moves:** no events (R3).
- **Reading `NSWorkspace.frontmostApplication` for the window gate:** stale without a run loop (R2).
- **Dropping key-ups under any gate:** strands modifiers (R8).
- **Blocking in `OnMessage`:** stalls the data channel (R12).
- **Letting `text` carry shortcuts:** frameworks ignore the Unicode string (Pattern 4).
- **Computing double-click from daemon arrival time:** jitter (R11).
- **Treating AXIsProcessTrusted / CGPreflightPostEventAccess as live:** cached per process (R08-A/B); measure the effect instead.
- **`or {}`-style silent fallbacks (CLAUDE.md):** an unknown code or malformed message is a counted drop, not a guess.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Key code table values | Hand-typed hex from memory | Table generated or asserted against `Events.h` lines quoted above | A wrong kVK types the wrong key on every layout |
| UTF-16 chunking | Byte slicing | `utf16.Encode` on runes plus a grapheme-safe splitter (or split on rune boundaries, never inside a surrogate pair or ZWJ sequence) | Splitting a pair types two replacement characters |
| Concealed-item detection | Content heuristics (regex for passwords) | The `org.nspasteboard.*` type check | Documented convention password managers set |
| Double-click detection | Per-app timing guesses | `kCGMouseEventClickState` set from viewer timestamps | The system and apps consume this field |
| Window/app activation | Synthetic title-bar clicks | AX frontmost + AXRaise, then NSWorkspace open | A synthetic click also acts on content |
| Bounded input queue and size caps | New primitives | The v1.2 `MaxInputPayload`/bounded-queue idiom (`internal/relay` `input.go`, `protocol.go:137-141`) | Existing, tested pattern |
| Clipboard permission prompt | Provoking and dismissing the alert | `accessBehavior` where present | Never trigger a prompt nobody can click |

**Key insight:** every hard part of this phase has a known failure mode that only shows up on a real Mac with real apps (stuck keys, layout, activation, Retina, IME). Spend the effort on the fake-poster core and on early VM rows, not on cleverness.

## Runtime State Inventory

Omitted: this is a greenfield feature phase, not a rename/refactor/migration. (`desktop.json` is unchanged per CONTEXT [Surface]; nothing is persisted by this phase.)

## Common Pitfalls

### Pitfall 1: Stuck modifiers after Cmd chords on a Mac viewer
**What goes wrong:** the key pressed with Cmd held never gets a `keyup` in the browser.
**Why:** macOS delivers no keyup for non-modifier keys while Cmd is down (Firefox bug 1299553; same pattern in native apps).
**How to avoid:** viewer tap rule and Meta-up flush (R7); daemon dead-man backstop.
**Warning signs:** after Cmd+C the remote shows `C` held, heartbeat never starts, release happens at the 3 s mark.

### Pitfall 2: Gates drop the releases
**What goes wrong:** Lock Screen chord's Ctrl/Cmd/Q ups are dropped by the new lock gate; modifiers stay down.
**How to avoid:** releases of held items bypass all gates (Pattern 6); test: hold Cmd, flip `locked`, assert the Cmd-up is posted.

### Pitfall 3: Lock poll lag lets a viewer type into the lock screen
**What goes wrong:** keys sent in the lag between the chord and the gate closing reach loginwindow (R16 proves they do).
**How to avoid:** synchronous cached lock check on presses plus optimistic lock on the chord (R8).

### Pitfall 4: Window raise does nothing and every input is dropped
**What goes wrong:** `activate` is ignored from a background daemon, so the frontmost check never passes and the window share is dead (safe, but broken).
**How to avoid:** the ladder, read-back verification, a visible failure state (R1); VM row.

### Pitfall 5: Resolution change shows a frozen or garbled picture
**What goes wrong:** new SPS not honoured by an engine, or no frame after the filter swap on a still surface.
**How to avoid:** Wave 0 measurements (R5); per-engine fallback to renegotiation.

### Pitfall 6: Ctrl↔Cmd swap strands a key
**What goes wrong:** the user toggles the swap while a key is down, so keyup maps to a different code than keydown.
**How to avoid:** resolve the mapped code at keydown and store it in the per-code map used for keyup; the toggle only affects subsequent downs. Implement the swap as an exchange of Control and Meta pairs so Ctrl chords for a remote terminal stay reachable via the Windows key (confirm in Open Question 3).

### Pitfall 7: Fullscreen/Keyboard Lock misreads
**What goes wrong:** Keyboard Lock needs JS-initiated fullscreen and (Chrome 130+) a permission, and Esc held 2 s exits. It never captures OS-level Cmd+Tab/Cmd+Space; the Send keys menu is the only path.
**How to avoid:** treat rejection as non-fatal (R15); do not promise Cmd+Tab capture.

### Pitfall 8: Data-channel backpressure and head-of-line blocking
**What goes wrong:** on a lossy path the reliable ordered channel delays everything, including key-ups; the viewer keeps queuing moves.
**How to avoid:** skip coalesced moves when `bufferedAmount` exceeds a threshold; never skip presses/releases; daemon side latest-wins moves; heartbeat tolerance of 3 s covers jitter.

### Pitfall 9: Accessibility detection moves the user's pointer
**What goes wrong:** a nudge test (as in the spike, which also sleeps 100 ms) both disturbs the pointer and blocks the input goroutine.
**How to avoid:** verify the effect of real posted moves after a ≥50 ms delay on a separate timer (compare `CGEventGetLocation(CGEventCreate(NULL))` to the target); if a nudge is needed, pick the direction away from the screen edge (the spike's own docs note a move can fail "at the right screen edge" [VERIFIED: spike/desktop/sys_darwin.m:60-61 "1 moved, 0 did not (blocked, or at the right screen edge)"]). Three consecutive misses push Control needs Accessibility.

### Pitfall 10: Local pointer dot and keyboard focus confusion
**What goes wrong:** keystrokes meant for the viewer's own Mac go to the remote Mac.
**How to avoid:** keyboard captured only while the video area has focus (CONTEXT); show a "Control active" cue; blur releases all.

## Code Examples

### Viewer: forwarded-down map and Cmd-chord tap rule (sketch)
```typescript
// Source: design from this research (Risks R7, Pitfall 6). Not copied from a library.
const forwarded = new Map<string, string>()   // physical code -> code actually sent (after Ctrl<->Cmd swap)
function onKeyDown(e: KeyboardEvent) {
  if (e.isComposing || e.keyCode === 229 || e.key === 'Dead') return   // IME owns it (Safari: keyCode 229)
  const sent = swapCtrlCmd ? swap(e.code) : e.code
  send({ t: 'key', code: sent, down: true, repeat: e.repeat, mods: snapshot(e), caps: e.getModifierState('CapsLock'), k: e.key, ts: performance.now() })
  forwarded.set(e.code, sent)
  if (e.metaKey && !isModifier(e.code)) scheduleSyntheticUp(e.code)   // Mac: no keyup arrives while Cmd is held
}
function onKeyUp(e: KeyboardEvent) {
  const sent = forwarded.get(e.code); if (!sent) return                 // only release what was pressed
  forwarded.delete(e.code)
  send({ t: 'key', code: sent, down: false, mods: snapshot(e), ts: performance.now() })
}
function releaseAll() { for (const [, sent] of forwarded) send({ t: 'key', code: sent, down: false }); forwarded.clear(); send({ t: 'release' }) }
```

### Daemon: handler stays non-blocking (pion)
```go
// Source: pion/webrtc v4.2.29 datachannel.go:409-428 (readLoop invokes onMessage inline).
dc.OnMessage(func(m webrtc.DataChannelMessage) {
	msg, ok := parseInput(m.Data)          // size already bounded by SetSCTPMaxMessageSize
	if !ok { counters.drop("malformed"); return }
	viewer.enqueue(msg)                    // non-blocking; moves replace the pending move, presses/releases never dropped
})
```

### Surface switch ordering (Apple sample, adapted)
```swift
// Source: developer.apple.com/documentation/screencapturekit/capturing-screen-content-in-macos
try await stream?.updateConfiguration(configuration)
try await stream?.updateContentFilter(filter)
// then: recreate the VT session if the size changed, force IDR (SPS/PPS), epoch += 1, release all held input
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `activate(ignoringOtherApps:)` | Cooperative activation: `yieldActivation(to:)` then `activate()`; activation is a request | macOS 14 | A background daemon cannot reliably raise another app via NSRunningApplication alone (R1) |
| Unrestricted `NSPasteboard` reads | `accessBehavior`, `detectedPatterns`, `detectedValues` (alert on programmatic reads) | macOS 15.4 (shipping state on 26/27 unclear) | Must measure on 27.2 from a launchd agent (R14) |
| Recreate `SCStream` to change target | `updateConfiguration` / `updateContentFilter` on a running stream | macOS 12.3+ | Display/window switch without restart |

**Deprecated/outdated:** `CGWarpMouseCursorPosition` for remote control moves (generates no events); `NSApplication activateIgnoringOtherApps` (deprecated, no longer reliable); trusting `AXIsProcessTrusted()` as a live value in a long-lived process (R08-A/B).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | AX `kAXFrontmostAttribute` / NSWorkspace-open rungs raise another app from the launchd daemon on macOS 27.2 | R1 | Window share cannot raise; stays fail-closed but unusable |
| A2 | AX system-wide focused-app read is live (no run loop needed) | R2 | Gate could be stale |
| A3 | HID-tap posting of Cmd+Tab and Cmd+Space triggers the system app switcher and Spotlight on 27.2 (old Cocoa-dev threads say HID tap works, annotated session tap beeps; dated) | DCTL-03 | Send keys items do nothing; need VM row, no fallback exists in CONTEXT |
| A4 | Ctrl+Cmd+Q posted as ordinary key events locks the screen (R15 locked the guest with the click tool's key events, which is strong but not the daemon's own post path) | DCTL-03 | Lock Screen item fails |
| A5 | Posted keys reach password fields even with Secure Event Input on (TN2150 only says event *taps* stop seeing keys; posting is not addressed; R16 shows posts reach loginwindow) | DCTL-02 | Typing into password fields drops |
| A6 | `CGEventKeyboardSetUnicodeString` limit of 20 UTF-16 units per event | Pattern 4 | Longer chunks silently truncated |
| A7 | Natural-scroll setting does not re-invert synthetic pixel scroll posted at the HID tap | Pattern 2 | Scroll direction wrong on natural-scroll Macs |
| A8 | Caps Lock handling via an alpha-shift flag yields uppercase in apps | R6 | Uppercase wrong when viewer caps differs |
| A9 | `kCGEventSourceUserData` survives to taps/listeners (Phase 22 use) | Packages | Phase 22 cannot tell remote from physical |
| A10 | NSPasteboard writes do not trigger the paste alert; read alert status on 27.2 | R14 | Paste to Mac prompts |
| A11 | In-band resolution change at an IDR decodes in Chrome, Safari, Wails WKWebView, iPhone | R5 | Switch needs renegotiation per engine |
| A12 | SCK emits a first frame after `updateContentFilter` onto a still surface | R5 | Viewer stuck on stale frame after a switch |
| A13 | Sheets are/aren't captured in a window share (WWDC22 only covers child/pop-up windows) | R10 | UX hint text wrong |
| A14 | Cheap single-window frame fetch via `CGWindowListCreateDescriptionFromArray` at 5 Hz | Pattern 7 | Slightly more CPU |
| A15 | System double-click interval constant (500 ms) acceptable | R11 | Double click feels off; use the system value |

## Open Questions (RESOLVED)

1. **Layout-aware typing for printable keys.** RESOLVED (planning, 2026-11-26): no layout fallback. Printable keys always go by `code`; every key message still carries `k` and `caps` (CONTEXT Claude's Discretion after research), so a fallback can be added later with no protocol change. Deciding artifact: 20-02-PLAN.md "Flagged decisions" (No layout fallback; rated reversible), wire fields frozen in 20-01-PLAN.md; a non-US viewer layout is an owner-run observation in 20-11-PLAN.md.
   - What we know: the accepted `key`+`code` design types the remote layout's characters; success criterion 2 says "arrives as typed".
   - What's unclear: whether the user's two Macs share a layout (probably US on both) and whether non-US viewers matter this milestone.
   - Recommendation: ship the accepted design with `k` and `caps` fields (R6); ask the user before adding a fallback that sends printable unmodified keys as `text`.
2. **Lock Screen in the Send-keys menu before Phase 23.** RESOLVED: keep it behind a confirm dialog (CONTEXT Claude's Discretion after research: "Lock Screen in the Send-keys menu sits behind a confirm dialog"), greyed in window shares (research R9; 20-UI-SPEC.md Decisions "Lock Screen" and "Window-share chords"). Deciding artifacts: 20-CONTEXT.md, 20-UI-SPEC.md Surface D; implemented in 20-07-PLAN.md (confirm) and 20-04-PLAN.md (daemon drops the chord in window shares).
   - What we know: the lock gate blocks every viewer after the lock; unlock is Phase 23; v2.0 ships after Phase 23.
   - What's unclear: whether a confirm dialog is enough or the item should be hidden until Phase 23 (the office Mac is unattended).
   - Recommendation: confirm dialog; hide in window shares.
3. **Ctrl→Cmd swap semantics.** Exchange (Ctrl↔Cmd) versus one-way. Recommendation: exchange (keeps Ctrl chords for remote terminals reachable). RESOLVED: exchange (CONTEXT Claude's Discretion after research: "the Ctrl to Cmd option swaps the two keys"); implemented in 20-06-PLAN.md (mapped code fixed at keydown).
4. **Topmost-window hit test on window shares (R-Pattern 7 item 4).** Not in CONTEXT; adds one CGWindowList call per press. Recommendation: include; ask only if the user wants it left out. RESOLVED: included (CONTEXT Claude's Discretion after research: "every click first checks which window is on top at that point"); implemented in 20-04-PLAN.md Task 3 (drop reason `covered`).
5. **Where Phase 19 leaves the capture interface** (see "Phase 19 design details"): planner should read Phase 19's plans once they exist and reconcile. RESOLVED (planning, 2026-11-26, reconciled against merged code at 44670565: 19-01..19-11 and 19-17):
   - Item 1 capture interface: `screencap.Source.SelectSurface` exists and is documented as the Phase 20 hook; Phase 20 adds `SurfaceInfo`, `ListSurfaces` (Deps seam) and the live switch (20-03-PLAN.md).
   - Item 2 encoder re-creation: `pc_encoder_rebuild` and `DisplaySource.Rebuild` exist (19-05); per-unit `AccessUnit.Width/Height` already drives the `size` message; 20-03 adds `pc_stream_select` with `updateContentFilter` and a still-frame fallback.
   - Item 3 offer shape: the viewer builds its offer after the server hello (19-06 `handle`), so 20-06 creates the `input` channel before `createOffer` when the server hello carries `caps.input`; `Deps.OnDataChannel` is registered before the offer is applied (19-03 peer.go) and SCTP max message size is 8192.
   - Item 4 per-viewer identity and rights: `Viewer{ID, Rights, JoinedAt}` with `RightsViewOnly` exists (19-03); 20-01 adds the rights bits and per-viewer input state.
   - Item 5 lock-state source: `Manager.Locked()` (250 ms wall-clock cache, true on a read error) exists (19-09); 20-01 and 20-02 call it on every press.
   - Item 6 video rect: `videoContentRect` and `onVideoRect` exist in DesktopView.tsx (19-06); 20-06 normalizes against them.
   - Item 7 TESTING.md numbering: computed at run time after Phase 19's Category BJ (20-11-PLAN.md).
   - 19-10 (handoff closes desktop viewers with CloseAll then Close) and 19-11 (perm-probe, permissions endpoint, setup sheet) merged during planning and are cited as merged in 20-01, 20-09 and 20-10.
   - Still-unmerged Phase 19 plans Phase 20 depends on, each enforced by a `<precondition>` in the dependent plan: 19-12 (VM real-media proof, VM grants, `vmdesktop` tag guard; 20-09), 19-13 (Settings Desktop group, DesktopStatusResult; 20-10; also edits style.css and RegenerateKeyModal.tsx, reconciled append-only in 20-06..20-08), 19-14 (tray desktop line; 20-10), 19-15 (final VM matrix in the shared VMs; 20-09 waits for it), 19-16 (TESTING.md Category BJ; 20-11).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all Go work | ✓ | go1.27.4 darwin/arm64 (go.mod says go 1.26.4, toolchain go1.26.11) | — |
| Node / pnpm | frontend | ✓ | node v24.22.1, pnpm 12.6.0 | — |
| macOS SDK 27.2 headers | cgo ObjC | ✓ | `/Library/Developer/CommandLineTools/SDKs/MacOSX27.2.sdk` | — |
| Tart | VM rows (CGEvent, raise, pasteboard, resolution switch) | ✓ | 2.41.1 | VM `portty-test` presence not probed in this session |
| Real office Mac / second Mac | M-NN rows (Retina, external display, IME, clipboard) | not probed (user's machines) | macOS 27.2.x | VM rows cover everything except real Retina/external display and real IME |
| Playwright/Chrome (browser e2e) | CSP/e2e precedent exists in `internal/webserver/browser_csp_e2e_test.go` | not probed | — | unit + loopback tests |

**Missing dependencies with no fallback:** none found. **Host-safety note:** this research ran no capture, no input injection, touched no tailscaled and no 100.x address, and opened nothing under ~/Downloads or /Applications beyond reading SDK headers from `/Library/Developer/CommandLineTools`.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json` [VERIFIED: config.json "nyquist_validation": false]. Test approach is fixed by CONTEXT [Testing]. Planner notes carried from CLAUDE.md: update `TESTING.md` (Suite Manifest, new M-NN rows continuing after Phase 19's last item, traceability rows for every new test file covering a v4.0+ requirement), and run `bash tests/check-traceability-paths.sh` before committing [VERIFIED: tests/check-traceability-paths.sh exists]. Suggested Wave 0 VM rows (each a short measured probe, results into the plan summary): (1) in-band resolution change per engine, (2) first frame after `updateContentFilter` on a still surface, (3) raise ladder from the launchd daemon, (4) pasteboard `accessBehavior`, `types` read and `stringForType:` behaviour from a launchd agent, (5) Cmd+Tab, Cmd+Space, Ctrl+Cmd+Q posted by the daemon, (6) 1 KiB accent/emoji `text` typing, (7) scroll direction with natural scrolling on and off, (8) window-share sheets, Open/Save panels and popup menus.

## Security Domain

`security_enforcement` is not disabled in config (absent = enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (inherited) | Same-user WhoIs + desktop token gate from Phase 19; input never reachable without the established viewer connection |
| V3 Session Management | yes | Per-viewer held state, release on close/revoke; token carries no permission claims (rights are daemon runtime state) |
| V4 Access Control | yes | Lock gate, window confinement, epoch check, own-account-only in this phase; Phase 22 per-person grants build on this |
| V5 Input Validation | yes | Strict JSON schema, closed key table, size caps (4 KiB message, 1 KiB text, 1 MiB clipboard), rate cap, UTF-8 validation, chunk reassembly checks |
| V6 Cryptography | no | None added (DTLS/SRTP and TLS inherited) |
| V8 Data Protection | yes | Concealed/transient pasteboard items never read; no key, text or clipboard content logged |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Remote input into the lock screen / password prompt | Elevation of privilege | Fail-closed lock gate, synchronous check, optimistic lock on chord (R8) |
| Keystrokes escaping a window share into another app | Tampering / Info disclosure | AX live frontmost check, raise-then-drop, topmost-window hit test (R1, R2, Pattern 7) |
| Stale-surface input landing on the wrong display | Tampering | Epoch bump and drop; releases still post |
| Password-manager content exfiltrated via clipboard | Info disclosure | Types-first concealed/transient skip, explicit action only, no polling |
| Clipboard prompt hang on unattended Mac | Denial of service | `accessBehavior` gate, bounded read with timeout, Waiting for approval state |
| Input flood / oversized messages | DoS | pion `SetSCTPMaxMessageSize`, bounded queue, per-viewer rate cap, moves latest-wins |
| Logging secrets | Info disclosure | Counters and kinds only (CONTEXT) |
| Stuck key or button after disconnect | Tampering (availability of the Mac) | Release-all triggers plus dead-man; releases bypass gates |
| Cross-token confusion | Spoofing | Inherited DSHR-08 tests; add a test that a terminal session cap cannot open the `input` channel or clipboard messages |
| CSP regression | Tampering | No new directive; Phase 19's pinned /desktop policy test stays green (Fullscreen/Keyboard Lock/Clipboard APIs need no CSP change [ASSUMED]) |

## Project Constraints (from CLAUDE.md)

- Go: `go fmt`, `golangci-lint`, context-aware functions (`ctx context.Context`); TS: ESLint + Prettier, camelCase, PascalCase components; pnpm preferred; Python not used here (venv rule N/A).
- **Testing:** 80%+ coverage in critical components (the input core and gates qualify); integration tests for routes/services.
- **Silent fallbacks:** "`or {}` converts hard failures into silent corruption. Let it crash." Applied as counted, logged-by-kind drops, never guessed keys; Phase 19's "recover, never panic" crash containment still applies in the desktop goroutines.
- **Chesterton's Fence / Premature abstraction (3 real examples):** do not abstract the key table or gates beyond what this phase needs; Phase 22 replaces arbitration wholesale.
- **Regression standing rule (portty/CLAUDE.md):** update `TESTING.md` Sections 2, 4, 5 for every new test file and manual behaviour; run `bash tests/check-traceability-paths.sh`; re-apply branch protection if `build.yml` matrix changes.
- **Project (.claude/CLAUDE.md):** macOS only; keep Go/Wails/React/tsnet stack; tailnet-only, token-gated links and CSP intact; deletion discipline; MIT license metadata untouched; work through GSD commands.
- **Memory constraints:** reliability first (nobody can fix the remote Mac); radical simplicity (default to cutting features); GUI tests in the Tart VM, never the host; no live tailnet or host side effects in research.
- **This research:** wrote only this file; no commits.

## Sources

### Primary (HIGH confidence)
- macOS 27.2 SDK headers read this session: `CoreGraphics/Headers/CGEvent.h`, `CGEventTypes.h`, `CGEventSource.h`, `CGRemoteOperation.h`; `HIServices/Headers/AXUIElement.h`, `AXAttributeConstants.h`, `AXActionConstants.h`; `HIToolbox/Headers/Events.h`
- In-repo: `20-CONTEXT.md`, `19-CONTEXT.md`, `17-SPIKE-MATRIX.md` (R08, R15, R16, R17 rows), `.planning/research/{STACK,PITFALLS,ARCHITECTURE,FEATURES,SUMMARY}.md`, `spike/desktop/sys_darwin.m:55-92`, `internal/webserver/input_limit_test.go:14-15`, `frontend/src/lib/pasteGuard.ts`, `frontend/src/lib/ctrlMap.ts`, `go.mod`, `spike/desktop/go.mod`
- pion/webrtc v4.2.29 source in the module cache (`datachannel.go`, `constants.go`, `settingengine.go`)

### Secondary (MEDIUM confidence)
- developer.apple.com/documentation/appkit/passing-control-from-one-app-to-another-with-cooperative-activation (fetched)
- developer.apple.com/documentation/screencapturekit/capturing-screen-content-in-macos (fetched)
- developer.apple.com/documentation/screencapturekit/scdisplay, scwindow (fetched)
- developer.chrome.com/docs/capabilities/web-apis/keyboard-lock (fetched)
- nspasteboard.org (fetched; type strings quoted)
- mjtsai.com/blog/2025/05/12/pasteboard-privacy-preview-in-macos-15-4 (fetched; includes commenter reports about 26/27.2 state)

### Tertiary (LOW confidence, marked for validation)
- WebSearch summaries: HID-tap Cmd+Tab/Cmd+Space (Cocoa-dev 2008 threads), Safari keyCode 229 (bugs.webkit.org/show_bug.cgi?id=165004), Safari clipboard async gesture (bugs.webkit.org/show_bug.cgi?id=244812), Cmd-held missing keyup (bugzilla.mozilla.org/show_bug.cgi?id=1299553), WWDC22 10155 (window capture excludes child/pop-up windows), Apple forum 793253 (activation from non-frontmost app), Apple docs on NSRunningApplication run-loop staleness

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, all system frameworks plus already-chosen libraries; versions read from files.
- Architecture: MEDIUM-HIGH, matches locked decisions; fake-poster seam design is standard.
- Pitfalls: MEDIUM, several rest on web reports and unmeasured macOS 27.2 behaviour; each unmeasured item has a named VM row in the Validation note.
- Unmeasured on this OS (LOW until a VM row): R1 efficacy, R5 in-band resolution change and still-surface first frame, A3/A4 system chords from the daemon, A5 secure input, A7 natural scroll, A10 pasteboard.

**Research date:** 2026-11-25
**Valid until:** 2026-12-25 for the stack; re-check the pasteboard and activation findings against any macOS point release before the real-Mac rows run.
