# Phase 20: Remote Control - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

A viewer with control drives the shared Mac as if sitting at it. In scope: pointer move, click, scroll and drag landing on the right spot at any Retina scale and viewer window size (DCTL-01); typing with shortcuts and non-ASCII or input-method text, with no key or modifier left held after blur, close or disconnect (DCTL-02); a Send-keys menu for Cmd+Tab, Cmd+Space and Lock Screen, plus an optional Ctrl→Cmd swap (DCTL-03); window share with the window raised when controlled and clicks confined to it (DCTL-05); text clipboard both ways by explicit action only, with concealed items never synced (DCTL-06); whole-display or single-window choice (DESK-05); display switching mid-session without the viewer reconnecting (DESK-06). The viewers in scope are the Phase 19 browser viewer in desktop Chrome and Safari (the same page on iPhone, without new touch controls), and only own-account always-available viewers exist in this phase. Not in this phase: control arbitration between several controllers and physical-input-wins (DCTL-04, Phase 22); per-person control and clipboard grants, admit and guests (DSHR-03..07, Phase 22); the Portty app Desktop tab, Cmd+W/Q/C forwarding in the app, and iPhone trackpad, touch and soft-keyboard controls (DVWR-01, DVWR-03, DVWR-04, Phase 21); the guided remote unlock flow (UNLK-01, Phase 23); audio and adaptive quality (Phase 23).

</domain>

<decisions>
## Implementation Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the six hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

### Input wire protocol
- Pointer and key events travel on one reliable, ordered WebRTC data channel per viewer labelled `input`, carrying small JSON messages `{t, ...}`, with the input protocol version negotiated once in the signalling hello (`input: 1`); pointer moves are coalesced to one per animation frame on the viewer. Low-rate, bulk or stateful messages (surface list and select, clipboard) stay on the versioned /desktop/ws signalling socket. The data channel is proven in every viewer engine (R13, R14), avoids head-of-line blocking behind signalling, and ordered delivery keeps move-before-click correct — **Reversibility:** costly — once Phase 21 ships, the home Mac's Portty app (its own frontend bundle) talks to the office daemon across versions, and the iPhone and browser viewers depend on the same messages.
- A viewer sends pointer positions as normalized `x, y` in 0..1 of the currently shared surface (display or window), plus the surface `epoch` it last received; the daemon maps to global points itself (display origin including negative origins, backing scale, window frame) and drops input carrying a stale epoch. Buttons are down/up events with a button index, the daemon computes click count (double and triple click) from timing and distance, and scroll is pixel deltas (dx, dy) after the viewer converts line and page modes. Viewers never need to know Retina scale, display layout or window position, so the browser, the Wails tab and the Phase 21 iPhone trackpad mode all send the same thing — **Reversibility:** costly — it is the cross-version contract every viewer encodes.
- Keyboard input uses two message kinds: `key` with the W3C `KeyboardEvent.code` (layout-independent physical key), down/up, a repeat flag and a modifier snapshot, which the daemon maps to a macOS virtual key code through a closed table (unknown codes dropped) so shortcuts work and the Mac's own layout picks characters; and `text` with a composed Unicode string (IME commit, emoji, dictation, iPhone soft keyboard) typed with CGEventKeyboardSetUnicodeString. Send-keys items and the Ctrl→Cmd swap are built in the viewer from ordinary `key` messages, so the protocol has no special actions — **Reversibility:** costly — keyboard semantics are part of the cross-version contract and the hardest part to change without breaking a viewer.

