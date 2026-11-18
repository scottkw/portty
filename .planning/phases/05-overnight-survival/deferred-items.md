# Phase 5 Deferred Items

## From 05-04

- **Flaky `SharePopover: Copy > Copy writes the full link and reads Copied! for 2 s`** (`frontend/src/components/__tests__/SharePopover.test.tsx`). It passes when run alone and fails in roughly half of full `vitest run` passes. It also failed in 2 of 3 full runs at the plan base (7d5a9abd), so 05-04 did not cause it. One base run also failed `initialGrid.test.ts > measureInitialGrid > returns null under jsdom`, another load flake. Likely cause: `vi.useFakeTimers({ shouldAdvanceTime: true })` lets real time pass under parallel-worker load, so the 1999 ms check sees the 2 s timer already fired. Out of scope for 05-04; not fixed.

## From 05-05

- Same two load flakes seen again: two default full `vitest run` passes each failed only `SharePopover: Copy ... for 2 s` and `initialGrid > returns null under jsdom` (a 5 s timeout on a synchronous test). Host load average was about 330 at the time. Both pass when run alone, and `vitest run --maxWorkers=4` passed 83/83 files (1119 tests). 05-05 does not touch either file. Not fixed.

## From 05-06

- **`TestCLIAttach_LongHistoryJoiners/gui_tab_through_proxy` load flake.** One full `go test -race -short ./...` run failed it with the scene's 15 s context expiring while 200 history lines were typed one at a time (the dial at the end hit "context deadline exceeded"). It passes alone in about 2.5 s per subtest, and the next full daemon-package run and full-suite run passed. Two test VMs were loading the host. Not changed by 05-06 (newAttachScene only moved the viewing-daemon setup into newViewingDaemon). Not fixed.
- **`gofmt -l` lists `internal/relay/scrollback_altscreen_test.go`** (last touched in 05-01, bd74dc3d). Formatting only; out of scope for 05-06. Not fixed.
