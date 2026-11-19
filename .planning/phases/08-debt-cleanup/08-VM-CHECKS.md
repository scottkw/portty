# Phase 8 VM Checks: cold launches (DEBT-06)

Evidence for plan 08-04 Tasks 1 and 2. Each row is one GUI launch Claude ran in the home Tart VM after a VM reboot. Result is exactly `PASS` or `FAIL`. No share links, capability values or keys are in this file; the capture scripts passed every log line through a `cap=` redaction and none was needed.

- **Date:** 2026-11-19 (VM clock, UTC)
- **Build for L-01..L-05 and B-01..B-08:** the deployed `~/ptest/Portty.app` from the 07-VM-CHECKS R-run: real app (`wails build -tags wailsassets`) at `bcb837dd`, ad-hoc signed, binary sha256 `78504a15dd735943648b5d17df6b74254b9d5eddfa6877f6d7a29475eb2353fd` (checked in the VM before the first reboot). `git log bcb837dd..2e27e73c -- . ':!.planning'` is empty, so this is Phase 7's shipped code, including the 07-03 log lines.
- **VM:** home `portty-test-2` (192.168.64.109, macOS 27.2, 3 vCPUs). The office VM was not used.
- **Reboots:** `sudo shutdown -r now` over SSH, inside the VM. Before each reboot the GUI and then the daemon were killed with `kill -9`, which leaves the previous run's `daemon.sock` file on disk, as a crash or power loss would. After each boot: wait for SSH and for `stat -f %Su /dev/console` = `admin` (auto-login), then `pgrep -fl Portty`.
- **Launch and capture (L rows):** `open --stderr /tmp/cold-N.err ~/ptest/Portty.app`, 30 s wait, then: `grep -c 'respawn attempt'` and `grep -c 'not answering right after launch'` on the capture, the daemon pid and start time before and after (`ps -o pid,ppid,lstart`), and `daemon.log` / `daemon.log.1` (`~/Library/Application Support/portty/`). The daemon rotates `daemon.log` to `daemon.log.1` once it has bound its socket, so a run's startup lines are in `daemon.log.1`.
- **Settings:** WebGL off in the VM's settings.json for the run (backed up to `~/ptest/settings.json.pre0804` first, restored afterwards, `cmp` clean).
- **Nothing on the host** was clicked, killed or changed. The host ran only SSH and, for Task 2, the build.

## Finding: macOS reopens Portty at login

