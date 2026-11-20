---
phase: 08-debt-cleanup
plan: 03
subsystem: frontend/peer-tabs
status: complete
tags: [DEBT-04, DEBT-05, peerSnapshots, relayClient, TerminalPanel, paste]
requires: [07-02, 07-04]
provides:
  - "peerSnapshotHost / peerSnapshotKey: the one key helper (host + NUL + session id) used by both save and drop"
  - "dropPeerSnapshot with a per-key generation guard; removeTabState drops a peer tab's entry on every removal path"
  - "PEER_SNAPSHOT_MAX_BYTES (8 MiB, UTF-16) plus the 8-entry cap; over-budget snapshots not stored"
  - "PEER_FLUSH_TIMEOUT_MS: a hidden peer xterm is always disposed; PEER_SAVE_WAIT_MS: a quick remount waits for the pending save"
  - "RelayClient.onPasteInterrupted({ sentBytes | null, totalBytes }) from bufferedAmount and an ack ping"
  - "SessionEndedBanner kind 'paste-interrupted' (two fixed copies, Dismiss)"
affects:
  - "08-05 (TESTING.md rows for the test files below)"
tech-stack:
  added: []
  patterns:
    - "split-paste delivery is confirmed by the pong to a MSG_PING sent right after the paste (server handles a socket's frames in order)"
    - "mount effect start() step with a cancelled flag, so the dial can wait on async work and the cleanup copes with no client"
key-files:
  created: []
  modified:
    - frontend/src/lib/peerSnapshots.ts
    - frontend/src/lib/__tests__/peerSnapshots.test.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
decisions:
  - "DEBT-04: the snapshot key is connectHost ?? peerHost plus NUL plus session id, built by one helper for save and drop"
  - "DEBT-04: putPeerSnapshot takes the generation read at mount (default: current); a drop in between makes the put a no-op"
  - "DEBT-04: awaitPeerSave returns null when nothing is pending, so the panel dials synchronously as before"
  - "DEBT-05: a split paste is acknowledged by the pong to a MSG_PING sent right after it; an unacknowledged paste with nothing buffered reports sentBytes null"
  - "DEBT-05: the paste notice is not gated on the guest path; a host-path drop with a split paste also shows it"
requirements-completed: [DEBT-04, DEBT-05]
metrics:
  duration: ~40m
  completed: 2026-11-19
actuals:
  tokens: 15900
  tasks: 3
  commits: 6
plan_head_before: 292daeea39c089bbdb9938f4576138403c47cd61
plan_head_after: 865c4bb49272489d4fba37b795117c5a0c7e1fc6
---

# Phase 8 Plan 03: Peer Snapshot Store and Partial Paste Notice Summary

The peer-tab snapshot store is now keyed by host plus session, bounded by 8 entries and 8 MiB, and dropped when its tab is removed. A generation guard stops a late flush from storing it again. A flush timeout always disposes the hidden xterm, and a quick switch back waits up to 250 ms for the pending save, so it still gets a delta. A split paste that a socket drop cuts short now shows "Paste interrupted: N of M KB sent", where N counts only whole frames that left the browser. If the host never acknowledged the paste, the notice reads "Paste may have been cut short (M KB)" instead. Nothing is resent.

## Tasks

| # | Task | Commits |
|---|------|---------|
| 1 | Snapshot keyed by host plus session from App to the store, dropped on close with a generation guard (tracer) | 33d995aa (RED), a0fa06f3 (GREEN) |
| 2 | 8 MiB byte budget, flush timeout dispose, pending-save wait | d0e409dd (RED), 744f0f77 (GREEN) |
| 3 | Split-paste accounting in RelayClient, paste-interrupted banner, full Vitest run | fa91ea64 (RED), 865c4bb4 (GREEN) |

## Tests added or edited (for TESTING.md, 08-05)

