---
status: awaiting_human_verify
trigger: "go test -race -count=10 -run '^TestShare_DeadViewerReapedBacklogDelivered$' ./internal/webserver/ fails 4 of 10 on gsd/v1.2 (a4a81f1e): input_queue_test.go:175: input idle at the reap, want the backlog still queued behind the blocked PTY. Is a reaped viewer's queued paste discarded (product bug, INPT-01) or is it a test race?"
created: 2026-11-21T00:00:00Z
updated: 2026-11-21T16:40:00Z
---

## Current Focus

hypothesis: test race, not product. The test builds the 8 MiB paste (pastePayload, a byte loop that takes tens to 150+ ms under -race) AFTER joinPasteViewer returns. The viewer reads nothing in that time, so the 50 ms / 100 ms heartbeat can reap it before the first input frame is sent; nothing was ever queued, so WaitInputIdle(0) is idle (inBytes == 0).
bug_class: Heisenbug (timing: payload build time vs a 150 ms heartbeat window); SBFL not run (single flaky test, flaky spectrum)

reasoning_checkpoint:
  hypothesis: "The failing runs reap the silent viewer while the test is still building the paste, so the server read zero input frames and the hub input is idle because it never held anything. The reap path itself (Heartbeat -> CloseNow -> read pump returns -> InputConn.Close) never discards queued input."
  confirming_evidence:
    - "Instrumented 20 runs (dvb-instr1.log): both failures show join, replay, 'heartbeat timeout' and 'leave' logged BEFORE 'DBG paste start' (logged right after pastePayload()), with clientSent=0 and queued=0 at the check, 82-112 us after paste start"
    - "All 18 passing runs reaped 40-92 ms after paste start with 8-49 frames sent and 163,835-950,243 bytes queued, i.e. pastePayload had already eaten ~60-110 ms of the ~150 ms ping+timeout window"
    - "Code: the reap path never calls revokeInput. Heartbeat calls c.CloseNow(); the read pump's conn.Read errors, deferred in.Close() only marks the conn closed (input.go:112-127); Unsubscribe touches subscribers/owner only. revokeInput (the CR-01/CR-02 discard) is called only by RevokeWebViewers, whose only callers are daemon/share.go:350 and :367 (Stop Sharing / disable)."
    - "In the 18 passing runs the rest of the test (sink gets frames*32767 bytes, in-order prefix of the paste, input idle) passed: the backlog of a reaped viewer is delivered whole on current code"
  falsification_test: "If, with the payload built before the join, a run still shows 'input idle at the reap' while the server had read frames (input total frames > 0), the product discards on reap and this hypothesis is wrong"
  fix_rationale: "The defect is the unanswered-ping window containing the payload build; building it before joinPasteViewer leaves only one frame's write/read (about a millisecond) inside a >= 150 ms window. Checking the setup precondition (the server read frames) before the idle check means a residual setup stall can never again be reported as a product discard; the idle sample stays valid there because nothing reads the PTY pipe until the sink starts."
  blind_spots: "Residual: a >= 150 ms stall of local goroutine/socket work between the join and the first queued frame would still fail the test (now as 'the read pump never logged its input total', a setup failure, not a product one). Not eliminated without a product test hook. The VM reap row (R-05) ran at 35aaa035, before the review fixes; current-code reap delivery is evidenced by unit runs, not a VM rerun."
  candidate_causes:
    - "code (test): payload build inside the silent-viewer heartbeat window - CONFIRMED"
    - "code (product): reap routed through the CR-01/CR-02 revokeInput discard - ruled out by code reading and the passing runs' full delivery"
    - "code (product): hub input dead (inDead) at the reap - ruled out: no 'input writer stopped' line for the iteration before the check; the late 'input writer stopped ... queued=0' lines are prior iterations' manager.Shutdown cleanup"
    - "environment: host load stretching the build - an amplifier only; the quiet host still failed 2/20"
  and_gate: "yes: needs the payload build inside the window (cause) and -race slowing that byte loop to ~150 ms (amplifier); without -race the build is a few ms"
next_action: none in code; caller to confirm the verdict (test race, no product change) and merge the worktree branch

## Symptoms

expected: the test passes every run: a silent viewer is reaped while its paste backlog is queued behind a blocked PTY, then every frame the server read reaches the PTY in order
actual: 4 of 10 runs fail at the idle check right after the reap (quiet host, 2 s run)
errors: input_queue_test.go:175: input idle at the reap, want the backlog still queued behind the blocked PTY
reproduction: export GOTOOLCHAIN=go1.26.11; go test -race -count=10 -run '^TestShare_DeadViewerReapedBacklogDelivered$' ./internal/webserver/
started: passed in 10-03 and later gates; flaky now, possibly after the Phase 10 review fixes (CR-01, CR-02, WR-01, WR-04, WR-05, IN-09)

## Eliminated

- hypothesis: product bug - a heartbeat reap discards the reaped viewer's queued input through the CR-01/CR-02 revoke path
  evidence: reap = relay.Heartbeat -> c.CloseNow() (heartbeat.go:39); the read pump returns on the Read error and its deferred in.Close() only sets closed/backlogged (input.go:113-127); Unsubscribe (hub.go:354) does not touch the input FIFO. revokeInput (input.go:141) is called only from RevokeWebViewers (hub.go:472), called only from daemon/share.go:350/:367. In the instrumented run every reap that happened after frames were queued left 163 KB-950 KB queued and the whole backlog was then delivered in order.
  timestamp: 2026-11-21T16:05Z
- hypothesis: hub input died (inDead) before the check, so WaitInputIdle reports idle
  evidence: failing iterations log no 'input writer stopped' before their check; queued=0 with clientSent=0 means nothing was ever queued, not that it was dropped
  timestamp: 2026-11-21T16:05Z