### Input security boundaries
- While CGSSessionScreenIsLocked is set, the daemon drops all pointer and key input from every viewer, and viewers show Locked with control greyed out; Phase 23 opens a dedicated, own-account-only unlock path (wake key, wait for the password field, type, check, retry once). Failing closed means the Phase 22 guest work cannot accidentally inherit unlock, and v2.0 ships after Phase 23, so nothing is lost at release — **Reversibility:** costly — it is a security boundary later phases build on; loosening it is easy, tightening it after viewers rely on it is not.
- In a window share, keys and clicks are posted only while the shared window's app is frontmost and the window is its main window; the first input after focus moved elsewhere raises the window (activate the app, AXRaise the window) and that triggering event is dropped; pointer moves are clamped to the window frame and clicks outside it are dropped. CGEvent posts are global, so without the frontmost check a viewer typing into a shared window could type into a password manager or another app they cannot see (research PITFALLS 11) — **Reversibility:** costly — it defines what a window share permits, which Phase 22 guests inherit.

### Clipboard and permissions
- The daemon reads NSPasteboard only when a viewer clicks Copy from Mac, checks item types first and skips concealed and transient items before reading content. A VM row first measures whether macOS 27.2 shows the paste-privacy alert for a launchd daemon read (and that a launchd Aqua agent can read NSPasteboard at all). If the alert appears, the Phase 19 permissions sheet gains a Paste from Other Apps row (set to Allow during the same office-Mac visit as Screen Recording and Accessibility), and a read that would prompt with nobody there returns Waiting for approval at the Mac instead of hanging. This keeps DCTL-06 two-way while honouring the Phase 19 one-trip principle (v2.0 is not on the office Mac yet) — **Reversibility:** costly — it may change the permission set users are walked through on the corporate Mac.

### Decided autonomously
- [Packages] Q: "Where does host input code live?" → Selected: "CGEvent, AX raise and NSPasteboard bridges as cgo ObjC in internal/screencap (input_darwin.m, pasteboard_darwin.m); per-viewer input state, gates and surface selection in internal/desktop; frontend lib/desktopInput.ts beside desktopClient.ts" — internal names
- [Posting] Q: "How are events posted?" → Selected: "CGEventPost at kCGHIDEventTap from one dedicated CGEventSource, every event tagged in kCGEventSourceUserData with a Portty marker so Phase 22 can tell remote from physical input" — local bridge detail
- [Posting] Q: "How are drags and moves posted?" → Selected: "Dragged event types while a button is held; moves warped to the mapped point; one input goroutine per share serialises posts" — server-only
- [Mapping] Q: "How are normalized coordinates mapped?" → Selected: "Global points from SCDisplay/CGDisplayBounds (negative origins and scale handled) or the SCWindow frame refreshed on move and resize; mapping rebuilt on display reconfiguration" — server-only math, unit-tested
- [Keys] Q: "Which code-to-keycode table?" → Selected: "Hand-written closed table of W3C codes to kVK constants (ANSI plus ISO/JIS extras); modifiers posted as their own key events with flags from the daemon's held set; browser repeat keydowns forwarded with the autorepeat field" — table edits are local
- [Stuck keys] Q: "How are held keys released?" → Selected: "Per-viewer held key and button sets; release-all on viewer blur, visibilitychange hidden, pagehide, data-channel close, peer failure, revoke, surface switch and lock; dead-man: while anything is held the viewer sends a 1 s heartbeat and the daemon releases all after 3 s of silence" — runtime policy
- [Limits] Q: "What input limits?" → Selected: "Data-channel messages up to 4 KiB, text messages up to 1 KiB each (viewer splits), a per-viewer rate cap with drop counters; clipboard up to 1 MiB text sent in chunks of at most 32 KiB to keep the HARD-02 WebSocket read limit" — constants
- [Logging] Q: "What is logged about input?" → Selected: "Only counts and event kinds; never key codes, text or clipboard contents (UNLK-01's never-logged rule applied from the first input path)" — tightening a log rule is cheap
- [Permissions] Q: "How is control availability shown?" → Selected: "Because AX preflights are cached per process (R08), the daemon checks the effect of a posted move (fresh CGEvent location) and pushes a Control needs Accessibility state to viewers and the GUI" — internal check
- [Surface] Q: "How does a display or window switch run?" → Selected: "select message on /desktop/ws, SCStream updateContentFilter on the running stream, encoder reconfigure for the new size, forced IDR, epoch bump pushed to viewers, held input released; no renegotiation" — server-side
- [Surface] Q: "Is the chosen surface persisted?" → Selected: "No; every capture start uses the main display and desktop.json is unchanged" — adding a field later is cheap
- [Window] Q: "How is the shared window raised?" → Selected: "NSRunningApplication activate plus AXRaise on the AX window matched by title and frame (no private _AXUIElementGetWindow)" — local bridge code
- [Clipboard] Q: "Which clipboard types?" → Selected: "public.utf8-plain-text only; skip items carrying org.nspasteboard.ConcealedType or org.nspasteboard.TransientType; writes from viewers set a plain string" — local filter
- [Arbitration] Q: "What if two own-account viewers both drive?" → Selected: "No arbitration in Phase 20; events interleave per viewer with separate held sets; DCTL-04 adds the 2 s hold in Phase 22" — replaced wholesale later
- [Dependencies] Q: "Any new libraries?" → Selected: "None: system frameworks via cgo and vanilla browser APIs" — nothing added
- [Testing] Q: "What test approach?" → Selected: "Pure-Go tests behind a fake poster for code mapping, coordinate mapping, epoch drop, release-all, lock gate, window confinement and limits; loopback data-channel integration tests reusing the Phase 19 synthetic source; VM rows for real CGEvent posting, window raise and the pasteboard alert check (R08 shows VM posts work); real-Mac M-NN rows for Retina plus external display mapping, IME and emoji typing, and clipboard on the office Mac; TESTING.md updated per the standing rule" — test layout

