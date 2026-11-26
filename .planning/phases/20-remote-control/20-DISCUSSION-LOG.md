# Phase 20: Remote Control - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-25
**Phase:** 20-remote-control
**Areas discussed:** Input wire protocol, Input security boundaries, Clipboard and permissions

**Mode:** autonomous smart discuss. The user delegated on 2026-11-25 ("I don't need to answer a bunch of questions"). Reversible technical choices were decided autonomously, product and UX choices went to Claude's Discretion, and only hard-to-reverse questions were presented, each with a recommendation. The user replied "Accept all" for all six.

---

## Input wire protocol

Q1: How do pointer and key events travel from viewer to Mac?

| Option | Description | Selected |
|--------|-------------|----------|
| Reliable ordered data channel `input`, versioned JSON | Proven in every viewer engine (R13, R14); clipboard and surface select stay on /desktop/ws (costly) | ✓ |
| Compact binary frames on the data channel | Smaller, but a second encoding to version and debug for no measured need | |
| Everything over /desktop/ws | One channel, but TCP head-of-line blocking puts input behind clipboard and signalling | |

Q2: What pointer coordinates does a viewer send?

| Option | Description | Selected |
|--------|-------------|----------|
| Normalized 0..1 of the shared surface plus epoch | Daemon maps to points, computes click count, drops stale-epoch input; scroll as pixel deltas (costly) | ✓ |
| Video-pixel coordinates | Viewer must know the encoded size, which changes on switch and under the cap | |
| Viewer sends click count from MouseEvent.detail | iPhone taps and Phase 21 trackpad mode would each reinvent it | |

Q3: What does a viewer send for keyboard input?

| Option | Description | Selected |
|--------|-------------|----------|
| `key` with KeyboardEvent.code plus `text` with composed Unicode | Layout-correct shortcuts and IME/emoji text; Send-keys and Ctrl→Cmd built in the viewer (costly) | ✓ |
| Characters only | Breaks shortcuts and non-US layouts | |
| Viewer sends macOS key codes | Ties every viewer to a Mac table and still needs a text path | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Input security boundaries

Q4: May a viewer click or type while the Mac is locked?

| Option | Description | Selected |
|--------|-------------|----------|
| No; drop all input while locked, Phase 23 adds an own-account unlock path | Fails closed so Phase 22 guests cannot inherit unlock (costly) | ✓ |
| Let own-account viewers type at the lock screen now | Unlock works early (about 11 of 13 per R16-B); Phase 22 must add a guest gate | |
| Allow pointer only while locked | Still reaches the lock screen's Sleep, Restart and Shut Down buttons | |

Q5: In a window share, where may keystrokes land?

| Option | Description | Selected |
|--------|-------------|----------|
| Only while the shared window's app is frontmost; raise and drop the triggering event otherwise | Keys can never reach unseen apps; clicks outside dropped, moves clamped (costly) | ✓ |
| Confine clicks only | DCTL-05 literal and simpler, but keys can reach unseen apps | |
| Drop keys while not frontmost, no raise | Safe, but the viewer must click first every time focus moves | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Clipboard and permissions

Q6: How does Copy from Mac read the pasteboard without hanging an unattended Mac on a paste-permission alert?

| Option | Description | Selected |
|--------|-------------|----------|
| Read only on an explicit click, types checked first; VM measures the 27.2 alert; if present, add a Paste from Other Apps row to the Phase 19 sheet and show Waiting for approval at the Mac | Keeps DCTL-06 two-way within the one-trip principle (costly) | ✓ |
| Viewer-to-Mac paste only | Writes never prompt, but DCTL-06 becomes one-way | |
| Read through the GUI process only | Fails exactly in the unattended case | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Decided autonomously

