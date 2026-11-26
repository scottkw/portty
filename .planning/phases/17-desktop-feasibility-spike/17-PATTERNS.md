# Phase 17: Desktop Feasibility Spike - Pattern Map

**Mapped:** 2026-11-24
**Files analyzed:** 17 (all new, except 2 edits: `.gitignore`, `TESTING.md`)
**Analogs found:** 11 / 17 (the rest are net-new technology: SCK, VT, pion, SMAppService, viewer page)

All analog paths below are git-tracked (checked with `git ls-files`), except the VM tools under `~/.local/share/portty-vm-tools`, which are outside the repo by design. No `.gsd` mirror paths are used.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `spike/desktop/go.mod`, `go.sum` | config | n/a | root `go.mod` (go directive/toolchain pin) | role-match |
| `spike/desktop/main.go` (subcommands `vmprobe`, `probe`, `daemon-probe`, `register`, `serve`, `host-run`) | entrypoint | request-response / batch | `internal/daemon/process_unix.go` (spawns `exe daemon`) | partial |
| `spike/desktop/capture_darwin.m` + `.go` (SCStream, PNG dump) | utility (cgo bridge) | streaming | `appearance_darwin.m` + `appearance_darwin.go` | role-match (bridge shape only) |
| `spike/desktop/encode_darwin.m` + `.go` (VT H.264) | utility (cgo bridge) | streaming/transform | `appearance_darwin.m` + `.go` | role-match |
| `spike/desktop/opus_darwin.m` + `.go` (AudioConverter) | utility (cgo bridge) | transform | `appearance_darwin.m` + `.go` | role-match |
| `spike/desktop/sess_darwin.m` + `.go` (CGSession, AX, IOPM, SMAppService) | utility (cgo bridge) | request-response | `terminate_darwin.m` + `terminate_darwin.go` | role-match |
| `spike/desktop/server.go` (pion, mux, HTTP, token, `/offer`, `/report`) | service | request-response + streaming | `internal/webserver/server.go` (tailnet bind check) | partial |
| `spike/desktop/viewer.html` | component | event-driven | none | no analog |
| `spike/desktop/packaging/` (Info.plist, helper plist, legacy plist, `sign.sh`) | config / script | batch | `.github/workflows/release.yml:195-230`, `build/entitlements.plist`, `internal/daemon/service.go` | role-match |
| `spike/desktop/wailsshell/` (Wails v2 app loading viewer.html) | component | request-response | root Wails app (`main.go`, `go.mod` v2.10.2) | role-match |
| `.planning/phases/17-*/17-SPIKE-MATRIX.md` | doc | n/a | TESTING.md Section 5 M-NN format (field names only) | partial |
| `.planning/phases/17-*/evidence/*` | evidence | file-I/O | none | no analog |
| `.gitignore` (add `spike/desktop/build/`) | config (edit) | n/a | existing `.gitignore` lines (`/portty`, `build/bin/`) | exact |
| `TESTING.md` (M-157+, new category BH; Section 2 note) | doc (edit) | n/a | Category BG block, TESTING.md:1888-1900 | exact |
| VM drive scripts (scratch, not tracked) | script | request-response | `~/.local/share/portty-vm-tools/vm.sh`, `vdeploy.sh`, `vshot.sh` | exact |

## Pattern Assignments

### cgo ObjC bridge files: `capture_darwin.*`, `encode_darwin.*`, `opus_darwin.*`, `sess_darwin.*`

**Analog:** `appearance_darwin.go` and `appearance_darwin.m` (also `terminate_darwin.*`, `about_darwin.*`, `tray_objc_darwin.m`).

**Go side: cgo preamble** (appearance_darwin.go:1-14). The Go file declares the C prototypes and links frameworks; the ObjC lives in a sibling `.m` so cgo compiles it once:
```go
package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in appearance_darwin.m.
void setAppAppearance(int mode);
void setSidebarMaterial(void);
*/
import "C"
```
Spike adaptation: extend `LDFLAGS` with `-framework ScreenCaptureKit -framework VideoToolbox -framework CoreMedia -framework CoreVideo -framework AudioToolbox -framework ServiceManagement -framework ApplicationServices -framework IOKit`. Add `-fobjc-arc` to CFLAGS (the repo files are non-ARC, per the comment at appearance_darwin.m ~line 47; research compiled the spike APIs with `-fobjc-arc`, so state the choice in the file header). Research compile-checked with `-mmacosx-version-min=13.0`.

