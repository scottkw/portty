---
status: awaiting_human_verify
trigger: "go test -race -count=20 ./internal/daemon/ held 1,059 /dev/ptmx fds in one daemon.test process until kern.tty.ptmx_max (511) was exhausted; later tests failed with `open pty: device not configured`. Is the PTY fd leak in the product (internal/pty, engine, Phase 9 dup/adopt) or only in the tests?"
created: 2026-11-20T20:00:00Z
updated: 2026-11-20T21:25:00Z
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-23
  status: awaiting_human_verify
---

## Current Focus

hypothesis: CONFIRMED. The product releases go-pty's master, the Phase 9 pollable dup and the slave on every session-ending path it owns; the accumulation is test-only: testDaemon users and handoffScene leave live sessions behind, and a live session's hub read keeps its *os.File reachable, so GC never closes it.
test: one -race -count=3 run of ./internal/daemon with lsof sampling of the binary's ptmx fds
expecting: ptmx count stays flat across passes (no per-pass growth)
next_action: none; awaiting human confirmation that a -count=20 run on the host no longer runs out of ptys

reasoning_checkpoint:
  hypothesis: "PTYs accumulate across -count passes because tests (testDaemon users: api/share/attach tests; handoffScene: every handoff test) never KillSession their live sessions; the hub goroutine blocked reading the dup keeps the Session and its files reachable, so neither cleanup nor GC closes them. The product closes all three fds on KillSession (created and adopted), on dismissal after a natural exit, on failed spawn, and Adopt closes the received master."
  confirming_evidence:
    - "TestEngine_PTYFDsReleased (real engine, /dev/fd count incl. revoked slaves) returns to baseline for KillSession, natural exit + dismiss, failed spawn, and export+SCM_RIGHTS+adopt+kill; passes -race -count=3"
    - "Mutants bite: Adopt not closing the received master -> +1 after adopt and after round trip; closePTY not closing the dup -> +5 after KillSession and after dismissal; failed-spawn path not closing -> +15"
    - "Per-test survey (each top-level test alone + forced-GC fd count): every leaker is a testDaemon user (TestAPICreateSession, TestShare_*, TestCLIAttach_*) or a handoffScene test (TestHandoff*); engine_test/natural-exit tests leave nothing"
    - "After the test-cleanup fix, the leaker set run together (-race) ends at baseline (PTYFDS=1, the inherited stdin)"
  falsification_test: "Any product path above leaving a PTY fd open after it returns (the regression test would fail), or ptmx count growing per pass in a -count=3 run after the test fix"
  fix_rationale: "No product change is warranted. Tests now release what they create: testDaemon kills its engine's sessions on cleanup; handoffScene kills every engine's sessions (live ones before its SIGKILL sweep so no exit watcher starts a Wait; ended ones after their watcher marks them stopped, so KillSession's Wait never races ReapNaturalExit's)."
  blind_spots: "Daemon shutdown and the committed old side release by process exit, not tested in-process. Stopped (naturally exited) sessions keep their 3 fds until dismissed by design (GUI dismisses stopped sessions with KillSession, TERM-05); a headless daemon whose shells exit and nobody dismisses keeps one pty pair per stopped card."
  candidate_causes:
    - "code (product): closePTY / Adopt / Create error paths miss a close -> refuted by regression test + mutants"
    - "code (test harness): tests never kill live sessions -> confirmed by survey"
    - "environment: kern.tty.ptmx_max=511 per host shared with a concurrent suite -> contributing only (the ceiling), not a cause of growth"
  and_gate: "yes: growth needs live sessions left behind (test harness) AND those Sessions staying reachable (hub read goroutine), otherwise GC finalizers would close the files; the failure needs that growth AND the 511 ceiling"

## Symptoms

