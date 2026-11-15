---
phase: 01-plain-macos-terminal
reviewed: 2026-11-15T01:33:03Z
depth: standard
iteration: 4
files_reviewed: 6
files_reviewed_list:
  - .github/workflows/release.yml
  - frontend/src/App.tsx
  - frontend/src/components/Hub/HubPanel.tsx
  - frontend/src/components/Hub/HubPanel.test.tsx
  - frontend/src/components/__tests__/App.shellExit.test.tsx
  - frontend/src/components/__tests__/App.tailscale.test.tsx
findings:
  critical: 0
  warning: 2
  info: 7
  total: 9
  necessity: 1
status: issues_found
---

# Phase 1: Code Review Report (re-review after fix iteration 2)

**Reviewed:** 2026-11-15T01:33:03Z
**Depth:** standard
**Files Reviewed:** 6 (the files touched by fix commits 3b5c76c9..e47fe583; everything else in the phase scope is unchanged since the previous review, 01-REVIEW.iter3.md)
**Status:** issues_found

## Summary

**Scope.** This pass reviews `git diff 3b5c76c9~1..e47fe583` in context. For WR-03 (the exit path no longer calls the daemon), I traced what the daemon still does when a session exits:
- `handleDeleteSession` → `engine.KillSession` + `runSessionExitCleanup`;
- `handleWebServe`.

For the `isRemote` change, I traced the modal's render path through `HubModal` → `HubInteractiveModal` → `TerminalChatHost` → `TerminalPanel`.

**Ground truth:**
- `go build ./... && go vet ./...`: clean.
- `pnpm exec tsc --noEmit`: clean.
- `pnpm exec vitest run`: 117 files, 1824 tests pass.
- `go test -race -short -count=1 ./...`: every package passed except one failure in `internal/webserver`, `TestRelay_MixedReplyAndKeystrokes` ("timed out waiting for pipe data"). That run overlapped the vitest run.
  - The test passes at `-count=3` on its own, and the whole package passes when re-run.
  - It was last changed in Phase 111, and no Go file changed in this iteration.
  - This is a timing flake under load in a file outside this scope. I am noting it here, not filing it.

**Status rule.** `clean` means reviewed with no findings. Any warning or info finding means `issues_found`, and a necessity warning counts. WR-02 (necessity) and the info items alone would still give `issues_found`. WR-01 is a non-necessity warning, so it falls inside a `critical_warning` fix scope.

### Previous findings (01-REVIEW.iter3.md WR-01..WR-05), checked in the code

| ID | Verdict | Evidence |
|----|---------|----------|
| WR-01 (init reply overwrites newer health event) | **Resolved** | `healthEventSeqRef` is bumped in the `tailscale:health` handler (App.tsx:546). init and retryInit capture it before `Promise.all` and apply the reply only if it is unchanged (App.tsx:441/455, 1219/1233). Details below. |
| WR-02 (restore of an exited session) | **Resolved** | Both restore paths filter `!exitedIdsRef.current.has(s.id)` (App.tsx:449, 1227). The filtered list feeds both `setTabs` and `setHubSessions`. |
| WR-03 (redundant kill + warn on every exit) | **Resolved** | The `session:exit` handler calls only `removeTabStateRef.current`, which makes no daemon call (App.tsx:617-626). `handleCloseTab` still runs `ToggleWebServing` (when web-enabled), then `KillSession`, then `removeTabState` (App.tsx:776-787). Details below. |
| WR-04 (remote modal auto-closed on probe failure) | **Resolved** | `isRemote` is recorded at open in both `handleCardClick` and `handleCapAcquired` (HubPanel.tsx:399, 415). The auto-close skips remote modals (HubPanel.tsx:302-308). There is a behavioural test. The render-time routing left behind is new WR-01. |
| WR-05 (CI timeouts) | **Resolved** | `sign-macos` has `timeout-minutes: 90` and `notarytool --wait --timeout 60m`. `publish` has `timeout-minutes: 15`. |

### New code checked for regressions

