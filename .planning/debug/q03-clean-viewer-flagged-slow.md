---
status: awaiting_human_verify
trigger: "Q-03 FAIL (DESK-07): unshaped loopback viewer also showed Slow connection in 59 of 85 samples (run a); clean loopback viewer with no pipe and 0 loss fell from ceiling to floor under heavy motion and held the floor for minutes (finding 2)"
created: 2026-11-27T00:00:00Z
updated: 2026-11-28T06:00:00Z
---

## Current Focus

cycle: 2 (finding 2 persists on 23.10.2: lone clean loopback viewer sat at exactly 501680 for 118 s after two cuts, not slow, 0 loss)
bug_class: Bohrbug in the policy given its inputs (GCC estimates, the sent rate, the still flag); deterministic once GCC is in its additive regime

reasoning_checkpoint:
  hypothesis: "After two or more GCC cuts at similar received rates, pion's rate controller increases additively and returns min(est + inc, 1.5 x received over a 500 ms arrival window) (rate_controller.go increase, rate_calculator 500 ms). An app-limited sender kept near the rate of those cuts (the target the cuts produced) stays in that regime, so its estimate is pinned to 1.0-1.5 x what it is sent and, with bursty screen content, often under it (1 Hz content: at the floor clamp). The controller reads that estimate every tick as a limit unless it is >= 1.2 x the 1 s sent rate; it flips across that line every few ticks (or never reaches it), so decide's 5 s continuous room never holds: no raise, and no cut either (m >= 0.85 x target near the floor). The unconverted estimate is under 2 Mbps, so the ladder also steps down on a clean link."
  confirming_evidence:
    - "real-GCC probe, no congestion after t=20: heavy content stuck at 1071793 for 27 s with est 1.0-1.3 x sent and the ladder stepping down; light content est 500000-507716 (floor clamp) for 50 s, target exactly 500000, ladder to step 3; GCC state increase / usage normal at every tick"
    - "VM lone 23.10.2: ladder fixed at step 1 for 112 s and target fixed for 118 s means m crossed >= 2 Mbps (converted ceiling) at least every 2 s and fell <= 1.2 x target at least every 5 s: the per-tick flip"
    - "VM 23.10.3 light (idle CPU, steady jitter buffer): exactly 516732 for 80 s, unlocked only by a still blip (a refine keyframe burst moves GCC out of the additive band)"
  falsification_test: "a controller test with an estimate that follows pion's additive cap over the real viewerBWE byte path (1 Hz bursty content, GCC state increase): if the fix (only an estimate GCC derived while cutting constrains) still leaves the target at the floor, or if a cut GCC reports (state decrease) is no longer followed, the hypothesis or the fix is wrong"
  fix_rationale: "use GCC's own congestion signal instead of the estimate's level: GCC's delay controller is 'decrease' when it saw overuse and keeps that state through the hold after a cut (pion only rewrites latestStats on increase/decrease updates). viewerBWE latches it at every transport-cc report (GetStats) and the controller reads latch-or-current each tick; a viewer whose GCC has not been cutting since the last tick counts as the ceiling (its estimate is the app-limited cap), one that has keeps today's rule (raw estimate unless it has 1.2 x room over what was sent). While still, each viewer keeps its last moving-screen estimate instead of reading the raw one (a still tick cut 8 M -> 4.6 M in the lone run). decide stays pure and unchanged."
  blind_spots: "a congestion episode shorter than one feedback interval can be missed (then it is not followed, which the contract allows for transients); GCC blind from its 250-packet history (flood into a very slow link) shows no decrease, so such a viewer is not constrained while it counts but is still made slow by its receiver-report loss; random loss without queueing no longer pulls the shared target down (it never was congestion); VM GCC state is not observed directly yet (the new log line shows it)"
  candidate_causes:
    - "code: decideLocked reads the GCC estimate level as a limit; decide's 5 s continuous room hold; still ticks read the raw estimate"
    - "environment/data: app-limited encoder near the rate of past cuts and bursty screen content (1 Hz date, jittered VM capture) swing GCC's 500 ms received rate; VM CPU starvation supplies the initial cuts"
  and_gate: "yes: the trap needs (1) GCC in its additive regime (>= 2 past cuts at similar received rates, from CPU-starved receivers or a real transient) AND (2) an app-limited sender whose 500 ms received rate swings (bursty content) AND (3) the controller reading the pinned estimate as a limit. The fix removes (3)."

next_action: "VM re-run of the lone loopback viewer (heavy and light) on a build with the cycle-2 fix; read daemon.log 'desktop: target changed' lines (see vm_recheck)"

## Symptoms

