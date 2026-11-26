# Phase 18: Self-Installing Updates - Pattern Map

**Mapped:** 2026-11-25
**Files analyzed:** 17 (9 new, 8 modified)
**Analogs found:** 16 / 17 (all analog paths verified git-tracked)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/updater/appinstall.go` (new: Mount, VerifyApp, Stage, Preflight, Detach) | service | request-response (Runner exec) | `internal/updater/install.go` (`Verify`, `Reveal`, `Runner`) | exact |
| `internal/updater/installer.go` (new: RunInstaller, seams) | service | event-driven / batch | `internal/daemon/process.go` (`ensureHooks`, `spawnTakeover`) | role-match |
| `internal/updater/mover.go` (new: DetectDMGRun, job builder) | utility | transform | `internal/updater/install.go` (`hasLine`, Runner use) | partial |
| `internal/updater/job.go` (new: Job/Result, atomic write, validation) | model | file-I/O | `internal/updater/install.go` (`Clean`, 0600 file handling) | partial |
| `internal/updater/service.go` (mod: states, Install hook, busy()) | service | state machine | itself (`Download`, `busy`, `fail` closure) | exact |
| `internal/updater/update.go` (mod: asset-name list) | utility | transform | itself (`DMGAssetName`, line 198 use) | exact |
| `internal/updater/install.go` (mod: Clean globs, optional stdout runner) | utility | file-I/O | itself (`Clean`, `ExecRunner`) | exact |
| `cmd_install_update.go` (new: hidden subcommand) | route/CLI | request-response | `cmd_daemon.go` `--takeover` case + `cmd_cli.go` `cliNeedsDaemon` | exact |
| `app_install.go` (new: hooks, result reader, mover boot branch) | controller | event-driven | `app_update.go` (`newUpdateService`, `offerDownload`, `dialog`) | exact |
| `app_update.go` (mod: wire Install, `InstallUpdate` binding) | controller | request-response | itself (`DownloadUpdate`) | exact |
| `app.go` (mod: `startup()` mover branch, `ensure()` keepDaemon) | controller | event-driven | itself (`startup` 265-300, `ensure` 331-370, `onTerminate`) | exact |
| `trash_darwin.go` + `trash_darwin.m` (new: trashItem cgo) | utility | file-I/O | `terminate_darwin.go` + `terminate_darwin.m` | exact |
| `frontend/src/components/UpdateBanner.tsx` (mod: states and copy) | component | request-response | itself | exact |
| `frontend/src/App.tsx`, `wailsjs/go/main/App.d.ts` (mod) | component/binding | request-response | existing update wiring in same files | exact |
| `internal/updater/*_test.go`, `app_*_test.go` (new tests) | test | - | `internal/updater/service_test.go` (`newSvc`, `fakeRun`, `emits`) | exact |
| `internal/updater/real_darwin_test.go` (extend, env-gated) | test | file-I/O | itself | exact |
| `TESTING.md`, `tests/check-traceability-paths.sh` obligations | config/doc | - | existing TESTING.md Sections 2/4/5 | exact |

## Pattern Assignments

### `internal/updater/appinstall.go` (service, Runner-seam exec)

**Analog:** `internal/updater/install.go`

**Runner seam** (lines ~113-121): all tools by absolute path, no shell, injectable.
```go
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
```
Add a stdout-only runner with the same signature for `hdiutil ... -plist` (stderr deprecation warning corrupts plist). Skeleton is in RESEARCH "Stdout-only runner".

**Step-named errors** (Verify, ~lines 128-150): every failure wraps a sentinel with the step name, so tests and logs name the failed step without a path.
```go
if _, err := run(ctx, "/usr/bin/codesign", "--verify", "--strict", path); err != nil {
	return fmt.Errorf("%w: codesign", ErrVerify)
}
out, err := run(ctx, "/usr/bin/codesign", "-dvv", path)
if err != nil || !hasLine(out, "TeamIdentifier="+teamID) {
	return fmt.Errorf("%w: team", ErrVerify)
}
if !hasLine(out, "Identifier="+strings.TrimSuffix(assetName, ".dmg")) { ... "identifier" }
out, err = run(ctx, "/usr/sbin/spctl", "-a", "-t", "open", "--context", "context:primary-signature", "-vv", path)
```
VerifyApp copies this shape for the app inside: `codesign --verify --deep --strict`, `-dvv` Team `S2K7P43927` plus `Identifier=app.portty.Portty`, `spctl -a -t exec -vv` ("source=Notarized Developer ID").

**Quarantine write** (install.go:146-147) is the Chesterton's Fence for Reveal; re-write the same flag before any Reveal fallback:
```go
flag := fmt.Sprintf("0081;%x;Portty;", now.Unix())
run(ctx, "/usr/bin/xattr", "-w", "com.apple.quarantine", flag, path)
```
**Reveal fallback** (install.go `Reveal`): `run(ctx, "/usr/bin/open", "-R", path)`; wrap `ErrReveal`.

### `internal/updater/service.go` (modify: states, Install hook, busy)

**Analog:** itself.

**State doc + Status** (lines 15-25): extend the comment and states with `installing`, `relaunching`; `revealed` + `Error` becomes the warning variant.
**busy()** (line 73): `func busy(state string) bool { return state == "downloading" || state == "verifying" }` -> add `installing`, `relaunching`.
**Failure closure** (Download, lines 178-185): copy for the install-phase failures; texts must hold no path/URL.
```go
fail := func(kind, text string) (Status, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	return s.set(Status{State: "available", Latest: rel.Version, Error: text}), &Error{Kind: kind, Text: text}
}
```
**Verify-then-Reveal tail** (lines 210-233): insert install steps between Verify and Reveal; on any pre-point-of-no-return failure, cleanup, re-set DMG quarantine, call existing `Reveal`, `set(Status{State:"revealed", Error: text})`. Own `context.WithTimeout` per phase like `vctx` (2 min). Add injected `Install`/`Quit`/`Spawn` func fields next to `Run`/`Now` (struct lines 31-45). Logging rule: step names and `filepath.Base` only (`log.Printf("update: %v", err)`).

### `internal/updater/update.go` (modify: asset-name list)

**Analog:** itself. Line 78 `DMGAssetName(v)` is used at line 198 (`name := DMGAssetName(rel.Version) // built from the validated version, never taken from JSON`). Replace with an ordered `var` list (universal, arm64 per A11/O1), still built from the validated version. Other users that must keep compiling: `app_update_test.go:83`, `service_test.go:101,399,413`, `real_darwin_test.go:76,132`. Keep `DMGAssetName` for the universal name. `Verify` already derives the identifier from the chosen name.

### `internal/updater/install.go` (modify: Clean)

Lines ~174-175 glob only `portty-v*-darwin-universal.dmg` and `.part`; widen to the list with the same `filepath.Match` + `e.Type().IsRegular()` guard.

### `internal/updater/installer.go` (RunInstaller with seams)

**Analog:** `internal/daemon/process.go` (hooks struct + test-binary refusal).

**Hooks struct and testing guard** (lines 245-258):
```go
// spawnTakeover starts the takeover process. Like the fresh spawn, a test
// binary never starts one: the child would take the user's real daemon.
func spawnTakeover(exe string) (int, <-chan int, error) {
	if testing.Testing() {
		return 0, nil, errors.New("refusing to spawn a takeover from a test binary")
	}
	return startDetachedTakeover(exe)
}
```
Copy: every effect (Run, Swap, Trash, Open, Health, Procs, Now) is a field of an env struct; the real spawn refuses under `testing.Testing()`.

**Detached spawn** (`internal/daemon/process_unix.go:60-75`):
```go
cmd := exec.Command(exe, "daemon", "--takeover")
cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
cmd.Env = append(os.Environ(), ...)
logFile, err := openDaemonLog(daemonConfigDir()) // cmd.Stdout/Stderr = logFile
```
Installer spawn: same `Setsid: true`, cwd `/`, log to `update.log`; exe is `Contents/MacOS/Portty` inside a `ditto` copy of the whole running bundle at `<configDir>/update/run-*/Portty.app` (revised 2026-11-25: a lone executable copy is killed by AMFI on Developer ID builds, row P2d; see 18-CONTEXT and 18-10).

**Health seam:** `daemon.NewDaemonClient(sock).HealthInfo()` (`process.go:253-262` shows its use) with `Version == "v"+newVersion`; `GetDaemonVersion()` exists too. Injected as a `Healthy` func from package main.
**Swap:** `unix.RenamexNp(staged, target, unix.RENAME_SWAP)`; `os.Rename` when target is absent (RESEARCH "Atomic swap" skeleton).
**Downgrade wall** (`handoff.go:473-474`, `process.go:272-292`): rollback must set `keepDaemon`; see `app.go ensure()` below.

### `cmd_install_update.go` (hidden subcommand)

**Analog:** `cmd_daemon.go:32` and `cmd_cli.go:76`.
```go
case "--takeover": // internal, so not in the usage: only EnsureDaemon spawns it (09-05)
	runTakeoverFunc()
	return nil
