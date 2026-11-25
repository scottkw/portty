# Phase 23: Audio, Adaptive Quality, Unlock and Release - Research

**Researched:** 2026-11-25
**Domain:** macOS system-audio capture and Opus over WebRTC (pion), sender-side adaptive video quality (GCC/TWCC + VideoToolbox), remote lock-screen unlock (CGEvent typing), Apple Silicon signing/notarization/release pipeline, upgrade and soak hardening
**Confidence:** MEDIUM overall. Release pipeline and updater compatibility are HIGH (read and run against real files this session). Audio, adaptive quality and unlock are MEDIUM/LOW at the detail level because Phases 19-22 are designs only (not built) and both Tart VMs were unreachable this session, so every VM-dependent item below is marked unverified.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, and product and UX choices are under Claude's Discretion. The user accepted the five hard-to-reverse recommendations below with "Accept all" (2026-11-25). Audio, adaptive quality and hardening had no hard-to-reverse questions.

#### Release packaging and signing
- The v2.0 release publishes two DMGs built from one arm64-only build: `portty-vX.Y.Z-darwin-universal.dmg`, kept only so v1.2.x copies find the update, and `portty-vX.Y.Z-darwin-arm64.dmg`. Each DMG is signed with `--identifier` equal to its name without .dmg, and each is notarized, stapled and checked the way the updater checks it. Both go into checksums.txt. Both names keep shipping until a later release decides to drop the universal name, which is safe once both Macs run a build from Phase 18 on — **Reversibility:** one-way — release asset names are a published contract read by every installed copy (v1.2's updater knows only the universal name and its identifier), so a missing name strands that copy's update path.
- Signing switches from `codesign --deep` to explicit inside-out signing of every Mach-O in the bundle. Today that is only Contents/MacOS/Portty, because capture runs in the daemon. Signing keeps identifier `app.portty.Portty`, hardened runtime, timestamp and the unchanged entitlements file (network client and server only); SCK audio and CGEvent typing need no entitlement. CI enumerates every Mach-O. It fails on unsigned code, the wrong team, a missing runtime flag, a non-arm64 slice or a minos below 14.0. It also pins the designated requirement to the v1.2 one — **Reversibility:** one-way — the office Mac's Screen Recording and Accessibility grants are keyed to the designated requirement, so a changed identifier or requirement silently drops them and costs an in-person trip to the unattended Mac.
- v2.0 does not wait for the persistent-content-capture entitlement (request T558S9Q5B3). The signing step is written so a provisioning profile and one entitlement can be added later without other changes. If Apple grants the request before Phase 23 planning closes, wiring it becomes a separate last plan. That plan runs only after the plain release pipeline is green and has been tested on the two-build rig; Phase 18 rollback covers a profile that will not launch — **Reversibility:** one-way — an embedded profile and a restricted entitlement change the signature every later build must carry and tie the app to a profile that expires; a bad profile stops the app launching on the remote Mac.

#### Remote unlock
- Each viewer gets a dedicated Unlock field, shown only to own-account viewers while the state is Locked. Submit sends the password as one message on the encrypted control data channel: DTLS end to end to the daemon, never the signalling socket and never a URL. The daemon holds it in one byte buffer only for the attempt: wake key, wait for the field, type, check, at most one retry, about 10 s. It then zeroes the buffer. The password is never logged and never appears in the audit log, not even its length. It is never persisted, and it is kept out of error messages and crash state. The viewer clears its field on submit and keeps nothing — **Reversibility:** one-way — this is the security boundary for a macOS login password and a wire message every viewer build sends; UNLK-01's retry-once needs the daemon to retype, which keystroke passthrough cannot do.
- While CGSSessionScreenIsLocked is set, the daemon drops ordinary key events from every viewer, own-account and guests alike. The Unlock field is the only way to type at the lock screen. Own-account pointer input still passes, and guest pointer input is dropped too — **Reversibility:** costly — it is a security rule that Phase 20-22 input code must keep (guests with control could otherwise type at the lock screen, against SC3); loosening it later is easy, tightening it after people rely on typing is not.

#### Decided autonomously
- [Audio] Q: "When is audio captured and sent?" → Selected: "Whenever the desktop stream runs, on the same SCStream with excludesCurrentProcessAudio on; mute is on the viewer only; silence costs a few bytes per packet (R21 control run: 3-byte packets)" — runtime policy
- [Audio] Q: "Opus parameters?" → Selected: "48 kHz stereo, 20 ms frames (960), about 64 kbps target, SDP opus/48000/2 with stereo=1; AudioConverter recipe from spike media_darwin.m" — encoder constants
- [Audio] Q: "How do audio and video stay in sync?" → Selected: "Separate audio and video tracks in one MediaStream, both stamped from SCK presentation timestamps on the host clock, so the browser lip-syncs from RTCP sender reports" — local timing code
- [Audio] Q: "What if the encoder or audio capture fails?" → Selected: "Audio failure becomes a state and the picture keeps streaming; no crash and no stream restart (Phase 19 crash containment)" — local error handling
- [Audio] Q: "Real output device check?" → Selected: "M-162 runs in this phase on a real Mac (the spike measured only the guest's virtual device)" — test row
- [Adaptive] Q: "Bitrate policy across viewers?" → Selected: "Target = minimum of per-viewer GCC estimates, clamped to a floor and ceiling; down quickly, up slowly with hysteresis; a viewer below the floor is left out of the minimum so it cannot drag the others down (SC2 'without stalling other viewers')" — tunable constants
- [Adaptive] Q: "How is 'sharp when still' made?" → Selected: "After about 300 ms with no new SCK frame, re-encode the cached last frame once at high quality (VideoToolbox Quality and bitrate properties, not a new session), then hold; plus an idle refresh about every 2 s so Safari does not mark the stream stalled (PITFALLS 21 and 5)" — encoder state machine
- [Adaptive] Q: "How does motion degrade under a poor network?" → Selected: "First lower bitrate, then frame rate (30 to 15), then resolution steps below the 2560 px cap via SCStreamConfiguration updates without restarting the stream; recover in reverse order" — policy ladder
- [Adaptive] Q: "How are bitrate changes applied?" → Selected: "Only through VideoToolbox session properties (AverageBitRate, DataRateLimits); never by recreating the session" — local code
- [Adaptive] Q: "How is it tested?" → Selected: "Unit tests of the policy with fake estimates and frame timings; loopback integration with the synthetic frame source and a shaped link; a real two-Mac M-NN row for text sharpness and smooth scrolling" — test layout
- [Unlock] Q: "How is 'wait for the password field' done?" → Selected: "Send one harmless key (or IOPMAssertionDeclareUserActivity if the display is dark), wait a fixed settle time (about 1 s, tuned in the VM), then type; R16-B shows the first key reveals the field" — a timing constant
- [Unlock] Q: "How are password characters typed?" → Selected: "CGEventPost at the HID tap with key codes mapped from characters through the current keyboard layout (R16 method, 60 ms per key); a character with no key on the layout refuses the attempt with a clear message before anything is typed" — local mapping code
- [Unlock] Q: "When does the retry run?" → Selected: "If CGSSessionScreenIsLocked is still set about 3 s after Return, retype once from the held buffer; then report the result and zero the buffer; never more than one retry per submit" — UNLK-01 fixes the count
- [Unlock] Q: "Is unlock audited?" → Selected: "An unlock event (who, when, success or failure) is added to Phase 22's local audit log; never the password or its length" — an additive event type
- [Unlock] Q: "Who enforces own-account-only?" → Selected: "The daemon checks the viewer's grant kind (own-account always-available grant from Phase 19, not a Phase 22 guest grant) before accepting an unlock message; the viewer UI hides the field as a convenience only" — server-side check
- [Unlock] Q: "How is it tested?" → Selected: "Unit tests that the buffer is zeroed and that no log line, audit line or error carries the password (a sentinel password grep over captured output); VM rows in portty-test with its public password for success, the hidden-field race and the retry; an office-Mac M-NN row" — test layout
- [Release] Q: "Build target?" → Selected: "wails build -platform darwin/arm64 in release.yml and build.yml; artifact names drop 'universal'; build.sh and tests/build-script.test.sh follow" — CI config
- [Release] Q: "Branch protection?" → Selected: "The build.yml matrix context changes to build (portty, darwin/arm64, macos-latest), so TESTING.md Section 3 is updated and the user re-applies branch protection (CLAUDE.md rule 4)" — CI config plus a user step
- [Release] Q: "Notarization time with two DMGs?" → Selected: "Raise the sign-macos timeout to cover three notarization waits (.app and two DMGs)" — a CI number
- [Release] Q: "How are the release checks pinned?" → Selected: "Extend release_workflow_test.go: both DMG names, identifier equals name for each, arm64-only build flag, no --deep, the verify loop present" — test only
- [Release] Q: "First real release check?" → Selected: "M-93 (notarized DMG verify, reveal and notification) is run on the v2.0 release, plus a v1.2.x copy updating to v2.0 through the universal name and a Phase 18 build updating through self-install" — manual rows
- [Hardening] Q: "SC5 upgrade check rig?" → Selected: "The Phase 18 two-build rig in portty-test with a live desktop viewer from portty-test-2 and ticking terminal sessions, then once on the office Mac as an M-NN row" — test rig
- [Hardening] Q: "SC5 soak measurement?" → Selected: "One hour of mixed motion and idle with a viewer connected, sampling RSS, goroutines, open fds, CPU and stream state every 60 s; pass = no crash, no restart, no stuck state, RSS and fd counts flat after warm-up" — thresholds tunable
- [Hardening] Q: "TESTING.md?" → Selected: "New test files go into Section 2 and Section 4 rows for DCTL-07, DESK-07, UNLK-01 and REL-01; real-Mac items go into a new Category in Section 5; bash tests/check-traceability-paths.sh before each commit (standing rule)" — documentation

### Claude's Discretion
User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Claude may adjust these during planning:
- Unmute control: a speaker button in the viewer toolbar plus a one-time "Tap to unmute" hint over the picture; the choice is not remembered between visits. Reason: DCTL-07 says audio starts muted, so remembering an unmute would break that.
- Click-to-share guests hear audio like the picture, starting muted; the host gets no separate Share Audio switch. Reason: radical simplicity; the sharer already chose to show the desktop to that person and can revoke them.
- After a remote unlock, the Mac locks itself again when the last own-account viewer leaves and does not come back within about 30 s, unless someone used the Mac's own keyboard or mouse since the unlock. Reason: an unlocked office Mac must not be left open after you disconnect from home; the research lists "lock on disconnect only if the session started locked" as a differentiator.
- Unlock copy: the field says "Mac password", the button says Unlock; after the retry also fails the viewer shows "Unlock didn't work. Check the password and try again." and does not retry by itself. Reason: plain wording, and each failed try counts toward macOS's failed-attempt limit.
- The Unlock field appears in the browser, iPhone and Portty app viewers alike (each gets it from the shared DesktopView). Reason: the user's main remote device may be any of them.
- Release notes for v2.0 state "Apple Silicon and macOS 14 or later only" at the top. Reason: v1.2.x updaters do not check architecture or OS and would offer the update anyway.
- The adaptive behaviour has no user-facing quality setting. Reason: the requirement asks for automatic adaptation, and radical simplicity.

### Deferred Ideas (OUT OF SCOPE)
- Persistent-content-capture provisioning profile wiring, if Apple has not granted it before Phase 23 planning closes (REQUIREMENTS Future).
- Dropping the universal-named DMG: a later release, once both Macs run a build from Phase 18 on.
- libopus: only if AudioConverter Opus fails on the office Mac's real output device (M-162); that would be a new dependency and a new question.
- A second, lower-bitrate encoder tier for slow viewers (research PITFALLS: a later optimisation, not v2.0).
- HEVC, H.264 High profile and a data-channel cursor (REQUIREMENTS Future).
- Unlock at the login window after reboot or with FileVault (REQUIREMENTS Out of Scope).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DCTL-07 | Viewer hears the shared Mac's audio; it starts muted on the viewer with tap to unmute | SCK audio on the existing SCStream (R21 GO), AudioConverter Opus recipe, pion Opus track, `stereo=1` receive-side requirement, A/V timestamp rules, viewer unmute pattern (Standard Stack, Patterns 1-2, Pitfalls 8-10) |
| DESK-07 | Picture is sharp when the screen is still and smooth when moving, adapting to the network | pion GCC/TWCC wiring, estimator behavior read from source (1.5x received-rate cap, unbounded pacer queue), still-refine state machine, ladder, verified SCK Idle-frame behavior (Patterns 3-5, Pitfalls 1-7) |
| UNLK-01 | Own-account viewer sees the lock screen and types the password to unlock; never stored or logged | R15/R16/R17 evidence, unlock sequence, wake-key and Caps Lock gaps, byte-buffer handling, throttle, lock gate conflict with Phase 20 (Pattern 6, Pitfalls 11-16, Security Domain) |
| REL-01 | Releases are Apple Silicon only with minimum macOS 14; every nested binary signed, notarized and verified in CI; v1.2.x copy still finds and installs v2.0 | Updater asset/identifier matching read in current code, v1.2.0 release inspected (assets, signature, designated requirement), Wails 10.13 default minos trap proven by build, release.yml and build.yml change list, two-DMG scheme, verify script (Patterns 7-9, Pitfalls 17-22) |
</phase_requirements>

## Summary

This phase is four mostly independent workstreams plus a hardening pass. The release workstream (REL-01) can be planned and built against the repo as it is today. It is the best-evidenced part of this research: the v1.2 updater's matching rules were read in code, the published v1.2.0 release was inspected (one DMG asset named `portty-v1.2.0-darwin-universal.dmg`, signed with identifier `portty-v1.2.0-darwin-universal`, app designated requirement captured verbatim), and the two-DMG scheme in CONTEXT is compatible with both the v1.2 updater and Phase 18's widened lookup. The one thing CONTEXT does not anticipate: Wails v2.10.2 injects `-mmacosx-version-min=10.13` into CGO_CFLAGS and CGO_LDFLAGS unless they are already set, and a scratch build here shows the resulting Mach-O reports `minos 11.0`; only setting both variables to 14.0 yields `minos 14.0`. CI must set both variables and assert the result, or the "minos below 14.0" gate will fail the first release.

Audio, adaptive quality and unlock all depend on code Phases 19-22 have designed but not built (`internal/screencap`, `internal/desktop`, `DesktopView.tsx`, the input data channel, grant kinds, the audit log). Plans for them must be written against those designs and re-validated when the earlier phases land. Within that limit, the pion bandwidth-estimation source (v0.1.51, read this session) shows three behaviors the CONTEXT policy has to account for: GCC cannot raise its estimate above 1.5x the receive rate, so a mostly still screen never "earns" a higher estimate; the default leaky-bucket pacer has an unbounded queue and carries audio and video in one FIFO; and the estimate only exists if the browser returns TWCC feedback, which has no evidence for Safari, iPhone or WKWebView in this repo.

Unlock is feasible per R15-R17 and ships. The remaining risk is in details CONTEXT leaves open: which non-printing wake key reveals the password field without inserting a character, Caps Lock state at the lock screen, how a password reaches the daemon without becoming an immutable Go string, an attempt throttle for a stolen own-account device, and a conflict between this phase (own-account pointer passes while locked) and Phase 20 (all pointer and key input dropped while locked).

**Primary recommendation:** Plan the release pipeline first and independently (Wave 1), fix the build so minos 14.0 is real and asserted, ship two DMGs signed per name, and treat audio/quality/unlock as extensions of Phase 19/20/22 interfaces with an explicit upstream-contract checklist (see Architecture Patterns) that the planner re-checks against the built code before executing.

## Decision reference index (used by the findings in this document)

CONTEXT decisions have no numeric IDs. This index names them so the findings below and the hand-off message can point at them.

| ID | CONTEXT decision |
|----|------------------|
| R-1 | Release packaging: two DMGs from one arm64 build |
| R-2 | Release signing: inside-out signing, CI enumeration and gates, DR pin |
| R-3 | Release: entitlement not awaited |
| R-build | [Release] Build target (`wails build -platform darwin/arm64`) |
| R-bp | [Release] Branch protection context rename |
| R-time | [Release] Notarization timeout for three waits |
| R-pin | [Release] Release checks pinned in release_workflow_test.go |
| U-1 | Unlock field, password on the control data channel, byte buffer, zeroing |
| U-2 | Lock-time key drop; own-account pointer passes while locked |
| U-wake | [Unlock] wake key then settle |
| U-type | [Unlock] key-code typing from the keyboard layout |
| U-retry | [Unlock] retry once |
| U-audit | [Unlock] audit event |
| U-grant | [Unlock] daemon checks grant kind |
| D-relock | Discretion: re-lock after the last own-account viewer leaves |
| A-audio | [Audio] decisions (capture policy, Opus constants, sync, failure, M-162) |
| A-policy | [Adaptive] min of per-viewer GCC estimates, floor and ceiling, hysteresis |
| A-still | [Adaptive] sharp when still (300 ms rule, Quality property, 2 s refresh) |
| A-ladder | [Adaptive] bitrate, then fps, then resolution via SCStreamConfiguration |
| A-apply | [Adaptive] bitrate changes only via VT properties, never recreate the session |
| H-soak | [Hardening] soak and upgrade rig |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| System-audio capture, Opus encode | Daemon (cgo ObjC in `internal/screencap`) | — | Capture runs in the daemon under the app bundle ID (FEAS-02 in-daemon); audio rides the same SCStream and the existing Screen Recording grant |
| Audio and video RTP, TWCC feedback, sender reports | Daemon (`internal/desktop`, pion) | — | One PeerConnection per viewer in the daemon; pion interceptors live there |
| Mute / unmute and playback | Browser / Client (DesktopView) | — | Mute is viewer-only by decision; needs a user gesture on the viewer (Chrome, iOS) |
| Per-viewer bandwidth estimate | Daemon (per-PC GCC estimator) | — | Estimator needs the RTP send path and TWCC feedback |
| Target bitrate, fps and resolution policy | Daemon (one policy for the shared encoder) | — | One encoder serves all viewers; policy takes the min across viewers |
| Still-screen refinement, idle refresh | Daemon (encoder state machine + cached frame) | — | Only the daemon holds the last captured frame and the VT session |
| Resolution step | Daemon (SCStreamConfiguration update + encoder reconfigure) | Browser (video size change) | Same path Phase 20 builds for display switch |
| Unlock field and result UI | Browser / Client (shared DesktopView) | — | Convenience only; hidden for guests |
| Unlock authorization (own account only) | Daemon (grant kind) | Share server (WhoIs same-user gate) | Server-side check is the boundary; UI hiding is cosmetic |
| Password typing, lock-state check, retry, zeroing | Daemon (`internal/screencap` input bridge) | — | CGEventPost at the HID tap from the Accessibility-holding process (R16) |
| Lock-time input drop | Daemon (Phase 20 input entry point) | — | Phase 20 owns the filter; Phase 23 adds the one explicit exception path |
| Re-lock after leaving | Daemon | Phase 22 physical-input signal | Needs "someone used the Mac" from Phase 22 |
| Signing, notarization, bundle verification | CI (`release.yml`, `build.yml`, verify script) | — | Pipeline-only; no runtime component |
| Update lookup and identifier check | Installed app (`internal/updater`) | CI (publishes both names) | The client contract is fixed by v1.2 builds in the field |
| Upgrade and soak hardening | Test rig (Tart VMs) + office Mac (M-NN rows) | Daemon stats endpoint | Measurement needs real silicon for the final row |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pion/webrtc/v4` | v4.2.29 | PeerConnection, Opus and H.264 tracks, interceptor registry | Pinned by Phase 19 (19-CONTEXT). `go list -m -versions` this session lists v4.2.29 as the newest v4 release [VERIFIED: go proxy] |
| `github.com/pion/interceptor` | v0.1.51 | `pkg/cc` + `pkg/gcc` send-side bandwidth estimation, TWCC | Pinned by Phase 19. v0.1.51 is the newest listed [VERIFIED: go proxy]; source read in the module cache this session |
| ScreenCaptureKit (system framework) | macOS 14+ | `capturesAudio`, `excludesCurrentProcessAudio`, `updateConfiguration` | Same stream as video; R21 GO in the guest [VERIFIED: 17-SPIKE-MATRIX R21] |
| AudioToolbox `AudioConverter` | system | Opus encode, 960-frame packets | Decided (research C7); R21 GO, framesPerPacket 960 [VERIFIED: 17-SPIKE-MATRIX R21] |
| VideoToolbox | system | H.264 session properties for bitrate, ForceKeyFrame | In use since Phase 19 |
| ApplicationServices (CGEvent), IOKit (`IOPMAssertionDeclareUserActivity`), CoreGraphics (`CGSessionCopyCurrentDictionary`) | system | Unlock typing, wake, lock state | R16/R17 recipe [VERIFIED: 17-SPIKE-MATRIX R16-B, R17-B] |
| Carbon/HIToolbox `TISCopyCurrentKeyboardLayoutInputSource` + `UCKeyTranslate` | system | Reverse-map characters to key codes through the current layout | Needed for U-type; no third-party layout table [ASSUMED] |
| Xcode tools: `codesign`, `notarytool`, `stapler`, `spctl`, `vtool`, `lipo`, `file` | Xcode on macos-latest | Sign, notarize, verify | Already used by release.yml; `vtool`, `lipo`, `codesign` run on this host this session |
| GitHub Actions `macos-latest` | macOS 26 arm64 | CI runner | `macos-latest` points at the macOS 26 arm64 image per GitHub changelog [CITED: github.blog/changelog/2026-02-26-macos-26-is-now-generally-available-for-github-hosted-runners/]; exact current label state not confirmed today |

No new Go module, npm package or system library is added by this phase. libopus stays deferred.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gcc.NewNoOpPacer()` / custom `gcc.Pacer` | in interceptor v0.1.51 | Replace the unbounded leaky-bucket pacer | Pitfall 3; use `gcc.SendSideBWEPacer(p)` |
| `pion/transport/v5/vnet` (in webrtc module) | via webrtc v4.2.29 | In-process shaped network for tests | Only if it composes with Phase 19's UDP mux seam; otherwise a userspace UDP shaper in the test (Pattern 10) [ASSUMED] |
| `footprint`, `vmmap`, `lsof`, `ps` (macOS) | system | Soak sampling that sees IOSurface memory | `/usr/bin/footprint`, `/usr/bin/vmmap` exist on this host [VERIFIED: which] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| AudioConverter Opus | libopus via cgo | New dependency, signing weight; deferred unless M-162 fails |
| pion GCC min-of-viewers | One encoder tier per viewer class | Deferred in CONTEXT (second tier is later optimisation) |
| `TrackLocalStaticSample` | `TrackLocalStaticRTP` + `rtp.Packetizer` with PTS-exact timestamps | More code, exact timestamps after idle gaps (Pitfall 9). Depends on what Phase 19 built |
| `codesign --deep` | explicit inside-out per Mach-O | Decided; one Mach-O today so the loop is trivial but must enumerate |

**Installation:** no install step for this phase.

**Version verification:**
```bash
go list -m -versions github.com/pion/interceptor   # newest v0.1.51
go list -m -versions github.com/pion/webrtc/v4     # newest v4.2.29
```
Root `go.mod` has no pion lines yet (Phase 19 adds them) [VERIFIED: grep -c pion go.mod returned 0].

## Package Legitimacy Audit

No new external package is installed by this phase, so there is nothing to submit to the legitimacy seam (it covers npm, PyPI and crates, not Go modules). The only third-party code touched is pion, already pinned by Phase 19 and present in the local module cache at the versions above.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/pion/webrtc/v4 v4.2.29 | Go proxy | multi-year | n/a | github.com/pion/webrtc | not run (Go) | Approved by Phase 19 pin; version confirmed newest |
| github.com/pion/interceptor v0.1.51 | Go proxy | multi-year | n/a | github.com/pion/interceptor | not run (Go) | Approved by Phase 19 pin; version confirmed newest |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 macOS host (office Mac, daemon under launchd, app bundle ID app.portty.Portty)

  SCStream (one) --- video CMSampleBuffers (Complete / Idle) ---+--> frame cache (last Complete frame, IOSurface)
     |  capturesAudio, excludesCurrentProcessAudio             |        |
     |                                                         v        v
     +-- audio CMSampleBuffers (48k f32 stereo) --> interleave --> AudioConverter(Opus, 960) --> audio RTP (PTS-stamped)
                                                               encoder state machine (VideoToolbox, one session)
   viewers' target bitrate  ---> policy: min(estimates, floor..ceiling) ---> VT AverageBitRate / DataRateLimits
   viewers' PLI/FIR (coalesced) ---> ForceKeyFrame                        fps / resolution ladder ---> SCStream.updateConfiguration (+ Phase 20 encoder reconfigure)
          |                                                               stillness timer (no Complete frame ~300 ms) ---> refine cached frame once, then 2 s idle refresh
          v
   per-viewer PeerConnection (pion): video track + audio track, TWCC ext, GCC estimator, bounded pacer
          |  RTP/SRTP over UDP (tailnet IP only)         ^ RTCP: TWCC feedback, PLI, receiver reports
          v                                              |
   ======================== tailnet ========================
          v
   Viewer (browser / iPhone / Portty app tab, DesktopView)
     <video muted playsinline srcObject=MediaStream(video+audio)>  --tap--> video.muted=false
     Unlock field (own account only) --password bytes--> control data channel (DTLS) --> daemon unlock typer
                                                                                   |
   daemon unlock typer:  grant check -> declare user activity + non-printing wake key -> settle ~1 s -> type via CGEventPost(HID tap)
        -> poll CGSSessionScreenIsLocked ~3 s -> retype once -> zero buffer -> result message + audit event (no password, no length)
   Phase 20 input gate: while CGSSessionScreenIsLocked, drop viewer key (and, per Open Question 1, pointer) input; unlock typer bypasses it internally
```

### Upstream contract checklist (Phases 19-22 are designs, not code)

Plans for audio, quality and unlock extend interfaces that do not exist yet. The planner must re-check each row against the built code before executing and keep the plan text tolerant ("extend X as built by Phase N").

| Needed from | Interface | Why Phase 23 needs it |
|-------------|-----------|------------------------|
| Phase 19 | The `webrtc.API` / interceptor registry construction in `internal/desktop` is where GCC, TWCC and the pacer are registered. 19-CONTEXT says only "fixed target bitrate"; it does not say GCC is registered | DESK-07 changes how every viewer's PC is built, so this is an edit to Phase 19 code, not an addition |
| Phase 19 | Track type (`TrackLocalStaticSample` vs `TrackLocalStaticRTP`) and how video timestamps are produced | A/V sync and post-idle latency (Pitfall 9) |
| Phase 19 | Encoder seams: `SetBitrate`, `RequestKeyframe` (coalesced), cached last frame, forced refresh | A-policy, A-still |
| Phase 19 | Signalling version field and state enum including `locked` | Audio m-line, unlock result, version bump decision (additive) |
| Phase 19 | Viewer offer construction in `desktopClient.ts` | Must add an audio recvonly transceiver and the `stereo=1` offer munge |
| Phase 20 | The single daemon-side input entry point per viewer, the lock gate, `release-all`, and the encoder reconfigure used by display switching | U-2 exception path; resolution ladder reuses the reconfigure path; ladder must not bump the input epoch |
| Phase 20 | The `input` data channel reader: text JSON vs binary messages | U-1 wants a binary unlock message (Pattern 6) |
| Phase 22 | Grant kind (own account vs guest) lookup per viewer | U-grant |
| Phase 22 | Audit log appender and its event list (a closed list in 22-CONTEXT) | U-audit adds an event type |
| Phase 22 | A "physical input since T" signal | D-relock |

### Recommended Project Structure (names are suggestions; follow what Phase 19 created)

```
internal/screencap/
  audio_darwin.m        # SCK audio output handler, interleave, AudioConverter Opus
  unlock_darwin.m       # layout map, wake key, typing, lock-state read (no logging)
  encoder_quality_darwin.m  # refine/idle-refresh entry points, property setters
internal/desktop/
  audio.go              # audio track, RTP timestamps, failure-as-state
  quality.go            # policy: floor/ceiling/hysteresis, ladder, stillness timer
  pacer.go              # bounded, audio-priority pacer (or NoOp) for gcc.SendSideBWEPacer
  unlock.go             # grant check, throttle, sequencing, zeroing, result, relock
  stats.go              # daemon-socket stats for the soak (goroutines, fds, estimates, ladder step)
frontend/src/
  components/DesktopView.tsx      # unmute button + hint, Unlock field
  lib/desktopClient.ts            # audio transceiver, stereo munge, unlock send
scripts/
  verify-bundle.sh      # Mach-O enumeration, arch, minos, signature, DR (used by build.yml and release.yml)
  desktop-soak.sh       # 60 s sampler (RSS, footprint, fds, goroutines)
```

### Pattern 1: Audio on the same SCStream

**What:** Set `capturesAudio = YES`, `sampleRate = 48000`, `channelCount = 2`, `excludesCurrentProcessAudio = YES` on the one `SCStreamConfiguration`, add an `SCStreamOutputTypeAudio` output on its own queue, interleave the non-interleaved float32 buffers (R21 saw "float32 stereo, non-interleaved"), accumulate to 960-frame blocks and feed AudioConverter.
**When to use:** Always while the desktop stream runs (A-audio).
**Example (from the spike, reference only, never imported):**
```objc
// Source: spike/desktop/media_darwin.m:923-925 and :474-475 (read this session)
cfg.capturesAudio = YES;
cfg.sampleRate = kOpusRate;      // 48000
cfg.channelCount = 2;
enum { kOpusFrames = 960, kOpusRate = 48000, kOpusBitrate = 96000, kOpusMaxPacket = 4096 };
```
Set the bitrate constant to 64000 per A-audio and read back the avg kbps; the spike measured 55 kbps average for a sine at a 96 kbps target, and no run used 64 kbps [ASSUMED that 64000 is accepted by `kAudioConverterEncodeBitRate`; verify in a VM probe].

### Pattern 2: Viewer unmute (user gesture)

**What:** Keep one `<video playsInline muted autoPlay>` fed by a MediaStream with both tracks. Unmute only inside a click/tap handler. If `play()` rejects after unmuting, mute again and show the hint. Never persist the choice.
**Example:**
```tsx
// Source: pattern from Chrome autoplay docs (developer.chrome.com/blog/autoplay): muted media may
// autoplay; unmute from a user gesture. Not yet run in this repo.
function onToggleMute(video: HTMLVideoElement, setMuted: (m: boolean) => void) {
  const next = !video.muted
  video.muted = next
  if (!next) {
    video.play().catch(() => { video.muted = true; setMuted(true) })
  }
  setMuted(next)
}
```
Receive-side stereo: the viewer must put `stereo=1` into its offer's Opus fmtp (munge `useinbandfec=1` to `useinbandfec=1;stereo=1`) and the answer should also carry `stereo=1;sprop-stereo=1`. In libwebrtc the Opus decoder defaults to mono when `stereo` is absent (older source; a field trial can flip it) [CITED: webrtc.googlesource.com audio_decoder_opus.cc, mediamtx issue 2043]. Verify with a left-only test tone in a VM viewer (Pitfall 8).

### Pattern 3: pion registry with TWCC, GCC and a controlled pacer

**What:** One `cc.NewInterceptor` per API (not per PC); `OnNewPeerConnection` hands each PC's estimator to the policy. Replace the default pacer.
**Example:**
```go
// Source: pion/webrtc v4.2.29 examples/bandwidth-estimation-from-disk/main.go (read this session),
// pacer option from pion/interceptor v0.1.51 pkg/gcc/send_side_bwe.go:96 (SendSideBWEPacer)
cc, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
    return gcc.NewSendSideBWE(
        gcc.SendSideBWEInitialBitrate(ceilingBps), // start at the ceiling, see Pitfall 1
        gcc.SendSideBWEMinBitrate(floorBps),
        gcc.SendSideBWEMaxBitrate(ceilingBps),
        gcc.SendSideBWEPacer(newBoundedPacer()), // not the default unbounded leaky bucket
    )
})
cc.OnNewPeerConnection(func(id string, est cc.BandwidthEstimator) { policy.Register(id, est) })
reg.Add(cc)
_ = webrtc.ConfigureTWCCHeaderExtensionSender(me, reg)   // registers transport-cc for audio and video
_ = webrtc.RegisterDefaultInterceptors(me, reg)          // NACK, RTCP reports (incl. sender reports), stats, TWCC sender
api := webrtc.NewAPI(webrtc.WithInterceptorRegistry(reg), webrtc.WithMediaEngine(me))
```
Read each viewer's `est.GetTargetBitrate()` on a ticker or via `est.OnTargetBitrateChange`. The `BandwidthEstimator` interface has `AddStream`, `WriteRTCP`, `GetTargetBitrate`, `OnTargetBitrateChange`, `GetStats`, `Close` [VERIFIED: pkg/cc/interceptor.go:24-33]. The `Pacer` interface is `interceptor.RTPWriter` + `AddStream(ssrc, writer)` + `SetTargetBitrate(int)` + `Close() error` [VERIFIED: pkg/gcc/send_side_bwe.go:31-36].

### Pattern 4: Policy (min of estimates, hysteresis, ladder)

**What:** A pure function over `(per-viewer estimates, current step, now)` returning `(bitrate, fps, scale step)`, unit-tested with fake estimates, no pion or VT. Starting constants below are [ASSUMED] tuning points, to be set by the shaped-link test and M-161:
- ceiling 8 Mbps (the spike's tested bitrate), floor about 500 kbps, initial estimate = ceiling.
- Down: apply within about 1 s when `min < current * 0.85`. Up: raise about 10 percent per 2 s only after `min > current * 1.2` held 5 s.
- Ladder order: bitrate down to about 2 Mbps, then fps 30 to 15, then 2560 to 1920 to 1280 long edge; recover in reverse with a 10 s hold per step.
- A viewer whose estimate stays below the floor for N seconds is excluded from the min, shows "Slow connection" (viewer-side) and its PLIs are ignored (Pitfall 5).

### Pattern 5: Still-screen refinement state machine

**What:** States `moving` and `still`. Enter `still` when no `SCFrameStatusComplete` frame has arrived for about 300 ms (Idle frames do not count). On entering: encode the cached last Complete frame once as a refinement; then every about 2 s encode the cached frame again as a tiny P-frame. Leave `still` on the next Complete frame. Per R15-A, SCK sends Idle frames at about 27 per second for roughly 14 s after the last change and then nothing, so the trigger must key on Complete frames, not on "no frame" [VERIFIED: 17-SPIKE-MATRIX R15-A: "Idle frames at about 27 per second for roughly 14 s after the last change, then 0 frames of any status"].
How the refinement is made sharp is not settled (Pitfall 4); measure SSIM in a VM with the spike's ffmpeg method before choosing among: (a) raise AverageBitRate plus ForceKeyFrame for one frame then restore, (b) set `kVTCompressionPropertyKey_Quality`, (c) re-encode the same pixel buffer 2-3 times so each P-frame refines the last [ASSUMED].

### Pattern 6: Unlock sequence and password handling

**What:** One goroutine per Mac (a mutex), so two submits never interleave.
1. Check grant kind is own-account always-available (not a Phase 22 guest grant) and state is Locked, else refuse.
2. Check throttle (Pitfall 15). Check Caps Lock state is off; if on, refuse with a clear message (Pitfall 13).
3. Map every rune through the current keyboard layout before typing anything; refuse the whole attempt if any rune has no key (U-type).
4. `IOPMAssertionDeclareUserActivity`, then one non-printing wake key (Pitfall 12), settle about 1 s.
5. Post key down, wait 60 ms, key up per character with `CGEventPost` at `kCGHIDEventTap`, then Return [VERIFIED: 17-SPIKE-MATRIX R16-B "key down and up with CGEventPost at the HID tap, 60 ms apart"].
6. Poll the CGSession dictionary for `CGSSessionScreenIsLocked` for about 3 s. If still set, retype once from the held buffer and poll again.
7. Zero the buffer in a `defer` that also runs on panic; report ok/fail; write the audit event.
Carry the password as `[]byte` end to end. Send it from the viewer as a binary data-channel message (type byte plus UTF-8 bytes) instead of inside a JSON string, so the daemon never builds an immutable Go string that cannot be zeroed. Zero the inbound message slice too. Go's `runtime/secret` is linux-only in Go 1.26 so it is not an option here [CITED: pkg.go.dev/runtime/secret; go1.26 release notes]. Best-effort zeroing is the honest limit: pion's SCTP reassembly buffers are not under the daemon's control.

### Pattern 7: Two DMGs from one build

**What:** Create the DMG once, copy it to the second name, sign each with its own identifier, then notarize and staple each, then verify each exactly as the updater does.
**Example:**
```bash
# Source: adapted from .github/workflows/release.yml:225-260 (read this session)
VERSION="${GITHUB_REF_NAME}"
for NAME in "portty-${VERSION}-darwin-universal" "portty-${VERSION}-darwin-arm64"; do
  DMG="build/bin/${NAME}.dmg"
  [ -f "$DMG" ] || cp "build/bin/portty-${VERSION}.base.dmg" "$DMG"
  codesign --force --timestamp --identifier "${NAME}" --sign "$MACOS_CERTIFICATE_NAME" "$DMG"
  xcrun notarytool submit "$DMG" --keychain-profile "portty-notarize" --wait --timeout 60m
  xcrun stapler staple "$DMG" && xcrun stapler validate "$DMG"
  DVV=$(codesign -dvv "$DMG" 2>&1)
  grep -qx 'TeamIdentifier=S2K7P43927' <<< "$DVV"
  grep -qx "Identifier=${NAME}" <<< "$DVV"
  spctl -a -t open --context context:primary-signature -vv "$DMG" 2>&1 | grep -qx 'source=Notarized Developer ID'
