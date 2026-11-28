# Phase 23 VM checks: audio, adaptive quality, unlock and upgrade on macOS 27.2

Rig: `.planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/p23-rig.sh` (builds on `p22-rig.sh`, `p20-rig.sh` and `p19-rig.sh`; page scripts in `rig/js/`). Office = portty-test (the shared Mac: Portty GUI and daemon, afplay, the stats route), home = portty-test-2 (the viewer: Safari through safaridriver, Chrome, the Portty app). Every capture, playback, AX call, input post and traffic-shaping call runs inside a VM through ssh; the host only compiles, generates a test tone file, copies and writes scrubbed evidence. Nothing runs Portty, capture, playback, an input tap or traffic shaping on the host, and nothing contacts the host's tailscaled or a 100.x address. Neither VM is stopped. Evidence files carry no real address, host name, account name or token (the rig scrubs before writing; `tests/check-brand.sh` gates every commit).

## Criteria (written before any row of this plan ran)

A row is PASS only when every numbered condition holds. PARTIAL means a named condition could not be observed in the VM and the reason is recorded (the observed conditions held). FAIL means a condition was observed not to hold. NOT-RUN means the row could not be driven at all. Ceiling is the controller's 8000000 bit/s, floor 500000, ladder step 0 is full size and rate, step 3 the smallest.

Rows of this plan (23-09):

- **S-00 tracer (DCTL-07, DESK-07).**
  1. The Phase 23 build (version 23.9.0, tags `wailsassets vmdesktop`) is installed on portty-test and portty-test-2 (bundle version read in each VM) and portty-test's daemon answers /health with that version.
  2. portty-test-2's Safari, through safaridriver, opens the always-available desktop link with `#debug` and reaches status `streaming` with `framesDecoded` rising across two reads at least 2 s apart.
  3. GET /desktop/stats on portty-test's daemon socket answers with exactly the closed key set pinned by `TestDesktopStats_ClosedKeySet` (15 flattened keys), reads `viewers` 1 and `audio` `ok`.
  4. In the first samples after the join (the first 5 s of media) `quality.target_bps` reads the ceiling 8000000 at step 0 (the controller starts at the ceiling, RESEARCH Pitfall 3). What the estimate does afterwards over the relay is recorded and is not a condition.
- **A-01 stereo (DCTL-07; RESEARCH Pitfall 8, A12).**
  1. portty-test-2's Safari viewer has pressed Unmute by a real element click and the video element reports `muted` false and not paused.
  2. A 5 s left-only 440 Hz test tone is played with afplay inside portty-test (the file made on the host scratch directory, never played there).
  3. A WebAudio channel splitter on the received track reads, over the tone's window, a left RMS at least 3 times the right RMS and the left RMS above 0.005.
  4. getStats lists the audio codec `audio/opus` with `channels` 2.
- **A-02 muted start and Unmute (DCTL-07, UI-SPEC Surface 1).**
  1. A freshly opened page reports the video element `muted` true and the accessibility tree (home, Safari) has a button named `Unmute`.
  2. After the Unmute click the tree has a button named `Mute`, the element reports `muted` false and not paused, and the audio `packetsReceived` rises across two reads.
  3. The same sequence in Chrome (home) when Chrome can be scripted; otherwise the row is PARTIAL with the reason.
- **A-03 audio fault (DCTL-07).**
  1. With a viewer streaming, an audio fault induced inside portty-test (coreaudiod restarted with sudo inside the VM) makes the stats route read `audio` `failed` while the viewer's `framesDecoded` keeps rising; PARTIAL with the reason when no fault can be induced or the state does not change.
  2. The stats route reads `audio` `ok` again once audio recovers, or the recovery is recorded as not observed.
- **Q-01 transport-cc (DESK-07; RESEARCH Pitfall 2, A4).**
  1. Safari's own offer carries the transport-wide-cc extension (read from `localDescription` in the page) and the stats route reads `quality.no_feedback` 0 after at least 10 s of streaming to it.
  2. The same for Chrome when Chrome can be scripted; otherwise PARTIAL with the reason.
- **Q-02 ladder down and back up on the running stream (DESK-07; RESEARCH A9).**
  1. The stream runs with one viewer and a light moving screen. A pipe of about 300 kbit/s is added inside portty-test-2 on traffic from portty-test (dummynet through pf anchor `com.apple/p23`): within 90 s the stats route reads a ladder step above the step read just before the pipe, or step 3.
  2. While it moves, the viewer page (same page session, no reload) reads status `streaming` at every 1 s sample, never `reconnecting`, its `frameWidth` takes at least two different values (a resolution step on the live encoder, no new connection), and the accessibility tree never holds the text `Reconnecting`.
  3. After the pipe is removed (`dnctl list` empty), within 180 s the stats route reads a ladder step below the shaped one and `quality.target_bps` of at least 1000000 at some sample.
  4. No dummynet pipe remains in portty-test-2 at the end.
