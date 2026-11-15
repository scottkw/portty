---
phase: 01-plain-macos-terminal
reviewed: 2026-11-15T15:44:52Z
depth: standard
iteration: 5
files_reviewed: 5
files_reviewed_list:
  - TESTING.md
  - app.go
  - app_test.go
  - frontend/src/components/SettingsTab.tsx
  - frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx
findings:
  critical: 0
  warning: 3
  info: 10
  total: 13
  necessity: 1
status: issues_found
---

# Phase 1: Code Review Report (incremental re-review, iteration 5)

**Reviewed:** 2026-11-15T15:44:52Z
**Depth:** standard
**Files Reviewed:** 5. These are the files touched by commit 452cf7d4 ("fix(01): gate Start Web Server on Tailscale (NET-01, Amendment 3)"), reviewed as `git diff 61e02e80 HEAD` in context. Every other commit since 61e02e80 changes docs only.
**Status:** issues_found

## Summary

**Scope.** Commit 452cf7d4 does three things:
- It adds a Go guard: `StartWebServer` now returns the NET-01 sentence when `!h.Connected`.
- It adds a frontend gate: Start Web Server is disabled, with the NET-01 sentence as its title, while `tailscaleOff`, and Stop stays enabled.
- It adds one Go test, two vitest cases, and the matching TESTING.md updates.

To check the commit's claim that this "closes review IN-04", I traced how a web server can actually start: through `internal/daemon/process.go` (daemon auto-start and upgrader) and `internal/daemon/api.go` (`handleWebServerStart`, `AutoStartWebServer`).

**What the commit gets right:**
- The Go guard is real defence in depth. `GetTailscaleStatus` runs a fresh LocalAPI check (5 s timeout) instead of trusting the frontend. So the backend refuses even when the frontend thinks Tailscale is connected, or the first check is still pending (`tailscaleHealth === null`, where the button is enabled).
- The refusal goes into the shared emitter via `checkTailscaleHealth`. If it sees a change, the banner updates too.
- The title order (running → none, off → NET-01, undisclosed → CT) matches the disabled condition.
- `tailscaleOff` uses the same derivation as `App.tsx:133`.

**The gap.** The button gate is the only path this commit closes. The daemon still starts a LAN-mode server by itself at launch when Tailscale is off (WR-01). The gated button still starts a LAN-mode server when Tailscale is connected without HTTPS certificates (WR-02). So the situation IN-04 described can still happen: a server on the LAN, with its URL and password shown in Settings, under a banner that says "Remote access and sharing are off". Removing LAN mode entirely is Phase 4 work ("tailnet-only web link"). Amendment 3 and the TESTING.md NET-01 row ("no LAN fallback") claim more than the code delivers.

**Ground truth (run for this pass):**
- `go build ./... && go vet .`: clean.
- `go test -count=1 -run 'TestStartWebServer|TestCheckTailscaleHealth' .`: pass.
- `pnpm exec vitest run src/components/__tests__/SettingsTab.tailscale-status.test.tsx`: 6/6 pass.
- `pnpm exec tsc --noEmit`: clean.

### Previous findings (iteration 4), checked in the code

| ID (iter 4) | Verdict | Evidence |
|-------------|---------|----------|
| WR-01 (remote modal flips to "Local") | **Resolved** (eda1a433) | `HubPanel.tsx:528` passes `remote={modalState.isRemote}`. |
| WR-02 (frontend `KillSession` of stopped sessions, necessity) | **Open**, carried forward as WR-03 | `App.tsx:450`, `App.tsx:1228` are unchanged. |
| IN-04 (Start Web Server starts LAN mode while Tailscale is off) | **Partially resolved** | The button and the Go guard are in. The daemon auto-start path and the connected-without-certs path remain. See WR-01 and WR-02. |
| IN-01, IN-02, IN-03, IN-05, IN-06, IN-07 | **Open**, carried forward as IN-05..IN-10 | Their files were not touched. |

## Warnings

### WR-01: The daemon auto-starts a LAN-mode web server whenever Tailscale is off at launch, so the Amendment 3 gate never applies on the default path

**File:** `internal/daemon/process.go:114-145`, `frontend/src/components/SettingsTab.tsx:106`, `frontend/src/components/SettingsTab.tsx:598`, `TESTING.md:233`
**Issue:** `runDaemonCore` runs `webserver.CheckHealth`. When the result is not `Connected && HasCerts && IP != ""`, it calls `api.AutoStartWebServer(lanIP, 7443, "", "local", localPassword)`, which binds to the machine's LAN IP with Basic Auth (process.go:131-141). Here is what happens on a normal launch with Tailscale off, such as the office MacBook before Tailscale comes up:
1. The daemon is already serving on the LAN before the GUI renders.
2. SettingsTab loads `IsWebServerRunning() === true` (line 106) and shows **Stop Web Server**, which is enabled by design. The disabled Start button is never displayed.
3. Settings shows the LAN URL and, because `webServerMode === 'local'`, the LAN password block (line 598).
4. Meanwhile the NET-01 banner says "Remote access and sharing are off until it is."

