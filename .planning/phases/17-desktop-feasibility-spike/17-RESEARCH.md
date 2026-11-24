# Phase 17: Desktop Feasibility Spike - Research

**Researched:** 2026-11-24
**Domain:** macOS screen capture (ScreenCaptureKit) + VideoToolbox H.264 + pion/webrtc v4 over a tailnet, TCC attribution (SMAppService helper vs in-daemon), lock/sleep/approval behaviour, Tart VM rig
**Confidence:** MEDIUM overall. HIGH for the recipes proven locally this session (VideoToolbox, Opus, pion, WKWebView feature probe, tccd log format). LOW for everything that only the spike can answer (SCK inside Tart, SMAppService from a dev path, lock-screen frames, 27.x approval behaviour). Those are listed as spike rows, not claimed.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Test rig**
- The Tart VMs carry the spike: `portty-test` (macOS 27.2, tailnet `portty-vm-office`) is the shared Mac, `portty-test-2` (`portty-vm-home`) is a viewer. VM rows cover permission attribution (helper vs daemon, SSH respawn, GUI closed), lock-screen capture and CGEvent typing, browser/Wails/iPhone connections over the tailnet, WebRTC in the Wails web view, and Opus encode. User asked "Can we test with the VMs you have used in the past?"
- Two rows run on a real Mac because VMs lack them: hardware-encoder CPU/latency/sharpness and display sleep/wake. They run on the user's personal MacBook Air (macOS 27.2.1, this host) as one short (~10 min) user-started run. The user will see a Screen Recording prompt for the spike binary and may revoke it afterwards. Never start that run without the user saying go.
- First spike step checks whether ScreenCaptureKit returns frames in a Tart VM at all (WebKit screenshots there have failed with `IOSurface creation failed`). If VM capture fails, the affected rows move to the host run and the matrix records why.
- The VMs were stopped ~20 h ago and use ephemeral tailnet keys, so re-joining likely needs the user (new auth key + `tailscale lock sign`). Plans must include that as a user checkpoint.
- The office Mac's macOS version is not yet known; the matrix records the macOS version for every row, and if the office Mac differs from 27.2.x, lock-screen and permission rows get an M-NN real-Mac item.

**Capture-host decision method (FEAS-02)**
- Two tiny permission probes, one per shape: (a) a helper inside a test .app bundle launched as a LaunchAgent via SMAppService, (b) capture inside the daemon-style process (same signed binary spawned detached, also respawned from an SSH session). Each records where macOS attributes the Screen Recording and Accessibility grants (`launchctl procinfo` responsible process, TCC list entry), whether capture works with the GUI closed, and across lock/unlock. The full video pipeline runs in only one shape. — **Reversibility:** costly — the result fixes the process architecture, bundle layout, signing and TCC identity every later phase builds on.

**Timebox**
- No timebox: every matrix row gets a real answer before Phase 19 planning. Rows that genuinely cannot be answered by a spike (e.g. the real 30-day re-approval prompt) are answered with the strongest available evidence and an explicit M-NN follow-up.

**Decided autonomously**
- [Spike code] Q: "Where does spike code live?" → Selected: "A separate Go module under `spike/desktop/`, never imported by the app module, removed or archived when the phase closes" — throwaway by design
- [Signing] Q: "How are spike binaries signed?" → Selected: "With the user's Developer ID Application identity (team S2K7P43927), so TCC behaviour matches a real release; never ad-hoc" — a build flag
- [Monthly re-approval] Q: "How is the monthly Screen Recording re-approval measured?" → Selected: "Read the grant's stored expiry/consent timestamp and record observed behaviour; the real 30-day prompt becomes an M-NN item" — observation only
- [Viewer] Q: "What does the test viewer look like?" → Selected: "One plain HTML page served by the spike, vanilla RTCPeerConnection, no React" — thrown away

### Claude's Discretion
- Exact probe implementation (cgo ObjC vs small Swift helper for the helper shape), order of matrix rows after the VM-capture check, matrix file format.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.

Also carried from CONTEXT `<specifics>`: subagents never contact the host's tailscaled or 100.x addresses (VM rows go through the VMs); host experiments never mount, copy or modify the user's real files (~/Downloads, /Applications); scratch-only, detach anything attached. The matrix must include per-browser full ICE vs ICE-lite results (C3), the 2560 px / 30 fps cap measurement (C4), Constrained Baseline decode (C5), AudioConverter Opus (C7). File-the-entitlement todo stays with the user.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FEAS-01 | Throwaway SCK → VT → pion → browser path shows the desktop live over the tailnet, with a recorded go/no-go for each risky assumption | Matrix rows R00-R23 below; verified recipes for pion, VideoToolbox, Opus; viewer page that self-reports; per-row evidence and no-go consequence |
| FEAS-02 | Capture-host decision (bundled helper vs in-daemon) recorded with evidence before the capture host is built | Probe design for four launch paths (R02-R07), root-free attribution evidence (tccd log), decision rule, recommended pipeline shape |
</phase_requirements>

## Summary

Milestone research already settled the stack (pion v4.2.29, SCK `420v`, VT Constrained Baseline, AudioConverter Opus). This research adds what the planner needs to turn the spike into executable plans: a concrete rig, a probe design that produces machine-readable evidence, recipes proven on this macOS 27.2.1 host, and the traps that would burn a plan. Four findings reshape the plan. (1) Where TCC attributed a grant can be read **without root** from the unified log: `tccd` writes `AUTHREQ_ATTRIBUTION` lines naming the `responsible` process path and identifier; `launchctl procinfo` needs root (it printed "requires root privileges"), so use the log as primary evidence and `procinfo` via `sudo` in the VM as corroboration. (2) The VideoToolbox hardware encoder emits SPS constraint byte `0xc0`, so `profile-level-id` is `42c0xx`, but pion's fmtp match compares the first two bytes exactly, so the SDP must still advertise `42e01f` (what browsers offer) while the bitstream says `42c0xx`; a Safari decode row decides whether the SPS needs patching. (3) The Tart-VM question has no published answer (searched), so row R00 is a 10-second binary probe whose verdict re-routes every capture-dependent row to the host run; the plan must be conditional on it. (4) The check-brand gate scans tracked files outside `.planning/` for banned words, so spike scripts and notes must avoid them.

The full pipeline and every probe come from **one Go + cgo/ObjC binary** signed three ways, which mirrors production (the GUI and the daemon are the same Mach-O). That keeps Swift out (the guest has no swiftc), matches the repo's existing `.m` bridging pattern, and makes the helper shape just a signed copy of the same executable at `Contents/Helpers/`. Evidence goes to `.planning/phases/17-desktop-feasibility-spike/evidence/` and is summarized in one `17-SPIKE-MATRIX.md`.