**frontend/src/components/__tests__/App.open-remote.test.tsx**, new describe "App — peer tab snapshots (DEBT-04)":
- the view gets the MagicDNS name for a MagicDNS peer and the hostname for a hostname-only peer
- closing a MagicDNS peer's tab drops its entry and keeps another host's entry with the same id
- closing a hostname-only peer's inactive tab drops its entry
- File > Close Tab (Cmd+W) on the active peer tab also drops its entry

**frontend/src/lib/__tests__/peerSnapshots.test.ts**, new describes:
- "peerSnapshots byte budget (DEBT-04 IN-02)": the budget is 8 MiB; two 3 MiB snapshots fit, a third evicts the oldest; a 9 MiB snapshot is not stored and leaves the others in place
- "peerSnapshots key and generation (DEBT-04 W1, close race)": equal session ids on two hosts are two keys; a put with the generation read before a drop stores nothing, the new generation stores
- "peerSnapshots pending save (DEBT-04 IN-03)": no pending save, so awaitPeerSave returns null and the panel starts at once; a pending save resolves awaitPeerSave when it settles, and the key is free again; a save that never settles resolves awaitPeerSave after PEER_SAVE_WAIT_MS
- The existing "keeps at most 8" case still covers the entry cap.

**frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx** (the afterEach drain uses the keyed form; the mock write callback is now controllable):
- a flush callback that never fires: the terminal is disposed at PEER_FLUSH_TIMEOUT_MS and nothing is stored
- a remount while the save is pending waits for it, restores the snapshot and dials resume=104
- a save that never finishes holds the remount PEER_SAVE_WAIT_MS at most; it then dials a full replay
- a drop before the unmount: the late flush stores nothing
- unmounting while still waiting on a pending save creates no RelayClient and disposes the terminal

**frontend/src/lib/relayClient.test.ts**:
- Edited (HARD-02): "sendInput of 100,000 characters on an open socket sends 4 MSG_INPUT frames in order" now asserts the 4 input frames in order followed by exactly one MSG_PING
- New describe "partial paste on a socket drop (DEBT-05)":
  - host path: 40,000 bytes still buffered at the drop leaves only frame 1 whole: (32767, 100000)
  - the split paste is followed by exactly one MSG_PING (the ack)
  - nothing buffered and the ack pong received: no notice
  - nothing buffered but no ack pong: one notice with sentBytes null
  - a pong to a ping sent before the paste is not the ack
  - keystrokes queued after the paste do not count against it: 20 buffered bytes and the ack pong, no notice
  - keystrokes buffered after an unacknowledged paste: unconfirmed (null), never a partial count
  - two split pastes back to back, part of the second buffered: one notice covering both
  - reconnect mode: a drop mid-paste gives one notice, and after the redial the new socket carries no input frame
  - reconnect mode: the watchdog abandoning a stale socket with paste bytes buffered gives one notice
  - a user close() mid-paste: no notice
  - a single-frame input then a drop: no notice and no ack ping

**frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx** (the afterEach drain uses the keyed form; MockWS gains bufferedAmount):
- a drop with paste bytes buffered shows "Paste interrupted: 31 of 98 KB sent", and Dismiss removes it (also asserts it shows together with the reconnecting banner)
- a drop with nothing buffered and no ack pong shows "Paste may have been cut short (98 KB)"

**frontend/src/components/__tests__/TerminalPanel.scale.test.tsx**: the source gate "guest onResize goes through applyHostGrid with recomputeScale as its follow-up (05-12)" now matches whitespace-tolerantly (see Deviations).

## Verification