- **Counter-based Tailscale guard: sound.**
  - `GetTailscaleStatus` emits under `healthMu` whenever its result changes. So any value the reply could carry that differs from the last emit also arrives as an ordered event.
  - If an event bumped the counter, skipping the reply loses nothing: the UI ends at the last emitted value, which equals `lastHealth`.
  - If no event arrived, the reply equals `lastHealth`.
  - The one way to go stale would need Wails to deliver events out of order. `healthMu` exists to rule that out.
- **`exitedIdsRef` restore filter: sound.** The `session:exit` subscription is registered synchronously in the same mount effect, so every exit that happens while `Promise.all` is pending is recorded before the filter runs. retryInit can only run after mount.
- **`removeTabState` / `handleCloseTab` split: no daemon-side cleanup was lost.** The removed `ToggleWebServing(id, false)` used to do four things: `DisableSession`, `ClearGrants`, `DisconnectWebViewers` and funnel teardown. Every exit route now gets all of that from the daemon itself:
  - **Watcher kill or `agenthub kill`:** both reach `DELETE /sessions/{id}`. `handleDeleteSession` runs `engine.KillSession` (which removes the relay hub, so web viewers drop) and then `runSessionExitCleanup` (`DisableSession`, `ClearGrants`, ref-counted funnel teardown), synchronously (internal/daemon/api.go:699-756).
  - **Daemon restart:** the old daemon's web and funnel state is gone with the process.
  - `removeTabState` still clears `webEnabled[id]` on the client side.
  - The user-close path keeps `ToggleWebServing` before its kill, so viewers still get the disconnect notice there.
  - The watcher's second `removeTabState` call after a user close is an idempotent no-op.

## Warnings

### WR-01: An open remote Hub modal flips to "Local" when the remote list empties, which hides the session-ended banner (fixer-flagged residual)

**File:** `frontend/src/components/Hub/HubPanel.tsx:518-531`, `frontend/src/components/TerminalPanel.tsx:164-165`, `frontend/src/components/TerminalPanel.tsx:965`, `frontend/src/components/Hub/HubModal.tsx:135-136`
**Issue:** The WR-04 fix keeps remote modals open across probe failures on purpose, so this path is now the designed one. The modal's render still recomputes `const isRemote = remoteIdSet.has(modalState.session.id)` and passes it as `remote`. It ignores the `modalState.isRemote` flag recorded at open.

When `remotePeers` empties, `remote` becomes `false` on the open modal. Triggers include a `ListTailnetPeers` error, an unreachable or slow peer, and `tailscaleOff`. The live WebSocket is not re-routed: TerminalPanel's mount effect depends on `[sessionId, wsURL]`, and the guest-only resize behaviour is captured at mount (TerminalPanel.tsx:367, 381-383). Everything that reads `remote` at render does flip:
- **Header:** `HubModal` switches to the desktop icon and the text "Local" for a session on another machine.
- **Session-ended banner:** `isGuestRef.current` becomes `false` (TerminalPanel.tsx:165). The banner is gated on it at render (`{isGuestRef.current && sessionEnded.ended && …}`, line 965). The mount closure still sets `sessionEnded`, but the banner is never shown.
- **Guest scale path:** an `isActive` toggle while flipped re-runs the effect at line 751 on the host-fit path for a guest terminal.

The likely trigger is correlated with the session ending. A peer that goes offline drops out of the probe, and its sessions end. So the user gets exactly what WR-04 (iteration 1) was meant to remove: a frozen terminal with no indication, now also labelled "Local". The routing read predates this phase, but this iteration's fix made it reachable by design. The fixer's report calls it a one-line follow-up.
**Fix:** Route from the provenance recorded at open:
```tsx
{modalState && relayPort !== undefined && relayPort > 0 && (
  <HubModal
    session={modalState.session}
    /* … */
    remote={modalState.isRemote}
  />
)}
```
Extend the "iteration-2 WR-04" HubPanel test so that, after `render([])`, the modal still shows the remote origin (the globe icon and hostname, not "Local").

### WR-02: Already-defended-elsewhere: init/retryInit `KillSession` of stopped sessions duplicates the Go watcher (carried forward, deliberately not fixed)

