# Phase 23: Audio, Adaptive Quality, Unlock and Release - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Viewers hear the shared Mac's system audio with the picture, starting muted with one tap or click to unmute, in every viewer: browser, iPhone and the Portty app tab (DCTL-07). The picture is sharp, with small text readable, when the screen is still, and smooth while scrolling or dragging. It adapts as the network gets worse or better without stalling other viewers (DESK-07). Remote unlock ships, because the 17-SPIKE-MATRIX verdict for R15-R17 is "feasible, ship". A viewer on the host's own account sees the lock screen of a locked, logged-in Mac and types the password. The daemon types after a wake key, checks CGSSessionScreenIsLocked and retries once. The password is never stored or logged, and click-to-share guests never get unlock (UNLK-01; ROADMAP SC3). v2.0 ships as a signed, notarized Apple Silicon release with minimum macOS 14, with every nested binary signed and verified in CI, and an installed v1.2.x copy still finds, verifies and installs it (REL-01; ROADMAP SC4). Hardening on real Macs covers two checks. A two-build upgrade with a live desktop viewer and live terminal sessions keeps every session and brings the viewer back. A one-hour desktop soak on the office Mac ends with no crash, leak or stuck state (ROADMAP SC5). Not in this phase: the desktop wire protocol, capture host, share-server routes and state model (Phase 19); the input path, control data channel and display or window switching (Phase 20); the Wails Desktop tab and the iPhone touch viewer (Phase 21); guest admit, per-viewer grants, soft control hand-off and the audit log (Phase 22). Persistent-content-capture wiring is also out, unless Apple grants the entitlement in time (see Release packaging and signing).

</domain>

<decisions>
## Implementation Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, and product and UX choices are under Claude's Discretion. The user accepted the five hard-to-reverse recommendations below with "Accept all" (2026-11-25). Audio, adaptive quality and hardening had no hard-to-reverse questions.