done
```
Each DMG needs its own notarization: the identifier is part of the signature, so each file's code directory hash differs [ASSUMED from how notarization keys on cdhash]. Delete `build/bin/portty-${VERSION}.base.dmg` before upload (the upload glob `portty-*.dmg` would otherwise publish it, and `sha256sum *.dmg` would list it).

### Pattern 8: Bundle verification script

**What:** One script, `scripts/verify-bundle.sh <app> [--signed]`, used by build.yml (arch and minos on the unsigned bundle) and by release.yml (everything). Commands below were run against the real v1.2.0 app this session.
```bash
APP="$1"
# 1. Enumerate every Mach-O; fail on none or unexpected extra
while IFS= read -r f; do
  [ "$(lipo -archs "$f")" = "arm64" ] || { echo "FAIL arch $f"; exit 1; }
  MINOS=$(vtool -show-build "$f" | awk '/minos/{print $2; exit}')
  [ "$(printf '%s\n14.0\n' "$MINOS" | sort -V | head -1)" = "14.0" ] || { echo "FAIL minos $MINOS $f"; exit 1; }
done < <(find "$APP" -type f -exec sh -c 'file -b "$1" | grep -q "^Mach-O"' _ {} \; -print)
# 2. Signed checks (release only)
codesign --verify --deep --strict --verbose=2 "$APP"
codesign -d -r- "$APP" 2>&1 | grep -qxF 'designated => identifier "app.portty.Portty" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] /* exists */ and certificate leaf[field.1.2.840.113635.100.6.1.13] /* exists */ and certificate leaf[subject.OU] = S2K7P43927'
# per Mach-O: codesign -dvv shows TeamIdentifier=S2K7P43927, a runtime flag (flags=0x10000(runtime)), a Timestamp= line
xcrun stapler validate "$APP"; spctl -a -t exec -vv "$APP" 2>&1 | grep -q 'source=Notarized Developer ID'
```
Observed on v1.2.0: `Identifier=app.portty.Portty`, `CodeDirectory v=20500 ... flags=0x10000(runtime)`, `Authority=Developer ID Application: Ken Scott (S2K7P43927)`, `Timestamp=Nov 23, 2026 ...`, `TeamIdentifier=S2K7P43927`, entitlements exactly `com.apple.security.network.client` and `com.apple.security.network.server`, and a stapled ticket at `Contents/CodeResources`. The designated requirement printed by `codesign -d -r-` is the string quoted above, which is the one to pin [VERIFIED: ran against portty-v1.2.0-darwin-universal.dmg mounted read-only in scratch, detached after].

### Pattern 9: Build for minos 14.0

**What:** Export both variables before `wails build`, in release.yml, build.yml and build.sh, and assert the result.
```bash
# Source: wails v2.10.2 pkg/commands/build/base.go:312-313 and :351-352 only add 10.13 when the
# variable lacks "-mmacosx-version-min" (read this session); proven with a scratch cgo build.
export CGO_CFLAGS="-mmacosx-version-min=14.0"
export CGO_LDFLAGS="-mmacosx-version-min=14.0"
wails build -platform darwin/arm64 -tags wailsassets -ldflags "-X main.Version=${GITHUB_REF_NAME}"
```
Scratch result (go1.27.4 local, trivial cgo program, never run): env 10.13 gives `minos 11.0`; both at 14.0 gives `minos 14.0`; CGO_CFLAGS 14.0 with CGO_LDFLAGS 10.13 stays `minos 11.0` and prints linker warnings. The local toolchain differs from the pinned go1.26.x, so the CI assert is what counts. Phase 19 also claims "cgo built with -mmacosx-version-min=14.0"; if it did that with `#cgo CFLAGS` only, the link-time minos is still 11.0.