The first reboot showed that macOS reopened Portty at login although Portty has no login item: loginwindow's relaunch list (`~/Library/Group Containers/group.com.apple.loginwindow.persistent-apps/persistantApps`) still listed `app.portty.portty`, because the GUI was killed rather than quit before the restart. A reopened app's stdout and stderr are `/dev/null` (`lsof -d 1,2`), so the plan's step "kill it and note it" applies. But every reopened launch also showed the blip's trace (a duplicate daemon, and bytes written to stderr), so extra rows B-01..B-08 chase that launch at login with stderr captured. For the L rows, Portty was removed from the relaunch list before each reboot (the list was backed up to `~/ptest/persistantApps.pre0804`), so the `open --stderr` launch is the first launch after the boot.

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| L-01 | PASS | Boot 1, no daemon before the launch: first stderr-captured GUI launch spawns exactly one daemon, no respawn or launch-blip line | Boot 16:26:48. macOS had reopened Portty at login (GUI 403 at 16:27:17, daemon 500 at 16:27:28; see B-01); both killed with `kill -9` at 16:29:12, `pgrep` then empty. `open --stderr /tmp/cold-1.err` at 16:29:20 (load average 225). After 30 s: GUI 867, daemon 873 (ppid 867, started 16:29:21), the only daemon. Capture: 0 lines, `respawn=0 blip=0`. `daemon.log.1`: `daemon: relay listening on port 49227`, `daemon: listening on …/daemon.sock` (one start) |
| L-02 | PASS | Boot 2, CLI-started daemon aged 3 min: the GUI launch uses it, no respawn, same daemon | Boot 16:53:19, `pgrep` empty. `Portty list` at 16:53:57 started daemon 632 (ppid 1, 16:53:58). `open --stderr /tmp/cold-2.err` at 16:57:15 (daemon age 3:17). After 30 s: daemon 632 throughout, GUI 880. Capture: 0 lines, `respawn=0 blip=0`. `daemon.log` not rotated (still 632's two startup lines in `daemon.log.1`, empty `daemon.log`) |
| L-03 | PASS | Boot 3, no daemon before the launch, launched 39 s after boot | Boot 16:58:14, `pgrep` empty. `open --stderr /tmp/cold-3.err` at 16:58:53 (load average 122). GUI 603, daemon 610 (ppid 603, 16:58:54), the only daemon. Capture: 0 lines, `respawn=0 blip=0`. `daemon.log.1`: one start (`relay listening on port 49152`, `listening on …`) |
| L-04 | PASS | Boot 4, CLI-started daemon aged 8 min: the GUI launch uses it, no respawn, same daemon | Boot 16:59:43, `pgrep` empty. `Portty list` at 17:00:18 started daemon 635 (ppid 1, 17:00:18). `open --stderr /tmp/cold-4.err` at 17:08:32 (daemon age 8:14). After 30 s: daemon 635 throughout, GUI 1113. Capture: 0 lines, `respawn=0 blip=0`. `daemon.log` not rotated |
| L-05 | PASS | Boot 5, no daemon before the launch | Boot 17:09:28, `pgrep` empty. `open --stderr /tmp/cold-5.err` at 17:09:56 (28 s after boot). GUI 539, daemon 556 (ppid 539, 17:09:58), the only daemon. Capture: 1 line, the AppKit hotkey line (`… Portty[539:3431] AdjustToIronwoodHotKeyChange …`), `respawn=0 blip=0`. `daemon.log.1`: one start |
| B-01 | FAIL | Boot 1: macOS reopened Portty at login (stderr not capturable) | GUI 403 (ppid 1) at 16:27:17, 29 s after boot; daemon 500 (ppid 403) at 16:27:28; at 16:27:41 a second daemon 532 was also listed. GUI fd 2 = `/dev/null`, offset 354 bytes written. `daemon.log.1`: `relay listening on port 49152`, `listening on …/daemon.sock`, then `daemon: already running (lock: resource temporarily unavailable)`: a second spawn while 500 was starting |
| B-02 | FAIL | Boot with a one-shot LaunchAgent running `open --stderr` at login, to race the reopen | The reopen won: GUI 335 at 16:33:09 (17 s after boot 16:32:52), daemon 486 (ppid 335) at 16:33:23, 14 s later; the agent's `open` only activated it (no capture file). GUI fd 2 `/dev/null`, 354 bytes. `daemon.log.1` again ends with `daemon: already running (lock: …)` |
| B-03 | PASS | Reopen removed from the relaunch list; LaunchAgent `open --stderr` at login | GUI 649 at 16:36:30, 61 s after boot (the agent's `open` took 30 s), daemon 662 the same second. Capture 0 bytes. One daemon start |
| B-04 | PASS | LaunchAgent execs the binary at login (`StandardErrorPath`), earlier than `open` | GUI 442 at 16:38:43 (28 s after boot), daemon 653 only at 16:39:42 (load average 360), socket bound 16:39:42. Capture 0 bytes. One daemon start |
| B-05 | PASS | Same with `ProcessType Interactive` | GUI 441 at 16:42:28 (34 s after boot), daemon 563 at 16:42:50, socket bound 16:42:53 (3 s). Capture 163 bytes, one line: AppKit's `… Portty[441:2761] AdjustToIronwoodHotKeyChange - CG (hotmod:1) HotKey : …` (not Portty's). One daemon start |
| X-01 | PASS | Control: CPU load alone (32 busy loops on 3 vCPUs), no reboot | `open --stderr` at 16:44:22 with stale socket left by a kill: GUI 930, daemon 932 the same second, socket bound 16:44:22. Capture 0 bytes. CPU contention without the login's cold caches does not slow the daemon |
| B-06 | PASS | VM-only harness: a wrapper app (`~/ptest/Wrap.app`, a script that execs `Portty.app/Contents/MacOS/Portty 2>>/tmp/cold-R1.err`) put in loginwindow's relaunch list, so the launch has the reopen's timing and a stderr file | GUI 371 at 16:46:54 (21 s after boot), daemon 502 at 16:47:08, socket bound 16:47:09. Capture 163 bytes (the AppKit hotkey line only). One daemon start. loginwindow then rewrote the list entry to `app.portty.portty` (the running bundle) |
| B-07 | FAIL | macOS reopen again (from the rewritten entry): how long the daemon takes to bind at login | GUI 376 at 16:49:11 (18 s after boot 16:48:53), daemon 536 (ppid 376) at 16:49:22, `daemon.sock` birth time 16:49:32: **10 s from spawn to bind**. `daemon.log.1` ends with `daemon: already running (lock: …)`, written 16:49:35. GUI fd 2 `/dev/null`, offset **191 bytes**, exactly the length of the B-08 respawn line (no AppKit line this time) |
| B-08 | FAIL | Wrapper harness again (entry re-armed before the reboot): the reopen-timed launch with stderr captured | GUI 382 at 16:51:10 (14 s after boot 16:50:56). Capture 354 bytes, 2 lines, `respawn=1 blip=0`: **`portty: daemon unreachable (do request: Get "http://daemon/health": dial unix /Users/admin/Library/Application Support/portty/daemon.sock: connect: connection refused), respawn attempt 1: ok`**, then the AppKit hotkey line. Surviving daemon 494 (ppid 382) at 16:51:22, socket bound 16:51:22; `daemon.log.1`: `relay listening on port 49152`, `listening on …`, `daemon: already running (lock: resource temporarily unavailable)` (written 16:51:24) |

## DEBT-06 outcome

**Reproduced, with the error text.** No L launch (the plan's `open --stderr` after boot) blipped. A launch at login did, every time macOS reopened Portty (B-01, B-02, B-07) and once with the wrapper harness that gives that launch a stderr file (B-08). The captured line:

```
portty: daemon unreachable (do request: Get "http://daemon/health": dial unix /Users/admin/Library/Application Support/portty/daemon.sock: connect: connection refused), respawn attempt 1: ok
```

There was no `not answering right after launch` line in any capture: the 10 s launch grace never applied.

**Root cause.** `EnsureDaemon` gives a daemon it spawned 3 s to answer `/health` and `/relay-port` (`internal/daemon/process.go`). At login the spawned daemon needs longer: B-07 measured 10 s from spawn to bind, and in B-08 the first spawn had not even taken its lock when the second one did. So:

1. Startup's `EnsureDaemon` returns `daemon did not become ready within 3s`. `startDaemon` stores it in `daemonErr`, emits `daemon:error`, and leaves `launchedAt` zero.
2. A zero `launchedAt` means no launch grace (HARD-04 deliberately gives a failed startup none). The tray poller's first check runs at once after startup. It dials the previous run's socket file, which nobody listens on yet (`connection refused`), and spends respawn attempt 1.
3. That `EnsureDaemon` spawns a second daemon. The lock (WR-05) lets only one run; the other prints `already running`. The health check then passes, so the line ends `: ok`.

In B-08 the daemon that survived (494, bound the second it started) is the respawn's: had it been startup's spawn, startup would have succeeded. The other process, startup's slow first spawn, wrote `already running` into the pre-rotation log. Code reading adds one consequence the captures cannot show: with `respawnAttempts` at 1, the IN-01 path keeps `daemonErr` until a minute of healthy ticks resets the streak.

The 05-VM-MATRIX blip (a warm launch against a daemon healthy for 8 min, older code, no error text) may have had another cause. H-04 and L-02 show the current code does not blip on a warm launch.

**Why the plan's L launches missed it.** They ran via SSH after the console session existed, 39 s to 3 min after boot, when the login's cold caches had mostly warmed. On the real Mac, the matching case is a restart (a macOS update, a crash or power loss) where macOS reopens Portty at login.

**Where the lines go on the real Mac.** An app macOS reopens, or one opened from the Dock or Finder, has stderr on `/dev/null`, so neither app.go line is ever seen. Use `open --stderr <file> /Applications/Portty.app` to capture them. With no capture, the trace is `daemon: already running (lock: …)` in `~/Library/Application Support/portty/daemon.log.1` (or `daemon.log`), right after a start.

## Fix and re-run (Task 2)

- **Fix (root cause above):** `c27b69d3` raises `daemonReadyWait` in `internal/daemon/process.go` from 3 s to 15 s. `27c90bb1` first moved the poll into `waitForDaemon(client, wait)` with no behaviour change. The 10 s launch grace (`respawnLaunchGrace`) and both app.go log lines are unchanged and stay as a safety net (IN-01 deferred).
- **Seam test:** `TestWaitForDaemon_SlowStart` (internal/daemon/process_test.go, `498d1c77`). A stale socket file refuses connections, and a fake daemon binds 4 s later. The test failed before the fix (`EnsureDaemon: daemon did not become ready within 3s`) and passes after. `go test -race -short -count=1 . ./internal/daemon/` passed once (`ok github.com/scottkw/portty 8.222s`, `ok …/internal/daemon 26.624s`). `go vet . ./internal/daemon/` is clean.
- **Build:** real app, `GOTOOLCHAIN=go1.26.11 wails build -platform darwin/arm64 -tags wailsassets` at `c27b69d3`, in this worktree. The fresh worktree needed a placeholder `frontend/dist/index.html`, which the frontend build replaced (gitignored). `codesign -dv`: `Signature=adhoc`, `TeamIdentifier=not set`. Binary sha256 `14db862309b49bc70a34615293fee3f390325922a9e9cac198dac35e697e1bfc`, the same on the host and in the home VM. Deployed with `tar` over SSH. frontend/src/wailsjs restored with `git checkout` after the build.
- **Reproducing condition:** the launch at login. For L-06, L-07 and L-09, the original relaunch-list entry (`app.portty.portty`, `/Users/admin/ptest/Portty.app`) was put back before the reboot, so macOS reopens Portty as in B-01/B-02/B-07. L-08 uses the wrapper harness (as in B-08), so its stderr is a file. A reopened launch's stderr is `/dev/null`, so its byte count is the check: 0 or 163 (the AppKit line) means no Portty line. A respawn line alone is 191 bytes.

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| L-06 | PASS | Fixed build, macOS reopen at login: no respawn, one daemon | Boot 17:15:11. GUI 368 (ppid 1) at 17:15:24, 13 s after boot; daemon 457 (ppid 368) at 17:15:30, socket bound 17:15:30. GUI fd 2 `/dev/null`, offset 0 bytes. `already running` lines in daemon.log/daemon.log.1: 0; one start |
| L-07 | PASS | Same, second run | Boot 17:16:49. GUI 376 at 17:17:04, daemon 498 (ppid 376) at 17:17:13, socket bound 17:17:13. GUI fd 2 offset 163 bytes (the AppKit line's length; no room for a 191-byte respawn line). `already running`: 0 |
| L-08 | PASS | Same with the wrapper harness: stderr captured | Boot 17:18:09. GUI 358 at 17:18:22, 13 s after boot; daemon 493 (ppid 358) at 17:18:30, socket bound 17:18:32. Capture `/tmp/cold-L8.err`: 163 bytes, 1 line, `respawn=0 blip=0`, the AppKit hotkey line only. One daemon start |
| L-09 | PASS | Extra run on the slow path: macOS reopen where the daemon took longer than the old 3 s to bind | Boot 17:19:43. GUI 392 at 17:19:57; daemon 510 (ppid 392) at 17:20:04, socket bound **17:20:08 (4 s after spawn)**. GUI fd 2 offset 163 bytes (AppKit line only). `already running`: 0; daemon 510 is the only one. With the old 3 s wait this launch would have failed startup and respawned (the B-07/B-08 path) |

In L-06..L-08 the daemon bound within 2 s, so they show the fix breaks nothing at login but do not reach the slow path. L-09 does: a 4 s bind, with no respawn line and no second daemon.

### Cleanup

- Portty killed; settings.json restored from `~/ptest/settings.json.pre0804` (`cmp` clean, WebGL back on). The relaunch list restored from `~/ptest/persistantApps.pre0804` (`cmp` clean; it lists Portty again, as before the run). `~/ptest/Wrap.app`, the LaunchAgent, both backups and the `/tmp/cold-*` captures removed.
- Portty relaunched normally with `open` (GUI 830, daemon 832); `portty list` is empty.
- The home VM now runs the fixed build (`14db8623…`). The office VM still runs `bcb837dd` (`78504a15…`). Both are version `dev`, so there is no version-mismatch restart between them.