**ObjC side: file header and style** (appearance_darwin.m:1-8, 21-25). A header comment naming the purpose, `#import`, plain C-linkage functions, `@available` guards:
```objc
// appearance_darwin.m — native appearance bridges ...
// Kept in its own .m file, like tray_objc_darwin.m, so ObjC symbols are compiled
// once by cgo.

#import <Cocoa/Cocoa.h>

void setAppAppearance(int mode) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (@available(macOS 10.14, *)) { ... }
	});
}
```
Copy: one C function per capability, `dispatch_async` (never `dispatch_sync`) onto the main queue for AppKit-touching calls (comment at appearance_darwin.m:13-18 explains the deadlock risk with a goroutine-driven loop). SCK/VT callbacks should instead call back into Go through exported C callbacks or write into a ring owned by the .m file; no repo analog for callbacks into Go, so that part is "no analog".

**Probe error handling (CLAUDE.md "let it crash")**: every probe function returns the OS error code (SCError codes -3801/-3802/-3811/-3814/-3815/-3821/-3823 per RESEARCH R00) into the JSON; never default to success.

---

### `spike/desktop/main.go` (entrypoint, subcommands) and the launch-path code

**Analog:** `internal/daemon/process_unix.go:23-54` (GUI-spawned Setsid path C, as production does it).
```go
func startDetachedDaemon(exe string) (exited <-chan struct{}, kill func(), err error) {
	cmd := exec.Command(exe, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logFile, err := openDaemonLog(daemonConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "portty: daemon log: %v\n", err)
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close() // the child keeps its own descriptor
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	...
```
Copy for path C: `exec.Command(exe, "daemon-probe")` + `Setsid: true` + stdout/stderr to a log file + reap goroutine. Path D (SSH) is the shell `nohup ... &`, path A/B/E are launchd (no Go analog).

**Legacy plist shape (path B):** `internal/daemon/service.go:59-64` (per RESEARCH, verified): `Arguments: []string{"daemon"}`, `"RunAtLoad": true`, `"KeepAlive": false`, kardianos template, no `ProcessType`. Read this file before writing the path-B plist so the spike plist matches (graft: `graft skeleton internal/daemon/service.go`).

---

### `spike/desktop/server.go` (service, request-response + streaming)

**Analog:** `internal/webserver/server.go:345-380` (tailnet bind guard) and `internal/webserver/whois.go` (LocalAPI WhoIs; the spike does NOT call it, per RESEARCH "spike never queries LocalAPI").

**Bind-address check to copy, but stricter** (server.go:348-380):
```go
var tailscaleCGNAT = netip.MustParsePrefix("100.64.0.0/10")

func (ws *WebServer) assertTailnetBindIP() {
	addr, err := netip.ParseAddr(bind)
	if err != nil { slog.Warn(...); return }
	if addr.IsLoopback() { return }
	if tailscaleCGNAT.Contains(addr) { return }
	slog.Warn("webserver: bind IP is outside the Tailscale CGNAT range ...")
}
```
Spike difference (R23): the production check only warns; the spike's `--bind` must REFUSE (return error) for unspecified (0.0.0.0, ::) and non-CGNAT addresses unless `--allow-lan`. Loopback allowed only for loopback harness runs. Use `net/netip`, same prefix constant.

**pion setup:** no repo analog (pion not in the repo). Use the RESEARCH "Code Examples" block (loopback-verified, pion/webrtc v4.2.29): `ice.NewUDPMuxDefault`, `SetICEUDPMux`, `SetLite`, `SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)`, SDP fmtp `42e01f`, `TrackLocalStaticSample`.

**HTTP + logging style:** use `log/slog` as `internal/webserver` does (`slog.Warn(..., "bind_ip", bind)` key/value style). Token gate on `/viewer.html`, `/offer`, `/report` with a random 128-bit token (RESEARCH Security Domain); cap request body with `http.MaxBytesReader`.

---

### `spike/desktop/packaging/sign.sh` and bundle templates

**Analog:** `.github/workflows/release.yml:205-210` and `build/entitlements.plist`.
```bash
codesign --deep --force --verbose \
  --options runtime \
  --timestamp \
  --entitlements build/entitlements.plist \
  --sign "$MACOS_CERTIFICATE_NAME" \
  build/bin/Portty.app
```
Entitlements (build/entitlements.plist, whole file): `com.apple.security.network.client` and `com.apple.security.network.server`, both `<true/>`. Copy these two keys into the spike's own `ent.plist` (do not reference or modify `build/entitlements.plist`; gate says no change to `build/`).