### Pattern 10: Shaped-link test without root

**What:** Loopback integration test with the synthetic frame source (Phase 19) and a pion client, with a small userspace UDP forwarder between them applying a token bucket, fixed delay and loss. Deterministic, runs in `go test -race -short` on macos-latest without pfctl/dnctl. `vnet` is an alternative only if it composes with the UDP mux seam [ASSUMED]. Real-network behavior (Safari/iPhone TWCC, DERP) goes to VM and M-NN rows.

### Anti-Patterns to Avoid

- **Recreating the VT session for bitrate** (A-apply is right). But the resolution step does need the encoder reconfigure Phase 20 builds; do not claim "never recreate" for resolution (Pitfall 6).
- **Trusting `GetTargetBitrate()` as an available-bandwidth reading on a still screen.** It is an estimate bounded by what was sent (Pitfall 1).
- **Letting a resolution step bump Phase 20's surface epoch.** Input is normalized 0..1 of the same surface; an epoch bump would drop input during a quality step.
- **Putting the password in JSON, a Go `string`, a log field, an error, or a metrics label.**
- **Using the default leaky-bucket pacer unchanged.**
- **Signing `Contents/MacOS/Portty` standalone first with a different identifier.** The bundle signature re-signs the main executable with the bundle identifier; keep `app.portty.Portty` so the DR does not drift.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Opus encoding | A Go or C Opus encoder | AudioConverter `kAudioFormatOpus` (R21) | Already proven; no dependency or signing weight |
| Congestion control | A custom bitrate controller | pion `pkg/cc` + `pkg/gcc` | Delay and loss estimators with TWCC; only the policy on top is ours |
| RTP packetization, NACK, sender reports | Custom RTCP | pion defaults (`RegisterDefaultInterceptors`) | Lip-sync depends on correct sender reports |
| Key-code mapping for the password | A hard-coded US table | `UCKeyTranslate` over the current layout, with refusal for unmapped runes | Layouts differ; a wrong map types a wrong password and burns a failed attempt |
| Mach-O discovery in the bundle | A path list | `find` + `file`/`lipo` over the whole bundle | A path list silently misses a future helper |
| Notarization and stapling | Anything custom | `notarytool` and `stapler` (already used) | Existing, working |
| Process memory measurement | RSS only | `footprint` and `vmmap -summary` plus `lsof` | RSS undercounts IOSurface memory (PITFALLS 20) |
| TLS or data-channel crypto for the password | Any wrapper encryption | The existing DTLS data channel | Decided (U-1); do not add a second layer |