- **Q-03 slow chip, only for the slow viewer (DESK-07, UI-SPEC Surface 3).**
  1. Two viewers are connected: portty-test-2 (shaped to about 200 kbit/s, below the 500000 floor) and an unshaped Safari on portty-test itself.
  2. Within 30 s the stats route reads `quality.slow` 1, the shaped viewer's page text contains `Slow connection`, and the unshaped viewer's page text does not.
  3. After the pipe is removed the shaped viewer's chip clears within 60 s (or the time is recorded) and no pipe remains.
- **Q-04 still (DESK-07, DESK-06).**
  1. After all motion stops (the Terminal stream quit), the stats route reads `still` true within 1 s of the picture becoming still as seen by a 0.25 s poll, or the delay is recorded when longer.
- **T-01 WKWebView sound and the tab mark (DCTL-07, UI-SPEC backstop).**
  1. In the real Portty.app on portty-test-2, the Desktop tab shows the picture, and an Unmute press in the tab's bar turns the item to `Mute`.
  2. The tab entry's accessible name ends `, playing sound` (AX) while sound is on.
  3. Hiding the window past the 2-minute pause clears the mark (the name no longer ends so).

Rows of 23-10 (written here, run there):

- **U-01 tracer (UNLK-01; RESEARCH Pitfall 12, A13).** On the locked portty-test each wake key candidate (Shift, F15, Left Arrow) is posted alone through the VM probe with the display awake; the first candidate that reveals the password field with length 0 and types no character is `unlockWakeKey`; one real unlock then succeeds.
- **U-02.** An own-account viewer on portty-test-2 unlocks portty-test from the Unlock panel with the VM's public admin password; the panel states (Unlocking, success), the unlocked state in stats and the `unlocked` audit line are recorded.
- **U-03.** Ten lock and unlock cycles with the admin password succeed ten times; the number that needed the retry is recorded (R16-B).
- **U-04.** A made-up wrong sentinel submit ends in the failed state with `failures` 1 and exactly two typed tries in loginwindow's log.
- **U-05.** A second submit within 5 s is refused before typing and the panel shows `Try again in N s`; a refused submit is not counted.
- **U-06.** Two more failed sentinels (three failed submits in total) show the paused line and `failures` 3; the pause is waited out or its remaining time recorded, then the admin password unlocks.
- **U-07.** A password holding an emoji is refused before typing with no key event in the log; Caps Lock on is refused before typing (PARTIAL when the VM cannot set Caps Lock).
- **U-08.** A guest page on a locked Mac shows the `Locked` chip text and no Mac password field in its accessibility tree, and the stats unlock state is unchanged.
- **U-09.** While locked, viewer keys and a click change nothing in loginwindow's log and the blocked-input notice shows once.
- **U-10.** After an unlock and the last own-account viewer leaving, the Mac reads unlocked at 25 s and locked at 35 s, and a `relocked` audit line exists.
- **U-11.** After an unlock and the viewer leaving, an untagged stand-in input (`~/ptest/click`) at 10 s keeps the Mac unlocked at 40 s with no `relocked` line.
- **U-12.** After every U row the wrong sentinel appears nowhere in daemon.log, desktop-audit.jsonl or the unified log for the Portty process, and the audit file holds `unlocked` and `unlock_failed` lines with mode own and no length field.
- **P-01.** After a submit in Safari and in Chrome on portty-test-2 the accessibility tree holds no save-password prompt or keychain suggestion.

Rows of 23-11 (written here, run there):

- **H-00 tracer (SC5).** One soak sample of the VM daemon with a viewer connected: pid, RSS, footprint, open files, CPU and the stats fields all read.
- **H-01.** Installing build N+1 over N with a live unmuted desktop viewer and two ticking sessions keeps both shells' pids and history, the tick counter has no missing or repeated number beyond one pause of a few seconds, and the viewer shows Reconnecting then streams again on the same link with no reload and audio back.
- **H-02.** Re-lock armed before the upgrade is still armed after it: closing the own-account viewer after the upgrade locks portty-test 30 s later.
- **H-03.** A one-hour soak alternating 5 minutes of motion and 5 of idle with a viewer connected ends with the same daemon pid, state streaming at every sample, and after a 5-minute warm-up RSS and footprint at most 1.15 times their minutes 5 to 15 median, open files within 5 and goroutines within 10 of their warm-up range.