expected: with two viewers, only the shaped (slow) one is told slow; a slow viewer never drags the others down; a clean viewer whose estimate dipped recovers toward the ceiling when feedback is good
actual: run a: unshaped office viewer showed Slow connection from 8 s after the pipe went on, 59 of 85 samples, quality.slow 2 for 58 samples; run b: not reproduced. Finding 2: lone loopback viewer, no pipe, video packetsLost 0, target left the ceiling within 10 to 40 s under heavy motion and sat at the 500 kbps floor for minutes
errors: none (behavioural)
reproduction: VM row Q-03 (23-VM-CHECKS.md), evidence/Q-03*.stats, evidence/explore-*.stats; intermittent (1 of 2 runs)
started: first seen in 23-09 VM run (build 23.9.0)

## Eliminated

- hypothesis: "H2 (cycle 2): GCC is in steady overuse/hold at the floor the whole time (genuine path delay)"
  evidence: "real-GCC probe reproduces the trap with zero congestion after the stalls: GCC state increase / usage normal at every tick while the target is stuck; VM light run stuck at 40-48% idle CPU with steady jitter buffer"
  timestamp: 2026-11-28T03:45:00Z
- hypothesis: "H3/H4 (cycle 2) as the cause of the trap: still ticks or refine-keyframe spikes in the 1 s sent window reset the hold"
  evidence: "still read true in 1 of 180 lone samples while stuck; the probe reproduces the trap with no still and no refine at all. Kept as a contributing defect: a still tick reads the raw (app-limited) estimate, which cut 8 M -> 4.6 M at t=57/58 of the lone run"
  timestamp: 2026-11-28T03:45:00Z

## Evidence

- timestamp: 2026-11-27T00:00:00Z
  checked: knowledge-base.md
  found: no entry matches (adaptive quality, slow chip, GCC, floor)
  implication: no known-pattern candidate

- timestamp: 2026-11-27T00:10:00Z
  checked: quality.go decideLocked (lines 460-497), decide (158-193), peer.go newAPI (106-117)
  found: each viewer has its own GCC (cc.NewInterceptor factory per PeerConnection, offered through bweSlot), so estimates are per viewer; but bad = est <= floorBps, good = est > 1.2 x floor (480), room = est > 1.2 x target (165), and GCC's MinBitrate is floorBps (peer.go:109) so "est == floorBps" is GCC's clamp; all viewers are fed one encoder (one target, quality.go:497)
  implication: the slow chip IS per-viewer feedback, not a global signal, but the feedback is relative to the shared send rate

- timestamp: 2026-11-27T00:15:00Z
  checked: pion interceptor v0.1.51 pkg/gcc rate_controller.go, loss_based_bwe.go, overuse_detector.go, send_side_bwe.go
  found: decrease() = 0.85 x latestReceivedRate (173) clamped to [min,max] (122); increase() multiplicative capped at 1.5 x received when coming from below (154-162), additive branch returns min(target+inc, 1.5 x received) (152) which can even LOWER the estimate with no congestion; loss controller min is 100 kbps (loss_based_bwe.go:49), not our floor; estimate = min(delay, loss) (send_side_bwe.go:301)
  implication: an app-limited viewer's estimate is pinned to what it is sent; under ~590 kbps sent any cut lands on the floor clamp

- timestamp: 2026-11-27T00:30:00Z
  checked: throwaway probe (real gcc.SendSideBWE, our options, synthetic lossless 5 ms link, TWCC recorder feedback every 50 ms, sender at 300 kbps, 300 ms delay ramps at 3/6/9 s)
  found: est 8000000 -> 515510 after the first ramp, 500000 (exact clamp) after the third, then +8%/s: 535476, 571851, 616927 (needs ~4 s to pass 600000); usage normal, loss 0 throughout
  implication: a clean, lossless, app-limited viewer reads "bad" (est <= floorBps) after a single transient and sits in the not-good band for seconds per transient; with transients every few seconds (CPU-starved VM) it never leaves

- timestamp: 2026-11-27T00:35:00Z
  checked: evidence/explore-heavy-motion-loopback-viewer.stats t=39..40, Q-03.stats, explore-two-unshaped-viewers.stats, loopback_shaped_test.go loadSource (238)
  found: target 1424756 -> 500000 in one tick with 0 loss (acked <= 588 kbps at a 1.42 Mbps target); Q-03 run a target 500000 step 3 for 85 s with slow 2; control: lone loopback viewer drove the target from 500000 up at 10%/2 s (healthy case); loadSource sends exactly its target
  implication: the encoder in the VM delivered far under its target; the unit tests never exercised an app-limited sender, which is why they pass while the VM fails

- timestamp: 2026-11-27T01:00:00Z
  checked: new tests before the fix (TestQuality_CleanViewerNotSlowBesideASlowOne, TestQuality_CleanViewerClimbsBackFromTheFloor), real viewerBWE byte path, encoder at 60% of its target, clean viewer's estimate following pion's caps
  found: RED exactly as the VM: "A, on a clean link, was told [true]", "QualityStats.Slow 2", "target 500000 a minute after B went slow"; lone viewer: "told [true]" with good feedback, "target 500000 at step 3" a minute later
  implication: the coupling and the floor trap reproduce deterministically in code, no CPU starvation needed once the sender is app-limited