[auto] [Packages] — Q: "Where does host input code live?" → Selected: "cgo ObjC bridges in internal/screencap; per-viewer state, gates and surface selection in internal/desktop; frontend lib/desktopInput.ts" (reversible technical — decided autonomously: internal names)
[auto] [Posting] — Q: "How are events posted?" → Selected: "CGEventPost at kCGHIDEventTap from one dedicated CGEventSource, tagged in kCGEventSourceUserData" (reversible technical — decided autonomously: local bridge detail)
[auto] [Posting] — Q: "How are drags and moves posted?" → Selected: "Dragged event types while a button is held; one input goroutine per share" (reversible technical — decided autonomously: server-only)
[auto] [Mapping] — Q: "How are normalized coordinates mapped?" → Selected: "Global points from SCDisplay/CGDisplayBounds or the SCWindow frame; rebuilt on reconfiguration" (reversible technical — decided autonomously: server-only math)
[auto] [Keys] — Q: "Which code-to-keycode table?" → Selected: "Hand-written closed W3C code to kVK table; modifiers as key events; repeats forwarded with autorepeat" (reversible technical — decided autonomously: table edits are local)
[auto] [Stuck keys] — Q: "How are held keys released?" → Selected: "Per-viewer held sets, release-all on blur, hide, close, failure, revoke, switch and lock; 1 s heartbeat, release after 3 s silence" (reversible technical — decided autonomously: runtime policy)
[auto] [Limits] — Q: "What input limits?" → Selected: "4 KiB data-channel messages, 1 KiB text messages, per-viewer rate cap; clipboard up to 1 MiB in 32 KiB chunks" (reversible technical — decided autonomously: constants)
[auto] [Logging] — Q: "What is logged about input?" → Selected: "Counts and event kinds only; never key codes, text or clipboard contents" (reversible technical — decided autonomously: tightening a log rule is cheap)
[auto] [Permissions] — Q: "How is control availability shown?" → Selected: "Check the effect of a posted move and push a Control needs Accessibility state" (reversible technical — decided autonomously: internal check)
[auto] [Surface] — Q: "How does a display or window switch run?" → Selected: "updateContentFilter on the running stream, encoder reconfigure, forced IDR, epoch bump, held input released" (reversible technical — decided autonomously: server-side)
[auto] [Surface] — Q: "Is the chosen surface persisted?" → Selected: "No; every capture start uses the main display" (reversible technical — decided autonomously: adding a field later is cheap)
[auto] [Window] — Q: "How is the shared window raised?" → Selected: "NSRunningApplication activate plus AXRaise on the window matched by title and frame" (reversible technical — decided autonomously: local bridge code)
[auto] [Clipboard] — Q: "Which clipboard types?" → Selected: "public.utf8-plain-text only; skip Concealed and Transient types" (reversible technical — decided autonomously: local filter)
[auto] [Arbitration] — Q: "What if two own-account viewers both drive?" → Selected: "No arbitration in Phase 20; DCTL-04 in Phase 22" (reversible technical — decided autonomously: replaced wholesale later)
[auto] [Dependencies] — Q: "Any new libraries?" → Selected: "None" (reversible technical — decided autonomously: nothing added)
[auto] [Testing] — Q: "What test approach?" → Selected: "Pure-Go tests behind a fake poster, loopback data-channel integration tests, VM rows for CGEvent, raise and pasteboard alert, real-Mac M-NN rows; TESTING.md updated" (reversible technical — decided autonomously: test layout)

## Claude's Discretion

User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). The viewer toolbar layout, control-on default with focus-gated keyboard, the viewer-side surface picker, the window list format, fallback to the main display when a shared window closes, the Ctrl→Cmd default, the Send-keys items, the Copy from Mac and Paste to Mac feedback, Cmd+C/Cmd+V passing through as keys, the local dot pointer, Fullscreen with Chrome Keyboard Lock, and the Locked bar state are recorded in CONTEXT.md with a one-line reason each.

## Deferred Ideas

Control arbitration (Phase 22); per-person grants and a host-side picker (Phase 22); Portty app Desktop tab and iPhone touch controls (Phase 21); guided unlock and lock-on-disconnect (Phase 23); rich clipboard; data-channel cursor; persisting the chosen surface.
