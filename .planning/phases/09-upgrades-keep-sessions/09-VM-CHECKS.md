# Phase 9 VM Checks: upgrades keep sessions (UPGR-01..UPGR-05)

Evidence for plan 09-06 Task 1 (rows I-01..I-03, the two-binary integration test) and, later, plan 09-07 (the two-build GUI/CLI rows). Result is exactly `PASS` or `FAIL`. No share links, capability values, tokens or keys are in this file: the integration test never shares a session.

- **Date:** 2026-11-20 04:47 UTC (VM clock; host 2026-11-19 local)
- **Base commit:** product code at `a13f4928` (09-05 complete). The IT binaries were built from the worktree at `0e9730af` (only the new test file differs from `a13f4928`); the test binary at `23509107` (the test's cleanup fix).
- **VM:** office `portty-test` (192.168.64.108, macOS 27.2, arm64), confirmed with `tart ip portty-test`; it was already running. The home VM was not used.
- **Install method:** `tar` over SSH into `~/p9it` in the VM; removed after the run. Each subtest runs its daemons under its own temp HOME (`/tmp/pit-*`), so the VM's own Portty (`~/ptest/Portty.app`, GUI 22590, daemon 22592) was not touched: both pids were the same before and after the run.
- **Monitoring:** the test's own `-test.v` log (the `t.Logf` lines quoted below), plus a post-run `pgrep -fl "portty-it|daemon-it"` (empty) and `ls -d /tmp/pit-*` (none).
- **Nothing ran on the host** except the builds and one `-test.run` of the test, which skips there.

## Builds

All are plain `GOTOOLCHAIN=go1.26.11 go build -ldflags "-X main.Version=<v>"` CLI/daemon builds (no Wails). `codesign -dv` reports `Signature=adhoc`, `TeamIdentifier=not set` for each (the Go linker's ad-hoc signature; no Developer ID). The sha256 values matched on the host and in the VM.

| Binary | Built from | Version reported (`<bin> version`) | sha256 |
|--------|-----------|-------------------------------------|--------|
| portty-itA | `0e9730af` | v1.2.0-itA | `3b749ecc24e054d2e23456d546215d835f21953ee3dbd7b1a6f436f763aa9795` |
| portty-itB | `0e9730af` | v1.2.0-itB | `e0686938e37798b382473117d16f0f0ee54da6b7c683cdde62c5ee62c90e8211` |
| portty-itBfault | `0e9730af`, `-tags vmpreview` | v1.2.0-itB | `f8453f7d9374ad82add15170d972c18a9a4422728718a45f59a8732dd06410e4` |
| portty-itOld | tag `v1.1` (`bbbd8060`), extracted with `git archive` | v1.1.0-itOld | `03e79f90bc6aceeb7977c04256eb5db11f66d4ac1c64d7d1b4c5c726050a9c29` |
| daemon-it.test | `23509107`, `go test -c ./internal/daemon/` | n/a | `53a4cc021261274ff45f06b8ab4e8e99d0b2479d2584297f3f9326b86e66f90c` |

## Integration test (I-01..I-03)

Command in the VM (04:47:00 to 04:47:11 UTC): `PORTTY_HANDOFF_IT=1 PORTTY_HANDOFF_IT_A=… _B=… _BFAULT=… _OLD=… ./daemon-it.test -test.run '^TestHandoffIT$' -test.v -test.count=1` → `--- PASS: TestHandoffIT (11.15s)`, rc 0. An earlier identical run (04:45:44, test binary `f0cd9819…`) also passed all three subtests; it left one `/tmp/pit-*` dir holding a `.zsh_history` (a shell saved history after cleanup removed the HOME), which led to the cleanup fix in `23509107` and this re-run.

Each subtest: its daemon is started as `<bin> daemon` (a child of the test, in a new session). Two sessions are created with `<bin> new`. Each gets a `HIST-k-1..300` burst and a 200 ms `TICK-k i` counter, typed through the relay WebSocket input frame. One viewer stays on session 1 through `attach.Run`, the CLI's own reconnect and resume loop. Shell pids come from `pgrep -P <daemon pid>`. Only `list --local` is used.

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| I-01 | PASS | `upgrade`: A to B under live ticking sessions and a viewer. Same ids and shell pids, `/health` B, A exits 0, the viewer reconnects with no gap and no repeat, a fresh viewer gets the pre-upgrade history, and no shell sees the status pipe or its variable (09-05 open item). | <ul><li>A daemon 27638; shells `[27640 27649 27656]` (two ticking sessions plus an idle check session).</li><li>`portty-itB list --local`: rc 0, stderr empty (no `couldn't move`).</li><li>`/health` after: `{Status:ok Version:v1.2.0-itB API:2 Handoff:1}`.</li><li>Ids before = after: `07ec8cba…`, `71e86015…`, `ac947215…`. Shells `[27640 27649 27656]` alive with ppid 1. A daemon 27638 exited 0.</li><li>A's log: `handoff: takeover pid 27678 (build "v1.2.0-itB") accepted`, then `handoff: committed: 3 sessions moved to pid 27678 in 13ms; exiting`. The takeover's daemon.log.1: `handoff: committed: serving 3 sessions handed over by pid 27638`.</li><li>Viewer: reconnects=1. TICK-1 runs 1..15 (15 lines, no gap, no repeat). It was at 5 when the upgrade ran.</li><li>Fresh viewer after the upgrade: its replay (4066 bytes) holds `HIST-1-1` and `HIST-1-300`.</li><li>fd/env check (`env \| grep -c PORTTY_TAKEOVER`, the shell's own `[ -e /dev/fd/3 ]`, and `ls /dev/fd`): the A shell before = `env=0 fd3=closed ls=0 1 2 3 4`; the same adopted shell after = `env=0 fd3=closed ls=0 1 2 3 4`; a shell started by B after the commit = `env=0 fd3=closed ls=0 1 2 3 4`. `ls /dev/fd` lists its own directory fd and is identical for A's shell and B's, so no descriptor leaked from the takeover.</li></ul> |
| I-02 | PASS | `fault`: A to B-fault with `PORTTY_VMPREVIEW_HANDOFF_FAULT=new:before-ready:exit`. B's CLI prints the couldn't-move line and the DEBT-08 refusal with rc 1. A keeps serving, the shells stay under A, and the viewer never disconnects. | <ul><li>A daemon 27740; shells `[27742 27748]`.</li><li>`portty-itBfault list --local`: rc 1. stderr: `EnsureDaemon: the handoff failed (outcome 1: the new version exited)` / `portty: couldn't move the running sessions to version v1.2.0-itB (the new version exited); they are still running on version v1.2.0-itA.` / `EnsureDaemon: version mismatch (daemon="v1.2.0-itA", binary="v1.2.0-itB") with 2 live sessions; keeping it` / `portty: the running daemon is version v1.2.0-itA with 2 live sessions; restarting it to version v1.2.0-itB would end them. Re-run as 'portty <command> --restart-daemon …' to restart anyway, or as 'portty <command> --keep-daemon …' to use it as it is.`</li><li>Takeover's daemon.log: `handoff-fault: exiting at new:before-ready pid=27769`. A's log: `handoff: takeover pid 27769 (build "v1.2.0-itB") accepted`, then `handoff: rolled back at ready after 5ms: EOF (sessions=2, takeover pid 27769)`.</li><li>`/health` still v1.2.0-itA. A daemon 27740 alive; shells `[27742 27748]` alive with ppid 27740.</li><li>Viewer: reconnects=0. TICK-1 runs 1..15 (15 lines, no gap, no repeat). It was at 5 at the attempt.</li></ul> |
| I-03 | PASS | `from-v1.1`: a v1.1 daemon gets the plain DEBT-08 refusal, with no takeover and no couldn't-move line, and keeps serving. | <ul><li>v1.1 daemon 27799; shells `[27801 27806]`.</li><li>`portty-itB list --local`: rc 1. stderr: `EnsureDaemon: version mismatch (daemon="v1.1.0-itOld", binary="v1.2.0-itB") with 2 live sessions; keeping it` / `portty: the running daemon is version v1.1.0-itOld with 2 live sessions; restarting it to version v1.2.0-itB would end them. Re-run as …`. No `couldn't move`.</li><li>`/health` after: `{"status":"ok","version":"v1.1.0-itOld"}` (no `api` key).</li><li>Daemon 27799 and shells `[27801 27806]` alive under it.</li><li>Viewer: reconnects=0. TICK-1 runs 1..10 (no gap, no repeat).</li></ul> |

### Cleanup (I-01..I-03)

- The test stops each viewer, sends SIGTERM to the daemon it started, and stops the handed-off B daemon with `/shutdown`. It waits for the recorded shells to exit on the hangup (SIGKILL for any left) and removes the temp HOME.
- After the run: `pgrep -fl "portty-it|daemon-it"` was empty, there was no `/tmp/pit-*`, and `~/p9it` was removed. The first run's leftover `/tmp/pit-2563436024` (only a `.zsh_history`) was removed by hand.
