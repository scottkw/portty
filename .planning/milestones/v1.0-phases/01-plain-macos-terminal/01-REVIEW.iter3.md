---
phase: 01-plain-macos-terminal
reviewed: 2026-11-15T01:22:19Z
depth: standard
iteration: 3
files_reviewed: 21
files_reviewed_list:
  - .github/workflows/build.yml
  - .github/workflows/release.yml
  - app.go
  - app_exit_test.go
  - app_test.go
  - cmd_cli.go
  - cmd_cli_test.go
  - frontend/src/App.tsx
  - frontend/src/components/Hub/HubPanel.test.tsx
  - frontend/src/components/Hub/HubPanel.tsx
  - frontend/src/components/__tests__/App.shellExit.test.tsx
  - frontend/src/components/__tests__/App.tailscale.test.tsx
  - internal/daemon/api.go
  - internal/daemon/api_test.go
  - internal/daemon/client.go
  - internal/daemon/client_test.go
  - internal/daemon/engine.go
  - internal/daemon/engine_natural_exit_test.go
  - internal/daemon/engine_test.go
  - internal/daemon/files_removed_test.go
  - internal/daemon/types.go
findings:
  critical: 0
  warning: 6
  info: 7
  total: 13
  necessity: 1
status: issues_found
---

# Phase 1: Code Review Report (re-review after fix iteration)

**Reviewed:** 2026-11-15T01:22:19Z
**Depth:** standard
**Files Reviewed:** 21 (the files touched by fix commits b436b10e..4a41c8ee; the other 178 files in the 199-file phase scope are unchanged since the previous review)
**Status:** issues_found

## Summary

**Scope.** This pass re-reviews the 9 fix commits (`git diff b436b10e~1..4a41c8ee`). I read each changed file in context, along with the code the fixes depend on:
- the daemon `KillSession` / `NativePTYBackend.Kill` / `killSession` path;
- `ListSessions` and the registry wiring;
- the natural-exit goroutine;
- the tray poller;
- the App.tsx init, retryInit, Hub-poll and remote-poll effects;
- `GetRemoteSessionsWithMeta`.

**Ground truth:**
- `go build ./... && go vet ./...`: clean.
- `go test -race -short -count=1 ./...`: every package passes.
- `pnpm exec tsc --noEmit`: clean.
- No references to the deleted `GetSessionStatus` / `StatusResponse` / `/sessions/{id}/status` remain in Go or TS code. One stale doc line remains in TESTING.md (IN-01).

### Status of the previous findings

| ID | Verdict | Evidence |
|----|---------|----------|
| CR-01 | **Resolved**, with one residual (WR-01 below) | `GetTailscaleStatus` → `checkTailscaleHealth` (app.go:1016-1018). `lastHealth` is read and written only under `healthMu` (app.go:1264-1275). The focus recheck no longer applies the reply (App.tsx:972-976). The poller now records every value the frontend has seen, so the transient-failure-then-recovery sequence emits. |
| WR-01 | **Resolved** | `seenSessions` records every listed id. An id that was seen and is now missing emits `session:exit` (app.go:330-346). This covers CLI kill and a daemon restart. |
| WR-02 | **Resolved** | The watcher kills stopped sessions itself. A failed kill leaves the id listed, and the next tick retries (app.go:336-339). The only error sources are "not found" during a concurrent kill (transient) and socket errors. `killSession` always returns nil (internal/pty/cleanup.go:20-81). |
| WR-03 | **Partially resolved** | `createTab` is guarded (App.tsx:671). The init and retryInit restore paths have the same race and are not guarded (WR-02 below). |
| WR-04 | **Resolved for local sessions** | HubPanel effect (HubPanel.tsx:300-304) plus the hubSessions filter in the exit handler (App.tsx:618). The same effect over-closes remote modals (WR-04 below). |
| WR-05 | **Resolved** | `a.client` is assigned once in `startup` before any poller goroutine starts (app.go:151). The `go` statement gives the pollers a happens-before edge, so their lock-free reads are sound. `RetryDaemon` only runs `EnsureDaemon` (app.go:193-200). The health poller starts exactly once. |
| WR-06 | **Resolved** | Route, handler, client method, engine method and `StatusResponse` are deleted. The negative route test discriminates: the old route answered 200 for unknown ids. |
| WR-07 | **Resolved** | `filepath.Abs` runs in the CLI's cwd. Extra positionals are rejected. Errors print as `agenthub new: <err>` (cmd_cli.go:71-97). |
| WR-08 | **Partially resolved** | `build`, `validate` and `build-macos` now have `timeout-minutes: 30`. `sign-macos` (macOS runner, `notarytool --wait` without `--timeout`) and `publish` still have none (WR-05 below). |

