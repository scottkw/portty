---
phase: "09"
slug: "upgrades-keep-sessions"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-20"
---

# Phase 09 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 09-01..09-07-PLAN.md (T-09-01..T-09-38, plus T-09-SC, which every plan repeats and which is counted once here). Each mitigation was checked against the code at `afe46fd9` (Phase 9 complete). `git diff --stat afe46fd9..HEAD -- . ':!.planning'` is empty at audit time, so the working tree is the audited code. The phase's code diff is `3be899dd..afe46fd9`: `b4d63644^..3be899dd` touches only `.planning`. 09-RESEARCH.md (Pitfalls 1-13, Security Domain), 09-REVIEW.md (both iterations) and 09-REVIEW-FIX.md were read, and every review fix that a mitigation depends on was traced into the code (CR-01, WR-01, WR-02 it2, WR-05, IN-01, IN-02).

Config: security enforcement on, ASVS level 1, block on `high`. The high-severity handoff threats were also checked at the boundary where each control sits, which is ASVS L2 depth. Examples: the uid check comes before any freeze or fd, the route is reachable only through the unix listener, and the fault seam is absent from the release binary. The audit verified that the declared mitigations exist. It did not scan for new threats. Observations found along the way are listed separately and are non-blocking.

Commands run (no test suite was run, by the user's rule on test binaries and firewall prompts):
- `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...`: exit 0.
- `go build -tags vmpreview ./...` and `go vet -tags vmpreview ./internal/daemon/`: exit 0.
- `go list -tags wailsassets -f '{{.IgnoredGoFiles}}' ./internal/daemon/ .`: all five `*_vmpreview.go` files are ignored.
- A default build and a `-tags vmpreview` build of the main package (in the scratchpad), then `strings`. Release binary: `PORTTY_VMPREVIEW_HANDOFF_FAULT` 0, `PORTTY_VMPREVIEW_MIN_DAEMON_API` 0, `handoff-fault:` 0. vmpreview binary: 2, 1, 3.
- `bash tests/check-brand.sh`: "OK: brand gate clean".
- Greps of the phase 9 docs for caps, tokens, tailnet keys and query-string URLs: 0 matches.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| takeover process → old daemon over the daemon unix socket | The peer receives every PTY master (shell write access), the API, relay and share listeners, and the daemon lock | fds (SCM_RIGHTS), HubState, share grant ids and tokens |
| old daemon memory → new daemon memory | Terminal contents (which can hold secrets) cross the socket only | ring, history, rendered screen seed |
| kernel → daemon | Peer uid/pid (LOCAL_PEERCRED, LOCAL_PEERPID), exit status (kqueue), fd rights | identity, process lifecycle |
| tailnet / loopback TCP → relay and share listeners | The listeners move between processes; TLS, the bind check and the token gate must not change, and the handoff route must not be reachable here | WebSocket frames, share links |
| release build → fault hooks | Env-driven faults must not exist outside the `vmpreview` tag | process control |
| GUI/CLI → takeover child | The status pipe carries the outcome that decides whether the user is asked to end sessions | one outcome byte |
| takeover daemon → its shells | The status fd and variable must not leak into sessions | fd 3, env |
| macOS (loginwindow, signals) → GUI | Terminate requests and SIGTERM decide whether the GUI quits | quit decision |
| executor → host / VMs; evidence file → repository | Daemons, kills, faults and clicks run only in the Tart VMs; evidence must hold no tokens | logs |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation (evidence at afe46fd9) | Status |
|-----------|----------|-----------|----------|-------------|-----------------------------------|--------|
| T-09-01 | Denial of service | EnsureDaemon cancel path (process.go) | high | mitigate | The only kill is in `startDaemon` (process.go:437-443). It runs only for the fresh `startDetachedDaemon` child this call spawned, after `waitForDaemon` failed under a cancelled ctx. The kill func is a no-op once the child is reaped (process_unix.go:41-46). `startDetachedTakeover` has no kill func (process_unix.go:57-59). `handOff` returns on `ctx.Done` without signalling (process.go:374-375). Test `TestStartDaemon_CancelAfterSpawnKillsChild` (process_test.go:714) | closed |
| T-09-02 | Denial of service | Quit All with a frozen daemon (client.go) | medium | mitigate | `doRetrying` (client.go:321-341) retries only a non-GET 503 that carries Retry-After, waits at most 2 s per retry, and stops at the deadline. `frozenRetryBudget = 20 s` (client.go:312) is greater than `maxFrozen()` = 19 s (handoff.go:90-92); `TestHandoff_TimeoutsFitClientRetry` (handoff_fault_test.go:784) pins this. `ShutdownDaemon` uses one deadline for both posts (client.go:263). `QuitAll` ignores the error and quits (app.go:736-745) | closed |
| T-09-03 | Tampering | swizzled applicationShouldTerminate (terminate_darwin.m) | medium | mitigate | The method is replaced once (guard `originalShouldTerminate != NULL`, terminate_darwin.m:58-60). The original IMP is kept (line 70) and called for every non-system terminate (line 50). `NSTerminateNow` is returned only for `powerOffSeen` or a loginwindow quit event whose reason is logout, restart or shutdown (lines 21-41, 46-48). Real-AppKit proof: 09-VM-CHECKS V-05d PASS (an `osascript` restart rebooted the office VM with the GUI open) | closed |
| T-09-04 | Repudiation | update-needed and couldn't-move copy | low | accept | The copy names versions only: App.tsx:1084 (`version ${updateNeeded}`), App.tsx:1364, and the fixed strings of `moveFailedReason` (handoff.go:105-117). See AR-09-01 | closed (accepted at plan time) |
| T-09-05 | Elevation | SIGTERM handler (app.go) | low | mitigate | `onTerminate` (app.go:660-667) only sets `quitting` and quits the GUI, with no `ShutdownDaemon`. It is wired at app.go:276-282. `onSystemPowerOff` (app.go:672-674) also only sets the flag. Test `TestOnTerminate_QuitsGUIOnly` (app_test.go:1404) | closed |
| T-09-06 | Spoofing | fdpass.PeerUID / PeerPID | high | mitigate | `PeerUID` reads `LOCAL_PEERCRED` through `GetsockoptXucred` (fdpass.go:115-125). `PeerPID` reads `LOCAL_PEERPID` (fdpass.go:128-138). Both read the kernel socket inside `SyscallConn().Control`. In the consumer, `serveHandoff` refuses any uid other than `os.Getuid()` (handoff.go:457-460), before `beginHandoff`, the freeze or any fd. It then watches the attested pid with `watchPeerExit(pid)` (handoff.go:469-481). Tests `TestPeerUID`, `TestPeerPID` (fdpass_test.go:126, 139) | closed |
| T-09-07 | Denial of service | fdpass.Recv | medium | mitigate | The control buffer is at least 4 KiB (fdpass.go:68). On `MSG_CTRUNC`, a parse error or a count mismatch, every parsed fd is closed and an error returned (fdpass.go:90-103). Received fds are marked close-on-exec (fdpass.go:107). `recvFDs` closes them on a malformed header (handoff.go:303-305). Tests `TestRecv_TruncatedClosesFds`, `TestRecv_CountMismatch` (fdpass_test.go:92, 110). Residual: O-1 | closed |
| T-09-08 | Tampering | session I/O (native.go, session.go) | high | mitigate | `Session.Read` and `Write` go through `rw()`, which returns the pollable dup `ioFile` (session.go:131-141, 165-181). `pollableDup` makes a non-blocking, close-on-exec dup with `F_DUPFD_CLOEXEC` inside Control (native.go:142-160). Both spawned sessions (native.go:47) and adopted ones (adopt_darwin.go:33) use it. Tests `TestSession_FreezeIdleRead`, `TestSession_FreezeTrulyIdleShell`, `TestSession_LargeWriteBlocksNotEAGAIN` (native_test.go:332, 390, 419) | closed |
| T-09-09 | Elevation | adopted kill (cleanup.go) | high | mitigate | `killSession` sends adopted sessions to `killAdopted` (cleanup.go:21-22). `killAdopted` signals `-pid` only while `s.exited`, the kqueue watcher, is still open (cleanup.go:97-120). `Adopt` and `takeover.invalid` refuse pid <= 1 (adopt_darwin.go:28, handoff.go:1071). Tests `TestKillAdopted` (adopt_test.go:272), `TestAdopt_RefusesPidOne`, `TestHandoff_RejectsPidOne`. Residual: O-3 | closed |
| T-09-10 | Tampering | share listener refactor (webserver/server.go) | high | mitigate | Both start paths go through `StartOn`: `Start` (server.go:360-370) and the handoff's `commit` (handoff.go:1246). `StartOn` runs `CheckListener` (TCP, bind IP, port; server.go:344-356) and then `assertTailnetBindIP` (server.go:379-384). `git diff --stat 3be899dd..afe46fd9` is empty for csp_mw.go, capability_mw.go, origin_mw.go, whois.go, internal/tailnet, internal/capability and frontend/index.html. `/meta`'s `sessionMetaItem` is untouched; `api` (omitempty) was added only to `/info`'s item | closed |
| T-09-11 | Tampering | NewHubFromState | high | mitigate | `NewHubFromState` runs `st.validate()` before `NewHub` and returns an error, and no hub, on any failure (handoff_state.go:190-194). The validation covers: ring max, length, end and truncation; the geometry offset; history caps and wrap flags; the wrap set; grid and emulator sizes (1..1000); the region; emulator-less state; tail length; charset; parser state; and open-link safety (handoff_state.go:354-419). The takeover maps an import error to `fail(takeoverImport)`, which writes ABORT (handoff.go:900-902, 1163-1169). Test `TestHandoffState_ImportValidates` (handoff_state_test.go:613); stream round-trip `TestHandoffRoundTrip_Streams` | closed |
| T-09-12 | Information disclosure | HubState | high | mitigate | The doc comments say so: handoff_state.go:49-51 ("moves memory to memory, never to a file") and history.go:17-20. handoff.go, handoff_state.go and fdpass.go contain no `os.Create`, `WriteFile`, `OpenFile` or `CreateTemp`. The state is gob-encoded straight onto the hijacked unix conn (handoff.go:664, 679-683). Each of the 25 `log.Printf` calls in handoff.go formats only step names, counts, pids, durations, codes or error text. Session ids appear only as `relay.DiagID` | closed |
| T-09-13 | Denial of service | Freeze | high | mitigate | `Freeze` fails at once when the reader has no `SetReadDeadline` (hub.go:646-649) or setting it errors (hub.go:665-667). It waits at most `freezeTimeout = 2 s` and then unfreezes (hub.go:632, 674-676), and it returns `ErrHubEnded` when Done closes (hub.go:672-673). Run's deadline path closes `h.frozen`, never Done (hub.go:611-621). The old side freezes hubs concurrently (handoff.go:503-507) and rolls back on any error other than an ended hub (handoff.go:518-521). Tests `TestHub_FreezeWithoutDeadline`, `TestHub_FreezeThaw`, `TestHub_FreezeWhileEnding` | closed |
| T-09-14 | Repudiation | viewer close at commit | medium | mitigate | `sub.Restart` closes with `StatusServiceRestart` (1012) and the fixed `CloseReasonServiceRestart` (relay/server.go:95-99, close_reasons.go:18-21). `RestartViewers` prefers it and leaves Done open (hub.go:446-467). `IsTerminalClose` is true only for 1000 with "share stopped" or "session ended" (close_reasons.go:27-31). `finish` calls `RestartViewers`, never `Hub.Shutdown` (handoff.go:760-766). Test `TestRestartViewers_Close1012` (server_test.go:500) | closed |
| T-09-15 | Spoofing / Elevation | handoff route (handoff.go) | high | mitigate | At handoff.go:457-460, a `peerUID(c)` error or a uid other than `os.Getuid()` gets REFUSE and a return. This happens before the version check, `beginHandoff` (line 474), the freeze (line 495) and the first `sendFDs` (line 534). The HELLO read before it is capped at 4 KiB (`maxHelloFrame`, handoff.go:172, 448). Test `TestHandoff_RefusesOtherUID` (handoff_test.go:715) asserts no freeze, no watch and an unchanged fd count | closed |
| T-09-16 | Elevation | route exposure | high | mitigate | The route is registered only on `a.mux` (api.go:199). `a.handler = freezeGate(a.mux)` (api.go:114) is served only by `http.Serve` on the unix listener, in `Start` (api.go:313) and `serveOn` (api.go:295). For `serveOn`, the takeover first checks that the listener is the daemon socket (handoff.go:1054-1056). The relay serves its own mux (relay_remote_files.go:21-34), and the share server serves its own `ws.mux`. `handleHandoff` also drops any connection that is not a `*net.UnixConn` (handoff.go:421-425). `API.Handler()` has no production caller. Test `TestHandoff_RouteOnlyOnUnixSocket` (handoff_test.go:684) | closed |
| T-09-17 | Tampering / DoS | frame codec and import | high | mitigate | `readFrame` checks the length against a per-frame cap before it allocates (handoff.go:256-260). The caps: HELLO 4 KiB, READY about 1 MiB (`maxReadyFrame`), SESSION `relay.HandoffStateMaxBytes` (18.4 MB), others 64 MiB (handoff.go:165-174). `writeFrame` applies the SESSION cap on the sender too (handoff.go:235-241). HELLO is versioned (handoff.go:461-464). Imports are limited to 256 sessions (handoff.go:864-866). Session metadata must be at most 4 KiB, with matching ids, no duplicates and pid > 1 (handoff.go:1065-1077); T-09-11 covers the hub state. Every error before COMMIT ends in `fail` or `readFailed` (ABORT, then cleanup) on the new side and in `rollback` on the old. Tests `TestHandoff_HelloCapped`, `TestHandoffState_FitsSessionFrameCap` | closed |
| T-09-18 | Tampering | PTY reader invariant | high | mitigate | Staged hubs are only stored with `AddStaged`, not run (engine.go:350-353). `Hub.Start` is called only in `commit` (handoff.go:1254-1257), and the takeover serves nothing before `commit` (`serveOn` at line 1244). After COMMIT the old side thaws only on `<-peerGone` (handoff.go:575-576, 586-595) and checks it again in `finish` (handoff.go:750-756). On the takeover, the watchdog exits only through `CAS(pending→dead)` (handoff.go:1230-1236, 1263-1266). A failed COMMITTED write while the old pid is alive leads to `die` (handoff.go:1273-1282). Tests `TestHandoffFault_NoThawUntilTakeoverExits`, `TestHandoffFault_WatchdogAfterCommittedWritten`, `TestHandoffFault_TakeoverGoneAtFinish` | closed |
| T-09-19 | Denial of service | socket unlink after the handoff | high | mitigate | `closeListenerCopies` calls `SetUnlinkOnClose(false)` before Close (handoff.go:387-389), and the takeover does the same on the received listener (handoff.go:1030). `finish` never calls `api.Stop`; it ends in `handoffExit(0)` (handoff.go:748-783). The SIGTERM guard `waitHandoff(maxFrozen())` returns before `api.Stop` once the daemon has handed off or committed (process.go:175-179, handoff.go:368-381). Test `TestHandoff_SIGTERMAfterCommitKeepsSocket` (handoff_fault_test.go:681) | closed |
| T-09-20 | Information disclosure | daemon.log | medium | mitigate | The log review is the same as for T-09-12: no handoff log line formats a token, cap, session name, workDir or terminal content, and `hello.Build` is `%q`-quoted. Test `TestHandoff_NoSecretsInLog` (handoff_test.go:937) and the HappyPath log check (handoff_test.go:620) | closed |
| T-09-21 | Tampering | fault hooks | high | mitigate | `handoffFault` is nil by default (handoff.go:127). It is assigned only in handoff_vmpreview.go:34 (`//go:build vmpreview`) and in one `_test.go` file. `go list -tags wailsassets` lists all five `*_vmpreview.go` files as IgnoredGoFiles. The default-build binary contains none of `PORTTY_VMPREVIEW_HANDOFF_FAULT`, `PORTTY_VMPREVIEW_MIN_DAEMON_API` or `handoff-fault:`; the vmpreview binary contains 2, 1 and 3. Release builds use `-tags wailsassets` only (release.yml:96, build.yml:78, build.sh:88). `tests/check-brand.sh` fails on the tag in .github, build.sh or scripts, tracked or untracked, and runs at build.yml:27 and release.yml:23; this run: "OK: brand gate clean". Test `TestHandoffFault_SeamNilInRelease` (handoff_fault_test.go:799) | closed |
| T-09-22 | Denial of service | COMMIT-to-COMMITTED window | low | accept | `shortcut:` comment at handoff.go:21-24. COMMIT goes out only after the READY digests match (handoff.go:553-573). See AR-09-02 | closed (accepted at plan time) |
| T-09-23 | Tampering | downgrade pull-back | medium | mitigate | `olderRelease(hello.Build, BuildVersion)` leads to REFUSE (handoff.go:465-468; the comparison is at 316-344). The GUI then gets the DEBT-08 question ("the running version refused"). Test `TestHandoff_RefusesOlderRelease` (handoff_test.go:733). Residual: O-5 | closed |
| T-09-24 | Information disclosure | share links after the handoff | high | mitigate | The old side sends grant id and token verbatim, for the frozen sessions only (handoff.go:632-638). The takeover reloads the signing key from capability.key (`BootstrapCapabilityState`, handoff.go:1099; api.go:239-249) and keeps records only for sessions it imported (handoff.go:1102-1107). It builds the share server with the same constructor (share.go:35-37) and wiring (share.go:268-313: `SetSigningKey`, `EnableSession`, `AddGrant`). The `onExit` share cleanup is re-attached to each adopted session (handoff.go:1253-1257, engine.go:278-303, api.go:433-449). Key regeneration and share-off are drained before the snapshot (api.go:131-169, share.go:103-108). Test `TestHandoff_HappyPath` asserts the same token (handoff_test.go:565) and that a wrong token is refused (handoff_test.go:571-574); also `TestHandoff_FreezeGateDrainsWrites` | closed |
| T-09-25 | Tampering | stale import after a quick rollback | medium | mitigate | `rollback` writes ABORT (handoff.go:708) before `resume()` and Thaw (handoff.go:710). The takeover stops at once on a received ABORT (`readFailed`, handoff.go:995-998), and `lost` reads any pending ABORT first (handoff.go:1177-1182). Test `TestHandoffFault_AbortBeforeThaw` (handoff_fault_test.go:361) | closed |
| T-09-26 | Denial of service | takeover stopped by SIGSTOP after COMMIT | low | accept | The old side waits on `<-peerGone` with no timer (handoff.go:575, 592), as the 09-04 threat model records. See AR-09-03 | closed (accepted at plan time) |
| T-09-27 | Denial of service | Quit All during a takeover (process.go) | high | mitigate | The takeover has no kill path: `startDetachedTakeover` returns only the pid and the outcome (process_unix.go:57-59). On `ctx.Done`, `handOff` returns `ctx.Err()` (process.go:374-375). A /shutdown during the freeze is retried (T-09-02). Test `TestEnsureDaemon_CancelNeverKillsTakeover` (process_test.go:1100) | closed |
| T-09-28 | Information disclosure / Tampering | status fd and variable (process.go) | medium | mitigate | `RunTakeover`'s first statement is `takeoverStatus()` (process.go:45). It runs `os.Unsetenv(PORTTY_TAKEOVER_STATUS_FD)` (line 78), refuses fd < 3 (line 80), and marks the fd close-on-exec (line 83). Run by hand, with no variable set, it returns nil and `reportTakeover` writes nothing (process.go:90-93). In the VM, 09-VM-CHECKS I-01 records `env=0 fd3=closed` for a shell of A, for the same adopted shell afterwards, and for a shell B started. Test `TestTakeoverStatus_FromEnv` (process_test.go:800) | closed |
| T-09-29 | Denial of service | a decision made on a timer during a slow commit | high | mitigate | `handOff` decides only from `<-outcome`; the `note` timer only prints (process.go:365-377). A committed outcome never asks (process.go:380-387). A daemon that answers as this build counts as moved (process.go:397-399). Tests `TestEnsureDaemon_SlowCommitNoQuestion`, `TestEnsureDaemon_NoWallClockDecision` (process_test.go:1018, 1036) | closed |
| T-09-30 | Tampering | test binaries | medium | mitigate | `spawnTakeover` refuses under `testing.Testing()` (process.go:252-257), as the fresh spawn does (process.go:332-334). Test `TestEnsureDaemon_TakeoverRefusedFromTestBinary` (process_test.go:1184) | closed |
| T-09-31 | Tampering | host machine (09-06) | high | mitigate | `TestHandoffIT` skips unless `PORTTY_HANDOFF_IT=1` (handoff_it_test.go:33-34). 09-VM-CHECKS I-01..I-03 ran in the office Tart VM. The 09-06-SUMMARY threat flags record that the host ran builds, one skipping `-run` and the gate only | closed |
| T-09-32 | Information disclosure | 09-VM-CHECKS I-rows | medium | mitigate | 09-VM-CHECKS.md line 3 states that the integration test never shares a session. At afe46fd9 the file has 0 matches for `cap=[A-Za-z0-9_-]{6,}`, `tskey-`, `token=[A-Za-z0-9_-]{8,}` or a URL with a query string. Every token-like string of 40 or more characters is the 64-hex sha256 of a binary | closed |
| T-09-33 | Spoofing | host tab and local attach reconnect | medium | mitigate | `hostReconnectVerdict` stops with `ended` unless the session is listed and not stopped, and retries when a checked read fails (hostReconnect.ts:19-31). The local attach re-auth ends when the session is missing or stopped (cmd_attach.go:127-148). Tests hostReconnect.test.ts and `TestAttachLocal_Reauth` (cmd_attach_test.go:749) | closed |
| T-09-34 | Denial of service | local attach re-auth | low | mitigate | `ensureLocalDaemon` runs at most once per re-auth, and only after a failed list (cmd_attach.go:129-136). The attach loop's backoff paces the re-auth (`Reauth: target.reauth`, cmd_attach.go:407) | closed |
| T-09-35 | Tampering | host machine (09-07) | high | mitigate | The 09-VM-CHECKS.md cleanup notes (line 113) say: "Nothing on the host was clicked, killed or launched; every SIGSTOP/SIGCONT, click and the reboot ran inside the office VM". The host-side steps are builds only (lines 46, 101, 118) | closed |
| T-09-36 | Information disclosure | 09-VM-CHECKS.md | high | mitigate | Line 43: share checks record HTTP status codes only. Line 86: `/tmp/s1.url` and `/tmp/b86.url` were deleted on both VMs. The T-09-32 grep, run over every file in the phase directory, finds 0 cap, token, tskey or query-string URL matches | closed |
| T-09-37 | Information disclosure | Safari caches in the home VM | medium | mitigate | 09-VM-CHECKS.md line 85 (and line 194 for the earlier run): the WebDriver session was ended, the SafariAutomation dir removed, the favicon row deleted and the DB vacuumed | closed |
| T-09-38 | Tampering | VM state after fault rows | low | mitigate | 09-VM-CHECKS.md line 88: settings.json restored, `cmp` clean after the HEAD launch. Line 110: cleanup.sh run and HEAD deployed. vmpreview builds were used only for the fault and update-needed rows (lines 46, 101) | closed |
| T-09-SC | Tampering | npm/pip/cargo installs (all seven plans) | high | mitigate | `git diff 3be899dd..afe46fd9` of go.sum, frontend/package.json and frontend/pnpm-lock.yaml is empty. go.mod only moves `golang.org/x/sys v0.46.0` from indirect to direct, with no version change | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Constraint Checks

| Constraint | Result | Evidence |
|------------|--------|----------|
| Share links stay token-gated and tailnet-only | holds | The capability, origin, CSP and WhoIs middleware and `internal/tailnet` have no diff in `3be899dd..afe46fd9`. A received share listener must be on the takeover's own Tailscale health IP and port 7444 before COMMIT (`CheckListener`, server.go:344-356; called at handoff.go:1125 and server.go:379). The relay listener must be loopback (handoff.go:1057-1059). Tokens move unchanged and the key is reloaded from disk (T-09-24) |
| CSP unchanged | holds | `frontend/index.html` and `internal/webserver/csp_mw.go` have no diff in `3be899dd..afe46fd9`; no diff line in the phase mentions Content-Security-Policy |
| Only same-UID processes can take over | holds | `LOCAL_PEERCRED` uid equals `os.Getuid()` before anything is frozen or sent (handoff.go:457-460). The route is reachable only on the unix socket (T-09-16) |
| `/health` api field | holds, no exposure | `HealthResponse{Status, Version, API, Handoff}` (types.go; api.go:371), served only on the unix socket mux. On the share server the `api` field appears only on the capability-gated `/api/sessions/{id}/info` |
| Digest | holds | Both sides hash the SESSION frame bytes as they cross the socket (`writeSessionFrame` and `readSessionFrame`, handoff.go:679-689). `checkDigests` requires exactly one matching digest per frozen session (handoff.go:691-701), and a mismatch rolls back before COMMIT (handoff.go:558-561) |

---

## Unregistered Flags

None. 09-01-SUMMARY raised two `threat_flag` entries, and both map to registered threats:
- `process-signal` (process_unix.go) maps to T-09-01.
- `appkit-terminate` (terminate_darwin.m) maps to T-09-03, and the VM check it asked for is 09-VM-CHECKS V-05d (PASS).

09-02..09-07-SUMMARY report no flags beyond their registers.

---

## Observations (outside the register, non-blocking)

These came up while verifying the threats above. None is a missing declared mitigation, and none was given a threat ID.

- **O-1 (residual on T-09-07):** `fdpass.Recv` closes only the fds it parsed. If `ParseSocketControlMessage` fails (it returns no messages) or `ParseUnixRights` fails partway (the `break` at fdpass.go:84-85), fds in the unparsed control messages stay open. The kernel builds the cmsg buffer, so a malformed one is not expected. The only receiver, the takeover, exits on any failure (process.go:65-67), which releases them.
- **O-2 (documentation accuracy):** the plans and code comments describe the daemon socket as 0600, but `listenDaemonSocket` is a plain `net.Listen` (ipc.go:10-12). On this host the socket is `srwxr-xr-x`, and confinement comes from the 0700 `~/Library/Application Support` and `portty` directories (socket.go:20 only creates the dir; it does not tighten an existing one). Phase 9 did not change this. The `LOCAL_PEERCRED` uid check does not depend on file mode, so T-09-15 holds either way. The takeover does not check the old daemon's peer credentials; it trusts whatever listens at `DefaultSocketPath()` (handoff.go:953-981), which only the same uid can bind.
- **O-3 (residual on T-09-09):** received session fds are not checked to be a PTY pair, and the pid is checked only to be > 1. The `getpgid(pid) == pid` check from review IN-02 was not added (REVIEW-FIX explains why). Pitfall 9's safety at adopt time rests on the exited shell being an unreaped zombie of the frozen old daemon. On a second upgrade, a shell that the previous takeover adopted has launchd as its parent and is reaped at once, which leaves a pid-reuse window of about a second between the freeze and `WatchExit`. Reuse would also need a wrap of the pid space. The sender is a same-uid process on the daemon socket.
- **O-4:** `CheckListener` skips the IP comparison when `BindIP` does not parse (server.go:349). On the handoff path `BindIP` is the takeover's own Tailscale health IP (share.go:36) and must be non-empty (handoff.go:1115), so skipping it would need tailscaled to report something that is not an IP.
- **O-5 (residual on T-09-23):** `olderRelease` treats unparseable versions (e.g. `dev`) as never older (handoff.go:325-329), and `hello.Build` is self-reported. A test pins this as intended. A real older release always carries a parseable version.
- **O-6:** the takeover takes the old daemon's pid from the HELLO-OK payload (handoff.go:856) rather than from `LOCAL_PEERPID`. That pid drives only the takeover's "the old daemon is gone" decisions (`lost`, a failed COMMITTED write). A real old daemon sends `os.Getpid()` (handoff.go:482).

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-09-01 | T-09-04 | The update-needed and couldn't-move copy names versions only, with no tokens, caps or session contents (App.tsx:1084, App.tsx:1364, handoff.go:105-117) | 09-01 plan-time threat model (low severity) | 2026-11-19 |
| AR-09-02 | T-09-22 | A crash between COMMIT and COMMITTED can lose the few output bytes the takeover read. The shells survive either way. The window is minimized: COMMIT is sent only after READY, and nothing is buffered in the old daemon. A `shortcut:` comment records it (handoff.go:21-24). CONTEXT decision, Research A6 | 09-04 plan-time threat model (low severity) | 2026-11-19 |
| AR-09-03 | T-09-26 | A takeover stopped by SIGSTOP after COMMIT keeps the old daemon frozen: output waits in the PTYs and writes get 503. Only a same-uid local actor can stop it, and `kill -CONT` or `kill -9` ends the wait. Thawing on a timer would risk two readers and lose data | 09-04 plan-time threat model (low severity) | 2026-11-19 |

All three were accepted in the plans' own threat models, are low severity (below the `high` block threshold), and were not newly accepted by this audit.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-20 | 39 | 39 | 0 | gsd-security-auditor (retroactive /gsd-secure-phase), ASVS L1 with the high-severity handoff threats checked at the boundary, code pinned at afe46fd9 |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-20