This is the contradiction IN-04 described, and Amendment 3 says it is "closed in Phase 1". The new test `TestStartWebServerRefusedWhileTailscaleOff` and the TESTING.md row ("no server starts (no LAN fallback)") only cover the App-side RPC, so they give false confidence. The exposure is limited for now: Basic Auth applies, and per-session Share is disabled while `tailscaleOff`. It is still an unrequested listener on the office LAN, which goes against the project constraint "Remote access only over the user's tailnet".
**Fix:** Pick one, then keep the docs consistent with the choice.
- (a) Close it now. Drop the local-mode branch of the auto-start, and let the existing upgrader start the server once Tailscale is ready. `RestartWebServer` → `AutoStartWebServer` already handles the case where no server is running.
  ```go
  if h.Connected && h.HasCerts && h.IP != "" {
      if err := api.AutoStartWebServer(h.IP, 7443, h.Domain, "tailscale", ""); err != nil { ... }
  } else {
      fmt.Fprintf(os.Stderr, "daemon: tailscale not ready; web server waits for it (NET-01)\n")
      go upgradeToTailscale(ctx, api)
  }
  ```
- (b) Defer it to Phase 4. Then correct Amendment 3 and TESTING.md:233 / M-57 so they say only the GUI Start path is gated, and the daemon LAN auto-start remains until Phase 4.

### WR-02: With Tailscale connected but no HTTPS certificates, the gated Start button still starts a LAN-only server

**File:** `app.go:736-743`
**Issue:** The guard only refuses `!h.Connected`. When `Connected && (!HasCerts || IP == "")`, `StartWebServer` falls through to `client.StartWebServer("", port, "", "local", pwd)`. `handleWebServerStart` then resolves `webserver.GetLANIP()` and binds there (api.go:1028-1035). Each Tailscale tailnet has to opt in to HTTPS certificates, so a fresh tailnet hits this path. In that state:
- No banner shows.
- The button is enabled.
- The CT disclosure the user just acknowledged says Tailscale will provision a Let's Encrypt cert.
- Clicking Start gives a self-signed server reachable from the LAN and **not** from the tailnet IP.

That is the opposite of NET-01's intent, and of Phase 4's "tailnet-only web link". The new doc comment ("local mode when connected without") records this as intended, but nothing in Amendment 3 or REQUIREMENTS asks for it. The daemon auto-start has the same condition (WR-01).
**Fix:** Refuse unless the Tailscale-mode preconditions hold, and give a specific reason. Mirror it in the frontend gate (`tailscaleHealth.connected && !tailscaleHealth.hasCerts`):
```go
if !h.Connected {
    return errors.New(tailscaleOffReason)
}
if h.IP == "" || !h.HasCerts {
    return errors.New("Tailscale HTTPS certificates are off. Turn on HTTPS in the Tailscale admin console to start the web server.")
}
_, err := a.client.StartWebServer(h.IP, port, h.Domain, "tailscale", "")
return err
```
If LAN mode is kept until Phase 4 on purpose, say so in Amendment 3 and TESTING.md. Also add a Go test that injects `{Connected: true}` and asserts the chosen behaviour.

### WR-03: Already-defended-elsewhere: init/retryInit `KillSession` of stopped sessions duplicates the Go watcher (carried forward from iteration 4 WR-02, deliberately not fixed)

**File:** `frontend/src/App.tsx:450-452`, `frontend/src/App.tsx:1228-1230`
**Category:** necessity
**Issue:** `startup` starts the exit watcher unconditionally. `checkExits` runs immediately and then every 500 ms, and kills every `stopped` session. It survives `RetryDaemon`. The frontend copy is a second, concurrent killer of the same ids at launch, and whichever call loses logs a spurious `KillSession of stopped session failed`. Unchanged since iteration 4. Removing it is a human call.
**Fix:** Delete the two `allSessions.filter((s) => s.state === 'stopped').forEach(KillSession…)` blocks. Keep the restore filter.

## Info

### IN-01: Redundant `h.Connected &&` after the new guard

**File:** `app.go:736`
**Issue:** The line `if !h.Connected { return … }` directly above it makes the conjunct always true. That is harmless, but it reads as though a disconnected state could still reach this branch.
**Fix:** `if h.IP != "" && h.HasCerts {`.

