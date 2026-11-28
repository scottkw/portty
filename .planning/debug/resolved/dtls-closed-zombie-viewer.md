---
status: resolved
trigger: "pion closes the server's peer connection by itself when the viewer's side sends a DTLS close notice ('Closing PeerConnection from DTLS CloseNotify', state Closed). serveAdmitted's connection-state handler (internal/desktop/manager.go ~796-814) acts only on Connected and Failed, so a viewer whose media closed but whose signalling socket stayed open keeps counting as a viewer and keeps capture running until the socket drops (over 60 s in the harness). Also look at: serveAdmitted's 2 s close ceiling does not hold while a viewer's read loop is blocked in coder/websocket (a silent peer's socket lingers ~5 s)."
created: 2026-11-27T03:00:00Z
updated: 2026-11-28T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED and fixed (commits e4eb9f23 Closed branch, 5fd2cde7 close ceiling) - the OnConnectionStateChange handler in serveAdmitted ignored PeerConnectionStateClosed, so a pion-initiated close (DTLS CloseNotify from the viewer) never ended the session; separately the 2 s close ceiling never cut a silent peer's blocked read short
bug_class: Bohrbug (deterministic: client pc.Close with socket kept open; silent peer cut)
test: TestFanout_JoinLeaveCyclesLeaveNoGoroutines/media_closed_socket_open and TestGuestStop_ThreeSilentPeersDoNotHoldItUp (3.5 s bound), both RED before and GREEN after
expecting: in a real VM run, a viewer whose media ends (page crash, half-closed client) leaves within about 2 s, and a Stop with silent guests ends them in about 2 s, not 5-7 s
next_action: human verify in the VM (not allowed here): G-04-style Stop with an unresponsive guest, read the "viewer left" timestamps (expect about 2 s after the stop, was 5-7 s); optional: kill a viewer page's media only (devtools pc.close()) and watch the viewer count drop within about 2 s

reasoning_checkpoint:
  hypothesis: "A viewer whose PeerConnection is closed by pion on the viewer's DTLS close notice stays joined because serveAdmitted's state handler has no Closed branch; only the socket dropping ends the read loop, which is what runs leave() (viewer count, capture stop)."
  confirming_evidence:
    - "Leave mode media_closed_socket_open (client pc.Close, socket left open and read): FAIL 'timed out waiting for viewer gone after its media closed' at 3 s; the viewer only left when the test's cleanup closed the socket."
    - "Previous session (pc_only mode, PION_LOG_INFO=all): 'Closing PeerConnection from DTLS CloseNotify', server pc closed, viewer kept 60 s+."
    - "manager.go:799-817: the switch has Connected and Failed cases only."
  falsification_test: "If the viewer still lingers after the handler ends the session on Closed (e.g. pion never reports Closed to OnConnectionStateChange for a CloseNotify close), the mode stays RED and the hypothesis is wrong."
  fix_rationale: "Ending the session on Closed (as on Failed) is the missing reaction itself. Routing both through cutViewer gives one close code per viewer (its CAS: a revoke's 4432 cannot race a 1001), the ordered cut and the close ceiling; an 'ended' flag stored just before serveAdmitted's own deferred pc.Close keeps the normal leave from cutting again (pion runs the handler on a goroutine started inside Close, so it sees the flag)."
  blind_spots: "Real browsers close pc and socket together (desktopClient.ts teardown): the cut and the client's own close may overlap; both are concurrency-safe in coder/websocket and the viewer is leaving anyway. Not run against a real browser or VM."
  candidate_causes:
    - "code: missing Closed branch in serveAdmitted's state handler - CONFIRMED"
    - "environment/library: pion not emitting Closed for a remote CloseNotify close - ELIMINATED by pion source (CloseNotify path calls pc.Close, which sets state Closed via onConnectionStateChange) and the previous session's log"
    - "data/protocol: the viewer page leaving its socket open after closing pc - real pages close both (desktopClient.ts:534-556), so production exposure is a page crash/bug or a half-closed client; the server must not depend on it"
  and_gate: "no for the zombie: the missing branch alone keeps the viewer joined whenever the socket outlives the media. The slow end on a silent peer is a separate cause (coder/websocket CloseNow-after-Close is a no-op wait; the ceiling never cut the read loop short)."

## Symptoms

expected: a viewer whose PeerConnection is closed by its peer (DTLS close notice) leaves within a couple of seconds: viewer count drops, its socket is closed, capture stops when it was the last viewer
actual: the server pc goes to Closed but the viewer stays registered and capture keeps running until the signalling socket drops (60 s+ in the harness)
errors: none; pion INFO log "Closing PeerConnection from DTLS CloseNotify"
reproduction: loopback harness, client pc.Close() with the signalling socket left open (debug session desktop-goroutine-leak, evidence 2026-11-27T01:05)
started: since serveAdmitted's handler was written (it only ever handled Connected and Failed)

## Eliminated

## Evidence