### Concurrency and ordering items you asked about

- **`checkTailscaleHealth` concurrency: acceptable.** The 5 s check runs outside the lock. The compare, store and emit all happen under `healthMu`, so emits are ordered the same way as `lastHealth`.
  - Out-of-order completion is still possible: a poller check that started earlier can finish after a fresher `GetTailscaleStatus` check and store a stale value.
  - That staleness is bounded. The next 10 s tick compares the real value against the stale `lastHealth` and emits.
  - Holding the mutex across `EventsEmit` only serialises health checks.
- **Exit-watcher seen-set: acceptable.** `seenSessions` is touched only by the single watcher goroutine (there is no longer a second start path) and by tests that call `checkExits` directly. A grep confirms there are no other accesses.
- **Kill retry: acceptable.** A user-initiated `KillSession` sets `StateStopped`, then waits up to 2-3 s in `killSession`, and only then does `e.registry.Remove` run.
  - During that wait, the watcher sees a "stopped" session. Its own kill gets not-found, and it retries every 500 ms until the id disappears, then emits once.
  - This costs a few cheap retries. It is not a leak or a double report.
  - Running `cmd.Wait` a second time after the natural-exit reap is sequential, not concurrent: `SetState(Stopped)` runs only after `ReapNaturalExit` returns.
- **Startup ordering: acceptable.** `a.client` now exists while `EnsureDaemon` runs.
  - `domReady` may dial and fail during that window. It then shows the window, which is the same outcome as the old nil-client path.
  - The pollers start after `EnsureDaemon` returns, whatever its result. While the daemon is down they fail quietly and retry.
- **Fixer residual 1, launch-time Tailscale overwrite: a finding (WR-01).** The window is narrow. The consequence is unbounded staleness, and a focus event can no longer repair it. The same window also exists on every `retryInit`, not only at launch.
- **Fixer residual 2, `console.warn` noise on every exit: a finding (WR-03).** It fires on natural exits and on every user tab close. It also makes a real `KillSession` failure look like expected noise.
- **Fixer residual 3, changed errors after a failed daemon start: acceptable.**
  - The frontend stops at `GetDaemonError()` before making any RPC.
  - The tray reports the error through `Health()` failing.
  - Only the error text changes, from "daemon not connected" to a socket dial error.
  - The `a.client == nil` branch in `refreshTrayState` is now unreachable in production, and its comment is stale (IN-01).

IN-01..IN-06 carry forward. The fixes resolved none of them. IN-01 gains two new stale references, and line numbers have been updated. IN-07 is new and covers pre-existing behaviour.

## Warnings

### WR-01: init/retryInit can overwrite a newer `tailscale:health` event, and a focus event can no longer repair it (CR-01 residual)

**File:** `frontend/src/App.tsx:438-451`, `frontend/src/App.tsx:1207-1220`, `app.go:1264-1275`
**Issue:** `init` and `retryInit` still apply the `GetTailscaleStatus()` reply (`setTailscaleHealth(health)`) after `Promise.all` resolves. That `Promise.all` also waits on `GetRelayPort`, `ListSessions` and `IsWebServerRunning`. The `tailscale:health` subscription (App.tsx:541) is active during that wait. Concrete sequence, most plausible at launch-at-login while tailscaled moves from `Starting` to `Running`:
1. init's check returns not-connected. `lastHealth` = NC, and NC is emitted.
2. A poller tick returns connected. `lastHealth` = C, C is emitted, and the UI shows C.
3. `Promise.all` resolves, and `setTailscaleHealth(NC)` overwrites the UI. The banner shows and Share is disabled.
4. Every later poll sees C == `lastHealth` and emits nothing.

CR-01 changed the focus recheck to *not* apply its reply, so step 5 below is new:

5. A focus or visibility event returns C == `lastHealth`, again emits nothing, and the UI stays at NC.