**File:** `frontend/src/App.tsx:450-452`, `frontend/src/App.tsx:1228-1230`
**Category:** necessity
**Issue:** The repository shows that the frontend kill guards nothing:
- `startup` starts the exit watcher unconditionally (app.go:162).
- `checkExits` runs immediately and then every 500 ms, and kills every session listed as `stopped` (app.go:301-346).
- The watcher keeps running across `RetryDaemon` (app.go:193-200).

The frontend copy is a second, concurrent killer for the same ids at launch. Whichever call loses logs `KillSession of stopped session failed`. With WR-03 fixed, that is now the only spurious warning left, as the fixer notes. It is out of `critical_warning` fix scope, and removing it is a human call.
**Fix:** Delete the two `allSessions.filter((s) => s.state === 'stopped').forEach(KillSession…)` blocks. Keep the `state !== 'stopped' && !exitedIdsRef…` restore filter.

## Info

All of these are carried forward unchanged. This iteration touched none of their files except App.tsx, and none of these items are in App.tsx.

### IN-01: Stale comments that now contradict the code

**File:** `internal/daemon/engine.go:365-366`, `internal/pty/cleanup.go:37`, `frontend/src/components/SettingsTab.tsx:53-54`, `tray_objc_darwin.m:248`, `frontend/src/wailsjs/go/main/App.js:69`, `TESTING.md:216`, `app.go:1181-1185`
**Issue:**
- engine.go says "On Linux/Windows ReapNaturalExit is a no-op: the platform exit detector…". No exit detector exists.
- cleanup.go mentions "a racing exit detector (Linux)".
- The SettingsTab doc says "shell and tailscale path overrides". Both were removed.
- tray_objc says "your session needs input".
- App.js has an empty `// Auto-install Tailscale bound method` header.
- TESTING.md:216 still says `TestDerivedStatus_ExitedErrored` checks `GetSessionStatus`, which is deleted.
- In `refreshTrayState`, the `a.client == nil // Startup failed` branch no longer describes a production state, because `startup` assigns `a.client` before any poller starts.

**Fix:** Update or delete these comments and doc lines.

### IN-02: `QuitGUIOnly` counts active sessions by `Status != "stopped"`, a value `Status` never takes

**File:** `app.go:244`
**Issue:** `Status` is `running|exited|errored`. Impact is small, because the watcher removes stopped sessions within about 500 ms.
**Fix:** Use `s.State != "stopped"`.

### IN-03: The Linux Playwright fixture has no exit detection

**File:** `internal/pty/reap_other.go:1`, `internal/pty/native.go`
**Issue:** On Linux, PTY EOF does not surface, so fixture sessions never reach "stopped", and the watcher's kill and exit never fire there. No current e2e test depends on it.
**Fix:** Document this in `reap_other.go`, or add a fixture-side guard.

### IN-04: "Start Web Server" with Tailscale off starts LAN mode while the banner says sharing is off (T-01-42-adjacent)

**File:** `app.go:725-740`, `frontend/src/components/SettingsTab.tsx:301-321`
**Issue:** `StartWebServer` silently falls back to `local` (LAN) mode while the banner reads "Remote access and sharing are off". This is known Phase 4 work.
**Fix:** Defer to Phase 4. At minimum, disable the button while `tailscaleOff`.

### IN-05: The traceability parser drops the second of two adjacent path cells

**File:** `tests/check-traceability-paths.sh:22`
**Issue:** `grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|'` consumes the closing pipe.
**Fix:** Split on `|` first (`awk -F'|' '{for(i=2;i<NF;i++) print $i}'`), then filter.

### IN-06: The `GetSessionQRCode` binding has no Go method

**File:** `frontend/src/wailsjs/go/main/App.d.ts:51`, `frontend/src/wailsjs/go/main/App.js:33`
**Issue:** This is an orphaned, hand-edited binding. Calling it rejects at runtime. It has no callers.
**Fix:** Remove both lines.

### IN-07: `daemonErr` is written and read without synchronisation (pre-existing)

**File:** `app.go:156`, `app.go:177-182`, `app.go:193-200`
**Issue:** `startup` and `RetryDaemon` write it, and `GetDaemonError` reads it, on separate Wails goroutines with no lock. In practice the calls are sequential, so impact is low.
**Fix:** Guard it with a small mutex, or use `atomic.Pointer[error]`.

---

_Reviewed: 2026-11-15T01:33:03Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
