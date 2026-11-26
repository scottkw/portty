# Phase 19 VM checks (plan 19-07): the launchd start model on macOS 27.2

Rig: `.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh` (host builds and tar streams; every launchctl, kill, rm and /Applications write runs inside portty-test through the ssh helper). Guest: portty-test ("office"), macOS 27.2, console user logged in, guest tailscale Running (not used by these rows).

Builds (both from commit `f8f4953b`, the tree with 19-02 and 19-17 merged, `wails build -tags "wailsassets vmpreview"`, ad-hoc signed, identifier app.portty.Portty):

| Build | Version | Signing |
|-------|---------|---------|
| N | 19.0.1 | ad-hoc |
| N+1 | 19.0.2 | ad-hoc |

Evidence files are in `evidence/` (written by `p19-rig.sh state ROW`; home paths shown as `~`, tailnet addresses and tokens never captured).

| Row | Requirement | Steps (short) | Verdict | Evidence |
|-----|-------------|---------------|---------|----------|
| L1 | DSHR-01 | `clean`, deploy N, `open /Applications/Portty.app`, wait for /health | PASS | `evidence/L1.state`: plist written with the label (KeepAlive SuccessfulExit false, AbandonProcessGroup true, RunAtLoad); `launchctl print` state running, pid 17623, runs 1; `ps` shows the daemon at pid 17623 with ppid 1; /health `launchd:true`, pid 17623 |
| V4 | DSHR-01 | read XPC_SERVICE_NAME of the daemon from `ps eww` and compare with the launchctl pid | PASS | `evidence/L1.state`: XPC_SERVICE_NAME equalled the label (`app.portty.Portty.daemon`), so the first branch of `ManagedByLaunchd` fired; the second branch (launchctl pid == own pid) also held (print pid 17623 == health pid 17623) |

## Notes

- **No daemon log line names the branch.** The plan expects a daemon log line from 19-02 that says which `ManagedByLaunchd` branch fired; 19-02 writes none, so V4 is read from the process environment and launchctl instead (same two facts the function tests).
- **A launchd-started daemon has no `daemon.log`.** The plist has no StandardOutPath/StandardErrorPath, and only the detached spawner opens `daemon.log` (`openDaemonLog` in `process_unix.go`); `startDaemonLog` rotates the file only when stderr already is that file. After L1, `daemon.log` stayed empty (0 bytes) and the daemon's stderr goes to /dev/null under launchd. Rows that quote daemon log lines for a launchd daemon cannot, so they are decided from `/health`, `launchctl print`, `ps` and the session ticks.
- The GUI's stderr is captured by starting it with `open --stderr` (rig `start-gui`), from row V2 on.