**Key insight:** every part of this phase except the policy constants, the unlock sequence and the release scripts already has a system or pion implementation. The risk is in wiring and measurement, not in algorithms.

## Runtime State Inventory

This is not a rename phase, but the DMG asset name and CI artifact names are a published contract, so the five categories are answered.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Installed copies cache update DMGs under `~/Library/Caches/app.portty.Portty/updates`; `Clean` globs only `portty-v*-darwin-universal.dmg` and `.part` [VERIFIED: internal/updater/install.go:174-175] | Phase 18 widens `Clean`; nothing new here beyond confirming a leftover arm64-named DMG is cleaned |
| Live service config | GitHub release v1.2.0 has two assets: `checksums.txt` and `portty-v1.2.0-darwin-universal.dmg` [VERIFIED: gh release view]. Branch protection on `main` does not exist (API returned 404) [VERIFIED: gh api]. Repo Actions are enabled; `e2e` is disabled [VERIFIED: gh workflow list] | v2.0 must publish the universal name or v1.2 copies find nothing. No protection to silently drop on the context rename |
| OS-registered state | TCC Screen Recording and Accessibility rows on the office Mac are keyed to bundle ID and designated requirement (R04: "both TCC rows are keyed to Bundle ID app.portty.spike"). The v1.2.0 DR is quoted in Pattern 8 | None if the DR is pinned. A changed DR silently drops grants: code gate in CI |
| Secrets and env vars | `MACOS_CERTIFICATE`, `MACOS_CERTIFICATE_NAME`, `MACOS_CERTIFICATE_PWD`, `MACOS_CI_KEYCHAIN_PWD`, `MACOS_NOTARIZATION_APPLE_ID`, `MACOS_NOTARIZATION_PWD`, `MACOS_NOTARIZATION_TEAM_ID` in the `release` environment [VERIFIED: release.yml:147-154]; the v1.2.0 Release run succeeded [VERIFIED: gh run list, 9m39s] | None: names unchanged |
| Build artifacts | Artifact names `portty-darwin-universal-unsigned` (release.yml:134,164) and `portty-darwin-universal` (build.yml upload); `build.sh:85-86` and `tests/build-script.test.sh:236-241` assert `darwin/universal`; `cmd/fakegh/main.go:50` serves the universal name (VM-only feed; stays correct because the updater prefers the universal name); TESTING.md lines 18 and 340 name the old check context | Rename artifacts and update the script, its test and TESTING.md; leave fakegh unless Phase 18 changes it |

