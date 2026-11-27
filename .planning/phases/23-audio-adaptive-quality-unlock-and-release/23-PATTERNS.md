# Phase 23: Audio, Adaptive Quality, Unlock and Release - Pattern Map

**Mapped:** 2026-11-26
**Files analyzed:** 27 (new or modified)
**Analogs found:** 25 / 27 (2 have only a partial analog)
**Scope note:** REL-01 moved to Phase 18.1 (complete, merged: scripts/sign-bundle.sh, scripts/verify-bundle.sh, release.yml two-DMG loop, build.yml arm64 context all EXIST). Phase 23 release work is only: v2.0.0-rc.1 dry run, the real v2.0 release, release-notes line, and TESTING.md rows. All analog paths below were checked as git-tracked.

## Upstream Interface Status (merged code vs plan only)

Verified against the working tree on branch gsd/phase-17-desktop-feasibility-spike, 2026-11-26.

| Interface Phase 23 needs | Status | Evidence |
|---|---|---|
| pion API and interceptor registry in `internal/desktop` | EXISTS, but only `RegisterDefaultInterceptors`. No TWCC sender config, no `pkg/cc`, no `pkg/gcc`, no audio codec | `internal/desktop/peer.go:78-96` `newAPI` registers one H.264 codec (PT 102) and `webrtc.RegisterDefaultInterceptors(me, ir)`. GCC/TWCC/pacer wiring is a Phase 23 edit to this function |
| pion/interceptor in go.mod | EXISTS | `go.mod:17` `github.com/pion/interceptor v0.1.51`, `:20` `webrtc/v4 v4.2.29`, `:18-19` rtcp, rtp (direct deps). No new module needed. Only `internal/desktop/peer.go` imports interceptor |
| Video track type | EXISTS: `TrackLocalStaticRTP` per viewer, fed by one shared packetizer | `peer.go:~118` `webrtc.NewTrackLocalStaticRTP(h264Capability, "video", "portty-desktop")`; `manager.go:~924` `writeUnit(au.Keyframe, packetize(pkt, au))`. Audio track must be a second `TrackLocalStaticRTP` added in `answer()` |
| Encoder seams: keyframe request (coalesced), force keyframe | EXISTS | `manager.go:861-899` `requestKeyframe`, `fireKeyframe`, `forceKeyframeNow`; `screencap/media_darwin.go:327-329` `DisplaySource.ForceKeyframe` re-encodes cached frame as IDR. Called from PLI/FIR at `peer.go:134-146` |
| Encoder bitrate setter, cached-frame refine, fps/resolution update | NOT EXISTING. `pc_stream_start(... FPS, MaxLongEdge, BitrateBps, KeyframeInterval ...)` takes fixed config at start (`media_darwin.go:264-281`). No `SetBitrate`, no `updateConfiguration` wrapper. Phase 23 adds them | `media_darwin.go:264-281` |
| Phase 20 control data channel (`input`, ordered, reliable) | EXISTS, text-JSON only | `internal/desktop/input.go:283-327` `acceptDataChannel`: label `input`, ordered, no retransmit limits; `OnMessage` rejects non-string (`!dm.IsString` -> `countInput("malformed")`). The binary unlock message needs a new branch here (Open decision for planner: binary vs JSON, see Pattern U-1) |
| Phase 20 hello with caps and rights map | EXISTS | `manager.go:118-119` `Rights map[string]bool`, `:588-592` own-account gets `RightsOwnAccount`; `viewer.go:10-45` `Rights` bit set (`RightsControl`, `RightsClipboard`, `RightsSurface`). There is NO `unlock` capability yet: UI-SPEC says the hello carries `unlock`; Phase 23 adds it additively |
| Lock gate | EXISTS | `internal/desktop/gates.go:14-21` `inputLocked()` (optimistic `lockUntil` OR `Locked()`); `state.go:113-135` `Locked()` fail-closed cache; `screencap/sys_darwin.go:47` `SessionLocked()` via `pc_session_locked` (`sys_darwin.m:26` reads `CGSSessionScreenIsLocked`); `input.go:405-420` releases held keys when locked |
| Lock gate drops ALL viewer input incl. pointer (Phase 20) | EXISTS and conflicts with CONTEXT U-2 | CONTEXT U-2 (own-account pointer passes while locked) vs 20-CONTEXT line 24 (all pointer and key input dropped) and Research Open Question 1 recommending drop. The later "After research" bullet in 23-CONTEXT (line 64) resolves it: clicks blocked too. Keep Phase 20 behavior; unlock bypasses the gate internally |
| Encoder reconfigure for display switch (resolution ladder reuse) | PARTIAL. Switch machinery exists (`surface.go:247-275` `beginSwitch/endSwitch`, `handleSelect`), but it restarts the source; no in-stream `updateConfiguration` | `internal/desktop/surface.go:247-275`. Resolution step (A-ladder) needs new code |
| CGEvent key posting at HID tap | EXISTS | `screencap/input_darwin.go:74-82` `InputBridge.PostKey` (refuses under `testing.Testing()`), `input_darwin.m` `pc_input_key`; `desktop/input.go:59-66` `InputBridge` interface |
| Layout reverse map (character -> key code) | PARTIAL. `screencap/keytable.go` is a static code-to-VK table for viewer keys, not a UCKeyTranslate reverse map | `internal/screencap/keytable.go` |
| IOPM user-activity declare | EXISTS | `screencap/sys_darwin.go` declares `pc_declare_remote_activity(const char *name)`; reuse for the wake step |
| Phase 21 shared DesktopView / desktopClient | EXISTS (21-01..07 merged) | `frontend/src/components/DesktopView.tsx` (`<video muted>` at :511, `setAttribute('muted','')` :342-343, `locked` :519, `releaseAll('lock')` :538, chip `Locked` :727); `frontend/src/lib/desktopClient.ts` (`addTransceiver('video', recvonly)` :656, `createDataChannel('input')` :600, `ontrack` :648); `DesktopControlBar.tsx` (bar groups, `REVEAL_FIRST_MS`). No audio transceiver and no `stereo=1` munge yet |
| Phase 21 app Desktop tab, iPhone viewer, bottom bar, key bar | ONLY PLANNED | 21-08-PLAN (Desktop tab, wave 4), 21-10-PLAN (iPhone trackpad, soft keyboard, key bar, wave 5), 21-09 (Cmd forwarding). The iPhone Sound item, tab speaker indicator, and empty-slot-on-Unlock-focus all attach to these unbuilt surfaces. 21-08..13 unmerged |
| Phase 20 remaining: VM proof, host-side strings, TESTING.md | ONLY PLANNED | 20-09 (VM proof), 20-10 (Settings/tray strings that Surface 5 of UI-SPEC rewrites), 20-11 (TESTING.md). The Lock This Mac dialog body text (UI-SPEC Surface 3) belongs to merged 20 frontend code: confirm it exists via `grep -rn "Remote unlock isn't available yet" frontend/src` at plan time |
| Phase 22 grant kinds (own-account vs guest) | PARTIAL. Own-account `RightsOwnAccount` EXISTS (`viewer.go`). Guest rights/admit/grants ONLY PLANNED | 22-04-PLAN (GuestShare, admission, guest signalling), 22-05-PLAN (live per-person control and clipboard grants on the press gate). Unlock grant check can be written now against `v.Rights == RightsOwnAccount`, plus a new explicit `Unlock` capability; guests never get it |
| Phase 22 audit log | ONLY PLANNED | `internal/desktop/audit.go` does not exist. 22-02-PLAN must_have: `OpenAudit(dir)`, `desktop-audit.jsonl` O_APPEND 0600, one `Record` call = one Write, closed list of twelve event names, `reason` only a fixed constant, never errors or blocks. Phase 23 adds an `unlock` event (and `unlock_failed` or a result field). Event list is a closed constant set, so this is a Phase 22 edit as built |
| Phase 22 arbiter and physical-input signal (D-relock) | ONLY PLANNED | 22-03-PLAN: `inputtap_darwin.m` listen-only tap, `PhysicalSource`, `arbiter.go` (`PhysicalState`, `Seen`, `Idle`). Not in tree. D-relock "someone used the Mac since unlock" depends on `screencap.PhysicalState`. Fail secure per research: if absent, re-lock anyway |
| `internal/desktop/audio.go`, `quality.go`, `unlock.go`, `pacer.go`, `stats.go`, `screencap/audio_darwin.m`, `unlock_darwin.m` | NOT EXISTING (Phase 23 creates) | `ls internal/desktop internal/screencap` |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/screencap/audio_darwin.m` (+ `audio_darwin.go`) | native capture + encoder | streaming | `internal/screencap/media_darwin.m` + `media_darwin.go`; reference `spike/desktop/media_darwin.m:470+` | exact (same SCStream, same cgo shape) |
| `internal/desktop/audio.go` | service (track + RTP) | streaming | `internal/desktop/peer.go` (track setup) + `manager.go:924` (`writeUnit`/fan-out) | role-match |
| `internal/desktop/peer.go` (modify `newAPI`, `answer`) | config/service | request-response | itself, `peer.go:78-135` | exact |
| `internal/desktop/pacer.go` | utility | streaming | `pion/interceptor@v0.1.51/pkg/gcc/noop_pacer.go` (module cache) | partial |
| `internal/desktop/quality.go` | service (pure policy) | event-driven | `internal/desktop/watchdog.go` (injected clock, state machine) and `manager.go:861-899` (coalesce) | role-match |
| `internal/screencap/media_darwin.m/.go` (modify: SetBitrate, refine, updateConfiguration) | native encoder | streaming | itself `media_darwin.go:264-281, 327-329` | exact |
| `internal/desktop/unlock.go` | service | request-response (one-shot, sequenced) | `internal/desktop/input.go` (`handleKey`, `gate`, `runInput`) and `gates.go` | role-match |
| `internal/screencap/unlock_darwin.m` (+ `.go`) | native input | event-driven | `internal/screencap/input_darwin.m/.go` | exact |
| `internal/desktop/input.go` (modify `acceptDataChannel`, lock exception) | controller | request-response | itself, `input.go:283-327, 507-530` | exact |
| `internal/desktop/manager.go` (hello `unlock` cap, re-lock hook) | controller | request-response | itself, `manager.go:118, 588-592` | exact |
| `internal/desktop/stats.go` | utility | request-response | `Manager.InputCounters()` `input.go:270` | role-match |
| `frontend/src/components/DesktopSoundButton.tsx` | component | event-driven | `frontend/src/components/DesktopControlBar.tsx` (bar item) | role-match |
| `frontend/src/components/DesktopUnlockPanel.tsx` | component | request-response | `frontend/src/components/DesktopClipboardPopover.tsx` (popover, textarea field) | role-match |
| `frontend/src/lib/unlockSecret.ts` | utility | transform | `frontend/src/lib/desktopInput.ts` | partial |
| `frontend/src/lib/desktopClient.ts` (modify: audio transceiver, stereo munge, unlock send) | service | request-response | itself, `:600-657` | exact |
| `frontend/src/components/DesktopView.tsx` (modify) | component | event-driven | itself, `:342, 511-540, 727` | exact |
| `frontend/src/components/DesktopControlBar.tsx` (modify: group 4 Sound) | component | event-driven | itself | exact |
| `frontend/src/style.css` (`.desktop-sound*`, `.desktop-unlock*`, `.tab__sound`) | config | n/a | `.desktop-chip*`, `.desktop-card*`, `.button*` blocks in the same file | exact |
| `internal/desktop/audio_test.go`, `quality_test.go`, `unlock_test.go`, `pacer_test.go` | test | mixed | `internal/desktop/input_test.go`, `fakes_test.go`, `loopback_test.go`, `watchdog_test.go` | exact |
| `internal/desktop/unlock_redact_test.go` (sentinel password grep) | test | transform | none (see No Analog) | none |
| `internal/desktop/loopback_shaped_test.go` (userspace UDP shaper) | test | streaming | `internal/desktop/loopback_test.go` | role-match |
| `frontend/src/**/__tests__/DesktopUnlockPanel.test.tsx`, `DesktopSoundButton.test.tsx`, `unlockSecret.test.ts` | test | event-driven | existing vitest files, e.g. `frontend/src/components/__tests__/` | role-match |
| `scripts/desktop-soak.sh` | utility script | batch | `.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh` | role-match |
| `internal/updater/release_workflow_test.go` (modify only if 18.1 left a gap; otherwise none) | test | transform | itself, `:68-102, :100` already pins `DMGAssetNames` | exact |
| `TESTING.md` (Sections 2, 4, 5 new Category BK or later, plus Section 3 already updated by 18.1) | doc | n/a | Category BJ `TESTING.md:2154+`, Category BI.1 `:2139` | exact |
| `.github/workflows/release.yml` (release notes body only) | config | batch | itself, publish step | exact |

## Pattern Assignments

### `internal/desktop/peer.go` (modify) - adding audio, TWCC, GCC (config, request-response)

**Analog:** itself, `internal/desktop/peer.go:78-135`. Current code (read this session):

```go
func newAPI(mux ice.UDPMux, bind netip.Addr) (*webrtc.API, error) {
	se, _ := newSettingEngine(mux, bind)
	me := &webrtc.MediaEngine{}
	if err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: h264Capability,
		PayloadType:        102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("desktop: register h264: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return nil, fmt.Errorf("desktop: interceptors: %w", err)
	}
	return webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me), webrtc.WithInterceptorRegistry(ir)), nil
}
```

Phase 23 extends in place: register an Opus `RTPCodecParameters` (48000/2, `stereo=1;useinbandfec=1`, `RTPCodecTypeAudio`), add the cc/gcc interceptor and `ConfigureTWCCHeaderExtensionSender` before `RegisterDefaultInterceptors` (Research Pattern 3). Error wrapping style: `fmt.Errorf("desktop: <what>: %w", err)`.

**Track + sender pattern to copy for the audio track** (`peer.go:~110-135`):
```go
track, err := webrtc.NewTrackLocalStaticRTP(h264Capability, "video", "portty-desktop")
if err != nil { return fail(fmt.Errorf("desktop: viewer track: %w", err)) }
sender, err := pc.AddTrack(track)
if err != nil { return fail(fmt.Errorf("desktop: add track: %w", err)) }
v.track = track
```
Use the same stream id (`"portty-desktop"`) for the audio track so the browser puts both in one MediaStream (CONTEXT: lip-sync via one stream).

### `internal/desktop/input.go` (modify) - unlock message and lock exception (controller, request-response)

**Analog:** itself, `internal/desktop/input.go:283-327` (data channel accept) and `:507-530` (gate).

**Message intake to extend** (:305-324): `OnMessage` runs inline on the SCTP read loop, must not block, and must `defer recoverLog()`:
```go
dc.OnMessage(func(dm webrtc.DataChannelMessage) {
	defer recoverLog()
	if !dm.IsString {
		m.countInput("malformed")
		return
	}
	msg, err := parseInput(dm.Data)
	...
	m.enqueueInput(vw, msg)
})
```
A binary unlock message (type byte + UTF-8 bytes, per Research Pattern 6) goes in the `!dm.IsString` branch. Hand the slice to the unlock goroutine without copying into a string, zero `dm.Data` after use, and count `unlock_*` via `m.countInput`. Do NOT route it through `parseInput`/`inputMsg` (JSON structs and logs would hold it).

**Gate to leave intact** (`gates.go:14-21`): every ordinary key and pointer event is already dropped while `inputLocked()`. The unlock typer must call the screencap bridge directly (not `handleKey`), so the lock gate needs no exception for ordinary input. Reuse `m.releaseAllViewers("lock")` (`input.go:885`) when the unlock attempt starts (CONTEXT: no-stuck-keys rule also runs when the drop starts).

### `internal/desktop/unlock.go` (service, request-response one-shot)

**Analog:** `internal/desktop/input.go` structure (single input goroutine owns the bridge; injected `Deps` for testing) and `internal/desktop/gates.go`.

**Seam pattern** (`input.go:59-66`): define a narrow interface so tests inject a fake, and the real one is `screencap.*`:
```go
type InputBridge interface {
	PostKey(vk uint16, down bool, flags uint64, autorepeat bool) error
	...
}
```
Copy this: `type UnlockBridge interface { MapRunes([]rune) ([]KeyStroke, error); Wake() error; TypeKey(...) error; SessionLocked() (bool, error); CapsLockOn() bool }` plus an injected `Now`/`Sleep` via `m.d` (see `m.d.Now()` use at `gates.go:20`, `state.go:114`) so settle time, 3 s poll and 5 s throttle are unit-testable without real sleeps.

**Lock read to reuse**: `Manager.Locked()` `state.go:113-135` is cached and fail-closed (true on error). For the post-Return check use a fresh `screencap.SessionLocked()` (uncached); do not use the cached value or the 3 s check will read stale.

**Authorization**: check grant kind server-side before reading the buffer. Today own-account = `v.Rights == RightsOwnAccount` (`viewer.go`, `manager.go:588`). Prefer a new explicit `RightsUnlock` bit or a boolean on the hello `caps` map, set only on the same-user route, so Phase 22 guests (which start at `RightsViewOnly`, `viewer.go:16-24`) can never gain it by accumulating control bits.

**Error handling pattern**: errors from the bridge are logged with `slog` using fixed reason strings only. Copy `slog.Debug("desktop: keyframe", "reason", reason)` (`manager.go:~878`) as the logging shape. Never pass the password, a length, a rune, or a key code to `slog`, `fmt.Errorf`, or `Record`. Wrap the buffer in a type with `String()`/`GoString()`/`Format` returning `[redacted]` so a stray `%v` is safe.

**Zeroing**: `defer zero(buf)` placed immediately after the buffer is obtained, so it also runs on panic (the recover pattern `defer recoverLog()` is already used on every callback).

### `internal/screencap/unlock_darwin.m` + `unlock_darwin.go` (native input, event-driven)

**Analog:** `internal/screencap/input_darwin.go:74-97` and `input_darwin.m`.

**Test-safety pattern to copy exactly** (`input_darwin.go:74-82`): every method that touches the real OS refuses under tests.
```go
func (*InputBridge) PostKey(vk uint16, down bool, flags uint64, autorepeat bool) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if rc := C.pc_input_key(C.uint16_t(vk), b2i(down), C.uint64_t(flags), b2i(autorepeat)); rc != 0 {
		return inputErr("posting a key event", rc)
	}
	return nil
}
```
Typing the password must run only in the VMs (public password) or on the office Mac with the user present (CONTEXT specifics, research rules), so `ErrRefusedInTest` is mandatory on every unlock native call.

**Lock read** (`sys_darwin.go:47`, `sys_darwin.m:26` `CGSSessionScreenIsLocked` via `readFlag`): copy for a `CapsLockOn()` read (`CGEventSourceFlagsState`). Wake step: reuse `pc_declare_remote_activity(name)` (declared in `sys_darwin.go` cgo preamble).

**Key tag**: posted events carry `screencap.EventMarker` (Phase 22-03 uses it to tell Portty from physical input). Typed unlock keys must carry the same marker.

### `internal/screencap/audio_darwin.m` (+ `audio_darwin.go`) (native capture, streaming)

**Analog (shape):** `internal/screencap/media_darwin.m` / `media_darwin.go` (the SCStream owner, `pc_stream_start`, `DisplaySource`, AccessUnit callback at `media_darwin.go:119`). **Analog (recipe, reference only, never imported):** `spike/desktop/media_darwin.m:470+, 923-925`:

```objc
cfg.capturesAudio = YES;
cfg.sampleRate = kOpusRate;      // 48000
cfg.channelCount = 2;
enum { kOpusFrames = 960, kOpusRate = 48000, kOpusBitrate = 96000, kOpusMaxPacket = 4096 };
```
Go side: mirror `media_darwin.go:119` `s.au(AccessUnit{...})` with an `AudioPacket{Data []byte, PTS time.Duration}` callback; mirror the `nativeMu`/`mu` serialization note at `media_darwin.go:173`. Set `excludesCurrentProcessAudio = YES` and bitrate 64000 (assumption A1: verify in a VM probe, fall back to 96000). Audio failure becomes a Manager state (like `noteReason("lock", err)` at `state.go:~125`), never an error that stops the video source.

### `internal/desktop/audio.go` (service, streaming)

**Analog:** `manager.go:~900-930` fan-out (`writeUnit(au.Keyframe, packetize(pkt, au))`) and `peer.go` per-viewer track. Copy: one shared packetizer, per-viewer `TrackLocalStaticRTP`, a viewer receives audio from the first packet after join (no keyframe gating needed for Opus). Timestamps from SCK PTS (Research Pitfall 9: use `TrackLocalStaticRTP` with an `rtp.Packetizer`, not `TrackLocalStaticSample`; the existing video track is already `StaticRTP`, so this matches).

### `internal/desktop/quality.go` and `pacer.go` (policy, event-driven)

**Analog:** `internal/desktop/watchdog.go` plus `watchdog_test.go` (injected clock, pure state transition, fake inputs) and `manager.go:861-899` (time-coalesced action using `m.d.Now()`).

**Coalesce shape to copy for hysteresis/hold timers:**
```go
now := m.d.Now()
if since := now.Sub(m.kfLast); !m.kfLast.IsZero() && since < keyframeCoalesce {
	m.kfPending = true
	m.kfTimer = time.AfterFunc(keyframeCoalesce-since, m.fireKeyframe)
	...
}
```
Keep the policy a pure function over `(estimates, current step, now)` so unit tests need no pion or VideoToolbox (Research Pattern 4). Pacer: copy the interface only from the module cache (`pkg/gcc/send_side_bwe.go:31-36`: `interceptor.RTPWriter` + `AddStream` + `SetTargetBitrate` + `Close`); use `gcc.NewNoOpPacer()` first, bound it only if the shaped-link test shows a leak (Pitfall 3).

### `internal/desktop/*_test.go` (tests)

**Analogs:** `input_test.go`, `fakes_test.go`, `export_test.go`, `loopback_test.go`, `watchdog_test.go`, `power_test.go`.
Copy: `fakes_test.go` fake `Deps` (Now, Lock, source), `export_test.go` for test-only exports, `peer.go:41` `AllowLoopbackForTest()` for loopback integration. Test names follow `TestXxx_Case` and are counted by `go test -list` in plan verify blocks (see 22-02 verify block: `test "$(go test -list 'TestAudit_|TestNames_' ./internal/desktop/ | grep -cE ...)" -ge 6`). Use `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/desktop/` as the verify command.

### `frontend/src/components/DesktopSoundButton.tsx` (component, event-driven)

**Analog:** `frontend/src/components/DesktopControlBar.tsx` (bar item metrics, `REVEAL_FIRST_MS`, heroicons import style):
```tsx
import React, { useEffect, useRef, useState } from 'react'
import { ArrowsPointingInIcon, ... } from '@heroicons/react/24/outline'
export const SWITCH_TIMEOUT_MS = 5000
export const REVEAL_FIRST_MS = 5000
```
Use `SpeakerWaveIcon`, `SpeakerXMarkIcon` from `@heroicons/react/24/outline`. Fire on `click` only (UI-SPEC; differs from Phase 14/21 pointerdown rule). Unmute pattern: Research Pattern 2 (`video.muted=false; video.play().catch(() => mute again)`). `DesktopView.tsx:342-343` already sets the `muted` attribute for Safari and `:511` renders `<video muted>`; the button receives the video ref, no storage.

### `frontend/src/components/DesktopUnlockPanel.tsx` (component, request-response)

**Analog:** `frontend/src/components/DesktopClipboardPopover.tsx` (popover panel with textarea; look and metrics per UI-SPEC) and DesktopView.tsx's overlay-layer children. Differences to enforce: uncontrolled input via ref, not in a `<form>`, `type="password"`, password-manager opt-outs, clear on submit/hide/unmount/Esc, sibling of (never child of) the key-listening stage element. Wiring: `DesktopView.tsx:519` `const locked = desk?.locked === true || desk?.state === 'locked'` and `:727` the Locked chip are the render gates; the panel additionally requires the hello `unlock` capability.

### `frontend/src/lib/desktopClient.ts` (modify)

**Analog:** itself `:600-657`.
```ts
const ch = conn.createDataChannel('input', { ordered: true })        // :600
...
if ((m.caps as Record<string, unknown> | undefined)?.input === 1) openInputChannel(g, conn)
conn.addTransceiver('video', { direction: 'recvonly' })               // :656
await conn.setLocalDescription(await conn.createOffer())
```
Add `conn.addTransceiver('audio', { direction: 'recvonly' })` immediately after the video one (before `createOffer`), and munge `stereo=1` into the offer's Opus fmtp before `setLocalDescription`. Add `sendUnlock(bytes: Uint8Array)` using `ch.send(bytes)` (binary) and zero after send. Mirror the `rights` emit pattern (`:652-654`) to emit `unlock` capability.

### `scripts/desktop-soak.sh` (utility script, batch)

**Analog:** `.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh` (host builds and tar-streams, every launchctl/kill/rm runs inside `portty-test` via the ssh helper, `state ROW` writes evidence to `evidence/` with home paths shown as `~` and no tailnet addresses or tokens). Copy: evidence file naming, path redaction, "never contact host tailscaled or 100.x" rule. Sampling tools: `footprint`, `vmmap -summary`, `lsof`, `ps` (present on host).

### Release workflow analogs (what Phase 23 consumes, already built by 18.1)

**`.github/workflows/release.yml`** (exists): `DMG_SUFFIXES: "darwin-universal darwin-arm64"` (line 13); inside-out sign `scripts/sign-bundle.sh` + `verify-bundle.sh --signed` (:227-230); notarize/staple per DMG loop (:271-283); updater-parity verify loop (:286-302) with `TeamIdentifier=S2K7P43927`, `Identifier=${NAME}`, `source=Notarized Developer ID`. Phase 23 only adds a release-notes body (Research Open Question 8: `body_path` in the publish step with the line "Apple Silicon and macOS 14 or later only") and runs the rc.1 dry run. Do not change signing or names (one-way decisions).

**`internal/updater/update.go:77-90`**: `DMGAssetName(v)` (universal) and `DMGAssetNames(v)` (universal first, arm64 second) EXIST; `install.go:221-232` `Clean` loops `DMGAssetNames("*")`; `release_workflow_test.go:100` pins `DMGAssetNames("1.2.3")` against the workflow. Extend only if the rc test finds a gap.

**`scripts/verify-bundle.sh`** exists (flags `--signed`, `--notarized`; `plist_get` helper, `fail()`), `tests/verify-bundle.test.sh` is its test, `scripts/sign-bundle.sh` has `BUNDLE_ID="app.portty.Portty"` guard and `ENTITLEMENTS` default `build/entitlements.plist`. No new script needed for REL-01.

**Gates to run before each commit:** `bash tests/check-brand.sh` (header documents: old product name, scottkw links, package-manager mention, vm-only tags in .github/scripts, `/Users/<name>/` developer paths anywhere including `.planning/`, real tailnet 100.64.0.0/10 addresses in `.planning/` and TESTING.md, webservertest import by non-test files) and `bash tests/check-traceability-paths.sh` (greps `| path.(go|ts|tsx|sh) |` cells in TESTING.md; Section 4 path column must be repo-relative file path only). release.yml's first job runs `check-brand.sh` and `verify-bundle.test.sh` (:29-35).

### `TESTING.md` (doc)

**Analog:** Category BJ (`TESTING.md:2154+`) and BI.1 (`:2139`). Section 2 suite manifest at `:22`; Section 3 merge gate `:331` (build.yml context already `build (portty, darwin/arm64, macos-latest)`; build.yml:9-11 comment confirms); Section 4 traceability `:388` (columns: Requirement | Test File | Suite Group | Notes; add rows for DCTL-07, DESK-07, UNLK-01); Section 5 manual checklist `:986`, last category BJ with M-177..M-179 (Phase 20 and 21 plans 20-11 and 21-13 will reserve later numbers: re-read the highest M-NN at plan time); Section 6 standing convention `:2221`. Row format to copy:
```
- **M-177** An upgrade on the office Mac with a desktop viewer attached and live terminal sessions (DSHR-01, ...; the owner runs it; human_needed; ...).
  1. ...
```
Note ID collision risk: M-162 and M-161 are referenced in 23-CONTEXT as spike rows ("M-162 real output device", "M-161 office Mac encoder at native size") that are already in the matrix; check `.planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md` and TESTING.md Category BH (`:2003`) before assigning new numbers.

## Shared Patterns

### Test-mode refusal for real OS calls
**Source:** `internal/screencap/input_darwin.go:74-97`
**Apply to:** `unlock_darwin.go`, `audio_darwin.go`, any new cgo caller
```go
if testing.Testing() {
	return ErrRefusedInTest
}
```

### Callback safety (never panic the SCTP/capture goroutine)
**Source:** `internal/desktop/input.go:290-304`
**Apply to:** every new pion callback and capture callback
```go
dc.OnOpen(func() {
	defer recoverLog()
	m.sendControl(dc)
})
```

### Fail-closed lock read
**Source:** `internal/desktop/state.go:113-135` `Manager.Locked()` (error -> true) and `gates.go:14-21`
**Apply to:** all input paths. Unlock uses a fresh uncached `screencap.SessionLocked()` only for the post-Return check.

### Injected clock and fakes for time-dependent logic
**Source:** `m.d.Now()` in `gates.go:20`, `state.go:114`, `manager.go:871`; `internal/desktop/fakes_test.go`
**Apply to:** quality policy, unlock throttle (5 s, 3 fails -> 15 min), settle time, 3 s poll, re-lock 30 s

### Error wrapping
**Source:** `internal/desktop/peer.go:84-96`
**Apply to:** all new Go code: `fmt.Errorf("desktop: <context>: %w", err)`; errors never contain secrets

### Logging
**Source:** `manager.go:~878` `slog.Debug("desktop: keyframe", "reason", reason)`
**Apply to:** fixed reason strings only; never password, length, key code, rune. Sentinel-grep test over captured slog output, audit file and error strings.

### Frontend icon and bar item conventions
**Source:** `frontend/src/components/DesktopControlBar.tsx:1-20`, `DesktopView.tsx:342-343, 511, 519-540`
**Apply to:** `DesktopSoundButton.tsx`, `DesktopUnlockPanel.tsx` (heroicons `24/outline`, no new npm package, no new token)

### TESTING.md standing rule
**Source:** `CLAUDE.md` + `TESTING.md:2221`
**Apply to:** every plan that adds a test file: add to Section 2, add Section 4 row for each v4.0+ requirement, add M-NN in Section 5, run `bash tests/check-traceability-paths.sh`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/desktop/unlock_redact_test.go` | test | transform | No sentinel-secret grep test exists anywhere in the repo (a case-insensitive search for "sentinel" in `internal/**/_test.go` found nothing). Build it from Research Security Domain V7 and CONTEXT Unlock test line |
| Unlock rune-to-key reverse map (`UCKeyTranslate` over `TISCopyCurrentKeyboardLayoutInputSource`) | native util | transform | `internal/screencap/keytable.go` maps viewer key codes to VKs, not characters to layout key codes; assumption A16 unverified. Use Research Standard Stack |
| In-stream encoder bitrate/fps/resolution update (`VTSessionSetProperty`, `SCStream updateConfiguration`) | native encoder | streaming | `media_darwin.m` fixes these at `pc_stream_start`. Research assumptions A7 and A9 are unverified on the target Macs (VMs were unreachable at research time) |
| Shaped-link loopback (userspace UDP shaper) | test | streaming | `loopback_test.go` is unshaped; no shaper exists. Research Pattern 10 |

## Metadata

**Analog search scope:** `internal/desktop`, `internal/screencap`, `internal/updater`, `frontend/src/components`, `frontend/src/lib`, `scripts`, `tests`, `.github/workflows`, `TESTING.md`, `.planning/phases/{18.1,19,20,21,22}`
**Files scanned:** about 45 (targeted reads and greps; large files read by section)
**VM harness:** `.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh` and `19-VM-CHECKS.md` (Tart VMs `portty-test` office role, `portty-test-2` home role; `wails build -tags "wailsassets vmpreview"`, ad-hoc signed; vm-only tags must never reach release builds per `check-brand.sh`)
**Caveat:** Research notes both VMs were unreachable on 2026-11-25; all VM-dependent assumptions (A1, A7, A9, A12, A13, A14, A16) remain unverified until the rig is healthy.
**Pattern extraction date:** 2026-11-26