## Results

Result cells hold exactly one RESULT token. Evidence names are files under `evidence/`.

| Row | Req | Action | Observed | Result | Evidence |
|-----|-----|--------|----------|--------|----------|
| S-00 | DCTL-07, DESK-07 | build, deploy, deploy-home, open-viewer home, stats-log, vstats, pcstats | Both VMs read bundle version 23.9.0 and the office daemon /health says v23.9.0. Safari in portty-test-2 reached `streaming`, framesDecoded 14, 34, 49 across 5 s, then 189 to 216 over 3 s. The stats route answered the closed key set (15 flattened keys; `stats-key` lists them) with viewers 1, own_viewers 1, audio ok. At the join (S-00.stats t=23 to t=25) target_bps read 8000000 at step 0, 30 fps, long edge 2560; one more read later (S-00.txt) had already fallen to 1727622 at step 1 (the relay constrains it, see Notes) | RESULT=PASS | S-00.stats, S-00.txt |
| A-01 | DCTL-07 | Safari unmuted through the bar's Unmute (element click); `levels home 8000` running while `tone-play` ran afplay on a left-only 440 Hz file in portty-test; `pcstats` | element muted false, not paused. During the tone window the WebAudio splitter read left RMS 0.16587 (peak 0.5392) and right RMS 0, peak 0; the no-tone control read 0 on both. getStats lists `audio/opus` with channels 2 and the answer SDP carries `opus/48000/2`. The left-only source arrives left-only: stereo is preserved end to end through capture, Opus, the relay and Safari | RESULT=PASS | A-01.txt |
| A-02 | DCTL-07 | Safari: fresh session, `pcstats`, `ax-safari`, a WebDriver element click (first attempt) then `p20 safari-menu Unmute` (bar revealed first); Chrome: `chrome-open`, `ax-home find` and `ax-home press` | Safari: fresh page `muted` true and the AX tree has `AXButton Unmute`; the first element click failed with "element not interactable" (the bar is hidden until the pointer reaches its zone, as designed) and left everything unchanged; the click through the reveal zone turned the AX button to `Mute`, element `muted` false and not paused, audio packetsReceived 2415 then 2576 (rising). Chrome: AX shows `Unmute` and the one-time hint "Sound is off..."; AXPress turned it to `Mute`. Chrome's element state and packet counts were not read (page scripts could not be run in Chrome's page world, see the Notes), so the Chrome half of condition 2 is unobserved | RESULT=PARTIAL | A-02.txt |
| A-03 | DCTL-07 | `audio-fault` (sudo killall coreaudiod inside portty-test, pid 326 to 32231) with a Safari viewer streaming, `stats-log 30`, `pcstats`, `vstats` | Frames kept rising (framesDecoded 534, 609, 673) but the audio stopped: audio packetsReceived 4554 before, 4584 at +8 s and 4584 at +18 s (SCStream delivered no more audio and raised no error). The stats route read `audio` `ok` on all 30 samples and afterwards: the daemon never flagged the failure because audio failure is only reported from the capture error callback, and a silent stall raises none. Audio came back only after the viewer closed and a new one started a fresh capture run (recv 60 to 118 in 4 s, then about 52 packets a second). Condition 1's `failed` state was not induced | RESULT=PARTIAL | A-03.txt, A-03.stats |
| Q-01 | DESK-07 | `pcstats home` on Safari (offer extension URIs), `stats` after more than 10 s; Chrome opened alone, `stats` after more than 10 s | Safari's own offer lists `http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01` among its extension URIs; the stats route read no_feedback 0 (viewers 1) with target 1243272. Chrome as the only viewer: no_feedback 0 after more than 10 s (the daemon received transport-cc reports from it), page title `st=streaming`. Chrome's own offer SDP was not read (page scripts could not be run in Chrome's page world), so that part is unobserved; the daemon-side fact (feedback arrived) is the stronger one | RESULT=PARTIAL | Q-01.txt |
| Q-02 | DESK-07 | Safari in portty-test-2 streaming a light moving screen; `stats-log 270` and `vlog home 270` running; `shape on 300` at guest epoch 1795823986, `shape off` at 1795824071 | The ladder was already at step 2 when the pipe went on (the relay had pushed it down), reached step 3 by t=25 (quality.slow 1, Slow connection on the page from t=21). On the live stream the picture width took 2560, 1920 and 1280 (resolution steps through updateConfiguration, 9, 21 and 240 samples). Over all 270 samples the page status stayed `streaming`, the page text never held Reconnecting (0 of 270; AX tree at the end 0 lines) and the load origin was constant (one page, no reload). After the pipe was removed the stats read step 2 (1920) at t=131 and target_bps up to 1726131, quality.slow back to 0 at t=115; the relay then pulled it back to step 3 (as it does with no pipe). `dnctl list` empty, anchor holds 0 rules, pf disabled again | RESULT=PASS | Q-02.txt, Q-02.stats, Q-02.viewer-home, Q-02.shape.txt |
| Q-03 | DESK-07 | Two viewers: Safari in portty-test-2 (shaped, `shape on 200`) and portty-test's own Safari (loopback, never shaped); `vlog home`, `vlog office`, `stats-log`; run twice, plus a control with no pipe | Run a: the unshaped office page ALSO showed Slow connection, from 8 s after the pipe was added, 59 of 85 samples, and quality.slow read 2 for 58 samples: condition 2 did not hold. Run b (same steps): quality.slow stayed 1, the office page never showed the chip (0 of 80), the home page showed it. Control (no pipe): quality.slow 1 for 65 of 70 samples, so the relay viewer is slow by itself in this VM, and the target climbed to 3363736 at step 0 driven by the loopback viewer. So the chip reached the unshaped viewer in one of two shaped runs; cause not separated (a CPU-starved receiver in a 3-core VM, or the controller's floor trap where an estimate sitting at the floor keeps a viewer slow) | RESULT=FAIL | Q-03.txt, Q-03.stats, Q-03.viewer-home, Q-03.viewer-office, Q-03b.stats, Q-03b.viewer-home, Q-03b.viewer-office, explore-two-unshaped-viewers.stats |
| Q-04 | DESK-07 | not run | Stopped after the Q-03 FAIL, as the standing decision for a DESK-07 failure requires (record it and report it to the orchestrator before going on) | RESULT=NOT-RUN | none |
| T-01 | DCTL-07 | not run | Stopped after the Q-03 FAIL for the same reason. The Phase 23 build is already installed in portty-test-2's /Applications and its AX helper and key poster are in place, so the row can start at once | RESULT=NOT-RUN | none |
| U-01 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-02 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-03 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-04 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-05 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-06 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-07 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-08 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-09 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-10 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-11 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| U-12 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| P-01 | UNLK-01 | run in 23-10 | pending | RESULT=NOT-RUN | |
| H-00 | SC5 | run in 23-11 | pending | RESULT=NOT-RUN | |
| H-01 | SC5 | run in 23-11 | pending | RESULT=NOT-RUN | |
| H-02 | SC5 | run in 23-11 | pending | RESULT=NOT-RUN | |
| H-03 | SC5 | run in 23-11 | pending | RESULT=NOT-RUN | |

