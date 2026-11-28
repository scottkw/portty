---
status: awaiting_human_verify
trigger: "Q-03 FAIL (DESK-07): unshaped loopback viewer also showed Slow connection in 59 of 85 samples (run a); clean loopback viewer with no pipe and 0 loss fell from ceiling to floor under heavy motion and held the floor for minutes (finding 2)"
created: 2026-11-27T00:00:00Z
updated: 2026-11-27T02:00:00Z
---

## Current Focus

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

next_action: VM re-run of Q-03 (three runs) and the explore-heavy-motion lone loopback viewer on a build containing commit 9be32f6a; see Resolution.vm_recheck

## Symptoms

expected: with two viewers, only the shaped (slow) one is told slow; a slow viewer never drags the others down; a clean viewer whose estimate dipped recovers toward the ceiling when feedback is good
actual: run a: unshaped office viewer showed Slow connection from 8 s after the pipe went on, 59 of 85 samples, quality.slow 2 for 58 samples; run b: not reproduced. Finding 2: lone loopback viewer, no pipe, video packetsLost 0, target left the ceiling within 10 to 40 s under heavy motion and sat at the 500 kbps floor for minutes
errors: none (behavioural)
reproduction: VM row Q-03 (23-VM-CHECKS.md), evidence/Q-03*.stats, evidence/explore-*.stats; intermittent (1 of 2 runs)
started: first seen in 23-09 VM run (build 23.9.0)

## Eliminated

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

## Resolution

root_cause: "AND-gate of two conditions. (1) code: the controller read each viewer's pion GCC estimate as absolute link capacity (quality.go decideLocked: bad = est <= floorBps, good = est > 1.2 x floor; decide: room = est > 1.2 x target), but GCC measures only what it is sent (cut = 0.85 x acknowledged, clamped at the floor, rate_controller.go:173/:122; rise capped near 1.5 x acknowledged, :152/:160). Every viewer is sent the one shared encoder's output, so while the slow viewer holds the target at the floor (and whenever the encoder delivers well under its target) a clean viewer's estimate sits on the floor clamp after any delay transient: told slow, never 'good' again, and the target and ladder can never climb back (floor trap). (2) environment/data: the VM encoder delivered far under its target (explore-heavy t=40: <= 588 kbps acknowledged at a 1.42 Mbps target) and its CPU-starved receivers produce delay transients that trigger cuts."
fix: "viewerBWE counts the bytes each viewer is sent (header included); the controller measures that per viewer each second (qViewer.sentRate) and, unless the screen is still, counts a GCC estimate with upRatio (20%) room over what the viewer was really sent as the ceiling, for both the minimum and the slow test. Estimates GCC derived from congestion (cuts land at 0.85 x acknowledged, loss cuts) stay under 1.2 x sent and still constrain. When the encoder fills its target this is the existing room rule, so the filled case (all existing tests) is unchanged; while still, estimates hold as before."
verification: "RED then GREEN on two new deterministic tests; mutants at every fix line killed (initial sentBps, threshold, byte counting, still gate); go test -race ./internal/desktop/ ./internal/daemon/ on the final code: both ok, no races (an earlier run showed one unrelated load flake, TestWatchdog_RetryProbeFailKeepsLapse, 10/10 alone); go vet and gofmt clean; tests/check-brand.sh clean. Commit 9be32f6a. Not verified: real macOS two-viewer run (VM or Mac)."
vm_recheck: "Q-03 three runs (200 kbit/s pipe on the relay viewer): office loopback page slow false in every sample; stats quality.slow never 2; daemon.log 'viewer slow state changed' lines only for the shaped viewer's short id; while the shaped viewer is slow, target_bps leaves 500000 and climbs about 10 percent per 2 s with step returning toward 0 within about a minute; the home page still shows the chip. Finding 2: lone loopback viewer under heavy motion, no pipe: target may dip on real delay cuts but does not sit at 500000 for minutes, climbs back (500k to 4M in about 50 s), quality.slow 0. Control: the relay viewer may now read not slow if its slowness was the clamp artifact. If the loopback viewer is still told slow, it is genuine sustained delay cuts on its own path with the encoder filling about the floor (receiver CPU): check guest CPU and the page's framesDropped/jitterBufferDelay."
prevention_note: "Why not caught: the shaped and policy tests used a source that sends exactly its target (loopback_shaped_test.go loadSource) and stub estimates, so an app-limited sender, where GCC's estimate is pinned to what was sent, was never exercised. Recurrence guard: internal/desktop/quality_policy_test.go TestQuality_CleanViewerNotSlowBesideASlowOne and TestQuality_CleanViewerClimbsBackFromTheFloor (encoder at 60 percent of target, real viewerBWE byte path)."
oracle_type: derived (DESK-07 contract: only the slow viewer is told slow; the target recovers for the others)
files_changed: [internal/desktop/quality.go, internal/desktop/quality_policy_test.go]