- timestamp: 2026-11-27T01:10:00Z
  checked: fix applied, all TestQuality_* (incl. the two new); mutants at the fix site
  found: GREEN; mutants killed: initial sentBps 0 (NoFeedbackViewerCountsAsCeiling), threshold 0.8 instead of upRatio (CleanViewerClimbsBack, the real-cut check), byte counting removed (CleanViewerClimbsBack). A "!lossy" guard survived mutation and was removed (pion's loss controller already cuts a lossy viewer under what it is sent)
  implication: the fix is minimal and every remaining line is load-bearing

- timestamp: 2026-11-27T01:30:00Z
  checked: the fix against a still screen (nothing sent, so every estimate has "room" over it)
  found: without a gate a slow viewer was cleared over a still screen (B told [true false]) and the target was free to climb on no measurement; gated on the still machine's own flag (q.still.still, held under q.mu) estimates hold as before while still. Assertion added to TestQuality_CleanViewerNotSlowBesideASlowOne; the no-gate mutant fails it
  implication: still-screen behaviour (RESEARCH Pitfall 1) is unchanged

- timestamp: 2026-11-27T01:35:00Z
  checked: first full race run (before the still gate): go test -race ./internal/desktop/ ./internal/daemon/
  found: daemon ok; desktop one failure TestWatchdog_RetryProbeFailKeepsLapse (state streaming instead of awaiting); alone -race -count=10: 10/10 pass. The test emits the restarted frame before setting AX off, so a probe landing between the two legitimately recovers: load timing, no quality-controller involvement
  implication: unrelated flake under full-package load (not on the known list; reported)

- timestamp: 2026-11-28T03:10:00Z
  cycle: 2
  checked: evidence/Q-03-lone.stats and Q-03r1..r3.stats (target, step, slow, still per second)
  found: "lone: target fixed at 501680 for t=62..180 and ladder fixed at step 1 for t=68..180. A step down needs m < 2 Mbps for 2 s, a step up m >= 2.4 Mbps for 10 s, a raise m > 602016 for 5 s: so m reached >= 2 Mbps (only the converted ceiling can) at least every 2 s and fell to <= 602016 at least every 5 s; m flapped between the ceiling and about the floor all the time. Same in r1 t=65..101 (500000 at step 0 for 37 s, slow 1 constant) and r2 t=59..90. r-runs: every cut back to ~500k happened with slow constant at 1, so it was the clean office viewer's own estimate, not the slow viewer flapping in and out of the minimum. Climbs away from the floor ran raise-every-2-s for up to 32 s (r2), so the stuck intervals are specific to being near the floor."
  implication: "the clean viewer's estimate flips across 1.2 x sent many times a minute near the floor; the 5 s continuous room hold never completes. The two-viewer saw-tooth is the same clean-viewer mechanism, not the slow viewer."

- timestamp: 2026-11-28T03:15:00Z
  cycle: 2
  checked: pion interceptor v0.1.51 rate_controller.go increase(), rate_calculator.go, delay_based_bwe.go, feedback_adapter.go
  found: "increase() has two branches. Additive when the received rate lies inside the EMA of received rates at past decreases +/- 3 sigma (sigma is 0 after a single decrease, so it needs >= 2): returns min(est + inc, 1.5 x received), which LOWERS the estimate whenever the received rate dips, with no congestion. Multiplicative otherwise: never lowers, and grows 8%/s without bound past 1.5 x received once the estimate is above it. The received rate is bits over the span of a 500 ms arrival window (rate_calculator newRateCalculator(500ms)), so it swings with bursty screen content. An app-limited sender held near the rate of its last cuts (the target those cuts produced) keeps the received rate inside the band, so GCC stays in the additive branch and its estimate is pinned under 1.5 x a 500 ms received rate. Also: rate controller uses time.Now() internally, so a real-GCC model has to run in real time."
  implication: "H5 (new): near the floor GCC's estimate dips under 1.2 x what was sent whenever a 500 ms window holds less than ~80% of the 1 s average (content swings), not on congestion; each dip resets the controller's 5 s hold. Above ~588 kbps the same dips can read as cuts. Explains the floor-specific asymmetry."

- timestamp: 2026-11-28T03:20:00Z
  cycle: 2
  checked: new VM data relayed by the orchestrator's VM agent (build 23.10.3, lone loopback viewer; files to be committed later as evidence/Q-03-lone2-{heavy,light}.*)
  found: "light motion (date once a second), CPU 40-48% idle, loss 0: cut 1.43 M -> 516732 at t=11, then EXACTLY 516732 for 80 s; still read true only at t=90-91 and right after the target climbed 516k -> 7.45 M in ~58 s. Heavy (CPU 5-12% idle): climbs 10%/2 s, cuts to ~500-532k, flat stretches 575498 (32 s), 524764 (23 s), 500000 (44 s+)."
  implication: "the trap does not need CPU starvation (light, idle CPU). 1 Hz content is the worst case for a 500 ms received-rate window (H5). The still blip unlocking it fits: a refine keyframe (4 x target) is a burst whose received rate is far outside the decrease band, so GCC leaves the additive branch (or a burst-driven decrease moves the band) and the multiplicative branch then climbs without bound. H3 (still ticks as the cause of the trap) is unlikely: still was nearly never true while stuck."

- timestamp: 2026-11-28T03:45:00Z
  cycle: 2
  checked: "throwaway real-time probe (internal/desktop/zz_gccprobe_test.go, build tag gccprobe, not committed): real gcc.SendSideBWE with our options behind the real viewerBWE and controller, twcc.Recorder feedback every 100 ms, synthetic lossless loopback (1 ms + jitter), audio 50 pps, encoder filling its target; five 300 ms receiver stalls (link drains at 100 kbps) at t=3..19 s to put GCC through repeated cuts, then NO congestion at all for 50 s"
  found: "heavy (8 fps jittered frames): stall cuts followed (8M -> 500000 at t=8, 605000 -> 500000 at t=20), then the target crawled to 1071793 by t=43 and sat there to the end (27 s) while GCC est hovered 1.07-1.42 M = 1.0-1.3 x sent (additive cap), 'above' flipping every few ticks, and the ladder stepped DOWN 0 -> 1 -> 2 on a clean link (unconverted est < 2 Mbps). Light (25 fps, 60% of each second's bytes in one 1 Hz frame, the 'date once a second' case): after the stalls GCC est sat at 500000-507716 (the floor clamp, 1.5 x a 500 ms window without the big frame is ~0.8 x sent) for 50 s, never >= 1.2 x sent, target exactly 500000 for 66 s, ladder walked to step 3 (1280). GCC stats at every one of those ticks: state increase, usage normal (no congestion seen by GCC itself)."
  implication: "H5 confirmed with pion's real code: the trap needs only an app-limited sender and a few past cuts; GCC's own delay detector reports no congestion the whole time. H2 (steady overuse) eliminated for the model. The estimate's level cannot separate congestion from the cap (light: est < sent always), so a value-based room rule (tolerating dips, comparing with a shorter sent window) cannot fix it; GCC's congestion signal (its delay controller in decrease) can."

- timestamp: 2026-11-28T04:00:00Z
  cycle: 2
  checked: new test TestQuality_PinnedEstimateDoesNotHoldTheTargetAtTheFloor on the cycle-1 code (real viewerBWE byte path; estimate follows pion's additive cap over a 500 ms window of 1 Hz bursty content; GCC stats say increase after a 3 s cut)
  found: "RED: 'a minute after the episode, GCC estimate 500000 (pinned, no congestion): target 500000 at step 3, want past 4 Mbps at step 0 (held at the floor)'. All other TestQuality_* pass."
  implication: "the trap reproduces deterministically in a unit test, matching the light probe and the VM light run"

- timestamp: 2026-11-28T04:40:00Z
  cycle: 2
  checked: "fix in quality.go (viewerBWE latches GCC state 'decrease' at each transport-cc report; decideLocked counts a GCC estimate only while GCC is cutting or the viewer is lossy, and not then with 1.2 x room over what was sent; while still each viewer keeps the estimate it last counted with; one 'desktop: target changed' log line per target change) and the TestQuality_* suite"
  found: "GREEN: all TestQuality_* pass. Existing fakes updated to model what pion reports: CleanViewerClimbsBack's dip and final cut set GCC cutting (the final cut through a report latch, then cleared before the tick); CleanViewerNotSlowBesideASlowOne's B is a full drop-tail queue (not cutting, 60% loss) and must hold the target at the floor while it still counts; NoFeedbackViewerCountsAsCeiling's first check now asserts no_feedback 0 before the first packet (its old 'a 900 kbps estimate pulls the target' premise is exactly what the fix removes: that fake GCC is not cutting). RED check: the test file against the cycle-1 quality.go fails only TestQuality_PinnedEstimateDoesNotHoldTheTargetAtTheFloor."
  implication: "the fix cures the deterministic reproduction and keeps congestion and loss constraining"

- timestamp: 2026-11-28T04:50:00Z
  cycle: 2
  checked: "hand-seeded mutants at every fix line (scratchpad mutants.py), TestQuality_* each"
  found: "8 of 8 killed: M1 cycle-1 rule only (ignore cutting/lossy) -> PinnedEstimate; M2 drop lossy -> CleanViewerNotSlowBesideASlowOne; M3 drop report latch -> CleanViewerClimbsBack; M4 drop tick poll -> CleanViewerClimbsBack + PinnedEstimate; M5a still reads raw -> PinnedEstimate; M5b no still gate -> CleanViewerNotSlowBesideASlowOne + PinnedEstimate; M6 drop room-over-sent -> CleanViewerClimbsBack; M7 latch never cleared -> PinnedEstimate. A slow-set freeze while still was tried and removed: it survived mutation (the held estimate already keeps a slow viewer slow)."
  implication: "every remaining fix line is load-bearing"

- timestamp: 2026-11-28T04:55:00Z
  cycle: 2
  checked: "the real-GCC probe on the fixed controller (same three scenarios plus a lossless 1.5 Mbps bottleneck with a deep queue, start 8 M)"
  found: "heavy: climbed 501680 -> 8 M by t=67 (no stuck stretch, ladder at 0 throughout). Light: stall cuts followed (1.43 M -> 740205 -> 529015), held while the stalls lasted (to t=19), then climbed from t=25 to 3.91 M at t=70 at step 0 while GCC est stayed pinned at 500000. Bottleneck 1.5 Mbps: GCC 'decrease' latched 55 times in 70 s, the target held 0.92-1.34 M (under the link) with the queue under ~350 ms; it never ran to the ceiling."
  implication: "with pion's real estimator the target climbs back after transient cuts on a clean path and still follows sustained congestion"

- timestamp: 2026-11-28T05:40:00Z
  cycle: 2 (ladder follow-up requested by the orchestrator before merge)
  checked: "the 10566b82 controller on a constrained link: decide stepped the ladder on the per-tick minimum, which is now the ceiling whenever GCC is not cutting"
  found: "regression: the real-GCC 1.5 Mbps bottleneck probe held the target at 0.92-1.34 M with the ladder at step 0 (2560 at 30 fps) throughout. New TestQuality_ConstrainedLinkStepsTheLadderDown (GCC cuts to 85 percent of 1.5 Mbps whenever the target is over it, not cutting between) is RED on 10566b82: 'ladder at step 0 (fps 30, long edge 2560) after 40 s with the target under 2000000, want 3'."
  implication: "the ladder has to follow a signal that stays low on a slow link between cuts; the target does (it follows cuts down at once and climbs only after 5 s of room)"

- timestamp: 2026-11-28T05:55:00Z
  cycle: 2
  checked: "decide drives the ladder from the new target (LowSince while Target < ladderBps, OKSince while Target >= 1.2 x ladderBps); decide stays pure; TestQuality_*, mutants, real-GCC probe"
  found: "GREEN. Four tests encoded the minimum-driven ladder timing and were updated to the new semantics, none showing a defect: DecideLadderDownThenBackUp (recovery now waits for the target to pass 2.4 Mbps at 23 s, then one step per 10 s), DecideFlappingMakesNoStormOfSteps (a flapping minimum holds the target down, so the ladder only steps down, >= 10 s apart; part 2 now starts from full quality), CleanViewerClimbsBackFromTheFloor and PinnedEstimateDoesNotHoldTheTargetAtTheFloor (step 0 asserted after 75 s instead of 60 s). Mutants: 10 of 10 killed (M1-M7 as before; L1 ladder-low from the minimum -> ConstrainedLink; L2 ladder-ok from the minimum -> DecideLadderDownThenBackUp). Probe: bottleneck 1.5 Mbps steps 0->1 at 2.75 s, ->2 at 13 s, ->3 at 23 s and stays at 3 with the target 1.0-1.35 M; clean heavy and light recoveries dip to step 3 during the climb and return to step 0 about 70 s after the last cut (heavy: 500000 at t=20 -> 8 M and step 0 at t=90; light: step 0 at t=88)."
  implication: "slow links get the smaller, slower picture again; the cost is on clean recoveries: after a cut under 2 Mbps the ladder steps down during the climb (two size steps down, two up, each a keyframe, >= 10 s apart) and full quality returns about 70 s after a cut to the floor instead of staying at 2560 x 30 fps throughout"

## Resolution

root_cause: "AND-gate (cycle 2, finding 2 persists). (1) GCC regime: after two or more cuts at similar received rates, pion v0.1.51's rate controller increases additively and returns min(est + inc, 1.5 x the received rate over a 500 ms arrival window) (rate_controller.go increase, rate_calculator.go); an app-limited sender kept near the rate of those cuts (the target they produced) stays in that regime, so its estimate is pinned to 1.0-1.5 x what it is sent and with bursty screen content often under it (1 Hz content: on the 500 kbps floor clamp), while GCC's own delay controller reports no congestion (state increase, usage normal). (2) code: quality.go decideLocked read that estimate every tick as a limit unless it was >= 1.2 x the 1 s sent rate (cycle-1 rule); it flipped across that line every few ticks or never reached it, so decide's 5 s continuous room never held: no raise and, near the floor, no cut (m >= 0.85 x target): stuck; the unconverted estimate is also under 2 Mbps, so the ladder stepped down on a clean link. Contributing: a still tick read the raw (pinned) estimate as a limit (lone run t=57 still -> t=58 cut 8 M to 4.6 M). (3) environment: the VM supplies the initial cuts (CPU-starved receivers) and bursty capture (5-11 fps jittered, or 1 Hz date). The two-viewer saw-tooth (Q-03r1..r3) is the same clean-viewer mechanism, not the slow viewer: slow stayed 1 through every cut."
fix: "Use GCC's own congestion signal instead of the estimate's level. viewerBWE latches GCC state 'decrease' (pion keeps it through the hold after a cut) at every transport-cc report; each tick reads latch-or-current (viewerBWE.cutting). A viewer's GCC estimate constrains only while GCC is cutting or the viewer is lossy (> 20 percent in its receiver reports: a full drop-tail queue shows no delay trend), and not even then with 1.2 x room over what it was really sent (cycle-1 rule kept); otherwise it counts as the ceiling. While the screen is still each viewer keeps the estimate it last counted with (qViewer.held) instead of reading the raw one. decide is unchanged and pure. Ladder (follow-up): decide steps the ladder on the new target instead of the per-tick minimum (LowSince while target < 2 Mbps, OKSince while target >= 2.4 Mbps), because between GCC's cuts the minimum is the ceiling even on a slow link; decide stays pure. Diagnostic: one slog.Info 'desktop: target changed' per target change with from, to, min, viewer, est (raw GCC), sent (that viewer's 1 s sent rate), cutting, still. Stats route key set unchanged."
verification: "RED then GREEN: new TestQuality_PinnedEstimateDoesNotHoldTheTargetAtTheFloor fails on the cycle-1 quality.go exactly as the VM ('GCC estimate 500000 (pinned, no congestion): target 500000 at step 3') and passes with the fix. Hand-seeded mutants at every fix line: 8 of 8 killed (scratchpad mutants.py; listed in Evidence). Real-GCC real-time probe (not committed): after the episode the target climbs back (heavy 501680 -> 8 M by t=67; light 529015 -> 3.91 M within 45 s of the last stall), while a lossless 1.5 Mbps bottleneck still holds it at 0.92-1.34 M. go test -race ./internal/desktop/ ./internal/daemon/: both ok first run (desktop 193 s, daemon 149 s), TestQualityShaped_* also ok alone with -race -v; go vet ./... and -tags vmdesktop ./... clean; gofmt clean; tests/check-brand.sh clean. Fix commit 10566b82. Ladder follow-up: TestQuality_ConstrainedLinkStepsTheLadderDown RED on 10566b82, GREEN with the target-driven ladder; 10 of 10 mutants killed; probe bottleneck steps to 3 and stays, clean recoveries back to step 0 about 70 s after the last cut; go test -race desktop and daemon, vet both tag sets, gofmt, check-brand all clean again (desktop 187 s, daemon 133 s, TestQualityShaped_* alone ok); ladder commit 08071149. Not verified: VM or real-Mac run of the fixed build."
vm_recheck: "Lone office loopback viewer, no pipe, 180 s each, heavy (Terminal line stream) and light (date once a second) motion, on a build carrying this fix: after any cut the target leaves its new value within about 10 s (5 s hold, then about 10 percent per 2 s) and keeps climbing to multi-Mbit/s (light: from about 500k past 4 M within about a minute); no flat stretch at or near 500000 longer than about 15 s unless the daemon log shows a fresh cut in it; quality.slow 0. Ladder (target-driven): after a cut under 2 Mbps it steps down during the climb (15 fps after 2 s, then 1920 and 1280, one step per 10 s) and comes back one step per 10 s once the target has held over 2.4 M, so step 0 about 30 s after the target passes 2.4 M, about 70 s after a cut to the floor; steps never closer than 10 s. Read daemon.log lines 'desktop: target changed': every downward line should say cutting=true (GCC delay controller cutting) for the office viewer, and the raise lines show min=8000000 with est (raw GCC) near sent, which confirms the pinned-estimate mechanism. If a flat stretch over 30 s appears with no 'target changed' lines, the trap is something else: capture the last lines' est/sent/cutting. Q-02 (lone viewer behind a 300 kbit/s pipe): the ladder steps down to 1280 at 15 fps as before (the target sits at the floor). Two-viewer Q-03 should still pass as before; cuts back toward 500k may remain (GCC really cutting on the CPU-starved VM, cutting=true, viewer = office id) but each climb resumes within about 10 s, with no 30 s flat stretches."
prevention_note: "Why not caught: the cycle-1 tests modelled GCC as rising 8 percent a second up to 1.5 x what was sent (cleanGCC), i.e. pion's multiplicative branch on smooth traffic; pion's additive branch, its 500 ms received-rate window and bursty screen content were never exercised, and the real-GCC shaped tests send exactly the target (loadSource). Recurrence guard: internal/desktop/quality_policy_test.go TestQuality_PinnedEstimateDoesNotHoldTheTargetAtTheFloor (pinnedGCC over 1 Hz bursty content, GCC not cutting) and the 'desktop: target changed' log line for every VM row that watches the target."
oracle_type: derived (DESK-07 contract: a clean viewer with no loss and no sustained congestion is not held at the floor; congestion or heavy loss still constrains; a still screen measures nothing)
files_changed: [internal/desktop/quality.go, internal/desktop/quality_policy_test.go] (commits 10566b82, 08071149)

## Cycle 1 focus (kept for history)

bug_class: Bohrbug in the policy (deterministic given the inputs); the VM supplies the triggers (delay transients from a CPU-starved receiver and unpaced bursts, encoder output well under its target)

reasoning_checkpoint:
  hypothesis: "The controller reads pion GCC estimates as absolute link capacity, but GCC only measures what it was sent: a cut lands at 0.85 x the acknowledged rate (rate_controller.go:173) clamped at the 500 kbps floor (peer.go:109, rate_controller.go:122), and a rise is capped near 1.5 x the acknowledged rate (rate_controller.go:152, :160). Every viewer is sent the one shared encoder's output, so when that output is under ~590 kbps (the slow viewer pulls the target to the floor while it counts; VideoToolbox/capture deliver well under the target) a clean viewer's estimate sits on the floor clamp after any delay transient -> quality.go:480 bad (est <= floorBps) -> told slow after 3 s; and est can never exceed 1.2 x floor (good) or 1.2 x target (room, quality.go:165) while output < 0.8 x target, so the viewer stays slow and the target and ladder never climb back (floor trap)."
  confirming_evidence:
    - "probe against real pion GCC (min floor, max ceiling, no-op pacer): sender app-limited at 300 kbps on a lossless constant-delay link, one 300 ms delay ramp -> est exactly 500000 (the clamp, quality.go 'bad'); from there it needs ~2.4 s at 8%/s to pass 600000"
    - "explore-heavy t=40: target 1424756 -> 500000 in one step with packetsLost 0: GCC landed at the clamp, so the acknowledged rate was <= 588 kbps (0.85 x acked <= floor) while the target was 1.42 Mbps: the encoder delivered ~40% of its target or less"
    - "Q-03 run a: target 500000 at step 3 for all 85 samples with quality.slow 2: both viewers out of the minimum -> lowestAll (quality.go:494-496) keeps the shared output at the floor, so the clean viewer's own estimate cannot leave [floor, 1.5 x what it is sent]"
    - "existing shaped tests pass because loadSource sends exactly its target (loopback_shaped_test.go:238): 1.5 x sent is then always > 1.2 x target, so the app-limited case is never exercised"
  falsification_test: "a controller test where the clean viewer is sent the real shared output (encoder at 60% of target) and its estimate follows pion's caps: if the clean viewer is still told slow or the target still cannot climb with the fix (estimate with >= 20% room over what was sent counts as the ceiling), the hypothesis is wrong"
  fix_rationale: "measure what each viewer was actually sent (bytes through its viewerBWE writer, per second) and treat an estimate with upRatio room over that as not limited by the link (counts as the ceiling for the minimum and the slow test). Only estimates GCC derived from congestion (cuts land at 0.85 x acked, under 1.2 x sent; loss cuts) still constrain. When the encoder fills its target (sent ~= target) this is exactly today's room rule, so the filled case is unchanged."
  blind_spots: "VM receivers may also be cut repeatedly by genuine delay spikes (CPU starvation): with the encoder filling the floor that still reads as slow, as before (environment, not coupling). A link that queues without ever losing while the encoder sends under ~420 kbps is told slow only once it loses (GCC's min stays at the floor). pion's additive branch could not be forced in the probe; the test models its cap as the worst case."
  candidate_causes:
    - "code: quality.go:480 bad/good on absolute est vs floor, quality.go:165 room vs target, both with GCC estimates that are app-limited by the shared encoder"
    - "environment: receiver CPU starvation (3-core VM, two software decoders) producing real delay overuse on the loopback path"
    - "data: encoder output far under its target (VideoToolbox average bitrate, capture fps under CPU load) - evidenced by explore-heavy t=40"
  and_gate: "yes: the clean viewer is flagged only when (1) its sender is app-limited under ~590 kbps (shared target at the floor and/or encoder undershoot) AND (2) a delay transient cuts its GCC. Run b had (1) without a cut in the trap window. The fix removes the code half: an app-limited estimate is never read as a limit."

next_action: Q-03 VM re-run PASSED (3 of 3) on 23.10.2; the lone-loopback finding 2 check did not (target sat at 501680 for 118 s); see vm_recheck_result at the end

## Cycle 1 resolution (kept for history)

root_cause: "AND-gate of two conditions. (1) code: the controller read each viewer's pion GCC estimate as absolute link capacity (quality.go decideLocked: bad = est <= floorBps, good = est > 1.2 x floor; decide: room = est > 1.2 x target), but GCC measures only what it is sent (cut = 0.85 x acknowledged, clamped at the floor, rate_controller.go:173/:122; rise capped near 1.5 x acknowledged, :152/:160). Every viewer is sent the one shared encoder's output, so while the slow viewer holds the target at the floor (and whenever the encoder delivers well under its target) a clean viewer's estimate sits on the floor clamp after any delay transient: told slow, never 'good' again, and the target and ladder can never climb back (floor trap). (2) environment/data: the VM encoder delivered far under its target (explore-heavy t=40: <= 588 kbps acknowledged at a 1.42 Mbps target) and its CPU-starved receivers produce delay transients that trigger cuts."
fix: "viewerBWE counts the bytes each viewer is sent (header included); the controller measures that per viewer each second (qViewer.sentRate) and, unless the screen is still, counts a GCC estimate with upRatio (20%) room over what the viewer was really sent as the ceiling, for both the minimum and the slow test. Estimates GCC derived from congestion (cuts land at 0.85 x acknowledged, loss cuts) stay under 1.2 x sent and still constrain. When the encoder fills its target this is the existing room rule, so the filled case (all existing tests) is unchanged; while still, estimates hold as before."
verification: "RED then GREEN on two new deterministic tests; mutants at every fix line killed (initial sentBps, threshold, byte counting, still gate); go test -race ./internal/desktop/ ./internal/daemon/ on the final code: both ok, no races (an earlier run showed one unrelated load flake, TestWatchdog_RetryProbeFailKeepsLapse, 10/10 alone); go vet and gofmt clean; tests/check-brand.sh clean. Commit 9be32f6a. Not verified: real macOS two-viewer run (VM or Mac)."
vm_recheck: "Q-03 three runs (200 kbit/s pipe on the relay viewer): office loopback page slow false in every sample; stats quality.slow never 2; daemon.log 'viewer slow state changed' lines only for the shaped viewer's short id; while the shaped viewer is slow, target_bps leaves 500000 and climbs about 10 percent per 2 s with step returning toward 0 within about a minute; the home page still shows the chip. Finding 2: lone loopback viewer under heavy motion, no pipe: target may dip on real delay cuts but does not sit at 500000 for minutes, climbs back (500k to 4M in about 50 s), quality.slow 0. Control: the relay viewer may now read not slow if its slowness was the clamp artifact. If the loopback viewer is still told slow, it is genuine sustained delay cuts on its own path with the encoder filling about the floor (receiver CPU): check guest CPU and the page's framesDropped/jitterBufferDelay."
prevention_note: "Why not caught: the shaped and policy tests used a source that sends exactly its target (loopback_shaped_test.go loadSource) and stub estimates, so an app-limited sender, where GCC's estimate is pinned to what was sent, was never exercised. Recurrence guard: internal/desktop/quality_policy_test.go TestQuality_CleanViewerNotSlowBesideASlowOne and TestQuality_CleanViewerClimbsBackFromTheFloor (encoder at 60 percent of target, real viewerBWE byte path)."
oracle_type: derived (DESK-07 contract: only the slow viewer is told slow; the target recovers for the others)
files_changed: [internal/desktop/quality.go, internal/desktop/quality_policy_test.go] (commit 9be32f6a)
vm_recheck_result: "2026-11-28, build 23.10.2 (d11e1811). Q-03 three runs PASS (evidence/Q-03-rerun.txt): office loopback page slow false in every sample (0 of 110 per run), quality.slow never 2, slow-state lines only for the shaped viewer, target left 500000 while the shaped viewer was slow (best 561282 to 2836967 in about 30 s; in two runs climbs were cut back to about 500000 every 20 to 30 s), home chip shown and cleared 13 to 31 s after the pipe was removed. Finding 2 check NOT passed (evidence/Q-03-lone.stats): lone loopback viewer, heavy motion, no pipe: target climbed 1.15 M to 8 M in 47 s, then after two cuts sat at exactly 501680 for 118 s to the end of the run (step 1, slow 0, video lost 0, framesDropped 0, jitter buffer 110 to 174 ms, guest CPU 13 to 33 percent idle). Left open in place: the clean-viewer slow flag is fixed, but a clean lone viewer can still sit at the floor for minutes after a cut without being slow; next step is to read why no increase decision is taken at 501680 (est vs 1.2 x sent while the encoder fills about the floor)."
