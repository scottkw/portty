# Phase 19 deferred items

## From 19-12 (found while verifying, out of scope)

- `go test ./internal/daemon/` on the host fails three loopback media tests, with and without the 19-12 edits (checked by removing `desktop_vmdesktop.go` and the `extraDesktopRoutes` change and re-running):
  - `TestDesktopAPI_OnLinkStreamTracer`: `desktop_api_test.go:369: no keyframe within 10s`
  - `TestDesktopAPI_ViewerCapRefusedBeforeUpgrade`: `desktop_api_test.go:749: viewer 0 stopped receiving video`
  - `TestHandoffDesktop_RollbackKeepsDesktop`: `handoff_desktop_test.go:182: no keyframe within 10s`
  They exercise the real pion path over loopback with a synthetic source. Cause not investigated (host network or load, or an earlier plan's regression); the executing host may differ from the planners'.
  - **Resolved (d2d7ea7a):** the cause was the host screen being locked. The daemon built the Manager with nil native Deps, so these tests read the real lock state; TestMain now injects quiet fakes through a test-only hook.
- V1 (daemon-posted lapse notification) is a known gap, see `19-VM-CHECKS.md` findings and `19-12-SUMMARY.md`.
  - **Resolved (19-14, row V1b PASS):** the notification block was queued on the main dispatch queue, which a headless daemon never runs; it now uses a private queue when NSApp is not running.

## From 19-14 (found while verifying, out of scope)

- Two more loopback media tests in `internal/daemon` fail on this host with the same `no keyframe within 10s` (`TestDesktopAPI_OffClosesViewers` at desktop_api_test.go:529, `TestDesktopAPI_RegenerateKey` at :585), next to the three 19-12 logged; the failing set varies between runs, so this looks like the same host-timing problem on the pion loopback path, not the 19-14 edits (they add a query flag, one route and a client method, and none of these tests touch them). `TestDesktopLapse_*` and every other `TestDesktopAPI_*` pass.
  - **Resolved (d2d7ea7a):** same cause as the 19-12 item, the host screen being locked; the 19-14 worktree predated the fix.