### Claude's Discretion
User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Claude may adjust these during planning:
- The viewer gets a slim auto-hiding top bar with Display/Window picker, Send keys menu, Copy from Mac, Paste to Mac, a Ctrl→Cmd toggle and a View only/Control toggle. Reason: one place for every DCTL control, and the video stays uncluttered.
- Control starts on for own-account viewers, and keyboard input is captured only while the video area has focus (click to focus). Reason: they are the user's own devices, and focus-gating stops stray typing meant for the viewer's own Mac.
- The display/window picker lives in the viewer, not the host GUI; the host's Settings Desktop group and tray state line show what is being shared. Reason: with nobody at the office Mac the remote user is the sharer; Phase 22 adds a host-side picker for click-to-share.
- The window list shows on-screen normal windows as "App — Title", including Portty's own. Reason: enough to pick, and nothing the viewer could not already see on the display.
- When a shared window closes or its display disappears, the share falls back to the main display with the notice "The shared window closed; showing the main display". Reason: the stream keeps running instead of going black.
- Ctrl→Cmd swap is off by default, toggled in the bar and remembered per browser. Reason: predictable for Mac viewers; non-Mac users turn it on once.
- The Send keys menu lists Cmd+Tab, Cmd+Space and Lock Screen (Ctrl+Cmd+Q chord) only. Reason: exactly DCTL-03; more items wait for a real need.
- Copy from Mac writes to the viewer's clipboard with a short "Copied from Mac" toast, shows the text in a selectable box when the browser refuses the write, and says "The Mac's clipboard holds a hidden item (password manager); not copied" for a concealed item. Reason: explicit, honest feedback for an explicit action.
- Paste to Mac sets the Mac's clipboard and toasts "Sent to the Mac's clipboard; press ⌘V", without pressing Cmd+V itself. Reason: the paste keystroke stays the user's, so text never lands in an unexpected app.
- Cmd+C and Cmd+V typed in the viewer go to the remote Mac as keys (acting on the remote clipboard) and never sync clipboards. Reason: DCTL-06 says sync only by explicit action.
- Over the video the local pointer is a small dot, with the real cursor visible in the frames. Reason: immediate feedback without hiding the baked-in cursor.
- A Fullscreen button enters fullscreen and, in Chrome, requests Keyboard Lock so Cmd and Esc chords reach the Mac; Safari gets fullscreen only. Reason: uses the platform where it exists, no workarounds (research STACK note).
- While locked, the bar shows Locked and the control items are greyed out. Reason: matches the Phase 19 state wording and the accepted lock gate.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 19 is planned but not built: internal/desktop, internal/screencap, DesktopView.tsx and lib/desktopClient.ts do not exist yet; Phase 20 extends them as 19-CONTEXT lays them out.
- spike/desktop (reference only, never imported): sys_darwin.m posts a pointer nudge and key codes with CGEventPost(kCGHIDEventTap); server.go OnDataChannel and handleControl show the string data-channel pattern (ping/pong, key:<code>); hostrun.go gates key input behind --allow-input and an allowlist; viewer.html creates the `control` channel.
- internal/webserver: input_limit_test.go pins the 32768-byte coder/websocket read limit (HARD-02) that clipboard chunking respects; whois.go requireSameUser and requireSameUserPage gate the desktop routes; server.go setupRoutes is where /desktop/ws lives.
- internal/relay MaxInputPayload and the v1.2 bounded input queue are the model for input caps.
- internal/capability capability.go Claims and HasPerm show the perms-string idiom; the Phase 19 desktop token deliberately has none, so viewer rights live in internal/desktop.
- frontend: KeyBar.tsx and lib/ctrlMap.ts (phone key bar, closed tables), lib/pasteGuard.ts (paste handling lessons), lib/pinchZoom.ts and touchScrollHandler.ts (Phase 21 reuse), WebShareSessionView.tsx (web viewer shell patterns), SettingsSheet.tsx Sharing pane (Desktop group from Phase 19).