Spike difference (deliberate trial of REL-01): sign inside-out without `--deep`: helper first with `--identifier app.portty.spike.helper`, then the `.app`. Identity string `Developer ID Application: Ken Scott (S2K7P43927)`; never ad-hoc. Skip notarization steps (release.yml:212-222); research assumption A8 covers Gatekeeper. Bundle ids `app.portty.spike` / `app.portty.spike.helper`, never `app.portty.Portty`.

Helper plist and bundle layout: take verbatim from RESEARCH "SMAppService helper plist and bundle layout" (no repo analog; Portty has no SMAppService code, confirmed by grep for `SMAppService`/`ScreenCapture` finding no source).

---

### `spike/desktop/wailsshell/` (R13)

**Analog:** repo root Wails app and `go.mod` (wails v2.10.2). Reuse the root `go.mod`/`go.sum` versions when creating the shell module so it builds offline (RESEARCH: built in ~8 s). Build with `wails build -s -skipbindings`. Shell should only load the spike's viewer page; origin `wails://` is the thing under test. Keep it out of the root module (nested `go.mod` makes `go list ./...` skip it, verified in RESEARCH Pitfall 6).

---

### `.gitignore` (edit)

**Analog:** existing lines in `.gitignore` (`/portty`, `build/bin/`). Add one line `spike/desktop/build/` in the same commit as the spike scaffold. Note the working tree already shows `.gitignore` modified and an untracked `agenthub` entry and `.ignore`; do not fold those into the spike commit.

---

### `TESTING.md` (edit): new Category BH, M-157 onward

**Analog:** Category BG block, `TESTING.md:1888-1900`. Exact shape to copy:
```markdown
### Category BG — v1.2 Cleanup (Phase 16)

Rig: the Tart VM `portty-test` (never stopped for hours, never logged out, never `tailscale down`) ... Recorded by 16-04 and runs at /gsd-verify-work.

- **M-156** <behavior> (SC1, SC2, D-01).
  1. <step>
  2. <step>
  4. PASS when: <observable criteria>.
  - _Last run:_ <date>, **PASS** in `portty-test` (macOS 27.2), build ... 
  - _Why not automatable:_ <reason>
  - _Source:_ 16-01-PLAN.md and 16-01-SUMMARY.md; 16-CONTEXT D-01; todos ...
```
Also add a Phase 17 note in the Section 2 header notes (like the line-38 note format): "Portty Phase 17 (desktop feasibility spike, FEAS-01/FEAS-02): no new test files; Not automated: Section 5 Category BH, M-157..M-NN." Next free ID M-157, next category letter BH (verified: last is M-156 / BG). Per project CLAUDE.md: traceability rows only for new test files (none expected); run `bash tests/check-traceability-paths.sh` before committing. Candidate M-items from RESEARCH: real 30-day approval prompt (R19), office-Mac lock/typing, office-Mac list entry/attribution, office-Mac HW encoder and display idle, iPhone Safari/Home Screen, office-Mac Tailscale mode.

---

### `17-SPIKE-MATRIX.md` (doc)

**Analog:** the M-NN field discipline in TESTING.md (steps, PASS-when, last-run with macOS version and build, why-not-automatable). Columns per RESEARCH: `ID | Question | Rig + macOS | Procedure | Result (GO/NO-GO/PARTIAL) | Evidence file | If NO-GO, requirement changed | Follow-up M-NN`. Put the FEAS-02 decision rule in the file BEFORE running probes (RESEARCH "Decision rule"). Redact tailnet name, IPs, tokens, user and host names in evidence.

---

### VM drive scripts (scratch use)

**Analog:** `~/.local/share/portty-vm-tools/` (outside the repo; do not track copies of these into `spike/`).