### Release packaging and signing
- The v2.0 release publishes two DMGs built from one arm64-only build: `portty-vX.Y.Z-darwin-universal.dmg`, kept only so v1.2.x copies find the update, and `portty-vX.Y.Z-darwin-arm64.dmg`. Each DMG is signed with `--identifier` equal to its name without .dmg, and each is notarized, stapled and checked the way the updater checks it. Both go into checksums.txt. Both names keep shipping until a later release decides to drop the universal name, which is safe once both Macs run a build from Phase 18 on — **Reversibility:** one-way — release asset names are a published contract read by every installed copy (v1.2's updater knows only the universal name and its identifier), so a missing name strands that copy's update path.
- Signing switches from `codesign --deep` to explicit inside-out signing of every Mach-O in the bundle. Today that is only Contents/MacOS/Portty, because capture runs in the daemon. Signing keeps identifier `app.portty.Portty`, hardened runtime, timestamp and the unchanged entitlements file (network client and server only); SCK audio and CGEvent typing need no entitlement. CI enumerates every Mach-O. It fails on unsigned code, the wrong team, a missing runtime flag, a non-arm64 slice or a minos below 14.0. It also pins the designated requirement to the v1.2 one — **Reversibility:** one-way — the office Mac's Screen Recording and Accessibility grants are keyed to the designated requirement, so a changed identifier or requirement silently drops them and costs an in-person trip to the unattended Mac.
- v2.0 does not wait for the persistent-content-capture entitlement (request T558S9Q5B3). The signing step is written so a provisioning profile and one entitlement can be added later without other changes. If Apple grants the request before Phase 23 planning closes, wiring it becomes a separate last plan. That plan runs only after the plain release pipeline is green and has been tested on the two-build rig; Phase 18 rollback covers a profile that will not launch — **Reversibility:** one-way — an embedded profile and a restricted entitlement change the signature every later build must carry and tie the app to a profile that expires; a bad profile stops the app launching on the remote Mac.

### Remote unlock
- Each viewer gets a dedicated Unlock field, shown only to own-account viewers while the state is Locked. Submit sends the password as one message on the encrypted control data channel: DTLS end to end to the daemon, never the signalling socket and never a URL. The daemon holds it in one byte buffer only for the attempt: wake key, wait for the field, type, check, at most one retry, about 10 s. It then zeroes the buffer. The password is never logged and never appears in the audit log, not even its length. It is never persisted, and it is kept out of error messages and crash state. The viewer clears its field on submit and keeps nothing — **Reversibility:** one-way — this is the security boundary for a macOS login password and a wire message every viewer build sends; UNLK-01's retry-once needs the daemon to retype, which keystroke passthrough cannot do.
- While CGSSessionScreenIsLocked is set, the daemon drops ordinary key events from every viewer, own-account and guests alike. The Unlock field is the only way to type at the lock screen. Own-account pointer input still passes, and guest pointer input is dropped too — **Reversibility:** costly — it is a security rule that Phase 20-22 input code must keep (guests with control could otherwise type at the lock screen, against SC3); loosening it later is easy, tightening it after people rely on typing is not.

### Decided autonomously
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
- After research (2026-11-25, user delegated): clicks are also blocked while the Mac is locked (matches Phase 20; a stray click on the lock screen's keyboard-layout menu could corrupt the typed password); the unlock box is the only input while locked. The wake key produces no character.
- After research: audio goes to every viewer that sees the picture, including guests and muted viewers; the share sheet says so.
- After research: unlock is rate-limited to one try about every 5 s; 3 failures pause remote unlock for about 15 minutes.
- After research: if the 'someone is at the keyboard' signal is unavailable, the Mac re-locks anyway (fail secure).
- After research: a v2.0.0-rc.1 draft-release dry run is planned before the real release; pushing that tag stays the user's call at that time.
- After research: release builds set both CGO_CFLAGS and CGO_LDFLAGS to -mmacosx-version-min=14.0 (Wails forces 10.13 otherwise), and build.yml checks every Mach-O's minos so a PR catches it.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/updater/update.go:78 DMGAssetName returns only the universal name; Phase 18 (18-02) adds DMGAssetNames [universal, arm64] and widens Clean; Phase 23 publishes what that list expects.
- internal/updater/install.go:162-175 Clean globs only portty-v*-darwin-universal.dmg and .part (Phase 18 widens it).
- internal/updater/release_workflow_test.go:68-102 pins the DMG --identifier to the asset name and the verify step's Identifier line; extend it for two names.
- spike/desktop/media_darwin.m:470+ AudioConverter Opus recipe (960 frames, 48 kHz, 96 kbps in the spike) and the SCK audio output; bridge_darwin.go opusprobe; reference only, never imported.
- spike/desktop/bundle/build.sh: inside-out per-Mach-O signing trial with a team check and refusal of ad-hoc signatures.
- R16 typing method: CGEventPost at the HID tap, key down and up 60 ms apart, from the in-daemon process with Accessibility.

### Established Patterns
- Phase 19 creates internal/screencap (cgo ObjC: SCK, VT, IOKit, CGSession) and internal/desktop (pion peers, fan-out, state, signalling); audio and adaptive quality extend both, and the encoder constants Phase 19 fixes are what DESK-07 makes adaptive.
- Release checks are pinned by Go tests that read release.yml (release_workflow_test.go); the DMG signing identifier equals the asset name without .dmg.
- build.yml keeps a single-entry matrix so the required check context name stays stable; changing it requires re-applying branch protection (TESTING.md Section 3, CLAUDE.md rule 4).
- TESTING.md standing rule (Section 6): new test files in Section 2, requirement rows in Section 4, manual M-NN items in Section 5.

### Integration Points
- .github/workflows/release.yml: `wails build -platform darwin/universal`, artifact `portty-darwin-universal-unsigned`, `codesign --deep`, one DMG create/sign/notarize/verify step set hardcoded to the universal name, sign-macos timeout 150 min, publish uploads artifacts/*.dmg and checksums.txt.
- .github/workflows/build.yml:9-17,80-85 single-entry matrix with check context `build (portty, darwin/universal, macos-latest)`.
- build.sh and tests/build-script.test.sh:236-241 assert `darwin/universal`.
- build/entitlements.plist holds only network.client and network.server; build/darwin/Info.plist has CFBundleIdentifier app.portty.Portty and LSMinimumSystemVersion 10.13.0 (Phase 19 raises it to 14.0); build/bin/Portty.app today holds one Mach-O (Contents/MacOS/Portty).
- frontend DesktopView.tsx and lib/desktopClient.ts (Phase 19) get the unmute control and the Unlock field; Phase 21 reuses them in the Wails tab and the iPhone view.
- Phase 20's control data channel carries the unlock message; Phase 22's audit log gains the unlock event.

</code_context>

<specifics>
## Specific Ideas

Settled by prior decisions (not re-asked):
- Opus is encoded with AudioToolbox AudioConverter, with no libopus and no new audio dependency (research SUMMARY C7; 17-SPIKE-MATRIX R21 GO: framesPerPacket 960, SCK system audio delivered and encoded in the guest).
- System audio comes from the same ScreenCaptureKit stream (capturesAudio, 48 kHz stereo) and is covered by the existing Screen Recording grant; no microphone and no new TCC category (R21; REQUIREMENTS Out of Scope: microphone/voice).
- Audio starts muted on the viewer with tap to unmute (DCTL-07 text; iOS will not autoplay, R14 and DVWR-03).
- Bandwidth estimation uses pion's GCC with TWCC from pion/interceptor v0.1.51, which Phase 19 already pins as a root dependency with pion/webrtc v4.2.29, so adaptive quality adds no new module (19-CONTEXT specifics; research STACK pion/interceptor pkg/gcc).
- One shared encoder serves all viewers, with coalesced PLI/FIR and an IDR on join (19-CONTEXT Decided autonomously: Encode; research ARCHITECTURE item 2; R22 GO).
- The default cap stays 2560 px on the long edge at 30 fps, measured on the MacBook Air at 29.15 fps, 12.85 ms mean encode latency, 27 percent of one core and SSIM 0.9993 (research C4; R20-host GO). H.264 Constrained Baseline stays the codec, and HEVC and High profile are Future (REQUIREMENTS Future).
- Unlock is feasible and ships, not cut (17-SPIKE-MATRIX closing table row R15/R16/R17; REQUIREMENTS UNLK-01 "feasible per FEAS-01"; R16-B: 11 of 13 daemon tries unlocked, and the one failure mode is keys arriving while loginwindow's password field is still hidden).
- UNLK-01 fixes the unlock sequence: a wake key first, wait for the password field, type, check CGSSessionScreenIsLocked, retry once (R16-B Phase 23 constraint).
- The password is never stored or logged, and stored-password unlock is out of scope (UNLK-01; REQUIREMENTS Out of Scope).
- Only a viewer on the host's own Tailscale account may unlock; click-to-share guests never get unlock (UNLK-01; ROADMAP SC3).
- Only "locked, still logged in" is in scope; pre-login, reboot and FileVault are out (research SUMMARY User Facts; REQUIREMENTS Out of Scope). The office Mac uses the plain macOS lock screen, with no Jamf Connect or Platform SSO (research SUMMARY User Facts).
- Lock state comes from CGSessionCopyCurrentDictionary key CGSSessionScreenIsLocked, and lock-screen frames keep streaming while the display is held awake (19-CONTEXT Decided autonomously: Lock; R15, R17).
- Phase 19 holds the display awake while viewers are connected (DPRM-04), so the lock screen is visible and typeable whenever a viewer is connected (R15 and R17: without the assertion the stream dies about 31 s after a lock).
- Releases are Apple Silicon only with minimum macOS 14, and universal builds are dropped (PROJECT Key Decision 2026-11-24; REL-01; REQUIREMENTS Out of Scope: Intel Macs). Phase 19 already raises LSMinimumSystemVersion to 14.0 and builds cgo with -mmacosx-version-min=14.0 (19-CONTEXT Decided autonomously: Build).
- The arm64 asset name is `portty-vX.Y.Z-darwin-arm64.dmg`, and every build from Phase 18 on accepts it alongside the universal name, universal first (18-CONTEXT Decided autonomously: Assets; the 18-02 PLAN checkpoint O1 confirms the exact name).
- The updater binds the DMG signing identifier to the asset name without .dmg, and release_workflow_test.go pins that rule (internal/updater Verify; 18-RESEARCH Pattern 9).
- One Mach-O and one TCC identity: capture, encode, input and WebRTC run in the daemon under the app bundle ID app.portty.Portty, with no helper bundle (17-SPIKE-MATRIX FEAS-02 DECIDED, in-daemon; 19-CONTEXT specifics).
- Desktop sessions are not handed off on upgrade. Viewers reconnect by themselves, and HandoffVersion stays 1 (19-CONTEXT specifics; ROADMAP Phase 19 notes).
- The persistent-content-capture request is filed (T558S9Q5B3), and wiring the provisioning profile is a Future requirement once Apple grants it (todo 2026-11-24-persistent-content-capture-entitlement; REQUIREMENTS Future).
- Silent update install while viewers are connected is out of scope (REQUIREMENTS Out of Scope).

Upstream dependencies on Phases 20-22 (planned in parallel; their protocols are upstream and not decided here):
- Phase 20 owns the input path and the control data channel. The unlock message and the lock-time key drop ride on them, so Phase 23 adds one message type and one filter, not a new channel.
- Phase 20's DCTL-03 Send-keys menu already has Lock Screen, and the re-lock-on-leave behaviour reuses that lock action.
- Phase 20's no-stuck-keys rule (release all keys on blur or disconnect) must also run when the lock-time key drop starts.
- Phase 21 owns the Wails Desktop tab and the iPhone viewer. Both get the unmute control and the Unlock field through the shared DesktopView, and the iPhone needs a user tap for audio (R14).
- Phase 22 owns grant kinds (own account vs guest), per-viewer control grants, soft control hand-off and the audit log format. The own-account check for unlock and the unlock audit event depend on them.
- Phase 22's DCTL-04 "physical input at the Mac always wins" is the signal the re-lock rule uses to know someone is at the Mac.
- Phase 19's signalling version field and state enum (including locked) are the base for the audio m-line, the TWCC extension and the unlock result state.

Other specifics:
- Manual step: the build.yml matrix context changes from `build (portty, darwin/universal, macos-latest)` to `build (portty, darwin/arm64, macos-latest)`, so the user must re-apply branch protection on main with the new context name (TESTING.md Section 3; CLAUDE.md rule 4).
- Risk: UNLK-01's retry-once costs two attempts when the password is actually wrong, and each try counts toward macOS's failed-attempt limit, which a corporate policy on the office Mac may enforce (delays or account lockout). The viewer therefore never retries beyond the one daemon retry per submit.
- ROADMAP SC4 states the asset trap directly: v1.2's updater looks only for `portty-v<ver>-darwin-universal.dmg` and that signing identifier; STATE Blockers carries the same concern for REL-01.
- Phase 18's lookup prefers the universal name when both are present. While both ship, builds from Phase 18 on install the universal-named DMG, and dropping that name later moves them to the arm64 name with no client change.
- R16-B evidence: on a success, loginwindow logs one "View is hidden password field length = 0" keyPressed line, because the first key reveals the field. On the failed try it logged that line for all six keys and then "INCORRECT password".
- R15/R17: without a display-awake assertion the lock-screen stream ends about 31 s after the lock (SCError -3815). Keys typed into a dark display did not unlock, and only a user-activity assertion woke it.
- R20-host numbers are the starting point for the adaptive ceiling: 2560 x 1664 native, 29.15 fps, 27 percent of one core in motion and 2.1 percent still. M-161 (the office Mac encoder at native size) is still open.
- R21 ran only on the guest's Apple Virtual Sound Device; M-162 (real output device) is still open.
- The office Mac is unattended and nobody can get in to fix it, so signing must not change the TCC identity, and the entitlement must not risk a launch failure there.
- Research rules carry over: no contact with the host's tailscaled or 100.x addresses during research, and nothing on the host touches ~/Downloads or /Applications. Unlock typing runs only in the VMs (public password) or on the office Mac with the user present.

</specifics>

<deferred>
## Deferred Ideas

- Persistent-content-capture provisioning profile wiring, if Apple has not granted it before Phase 23 planning closes (REQUIREMENTS Future).
- Dropping the universal-named DMG: a later release, once both Macs run a build from Phase 18 on.
- libopus: only if AudioConverter Opus fails on the office Mac's real output device (M-162); that would be a new dependency and a new question.
- A second, lower-bitrate encoder tier for slow viewers (research PITFALLS: a later optimisation, not v2.0).
- HEVC, H.264 High profile and a data-channel cursor (REQUIREMENTS Future).
- Unlock at the login window after reboot or with FileVault (REQUIREMENTS Out of Scope).

</deferred>