### Established Patterns
- Closed lookup tables for key mapping (ctrlMap.ts): a key with no entry is dropped or sent as typed, never guessed.
- Every share-server route checks Origin, then WhoIs same-user, then the capability; token-bearing pages are no-store.
- Input is size-limited and queued with bounded memory (HARD-02, INPT-01); senders split around the limit.
- Seams are package vars with SetXForTest; exec'd tools go through a Runner seam; cgo ObjC .m files at the repo root (tray.go, notification_darwin.go, appearance_darwin.m) show the bridging style.

### Integration Points
- internal/desktop: per-viewer rights, held-input sets, lock gate, window confinement, epoch and surface selection; the input data channel is opened on each viewer's PeerConnection.
- internal/screencap: input_darwin.m (CGEvent post, AX raise), pasteboard_darwin.m, SCStream updateContentFilter and encoder reconfigure for switching.
- /desktop/ws signalling gains the input version in the hello, surface list and select, epoch, clipboard and Control needs Accessibility state messages.
- frontend: lib/desktopInput.ts (event capture, coalescing, key/text split, release-all, heartbeat) and the DesktopView toolbar.
- Settings Desktop group and tray state line show the shared surface; the Phase 19 permissions sheet may gain a Paste from Other Apps row depending on the VM check.

</code_context>

<specifics>
## Specific Ideas