**Primary recommendation:** Build one `spike/desktop/` module (pion v4.2.29 + cgo ObjC) with subcommands `vmprobe`, `probe`, `serve`, signed with the Developer ID identity as a registrar `.app` + `Contents/Helpers/` copy + legacy-plist daemon copy; run R00 first on the VM, run attribution probes next, run the pipeline in whichever shape passed, collect every browser result through a self-reporting viewer page, and finish with a single user-gated host run for the hardware-encoder, sleep/wake and real-Mac lock rows.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Screen capture (SCK) | Capture host process in the user's Aqua session (helper or daemon) | — | SCK needs a WindowServer session and the TCC grant of its responsible process |
| H.264 / Opus encode | Same process as capture | — | Zero-copy IOSurface to VT; frames never cross a process boundary in the spike |
| WebRTC peer, UDP mux, ICE | Spike server process (same binary, `serve`) | — | pion binds one UDP socket on the tailnet IP; spike takes `--bind`, never queries LocalAPI |
| Signalling (offer/answer) | Spike HTTP server on the tailnet IP | Wails shell (origin `wails://`) for R13 | One POST, non-trickle; token-gated even in the spike |
| Viewer decode/render | Browser (Safari, Chrome, WKWebView, iOS Safari) | — | Vanilla `RTCPeerConnection`; page posts a JSON report back for evidence |
| Input injection | Capture host process (CGEvent) | Viewer data channel carries key messages | Accessibility is per responsible process; must test where it lands |
| Permission state / consent | macOS TCC + replayd (not Portty) | Spike reads via tccd log and plist | No public API for approval state on 27.x; observe, never poll in a loop |
| Evidence store | `.planning/phases/17-*/evidence/` | Viewer reports, tccd excerpts, PNG frames | One matrix file points at raw artifacts |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pion/webrtc/v4` | v4.2.29 (published 2026-11-22, latest v4) [VERIFIED: proxy.golang.org/github.com/pion/webrtc/v4/@latest] | PeerConnection, `TrackLocalStaticSample`, DataChannel, ICE-lite | Milestone STACK decision; API proven this session in a loopback harness |
| `github.com/pion/ice/v4` | v4.4.9 (pulled in by webrtc; import directly for `NewUDPMuxDefault`) [VERIFIED: proxy.golang.org] | Single-port UDP mux | `SetICEUDPMux` takes an `ice.UDPMux` |
| `github.com/pion/interceptor` | v0.1.51 [VERIFIED: proxy.golang.org] | NACK/PLI responders via `RegisterDefaultInterceptors` | Optional for the spike; needed so retransmits work if packet loss shows up |
| ScreenCaptureKit, VideoToolbox, AudioToolbox, ServiceManagement, CoreMedia, CoreVideo, ApplicationServices, IOKit | macOS SDK 27.2 (Xcode 27.2) [VERIFIED: `xcrun --show-sdk-version` = 27.2; compile-only check of every API used below passed] | Capture, encode, helper registration, power assertions | System frameworks, no packages |
| Wails v2 | v2.10.2 (already in repo `go.mod`) [VERIFIED: go.mod line 16] | Throwaway shell app for the WKWebView row (R13) | Same WKWebView host as Portty; built offline in ~8 s with the repo's go.mod/go.sum |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `ffmpeg` / `ffprobe` | on host PATH [VERIFIED: `which ffmpeg`] | Decode/inspect captured `.h264`, compute SSIM/PSNR for sharpness | Evidence analysis on the host only; the guest has none (assumed) |
| `log show` / `log stream` | system | tccd attribution lines (root-free) | Every attribution row |
| `launchctl print gui/<uid>/<label>`, `launchctl procinfo <pid>` (root) | system | Job state; responsible-process corroboration | VM, with sudo for procinfo |
| `sfltool dumpbtm` (root) | system [VERIFIED: `/usr/bin/sfltool` present] | Background Task Management record of the SMAppService registration | R02 evidence |
| `tccutil reset ScreenCapture <bundle-id>` | system [VERIFIED: usage line "reset SERVICE [BUNDLE_ID]"] | Clear one app's grant between runs | Cleanup, re-prompt tests |
| `top -l`, `pmset -g therm`, `pmset -g assertions`, `proc_pid_rusage` | system [VERIFIED: all ran without root] | CPU, thermal, assertions without `powermetrics` | Host run |
| `xcrun` clang + cgo | Xcode 27.2 | Build the ObjC bridge | Host builds; copy binaries to the guest |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cgo ObjC in one Go binary | Separate Swift helper | Guest has no swiftc (memory note), second toolchain, and the repo already bridges with `.m`. Use Swift only if a probe needs an API cgo cannot reach (none found) |
| tccd log for attribution | Reading `TCC.db` | `TCC.db` needs Full Disk Access even for root; log lines worked unprivileged here. Try the DB only as a bonus in the VM |
| Real Chrome in a VM | Chrome on the host as viewer | Chrome-on-host viewing a VM is a user-run step (no subagent may touch the host tailnet); Chrome in the home VM keeps it executable (download needed, assumed) |

**Installation (spike module, executed by the plan):**
```bash
mkdir -p spike/desktop && cd spike/desktop
go mod init portty-spike-desktop       # separate module; go directive 1.26.4, no toolchain line needed
go get github.com/pion/webrtc/v4@v4.2.29 github.com/pion/interceptor@v0.1.51
# A scratch module that resolved pion offline needed golang.org/x/{crypto v0.53.0, net v0.56.0, sys v0.46.0}
# (cached versions). Online `go mod tidy` will pick its own; no action needed.
```

**Version verification:** pion/webrtc v4.2.29, pion/ice v4.4.9, pion/interceptor v0.1.51, pion/rtp v1.10.6 confirmed as `@latest` on proxy.golang.org on 2026-11-24 with VCS origin `github.com/pion/*` [VERIFIED: proxy.golang.org]. pion/webrtc `go.mod` declares `go 1.24.0` [VERIFIED: module cache go.mod], so the repo's Go 1.26 toolchain pin is compatible.

## Package Legitimacy Audit

> The `package-legitimacy` seam accepts only `npm|pypi|crates` (it printed "Usage: gsd-tools package-legitimacy check --ecosystem <npm|pypi|crates> <pkg1> ..."), so no seam verdict exists for Go modules. Legitimacy below rests on the module proxy's recorded VCS origin plus source read from the module cache this session.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/pion/webrtc/v4 | Go proxy | v4 line, 29 patch releases | n/a (proxy) | github.com/pion/webrtc (proxy Origin) | not evaluable by seam (Go) | Approved: named in milestone STACK, source read here |
| github.com/pion/ice/v4 | Go proxy | — | n/a | github.com/pion/ice | not evaluable | Approved (transitive of webrtc) |
| github.com/pion/interceptor | Go proxy | — | n/a | github.com/pion/interceptor | not evaluable | Approved, optional |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none (seam cannot grade Go). No `postinstall` concept for Go modules. The module is throwaway and never imported by the app module, so the supply-chain exposure is the spike directory only.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌──────────── shared Mac (VM office, or host) ────────────┐
 launch paths (R02-R07):  │                                                          │
  A SMAppService agent ───┼─▶ [helper Mach-O  Contents/Helpers/spike-helper]  ─┐     │
  B legacy plist agent ───┼─▶ [spike  Contents/MacOS/spike daemon-probe]       │     │
  C GUI Setsid spawn  ────┼─▶ [spike daemon-probe]                             ├─▶ capture host role
  D SSH nohup spawn   ────┼─▶ [spike daemon-probe]  (responsible = sshd?)      │     │
  E SSH launchctl kickstart ▶ [agent B/A restarted by launchd]               ─┘     │
                          │                                                          │
   capture host role:  SCStream (420v, 2560 cap, 30 fps) ─▶ CVPixelBuffer (IOSurface)│
        │  idle frame? re-submit cached buffer + ForceKeyFrame on viewer join        │
        ▼                                                                            │
   VTCompressionSession (HW if present, Constrained Baseline, no B-frames, RealTime) │
        │ AVCC ─▶ Annex-B, SPS+PPS before each IDR                                   │
        ▼                                                                            │
   pion: TrackLocalStaticSample(H264)  ◀── Opus AudioConverter (optional row R21)    │
        │  ICE-lite|full, UDPMux on ONE tailnet IP:port, mDNS off, ICEServers empty  │
        ▼                                                                            │
   HTTP on same IP: /viewer.html?t=TOKEN  /offer (POST SDP, non-trickle)  /report    │
        ▲                                         DataChannel "control": ping, keys  │
        └──────────────────────────────┬───────────────────────────────────────────┘
                                       │ tailnet utun (100.x) UDP + TCP
        ┌──────────────────────────────┴─────────────────────────────┐
        ▼              ▼                 ▼                ▼          ▼
   Safari (home VM)  Chrome (home VM)  Wails shell      iPhone     (host run viewer)
        └─ each viewer page: getStats ─▶ POST /report ─▶ evidence/<row>-<ts>.json
```

### Recommended Project Structure
```
spike/desktop/                       # separate Go module, never imported by the app module
├── go.mod / go.sum
├── main.go                          # subcommands: vmprobe | probe | daemon-probe | register | serve | host-run
├── capture_darwin.m/.go             # SCStream, status counters, PNG dump, IOSurface check
├── encode_darwin.m/.go              # VT session, AVCC->Annex-B, force keyframe
├── opus_darwin.m/.go                # AudioConverter Opus (R21)
├── sess_darwin.m/.go                # CGSession dict, preflight, AXIsProcessTrusted, IOPM assertions
├── server.go                        # pion API, mux, HTTP, token, /report
├── viewer.html                      # vanilla RTCPeerConnection, self-reporting
├── packaging/                       # Info.plist, helper plist, legacy plist templates, sign.sh
├── wailsshell/                      # tiny Wails v2 app that loads viewer.html (R13)
└── build/                           # git-ignored build outputs
.planning/phases/17-desktop-feasibility-spike/
├── 17-SPIKE-MATRIX.md               # rows + FEAS-02 decision section
└── evidence/                        # probe JSON, tccd excerpts, PNGs, viewer reports (redacted)
```

### Pattern 1: One binary, three signings (mirrors production)
**What:** The registrar app's main executable is the same Mach-O as the daemon (`Contents/MacOS/spike`, subcommands), exactly like `Portty.app/Contents/MacOS/portty daemon`. The helper shape is a copy at `Contents/Helpers/spike-helper` signed with identifier `app.portty.spike.helper`. The legacy-plist shape uses `Contents/MacOS/spike daemon-probe`.
**When to use:** Always for the spike; it makes shapes comparable and the pipeline portable between shapes.
**Bundle ids:** use `app.portty.spike` / `app.portty.spike.helper`, never `app.portty.Portty`, so no TCC row is shared with the real app that may already be installed in the VM.
**Signing (inside-out, no `--deep`):**
```bash
ID="Developer ID Application: Ken Scott (S2K7P43927)"
codesign --force --options runtime --timestamp --identifier app.portty.spike.helper --sign "$ID" Spike.app/Contents/Helpers/spike-helper
codesign --force --options runtime --timestamp --entitlements ent.plist --sign "$ID" Spike.app
```
`ent.plist` = the two keys in `build/entitlements.plist`: `com.apple.security.network.client` and `com.apple.security.network.server` [VERIFIED: build/entitlements.plist, whole file read]. Release signing today is `codesign --deep --force --options runtime --timestamp --entitlements build/entitlements.plist --sign` [VERIFIED: .github/workflows/release.yml:205-210]; REL-01 will need inside-out, so the spike's script is a trial run of it. Sign on the host. The first sign may raise a Keychain access dialog (not TCC); that is a user checkpoint [ASSUMED].

### Pattern 2: Four launch paths, one probe function
Every path runs the same `Probe()` and writes one JSON file, so rows are comparable. Paths:
- **A (helper):** registrar `.app` launched via `open` calls `SMAppService agentServiceWithPlistName:` `registerAndReturnError:`; launchd then runs `BundleProgram` `Contents/Helpers/spike-helper`.
- **B (legacy plist daemon):** hand-written `~/Library/LaunchAgents/app.portty.spike.daemon.plist` shaped like the kardianos plist Portty installs: `RunAtLoad true`, `KeepAlive false`, `ProgramArguments [exe, daemon]`, `SessionCreate`, no `ProcessType`, no `LimitLoadToSessionType` [VERIFIED: internal/daemon/service.go:59-64 `Arguments: []string{"daemon"}`, `"RunAtLoad": true`, `"KeepAlive": false`; kardianos v1.2.4 `service_darwin.go` template lists only Disabled, EnvironmentVariables, KeepAlive, Label, ProgramArguments, RootDirectory, RunAtLoad, SessionCreate, Standard*Path, UserName, WorkingDirectory]. Load with `launchctl bootstrap gui/$(id -u) <plist>`.
- **C (GUI-spawned):** registrar app does `exec.Command(exe, "daemon-probe")` with `SysProcAttr{Setsid: true}` [VERIFIED: internal/daemon/process_unix.go:26-27 `cmd := exec.Command(exe, "daemon")`, `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`].
- **D (SSH-spawned):** `ssh vm 'nohup <app>/Contents/MacOS/spike daemon-probe >/tmp/d.log 2>&1 &'` (the "CLI respawn" case).
- **E (SSH kickstart):** `ssh vm 'launchctl kickstart -k gui/$(id -u)/<label>'` restarts path A or B through launchd, so the job is its own responsible process even though the request came over SSH [ASSUMED: expected from launchd semantics; the row measures it]. This is the cheap rescue if D fails and decides whether Phase 19 can still respawn the daemon from the CLI.
For GUI-path tests launch the registrar with `open -n <app>` (never by executing the binary from a shell): a shell-launched binary is attributed to the shell/Terminal/sshd, and the prompt would name the wrong app [ASSUMED].

### Pattern 3: Root-free attribution evidence
```bash
# capture a window around the probe run (works unprivileged for an admin user; format verified on 27.2.1)
/usr/bin/log show --start "$T0" --predicate 'subsystem == "com.apple.TCC"' --style compact > evidence/tcc-<row>.log
# fields: AUTHREQ_CTX (service=kTCCServiceScreenCapture ...), AUTHREQ_ATTRIBUTION (responsible/accessing/requesting),
#         AUTHREQ_RESULT (authValue=2 allowed, authReason=4 ...)
```
Verified on this host: lines look like `[com.apple.TCC:access] AUTHREQ_ATTRIBUTION: msgID=…, attribution={responsible={TCCDProcess: identifier=…, pid=…, responsible_path=…, binary_path=…}, accessing={…}, requesting={…}}`, `AUTHREQ_CTX: … function=TCCAccessRequest, service=kTCCServiceScreenCapture, preflight=yes`, `AUTHREQ_RESULT: … authValue=2, authReason=4 …` [VERIFIED: `/usr/bin/log show` on macOS 27.2.1, 2026-11-24; ScreenCapture service name confirmed in a CTX line]. Join the three lines by `msgID`. The row's "responsible" evidence is the `responsible_path`/`identifier` in the line whose CTX service is ScreenCapture or Accessibility and whose `accessing.pid` is the probe pid. Do not paste whole logs into tracked files: they name every app on the machine; filter to the probe pid.
Corroboration in the VM: `sudo launchctl procinfo <pid>` (root only [VERIFIED: "This subcommand requires root privileges: procinfo"]), `launchctl print gui/$(id -u)/<label>`, `ps -o pid,ppid,sess,comm`, `launchctl managername` (Aqua vs Background, called from inside the probe's context [VERIFIED: prints `Aqua` in a local shell]).

### Pattern 4: Self-reporting viewer
The viewer page builds the offer, waits for gathering (capped at 2.5 s), POSTs to `/offer?t=TOKEN`, and after 10 s posts `getStats()` results to `/report`: user agent, `iceConnectionState`, time-to-connected, selected candidate pair (types, addresses, protocol), `inbound-rtp` video (framesDecoded, framesDropped, frameWidth/Height, framesPerSecond, jitterBufferDelay/jitterBufferEmittedCount, totalDecodeTime, decoderImplementation, powerEfficientDecoder), the codec entry (`mimeType`, `sdpFmtpLine`), and a data-channel round-trip time. Every browser row is then one JSON file, including iPhone.

### Pattern 5: Conditional plan shape (the order matters)
1. **Wave 0 (host, no VM, no tailnet):** module scaffold, packaging templates, signing script, `check-brand.sh` run, synthetic-source `serve` that runs the pion loopback harness, matrix skeleton. Everything here needs no user.
2. **User checkpoint:** boot both VMs, re-key and `tailscale lock sign` (ephemeral nodes; stopped VMs are deleted from the tailnet after hours). Warn that stopping them again later forces another re-key.
3. **R00 verdict** (VM capture and encoder). Record it, then branch: GO keeps R02-R07, R15-R17, R19-R22 in the VM; NO-GO moves capture-dependent rows to the host run and the plan must already carry that host scope (see "Host run").
4. **Attribution probes R02-R10** (no WebRTC needed).
5. **Pipeline in one shape** (the shape that passed; if both passed use the in-daemon shape because the same binary also runs as the helper), then browser rows R11-R14, lock rows, Opus, approval rows.
6. **Host run** (user says go): perf, sleep/wake, real-Mac lock, iPhone.
7. **Decision + cleanup + gates:** write FEAS-02 decision, TESTING.md M-items, revoke grants, remove registrations, SC4 checks.

### Anti-Patterns to Avoid
- **Running a probe from an SSH shell and reading it as the GUI-path result.** The attribution differs by launch path; that difference is the experiment.
- **Polling `SCShareableContent`/preflight in a loop on macOS 27** (each call can raise an alert per milestone research). Probe once per run.
- **Letting pion default `MulticastDNSMode`.** pion's agent sets `MulticastDNSModeQueryOnly` when the mode is zero [VERIFIED: pion/ice v4.4.9 agent.go:441-443 `if mDNSMode == 0 { mDNSMode = MulticastDNSModeQueryOnly }`], which opens multicast sockets and can trigger Local Network prompts. Set `Disabled` explicitly.
- **Using `SetNAT1To1IPs`** (removed in v5; unnecessary with a mux bound to the real IP).
- **Writing "homebrew", "brew install", the old product name, or non-portty `github.com/scottkw/` links in tracked files outside `.planning/`** [VERIFIED: tests/check-brand.sh:46-67]. In particular never write the ffmpeg absolute path; call `ffmpeg` from PATH.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| H.264 RTP packetization | Own FU-A/STAP-A | pion `TrackLocalStaticSample` + `WriteSample` | Handles fragmentation, SPS/PPS caching; you only supply Annex-B access units and wall-clock durations |
| Opus encoding | libopus cgo, pion/opus | AudioToolbox `AudioConverter` to `kAudioFormatOpus` | Verified locally: 960-frame packets, no extra dependency |
| Single UDP port | Own demux | `ice.NewUDPMuxDefault` + `SetICEUDPMux` | Verified in the loopback harness |
| TCC state reading | Parsing `TCC.db` | tccd unified-log lines | DB needs Full Disk Access; log is unprivileged |
| Plist parsing in probes | Custom parser | `plutil -convert json -o -` or `plutil -p` | System tool |
| WKWebView test shell | Hand-built Cocoa app | Wails v2 `wails build -s -skipbindings` | Same web view stack as Portty; builds in under a second after the first build |
| Frame sharpness metric | Custom diff | `ffmpeg -lavfi ssim` / `psnr` on host | Standard filters |

**Key insight:** every piece of the spike has an off-the-shelf mechanism; the effort is the rig and the evidence discipline, not code.

## Matrix: rows, rigs, evidence, consequences

Suggested file: `17-SPIKE-MATRIX.md`, one table, columns `ID | Question | Rig + macOS | Procedure | Result (GO / NO-GO / PARTIAL) | Evidence file | If NO-GO, requirement changed | Follow-up M-NN`. Add a header line with the macOS version of each rig. Raw artifacts live in `evidence/`, referenced by relative path; redact tailnet name, IPs, tokens, user and host names.

| ID | Question | Rig | GO means | If NO-GO, changes |
|----|----------|-----|----------|-------------------|
| R00 | SCK frames, IOSurface, VT encoders inside a Tart guest | VM office | `vmprobe` prints frames with status Complete, `ioSurface=true`, and an H.264 session (HW or SW) | Moves R02-R10, R15-R17, R19, R22 to the host run; no requirement changes |
| R01 | Desktop live from another device over the tailnet, no STUN/TURN/cloud | VM office to VM home Safari; host to iPhone | Viewer report: frames decoded, fps, size; pair type from 100.x; `lsof -nP -iUDP -a -p <pid>` shows only `<tailnet-ip>:port`; no packets to 3478/5353 in `tcpdump` | Whole milestone premise; fall back to Plan B (H.264 over WSS + WebCodecs) from PITFALLS; DESK-02 wording |
| R02 | Helper (SMAppService agent): where is the grant attributed, does capture work, list entry | VM office | tccd responsible = helper identity, frames Complete, entry appears in Screen Recording | Helper shape rejected (FEAS-02) |
| R03 | Helper with the GUI app quit | VM office | Same as R02 after `kill` of the registrar app | DSHR-01 under the helper shape |
| R04 | Daemon via legacy LaunchAgent (production always-on path), GUI closed | VM office | Frames Complete, one attribution identity | In-daemon shape rejected; DSHR-01 architecture |
| R05 | Daemon spawned by GUI (Setsid) | VM office | Responsible = spike app, shares its grant | Same |
| R06 | Daemon spawned over SSH (nohup), GUI closed | VM office | Report what happens: likely responsible = sshd wrapper or no WindowServer | If NO-GO and E also fails: daemon must never be respawned from CLI/SSH; CLI must kickstart launchd instead (DSHR-01, DPRM-01) |
| R07 | `launchctl kickstart` over SSH of path A/B | VM office | Job runs in Aqua, attribution = job identity | Same as R06 |
| R08 | Accessibility attribution per path (CGEvent effect + `AXIsProcessTrusted` + tccd) | VM office | Post takes effect; attribution named | DCTL-* under the shape |
| R09 | Settings list entries per shape (plain Mach-O vs bundle path, count, names) | VM office, AX tree via `axdump` | Entry present and revocable | DPRM-01 onboarding copy; helper vs daemon identity count |
| R10 | Grant survives re-sign with same identity and a bundle swap (RENAME_SWAP) | VM office | Capture still works after swap, no new prompt | REL-01 signing rule; Phase 18 swap |
| R11 | Safari (macOS): full vs lite ICE, Constrained Baseline decode | VM home | Report GO per mode: connected, decoded frames, profile | Browser-specific default (C3, C5); SPS patch fallback |
| R12 | Chrome: full vs lite, decode | VM home (Chrome installed) or host Chrome (user-run) | Same | Same |
| R13 | Wails WKWebView: RTCPeerConnection, H.264 receive, DataChannel, Local Network prompt owner, origin `wails://` | VM home with `wailsshell` | Report GO; prompt text recorded | DVWR-01 falls back to opening the browser link |
| R14 | iPhone Safari (and Home Screen) | Host run (user) | Report posted from the phone | DVWR-03 viewer path; Plan B |
| R15 | SCK frames while locked: status, black or lock UI or stale | VM office; host run (real) | PNG of frame during lock inspected; status counts recorded | UNLK-01 cut ("works while awake and unlocked") |
| R16 | CGEvent typing at the lock screen from the capture-host process | VM office (admin password is public); host: type one harmless character and observe | Unlock succeeds in VM; character appears on host | UNLK-01 cut |
| R17 | Lock detection and stream continuity across lock/unlock | VM office | `CGSSessionScreenIsLocked` appears and clears; stream state per transition | DPRM-02 "locked" state source |
| R18 | Display sleep/wake: stream behaviour, wake via assertion, time to first frame | Host run; VM best effort | Wake by `IOPMAssertionDeclareUserActivity` brings frames back within N s | DPRM-04 limited to "keep awake while viewing" |
| R19 | Approval store: where, key, timestamp refresh; backdated behaviour | VM office | Documented store + observed prompt/zero-frame behaviour | DPRM-03 detection method (zero-frame heuristic vs store read) |
| R20 | Hardware encoder CPU, encode latency, sharpness at 2560 px / 30 fps | Host run | `UsingHardwareAcceleratedVideoEncoder=true`, numbers recorded | DESK-01 / DESK-07 resolution/fps cap |
| R21 | Opus encode (done on host) and SCK system-audio capture | Host (encode, verified); VM or host run (capture) | Packets of 960 frames; capture delivers audio buffers | DCTL-07 cut or libopus (C7) |
| R22 | First picture on viewer join with a static screen; idle-frame keepalive | VM office or host | Picture within ~1 s of join with no screen change (cached frame + forced IDR) | DESK-04 design requirement |
| R23 | Tailnet-only binding evidence | Same runs as R01 | Mux bound to one tailnet IP; `--bind` refuses 0.0.0.0, `::`, non-100.64/10 unless `--allow-lan` | DESK-02 test design |

**Rows that need M-NN follow-ups (next free number is M-157; next category letter is BH):** the real 30-day approval prompt (R19); office-Mac lock screen and typing (R15/R16) if its macOS is not 27.2.x or its lock UI differs; office-Mac Screen Recording list entry and attribution (R02-R09); office-Mac hardware-encoder and display idle policy (R18/R20); iPhone Safari and Home Screen if not covered live (R14); office-Mac Tailscale mode (network extension vs userspace) [from milestone gaps]. Add these in TESTING.md Section 5 (standing rule), plus a Section 2 note that Phase 17 adds no suite files. [VERIFIED: highest existing item `**M-156**`; last category heading "Category BG — v1.2 Cleanup (Phase 16)" at TESTING.md:1888]

### Decision rule for FEAS-02 (write it into the matrix before running)
Prefer **in-daemon** (one TCC identity, no helper artifacts) only if R04 is GO (LaunchAgent daemon, GUI closed) and either R06 is GO or R07 is GO (so a CLI/SSH respawn can be routed through launchd). Choose **helper** if R04 fails or the in-daemon identity does not appear in Settings, or R06 and R07 both fail and the product needs SSH-triggered respawn. Count the prompts and list entries (R09) as the tiebreaker: two identities means two monthly approvals (the approval store is keyed by executable path [CITED: community.jamf.com monthly-tccc-prompts thread], so a helper adds a second key). Record crash isolation as a non-measured argument, not evidence.

## Rig procedures

### R00 quick detection (under 10 seconds, no new code needed first)
Cheap shell checks over the existing SSH rig: `sysctl kern.hv_vmm_present` (host prints 0 [VERIFIED]; a guest should print 1 [ASSUMED]), `system_profiler SPDisplaysDataType` (chipset, Metal), and the existing `vshot.sh` path (`screencapture -x` over SSH) with a pixel sample via `px.swift`. Then the `vmprobe` binary (compiled on the host, copied in by `tar`, run via `open` so the grant attaches to the app): it prints in order `hv_vmm_present`; `CGPreflightScreenCaptureAccess`; `SCShareableContent` displays (id, size, scale) or the NSError code; a 5 s `SCStream` with counts per `SCFrameStatus`, first-frame ms, pixel format, `CVPixelBufferGetIOSurface != NULL`, and the first frame saved as PNG with its mean luma (black-frame detector); then `VTCopyVideoEncoderList` plus a session created with Enable-only and the `UsingHardwareAcceleratedVideoEncoder` readback, encoding the captured frames to `.h264` for host-side `ffprobe`. Classify failures with the SDK codes: `-3801` user declined, `-3802` failed to start, `-3811` internal error (the likely IOSurface case), `-3814` no display list, `-3815` no capture source, `-3821` system stopped stream, `-3823` not supported on this platform [VERIFIED: ScreenCaptureKit/SCError.h read from the Xcode 27.2 SDK]. No published Tart/Virtualization.framework evidence exists for SCK or the hardware encoder (several searches returned nothing specific), so treat the guest result as unknown until R00 runs [LOW]. Only the host-recorded fact exists: WebKit screenshots in the guest failed with `IOSurface creation failed` [CITED: memory portty-gui-testing-in-tart-vm.md], while `screencapture -x` over SSH is the rig's working screenshot path.

### Granting permissions in a VM
TCC cannot be pre-granted with SIP on. Sequence per run: launch the app via `open`, let it call `CGRequestScreenCaptureAccess()` / `AXIsProcessTrustedWithOptions` once, then drive System Settings with the existing click tools and read state through `axdump`/`axmenu` (WebKit screenshots are blank, native windows are fine). Reset between runs with `tccutil reset ScreenCapture app.portty.spike` and `tccutil reset Accessibility app.portty.spike`. Snapshot ideas from milestone research (grant once, then clone) are unsafe: a clone of a joined VM duplicates its ephemeral tailnet identity and can knock the original offline.

### Lock-screen procedure in the VM
Preconditions: the rig disabled lock and sleep earlier (memory); re-enable only for this row with `sysadminctl -screenLock immediate -password <pw>` [VERIFIED: usage text `-screenLock <status || immediate || off || seconds> -password <password>`; `sysadminctl -screenLock status` printed "screenLock delay is immediate" on the host, read-only]. Lock with the existing `click key 12 cmd ctrl` (Ctrl+Cmd+Q, keycode 12 = q) or `pmset displaysleepnow` [VERIFIED: man pmset]. `CGSession -suspend` no longer exists on this macOS [VERIFIED: path absent]. Detect lock state with `CGSessionCopyCurrentDictionary()`: on the unlocked host it returns `kCGSSessionOnConsoleKey = 1`, `kCGSessionLoginDoneKey = 1`, and no `CGSSessionScreenIsLocked` key [VERIFIED: local run]; the key is reported to appear only while locked [CITED: community.jamf.com screen-lock threads, anecdotal], so the row records the dictionary before, during and after. While locked, keep a stream running and record per-status frame counts (`Complete`, `Idle`, `Blank`, `Suspended` [VERIFIED: SCFrameStatus in SCStream.h]) and dump one PNG per second to evidence. Typing: send keys through the capture-host process (data channel `key` message to `CGEventPost`), not the SSH `click` tool, because production will inject from that process. Prior evidence that CGEvent typing reaches the lock UI in this VM: the VM was unlocked by typing the admin password as keycodes on 2026-11-15 [CITED: memory portty-gui-testing-in-tart-vm.md]; the row still needs a recorded run from the spike binary. Only the lock screen with the user's session alive is in scope; pre-login is out of scope (REQUIREMENTS Out of Scope).

### Monthly re-approval (R19)
Known from 2024-era sources, to re-verify on 27.2: approvals are stored in `~/Library/Group Containers/group.com.apple.replayd/ScreenCaptureApprovals.plist`, keys are executable paths, values are dates (`2024-09-21 12:40:36 +0000` shape); on 15.1 `replayd` rewrites an app's date at each launch of an app that requests capture; Jamf says the file is TCC-protected, tinyapps says it was edited without Full Disk Access (disagreement) [CITED: community.jamf.com/…/monthly-tccc-prompts-macos-v15/m-p/326764; tinyapps.org/blog/202409180700_disable_sequoia_nag.html]. Procedure: after the first grant, try `plutil -p` on the file over SSH (record success or the denial); relaunch and see whether the date moves; then backdate the entry by 31 days, restart `replayd` (`killall replayd`, launchd restarts it [CITED: tinyapps]) while a stream is running and again before a fresh start, and record: is a prompt shown, what `SCStream` delivers meanwhile (zero frames, an error code, `Blank`), and whether the zero-frame heuristic from PITFALLS catches it. If the file is unreadable, fall back to advancing the guest clock 31 days. **Do the clock jump only on a VM that is not (or no longer) on the tailnet**: a large skew breaks Tailscale key validity and the VM would drop off. A disposable clone of the vanilla image `ghcr.io/cirruslabs/macos-golden-gate-vanilla:27.2` (present locally, [VERIFIED: `tart list`]) has no tailnet identity and suffices for R00-adjacent, R15-R17 and R19 rows that need no tailnet. The real 30-day prompt is an M-NN item. Never read this plist on the host (App Data prompt risk).

### Host run (user-started, about 10 minutes)
Gate: the user says go. Prerequisites the plan prepares: signed app at `spike/desktop/build/` (the repo, not Downloads or Applications), a token, and the bind address the user supplies from `tailscale ip -4` (the spike never calls LocalAPI). Start the app with `open -n <app> --args host-run --bind <ip> --token <t>` so the prompt and the grant name the spike app, not Terminal [ASSUMED]. Sequence in one stream: (1) static text-heavy screen, record SSIM against a decoded viewer frame; (2) motion workload (scrolling, window drag) at 2560 px, 30 fps, 60 s; (3) display sleep via `pmset displaysleepnow`, then wake by the viewer button (`IOPMAssertionDeclareUserActivity`); the host's lock delay is immediate [VERIFIED], so the wake lands on the lock screen, which is also the real-Mac lock row (record frame status and a PNG; the user then unlocks by hand and the spike types nothing secret: one harmless character into the field to observe CGEvent delivery); (4) iPhone Safari opens the link and posts its report. Measure without root: spike self-timing from `SCStreamFrameInfoDisplayTime` to the VT output callback (encode latency); data-channel ping RTT; viewer `getStats` jitter-buffer and decode times (a composite for latency, no camera); `top -l 5 -stats pid,command,cpu -o cpu` for the top processes plus system idle% (the capture and encode CPU lands in `replayd`, WindowServer and the VT encoder service, not in the spike process, so own-process CPU undercounts); `pmset -g therm`; optional `sudo powermetrics --samplers cpu_power,thermal -i 2000 -n 5` (needs root [VERIFIED: "powermetrics must be invoked as the superuser"]). Baseline the same counters for 30 s before starting. Keep screens clean: frames of the user's real desktop must not be committed. Afterward the user can revoke: `tccutil reset ScreenCapture app.portty.spike`. Do not run the VMs hot during the perf window, or record that they were.

## Code Examples

All of the following were run on this host unless marked untested.

### pion server side: lite, one IP, mDNS off, H.264 track, data channel
```go
// Source: loopback harness run 2026-11-24 (pion/webrtc v4.2.29, pion/ice v4.4.9); production binds the tailnet IP
bindIP := net.ParseIP(*bind) // reject unspecified and non-100.64.0.0/10 unless --allow-lan
conn, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: *udpPort})
mux := ice.NewUDPMuxDefault(ice.UDPMuxParams{UDPConn: conn})

se := webrtc.SettingEngine{}
se.SetICEUDPMux(mux)
se.SetLite(*iceMode != "full")                                   // C3 toggle
se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(bindIP) }) // belt and braces; the mux already limits candidates
// se.SetIncludeLoopbackCandidate(true)  // loopback tests only; UDPMuxDefault skips loopback otherwise (gather.go:578)

me := &webrtc.MediaEngine{}
me.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{
    MimeType: webrtc.MimeTypeH264, ClockRate: 90000,
    SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"}, PayloadType: 102}, webrtc.RTPCodecTypeVideo)
api := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me))
pc, _ := api.NewPeerConnection(webrtc.Configuration{})           // ICEServers empty: no STUN/TURN
track, _ := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000,
    SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"}, "video", "spike")
sender, _ := pc.AddTrack(track)
go func() { b := make([]byte, 1500); for { n, _, err := sender.Read(b); _ = n; if err != nil { return } } }() // parse RTCP for PLI/FIR here
// non-trickle: SetRemoteDescription(offer); CreateAnswer; g := webrtc.GatheringCompletePromise(pc); SetLocalDescription; <-g
// per access unit, wall-clock duration from SCK display times:
track.WriteSample(media.Sample{Data: annexBAccessUnit, Duration: elapsedSinceLastSample})
```
Result observed: answer contained `a=ice-lite`, exactly one candidate (`typ host` on the mux IP:port, two component lines), `a=setup:active`, `a=fmtp … profile-level-id=42e01f`; ICE `connected`; DataChannel ping returned `pong:ping`; 197 RTP packets for 120 frames; the receiver's Annex-B output decoded with `ffmpeg` and `ffprobe` reported Constrained Baseline level 50 at 2560x1600. `ICE=full` also connected [VERIFIED: local harness]. With mDNS disabled, remote `.local` candidates are logged and ignored; pion learns the browser from its checks [VERIFIED: pion/ice agent.go:1019-1023 `Remote mDNS candidate added, but mDNS is disabled`]. Practical consequence for C3: the browser is always full ICE, so "lite vs full" only changes whether pion initiates checks; expect equal connectivity unless a browser offers a non-mDNS candidate. Let the viewer report decide per browser.

### VideoToolbox session (verified settings)
```objc
// Source: vtprobe run 2026-11-24, M3, macOS 27.2.1
spec[kVTVideoEncoderSpecification_EnableHardwareAcceleratedVideoEncoder] = @YES;   // do NOT set Require... in the guest
VTCompressionSessionCreate(NULL, w, h, kCMVideoCodecType_H264, spec, NULL, NULL, cb, ctx, &s);
VTSessionCopyProperty(s, kVTCompressionPropertyKey_UsingHardwareAcceleratedVideoEncoder, NULL, &hw); // record this
VTSessionSetProperty(s, kVTCompressionPropertyKey_RealTime, kCFBooleanTrue);
VTSessionSetProperty(s, kVTCompressionPropertyKey_AllowFrameReordering, kCFBooleanFalse);
VTSessionSetProperty(s, kVTCompressionPropertyKey_ProfileLevel, kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel);
// AverageBitRate, ExpectedFrameRate, MaxKeyFrameInterval (long), BT.709 primaries/transfer/matrix tags
// per output: key = !kCMSampleAttachmentKey_NotSync; on key, emit SPS+PPS from CMVideoFormatDescriptionGetH264ParameterSetAtIndex,
// then replace each 4-byte AVCC length with 00 00 00 01.
// request IDR: pass @{kVTEncodeFrameOptionKey_ForceKeyFrame: @YES} as frame properties
```
Observed: encoder list on this host has `com.apple.videotoolbox.videoencoder.ave.avc` (hardware) and `com.apple.videotoolbox.videoencoder.h264` (software); 1920x1080 SPS `42c028`, 2560x1600 SPS `42c032` (level 5.0); hardware in use at both sizes with the Require flag; average encode latency 10 ms at 1080p and 16 ms at 2560x1600 (max 24 ms) for a synthetic moving checkerboard, 30 fps. `ri_user_time`/`ri_system_time` from `proc_pid_rusage` are mach ticks, not nanoseconds: convert with `mach_timebase_info` (unconverted it reported 0.03 s, converted 1.03 s for the same run) [VERIFIED: local]. Even with only Enable set the HW encoder was chosen on this host; the guest result comes from the readback, not from assumption.

### Opus via AudioConverter (verified)
```objc
// Source: opusprobe run 2026-11-24
in  = {48000, LinearPCM, Float|Packed, 8 bytes/frame, 2 ch, 32 bit};
out = {48000, kAudioFormatOpus, mChannelsPerFrame=2, mFramesPerPacket=960};
AudioConverterNew(&in, &out, &c);
AudioConverterSetProperty(c, kAudioConverterEncodeBitRate, 4, &(UInt32){96000});
// loop: AudioConverterFillComplexBuffer(c, feedPCM(960 frames per call), NULL, &packets=1, &outList, &pd);
```
Observed: converter created, `framesPerPacket=960`, prime leading 312 frames, first packet 442 bytes with TOC `0xfc`, 481 packets for 480 input blocks, average 58.8 kbps for a sine at a 96 kbps target; packets are raw Opus (what RTP carries). Each packet is one `media.Sample{Duration: 20ms}`. Status: R21 encode portion = GO on macOS 27.2.1 [VERIFIED: local]. SCK system-audio capture remains a run row.

### SCK stream (compile-checked only; never run on the host)
`SCStreamConfiguration`: `width`, `height`, `minimumFrameInterval = CMTimeMake(1,30)`, `pixelFormat = kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange`, `queueDepth = 5`, `showsCursor`, `colorMatrix = kCGDisplayStreamYCbCrMatrix_ITU_R_709_2`, `capturesAudio`, `sampleRate = 48000`, `channelCount = 2`, `excludesCurrentProcessAudio`; per frame read `SCStreamFrameInfoStatus`, `…DisplayTime` (mach absolute time when the window server displayed it), `…ScaleFactor`, `…ContentRect`; `SMAppService agentServiceWithPlistName:` / `registerAndReturnError:` / `status` (`SMAppServiceStatusEnabled`, `…RequiresApproval`, `…NotRegistered`, `…NotFound`) / `openSystemSettingsLoginItems`; `CGPreflightScreenCaptureAccess`, `CGRequestScreenCaptureAccess`, `AXIsProcessTrusted`, `CGPreflightPostEventAccess`, `CGRequestPostEventAccess`, `CGSessionCopyCurrentDictionary`, `IOPMAssertionDeclareUserActivity`, `IOPMAssertionCreateWithName(kIOPMAssertPreventUserIdleDisplaySleep…)`. All compiled with `clang -fobjc-arc -fsyntax-only -mmacosx-version-min=13.0` against SDK 27.2 [VERIFIED: local compile, exit 0]. If 420v is refused in the guest, retry BGRA and let VT convert.

### SMAppService helper plist and bundle layout
```
Spike.app/Contents/MacOS/spike                         # registrar + daemon shape (same Mach-O as production pattern)
Spike.app/Contents/Helpers/spike-helper                # identifier app.portty.spike.helper
Spike.app/Contents/Library/LaunchAgents/app.portty.spike.helper.plist
```
```xml
<dict>
  <key>Label</key><string>app.portty.spike.helper</string>
  <key>BundleProgram</key><string>Contents/Helpers/spike-helper</string>
  <key>ProgramArguments</key><array><string>spike-helper</string><string>helper-probe</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><false/>
  <key>LimitLoadToSessionType</key><string>Aqua</string>
  <key>ProcessType</key><string>Interactive</string>
  <key>StandardOutPath</key><string>/tmp/spike-helper.log</string>
  <key>StandardErrorPath</key><string>/tmp/spike-helper.log</string>
</dict>
```
`BundleProgram` is an app-bundle-relative path and is only supported for plists installed through SMAppService; `LimitLoadToSessionType` limits an agent to the named session types [VERIFIED: man launchd.plist, read locally]. Apple documents the layout (`Contents/Library/LaunchAgents`, `BundleProgram` relative path) and that bundle agents appear in Login Items under the app [CITED: developer.apple.com/documentation/servicemanagement/updating-helper-executables-from-earlier-versions-of-macos]. **`ProcessType` matters:** unspecified means "light resource limits … throttling its CPU usage and I/O bandwidth"; `Interactive` means "the same resource limitations as apps, that is to say, none" [VERIFIED: man launchd.plist]. The kardianos plist Portty uses today sets no `ProcessType`, so the legacy-plist daemon path is subject to that throttle [VERIFIED by template read; throttle effect on a real stream is a measured sub-row of R04: run it once with and once without `ProcessType Interactive` and compare fps and encode latency]. Registration status after `register()` and `sfltool dumpbtm` (root) go into R02 evidence. Whether `register()` works from `~/ptest` or needs `/Applications`, and whether first registration reports `requiresApproval`, is unknown [ASSUMED]; the probe prints the status and error either way.

### Viewer page core (vanilla; not yet run in a browser)
```html
<video id=v autoplay playsinline muted></video><pre id=log></pre>
<script>
const q = new URLSearchParams(location.search), tok = q.get('t');
const pc = new RTCPeerConnection({ iceServers: [] });
pc.addTransceiver('video', { direction: 'recvonly' });
const dc = pc.createDataChannel('control');
pc.ontrack = e => { v.srcObject = e.streams[0] || new MediaStream([e.track]); };
const t0 = performance.now();
await pc.setLocalDescription(await pc.createOffer());
await new Promise(r => { if (pc.iceGatheringState === 'complete') return r();
  pc.onicegatheringstatechange = () => pc.iceGatheringState === 'complete' && r(); setTimeout(r, 2500); });
const ans = await (await fetch('/offer?t=' + tok, { method: 'POST', body: JSON.stringify(pc.localDescription) })).json();
await pc.setRemoteDescription(ans);
// after 10 s: pc.getStats() -> candidate-pair (nominated/succeeded), inbound-rtp video, codec; POST JSON to /report?t=
</script>
```
Bare WKWebView on this host exposes `RTCPeerConnection`, `RTCDataChannel`, `requestVideoFrameCallback`, `navigator.clipboard.writeText`, Opus receive, and H.264 receive with `640c1f` and `42e01f` (packetization-mode 1 and 0) in `RTCRtpReceiver.getCapabilities` [VERIFIED: offscreen WKWebView run, no RTCPeerConnection constructed so no Local Network prompt]. R13 still needs the Wails configuration and a constructed connection.

## Common Pitfalls

### Pitfall 1: SDP says `42e01f`, bitstream says `42c0xx`
**What goes wrong:** The VT hardware encoder wrote SPS profile 0x42, constraint 0xc0 (profile-level-id `42c028`/`42c032` observed) while browsers offer `42e01f` and pion matches only the first two bytes, so registering the real value (`42c0…`) would fail negotiation against browsers that offer `42e0`.
**Why it happens:** `profileLevelIDMatches` compares `aa[0] == bb[0] && aa[1] == bb[1]` [VERIFIED: pion/webrtc v4.2.29 internal/fmtp/h264.go:10-25].
**How to avoid:** Advertise `42e01f` in the MediaEngine and track (verified to negotiate and decode via ffmpeg). The R11/R12/R13/R14 reports record decode success per browser. If Safari or WKWebView refuses, patch SPS byte 2 from `0xc0` to `0xe0` during Annex-B conversion and re-test.
**Warning signs:** Connected but zero `framesDecoded`; a profile mismatch in the viewer report's codec entry.

### Pitfall 2: Level 5.0 at 2560x1600
**What goes wrong:** The SPS level is `0x32` (5.0) at the default cap, above the 3.1 the browsers offer.
**How to avoid:** Keep `level-asymmetry-allowed=1`; the matrix measures decode of the 2560 px stream per browser (C4 and C5). Do not lower the level byte to satisfy SDP.

### Pitfall 3: Frames only on change; new viewer sees nothing
**What goes wrong:** `SCFrameStatusIdle` means "the display did not change"; a still screen gives no frames to encode, so a joining viewer (and Safari's stalled-stream heuristic) sees black [VERIFIED: SCFrameStatus doc in SCStream.h].
**How to avoid:** Keep the last pixel buffer; on viewer join and PLI, re-encode it with `ForceKeyFrame`; send a periodic refresh during idle. Row R22 proves it.

### Pitfall 4: Attributing the wrong process
**What goes wrong:** Running a probe from an SSH shell, or the host-run binary from Terminal, grants or tests the wrong identity.
**How to avoid:** One probe function, launched by each path as defined in Pattern 2; GUI paths use `open`; evidence is the tccd `responsible_path`, not what the probe thinks.

### Pitfall 5: Own-process CPU undercounts
**What goes wrong:** Capture work runs in `replayd`/WindowServer and hardware encode in the media engine/VT service; the spike's own CPU looks tiny.
**How to avoid:** Record system idle% and top-N processes for a baseline and for the run.

### Pitfall 6: Spike reaches the shipping build or trips gates
**What goes wrong:** Banned words in spike files fail `tests/check-brand.sh` (checked on tracked files outside `.planning/`); a stray import couples the modules.
**How to avoid:** `spike/desktop/` has its own `go.mod`, which the root module's `./...` excludes [VERIFIED: nested-module scratch test, `go list ./...` listed only the outer package]. Run `bash tests/check-brand.sh` and the baseline `GOTOOLCHAIN=go1.26.11 go test -race -short ./...` [VERIFIED: TESTING.md Section 2 run command] before and after. At phase close remove or archive the directory in its own commit. Build outputs under `spike/desktop/build/` must be git-ignored; the root `.gitignore` ignores `/portty`, `build/bin/` and others but nothing for `spike/` [VERIFIED: .gitignore read], so add an ignore line in the spike commit.

### Pitfall 7: Clock jump knocks the VM off the tailnet
See R19. Do it only on a non-tailnet VM.

### Pitfall 8: Stopping the VMs deletes them from the tailnet
Ephemeral nodes disappear after hours offline [CITED: memory portty-vms-ephemeral-tailnet.md]. Keep both running until every tailnet row is done, and tell the user before shutting down. The user must run `tailscale lock sign`; the classifier blocks the agent from doing it [CITED: memory].

### Pitfall 9: Evidence leaks
Host-run frames and PNGs can contain the user's real desktop; viewer reports contain tailnet addresses and the token. Redact before committing under `.planning/`; keep host-run PNGs to test content only.

### Pitfall 10: Tooling gaps in the guest
No swiftc (build on the host and `tar`-copy, as `vdeploy.sh` does), no ffmpeg (copy `.h264` back to the host), `vm.sh` has the VM IP hard-coded (update after each boot, `tart ip`) [CITED: memory].

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `CGSession -suspend` to lock | Ctrl+Cmd+Q via CGEvent, or `pmset displaysleepnow` with immediate lock | Binary absent on 27.2.1 [VERIFIED] | Lock procedure uses the click tool |
| `Program` key in a plist | `BundleProgram` for SMAppService | macOS 13 | Helper path is bundle-relative |
| Persistent Screen Recording consent | Monthly approval stored by `replayd` | macOS 15 | R19; entitlement request is the user's todo |
| pion `SetNAT1To1IPs` | Mux on the real IP; rewrite rules exist in v4.2.29 (`SetICEAddressRewriteRules`) | v5 removes NAT1To1 | Not needed with a tailnet IP |

**Deprecated/outdated:** pion v5 release candidates (not used); `hdiutil` stderr noise is a Phase 18 concern, not this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | SCK frames may fail in a Tart guest (IOSurface) and `VTCopyVideoEncoderList` may lack a HW encoder there | Rig R00 | If it works fully, more rows stay in VMs; harmless |
| A2 | `register()` of an SMAppService agent works for a Developer ID signed, non-notarized, non-quarantined app run from `~/ptest` | Pattern 2, plist | If it needs `/Applications` or approval, the helper rows need that step first |
| A3 | A process spawned over SSH is attributed to the sshd wrapper; a launchd job is its own responsible process; a GUI-spawned child shares the GUI app's grant | Pattern 2/3 | These are what the rows measure; a different result changes the decision, not the method |
| A4 | `open -n <app> --args …` makes the app (not Terminal) the responsible process | Host run | Prompt would name Terminal and grant the wrong identity |
| A5 | `ScreenCaptureApprovals.plist` still exists on 27.2 with the 2024 semantics, and backdating or a clock jump triggers the prompt | R19 | Fall back to observation plus an M-NN |
| A6 | SCK behaviour at lock and display sleep (frames, Blank/Suspended, errors) | R15/R18 | None assumed; the rows measure it |
| A7 | `CGSSessionScreenIsLocked` appears only while locked | R17 | Lock-state source for DPRM-02 may differ |
| A8 | Non-notarized Developer ID code with no quarantine attribute launches without a Gatekeeper dialog | Signing | Spike binaries would need notarization (slows iteration) |
| A9 | Safari/WKWebView decode `42c0xx` SPS under a `42e01f` SDP | Pitfall 1 | SPS byte patch needed |
| A10 | Chrome can be installed in the home VM (arm64 download) | R12 | Chrome row moves to a user-run host step |
| A11 | `tailscale ping` in the VM shows whether the path is direct or DERP | R01 | Path type from `getStats` pair types only |
| A12 | First `codesign` with the Developer ID key may show a Keychain dialog | Signing | Extra checkpoint, otherwise none |
| A13 | `launchctl kickstart -k gui/<uid>/<label>` over SSH restarts the job in the Aqua session | Pattern 2 E | R07 NO-GO; SSH respawn stays unsolved |
| A14 | `launchctl managername` over SSH prints Background | Pattern 3 | Cosmetic; use the CGSession dictionary instead |
| A15 | A wake via `IOPMAssertionDeclareUserActivity` works in the VM's virtual display | R18 | Row is host-only (already planned) |
| A16 | Wails WKWebView behaves like a bare WKWebView for RTCPeerConnection | R13 | Row answers it directly |

## Open Questions

1. **What macOS does the office Mac run?**
   - What we know: unknown (CONTEXT); the host and VMs are 27.2.x.
   - What's unclear: lock UI and permission differences.
   - Recommendation: record per row; ask the user for `sw_vers` during the first checkpoint and add M-NN items if it differs.
2. **Does SCK work in the Tart guest at all?**
   - Recommendation: R00 first; plan carries the conditional host scope.
3. **Who is the viewer in the host run?** The VMs can view the host over the tailnet only if they are up; the iPhone can.
   - Recommendation: iPhone is the required viewer (also R14); VM home Safari optional, record whether VMs were running during the perf window.
4. **Does the user want Chrome row evidence from the host's Chrome?** Needed only if the VM Chrome install fails.
5. **Does SMAppService registration need `/Applications`?** Answered by R02 on first run.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | spike build | ✓ | go1.27.4 (module pins 1.26.4; root repo uses 1.26.11) [VERIFIED] | `GOTOOLCHAIN=local` worked for pion builds |
| Xcode / clang / SDK | cgo ObjC, compile checks | ✓ | Xcode 27.2, SDK 27.2 [VERIFIED] | — |
| swiftc | optional | ✓ host, ✗ guest [CITED: memory] | — | Build on host |
| Wails CLI | R13 shell | ✓ | v2 (`~/go/bin/wails`); v2.10.2 library [VERIFIED] | — |
| ffmpeg / ffprobe | sharpness, stream checks | ✓ host | — | — |
| tart | VMs | ✓ | 2.41.1; VMs `portty-test`, `portty-test-2` exist and are stopped; vanilla 27.2 image present [VERIFIED: `tart list`] | — |
| Developer ID identity | signing | ✓ | 1 valid: `Developer ID Application: Ken Scott (S2K7P43927)` [VERIFIED: `security find-identity`] | — |
| Tailnet in VMs | R01, R11-R14 | ✗ until re-keyed | — | Tart LAN with `--allow-lan` gives partial results only (not the utun path) |
| Chrome in VM | R12 | ✗ unknown | — | User-run host Chrome |
| `sudo` in guest | `procinfo`, `sfltool`, `tcpdump` | ✓ [CITED: memory, admin account] | — | tccd log only |
| `powermetrics` | optional energy data | root only [VERIFIED] | — | `top`, `pmset -g therm`, rusage |

**Missing dependencies with no fallback:** none for the spike as a whole.
**Missing dependencies with fallback:** VM tailnet membership (user checkpoint), Chrome in guest.

## Validation Architecture

> `workflow.nyquist_validation` is `false` in `.planning/config.json`, so the standard test-map section is skipped. This section is included because the orchestrator asked how each matrix row is evidenced. The phase adds no automated suite; success is the matrix plus gates.

| Row | Evidence artifact (under `evidence/`) | Producer | Checked by |
|-----|---------------------|----------|-----------|
| R00 | `r00-vmprobe.json`, `r00-frame.png`, `r00.h264` + `ffprobe` text | `vmprobe` | Read the JSON and PNG |
| R01/R23 | `r01-report-*.json`, `r01-lsof.txt`, `r01-tcpdump.txt` | viewer page, `lsof`, `tcpdump` | Pair types in report; no non-tailnet bind or 3478/5353 traffic |
| R02-R08 | `r0x-probe-<path>.json`, `r0x-tcc.log` (filtered to pid), `launchctl print`, `sfltool dumpbtm`, `procinfo` | `probe`, `log show`, sudo tools | `responsible_path` line equals expected identity |
| R09 | `r09-settings-ax.txt` | `axdump` on System Settings | List contains/omits entry |
| R10 | before/after probe JSON, `codesign -dr-` diff | probe + codesign | Same DR, capture still works |
| R11-R14 | `r1x-report-<browser>-<ice>.json` | viewer page | `framesDecoded > 0`, size, codec entry |
| R15-R17 | `r15-status-counts.json`, PNG per second, CGSession dict snapshots, `r16-unlock.json` | probe + frames | Human reads PNGs (Read tool) |
| R18 | `r18-timeline.json`, `pmset -g assertions` | host run | Transition times |
| R19 | `r19-plist.txt` or denial text, prompt observation notes with AX dump | probe + `axdump` | Backdated behaviour recorded |
| R20 | `r20-top.txt`, `r20-latency.json`, `r20-ssim.txt`, `pmset -g therm` | host run, ffmpeg | Numbers in matrix |
| R21 | `r21-opus.txt` (local run output) | `opusprobe` | 960-frame packets |
| R22 | `r22-join.json` | viewer + server timestamps | First frame < 1 s on a still screen |

**Gates (phase level, SC4):** `GOTOOLCHAIN=go1.26.11 go test -race -short ./...` and `bash tests/check-brand.sh` green before and after; `go list ./...` at the repo root lists no `spike` package; `git diff main --stat` outside `spike/`, `.planning/` and `TESTING.md` is empty; no change to `release.yml`, `build/`, `Info.plist`; `bash tests/check-traceability-paths.sh` still passes (no new test files, so no traceability rows). **Wave 0 gaps:** the spike module, `17-SPIKE-MATRIX.md` skeleton, `evidence/` directory, `.gitignore` line for `spike/desktop/build/`.

## Security Domain

`security_enforcement` is enabled (absent or true) in config; ASVS level 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (spike) | Random 128-bit token in the URL as the only gate; a real design is Phase 19/22 |
| V3 Session Management | no | Single-run process; killed after the run |
| V4 Access Control | yes | Token on `/viewer.html`, `/offer`, `/report`; data-channel input disabled unless `--allow-input`; bind only to the supplied tailnet IP |
| V5 Input Validation | yes | Cap SDP and JSON body size, reject malformed offers, validate `--bind` |
| V6 Cryptography | no | DTLS-SRTP from pion; no custom crypto |
| V7/V8 Logging, Data Protection | yes | No tokens, passwords or tailnet names in tracked evidence; no desktop frames of the real host desktop committed |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Any tailnet node reaches an open desktop stream | Information disclosure | Token, tailnet-IP-only bind, short run, kill and revoke after |
| Remote input injection into a shell-capable Mac | Elevation | `--allow-input` off by default; keys only in the lock-screen rows; never type a real password through the spike |
| Spike left running or grant left on the host | Information disclosure | End-of-run checklist: stop process, `tccutil reset ScreenCapture app.portty.spike`, remove LaunchAgents, `sfltool`/unregister in VMs |
| Wildcard UDP bind or mDNS multicast exposure | Information disclosure | Explicit mux on one IP, mDNS Disabled; R23 `lsof` check |
| Evidence containing secrets or private screenshots | Information disclosure | Redact; PNGs from the host run limited to test content |

## Project Constraints (from CLAUDE.md)

- `TESTING.md` is the regression home: update it for any test/behavior change; manual items that cannot be automated become M-NN rows in Section 5 (next free: M-157, category BH); traceability rows only for new test files covering v4.0+ requirements (none expected); run `bash tests/check-traceability-paths.sh` before committing.
- MIT license, tailnet-only network, keep CSP intact, token-gated share links; Chesterton's Fence before removing anything (the spike removes nothing).
- Go: `go fmt`, context-aware functions; `golangci-lint` conventions; Python venv rules do not apply.
- "Silent fallbacks: let it crash": probes must fail loudly and record the error, not default to success.
- Do not make repo edits outside a GSD workflow; spike edits happen under `/gsd-execute-phase`.
- Never kill Node processes; prefer LSP/graft for code navigation; VM GUI checks are driven by the agent, not handed to the user (user feedback), except the checkpoints that physically need the user (re-key, host-run go, iPhone).
- Research constraints: no contact with the host's tailscaled or any 100.x address; do not start, stop or modify the Tart VMs during research; host experiments only on scratch files.

## Sources

### Primary (HIGH confidence)
- Local experiments this session on macOS 27.2.1, Apple M3, Xcode 27.2: VideoToolbox H.264 probe, AudioConverter Opus probe, pion loopback harness (lite and full), offscreen WKWebView feature probe, `CGSessionCopyCurrentDictionary` dump, `log show` TCC lines, compile-only SCK/SMAppService/IOKit API check, Wails shell build, nested-module `go list` test, `launchctl procinfo` and `powermetrics` privilege checks.
- Source read: pion/webrtc v4.2.29 (`settingengine.go`, `internal/fmtp/h264.go`, `icegatherer.go`, `track_local_static.go`, `examples/ice-single-port`), pion/ice v4.4.9 (`gather.go`, `agent.go`, `udp_mux.go`), kardianos/service v1.2.4 `service_darwin.go`, repo files `internal/daemon/service.go`, `internal/daemon/process_unix.go`, `build/entitlements.plist`, `.github/workflows/release.yml`, `tests/check-brand.sh`, `TESTING.md`, `.gitignore`, SDK headers `SCStream.h`, `SCError.h`, `CGWindow.h`.
- proxy.golang.org `@latest` and `@v/list` for pion modules.
- `man launchd.plist`, `man pmset`, `man caffeinate`, `sysadminctl` usage, `tccutil` usage, `tart list`, `tart get`.

### Secondary (MEDIUM confidence)
- developer.apple.com SMAppService and "Updating helper executables" docs (bundle layout, `BundleProgram`, Login Items association).
- community.jamf.com monthly-tccc-prompts thread and tinyapps.org (approval plist path, key and value shape; 2024 beta era).

### Tertiary (LOW confidence)
- Web searches for SCK/VideoToolbox behaviour in macOS guest VMs and for SCK frames at the lock screen returned nothing specific; Jamf/Kolide screen-lock key anecdotes; WebRTC mDNS/Tailscale forum posts.
- Memory notes (`portty-gui-testing-in-tart-vm.md`, `portty-vms-ephemeral-tailnet.md`, `portty-no-live-tailnet-in-research.md`) for rig facts (tools, lock/unlock history, ephemeral keys).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. Versions confirmed on the proxy, APIs exercised locally.
- Architecture and probe design: MEDIUM. Mechanisms verified (tccd log, plist keys, session dict); outcomes (attribution per path, SMAppService from a dev path) are exactly what the spike measures.
- Pitfalls: MEDIUM. Pion/VT/rusage/fmtp pitfalls verified; VM, lock and approval behaviour unverified by design.

**Research date:** 2026-11-24
**Valid until:** 2026-12-08 for macOS behaviour (27.x point releases change TCC prompts); pion pin stable until a v4 patch matters.