## Common Pitfalls

### Pitfall 1: GCC cannot raise its estimate above 1.5x what was sent
**What goes wrong:** On a still or low-motion screen the sender transmits little, the estimate stays low, and when motion starts the encoder is held at a stale low target (blurry start of every scroll).
**Why it happens:** In pion's `rateController.increase`, growth is capped at `1.5 * latestReceivedRate` (`min(c.target+increase, 1.5*latestReceivedRate)`), and in the multiplicative branch `if rate > received && received > c.target { return received }` [VERIFIED: pkg/gcc/rate_controller.go:152 and :159-160]. pion has no probe clusters; the estimate follows the sender.
**How to avoid:** Initialize the estimator at the ceiling (`SendSideBWEInitialBitrate(ceiling)`), so the normal state is "high until congestion says otherwise", and rely on decreases. Treat an estimate as a constraint, not as a measurement of free capacity. Test in the shaped-link test: still 30 s, then motion; assert the target did not collapse and recovers within the hold time. `TrackLocalStaticSample.GeneratePadding` exists if probing is ever needed.
**Warning signs:** `GetTargetBitrate()` near the initial value forever; target tracking the encoder's own output.

### Pitfall 2: No TWCC feedback means no estimates
**What goes wrong:** If a viewer does not return transport-cc feedback, `WriteRTCP` never gets TWCC reports and the estimate stays at its initial value; adaptation silently does nothing for that viewer.
**Why it happens:** TWCC works only when both sides negotiate the extension. There is no evidence in this repo for Safari, iPhone or the Wails WKWebView: a grep of `evidence/` and the matrix for transport-cc/twcc found nothing. Old third-party SDP dumps show Safari 13.1 offering transport-cc, which is not current evidence [CITED: gist SDP offer (Safari 13.1), via web search; low weight].
**How to avoid:** First VM task: log each viewer's offer SDP for the transport-cc extmap and rtcp-fb in Safari (VM home), Chrome if present, and the Wails shell; an iPhone M-NN row. Define the fallback for a viewer without feedback: pin that viewer's estimate at the ceiling but cap by RTCP receiver-report loss and fraction lost, and mark it in stats.
**Warning signs:** `GetStats()` shows zero feedback; estimate never moves in the shaped-link test for one browser.

