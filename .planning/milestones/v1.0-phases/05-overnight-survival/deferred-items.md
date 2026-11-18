# Phase 5 Deferred Items

## From 05-04

- **Flaky `SharePopover: Copy > Copy writes the full link and reads Copied! for 2 s`** (`frontend/src/components/__tests__/SharePopover.test.tsx`). It passes when run alone and fails in roughly half of full `vitest run` passes. It also failed in 2 of 3 full runs at the plan base (7d5a9abd), so 05-04 did not cause it. One base run also failed `initialGrid.test.ts > measureInitialGrid > returns null under jsdom`, another load flake. Likely cause: `vi.useFakeTimers({ shouldAdvanceTime: true })` lets real time pass under parallel-worker load, so the 1999 ms check sees the 2 s timer already fired. Out of scope for 05-04; not fixed.
  status: acknowledged

## From 05-05

- Same two load flakes seen again: two default full `vitest run` passes each failed only `SharePopover: Copy ... for 2 s` and `initialGrid > returns null under jsdom` (a 5 s timeout on a synchronous test). Host load average was about 330 at the time. Both pass when run alone, and `vitest run --maxWorkers=4` passed 83/83 files (1119 tests). 05-05 does not touch either file. Not fixed.
  status: acknowledged

## From 05-06

- **`TestCLIAttach_LongHistoryJoiners/gui_tab_through_proxy` load flake.** One full `go test -race -short ./...` run failed it with the scene's 15 s context expiring while 200 history lines were typed one at a time (the dial at the end hit "context deadline exceeded"). It passes alone in about 2.5 s per subtest, and the next full daemon-package run and full-suite run passed. Two test VMs were loading the host. Not changed by 05-06 (newAttachScene only moved the viewing-daemon setup into newViewingDaemon). Not fixed.
- **`gofmt -l` lists `internal/relay/scrollback_altscreen_test.go`** (last touched in 05-01, bd74dc3d). Formatting only; out of scope for 05-06. Not fixed.
  status: acknowledged

## From 05-13

- **A refused connection to the peer's share port ends a GUI peer tab as "stopped sharing".** `handlePeerConnect` maps `ECONNREFUSED` to `not_sharing` (RESEARCH Pitfall 5: the share server only runs while something is shared), and `peerConnectVerdict` makes that a final stop. Seen in the VM only with an artificial pf `block return` rule (a locally made RST). On a real tailnet a refusal comes from the host itself, so it most likely means sharing really stopped; one real case that could trip it is the host's daemon restarting while a viewer redials. Pre-existing (04/05-05); not changed by 05-13.
- **A GUI peer tab's link to the other Mac is the viewing daemon's proxy, and a silent peer is noticed only by the proxy's 45 s watchdog.** System wake and network-change events reach the frontend (05-13), not the proxy. Recovery after a real sleep was immediate anyway (the proxy's wall-clock watchdog fires at once after a long gap), and a network change makes the frontend redial. A black-holed peer with no wake or address change still takes about 45 s. Possible follow-up: feed `wakewatch` to the daemon proxy as well.
  status: acknowledged

## From 05-14

- **R6 FAIL: nothing restarts the viewing Mac's daemon.** After `kill -9` of the home VM's daemon, the GUI peer tab and `portty attach` both stayed "reconnecting" for good (checked for 1 min 39 s). No launchd agent is installed by default (`portty daemon install` is opt-in). The app calls `daemon.EnsureDaemon` only at startup (`App.startDaemon`) and from the Retry button after a failed startup (`App.RetryDaemon`). Neither the GUI tab's re-auth nor the CLI's redial respawns it. Once a daemon was started by hand (`portty list`), both viewers reconnected within 13 s with a delta replay. Possible fixes, for the orchestrator to choose from: the GUI's 5 s tray or health poller calls EnsureDaemon when health fails; the guest-tab re-auth path and the CLI redial call EnsureDaemon before redialing; or install a KeepAlive launchd agent by default. Not changed in 05-14.
- **ECONNREFUSED question (from 05-13) answered on the VMs (matrix R9, PASS).** None of these produced a refusal or a false "stopped sharing": office or home tailscaled restarted or stopped for 25 s, office en0 down for 20 s, office GUI crashed and relaunched with its daemon running, silent pf drops, and home interface or address changes. The share listener is the daemon's and survives tailscaled restarts. A refusal came only when the host daemon was really gone (R7, office reboot). The pf `block return` artifact is still the only false-stop path seen.
- **Recovery after the host wakes is bounded by the viewer backoff (R5).** When the office VM resumed from a 3 min suspend, the viewers came back 32 s (GUI) and 46 s (CLI) later. The viewing side has no signal that the host woke, so it waits out the 30 s backoff cap plus the 10 s peer dial timeout. Possible follow-up: retry faster once `tailscale status` shows the peer online again.
- **The sidebar took 20 to 45 s to list the office VM again after it rebooted.** The session was created at about 06:33:05. It was missing from the home sidebar at about 06:33:30 and present at 06:33:53, even though `portty list` on the home VM already showed it. This is likely the same unreachable-peer backoff. Not investigated.
- **The R4 viewing-side wake took 9 to 12 s with a real `tart suspend`.** 05-13's kill -STOP run took 1.5 s. Most of the time is the home daemon proxy noticing its dead peer sockets, about 8 s after resume. The 05-13 note still applies: feeding `wakewatch` to the daemon proxy would make this immediate.
  status: acknowledged