- Task 1 verify: 5 files, 89 tests passed; tsc clean; grep gates: peerSnapshotKey export 1, `dropPeerSnapshot(` in App 1, `peerSnapshotHost(` in App 2, snapshotHost in WebShareSessionView 3, in TerminalPanel 3.
- Task 2 verify: 7 files, 155 tests passed; tsc clean; PEER_SNAPSHOT_MAX_BYTES 3, PEER_FLUSH_TIMEOUT_MS in TerminalPanel 2, PEER_SAVE_WAIT_MS 3.
- Task 3 verify: relayClient, TerminalPanel.reconnect and SessionEndedBanner: 3 files, 122 tests passed.
- **The one full Vitest run: 90 test files, 1267 tests passed (39.5 s).** No flake reruns needed. `pnpm exec tsc --noEmit` clean.
- Grep gates: onPasteInterrupted in relayClient.ts 3, in TerminalPanel.tsx 1; "Paste interrupted" 1; "may have been cut short" 1; `export const MAX_INPUT_PAYLOAD = 32767` 1 (unchanged).
- `git status --porcelain -- frontend/src/wailsjs` is empty (no Wails build was run).
- The daemon proxy (internal/daemon/remote_ws_proxy.go) pumps each direction in one loop, so input frames, the ack ping and the pong keep their order end to end. relay/server.go handles MsgInput before the following MsgPing in the same read loop.

## Tab-removal paths

Every peer-tab removal goes through `removeTabState`: the close button and the Close Tab menu (closeTab), Cmd+W (`menu:close-tab` calls removeTabStateRef), End Session (endSession), and the session:exit watcher. The other `setTabs` calls are the startup restores, which replace an empty or local-only tab list (no peer tabs exist then), and the tray focus, which only adds. No other path removes peer tabs.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TerminalPanel.reconnect.test.tsx afterEach drain moved to the keyed form in Task 1**
- **Found during:** Task 1 GREEN
- **Issue:** The test's afterEach drained `takePeerSnapshot('s1')`. With the keyed store that no longer removed the snapshot, so it leaked into the next case ("a full replay ... writes RIS once" saw two RIS writes).
- **Fix:** The same keyed drain as tabSwitch: `takePeerSnapshot(peerSnapshotKey('', 's1'))`. The file is in this plan's files_modified (Task 3).
- **Commit:** a0fa06f3

**2. [Rule 3 - Blocking] Whitespace-tolerant source gate in TerminalPanel.scale.test.tsx (outside files_modified)**
- **Found during:** Task 2 GREEN
- **Issue:** The 05-12 source gate matched `'onResize: isGuest\n        ? (cols, rows) => applyHostGrid(...)'` with an exact indent. Moving the client into `start()` indents it two more spaces.
- **Fix:** Changed it to a regex, `/onResize: isGuest\n\s+\? \(cols, rows\) => applyHostGrid\(term, cols, rows, recomputeScale\)/`. It makes the same assertion, only the indent check is gone. Neither 08-01 nor 08-04 touches this file.
- **Commit:** 744f0f77

**3. [Plan wording] End Session case replaced by Cmd+W in the App test**
- **Issue:** The plan's behaviour list has "Ending a session (End Session confirm) also drops its entry". End Session… is offered on local tabs only (TabBar `isLocal`, Sidebar local rows), so a peer tab cannot be ended from the UI.
- **Fix:** The App test covers the other real peer-tab removal path instead: File > Close Tab (Cmd+W, `menu:close-tab`). endSession still calls removeTabState, so it would drop a peer entry too.

**4. [Test addition] One extra relayClient case**
- "keystrokes buffered after an unacknowledged paste: unconfirmed (null), never a partial count". The plan's keystroke case ends with the ack pong, which clears the record by itself. This case checks that bytes queued after the paste are never counted as unsent paste bytes.

### Notes

- In Task 2 RED, the close-race case ("a drop before the unmount: the late flush stores nothing") and the generation unit test already passed, because Task 1 shipped the generation guard. They lock in that guard; the other new Task 2 cases failed as expected.
- `putPeerSnapshot`'s generation parameter defaults to the current generation, so the existing store tests that call `putPeerSnapshot(id, snap)` run unchanged.

## Known Stubs

None.

## Threat Flags

None. No new endpoint or protocol change. The ack is the existing MSG_PING. T-05-14 (input dropped while not open), MAX_INPUT_PAYLOAD and encodeInputFrames are unchanged. The banner renders only client-computed numbers.

## Self-Check: PASSED

- All 12 modified files exist; commits 33d995aa, a0fa06f3, d0e409dd, 744f0f77, fa91ea64, 865c4bb4 are ancestors of HEAD.