### IN-02: The NET-01 sentence is a duplicated string literal in Go, kept in sync with the frontend only by a comment

**File:** `app.go:734`, `app_test.go:643`, `frontend/src/lib/tailscale.ts` (`TAILSCALE_OFF_REASON`)
**Issue:** The Go error and the Go test each hard-code the sentence. Nothing checks that it still matches the frontend constant, so a wording change on one side drifts silently. The UI shows the Go string (`serverError`) next to the frontend string (title and status line).
**Fix:** Add `const tailscaleOffReason = "…"` in Go and use it in both `StartWebServer` and the test. Optionally, add a vitest or Go test that reads both sources and compares them.

### IN-03: `TestStartWebServerLocalModeFallback` asserts nothing and depends on the host's Tailscale

**File:** `app_test.go:619-635`
**Issue:** The test passes on any outcome: error or success. It runs the real LocalAPI check, so its path differs between CI and a dev Mac. On a Mac with Tailscale connected and certs, it starts a real tailnet listener. Its name no longer describes what it covers. This commit edited its comment to "any of these errors is expected", which entrenches the no-assertion test.
**Fix:** Inject `tailscaleCheckFunc` returning `{Connected: true}` (no certs) and assert the WR-02 behaviour. Or rename the test to something like `TestStartWebServerSmoke` and gate it behind `testing.Short()`.

### IN-04: The Amendment 3 frontend tests do not cover "Stop stays available" or the pending-check state

**File:** `frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx:184-222`
**Issue:**
- No case has `IsWebServerRunning → true` with `tailscaleOff`. That case should assert that Stop is enabled with no title, which is the amendment's explicit "Stop stays available" rule.
- No case covers `tailscaleHealth: null`, where the button is enabled and the Go guard is the only defence.
- The second test's `title === ''` assertion also depends on the `HasCTDisclosure → true` default mock. That holds today, but the dependency is implicit.

**Fix:** Add the running+off case, and a null-health case asserting the button is enabled.

### IN-05: Stale comments that now contradict the code (carried forward)

**File:** `internal/daemon/engine.go:365-366`, `internal/pty/cleanup.go:37`, `frontend/src/components/SettingsTab.tsx:54`, `tray_objc_darwin.m:248`, `frontend/src/wailsjs/go/main/App.js:69`, `TESTING.md:216`, `app.go:1181-1185`
**Issue:** These lines mention:
- Linux/Windows exit detectors.
- "shell and tailscale path overrides". This is in the SettingsTab doc, which this commit edited around and left stale.
- "your session needs input".
- An empty auto-install header.
- The deleted `GetSessionStatus`.
- An unreachable `a.client == nil` startup branch.

**Fix:** Update or delete these comments and doc lines.

### IN-06: `QuitGUIOnly` counts active sessions by `Status != "stopped"`, a value `Status` never takes (carried forward)

**File:** `app.go:245`
**Issue:** `Status` is `running|exited|errored`. Impact is small, because the watcher removes stopped sessions within about 500 ms.
**Fix:** Use `s.State != "stopped"`.

### IN-07: The Linux Playwright fixture has no exit detection (carried forward)

**File:** `internal/pty/reap_other.go:1`
**Issue:** On Linux, PTY EOF does not surface, so fixture sessions never reach "stopped". No current e2e test depends on it.
**Fix:** Document this in `reap_other.go`, or add a fixture-side guard.

### IN-08: The traceability parser drops the second of two adjacent path cells (carried forward)

**File:** `tests/check-traceability-paths.sh:22`
**Issue:** `grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|'` consumes the closing pipe.
**Fix:** Split on `|` first (`awk -F'|' '{for(i=2;i<NF;i++) print $i}'`), then filter.

### IN-09: The `GetSessionQRCode` binding has no Go method (carried forward)

**File:** `frontend/src/wailsjs/go/main/App.d.ts:51`, `frontend/src/wailsjs/go/main/App.js:33`
**Issue:** This is an orphaned, hand-edited binding with no callers. Calling it would reject at runtime.
**Fix:** Remove both lines.

### IN-10: `daemonErr` is written and read without synchronisation (carried forward, pre-existing)

**File:** `app.go:156`, `app.go:177-182`, `app.go:193-200`
**Issue:** `startup` and `RetryDaemon` write it, and `GetDaemonError` reads it, on separate Wails goroutines with no lock.
**Fix:** Guard it with a small mutex, or use `atomic.Pointer[error]`.

---

_Reviewed: 2026-11-15T15:44:52Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