expected: repeated test passes do not accumulate PTYs; each pass releases its sessions
actual: ~100 PTYs per pass accumulate in daemon.test (1,059 /dev/ptmx fds) until ptmx_max (511) is exhausted
errors: "open pty: device not configured" in later tests; TestWaitForDaemon_SlowStart fails after the first pass (slowBuild leaves BuildVersion = "v2")
reproduction: GOTOOLCHAIN=go1.26.11 go test -race -count=20 ./internal/daemon/ ; lsof -p <daemon.test pid> | grep -c ptmx
started: observed at d2998ad2 (Phase 9 added the pollable master dup + adopt path)

## Eliminated

- hypothesis: Phase 9 pollable dup is not closed on kill / dismissal
  evidence: closePTY closes ioFile first; TestEngine_PTYFDsReleased KillSession and natural-exit cases return to baseline; the dup mutant makes both fail
  timestamp: 2026-11-20T20:40:00Z

- hypothesis: Adopt leaks the received master or killAdopted leaks the dup/slave
  evidence: handoff round-trip case: new side holds exactly 2 (dup, slave) after import and 0 after KillSession; the "Adopt keeps master" mutant shows 6 and 4
  timestamp: 2026-11-20T20:40:00Z

- hypothesis: failed spawn leaks the PTY
  evidence: failed-spawn case returns to baseline over 5 attempts; mutant shows +15
  timestamp: 2026-11-20T20:42:00Z

## Evidence

- timestamp: 2026-11-20T20:05:00Z
  checked: internal/pty/session.go closePTY, native.go Create, adopt_darwin.go Adopt, cleanup.go killSession/killAdopted
  found: closePTY closes ioFile (dup), s.slave (adopted only) and go-pty Pty (master+slave) once. Create closes ioFile+p on every error path after the dup. Adopt closes the received master after the dup; caller closes both on dup failure. killSession and killAdopted both call closePTY.
  implication: KillSession path closes all three fds for created and adopted sessions (by reading)

- timestamp: 2026-11-20T20:08:00Z
  checked: engine.go watchSessionExit (natural exit), process.go runDaemonCore shutdown, handoff.go finish/cleanup, closeSessionFiles
  found: natural exit reaps + cancels ctx but does not close the PTY; the stopped card keeps its 3 fds until KillSession (dismiss). Daemon shutdown is api.Stop + Manager().Shutdown + process exit (no PTY close; exit releases). Exporter after COMMIT calls handoffExit(0) (process exit). closeSessionFiles closes HandoffFiles() = dup + slave, which for a created session omits go-pty's original master; it is only called on adopted sessions in product code.
  implication: product ownership is consistent if natural-exit fds are released on dismiss; in-process tests that never KillSession/never exit hold everything

- timestamp: 2026-11-20T20:25:00Z
  checked: fd table probe (fstat, F_GETPATH, F_GETFD) live / stopped / dismissed for one /bin/sh -c 'exit 3' session
  found: live: /dev/ptmx x2 (go-pty master, dup) + /dev/ttys005 (slave). Stopped: masters unchanged; the slave is still open (F_GETFD ok) but fstat and F_GETPATH return EBADF: macOS revoked the tty when the session leader exited. Dismissed: all three closed.
  implication: an fd counter that only counts S_IFCHR misses revoked slaves; the regression test counts open fds that fstat refuses too (scratchpad fd-probe-revoked-slave.txt)

- timestamp: 2026-11-20T20:35:00Z
  checked: TestEngine_PTYFDsReleased -race -count=3 (scratchpad fdleak-product-test-run2.txt)
  found: all four cases pass every pass; 5 stopped sessions hold 15 descriptors until dismissed, 0 after
  implication: no product leak on KillSession, natural exit + dismiss, failed spawn, handoff adopt

- timestamp: 2026-11-20T20:45:00Z
  checked: mutation runs (scratchpad mutant1-adopt-master-not-closed.txt, mutant2-closepty-dup-not-closed.txt, mutant3-failed-spawn-not-closed.txt)
  found: each mutant fails the matching case(s); without a GC-settled baseline, a prior subtest's unreachable leaked files were finalized mid-test and masked the natural-exit case, fixed by ptyFDBaseline (sentinel finalizer, two GC rounds)
  implication: the regression test bites on each close the product must do