The stale state lasts until Tailscale actually changes state again. This is the CR-01 failure class with a narrower trigger. `retryInit` can run at any time after a daemon error, so the window is not limited to launch.
**Fix:** Apply the reply only if no event arrived while the request was in flight:
```tsx
const healthEventSeqRef = useRef(0)
// tailscale:health handler: healthEventSeqRef.current++; setTailscaleHealth(h)
// init / retryInit:
const seq = healthEventSeqRef.current
const [port, allSessions, running, health] = await Promise.all([...])
if (healthEventSeqRef.current === seq) setTailscaleHealth(health)
```
If `GetTailscaleStatus` itself emitted on change, the event already carried the same value, so skipping the reply loses nothing. Add a test that fires an event between the call and its resolution, and asserts the event value wins.

### WR-02: The WR-03 fix is incomplete. init/retryInit restore a tab for a session that exited while `Promise.all` was pending.

**File:** `frontend/src/App.tsx:438-456`, `frontend/src/App.tsx:511-518`, `frontend/src/App.tsx:1207-1236`
**Issue:** `exitedIdsRef` is consulted only in `createTab`. In `init`, the `session:exit` subscription is already live, because it is registered synchronously in the same mount effect after `void init()`. Sequence:
1. `ListSessions()` resolves with a running session S.
2. `Promise.all` keeps waiting on the other calls, including `GetTailscaleStatus()`, which can take up to its 5 s timeout.
3. S's shell exits. Within 500 ms the watcher kills S and emits `session:exit`, which is reported once.
4. The handler filters `tabs` and `hubSessions`, which do not contain S yet.
5. `Promise.all` resolves. `setHubSessions(sessions)` and `setTabs(restoredTabs)` install S: a tab bound to a session the daemon has already removed.

The watcher never reports S again, because its id has left `seenSessions`. `retryInit` (App.tsx:1207-1236) has the same shape. The dead tab stays until the user closes it by hand, and that close then hits the WR-03 not-found warn.
**Fix:** Filter the restore set in both paths:
```tsx
const sessions = allSessions.filter(
  (s) => s.state !== 'stopped' && !exitedIdsRef.current.has(s.id),
)
```
Also extend `App.shellExit.test.tsx` so it asserts the guard in both restore paths, not only in `createTab`.

### WR-03: Every `session:exit` now runs a redundant `KillSession` (and `ToggleWebServing`) that fails and logs `console.warn`, which hides real kill failures

**File:** `frontend/src/App.tsx:611-620`, `frontend/src/App.tsx:750-773`, `app.go:334-346`
**Issue:** The watcher emits `session:exit` only after the session has left the daemon list. The exit handler still calls the full `handleCloseTab`, which:
- calls `ToggleWebServing(id, false)` on a removed session when `webEnabled[id]`;
- calls `KillSession(id)`, which now always fails with not-found and logs `[App] KillSession failed:`.

This now happens for:
- **Every natural exit**: the watcher kills, emits, and the frontend kills again.
- **Every user tab close**: the user's close kills the session. The watcher then sees a seen id go missing and emits `session:exit`. `handleCloseTab` runs a second time, and its `KillSession` fails.

So every closed session produces one `console.warn`. A genuine failure of the user-initiated kill (socket timeout, daemon busy) prints the same line. That failure is exactly the case where the tab is removed while the session keeps running, and it is now buried in expected noise. This is CLAUDE.md's "silent fallback" concern, and this fix introduced it.
**Fix:** Split UI removal from the daemon kill. The exit handler does only UI work:
```tsx
const removeTabState = useCallback((id: string) => {
  setWebEnabled((prev) => { const n = { ...prev }; delete n[id]; return n })
  setTabs((prev) => { /* existing filter + adjacent-tab activation */ })
  setFontSizes((prev) => { const n = { ...prev }; delete n[id]; return n })
}, [activeId])
const handleCloseTab = useCallback(async (id: string) => {
  if (webEnabled[id]) { try { await ToggleWebServing(id, false) } catch { /* ignore */ } }
  try { await KillSession(id) } catch (err) { console.warn('[App] KillSession failed:', err) }
  removeTabState(id)
}, [webEnabled, removeTabState])
// session:exit handler: removeTabStateRef.current?.(data.sessionId)
```

