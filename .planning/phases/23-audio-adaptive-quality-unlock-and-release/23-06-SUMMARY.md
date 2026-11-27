---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 06
subsystem: ui
tags: [react, webrtc, opus, audio, desktop-viewer, copy]
requires:
  - phase: 20-remote-control
    provides: the control bar, notice slot, Settings Row 1
  - phase: 22-click-to-share-for-tailnet-people
    provides: guest bar, Share Desktop sheet, Admit sheet and their strings
provides:
  - audio recvonly transceiver and a stereo Opus offer (withOpusStereo)
  - caps event (audio, unlock) and state audio failed on the desktop client
  - DesktopSoundButton in every bar, the one-time hint, failure notices, sound-only bar
  - five host strings that say audio reaches everyone watching
affects: [23-08 iPhone bar and app tab indicator, 23-12 TESTING.md, 23-01 daemon wire contract]
plan_head_before: 4004dae81c7840efaa29f5b599bf83ac50afd87f
plan_head_after: e7394cf09c68de290a889aa3b0e7b97dcb085236
actuals:
  tokens: 21000
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns: ["the video element's muted property is the source of truth for sound (no storage)", "audio failure is a daemon state that greys one item and never touches the picture"]
key-files:
  created:
    - frontend/src/components/DesktopSoundButton.tsx
    - frontend/src/components/__tests__/DesktopSoundButton.test.tsx
    - frontend/src/components/__tests__/DesktopView.sound.test.tsx
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/components/DesktopView.tsx
    - frontend/src/style.css
    - frontend/src/lib/shareDesktopCopy.ts
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/lib/__tests__/desktopClient.test.ts
    - frontend/src/lib/__tests__/desktopClient.guest.test.ts
    - frontend/src/components/__tests__/App.desktop-tab.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.share.test.tsx
    - frontend/src/components/__tests__/ShareDesktopSheet.test.tsx
    - frontend/src/components/__tests__/AdmitSheet.test.tsx
key-decisions:
  - "No separate soundOnly flag: a viewer with no control right already hides every other bar item, so rendering the bar for a viewer with audio yields the sound-only bar"
  - "A reconnect where the browser refuses unmuted play() mutes and retries once instead of showing the Tap to start card"
requirements-completed: [DCTL-07]
duration: 40min
completed: 2026-11-27
status: complete
---

# Phase 23 Plan 06: Browser Sound UI and copy updates Summary

Every browser viewer now asks for stereo Opus, starts muted, and unmutes with one click on a Sound item that sits in every bar; the sharer is told, in five strings, that everyone watching hears the Mac.

## What was built

