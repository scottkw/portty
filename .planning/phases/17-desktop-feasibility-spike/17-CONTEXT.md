# Phase 17: Desktop Feasibility Spike - Context

**Gathered:** 2026-11-24
**Status:** Ready for planning

<domain>
## Phase Boundary

A throwaway end-to-end path (ScreenCaptureKit → VideoToolbox H.264 → pion/webrtc v4 → plain HTML viewer) shows a Mac's desktop live over the tailnet, and a recorded go/no-go matrix answers every FEAS-01 question with measured evidence. FEAS-02: the capture-host decision (bundled helper vs in-daemon) is recorded with evidence before Phase 19 is planned. No spike code reaches the shipping build. Production capture, UI and share modes are later phases.

</domain>

<decisions>
## Implementation Decisions

### Test rig
- The Tart VMs carry the spike: `portty-test` (macOS 27.2, tailnet `portty-vm-office`) is the shared Mac, `portty-test-2` (`portty-vm-home`) is a viewer. VM rows cover permission attribution (helper vs daemon, SSH respawn, GUI closed), lock-screen capture and CGEvent typing, browser/Wails/iPhone connections over the tailnet, WebRTC in the Wails web view, and Opus encode. User asked "Can we test with the VMs you have used in the past?"
- Two rows run on a real Mac because VMs lack them: hardware-encoder CPU/latency/sharpness and display sleep/wake. They run on the user's personal MacBook Air (macOS 27.2.1, this host) as one short (~10 min) user-started run. The user will see a Screen Recording prompt for the spike binary and may revoke it afterwards. Never start that run without the user saying go.
- First spike step checks whether ScreenCaptureKit returns frames in a Tart VM at all (WebKit screenshots there have failed with `IOSurface creation failed`). If VM capture fails, the affected rows move to the host run and the matrix records why.
- The VMs were stopped ~20 h ago and use ephemeral tailnet keys, so re-joining likely needs the user (new auth key + `tailscale lock sign`). Plans must include that as a user checkpoint.
- The office Mac's macOS version is not yet known; the matrix records the macOS version for every row, and if the office Mac differs from 27.2.x, lock-screen and permission rows get an M-NN real-Mac item.

### Capture-host decision method (FEAS-02)
- Two tiny permission probes, one per shape: (a) a helper inside a test .app bundle launched as a LaunchAgent via SMAppService, (b) capture inside the daemon-style process (same signed binary spawned detached, also respawned from an SSH session). Each records where macOS attributes the Screen Recording and Accessibility grants (`launchctl procinfo` responsible process, TCC list entry), whether capture works with the GUI closed, and across lock/unlock. The full video pipeline runs in only one shape. — **Reversibility:** costly — the result fixes the process architecture, bundle layout, signing and TCC identity every later phase builds on.

### Portty-app check substitution
- User accepted (2026-11-24): the ROADMAP SC2 "Portty app" connection row is answered by a throwaway Wails v2 shell (same WKWebView engine and the app's CSP meta), not the real Portty app. Real-app integration (bundle id, React UI, signalling) is proven in Phase 21 (DVWR-01).

### UI safety gate waiver
- User decision (2026-11-24, during wave 1): the UI safety gate (execute:wave:post ui.safety-gate) is waived for Phase 17. spike/desktop/viewer.html and any spike shell UI are throwaway test tooling, not product UI, so no UI-SPEC.md. Product UI phases (19-22) still get UI-SPECs.

### Timebox
- No timebox: every matrix row gets a real answer before Phase 19 planning. Rows that genuinely cannot be answered by a spike (e.g. the real 30-day re-approval prompt) are answered with the strongest available evidence and an explicit M-NN follow-up.

### Decided autonomously
- [Spike code] Q: "Where does spike code live?" → Selected: "A separate Go module under `spike/desktop/`, never imported by the app module, removed or archived when the phase closes" — throwaway by design
- [Signing] Q: "How are spike binaries signed?" → Selected: "With the user's Developer ID Application identity (team S2K7P43927), so TCC behaviour matches a real release; never ad-hoc" — a build flag
- [Monthly re-approval] Q: "How is the monthly Screen Recording re-approval measured?" → Selected: "Read the grant's stored expiry/consent timestamp and record observed behaviour; the real 30-day prompt becomes an M-NN item" — observation only. SUPERSEDED 2026-11-24 by user decision: backdating the approval record or jumping the clock IS allowed, but only inside a disposable VM clone that is off the tailnet; the host and the tailnet VMs are never written
- [Viewer] Q: "What does the test viewer look like?" → Selected: "One plain HTML page served by the spike, vanilla RTCPeerConnection, no React" — thrown away

### Claude's Discretion
- Exact probe implementation (cgo ObjC vs small Swift helper for the helper shape), order of matrix rows after the VM-capture check, matrix file format.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/webserver` share server binds the tailnet IP (no tsnet) with LocalAPI WhoIs gating; the spike can bind pion's UDP mux on the same tailnet IP.
- Existing cgo `.m` files in the repo show the ObjC bridging pattern.
- `internal/updater` Runner seam and release.yml signing steps show how binaries are signed and notarized.
- VM tooling in `~/.local/share/portty-vm-tools` (`vm.sh`, `vshot.sh`, `click.swift`, `axdump`, `axmenu`) drives clicks, prompts and accessibility reads in the guest.

### Established Patterns
- Daemon is the same Mach-O as the GUI (`Portty.app/Contents/MacOS/portty daemon`), spawned detached (Setsid) by GUI or CLI; it can be respawned from an SSH session.
- Research findings are in `.planning/research/` (SUMMARY.md conflicts C1, C3, C4, C5, C7).

### Integration Points
- None into production code in this phase; the spike is a separate module.

</code_context>

<specifics>
## Specific Ideas

- Research constraints carry over: subagents never contact the host's tailscaled or 100.x addresses (VM rows go through the VMs), and host experiments never mount, copy or modify the user's real files (~/Downloads, /Applications); scratch-only, detach anything attached.
- Matrix must include per-browser full ICE vs ICE-lite results (C3), the 2560 px / 30 fps default cap measurement (C4), Constrained Baseline decode (C5), and AudioConverter Opus (C7).
- File-the-entitlement todo stays with the user; not a spike deliverable.
- User answers after phase research (2026-11-24): the office Mac runs macOS 27.2.x, same as the VMs and this host, so VM lock-screen and permission rows transfer (no extra office-Mac row needed for version mismatch). The host run's viewer is a VM browser (not the iPhone); the iPhone Safari row runs separately against a VM. Chrome row: try Chrome in the VM first; if it will not install or run there, fall back to Chrome on the host opening only the spike's test page.

</specifics>

<deferred>
## Deferred Ideas

- HEVC (H.265) over WebRTC: user chose "Not now" (2026-11-24). v2.0 stays H.264 Constrained Baseline only; no HEVC spike row. Revisit after v2.0.

</deferred>