- timestamp: 2026-11-20T20:55:00Z
  checked: per-test survey: every top-level test alone + TestZZReport (forced-GC PTY fd count), non-race binary (scratchpad survey-before.tsv; values are leftover-1 because the baseline run had a char-device stdin)
  found: leakers are only testDaemon users (TestAPICreateSession, TestAPIGetSession, TestAPICreateSessionWithArgs, TestAPIListSessionsHostname, TestCreateSession_NoAutoEnable, TestShare_* x16, TestCLIAttach_* x10) and handoffScene tests (TestHandoff*, TestHandoffFault_* x22); about 366 descriptors per pass. Engine tests that let sessions exit leave nothing once GC runs (their goroutines end, the files become unreachable).
  implication: test-only leak; live sessions stay reachable through the hub's blocked read and the exit watcher, so GC cannot reclaim them

- timestamp: 2026-11-20T21:00:00Z
  checked: first cleanup attempt (kill all sessions in handoffScene after the SIGKILL sweep) under -race
  found: DATA RACE in TestHandoff_SIGTERMAfterCommitKeepsSocket/before_COMMIT: serveDaemon's Manager().Shutdown() closes hub.Done, watchSessionExit takes it for a natural exit and blocks in cmd.Wait on the live shell; KillSession's own cmd.Wait then races it
  implication: cleanup must kill live sessions before any shell dies, and for ended hubs wait until the watcher marks them stopped (killSessions liveOnly / killAllSessions)

- timestamp: 2026-11-20T21:05:00Z
  checked: leaker set after the fix, -race -count=1 together with TestZZReport (scratchpad leakers-after-fix-race-count1.txt)
  found: PASS, PTYFDS=1 (baseline) at the end
  implication: tests no longer leave PTYs behind

- timestamp: 2026-11-20T21:06:00Z
  checked: slowBuild fix, -race -count=2 of TestWaitForDaemon_SlowStart + the two slow parallel tests (scratchpad slowbuild-after-fix-count2.txt)
  found: all pass on both passes, no race
  implication: BuildVersion is restored per pass

## Resolution

root_cause: Test-only leak. testDaemon (api, share, attach tests) and handoffScene (all handoff tests) never killed the sessions they created; each live session's hub read goroutine keeps its *os.File values reachable, so its go-pty master, pollable dup and slave stayed open for the life of daemon.test and accumulated per -count pass until kern.tty.ptmx_max (511). Separately, slowBuild left BuildVersion = "v2" after the first pass.
fix: testDaemon registers killAllSessions on cleanup; handoffScene.cleanup kills every engine's sessions (live first, then SIGKILL sweep, then ended ones after their exit watcher finishes); slowBuild sets BuildVersion after t.Parallel and restores it when the last of its two tests ends. New regression test TestEngine_PTYFDsReleased. No product change.
verification: one -race -count=3 run of the whole ./internal/daemon package (race-built test binary run from the package dir), lsof sampled every 3 s (scratchpad final-count3-ptmx-samples.txt, final-count3-output.txt): PASS, exit 0, 1173 PASS lines, 3 SKIP (env-gated TestHandoffIT), no DATA RACE, no "device not configured"; the binary's /dev/ptmx count never went above 10 (peak during pass 1), stayed at 0..4 for passes 2 and 3, and was 0 at the end; TestWaitForDaemon_SlowStart passed on all 3 passes. Before the fix: about 100 PTYs per pass, 1,059 ptmx fds at exhaustion. Regression test bites on 3 mutants (see Evidence).
oracle_type: specified (descriptor count returns to its pre-session baseline)
files_changed: [internal/daemon/engine_fdleak_test.go, internal/daemon/api_test.go, internal/daemon/handoff_test.go, internal/daemon/process_test.go, TESTING.md]
