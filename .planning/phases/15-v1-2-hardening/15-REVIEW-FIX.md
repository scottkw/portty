---
phase: 15-v1-2-hardening
fixed_at: 2026-11-23T15:25:00Z
review_path: /Users/ken/dev/portty/.planning/phases/15-v1-2-hardening/15-REVIEW.md
iteration: 3
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 15: Code Review Fix Report

**Fixed at:** 2026-11-23T15:25:00Z
**Source review:** /Users/ken/dev/portty/.planning/phases/15-v1-2-hardening/15-REVIEW.md
**Iteration:** 3

**Summary:**
- Findings in scope: 1
- Fixed: 1
- Skipped: 0

## Fixed Issues

### WR-01: `listenDaemonSocket` changes the process-wide umask while the daemon forks shells and creates files on other goroutines

**Files modified:** `internal/daemon/ipc.go`, `internal/daemon/socket_test.go`
**Commit:** cc7e67fb (fixed: requires human verification, since it is a security-ordering change that tests cannot prove)
**Applied fix:** Took the reviewer's preferred route. Removed `syscall.Umask`, `umaskMu` and the `sync`/`syscall` imports from `listenDaemonSocket`; it is back to `net.Listen`, then `os.Chmod(path, 0600)`, closing the listener and failing the start on a chmod error. The doc comment now says the bind-to-chmod window is fenced by the 0700 parent directory that `DefaultSocketPath` enforces plus the peer-UID check, and that the umask is deliberately left alone because the daemon forks shells. Deleted `TestListenDaemonSocket_RestoresUmask` (it tested only the removed mechanism). Kept `TestListenDaemonSocket_Mode0600` and `TestDefaultSocketPath_TightensExistingDir`, and the `DefaultSocketPath` tighten-to-0700 from f6925dbb.

**Not done (deliberate):** the reviewer's optional "fail the start if the parent dir is not 0700" check was not added. The task asked for the simpler route, and a hard fail could break daemon starts on custom socket paths and test temp dirs. The fence is enforced for the production path by `DefaultSocketPath`. A new test that fails without the umask line does not apply, because the mechanism is gone and the mode assertion is the chmod's job.

**Verification:** ran in an isolated git worktree (no node_modules needed for Go), with `GOTOOLCHAIN=go1.26.11`: `gofmt -l internal/daemon` clean, `go vet ./internal/daemon/` clean, `go test -race -short -count=1 ./internal/daemon/... ./internal/fdpass/...` passed (daemon 97s, fdpass 1s). The worktree was fast-forwarded into gsd/v1.2, then removed with its temp branch and recovery sentinel. Unrelated user files were not staged.

---

_Fixed: 2026-11-23T15:25:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