- timestamp: 2026-11-27T03:00:00Z
  checked: .planning/debug/desktop-goroutine-leak.md evidence 01:05 (pc_only leave mode, PION_LOG_INFO=all)
  found: "Closing PeerConnection from DTLS CloseNotify", server pc state closed, viewer stays joined 60 s+ until the socket drops; all goroutines released once the socket closes
  implication: the defect is the missing Closed branch, not a leak

- timestamp: 2026-11-27T03:05:00Z
  checked: knowledge-base.md for dtls/ceiling/zombie/CloseNow
  found: no match
  implication: no known-pattern candidate

- timestamp: 2026-11-27T03:10:00Z
  checked: pion webrtc v4.2.29 peerconnection.go:549-555 onConnectionStateChange, :2886-2923 close
  found: the state handler runs on its own goroutine (go handler(cs)); Close is idempotent (isClosed.Swap, second call returns nil); pion's DTLS CloseNotify path calls pc.Close itself (:3213)
  implication: a Closed handler also fires after OUR closes: cutViewer (sets v.cut before pc.Close) and serveAdmitted's deferred pc.Close (after leave, the read loop already ended). Acting on Closed must skip both, or it writes a second close frame (a revoke's 4432 could race a 1001).

- timestamp: 2026-11-27T03:15:00Z
  checked: serveAdmitted teardown (manager.go:795-848), cutViewer/closeSocket (revoke.go:120-158)
  found: defers run detach, leave (+OnViewerLeave), q.unregister, pc.Close (LIFO). cutViewer CAS v.cut, view-only, input ended, arbiter forget, pc.Close, syncTap, go closeSocket(ws) with the 2 s ceiling. The Failed branch closes the socket with a bare c.Close (no ceiling, no ordered cut).
  implication: routing Failed/Closed through cutViewer gives one close code (the CAS) and the ordered cut; a flag set in the deferred close before pc.Close keeps the normal leave from cutting again.

