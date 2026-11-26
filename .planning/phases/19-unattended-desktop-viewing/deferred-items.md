# Phase 19 deferred items

## From 19-12 (found while verifying, out of scope)

- `go test ./internal/daemon/` on the host fails three loopback media tests, with and without the 19-12 edits (checked by removing `desktop_vmdesktop.go` and the `extraDesktopRoutes` change and re-running):
  - `TestDesktopAPI_OnLinkStreamTracer`: `desktop_api_test.go:369: no keyframe within 10s`
  - `TestDesktopAPI_ViewerCapRefusedBeforeUpgrade`: `desktop_api_test.go:749: viewer 0 stopped receiving video`
  - `TestHandoffDesktop_RollbackKeepsDesktop`: `handoff_desktop_test.go:182: no keyframe within 10s`
  They exercise the real pion path over loopback with a synthetic source. Cause not investigated (host network or load, or an earlier plan's regression); the executing host may differ from the planners'.
  - **Resolved (d2d7ea7a):** the cause was the host screen being locked. The daemon built the Manager with nil native Deps, so these tests read the real lock state; TestMain now injects quiet fakes through a test-only hook.
- V1 (daemon-posted lapse notification) is a known gap, see `19-VM-CHECKS.md` findings and `19-12-SUMMARY.md`.
