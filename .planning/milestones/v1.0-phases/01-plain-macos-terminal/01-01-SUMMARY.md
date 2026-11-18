---
phase: 01-plain-macos-terminal
plan: 01
subsystem: platform / build / ci
tags: [strip-down, macos-only, ci, packaging, testing]
status: complete
requires: []
provides:
  - macOS-only Go tree (no *_windows.go / *_linux.go / win32input / nonwindows files)
  - Untagged internal/pty/reap_other.go so the linux e2e fixture still compiles
  - Portable tests/check-traceability-paths.sh (fails on zero or missing paths)
  - Single macOS build.yml job; macOS-only release.yml and distribute.yml
affects: [01-02, 01-03, 01-04, 01-05, 01-06, 01-07, 01-08, 01-09, 01-10]
tech-stack:
  added: []
  removed: [github.com/gen2brain/beeep, github.com/goreleaser/nfpm/v2 (direct deps)]
  patterns: [POSIX-only shell scripts (BSD + GNU grep)]
key-files:
  created: []
  modified:
    - internal/pty/reap_other.go
    - internal/pty/native.go
    - internal/pty/session.go
    - internal/daemon/socket.go
    - internal/daemon/path.go
    - internal/daemon/engine.go
    - internal/daemon/ipc.go (renamed from ipc_nonwindows.go)
    - internal/webserver/tailscale_paths.go
    - main.go
    - tray.go
    - tools.go
    - go.mod
    - go.sum
    - tests/check-traceability-paths.sh
    - tests/build-script.test.sh
    - build.sh
    - .github/workflows/build.yml
    - .github/workflows/release.yml
    - .github/workflows/distribute.yml
    - .gitignore
    - frontend/src/components/WelcomeTab.tsx
    - frontend/src/components/LocalNetworkBanner.tsx
    - frontend/src/components/SettingsTab.tsx
    - README.md
    - TESTING.md
key-decisions:
  - "Renamed internal/daemon/ipc_nonwindows.go to ipc.go (dropped its !windows tag) so no *nonwindows* file stays tracked"
  - "godbus/dbus and tailscale/go-winio remain in go.mod as // indirect transitive deps (wails linux backend, tailscale safesocket); go mod tidy re-adds them, so only the direct requires were removed"
  - "release.yml publishes only the signed DMG plus checksums.txt; the homebrew tap job only needs the DMG line"
  - "publish job stays on ubuntu-latest (it only downloads and uploads artifacts; no Linux build)"
requirements-completed: [STRP-03, STRP-04]
duration: 8 min
completed: 2026-11-14
actuals:
  tokens: 75000
  tasks: 3
  commits: 3
plan_head_before: d6ae2cb39f9e5ff458f177f3722c4797cbad6e7b
plan_head_after: 29bdad0b8e32e3e093dcf9581deb13443d10414e
coverage:
  - deliverable: "macOS-only Go tree; linux e2e fixture still cross-compiles; app bundle builds"
    human_judgment: false
    verification:
      - {kind: command, ref: "go build ./... && go vet ./... && go test -race -short ./...", status: pass}
      - {kind: command, ref: "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags playwrightfixture ./cmd/playwright-fixture && go build ./internal/...", status: pass}
      - {kind: command, ref: "wails build -platform darwin/arm64 -tags wailsassets", status: pass}
  - deliverable: "Portable traceability check that fails on zero or missing paths"
    human_judgment: false
    verification:
      - {kind: command, ref: "bash tests/check-traceability-paths.sh (OK: 114) + two negative temp-dir runs", status: pass}
  - deliverable: "macOS-only CI/release/distribute workflows, no winget/linux/windows packaging"
    human_judgment: false
    verification:
      - {kind: command, ref: "bash scripts/grep-gate.sh && ruby YAML parse of .github/workflows/*.yml", status: pass}
      - {kind: command, ref: "bash tests/build-script.test.sh (37/37)", status: pass}
  - deliverable: "Welcome tab / Tailscale hints / README show macOS only"
    human_judgment: false
    verification:
      - {kind: test, ref: "frontend pnpm test (148 files / 2402 tests) + tsc --noEmit", status: pass}
---

# Phase 1 Plan 01: macOS-only Tree, CI and Packaging Summary

Deleted 31 Windows/Linux Go files, 26 packaging/installer/binary paths and every non-macOS CI job. Rewrote the traceability check with POSIX `grep -E`; on its first run it caught two stale TESTING.md rows that the old `grep -oP` version had silently passed on macOS.

## Performance

- Start: 2026-11-14T20:54:43Z, end: 2026-11-14T21:03:17Z (8 min after re-dispatch on the phase branch)
- Tasks: 3 of 3, 103 files changed (+111 / -4900)

## Accomplishments

