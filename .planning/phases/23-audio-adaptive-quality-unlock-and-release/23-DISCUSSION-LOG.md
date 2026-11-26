# Phase 23: Audio, Adaptive Quality, Unlock and Release - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-25
**Phase:** 23-audio-adaptive-quality-unlock-and-release
**Areas discussed:** Release packaging and signing, Remote unlock (Audio, Adaptive quality and Hardening were all settled or reversible and not presented)

**Mode:** autonomous smart discuss. The user delegated on 2026-11-25 ("I don't need to answer a bunch of questions"). Reversible technical choices were decided autonomously, product and UX choices went to Claude's Discretion, and only hard-to-reverse questions were presented, each with a recommendation. The user replied "Accept all" for all five.

---

## Release packaging and signing

Q1 (R1): Which DMG names does the v2.0 release publish, and for how long?

| Option | Description | Selected |
|--------|-------------|----------|
| Both names from one arm64 build | `portty-vX.Y.Z-darwin-universal.dmg` (for v1.2.x copies) and `portty-vX.Y.Z-darwin-arm64.dmg`, each signed with identifier = name without .dmg, notarized and verified; keep both until a later release drops the universal name (one-way) | ✓ |
| Universal name only | Arm64 content under the old name; no second notarization, but misleading and the arm64 name stays untested | |
| Arm64 name only | Clean, but strands every v1.2.x install (fails SC4) | |

Q2 (R2): How is the app signed so every nested binary is signed and verified?

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit inside-out signing with a CI Mach-O loop | Identifier app.portty.Portty kept, entitlements unchanged, CI checks team, runtime, arm64-only, minos 14.0 and pins the designated requirement to v1.2's (one-way: TCC grants on the office Mac) | ✓ |
| Keep `codesign --deep`, add only the CI check | Works for one Mach-O, but Apple discourages --deep and it hides a future nested binary | |
| Per-binary signing with new identifiers | Only needed if a helper returns; resets grants | |

Q3 (R3): What does Phase 23 do with the persistent-content-capture entitlement?

| Option | Description | Selected |
|--------|-------------|----------|
| Not a ship requirement; profile-ready signing; a separate last plan only if granted before planning closes | Ships on time; a bad profile cannot block the release; Phase 18 rollback covers a profile that will not launch (one-way) | ✓ |
| Hold v2.0 until Apple grants it | Removes the monthly re-approval trip from day one, but blocks the milestone for weeks to months | |
| Leave it to a later milestone even if granted in time | Simplest, but keeps the monthly trip longer than needed | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Remote unlock

Q4 (U1): How does the password get from the viewer to the lock screen, and where may it live?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated Unlock field, one message on the control data channel, daemon holds it only for the attempt then zeroes it | Never logged, audited, persisted or in errors; supports UNLK-01's daemon retry (one-way: security boundary and wire message) | ✓ |
| Plain keystroke passthrough | Nothing new to hold, but no daemon retry (contradicts UNLK-01) and the hidden-field race stays | |
| Viewer-side retry | The daemon never holds it, but the password lives longer in a browser tab, including on an iPhone | |

Q5 (U2): While the Mac is locked, can ordinary keyboard input reach the lock screen?

| Option | Description | Selected |
|--------|-------------|----------|
| No: drop ordinary key events from every viewer while locked; Unlock field is the only typing path; own-account pointer passes, guest pointer dropped | Guests cannot type a password via the input path (SC3); one path to keep out of logs (costly) | ✓ |
| Allow own-account typing, block only guests | Familiar, but two password paths and the race returns | |
| Block only guests' keys, let guests click | Smallest change, but a guest can drive the lock screen UI | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Decided autonomously