- timestamp: 2026-11-27T03:20:00Z
  checked: coder/websocket v1.8.15 close.go Close/CloseNow/waitCloseHandshake, conn.go setupReadTimeout, read.go readFrameHeader
  found: Close and CloseNow share casClosing; CloseNow after Close has started only waits for c.closed. Close waits up to 5 s for readMu (held by the viewer's blocked read loop) and then closes. A Read whose ctx is cancelled closes the connection at once (context.AfterFunc -> c.close()), and readFrameHeader arms it for the blocking header read.
  implication: the 2 s ceiling cannot hold today (matches 22-REVIEW-FIX.md:130 and G-04's 6.9 s). A per-viewer read context that the ceiling cancels makes it hold: the blocked read closes the socket, Close sees c.closed and returns.

- timestamp: 2026-11-27T03:25:00Z
  checked: go test -race -run 'TestFanout_JoinLeaveCyclesLeaveNoGoroutines/media_closed_socket_open' before the fix
  found: FAIL goroutine_leak_test.go:53 timed out waiting for viewer gone after its media closed (3.3 s); "viewer left" logged only when the test's cleanup closed the socket
  implication: RED on the reported symptom

- timestamp: 2026-11-27T03:35:00Z
  checked: handler change (Failed and Closed -> cutViewer(1001 "media failed") unless ended), same test, then all 4 leave modes
  found: PASS; all modes return to the post-warm-up goroutine count (16 s for the whole test)
  implication: GREEN; the new mode also pins no residue for this leave

- timestamp: 2026-11-27T03:40:00Z
  checked: TestGuestStop_ThreeSilentPeersDoNotHoldItUp with -v before the ceiling fix
  found: guests joined 19:18:07, left 19:18:12 (about 5 s after the cut); the test passed only because it allowed 8 s
  implication: the 2 s ceiling did not hold (as 22-REVIEW-FIX.md:130 recorded)

- timestamp: 2026-11-27T03:45:00Z
  checked: same test with the bound tightened to 3.5 s after CloseGuests returns, before the ceiling fix
  found: FAIL revoke_test.go:363 timed out waiting for the guests to be gone (4.5 s)
  implication: RED on the ceiling

- timestamp: 2026-11-27T03:50:00Z
  checked: ceiling fix (serveAdmitted's read loop reads on rctx, a child of ctx whose cancel is v.drop; closeSocket calls drop at the ceiling before CloseNow), -race -count=3 with TestGuestViewer_ReconnectIsNotHeldUpByASilentOldSocket
  found: PASS; three silent guests leave about 2 s after the cut (2.9 s test), the replaced silent socket leaves at about 2 s (2.2 s test, was up to 9 s allowed)
  implication: GREEN; writes keep ctx, so a cut never cancels a broadcast's write to another viewer's socket

- timestamp: 2026-11-27T04:10:00Z
  checked: go test -race ./internal/desktop/ ./internal/daemon/ on the full fix; re-run of failures alone; -count=5 of both regression tests
  found: daemon ok; desktop only TestQualityShaped_StillScreenDoesNotCollapseEstimate failed (known flaky, mutation runs were loading the machine), passes alone; both regression tests 5/5
  implication: no neighbour broken

- timestamp: 2026-11-27T04:15:00Z
  checked: manual mutants (M1 Closed case removed; M2 ended guard forced true) against media_closed_socket_open and TestGuestRevoke|TestGuestStop|TestFanout_JoinLeave|TestMediaBinding|TestDesktopTracer
  found: M1 killed (viewer not gone in 3 s); M2 survives
  implication: the Closed branch is pinned; the ended guard only prevents a redundant, client-invisible second cut after our own close (kept per the brief: our own pc.Close must not run a cut)

## Resolution

root_cause: "serveAdmitted's OnConnectionStateChange handler acted on Connected and Failed only. pion closes the daemon's PeerConnection by itself when the viewer sends a DTLS close notice (state Closed), so a viewer whose media closed while its signalling socket stayed open was never ended: it kept counting as a viewer and kept capture running until the socket dropped. Separate cause for the slow end of a silent peer: closeSocket's 2 s ceiling called CloseNow, which in coder/websocket v1.8.15 only waits once Close has started, and Close waits about 5 s for the read lock held by the viewer's blocked read loop."
oracle_type: specified (viewer count 0 and socket closed within 3 s of the client's pc.Close; silent guests gone within 3.5 s of CloseGuests returning, against a 2 s ceiling)
fix: "Failed and Closed both end the session through cutViewer(1001 'media failed'): one close code per viewer (its CAS, so a revoke's 4432 cannot race a 1001), the ordered cut, and the ceiling. An 'ended' flag stored just before serveAdmitted's own deferred pc.Close keeps the normal leave from cutting again (pion runs the handler on a goroutine started inside that Close). Ceiling: serveAdmitted's read loop reads on rctx, a child of ctx whose cancel is the new viewer.drop; closeSocket calls drop at the ceiling, so the blocked read closes the socket at once. Writes keep ctx, so a cut never cancels a broadcast's write to another viewer."
verification:
  red_then_green_closed: "TestFanout_JoinLeaveCyclesLeaveNoGoroutines/media_closed_socket_open FAIL (viewer not gone in 3 s) before, PASS after; all four leave modes return to baseline"
  red_then_green_ceiling: "TestGuestStop_ThreeSilentPeersDoNotHoldItUp with a 3.5 s bound FAIL (guests left about 5 s after the cut) before, PASS after (about 2 s)"
  target_test: { result: pass }
  mutation_check: { result: pass (manual, no Stryker for Go), mutant_killed: "dropping the Closed case -> media_closed_socket_open FAIL; dropping drop() / reading on ctx (= pre-fix) -> ThreeSilentPeers FAIL", note: "the 'ended' guard mutant (if true) survives: a second cut after our own close is invisible to the client (coder/websocket close is idempotent, cutViewer after leave only repeats no-ops); it is kept so our own pc.Close never runs a cut, as the brief requires" }
  no_op_deletion: { result: pass, note: "additive: a new handled state, a flag, a child read context; the removed bare c.Close is replaced by cutViewer, which closes the same socket with the same code under the ceiling" }
  adjacent_tests: { result: pass, suites_run: ["go test -race ./internal/desktop/ ./internal/daemon/ (daemon ok 183 s; desktop: only TestQualityShaped_StillScreenDoesNotCollapseEstimate failed, known flaky, passes alone with TestQualityShaped_SlowViewerDoesNotStallOthers)", "-count=5 of both regression tests", "go vet (default and -tags vmdesktop), gofmt, tests/check-brand.sh"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true, fixed_on_reapply: true, note: "both tests were RED on the unfixed code and GREEN with the fix; the Closed-case mutant re-confirmed RED after the fix" }
  not_verified: "no VM or browser run (not allowed here); production exposure of the zombie needs a page that closes its pc but not its socket (real pages close both)"
  guardrail_verdict: accepted
files_changed: [internal/desktop/manager.go, internal/desktop/revoke.go, internal/desktop/goroutine_leak_test.go, internal/desktop/revoke_test.go]
vm_result: "2026-11-28, build 23.10.3 (5ea587fe), Z-01 RESULT=PARTIAL. Only window.__pc.close() was run on a home Safari viewer, with the page and its socket left open. Stats read viewers 0 at the next 0.25 s poll change; the daemon logged `viewer left` about 0.6 s after the close; the audit leave reason is viewer_closed. The page then reconnected as a new viewer 1.6 s later. The G-04 half (Stop with an unresponsive guest) was not run: the rig has no procedure for an unresponsive guest. Evidence (phase 23 evidence folder): Z-01.txt"
resolved_note: "Resolved 2026-11-28 by the orchestrator: the VM half that can be driven (Z-01, media closed with the socket open) left in about 0.6 s. The unresponsive-guest Stop half cannot be produced by the rig (no way to make a real guest page silent); its 2 s ceiling is covered by TestGuestStop_ThreeSilentPeersDoNotHoldItUp."
