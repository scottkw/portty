# Phase 23 deferred items

Out-of-scope discoveries logged by executors. Nothing here was fixed by the plan that found it.

## From 23-02

### RESOLVED: A source restart gives every viewer a new random RTP sequence number, and viewers freeze until it passes the old one (Phase 19, pre-existing)

- **Resolved 2026-11-27 (debug `rtp-seq-restart-freeze`):** the video packetizer and the audio clock now live on the Manager for its life, so sequence numbers and timestamps run on across a restart, and a new run's first video unit is stamped with the real gap. Guarded by `TestFanout_SourceRestartKeepsRTPSequence` and `TestFanout_SourceRestartStampsTheGap` (internal/desktop/fanout_test.go). A browser check on the VM or a real Mac is still to run: TESTING.md M-221, `.planning/debug/rtp-seq-restart-freeze.md`.

- **Where:** `internal/desktop/manager.go`, `fanOut`: `rtp.NewPacketizer(..., rtp.NewRandomSequencer(), ...)` is built per run, and a run ends with every source restart (`restartSource`: stall recovery, wake, awaiting-approval retry).
- **What happens:** the viewers' PeerConnections, SSRC and SRTP contexts survive the restart, but the packets now start at a new random 16-bit sequence number. SRTP replay protection (and the rollover-counter estimate) on the receiving side drops packets whose sequence is behind the last one it accepted, so the picture stays frozen until the new sequence overtakes the old, up to about 40 s at 830 packets per second, or longer after a jump the receiver estimates into the wrong rollover.
- **Evidence (host, loopback, pion client):** `TestQualityShaped_StillScreenDoesNotCollapseEstimate` in its first form had no pointer-nudge fake, so the Phase 19 watchdog classified the quiet source as stalled and restarted it ("stall recovered"). After the restart the fan-out delivered 30 units per second and the daemon wrote them to both viewers, but viewer A received 0 units for the 4 s that followed. Replacing the per-run sequencer with one that survives the restart (a throwaway experiment, reverted) made A receive 30 units per second again at once.
- **Why it was left alone:** it is Phase 19 behaviour, not caused by the adaptive controller (a quality step uses `Source.Rebuild` and never restarts the source), and a fix touches the media path every viewer uses.
- **Suggested fix (one line plus a test):** keep one `rtp.Sequencer` (and the RTP timestamp base) on the Manager for the life of its PeerConnections and hand it to every `fanOut`; add a loopback test that restarts the source and requires a unit on a viewer within 2 s. The new shaped-link tests only avoid the restart because their fake nudge makes the watchdog classify the screen as still.
- **Severity:** high for the rock-solid priority (a stall recovery or a wake that leaves a remote viewer frozen on an unattended Mac), unverified against a real browser (a browser's SRTP stack should behave like pion's here, but only a VM row can say).