[auto] [Audio] — Q: "When is audio captured and sent?" → Selected: "Whenever the desktop stream runs, on the same SCStream with excludesCurrentProcessAudio on; mute is on the viewer only" (reversible technical — decided autonomously: runtime policy)
[auto] [Audio] — Q: "Opus parameters?" → Selected: "48 kHz stereo, 20 ms frames, about 64 kbps, SDP opus/48000/2 stereo=1" (reversible technical — decided autonomously: encoder constants)
[auto] [Audio] — Q: "How do audio and video stay in sync?" → Selected: "Two tracks in one MediaStream stamped from SCK presentation timestamps" (reversible technical — decided autonomously: local timing code)
[auto] [Audio] — Q: "What if the encoder or audio capture fails?" → Selected: "Audio failure becomes a state; the picture keeps streaming" (reversible technical — decided autonomously: local error handling)
[auto] [Audio] — Q: "Real output device check?" → Selected: "M-162 runs in this phase on a real Mac" (reversible technical — decided autonomously: test row)
[auto] [Adaptive] — Q: "Bitrate policy across viewers?" → Selected: "Minimum of per-viewer GCC estimates, clamped, with hysteresis; viewers below the floor left out of the minimum" (reversible technical — decided autonomously: tunable constants)
[auto] [Adaptive] — Q: "How is 'sharp when still' made?" → Selected: "One high-quality refinement of the cached frame after about 300 ms idle, idle refresh about every 2 s" (reversible technical — decided autonomously: encoder state machine)
[auto] [Adaptive] — Q: "How does motion degrade under a poor network?" → Selected: "Bitrate, then frame rate, then resolution, recovered in reverse" (reversible technical — decided autonomously: policy ladder)
[auto] [Adaptive] — Q: "How are bitrate changes applied?" → Selected: "Only through VideoToolbox session properties" (reversible technical — decided autonomously: local code)
[auto] [Adaptive] — Q: "How is it tested?" → Selected: "Policy unit tests, loopback with a shaped link, a real two-Mac M-NN row" (reversible technical — decided autonomously: test layout)
[auto] [Unlock] — Q: "How is 'wait for the password field' done?" → Selected: "One harmless key or a user-activity assertion, about 1 s settle, then type" (reversible technical — decided autonomously: a timing constant)
[auto] [Unlock] — Q: "How are password characters typed?" → Selected: "CGEventPost key codes mapped through the current layout; unmappable characters refuse before typing" (reversible technical — decided autonomously: local mapping code)
[auto] [Unlock] — Q: "When does the retry run?" → Selected: "If still locked about 3 s after Return, retype once, then zero the buffer" (reversible technical — decided autonomously: UNLK-01 fixes the count)
[auto] [Unlock] — Q: "Is unlock audited?" → Selected: "An unlock event in Phase 22's audit log, never the password or its length" (reversible technical — decided autonomously: an additive event type)
[auto] [Unlock] — Q: "Who enforces own-account-only?" → Selected: "The daemon checks the grant kind before accepting an unlock message" (reversible technical — decided autonomously: server-side check)
[auto] [Unlock] — Q: "How is it tested?" → Selected: "Buffer-zeroing and sentinel-password log tests, VM rows, an office-Mac M-NN row" (reversible technical — decided autonomously: test layout)
[auto] [Release] — Q: "Build target?" → Selected: "wails build -platform darwin/arm64; artifact names drop 'universal'" (reversible technical — decided autonomously: CI config)
[auto] [Release] — Q: "Branch protection?" → Selected: "Context becomes build (portty, darwin/arm64, macos-latest); TESTING.md Section 3 updated; user re-applies protection" (reversible technical — decided autonomously: CI config plus a user step)
[auto] [Release] — Q: "Notarization time with two DMGs?" → Selected: "Raise the sign-macos timeout for three notarization waits" (reversible technical — decided autonomously: a CI number)
[auto] [Release] — Q: "How are the release checks pinned?" → Selected: "Extend release_workflow_test.go for both names, arm64-only, no --deep, the verify loop" (reversible technical — decided autonomously: test only)
[auto] [Release] — Q: "First real release check?" → Selected: "M-93 on v2.0, plus v1.2.x and Phase 18 build update paths" (reversible technical — decided autonomously: manual rows)
[auto] [Hardening] — Q: "SC5 upgrade check rig?" → Selected: "Phase 18 two-build rig in the VMs with a live desktop viewer, then the office Mac" (reversible technical — decided autonomously: test rig)
[auto] [Hardening] — Q: "SC5 soak measurement?" → Selected: "One hour, RSS, goroutines, fds, CPU and state every 60 s; flat after warm-up" (reversible technical — decided autonomously: thresholds tunable)
[auto] [Hardening] — Q: "TESTING.md?" → Selected: "Sections 2, 4 and 5 per the standing rule; traceability check before each commit" (reversible technical — decided autonomously: documentation)

## Claude's Discretion

User delegated ("I don't need to answer a bunch of questions", 2026-11-25): unmute control and not remembering it; guests hear audio with no Share Audio switch; re-lock when the last own-account viewer leaves after a remote unlock (about 30 s grace, cancelled by local input); unlock copy and no extra retries; the Unlock field in every viewer; release notes state Apple Silicon and macOS 14+; no user-facing quality setting.

## Deferred Ideas

Persistent-content-capture wiring if not granted in time; dropping the universal-named DMG later; libopus only if M-162 fails; a second encoder tier; HEVC, H.264 High, data-channel cursor; unlock at the login window after reboot or with FileVault.