### Pitfall 3: pion's default pacer queue is unbounded and shared by audio and video
**What goes wrong:** One viewer on a link slower than the floor accumulates an ever-growing queue (memory and latency, a soak-test leak), and Opus packets wait behind a large IDR or sharp-refresh burst, so audio glitches exactly when the picture refreshes.
**Why it happens:** `LeakyBucketPacer` holds one `list.List` queue, `Write` pushes every packet with no cap, and the interceptor's `BindLocalStream` calls `estimator.AddStream` for every local stream, audio included. The pacer budget is `target * 1.5` per 5 ms tick [VERIFIED: pkg/gcc/leaky_bucket_pacer.go (queue, Write, Run); pkg/cc/interceptor.go BindLocalStream; pkg/gcc/send_side_bwe.go:153-183].
**How to avoid:** Supply a pacer through `gcc.SendSideBWEPacer`: either `gcc.NewNoOpPacer()` (the encoder's `DataRateLimits` and the floor then bound bursts) or a small custom pacer with a hard queue cap that drops video packets (and requests a keyframe later) and sends audio first. Add a test that a stalled viewer's memory stays flat.
**Warning signs:** RSS or goroutine growth with a throttled viewer; audio dropouts at refresh instants.

### Pitfall 4: `Quality` and runtime rate properties on the hardware H.264 encoder are unproven
**What goes wrong:** The A-still plan assumes `kVTCompressionPropertyKey_Quality` makes the refinement frame sharp, and A-apply assumes `AverageBitRate`/`DataRateLimits` changes take effect live. Neither is established for the Apple Silicon hardware AVC encoder.
**Why it happens:** Apple's `Quality` page says only that some encoders such as JPEG use it and does not say H.264 hardware honors it [CITED: developer.apple.com VideoToolbox kVTCompressionPropertyKey_Quality]. Developer-forum reports on `AverageBitRate`/`DataRateLimits` are mixed and anecdotal, with overshoot and "no effect" reports, and low-latency rate control as the mode most likely to honor them, unmeasured on Apple Silicon [CITED: developer.apple.com/forums/thread/118021, /111647, /799459; web search].
**How to avoid:** Make a VM probe the first task: session as Phase 19 creates it, change bitrate live, measure produced bytes per second; set `Quality` and compare an IDR size and SSIM against the same frame at 8 Mbps. R20 already holds an SSIM method (ffmpeg ssim on a decoded IDR) and its numbers: 0.999327 on the real MacBook Air at 8 Mbps, so an IDR is already sharp [VERIFIED: 17-SPIKE-MATRIX R20-host]. The open problem is the post-motion P-frame, so measure that case. Fallbacks in Pattern 5. `kVTCompressionPropertyKey_ConstantBitRate` and `EnableLowLatencyRateControl` are candidate session options if live changes do not take effect, but changing session creation options belongs in Phase 19's encoder, so flag it as a Phase 19 follow-up rather than editing it silently.
**Warning signs:** Measured output rate not following the set value; refinement frame the same size as a normal P-frame.

### Pitfall 5: One bad viewer can still hurt the others
**What goes wrong:** Loss-driven PLIs from a viewer below the floor force a shared IDR (large) to everyone; the min policy hides this because the viewer was excluded from the minimum.
**How to avoid:** A viewer excluded from the min also has its PLI/FIR ignored (except on join), and is told "Slow connection". Keep the coalescing floor (research says at most one IDR per 500-1000 ms) and raise it for loss-driven requests to about 2 s. This is the practical meaning of SC2 "without stalling other viewers".

### Pitfall 6: A resolution step is not a bitrate change
**What goes wrong:** CONTEXT says resolution steps go "via SCStreamConfiguration updates without restarting the stream" and also that the VT session is never recreated. A VideoToolbox compression session has fixed dimensions; changing the captured size needs the encoder reconfigure Phase 20 builds for display switching.
**Why it happens:** Apple documents `SCStream.updateConfiguration` but does not state which properties may change while running [CITED: developer.apple.com SCStream updateConfiguration page]. Chromium-style capture updates width and height this way, but that is not verified here.
**How to avoid:** Reuse Phase 20's reconfigure (SPS/PPS on every IDR means no renegotiation, 19-CONTEXT Encode). Force an IDR on every step. Do not bump the input epoch; the scale step changes pixel size only. Debounce steps so a flapping estimate cannot cause repeated IDRs. Verify in a VM that `updateConfiguration` width/height change works on macOS 27.2.

### Pitfall 7: "No new frame" must mean no Complete frame
**What goes wrong:** Using "no frame for 300 ms" as the stillness trigger never fires for about 14 s after the last change because SCK keeps sending Idle frames at about 27 per second [VERIFIED: R15-A text above]. PITFALLS 21 and CONTEXT both say "no frames".
**How to avoid:** Trigger on the absence of `SCFrameStatusComplete` frames (the spike counts them separately: `SCFrameStatusComplete`, `SCFrameStatusIdle`, `SCFrameStatusBlank`, ...) [VERIFIED: spike/desktop/media_darwin.m:787-793].

### Pitfall 8: Stereo audio may play mono
**What goes wrong:** The viewer decodes Opus as mono and the stereo mix is downmixed.
**Why it happens:** libwebrtc's receive-side Opus config defaults to mono when `stereo` is absent from the fmtp (older source; a field trial changes it) [CITED: webrtc.googlesource.com audio_decoder_opus.cc; mediamtx issue 2043]. pion's default Opus registration is `{MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil}` with no `stereo` [VERIFIED: pion/webrtc v4.2.29 mediaengine.go:73].
**How to avoid:** Viewer munges `stereo=1` into its offer; the daemon registers an Opus codec whose fmtp carries `stereo=1;sprop-stereo=1` (check pion still matches it against the browser's `minptime=10;useinbandfec=1` offer). VM test: left-only test tone, WebAudio `ChannelSplitter` energy on L and R, and `getStats()` codec `channels`. Unverified until that test runs.

### Pitfall 9: Variable frame rate and RTP timestamps after idle gaps
**What goes wrong:** SCK delivers frames only on change, so video is variable-rate. With `TrackLocalStaticSample`, the packetizer stamps a sample with the accumulated durations of earlier samples, then adds its own: `Timestamp: p.Timestamp` then `p.Timestamp += samples` [VERIFIED: pion/rtp v1.10.6 packetizer.go:135,141; track_local_static.go WriteSample]. If each sample's `Duration` is the delta to the previous frame, the first frame after a long still gap is stamped almost immediately after the previous frame, and the next frame is stamped a whole gap later. A receiver jitter buffer can then hold frames for about the gap length or reset, which breaks "smooth right after still" and lip-sync.
**How to avoid:** Give video packets timestamps from the capture PTS (`base + (pts - t0) * 90000`), either with `TrackLocalStaticRTP` and `rtp.Packetizer` (set `Timestamp` on each returned packet) or with the timestamp-only advance trick; audio from a sample counter with a resync when the PTS-derived value drifts more than about 60 ms. Add a VM row measuring first-motion-frame latency after 10 s still with `getStats()` `jitterBufferDelay`. The post-idle case was not measured in Phase 17 (R22 measured join on a still screen only) [ASSUMED effect size; needs measurement]. Whether SCK audio and video PTS share one clock is [ASSUMED] (both are expected on the host clock).

### Pitfall 10: Audio reaches every viewer even while muted
**What goes wrong:** Mute is viewer-side only (A-audio), so the Mac's audio (calls, notifications) is delivered to every viewer, including Phase 22 guests, whether or not they unmute; a modified client can play it regardless. CONTEXT accepts this (Discretion: no Share Audio switch).
**How to avoid / decision:** Recorded as Open Question 2. If kept, state it once in the Share Desktop sheet copy. A per-viewer audio gate is possible with a custom `TrackLocal` wrapper but is extra code.

### Pitfall 11: Lock gate conflict with Phase 20
**What goes wrong:** U-2 says "Own-account pointer input still passes" while locked. 20-CONTEXT says "the daemon drops all pointer and key input from every viewer" while locked, and Phase 20's fail-closed rule is what Phase 22 relies on.
**Why it matters:** The lock screen has controls other than the password field. The R15-B frame (macOS 27.2 guest) shows an input-source indicator "ABC" and a keyboard icon at the top right, Wi-Fi, and a "?" next to the password field [VERIFIED: viewed .planning/phases/17-desktop-feasibility-spike/evidence/r15-B-locked.png]. A remote click on the input-source menu can change the layout, after which the key-code map types the wrong password. No power buttons were visible in that frame, so the larger "remote shutdown" worry is not supported by evidence for this macOS build, but other builds or multi-user lock screens are unchecked [ASSUMED].
**How to avoid:** Keep Phase 20's rule (drop pointer too) unless a concrete need for pointer at the lock screen is found; the keyboard-only unlock path does not need it. Recorded as Open Question 1.

### Pitfall 12: The wake key must reveal the field without typing a character
**What goes wrong:** R16-B shows a failure when keys arrive while the field is still hidden; loginwindow logs "View is hidden password field length = 0" on the first key even on success, so the first key reveals the field and is also consumed into it (the six-key password worked 9 of 10 times) [VERIFIED: 17-SPIKE-MATRIX R16-B]. Therefore a printable wake key would be inserted into the password and make a correct password fail.
**How to avoid:** Choose a non-printing wake (candidates: a modifier press, F15, a user-activity assertion), and prove in a VM that it (a) reveals the field and (b) inserts nothing, by checking `length = 0` in loginwindow's log after the wake and before typing. R17 notes a dark display ignored typed keys and a click and a Shift press did not wake the display, only a user-activity assertion did [VERIFIED: 17-SPIKE-MATRIX R17-A], so do both: `IOPMAssertionDeclareUserActivity` first, then the key. Unverified until the VM row runs.

### Pitfall 13: Caps Lock and layout at the lock screen
**What goes wrong:** With Caps Lock on at the lock screen, typed letters are upper case and the password fails; with a different input source at the lock screen than the session's, the key-code map is wrong.
**How to avoid:** Read `CGEventSourceFlagsState` for the alpha-shift flag before typing and refuse with a clear message; compute the map from the layout the lock screen shows (the R15-B frame shows "ABC"), and refuse unmapped runes before typing (already decided). [ASSUMED mechanics; verify in the VM.]

### Pitfall 14: A Go string cannot be zeroed
**What goes wrong:** `json.Unmarshal` into a `string` makes immutable copies; logging the struct prints it.
**How to avoid:** Binary message and `[]byte` (Pattern 6). Wrap the secret in a type whose `String()`, `GoString()`, `MarshalJSON()` and `LogValue()` return a fixed marker, so an accidental `%v` or slog field cannot leak it. Sentinel-grep test over log, audit and error output (decided).

### Pitfall 15: A stolen own-account device can try passwords through Unlock
**What goes wrong:** The always-available link plus a same-user device is enough to reach the Unlock field; nothing limits submits except macOS's own policy, which the office corporate policy may turn into an account lockout.
**How to avoid (recommendation, not in CONTEXT):** daemon-side throttle: at most one submit per about 5 s, and after 3 consecutive failed submits pause Unlock for about 15 min with a clear state ("Unlock paused"). Each submit can cost two macOS attempts (retry once), so 3 submits is 6 attempts. Constants are tunable. Recorded as Open Question 3.

### Pitfall 16: Re-lock depends on a physical-input signal that Phase 22 has not proven
**What goes wrong:** Injected CGEvents may count in the HID idle time, so "someone used the Mac since the unlock" cannot be read from idle time alone while a viewer is controlling. 22-CONTEXT says research must prove which method works in the VM.
**How to avoid:** Arm re-lock only after a Portty-performed unlock (not after the user unlocked physically). Use Phase 22's signal through a seam; if it is not available when this phase executes, default to re-locking (fail secure) after the last own-account viewer leaves and 30 s pass, and log that. The lock action is the DCTL-03 chord (Ctrl+Cmd+Q) posted by the daemon [VERIFIED: 17-SPIKE-MATRIX R15-A drove locks with "Ctrl+Cmd+Q as key 12 with cmd and ctrl"].

### Pitfall 17: The "minos 14.0" gate fails on the first build
**What goes wrong:** See Pattern 9. The local arm64 build and the published v1.2.0 slices report `minos 11.0`, not 10.13, because the linker clamps 10.13 up to the arm64 minimum [VERIFIED: vtool on build/bin/Portty.app/Contents/MacOS/Portty: `minos 11.0`; v1.2.0 arm64 slice: `minos 11.0`].
**How to avoid:** Set both CGO variables in all three build entry points and run `scripts/verify-bundle.sh` on the unsigned bundle in build.yml so a PR catches it, not a tag.

### Pitfall 18: Asset name and identifier must move together, per name
**What goes wrong:** Signing both DMGs with one identifier makes the updater's `Identifier=` check fail for one of them (Verify requires the asset name without .dmg) [VERIFIED: install.go:139 `hasLine(out, "Identifier="+strings.TrimSuffix(assetName, ".dmg"))`].
**How to avoid:** Per-name loop (Pattern 7) and a release_workflow_test.go that parses both names, since the current test extracts a single `DMG_NAME="..."` with a regex and compares one `--identifier` [VERIFIED: release_workflow_test.go:61-75].

### Pitfall 19: Release notes cannot protect Intel and older macOS users
**What goes wrong:** A v1.2.x copy on an Intel Mac or macOS older than 14 is offered the arm64-only v2.0 and installs a build that will not run. The v1.2 updater has no architecture or OS check [VERIFIED: internal/updater/update.go and install.go read in full; no such check].
**How to avoid:** Nothing can be done in the client already in the field. The release note line decided in Discretion is the only mitigation, and `release.yml` currently sets no release body (`softprops/action-gh-release` with `files:` only) [VERIFIED: release.yml:303-316], so add a `body`/`body_path` step. Both of the user's Macs are Apple Silicon on macOS 27.x, so this risk is for other installs.

### Pitfall 20: The first release run is the test
**What goes wrong:** A broken signing or verify step is found only on a real tag, and a bad tag is public.
**How to avoid:** Use a release-candidate tag. `release.yml` makes any tag containing `-rc` a draft and any tag containing `-` a prerelease, so the updater never offers it [VERIFIED: release.yml:310-313 `draft: ${{ contains(github.ref_name, '-rc') }}`, `prerelease: ${{ contains(github.ref_name, '-') }}`]. Run the full pipeline on `v2.0.0-rc.1`, download both DMGs from the draft, run `TestRealDMG` and `TestReleaseJSON` against each, then tag `v2.0.0`. Pushing the tag is a user action. The stamp step turns `v2.0.0-rc.1` into bundle version `2.0.0` [VERIFIED: release.yml:92-97].

### Pitfall 21: Timeout and notarization count
**What goes wrong:** The `sign-macos` job timeout is 150 minutes for "two notarization waits (.app and DMG), each capped at 60m" [VERIFIED: release.yml:143]. Three waits at the same cap is 180 minutes plus overhead.
**How to avoid:** Set about 210 minutes. In practice the whole v1.2.0 Release run took 9m39s [VERIFIED: gh run list], so the cap is a guard, not an expectation. Submitting both DMGs without `--wait` and waiting on both would cut wall time, but adds steps; not needed.

### Pitfall 22: Branch protection does not exist today
**What goes wrong:** CONTEXT and CLAUDE.md rule 4 treat the context rename as a risk to protection. `gh api repos/scottkw/portty/branches/main/protection` returns 404 "Branch not protected" [VERIFIED: gh api]. TESTING.md Section 3 says "CI is currently disabled", but Build and Release are `active` and a Build run was in progress on 2026-11-25 [VERIFIED: gh workflow list, gh run list]. The text is stale.
**How to avoid:** Still update TESTING.md Section 3 and line 18 for the new context string (standing rule), fix the stale CI-disabled sentence, and record "re-apply protection" as a user step that is optional because none is active.

## Code Examples

### Release workflow pins (extend the existing test)
```go
// Source: shape of internal/updater/release_workflow_test.go (read this session). The new test must
// parse BOTH names rather than the single DMG_NAME regex at lines 61-75.
for _, name := range []string{"darwin-universal", "darwin-arm64"} {
    want := "portty-${VERSION}-" + name
    // assert the sign loop mentions want as the --identifier and the verify loop greps "Identifier=" + want
}
// also assert: "wails build -platform darwin/arm64", no "--deep" in the sign step,
// "CGO_LDFLAGS=\"-mmacosx-version-min=14.0\"" and the verify-bundle.sh call, and sign-macos timeout >= 200
```

### Soak sampler
```bash
# Source: macOS tools present on this host (footprint, lsof, ps); thresholds are [ASSUMED] starting points.
PID=$(pgrep -f 'Portty.app/Contents/MacOS/.* daemon' | head -1)
while sleep 60; do
  echo "$(date +%s),$(ps -o rss= -p $PID),$(footprint -p $PID 2>/dev/null | awk '/phys_footprint:/{print $2$3}'),$(lsof -p $PID | wc -l),$(ps -o %cpu= -p $PID)"
done >> soak.csv
# plus: goroutines and stream state from a daemon-socket stats call (stats.go); pass = no crash,
# no restart (same PID), RSS, footprint and fd count flat after a 5 min warm-up.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `codesign --deep` on the bundle | Explicit inside-out signing plus a verify pass | Apple deprecated `--deep` signing guidance (PITFALLS 17) | Required by R-2; trivial loop today (one Mach-O) but the enumeration is the real check |
| Universal (arm64+x86_64) release | arm64-only release | PROJECT Key Decision 2026-11-24 | Halves test surface; DMG names kept for the old updater |
| Go `runtime/secret` for key erasure | Not available on macOS | Go 1.26 experiment is linux/amd64 and linux/arm64 only | Zeroing is best-effort only |
| macOS-latest = macOS 15 | macOS 26 arm64 | GitHub changelog 2026-02/05 | Runner is arm64 so `wails build -platform darwin/arm64` is native; Xcode 27 images exist separately |

**Deprecated/outdated:**
- STATE.md blocker "persistent-content-capture entitlement request is not filed yet": the todo file says FILED 2026-11-25, request T558S9Q5B3 [VERIFIED: .planning/todos/pending/2026-11-24-persistent-content-capture-entitlement.md]. STATE.md is stale on this.
- TESTING.md Section 3 "CI is currently disabled": stale, see Pitfall 22.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `kAudioConverterEncodeBitRate` accepts 64000 for Opus and averages near 64 kbps | Pattern 1 | Constant must change; encoder error path |
| A2 | Each DMG needs its own notarization because its signature hash differs | Pattern 7 | One fewer wait; harmless if wrong |
| A3 | `codesign -d --entitlements :-` and `--xml` output formats for the entitlements assertion | Pattern 8 | Verify script tweak |
| A4 | Browsers' TWCC support for Safari, iPhone and WKWebView | Pitfall 2 | No adaptation for that viewer; needs fallback |
| A5 | GCC start-at-ceiling avoids the low-estimate trap | Pitfall 1 | Policy needs a different seed or probing |
| A6 | Pion unbounded pacer queue grows for a stalled viewer in practice (read in source, not measured) | Pitfall 3 | Smaller issue than expected |
| A7 | `Quality` or runtime AverageBitRate/DataRateLimits take effect on the Apple Silicon hardware H.264 encoder | Pitfall 4 | Refinement and live bitrate need other mechanisms |
| A8 | Re-encoding the same buffer 2-3 times refines a P-frame sequence | Pattern 5 | Fallback (c) unavailable |
| A9 | `SCStream.updateConfiguration` can change width and height while running on macOS 27.2 | Pitfall 6 | Resolution step needs a stream restart |
| A10 | Post-idle first-frame latency problem from sample-duration timestamps | Pitfall 9 | Smaller effect; simpler implementation OK |
| A11 | SCK audio and video PTS share one host clock | Pitfall 9 | A/V sync offset |
| A12 | libwebrtc treats an offer with `stereo=1` as stereo receive; pion matches the stereo fmtp | Pitfall 8 | Mono playback |
| A13 | A non-printing key reveals the password field and inserts nothing | Pitfall 12 | Unlock fails or mistypes |
| A14 | Caps Lock flag readable via `CGEventSourceFlagsState` at the lock screen | Pitfall 13 | Failed unlock due to case |
| A15 | Other macOS builds or multi-user lock screens may show power controls | Pitfall 11 | Pointer risk larger or smaller |
| A16 | `TISCopyCurrentKeyboardLayoutInputSource` + `UCKeyTranslate` is the right reverse map and works from a launchd daemon | Standard Stack | Typing map needs a different source |
| A17 | iPhone ringer switch behavior for `<video>` audio (PITFALLS 19 says Safari obeys it) | Pitfall 8 notes | M-row wording only |
| A18 | Starting constants (floor, ceiling, ladder, hysteresis, throttle) | Pattern 4, Pitfall 15 | Tuning only |
| A19 | `vnet` composes with Phase 19's UDP mux seam | Pattern 10 | Use the UDP shaper |
| A20 | macos-latest remains arm64 for the life of the phase | Standard Stack | Build flag or runner label change |

## Open Questions / User Decisions Needed

1. **Pointer input while locked (conflicts with Phase 20).**
   - What we know: U-2 lets own-account pointer pass while locked; 20-CONTEXT drops all pointer and key input while locked and says Phase 23 opens only a dedicated unlock path. The keyboard-only unlock path needs no pointer. The lock screen has an input-source menu that could change the layout.
   - What's unclear: whether any pointer need exists at the lock screen.
   - Recommendation: drop pointer input from every viewer while locked, matching Phase 20, and revisit only if a concrete need appears. Needs a user yes before planning edits U-2.

2. **Audio reaches guests and muted viewers.**
   - What we know: Discretion says guests hear audio like the picture and there is no host switch; audio is sent whether or not the viewer unmutes.
   - Recommendation: keep the decision, add one line in the Share Desktop sheet ("Guests can hear this Mac's audio"). A per-viewer audio gate is optional later.

3. **Unlock throttle (new).**
   - Recommendation: one submit per about 5 s, and 3 consecutive failed submits pause Unlock for about 15 min. Tunable constants, daemon-side. Accept or drop.

4. **Re-lock signal source.**
   - Recommendation: arm only after a Portty-performed unlock; use Phase 22's physical-input signal; if absent at execute time, re-lock anyway (fail secure). Confirm the fail-secure default.

5. **Release-candidate dry run.**
   - Recommendation: push `v2.0.0-rc.1` (draft, never offered) and verify both DMGs before the real tag. The user pushes the tag.

6. **Branch protection.**
   - What we know: none is applied today. Recommendation: update TESTING.md for the new context name; decide separately whether to enable protection at all.

7. **Opus bitrate.** 64 kbps (CONTEXT) vs the spike's tested 96 kbps. Recommendation: keep 64 kbps, verify in a VM probe, fall back to 96 if the converter rejects it.

8. **Release notes mechanism.** Recommendation: add a `body_path` to the publish step so the "Apple Silicon and macOS 14 or later only" line cannot be forgotten.

9. **Phases 19-22 are unbuilt.** The plan text should name interfaces from the contract checklist and the planner should re-validate paths and symbols when those phases land. Recommendation: put the release workstream in the first wave since it does not depend on them.

10. **VM availability.** Both Tart VMs showed "running" in `tart list`, but `tart ip portty-test` printed "no IP address found" and `tart exec portty-test /usr/bin/sw_vers` timed out after 20 s (no observation; not investigated further). Every VM item above is unverified until the rig is healthy.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Xcode CLT: `codesign`, `vtool`, `lipo`, `file` | verify script, local checks | yes | on this host | — |
| `notarytool`, `stapler` | release workflow, local rehearsal | yes (under Xcode.app, not on PATH; `xcrun --find` resolves) | Xcode on host | CI runner has them (v1.2.0 release passed) |
| `gh` CLI (authenticated) | release checks, rc dry run | yes | — | — |
| Go | tests, builds | go1.27.4 locally; go.mod pins go 1.26.4 and TESTING.md says `GOTOOLCHAIN=go1.26.11` | — | CI uses `go-version-file` |
| Tart | VM rows | installed; `portty-test` and `portty-test-2` listed running | — | Both unreachable this session (no IP, exec timeout); desk research only, items unverified |
| macOS 27.2.1 arm64 host | build probe only | yes | — | — |
| Real Apple output device, office Mac | M-162, M-161, soak | not available to research | — | User-run M-NN rows |
| CI secrets (`MACOS_*`) | signing and notarization | present (v1.2.0 release succeeded 2026-11-23) | — | — |

**Missing dependencies with no fallback:**
- A reachable VM rig for the unlock, audio, TWCC, refinement and `updateConfiguration` probes. Not blocking research, but blocks any claim above that is tagged unverified.

**Missing dependencies with fallback:**
- None beyond the VM rig.

## Security Domain

`security_enforcement` is enabled in `.planning/config.json` with ASVS level 1 and block on high.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Unlock accepts a macOS login password: own-account grant check in the daemon (WhoIs same-user gate then grant kind), throttle (Pitfall 15), never persisted |
| V3 Session Management | no | No new session; reuses Phase 19/22 grants |
| V4 Access Control | yes | Daemon enforces grant kind for unlock and for the lock-time input drop; guests never unlock; UI hiding is cosmetic |
| V5 Input Validation | yes | Unlock message size cap and UTF-8 validation, closed rune-to-key map with refusal, Phase 20 message limits (4 KiB) apply |
| V6 Cryptography | yes | DTLS data channel only (pion); no custom crypto; zeroing is best-effort |
| V7 Logging | yes | Never log the password, its length, key codes or typed text; sentinel-grep test across log, audit and error output |
| V14 Configuration | yes | Hardened runtime, timestamp, unchanged entitlements, no `get-task-allow`, designated requirement pinned, every Mach-O verified |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Password leaks through logs, errors, metrics, crash output or JSON structs | Information disclosure | Byte buffer, redacting wrapper type, sentinel-grep test, no length in audit |
| Password brute force through a compromised own-account device | Spoofing / EoP | Daemon throttle and pause (Pitfall 15), macOS's own limits as backstop |
| Guest with control types at the lock screen | EoP | Lock-time key drop for every viewer; unlock only for own-account grant; no pointer while locked (Open Question 1) |
| Concurrent unlock attempts interleave keystrokes | Tampering | One attempt at a time; viewer input dropped during the attempt |
| Wrong layout or Caps Lock types a wrong password and burns attempts | Denial of service | Pre-type validation, refuse unmapped runes, Caps Lock check |
| Unsigned or wrong-team nested code in a release | Tampering | Mach-O enumeration, Team ID, runtime flag, DR pin in CI |
| Release asset name or identifier mismatch strands installs | Denial of service | Per-name sign loop, tests pinning both names, rc dry run |
| Audio leaks office conversations to guests | Information disclosure | Documented decision (Open Question 2) |
| Persistent unlocked Mac after the remote session ends | EoP | Re-lock after last own-account viewer leaves (D-relock, Pitfall 16) |
| Password in a browser password manager | Information disclosure | `autocomplete="off"`-style attributes, no `<form>` submit, clear on submit (JS strings cannot be zeroed; documented limit) |

## Sources

### Primary (HIGH confidence)
- Repo files read this session with line citations: `internal/updater/update.go`, `install.go`, `release_workflow_test.go`, `real_darwin_test.go`; `.github/workflows/release.yml`, `build.yml`; `build/entitlements.plist`, `build/darwin/Info.plist`, `wails.json`; `build.sh`, `tests/build-script.test.sh`; `TESTING.md` (Sections 2-3, Category AU); `spike/desktop/media_darwin.m`, `bundle/build.sh`, `bundle/entitlements.plist`
- `.planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md` rows R15-R17, R20, R21, R22 and the closing table; `evidence/r15-B-locked.png` viewed
- `.planning/phases/18-self-installing-updates/18-CONTEXT.md` and 18-02-PLAN.md (DMGAssetNames, CheckStagedApp); 19-CONTEXT, 20-CONTEXT, 21-CONTEXT, 22-CONTEXT
- pion source in the module cache: `pion/interceptor@v0.1.51/pkg/gcc` (rate_controller.go, send_side_bwe.go, leaky_bucket_pacer.go, noop_pacer.go) and `pkg/cc/interceptor.go`; `pion/webrtc/v4@v4.2.29` (interceptor.go, mediaengine.go:73, track_local_static.go, examples/bandwidth-estimation-from-disk/main.go); `pion/rtp@v1.10.6/packetizer.go`
- `wailsapp/wails/v2@v2.10.2/pkg/commands/build/base.go:295-365`
- Live checks: `gh release view v1.2.0`, `gh workflow list`, `gh run list`, `gh api .../branches/main/protection`; v1.2.0 DMG downloaded to the session scratch dir, mounted read-only there, inspected with `codesign`, `lipo`, `vtool`, detached (mount count 0 afterwards); scratch cgo builds for minos (never executed); `go list -m -versions`

### Secondary (MEDIUM confidence)
- [Apple: SCStream updateConfiguration](https://developer.apple.com/tutorials/data/documentation/screencapturekit/scstream/updateconfiguration(_:completionhandler:).json): states the call exists, lists no property restrictions
- [Apple: kVTCompressionPropertyKey_Quality](https://developer.apple.com/tutorials/data/documentation/videotoolbox/kvtcompressionpropertykey_quality.json): range only, no H.264 hardware statement
- [Chrome autoplay policy](https://developer.chrome.com/blog/autoplay): muted autoplay, unmute from a gesture
- [Go runtime/secret](https://pkg.go.dev/runtime/secret@go1.26.0): linux/amd64 and linux/arm64 only
- [GitHub changelog: macOS 26 GA for hosted runners](https://github.blog/changelog/2026-02-26-macos-26-is-now-generally-available-for-github-hosted-runners/)
- `.planning/research/PITFALLS.md` (Pitfalls 17-21), `STACK.md`, `ARCHITECTURE.md`

### Tertiary (LOW confidence)
- Apple developer forum threads on VideoToolbox rate control: [118021](https://developer.apple.com/forums/thread/118021), [111647](https://developer.apple.com/forums/thread/111647), [799459](https://developer.apple.com/forums/thread/799459): anecdotal and mixed
- [libwebrtc audio_decoder_opus.cc](https://webrtc.googlesource.com/src/+/branch-heads/72/api/audio_codecs/opus/audio_decoder_opus.cc) (branch 72) and [mediamtx issue 2043](https://github.com/bluenviron/mediamtx/issues/2043): stereo default; current main not checked
- Old Safari SDP offer gists showing transport-cc; dates 2017-2020, not current

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new dependency; pion versions and API read from source; system frameworks proven by the spike.
- Release pipeline and updater compatibility: HIGH. Current code, real release, real signature, and a build experiment.
- Architecture (audio, quality, unlock): MEDIUM. Rests on designs of unbuilt phases and unmeasured VideoToolbox/TWCC behavior.
- Pitfalls: MEDIUM. Items from pion source are firm; browser and encoder behavior items are flagged assumed and need VM rows.

**Research date:** 2026-11-25
**Valid until:** 2026-12-09 for the release and tooling findings (fast-moving runner images and Xcode); re-validate everything tied to Phases 19-22 when they land.