## User checkpoint

Orchestrator standing decision on the user's behalf (2026-11-27, the same decision as Phases 21 and 22). The user was not asked: the dispatch of this plan carried the decision that the executor takes the VM permission clicks itself with the rig's AX click helpers, and this section records what was checked, what was clicked and what the re-read returned.

What the rig read before any click: the Phase 23 build (23.9.0) is installed on both VMs; portty-test's daemon holds both permissions (`GET /desktop/permissions` answered `{"sr":true,"ax":true}`, read before S-00 and again after the Safari change below); the S-00 result is PASS. Both VMs answered over ssh and neither was started or stopped; both are on the tailnet (reaching each other through the relay).

1. **Screen Recording and Accessibility on portty-test.** Nothing needed granting; both held. Answer recorded: "permissions ok, handled by the executor inside the VMs (admin account), as earlier phases did". Re-read after the other steps: `{"sr":true,"ax":true}`.
2. **Safari on portty-test-2, Allow JavaScript from Apple Events.** It was off (no such key). The Develop menu was not shown, so, all inside portty-test-2 through `~/p22/axp22` presses and `~/ptest/click`: Safari Settings, Advanced, pressed "Show features for web developers" (Develop menu appeared); Develop, Developer Settings, pressed "Allow JavaScript from Apple Events"; clicked Allow on the "Are you sure" sheet; macOS then asked for the password of the VM's own user (an authentication dialog from the system). The first time it was cancelled (clicked Cancel) while the choice was made; the second time the VM's public password was typed as key codes through `~/p22/key` (the same public password the rig's lock and unlock helpers already use; it is not a real credential and is not in any file). Re-read: the checkbox reads value 1. The rig itself reads pages through safaridriver (its "Allow remote automation and external agents" was already on, value 1), so no row depended on this setting. Answer recorded: "safari ok, handled by the executor".
3. **Other clicks on portty-test-2.** A leftover system alert "Allow Google Chrome to access your microphone?" (from an earlier Chrome launch) was answered Don't Allow (nothing in this phase needs a microphone). An "Install Command Line Developer Tools" window from an earlier tool shim was closed with `pkill -9 -f "Install Command Line Developer Tools"`.
3b. **Automation prompt during A-02 (Chrome).** macOS asked "sshd-keygen-wrapper wants access to control Google Chrome" when the rig first tried AppleScript on Chrome in portty-test-2; Allow was clicked at the system dialog (inside the VM, with `~/ptest/click`). That route was then abandoned (see the Notes), so the grant is unused.
4. **Re-authentication.** Not needed: neither VM left the tailnet.

Never changed anything on the host Mac for these items.

## Build under test

| Build | Version | Tags | Used for |
|-------|---------|------|----------|
| V | 23.9.0 | `wailsassets vmdesktop` | every row of this plan (HEAD `98ca0390` of this phase branch, Developer ID, hardened runtime, identifier `app.portty.Portty`; carries `GET /desktop/stats` and the VM-only `POST /desktop/debug/vm20` probe route) |

Built with `p23-rig.sh build` (a scratch `git archive` of HEAD outside the repo; compile only on the host), installed on portty-test with the Phase 19 `deploy` and `start`, and copied to portty-test-2 for the tab row. Both VMs are on the owner's real tailnet and reach each other through a DERP relay (their virtual network isolates guests from one another, so no direct path exists: `tailscale ping` reports `via DERP` and the guests' ARP entry for each other is incomplete). The rows only ever connect the two VMs to each other.