### WR-04: The Hub modal auto-close treats absence from the best-effort remote list as "session gone", so an active remote terminal closes on a transient probe failure

**File:** `frontend/src/components/Hub/HubPanel.tsx:300-304`, `frontend/src/App.tsx:933-955`, `app.go:1074-1081`
**Issue:** The new effect closes the modal whenever its id is in neither `sessions` nor `remoteIdSet`. For local sessions that is correct. `hubSessions` changes only on exit/kill, and a failed local `ListSessions` keeps the last list.

For remote sessions, `remoteIdSet` is derived from `remotePeers`, which is a best-effort snapshot that collapses failures into emptiness:
- `GetRemoteSessionsWithMeta` returns `[]` when `ListTailnetPeers` errors (app.go:1078-1080). The remote poll then sets `remotePeers` to `[]` (App.tsx:941), and every remote modal closes.
- A peer whose metadata probe times out inside the 10 s budget comes back `Reachable: false` with no sessions. A modal on that peer's session closes, even though its terminal WebSocket may still be live.
- Any `tailscaleOff` reading runs `setRemotePeers([])` (App.tsx:934-936). That includes a transient one, and CR-01 now deliberately emits transient readings.

Before this fix, a remote modal survived these blips. Now the user's working remote terminal disappears with no explanation.
**Fix:** Record provenance when the modal opens, and auto-close only local modals, which have an authoritative list:
```tsx
setModalState({ session, sourceRect: rect, isRemote })   // in handleCardClick / handleCapAcquired
useEffect(() => {
  if (!modalState || modalState.isRemote) return
  if (!sessions.some((s) => s.id === modalState.session.id)) setModalState(null)
}, [sessions, modalState])
```
A remote modal can still close when its own WebSocket reports that the session has ended.

### WR-05: `sign-macos` and `publish` still have no `timeout-minutes`, and `notarytool --wait` has no timeout (WR-08 partially resolved)

**File:** `.github/workflows/release.yml:113-115`, `.github/workflows/release.yml:191-193`, `.github/workflows/release.yml:221-223`
**Issue:** `sign-macos` runs on `macos-latest`, which is billed at the macOS rate. It calls `xcrun notarytool submit ... --wait` with no `--timeout`. A stuck Apple notarization queue therefore holds the runner for the 6 h job default. That is the exact cost WR-08 was meant to bound. The fixer left it alone deliberately because notarization "can run long", but leaving it unbounded is a different choice from giving it a generous limit. `publish` (ubuntu) is also unbounded, though cheaper.
**Fix:**
```yaml
  sign-macos:
    runs-on: macos-latest
    timeout-minutes: 90
    # ...
          xcrun notarytool submit build/bin/agenthub-notarize.zip \
            --keychain-profile "agenthub-notarize" \
            --wait --timeout 60m
  publish:
    runs-on: ubuntu-latest
    timeout-minutes: 15
```

### WR-06: Already-defended-elsewhere: init/retryInit `KillSession` of stopped sessions duplicates the Go watcher and races it

**File:** `frontend/src/App.tsx:444-448`, `frontend/src/App.tsx:1213-1217`
**Category:** necessity
**Issue:** The repository shows that the frontend kill guards nothing:
- `startup` starts the exit watcher unconditionally (app.go:162).
- Its first `checkExits` runs immediately, then again every 500 ms (app.go:301-315).
- `checkExits` kills every session listed as `stopped` (app.go:336).
- `startup` is the only caller, and the watcher keeps running across `RetryDaemon` (app.go:193-200).

So every stopped session is removed by the watcher whether or not the frontend acts. The frontend copy adds a second concurrent killer for the same ids at launch. Whichever call loses logs `KillSession of stopped session failed`, and the watcher's exit event then triggers a third, failing kill through `handleCloseTab` (WR-03).
**Fix:** Delete the two `allSessions.filter((s) => s.state === 'stopped').forEach(KillSession…)` blocks. Keep the `state !== 'stopped'` filter on the restore set, together with the `exitedIdsRef` filter from WR-02.

## Info

### IN-01: Stale comments that now contradict the code (carried forward, plus two new)