`vm.sh` (whole file) is the SSH wrapper; note the VM IP is hard-coded and must be updated after each boot via `tart ip`:
```bash
ip=192.168.64.108
exec ssh -i ~/.ssh/transom_demo_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o BatchMode=yes -o LogLevel=ERROR "admin@${ip}" "$@"
```
`vdeploy.sh` deploy pattern to reuse for the spike app (kill, remove, tar over ssh, `caffeinate -u -t 1; open`):
```bash
$S/vm.sh 'pkill -9 -f "Portty.app/Contents/MacOS/"; sleep 1; rm -rf ~/ptest/Portty.app'
tar czf - "$APP" | $S/vm.sh "tar xzf - -C ~/ptest && caffeinate -u -t 1; open ~/ptest/$APP; sleep 7; pgrep -lf '$APP' | wc -l"
```
Gotchas: both scripts hard-code a stale scratchpad path in `S=` (an old session dir) and `vdeploy.sh` runs from `build/bin` and picks `Portty.app`; the spike needs its own copy with `S=` and `APP` adjusted (scratchpad, not repo). `vshot.sh` (`screencapture -x` over SSH, then `sips -Z 1280`) is the known-good guest screenshot path; WebKit screenshots in the guest fail with `IOSurface creation failed`. Launch GUI paths with `open`, never by executing the binary from the SSH shell (RESEARCH Pattern 2).

## Shared Patterns

### check-brand gate
**Source:** `tests/check-brand.sh:34-67`
**Apply to:** every tracked file under `spike/` and `TESTING.md` (the gate skips only `.planning/`).
Banned in tracked files outside `.planning/`: the old product name (case-insensitive, also in file names; note the untracked `agenthub` dir already at the repo root is not tracked, so not flagged), the macOS package manager name and "install/tap" phrasing (`internal/daemon/path.go` exempt), any `github.com/scottkw/<repo>` link other than `portty`, the VM-only build tags (`vmpr`+`eview`, `vmwh`+`ois`) under `.github`, `build.sh`, `scripts`, and an import of the `webservertest` package by non-test Go. Also never write the ffmpeg absolute path (it contains the banned word); call `ffmpeg` from PATH. When describing the banned words in notes under `spike/`, split or avoid them. Run `bash tests/check-brand.sh` before and after.

### Module isolation
**Source:** RESEARCH Pitfall 6 (verified); root `go.mod` unchanged.
**Apply to:** all `spike/desktop/**/*.go`. Own `go.mod` (module `portty-spike-desktop`); never import `portty/...` packages; copy small helpers rather than import. Gate: `go list ./...` at repo root lists no `spike` package; `git diff main --stat` outside `spike/`, `.planning/`, `TESTING.md`, `.gitignore` is empty; no change to `release.yml`, `build/`, `Info.plist`.

### Fail loudly
**Source:** project `~/dev/CLAUDE.md` ("Silent Fallbacks"). **Apply to:** all probes. Record the NSError code/domain and exit non-zero; no `or {}` style defaults.

### Go conventions
`go fmt`, context-aware functions (`ctx context.Context` first arg), `log/slog` key/value logging as in `internal/webserver`. Test command baseline: `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`.

### Research safety constraints
Subagents never contact the host tailscaled or 100.x addresses; host experiments never touch `~/Downloads` or `/Applications`; the host run starts only on explicit user go; the VM re-key plus `tailscale lock sign` is a user checkpoint.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `spike/desktop/viewer.html` | component | event-driven | No vanilla WebRTC page in the repo; frontend is React. Use RESEARCH "Viewer page core" and "Pattern 4: Self-reporting viewer". |
| pion/WebRTC code in `server.go` | service | streaming | Repo has no WebRTC; use RESEARCH verified loopback code block. |
| SCStream / VideoToolbox / AudioConverter / SMAppService .m code | utility | streaming | No ScreenCaptureKit or ServiceManagement use in the repo; only the bridge shape (above) transfers. Use RESEARCH "VideoToolbox session", "Opus via AudioConverter", "SCK stream" excerpts. |
| Callbacks from ObjC into Go (frame delivery) | utility | streaming | Existing `.m` files only go Go-to-ObjC (`dispatch_async` fire-and-forget); `//export` callbacks have no precedent here. |
| Launchd plists for helper (SMAppService) | config | n/a | Only the legacy kardianos plist (via `internal/daemon/service.go`) exists. |
| `evidence/*` artifacts | evidence | file-I/O | New by design. |

## Metadata

**Analog search scope:** repo root `*_darwin.{go,m}`, `internal/daemon`, `internal/webserver`, `.github/workflows/release.yml`, `build/`, `tests/`, `scripts/`, `TESTING.md`, `~/.local/share/portty-vm-tools`.
**Files scanned:** ~25 (targeted reads and greps; `graft` not needed).
**Pattern extraction date:** 2026-11-24