- **desktopClient.ts**: `conn.addTransceiver('audio', { direction: 'recvonly' })` after video and before `createOffer`; the offer goes through `withOpusStereo(sdp)` (adds `stereo=1` to the Opus fmtp, or an fmtp line if none; every other line untouched) before `setLocalDescription`; hello caps `{ video: 1, audio: 1, input: 1 }`; new `caps` event `{ audio, unlock }` from the server hello; the `state` event carries `audio: 'failed'` only for that exact wire value. `onTrack` keeps audio and video in one stream and an audio track alone is never a picture (the client's `streaming` status still waits for the video track).
- **DesktopSoundButton.tsx**: Unmute (`SpeakerXMarkIcon`) and Mute (`SpeakerWaveIcon`), click only. Unmute clears `muted` and calls `play()` in the same handler; the item stays Unmute until `play()` resolves, a rejection mutes again and reports `play-rejected`. Greyed with `aria-disabled`, tooltip and a linked hidden reason while audio failed; a click reports `unavailable`. A hidden `role=status` says Sound on or Sound off. A hidden copy of the word Unmute keeps the width fixed. The element's `muted` is the source of truth, so there is no storage of any kind. `SOUND_HINT_MS`, the hint and notice strings are exported for 23-08.
- **DesktopControlBar.tsx**: optional `sound` prop; the last group is Sound then Full Screen.
- **DesktopView.tsx**: `caps` and the state's audio field; one-time hint (5 s from the first picture, only with audio negotiated and muted, never under a card, not repeated after a reconnect, removed by unmuting, phone wording on a phone); the two failure notices as alerts; a notice once when sound that was on is lost; the bar is also rendered for a viewer with audio and no other right (view-only guest, own-account viewer without control).
- **style.css**: `.desktop-sound*` (fixed-width label, 44px phone form), declared before the 880px rule so compact still hides the label.
- **Host strings** (UI-SPEC Surface 5, verbatim). Where each was found:
  - Settings Row 1 description and the empty-state body: the same single constant, `DESKTOP_OFF_DESCRIPTION` in `frontend/src/components/SettingsSheet.tsx`.
  - Share Desktop sheet not-sharing description, Settings Row 6 description, footnote third sentence, Admit paragraph 1: `SHARE_NOT_SHARING_DESCRIPTION`, `SETTINGS_ROW_DESCRIPTION`, `SHARE_FOOTNOTE`, `ADMIT_PARAGRAPH_1` in `frontend/src/lib/shareDesktopCopy.ts`.
  - No Share Audio switch (the Share Desktop sheet test asserts there is no switch).

## Precondition record

All eight prior SUMMARYs exist (20-07, 20-08, 20-10, 21-08, 21-10, 22-07, 22-10, 22-11). `git log --oneline 1c55b159..HEAD -- desktopClient.ts DesktopView.tsx DesktopControlBar.tsx style.css` at base listed 23 commits, newest first: 4004dae8, b40367f3, 7b60fe5f, f82c4c87, 98c47df9, 64cdb045, d23310b1, 7ce37551, ad8ca062, 15a44f6e, 48f23917, 951908f4, b41055f0, bb55eae6, c292ab30, 6ca45021, e9e53a29, 9f4428be, 2e9f88c4, 3c22c9f7, 299d842f, 0b4978a1, f97d700e (Phase 20 to 22 viewer work; nothing from Phase 23 yet).

## Tests

New and edited test files for 23-12 (TESTING.md untouched here):

- New: `frontend/src/components/__tests__/DesktopSoundButton.test.tsx` (12 cases), `frontend/src/components/__tests__/DesktopView.sound.test.tsx` (20 cases).
- Edited (new audio cases): `frontend/src/lib/__tests__/desktopClient.test.ts` (new `audio (Phase 23-06)` block: withOpusStereo, transceiver order, offer stereo, caps, state audio failed, audio-only-track-is-not-a-picture).
- Edited (assertions for the audio transceiver and hello caps): `frontend/src/lib/__tests__/desktopClient.guest.test.ts`, `frontend/src/components/__tests__/App.desktop-tab.test.tsx`.
- Edited (string assertions): `SettingsSheet.desktop.test.tsx`, `ShareDesktopSheet.test.tsx`, `AdmitSheet.test.tsx`, and `SettingsSheet.share.test.tsx` (outside the plan's list; it asserts Row 6 and now pins the sound wording).

Gates: `tsc --noEmit` clean; full frontend vitest run 149 files, 2719 tests passed; `bash tests/check-brand.sh` clean. All acceptance greps meet their targets (the old Share Desktop description count is 0; the five new strings are 1, 1, 1, 1 and 2 including the Row 1 test literal).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Existing assertions on the video-only offer and the hello**
- **Found during:** Task 1
- **Issue:** `desktopClient.test.ts`, `desktopClient.guest.test.ts` and `App.desktop-tab.test.tsx` pinned `caps: { video: 1, input: 1 }`, exactly one transceiver, and the guest call order, which the plan's own change makes false.
- **Fix:** updated to the audio caps and two transceivers. Files are outside the plan's list; named here.
- **Commit:** 6bc11b3f

**2. [Rule 2 - Missing critical] Reconnect with sound on and a browser that refuses unmuted play()**
- **Found during:** Task 1
- **Issue:** `startPlay` treated every `play()` rejection as the Tap to start card, so a reconnect after Unmute on a strict browser would cover the picture with a card.
- **Fix:** when the element is unmuted, a rejection mutes it, shows the existing play-rejected alert, and retries once; the card appears only if a muted `play()` is also refused. Tested.
- **Commit:** 6bc11b3f

**3. [Plan structure] Task 2 behavior landed with Task 1**
- The hint, failures, sound-only bar, accessible status line and the `.desktop-sound*` CSS were small and already needed by the tracer's bar item (without the CSS the fixed-width label would render both words), so commit 6bc11b3f carries them with their view-level tests. Task 2's own commit (843c116f) is the 12-case component test file. Tests for the Task 2 behaviors exist at both levels.

**4. [Plan wording] Offer and hello assertions live in the client test**
- The plan's Task 1 action 4 asks `DesktopView.sound.test.tsx` to check the offer SDP and the hello. In that file the client is mocked, so those two assertions are made in `desktopClient.test.ts` against the real client with the fake peer connection and transport; `DesktopView.sound.test.tsx` covers the view.

No architectural changes.

## Scope choices worth knowing

- **Phone**: no Sound item is rendered in the page bar on a phone; the iPhone bottom-bar item and the app tab's playing indicator are 23-08 (the plan says so). The phone-form props (`phone`, `compact`) and the 44px class are in place for it. Until 23-08 lands, the phone hint ("Sound is off. Tap Unmute to hear this Mac.") names an item the phone bar does not yet have.
- **App tab**: the Sound item shows in the existing Desktop-tab bar (group 4 holds Sound alone there, since the app has no Full Screen). `DesktopView` has no `onSoundChange` prop for the tab indicator; 23-08 adds what it needs.
- **Wire**: names match 23-01's contract (`caps.audio`, `caps.unlock`, `state.audio: "failed"`). The daemon was not built here; nothing was run against it.

## Real-Mac and VM items (not run: no live daemon, no browser UAT on this host)

For 23-12's manual checklist and 23-09/23-10:

1. Real Chrome and Safari: the stereo offer actually yields a stereo Opus answer and both channels are audible (the SDP shape is unit-tested only against a Chrome-style fixture).
2. iPhone Safari and Home Screen: the click-versus-pointerdown assumption for the unmute gesture ([ASSUMED] in UI-SPEC) and the post-sleep second tap.
3. A reconnect with sound on in Safari: confirm the mute-and-retry path (deviation 2) in practice, and that nothing else regresses the picture.
4. Sound item in light and dark, compact (under 880 px) and full widths: fixed width, focus ring, the hint timing next to the bar's first-frame reveal.
5. Guest page in a real browser: a view-only guest sees the sound-only bar and can unmute; the Admit sheet and Share Desktop sheet read correctly.
6. Sound on the office Mac's real output device (M-162 in the phase): lip sync and the Mute/Unmute controls while it plays.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path, file access or schema. T-23-17 (remembered unmute) is mitigated and tested (storage and cookie unchanged, remount starts muted); T-23-18 (sharer unaware guests hear the Mac) is mitigated by the five strings including the Admit sheet.

## Self-Check: PASSED

- FOUND: frontend/src/components/DesktopSoundButton.tsx, DesktopSoundButton.test.tsx, DesktopView.sound.test.tsx
- FOUND commits: 6bc11b3f, 843c116f, e7394cf0 (ancestors of HEAD)