```
```go
func cliNeedsDaemon(args []string) bool {
	if args[0] == "health" || args[0] == "version" || args[0] == "open" {
		return false
	}
```
Add `install-update` to that skip list, dispatch like `daemon`, keep out of `usage`. Use a package-level func var seam (like `runTakeoverFunc`, `serviceControlFunc`) so `cmd_daemon_test.go` / `dispatch_test.go` style tests run without spawning. Do not add a GUI argv flag (`launchMode` rejects unknown leading flags).

### `app_install.go` and `app_update.go` (hooks, bindings, dialogs)

**Analog:** `app_update.go`.

**Service construction** (lines 28-36): add `Install`/`Quit`/`Spawn` fields here; `Quit: a.onTerminate`.
```go
return &updater.Service{
	Checker: &updater.Checker{...}, Current: strings.TrimPrefix(Version, "v"),
	Emit: a.onUpdateStatus, Dir: updateCacheDir(), Client: updater.NewDownloadClient(updateAllow), Run: updater.ExecRunner,
}
```
**Binding** (lines 157-165): `DownloadUpdate()` returns `updater.Status`, errors inside `Status.Error`; the install click reuses this (or adds `InstallUpdate`), regenerate `wailsjs/go/main/App.d.ts`.
**Dialog seam** (lines 217-231, 190-199): the mover's "Replace Portty <old> in Applications?" uses `a.dialog(runtime.MessageDialogOptions{... Buttons: []string{"Replace","Cancel"}, DefaultButton, CancelButton})`; never on the main thread; `a.messageDialogFunc` is the test seam. Copy `offerDownload`.
**Window seam:** `a.showWindowFunc`; mover mode never calls `showWindow`.
Also update the manual-check alert copy (line ~178-181 "It will be shown in Finder once it's verified") and `offerDownload` button text "Download" -> "Update" per RESEARCH.

### `app.go` (startup mover branch, ensure keepDaemon)

**Analog:** itself.
**startup()** (265-300): first statement after `a.ctx = ctx`; branch to mover mode before `signal.Reset`/`initTray` (line ~287), `startDaemon` (~296), pollers. Normal path stays untouched (Pitfall 8: pin with a test that normal startup calls the ensure seam).
**ensure()** (331-370) keepDaemon policy: current line
```go
if err := ensureDaemon(ctx, socketPath, daemon.EnsureOptions{Restart: a.restartConsent, NoHandoff: approved}); err != nil {
```
When the pending result file says `keepDaemon`, pass `Restart: func(daemon.RestartQuestion) bool { return false }, NoHandoff: true`. Test with the existing `ensureDaemon` var seam and `a.ensureDaemonFunc`.
**onTerminate()** (659-666): the GUI-only quit used as `Service.Quit`; sets `a.quitting` then `a.quitFunc` or `runtime.Quit`. Unit tests set `a.quitFunc`.

### `trash_darwin.go` + `trash_darwin.m`

**Analog:** `terminate_darwin.go` / `terminate_darwin.m`.
```go
/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa
// Implemented in terminate_darwin.m.
void installTerminateHook(void);
```
`.m` file header style: `#import <Cocoa/Cocoa.h>`, own `.m` so ObjC symbols compile once, comment citing why. Implement `int porttyTrashItem(const char *path)` with `[[NSFileManager defaultManager] trashItemAtURL:resultingItemURL:error:]` called synchronously (no dispatch_sync to main, per the note in terminate_darwin.m). Wrap in a Go func var seam so `RunInstaller` tests inject Trash. Reference for behaviour: `~/dev/transomwm/Sources/TransomMac/Onboarding/AppMover.swift` (outside repo, read-only reference).

### `internal/updater/job.go`, `mover.go`

No exact analog. Use `Clean` (install.go) for `ReadDir`/`IsRegular`/`Match` filename validation idiom, and `ParseVersion`/`Compare` (update.go) for version-newer checks. Job JSON: 0600, owner/mode check, `filepath.Base(target)=="Portty.app"`, staging sibling, `Lstat` reject symlinks (RESEARCH Pattern 3, Security Domain).

### `frontend/src/components/UpdateBanner.tsx`

**Analog:** itself (lines 14-50): `state` drives `message`; `failed = state === 'available' && error !== ''` drives the warning variant. Extend: `installing`, `relaunching` messages; `revealed && error` -> warning icon/`banner--warning` with the error text; `Download` button text -> `Update`; keep `role="alert"`, `banner__action`, `banner__dismiss`. Components never open URLs. Update vitest for the new states and dismiss rules (existing UpdateBanner/App tests).

### Tests

**Analog:** `internal/updater/service_test.go`.
- Fixture: `newSvc(t)` (lines 140-171) builds `Service` with `ghFake`, `emits` (`states()` joins emitted states), `fakeRun{team: goodDVV, spctl: goodSpctl}`, fixed `Now`. Extend `fakeRun` for hdiutil/ditto/xattr/plutil outputs; add fake Install/Quit.
- Existing `TestService_ErrorTextsHaveNoURLOrPath`: extend with new texts.
- Real-tool test: `real_darwin_test.go` (env-gated `UPDATE_REAL_*`, `t.TempDir()`); add `PORTTY_UPDATER_REAL=1` scratch-DMG test; never touch ~/Downloads or /Applications.
- `testmain_test.go` already points HOME at temp dir.
- Package main: `app_update_test.go`, `app_exit_test.go`, `cmd_daemon_test.go`, `dispatch_test.go` for dispatch, quit and seam patterns.
- TESTING.md: Section 2 manifest (new files), Section 4 traceability rows for UPD-05..UPD-09 (repo-relative `.go/.ts/.tsx/.sh` paths only), Section 5 manual items from M-165 (Category BI); run `bash tests/check-traceability-paths.sh`.

## Shared Patterns

### Runner seam for every external tool
**Source:** `internal/updater/install.go` `Runner`/`ExecRunner`. **Apply to:** appinstall.go, installer.go, mover.go. Absolute tool paths, context deadlines on every call, no shell.

### No path or URL in user text or logs
**Source:** `service.go` fail texts, `log.Printf("update: %v", err)` with step names. **Apply to:** all new Status.Error texts and installer logs; extend `TestService_ErrorTextsHaveNoURLOrPath`.

### Test-binary refusal for process-spawning seams
**Source:** `internal/daemon/process.go:253,332` (`testing.Testing()`). **Apply to:** installer spawn, trash, relaunch, hdiutil.

### GUI-only quit
**Source:** `app.go:659-666` `onTerminate`. **Apply to:** Service.Quit; installer signals only the GUI pid (never `daemon` processes; filter `ps` args).

### Internal subcommand outside usage
**Source:** `cmd_daemon.go:32`, `cmd_cli.go:76`. **Apply to:** `install-update`.

### Cgo ObjC in its own .m file
**Source:** `terminate_darwin.go/.m`. **Apply to:** trash.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/updater/job.go` / `mover.go` (job+result files, disk-image detection via `hdiutil info -plist`) | model/utility | file-I/O | No existing job/result-file protocol or plist/hdiutil handling in repo; use RESEARCH Patterns 3 and 7 plus the TransomWM AppMover (external reference) |
| Rig for two-build VM checks | test infra | - | `cmd/fakegh` and `updates_vmpreview.go` exist (tracked) but need extension (RESEARCH O5); `vdeploy.sh` pkills the daemon too, so write a new script |

## Metadata

**Analog search scope:** repo root `*.go`, `internal/updater`, `internal/daemon` (process, process_unix, handoff), `frontend/src/components`, `cmd_*.go`.
**Files scanned:** about 25 read or grepped; analog paths confirmed tracked via `git ls-files` (updater, terminate_darwin, cmd_cli, cmd_daemon, app_update, UpdateBanner).
**Pattern extraction date:** 2026-11-25