**File:** `internal/daemon/engine.go:365-366`, `internal/pty/cleanup.go:37`, `frontend/src/components/SettingsTab.tsx:53-54`, `tray_objc_darwin.m:248`, `frontend/src/wailsjs/go/main/App.js:69`, `TESTING.md:216`, `app.go:1181-1185`
**Issue:**
- Carried: engine.go's "On Linux/Windows ReapNaturalExit is a no-op: the platform exit detector already cached the code". No exit detector exists on any platform, and `reap_other.go` is untagged.
- Carried: cleanup.go's "a racing exit detector (Linux)".
- Carried: the SettingsTab doc says "shell and tailscale path overrides". Both were removed.
- Carried: tray_objc's "your session needs input".
- Carried: App.js has an empty `// Auto-install Tailscale bound method` header.
- **New (WR-06 fix):** TESTING.md:216 still describes `TestDerivedStatus_ExitedErrored` as checking "both `GetSessionStatus` and `ListSessions`". `GetSessionStatus` is deleted, and the test now checks only `ListSessions`.
- **New (WR-05 fix):** `refreshTrayState`'s `if a.client == nil { // Startup failed ... }` (app.go:1181-1185) no longer describes a production state. `startup` assigns `a.client` before `startTrayPoller`, even when startup fails. The tray shows the error icon through `Health()` failing.

**Fix:** Update or delete these comments and doc lines.

### IN-02: `QuitGUIOnly` counts active sessions by `Status != "stopped"`, a value `Status` never takes (carried forward)

**File:** `app.go:244`
**Issue:** `Status` is now `running|exited|errored`, so exited sessions count as "active". Impact is smaller than before, because the watcher now removes stopped sessions itself within about 500 ms.
**Fix:** Use `s.State != "stopped"`.

### IN-03: The Linux Playwright fixture has no exit detection at all (carried forward)

**File:** `internal/pty/reap_other.go:1`, `internal/pty/native.go`
**Issue:** The Linux Wait4 detector was deleted, and `reap_other.go` lost its build tag. On Linux, PTY master EOF does not surface, so fixture sessions never reach "stopped". The new watcher-driven kill therefore never fires there either. No current e2e test depends on it.
**Fix:** Document this limitation in `reap_other.go`, or add a fixture-side guard.

### IN-04: "Start Web Server" with Tailscale off starts LAN mode while the banner says sharing is off (carried forward, T-01-42-adjacent)

**File:** `app.go:725-740`, `frontend/src/components/SettingsTab.tsx:301-321`
**Issue:** When not connected, `StartWebServer` silently falls back to `local` mode, which exposes the server on the LAN. The persistent banner meanwhile reads "Remote access and sharing are off". `StartWebServer` now also feeds the shared health state, because it goes through `GetTailscaleStatus` → `checkTailscaleHealth`. That is consistent, and it changes nothing about this item. This is known Phase 4 work.
**Fix:** Defer to Phase 4. At minimum, disable "Start Web Server" while `tailscaleOff`.

### IN-05: The traceability parser drops the second of two adjacent path cells (carried forward)

**File:** `tests/check-traceability-paths.sh:22`
**Issue:** `grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|'` consumes the closing pipe, so in `| a.go | b.go |` the second cell never matches.
**Fix:** Split on `|` first (`awk -F'|' '{for(i=2;i<NF;i++) print $i}'`), then filter.

### IN-06: The `GetSessionQRCode` binding has no Go method (carried forward)

**File:** `frontend/src/wailsjs/go/main/App.d.ts:51`, `frontend/src/wailsjs/go/main/App.js:33`
**Issue:** This binding is an orphan that this phase hand-edited. Calling it rejects at runtime. It has no callers.
**Fix:** Remove both lines.

### IN-07: `daemonErr` is still written and read without synchronisation (pre-existing, adjacent to WR-05)

**File:** `app.go:156`, `app.go:177-182`, `app.go:193-200`
**Issue:** WR-05 made `a.client` write-once. `a.daemonErr` is still written by `startup` and `RetryDaemon` and read by `GetDaemonError`, each on a different Wails-dispatched goroutine, with no lock. No current test exercises these calls together, so `-race` cannot see it. In practice the calls are sequential (init calls `GetDaemonError` once, then retryInit calls `RetryDaemon`), so impact is low.
**Fix:** Guard `daemonErr` with a small mutex, or store it in an `atomic.Pointer[error]`.

---

_Reviewed: 2026-11-15T01:22:19Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
