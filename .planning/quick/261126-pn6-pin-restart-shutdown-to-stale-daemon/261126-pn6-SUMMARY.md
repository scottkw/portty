---
quick_id: 261126-pn6
status: complete
commit: 9e884e05
---

# Quick 261126-pn6 Summary: pin a restart's /shutdown to the stale daemon

## Finding: real

Path: CLI with a stale unmanaged daemon holding live sessions ->
`cliTakeover` -> `launchdTakeover` kickstarts the launchd job and polls for
this build up to `daemonReadyWait` (15 s). The job's daemon re-homes only
after its own 2 s lock wait plus the v1 handoff, and launchd may throttle a
job that exited recently, so the re-home can start after the CLI stopped
waiting. Timeout -> `takeoverFailed` -> `handOff` reads the version once (the
frozen old daemon still serves reads: old version) -> move-failed restart
question -> `--restart-daemon` says yes -> `ShutdownDaemon`. That call is built
for Quit All: it waits out the freeze (503 Retry-After, up to 20 s) and then
posts again, and after a dropped connection re-posts to "whichever daemon
answers" (IN-05). Once the re-home commits, that is this build's daemon, which
owns every session: all sessions end. RED test reproduced it.

## Fix

`DaemonClient.shutdownIfStale` (client.go): one pinned connection for
GET /health and POST /shutdown; stops the process only if its version is
stale; retries the freeze 503 on the same connection only; a dropped
connection stops nothing. `ensureDaemon`'s restart (GUI and CLI) uses it; when
this build answers it returns nil without stopping anything. Quit All keeps
`ShutdownDaemon`.

## Verification

- RED: `TestEnsureDaemon_RestartNeverStopsLateRehome` failed ("the restart
  stopped this build's daemon") before the fix; passes after.
- `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/daemon/ .` ok;
  `go vet` clean; `tests/check-brand.sh` clean; traceability paths ok.
- No VM or host runs.

## Residual risk

- A restart that stops the old daemon before the late re-home freezes it ends
  the sessions, as `--restart-daemon` consented; the late launchd daemon then
  finds no target and exits 0, and startDaemon kickstarts a fresh one.
- launchdTakeover's 15 s timeout still turns a slow re-home into a move-failed
  message; without `--restart-daemon` the command fails with the restart hint
  and sessions keep running.
- Not proven on a real launchd job (fakes only); VM row optional.