## Notes written during the run

- The criteria above were fixed before S-00 and the later rows were run, but after a first exploratory read of the stats route (the viewer opened, a few stats read) showed that over the relay the bandwidth estimate falls from the ceiling to the floor within about 10 to 40 s. Condition S-00.4 therefore reads the join, not the steady state, and the plan's text "a target equal to the ceiling" is kept in that form. The steady-state behaviour is recorded per row.
- **Findings the owner and 23-15 need (none was fixed here).**
  1. **Q-03 FAIL (DESK-07).** The slow chip reached an unshaped viewer in one of two shaped runs (see the row). Intermittent; the VM cannot say whether the receiver or the controller is at fault. It needs the real-Mac two-viewer row (M-161 and the 23-14 session) before release.
  2. **The estimate falls to the floor on a clean path.** With a Safari viewer on the loopback of the office VM and no pipe, the estimate left the ceiling within 10 to 40 s and sat at the floor for minutes under heavy motion (`explore-heavy-motion-loopback-viewer.stats`), while under light motion it held the ceiling for 25 s and recovered after a dip (`explore-light-motion-loopback-viewer.stats`). Over the relay it falls in under 10 s and recovers only briefly (`explore-light-motion-relay-viewer.stats`). Real loss was 0 (video packetsLost 0 at the loopback viewer), so this is delay-based or app-limited estimation, in a 3-core VM running two software decoders.
  3. **A silent audio stall is never reported (A-03).** After coreaudiod restarts, SCStream stops delivering audio without an error; the stats route reads `audio` `ok` while the viewer receives no audio packets until the capture run restarts. `noteAudioFailed` only fires from the capture error callback, so there is no stall detector. Rare in practice (coreaudiod restarts), but the failure the UI promises to show cannot show in this case.
  4. **Goroutine growth in the daemon.** The VM daemon read 32 goroutines at start with no viewer, 89 with one viewer, and 1020 with no viewer after about 25 viewer joins and leaves over two hours: about 16 to 20 left behind by each join and leave, and about 2 more every 10 s while idle after viewers came and went. Polling `GET /desktop/stats` 50 times changed nothing (1024 before and after), so the route is not the source. H-03 (23-11) measures this; expect it to fail its goroutine bound until the leak is found. The goroutine dump needed to find it was not taken (a SIGQUIT would count against the desktop's crash counter).
  5. **The VMs only reach each other through a DERP relay** (their virtual network isolates guests, ARP incomplete), so every cross-VM stream is relay-limited; the loopback viewer on portty-test is the only clean-path viewer available here.
  6. **Chrome could not be scripted in its page world.** AppleScript JavaScript runs in an isolated world (the page's RTCPeerConnection and debug object are invisible), and a raw DevTools socket client received no answer from Chrome's loopback endpoint; Chrome was read through the AX tree (web AX forced on) and the daemon's stats. This is why A-02 and Q-01 are PARTIAL.
  7. **Not run, carried forward:** Q-04 (still within 1 s) and T-01 (WKWebView unmute and the tab mark) were not run because the run stopped at the Q-03 FAIL; output through a real audio device is M-162 (23-14).