- **Task 1 (tracer):** removed all Windows/Linux Go sources and branches (named pipes, job objects, exit detector, ConPTY resize, Linux PATH dirs, GOOS shell fallback, Linux DMABUF env, Linux menu guards). `reap_other.go` no longer has a build tag, so `ReapNaturalExit` resolves on linux. Removed the nfpm tool import and ran `go mod tidy`. The tracer gate ran the full verify end-to-end on the committed tree, and it passed.
- **Task 2:** traceability script rewritten (set -euo pipefail, zero-path and missing-path failures, prints `OK: <N>`). `build.sh` is macOS-only, and the tests now assert that `linux`, `windows` and `--all` are rejected. `build.yml` is a single macOS job, and the check context `build (agenthub, darwin/universal, macos-latest)` keeps the same name. `release.yml` validate moved to macos-latest; build-windows/build-linux were deleted. `distribute.yml` lost submit-winget. `bin/*` and the root `playwright-fixture` binary are untracked and gitignored, and still on disk.
- **Task 3:** Welcome tab shows only the macOS row. The Tailscale "daemon stopped" hint shows the darwin text and otherwise falls back to the generic text. README has no Windows/Linux install, build or CI content (release history left as is). Removed 64 trivial Windows `t.Skip` guards and the now-unused `runtime` imports. TESTING.md: Categories F/N/O and M-52 removed, manifest recounted, CI mapping and branch-protection snippet updated.

## Recorded Outputs (per plan `<output>`)

- **Final Suite Manifest counts:** Go 132, vitest 148, Playwright 9, build-script 1, total 290 (from `git ls-files`).
- **macOS artifact globs kept in release.yml publish:** `artifacts/*.dmg` and `artifacts/checksums.txt` (checksums: `sha256sum *.dmg`). The `macos-dmg` artifact from sign-macos provides `agenthub-*.dmg`. The unsigned `AgentHub.app.tar.gz` and `attestation.json` from build-macos are intentionally excluded.
- **Branch protection:** CI is disabled on the repo. When CI is re-enabled, the user must re-apply GitHub branch protection with the two contexts `build (agenthub, darwin/universal, macos-latest)` and `playwright` (TESTING.md Section 3). The executor did not call the GitHub API.

## Task Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | c24d73d2 | feat(01-01): macOS-only Go tree, drop Windows/Linux sources |
| 2 | fce3d400 | feat(01-01): macOS-only CI, packaging and build script |
| 3 | 29bdad0b | feat(01-01): macOS-only desktop copy, README and TESTING.md |

Intentional deletions: 31 Go files (Task 1). Task 2 deleted 27 paths: packaging/winget (7), build/linux (6), build/windows (3), install.sh and its test, 2 docs, 6 bin/ binaries and playwright-fixture (the last 7 untracked only, still on disk).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed `internal/daemon/ipc_nonwindows.go` to `ipc.go`**
- Found during: Task 1. The verify's `git ls-files '*nonwindows*'` must be empty, but this kept Unix IPC file (not in files_deleted) matched.
- Fix: `git mv` plus removing its `//go:build !windows` line. Behavior unchanged. Commit c24d73d2.

**2. [Rule 1 - Plan error] go.mod still lists godbus and go-winio (acceptance criterion expected 0, actual 2)**
- Found during: Task 1. Removed the direct requires for beeep, godbus, go-winio and nfpm. `go mod tidy` adds back `godbus/dbus/v5` (needed by wails `internal/frontend/desktop/linux`) and `tailscale/go-winio` (needed by `tailscale.com/safesocket`) as `// indirect`, because tidy considers all GOOS. They cannot be removed without breaking `go mod tidy`. beeep and nfpm are gone completely. Verified with `go mod why -m`.

**3. [Rule 3 - Blocking] Removed stale TESTING.md traceability rows in Task 2 instead of Task 3**
- Found during: Task 2. The new script immediately reported `path_windows_test.go` and `concurrent_read_windows_test.go` as missing (deleted in Task 1), and Task 2's verify requires the check to pass. Deleted those two rows and the INSTALL-01 `tests/install-sh.test.sh` row in commit fce3d400.

**4. [Rule 3 - Blocking] Deleted 2 Linux/Windows source-gate tests in `SettingsTab.test.tsx`**
- Found during: Task 3. That file (not in the task's file list) asserted the removed `systemctl` / Start-menu strings. Deleted those two tests in commit 29bdad0b.

**5. [Rule 1] Removed the trivial skip guards in `socket_test.go` / `path_test.go` during Task 1** (they were Task 1 files). Also dropped the Linux-only `TestAugmentServicePath_FlatpakUser` and the Windows branch in `TestSocketPathDefault`.

**Total deviations:** 5 (3 blocking, 2 plan corrections). **Impact:** none on behavior. Every verify command and acceptance criterion passes except the go.mod indirect count, which the plan cannot meet as written.

## Issues Encountered

- First dispatch stopped at a blocking-human checkpoint: HEAD was protected `main`. The user switched to `branching_strategy: phase`, and this run executed on `gsd/phase-01-plain-macos-terminal`.
- Linux e2e note: with `exit_linux.go` gone, a real Linux PTY would no longer get the Wait4 exit detector. The Playwright fixture uses an `io.Pipe` and no real PTY, so this does not affect e2e.

## Known Stubs

None.

## Next Phase Readiness

Ready for 01-02. Every later plan can use `bash tests/check-traceability-paths.sh` as a real gate on macOS. After any `wails build`, revert the generated `frontend/src/wailsjs/wailsjs/` stubs (Pitfall 5).

## Self-Check: PASSED

- Commits c24d73d2, fce3d400, 29bdad0b are ancestors of HEAD; the rev-list count from plan_head_before is 3.
- Plan-level verification re-run after Task 3: go build/vet/test -race, linux fixture and internal cross-compile, tsc, full vitest (148/2402), build-script 37/37, traceability OK 114, grep-gate PASS, wails build OK, clean `git status` (only the pre-existing untracked `.ignore` and `.planning/state.json`).