Settled by prior decisions (not re-asked):
- Input is posted by the daemon itself with CGEventPost at the HID tap; Accessibility is attributed to the app bundle ID for the launchd and GUI-spawned daemon (17-SPIKE-MATRIX R08-B, R08-C GO; FEAS-02 in-daemon).
- An SSH-started daemon is unsupported for input; the CLI only kickstarts launchd (R08-D PARTIAL user-accepted; REQUIREMENTS DSHR-01 as amended; 19-CONTEXT Launchd).
- Accessibility is already requested by the Phase 19 onboarding sheet, so input adds no in-person trip (19-CONTEXT Claude's Discretion; DPRM-01).
- A WebRTC data channel works in Chrome, Safari, iPhone Safari and the Wails web view (R13 data-channel RTT median 59 ms; R11, R12, R14).
- Signalling is versioned JSON over WSS at /desktop/ws on the share server (19-CONTEXT Signalling).
- The desktop token has no permission claims, so per-viewer control and clipboard rights are daemon-side runtime state, never token claims (19-CONTEXT token decision; DSHR-04 grants per person at runtime).
- Own-account always-available viewers get control and clipboard with nobody at the Mac; guests are view-only until granted in Phase 22 (DSHR-01; PROJECT Key Decision on click-to-share guests; PROJECT "Full mouse/keyboard control").
- One shared encoder and track per share, so the shared surface is per share and every viewer of it sees a switch (research Architecture item 2; 19-CONTEXT WebRTC and Encode).
- SPS/PPS on every IDR lets a switch change resolution in-band with no renegotiation (19-CONTEXT Encode; research ARCHITECTURE Display/window switch).
- The cursor is drawn into the frames; a data-channel cursor is a Future requirement (research C6; REQUIREMENTS Future).
- The 2560 px long-edge cap at 30 fps applies to every surface, windows included (research C4; R20).
- Clipboard is text, by explicit action both ways, never polled or background-synced, with concealed and transient items skipped (DCTL-06; research C2; PITFALLS 12).
- Browsers cannot capture Cmd+Tab or Cmd+W and Keyboard Lock exists only in Chromium fullscreen; no workarounds beyond the Send-keys menu (research STACK; FEATURES Keyboard Lock note; DCTL-03).
- No new entitlements are needed for CGEvent or NSPasteboard under the hardened runtime (research STACK "No new entitlements").
- No CSP directive is added; a test pins the /desktop policy (19-CONTEXT CSP).
- Remote unlock with a wake key, a lock check and one retry is Phase 23; the password is never stored or logged (UNLK-01; R16-B rerun).

Findings from this discussion:
- Stale Accessibility preflight: R08-A and R08-B show AXIsProcessTrusted and CGPreflightPostEventAccess keep their first answer for the life of a process, while posted events follow the live grant (toggling Accessibility takes effect without a restart). A long-lived daemon therefore cannot use the preflight for control state; Phase 20 measures the effect of a posted move instead. The same caching may affect Phase 19's plan to poll AXIsProcessTrusted about once a second during onboarding.
- Input Monitoring risk for Phase 22 (noted, not decided here): detecting physical input for DCTL-04 with a CGEventTap would need Input Monitoring (ListenEvent, seen in R08-B tccd), a grant Phase 19 chose not to ask for; CGEventSourceSecondsSinceLastEventType plus the Portty event tag may avoid it.

Other specifics:
- R15 and R16: Ctrl+Cmd+Q locks the guest and posted keys reach loginwindow, which is why the lock gate exists and why Lock Screen can be an ordinary chord.
- R13: data-channel RTT median 59 ms over a DERP path in the Wails web view; direct paths should be lower.
- Pasteboard privacy on macOS 27.x rests on a single source (research PITFALLS 12); the VM measurement comes before any onboarding change.
- The window-share keystroke risk is research PITFALLS 11 ("a viewer typing into a shared terminal window types into the password manager").
- Research rules carry over: subagents never contact the host's tailscaled or 100.x addresses; nothing touches ~/Downloads or /Applications on the host; input and clipboard checks run in the VMs, or on the user's Macs with their go-ahead.

</specifics>

<deferred>
## Deferred Ideas

- Control arbitration (last typist holds about 2 s, physical input wins): DCTL-04, Phase 22.
- Per-person control and clipboard grants, guest view-only, and a host-side surface picker for click-to-share: Phase 22.
- Portty app Desktop tab with Cmd+W/Q/C forwarding, and iPhone trackpad mode, long-press right-click, two-finger scroll and soft keyboard: Phase 21.
- The guided unlock flow and lock-on-disconnect when the session started locked: Phase 23.
- Images and rich text on the clipboard; file transfer stays out of scope.
- A data-channel cursor (REQUIREMENTS Future).
- Persisting the chosen display or window across captures.

</deferred>