- hypothesis: the review fixes (CR-01 e3e2eb9b, CR-02 973af88b, WR-01 be893a7b, WR-04 ac85ae30, WR-05 4d32b433, IN-09 4df30b42) made the test flaky
  evidence: none of them touch the reap path or the test; the failure is the test's own ordering (pastePayload after the join), present since the test was written in 6d8ecb87. Earlier gates passed on timing luck.
  timestamp: 2026-11-21T16:05Z

## Evidence

- timestamp: 2026-11-21T15:55Z
  checked: reap path vs revoke path (internal/relay/heartbeat.go, internal/webserver/server.go:932-1026, internal/relay/hub.go:354-480, internal/relay/input.go)
  found: Heartbeat on an unanswered ping calls CloseNow and returns true; the share handler logs 'heartbeat timeout'. The read pump's deferred in.Close() keeps queued frames ("queued frames are still delivered after the socket goes"). WritePump returns on readDone; deferred Unsubscribe. The only discard of queued web input is revokeInput, reached only from RevokeWebViewers (Stop Sharing / share disable in daemon/share.go). killInputLocked drops the queue only on PTY write error or hub.done.
  implication: by code, a reaped viewer's backlog is kept and delivered; only revoke discards (matches INPT-01)
- timestamp: 2026-11-21T16:00Z
  checked: instrumented copy of the test (timestamps in the log, counted client writes, queued/clientSent/elapsed printed at the idle check), go test -race -count=20 -v; output dvb-instr1.log in the scratchpad
  found: 2/20 failed. Both failures: join, replay, heartbeat timeout, leave all logged before 'DBG paste start' (which is right after pastePayload()); at the check queued=0 clientSent=0, 82-112 us after paste start. 18 passes: reap 40-92 ms after paste start, clientSent 8-49 frames, queued 163,835-950,243 bytes, then full in-order delivery.
  implication: the viewer is reaped while the test builds the paste; no frame was ever sent or queued. Test race, not product discard.
- timestamp: 2026-11-21T16:05Z
  checked: git history of input_queue_test.go
  found: last changed in c054b350/6d8ecb87 (10-03); payload has always been built after joinPasteViewer with a 100 ms heartbeat timeout
  implication: latent since the test was written; not introduced by the review fixes
- timestamp: 2026-11-21T16:05Z
  checked: 10-VM-CHECKS.md R-05 (second pass, 35aaa035) vs commits after it
  found: R-05 PASS (reap with a non-empty backlog, drained in full, exact prefix) ran before the review fixes. The review fixes after 35aaa035 (CR-01, WR-01, WR-02, WR-04, IN-02, IN-03, CR-02, WR-05, IN-09, RF-03) do not touch the reap path; CR-01/CR-02 discard only from RevokeWebViewers.
  implication: R-05's evidence still describes the current reap behavior; no product change needs a VM rerun on this account
- timestamp: 2026-11-21T16:20Z
  checked: fixed test, go test -race -count=50 of both share paste tests (dvb-targeted50.log)
  found: ok, 100 runs, 143.6 s, no failure
  implication: the flake is gone with the payload built before the join (was 2/20 and 4/10)
- timestamp: 2026-11-21T16:40Z
  checked: go build ./... && go vet ./... (dvb-buildvet.log); go test -race -short -count=1 ./... (dvb-full-short.log)
  found: build and vet clean; all 13 packages ok
  implication: no neighbor broken; the change is test-only

## Resolution

root_cause: test race in TestShare_DeadViewerReapedBacklogDelivered: pastePayload() (8 MiB byte loop, ~60-150+ ms under -race) runs after joinPasteViewer, inside the silent viewer's 50 ms ping + 100 ms timeout window, so the heartbeat sometimes reaps the viewer before it sends a single frame; with nothing ever queued the input is trivially idle. Not a product bug: the reap path never reaches revokeInput.
fix: internal/webserver/input_queue_test.go TestShare_DeadViewerReapedBacklogDelivered builds the paste before joinPasteViewer (so the silent viewer's heartbeat window holds only the send), and checks the setup precondition (the read pump's input total line, frames > 0) before the idle sample, so a residual setup stall reports as setup, never as a discard. Assertions unchanged in content: reaped by the heartbeat, backlog still queued at the reap, every frame the server read reaches the PTY whole and in order, input idle after. No product change.
verification:
  target_test: { result: pass, detail: "-race -count=50 of TestShare_DeadViewerReapedBacklogDelivered and TestShare_PasteBacklogKeepsViewer: ok, 143.6 s (dvb-targeted50.log)" }
  mutation_check: { result: skipped, reason_if_skipped: "no Go mutation tool configured; a manual product mutant would cost another test binary (a firewall alert each). By reasoning: a reap that dropped queued frames leaves the in-flight frame counted, so the delivery check (sink == frames*32767) fails; a reap that killed the input (inDead) fails the idle check. The assertions are the ones 6d8ecb87 wrote RED against the pre-FIFO code, only reordered." }
  no_op_deletion: { result: pass, deletion_justified_by_rca: true, detail: "no assertion removed or weakened: one statement moved earlier, one check moved after the setup precondition; justified by the window analysis above" }
  adjacent_tests: { result: pass, suites_run: ["go build ./... && go vet ./... : ok (dvb-buildvet.log)", "go test -race -short -count=1 ./... : all 13 packages ok, webserver 43.7 s, relay 459.5 s (dvb-full-short.log)"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true, fixed_on_reapply: true, detail: "original ordering: 4/10 (report) and 2/20 (dvb-instr1.log, same ordering, instrumented); fixed ordering: 0/50" }
  guardrail_verdict: accepted
files_changed: [internal/webserver/input_queue_test.go]
