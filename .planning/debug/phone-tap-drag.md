---
status: awaiting_human_verify
trigger: "M-195 real-iPhone UAT (iPhone 16 Pro, Safari, build cabf716f on portty-test, host load ~4, no floating panels): tap once, then immediately touch and drag the TextEdit window's title bar: the window does not move. One-finger move, tap, double-tap, long-press right click and two-finger scroll all work. VM input summaries for the phone: btn:10 rmove:827 wheel:215 and btn:36 key:42 rmove:1558 wheel:202 (no drop reasons, no not_held)."
created: 2026-11-27T00:00:00Z
updated: 2026-11-27T00:00:00Z
---

## Current Focus

hypothesis: two device-only gaps between the phone's gesture and a title-bar drag. (1) The phone presses at the Mac's last cursor report, and the daemon throttled reports to one per 33 ms with no trailing report, so the report of a pointer at rest could be the point before the last move(s): the press jumped the pointer back and could land off a 28-point title bar (the drag then selected text or did nothing). (2) After a tap, a second touch that rested 500 ms before moving became a click plus a right click, not a drag, although UI-SPEC says the button goes down at the second touch.
bug_class: device-only timing and position (not reproducible without the iPhone)

reasoning_checkpoint:
  hypothesis: "The press reaches the Mac (no drop counters) but either lands off the title bar (stale report) or never happens as a left press (the rest-then-move path right-clicks)."
  confirming_evidence:
    - "control.go reportCursor: 'shortcut: no trailing report, so the last position of a burst is reported with the next move; upgrade if the viewer needs it exact at rest'. The phone needs it exact at rest: DesktopPhoneLayer presses and clicks at cursorNow()."
    - "touchTrackpad pressTimer: in mode 'second' it flushed the tap (a click) and emitted longPress (a right click) at 500 ms."
    - "Daemon log summaries show only btn/rmove/wheel/key: no gate dropped a press (no_right, locked, stale_epoch, lease_held, rate, ...), no not_held, so presses and releases were posted."
  falsification_test: "Re-run M-195 on the fix build and read the viewer's input summary in the VM daemon log: drag:N>0 with the window still not moving means presses and drags were posted at the right place and macOS is not moving the window (then trace the posted points); drag:0 means the phone never held the button (the tap-then-drag window, 300 ms from lift to the second touch, is too tight for real hands)."
  fix_rationale: "A trailing report makes the last report exact once the pointer rests (at most 33 ms late), so the press lands where the user sees the pointer. A rest after the tap now presses (dragStart) at 500 ms, as the spec row says; the lift releases."
  blind_spots: "Not run on a device. The 300 ms re-touch window is unchanged (widening it delays every single click by the same amount). Window share: the capture follows a moved window, so the picture may not show it moving; M-195 is a display share."

eliminated:
  - hypothesis: "macOS ignores Portty's synthetic drags (private event source, no click state on dragged events)"
    evidence: "VM experiment (portty-test, /tmp/dragx, the same CGEvent calls as input_darwin.m: private source, HID tap, deltas set, no click state on dragged events): down on TextEdit's title bar then 20 dragged events moved the window by 99 points, with 50 ms and with 0 ms between the down and the first drag; moved back the same way."
  - hypothesis: "a Phase 22/23 gate (arbiter lease, press gate, lock, window_key_focus) drops the phone's press while rmove continues"
    evidence: "No drop reason in either viewer summary; window_key_focus applies to keys and text in a window share only."
  - hypothesis: "p.drag is not set when the press comes from the phone and the moves come as rmove"
    evidence: "handlePress records the button under m.in.mu before posting; handleRMove -> postPointer -> moveTo returns lowestHeld(); TestControl_PhoneTapThenDrag shows Down, Dragged x3, Up."

## Evidence

- timestamp: 2026-11-27 VM daemon log: viewer d4170a6a (the 21:52 drag retry) global counters delta btn:14 rmove:240, no drop counters.
- timestamp: 2026-11-27 VM experiment: Portty-identical drag posting moves TextEdit's window (146,81 -> 245,81 and back).

## Resolution

root_cause: "(1) No trailing cursor report: the phone's press used a report up to 33 ms older than the pointer's last move, jumping the pointer back off the title bar. (2) A second touch after a tap that rested 500 ms right-clicked instead of pressing."
fix: "internal/desktop/control.go reportCursor sends one trailing report with the newest point when the 33 ms window ends; input.go counts 'drag' (moves posted with a button held) in the leave summary. frontend/src/lib/touchTrackpad.ts: a rest in mode 'second' starts the drag (dragStart) instead of click + right click."
verification: "Failing first: TestControl_PhoneTapThenDrag and the updated TestControl_CursorReportAfterRmove (timed out waiting for the trailing report); touchTrackpad 'M-195: ... rests 600 ms ...' and '... held still and lifted ...'; DesktopPhoneLayer 'M-195: tap, then touch and rest on a title bar, then drag ...' (3 failed before the fix). After: go test -race ./internal/desktop pass, tsc clean, full vitest pass."
files_changed:
  - internal/desktop/control.go
  - internal/desktop/input.go
  - internal/desktop/control_test.go
  - frontend/src/lib/touchTrackpad.ts
  - frontend/src/lib/__tests__/touchTrackpad.test.ts
  - frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx

## Re-check on the real iPhone (Safari)

Build to test: v23.8.2 from 6ea4e35e (p22-rig build22, tags "wailsassets vmdesktop", Developer ID), deployed to portty-test 2026-11-27 with a fresh always-available link; TextEdit frontmost with ~/p21/scroll-test.txt.

1. Move the pointer onto TextEdit's title bar with one finger and lift. Tap once, then within about a quarter second touch again and drag: the window follows the finger and drops on lift. Watch for the small ring at the drag start (it marks the press).
2. Same, but after the second touch rest the finger for a full second before moving: the ring shows at about half a second and the window then follows (no context menu).
3. Do 1 three times with quick and slow approaches to the title bar.
4. Afterwards, in the VM: `grep "input summary" "$HOME/Library/Application Support/portty/daemon.log" | tail -1` (or `p22-rig.sh dlog`): note drag:N. If the window still did not move: drag:0 means the phone never held the button (report the gap you leave between the tap and the second touch); drag>0 means the press and drags were posted (report where the pointer jumped, if it did).
