---
status: awaiting_human_verify
trigger: "U-load FAIL (23-VM-CHECKS.md notes item 9, evidence/U-load-encoder.txt): under extreme load in a macOS 27.2 Tart VM the VideoToolbox encoder service (ParavirtualizedVideoEncoder in VTEncoderXPCService) failed changes (-12902, -90004, -19092) and then refused every new encoder: every viewer join logged capture start failed (starting the encoder failed, code -12902), join failed, viewer left, after the load stopped too. sudo pkill -9 VTEncoderXPCService fixed joins on the same daemon pid. The daemon has no recovery; restarting the daemon is not acceptable (it hosts the terminal sessions)."
created: 2026-11-28T03:44:00Z
updated: 2026-11-28T05:05:00Z
---

## Current Focus

bug_class: Mandelbug in the environment (the OS encoder service wedges under extreme load, not reproducible on demand); the missing recovery in the daemon is a deterministic Bohrbug (no code path ever handles a service that keeps refusing). SBFL not run (no failing test existed; the defect is an absent path)

reasoning_checkpoint:
  hypothesis: "Joins stay broken because (1) the VideoToolbox encoder service instance that launchd runs for this daemon (VTEncoderXPCService, ServiceType Application) is wedged and refuses every new session with -12902, and (2) the daemon never does anything but retry the same request: noteStartFailure only classifies -3801/-3815, and an encoder failure is an untyped error. Ending the daemon's own instance makes launchd start a fresh one on the next request (U-load step 3b), so a policy that counts consecutive encoder start failures and ends that instance at the third restores joins on the same daemon pid."
  confirming_evidence:
    - "U-load: every join from 03:12:33 to 03:20:29 failed with code -12902, with the load gone (load 2 at 03:18); the old service logged an error on each create"
    - "U-load step 3/3b: sudo pkill -9 VTEncoderXPCService (91894 -> 10381) and the next join streamed on the same daemon pid 91582: no daemon state needed clearing"
    - "VTEncoderXPCService Info.plist: ServiceType Application, so the daemon has its own instance; the responsibility call identifies a service's client without root (host probe on VTDecoderXPCService)"
    - "code: every start goes through Manager.startSource; nothing above screencap can tell an encoder failure from another start failure"
  falsification_test: "VM: reproduce the wedge (U-load); if after the policy's reset line (naming a pid that was the daemon's service) the next join still fails with -12902, the wedge is not (only) in the service process and the hypothesis is wrong. Also wrong if the lookup finds no service while one serves the daemon (responsible pid is not the daemon in a LaunchAgent)."
  fix_rationale: "Typing the encoder failure (ErrEncoderStart, via an out flag from pc_stream_start) lets the Manager count it; the policy (3 consecutive, at least 1 min between resets, a good start clears) runs in Go on the one start path; the action ends only a VTEncoderXPCService* process whose responsible pid is this process, never while this process has a running encoder (gEnc), and only after an encoder start failed. It recovers the OS service the evidence shows wedged, without restarting the daemon (terminal sessions survive)."
  blind_spots: "Not reproduced in the VM with the fix (needs the U-load wedge). Not known: whether a real Mac's hardware encoder service can wedge the same way; whether a wedge could sit below the service (kernel/paravirtual device), where a reset would not help; whether VideoToolbox's own software fallback (spec prefers, does not require, hardware) already runs in the same service. Responsibility attribution for a LaunchAgent's XPC service is inferred (ExtensionKit sample), not observed for the daemon."
  candidate_causes:
    - "environment: wedged per-client VTEncoderXPCService instance (ParavirtualizedVideoEncoder) after load-induced failures - CONFIRMED by the kill"
    - "code: daemon has no recovery for an encoder that keeps refusing (untyped error, no escalation) - CONFIRMED by reading"
    - "code: daemon-held state (a leaked PCEncoder or session) blocking new sessions - ruled out: same daemon pid recovered after only the service was killed; pc_stream_start allocates a fresh PCEncoder per start and failed starts hold nothing (gStream stays nil)"
    - "data/config: wrong encoder parameters (-12902 is kVTParameterErr) - ruled out: identical parameters (2560 long edge, 30 fps, 8 Mbps) streamed before the wedge and right after the kill"
  and_gate: "yes: the user-visible failure needs both the wedged service (environment) and the absent recovery (code). The fix removes the code half; the environment half is not preventable from the daemon."
next_action: VM re-check M-222 (TESTING.md): deploy a build with this fix to portty-test, confirm before the wedge that the daemon's VTEncoderXPCService has the daemon as its responsible pid (sudo launchctl procinfo), reproduce U-load, and watch for the reset line after the third failed start and a streaming join after it on the same daemon pid

## Symptoms

expected: a viewer join after the encoder service misbehaved under load streams again (at worst after a short recovery), with no one at the Mac and no daemon restart
actual: from 03:12:33 every join logged `capture start failed err="screencap: starting the encoder failed (code -12902)"`, `viewer left`, `join failed`; the page looped on reconnecting; persisted with the load gone (03:20, load 2); fixed at once by `sudo pkill -9 VTEncoderXPCService` (pid 91894 -> 10381), same daemon pid 91582
errors: picture size change failed (rebuilding the encoder failed, code -12902); frame rate change failed (code -90004); still refresh failed (refining the picture failed, code -19092); capture error (encoder output callback failed, code -19092); bitrate change failed (code -19092); then capture start failed (starting the encoder failed, code -12902) on every join
reproduction: VM row U-load: portty-test (macOS 27.2 Tart guest, paravirtualized encoder) under guest load average 100-160 with a viewer streaming, joins repeated; wedge seen once (03:08-03:12)
started: first seen 2026-11-28 23.10.3 re-check (U-load); no recovery path has ever existed

## Eliminated

## Evidence

- timestamp: 2026-11-28T03:44:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entry matches (encoder, VideoToolbox, -12902, XPC service)
  implication: no known-pattern candidate

- timestamp: 2026-11-28T03:50:00Z
  checked: media_darwin.m pc_stream_start (856-960), PCEncoder createSessionLocked (196-235); internal/desktop manager.go join/startSource/restartSource, runtime_state.go noteStartFailure, watchdog.go ensureSource/restartForStall
  found: pc_stream_start resolves the surface, then makes the encoder (startWidth -> createSessionLocked: VTCompressionSessionCreate with EnableHardwareAcceleratedVideoEncoder YES (preferred, not required), properties, PrepareToEncodeFrames) and only then the SCStream; an encoder failure returns the OSStatus with the message "starting the encoder failed" and Go formats it through mapSCKError as a plain error, so nothing above can tell it from any other start failure. Every capture start (first join, stall restart, wake, ensureSource retry every 10 s while a viewer waits) goes through Manager.startSource -> src.Start -> noteStartFailure, which only classifies -3801 and -3815. A join whose start fails calls leave, so with no viewer kept nothing retries: only the page's reconnect drives new starts (3-12 s apart in U-load). No code path ever touches the encoder service.
  implication: one choke point (startSource) sees every start outcome; the encoder failure needs a typed error to be counted

- timestamp: 2026-11-28T03:55:00Z
  checked: VideoToolbox.framework XPCServices/VTEncoderXPCService.xpc Info.plist (host, macOS 27.2.1); host process table; responsibility_get_pid_responsible_for_pid on the host's running VTDecoderXPCService (scratch probe, read-only)
  found: XPCService ServiceType = Application (one instance per client application; VTEncoderXPCServiceZonto.xpc likewise); the service runs as the user (uid 501, ppid 1), so parentage says nothing; the responsibility call needs no root and returned the client for the decoder service (15284 -> WallpaperAerialsExtension 898, an ExtensionKit process responsible for itself). A process started from a terminal is attributed to the terminal (probe self -> 871). The daemon is primarily a LaunchAgent (internal/daemon/launchd.go, its own responsible process); the fallback detached spawn (process_unix.go) does not disclaim responsibility.
  implication: this daemon's encoder service is the VTEncoderXPCService* process whose responsible pid equals the daemon pid; another app's instance never matches; in the detached-spawn fallback nothing matches (no recovery, but no wrong kill)

- timestamp: 2026-11-28T03:58:00Z
  checked: U-load evidence after the kill (step 3b): unified log
  found: the relaunched service (pid 10381) served the same client pid 91582 with ParavirtualizedVideoEncoder, the join streamed (framesDecoded 70, 2560 wide); the old service had logged an error on every create (com.apple.coremedia:VTCompression) from 03:20:22
  implication: the wedge lives in the service process, not in the daemon or the paravirtualized device; ending the daemon's own instance is sufficient and needs no daemon restart

- timestamp: 2026-11-28T04:05:00Z
  checked: RED: new tests before any fix (stubs only): internal/desktop encoder_recovery_test.go (policy), encoder_wedge_test.go (real start path through the rig), internal/screencap encoder_service_darwin_test.go (typing, lookup)
  found: all policy tests fail ("reset false, want true" at the third failure); TestEncoderWedge_ResetsOwnServiceAndRecovers fails ("Status.Reason \"\" after an encoder start failure"); TestEncoderWedge_ResetFindsNothingIsLogged times out; TestStartError_EncoderIsTyped fails ("want ErrEncoderStart"); TestOwnEncoderServices_ReadOnly fails ("not implemented")
  implication: the tests reproduce the missing recovery in code

- timestamp: 2026-11-28T04:20:00Z
  checked: candidates (a) end the daemon's own service, (b) software encoder fallback, (c) both, against the evidence and the code
  found: (a) is the action the evidence shows works (step 3b). (b) is unproven: createSessionLocked already asks for hardware as preferred, not required (EnableHardwareAcceleratedVideoEncoder YES, no RequireHardware), so VideoToolbox may already fall back to its software encoder when the hardware one cannot be created; whether the software encoder is hosted in the same wedged service process is unknown (both services carry one sandbox profile, VTEncoderSandboxProfile); it would also put a 2560-wide 30 fps software encode on a CPU already at load 100+. A forced software retry would add ObjC and a second session path for a benefit no evidence shows.
  implication: (a) alone; (b) stays a follow-up if the VM re-check shows -12902 surviving a reset

- timestamp: 2026-11-28T04:30:00Z
  checked: GREEN: the fix (screencap ErrEncoderStart + encoderFailed flag from pc_stream_start; pc_encoder_service_pids + ResetEncoderService; desktop encoderRecovery policy, Manager.encoderOutcome called from startSource, Deps.ResetEncoder defaulting to screencap.ResetEncoderService) and the new tests
  found: all 7 desktop tests and 4 screencap tests pass (screencap ones never run the encoder or signal anything: the start-error mapping, a source check, the refusal under tests, and the read-only lookup)
  implication: the policy, its wiring on the real start path, the reason in Status, the log lines and the error typing behave as designed

- timestamp: 2026-11-28T04:35:00Z
  checked: the lookup's mechanics on the host, read-only (scratch copy of the same loop with the prefix and the "self" pid as arguments) against the running VTDecoderXPCService
  found: 962 pids listed, 1 under the prefix; matched with self = its client (898), not matched with self = 871
  implication: proc_listallpids + proc_pidpath prefix + responsibility call pick exactly the client's instance and nobody else's

- timestamp: 2026-11-28T04:40:00Z
  checked: mutation run (scratch mutate.py, one mutant at a time, targeted tests)
  found: 14 of 15 killed: threshold 2, success keeps the count, every error counts, no backoff, clock step blocks, reset keeps the count, encoderOutcome not called from startSource, no reason on failure, reason not cleared on success, default ResetEncoder not wired, startError ignores the flag, ObjC flag dropped, ObjC lookup without the gEnc guard, ObjC lookup without the responsible check. Survived: the missing-SPI code -1 mapped to 0 (only the log wording differs when the call is absent; not reachable on a macOS that has it)
  implication: every behavioral line is pinned by a test; the survivor is diagnostic only

- timestamp: 2026-11-28T04:45:00Z
  checked: nm -u and strings on a wailsassets build of the app
  found: dlsym, proc_listallpids, proc_pidpath are imports; responsibility_get_pid_responsible_for_pid is only a string (runtime lookup)
  implication: if Apple removes the private call, the daemon still loads; recovery logs that it cannot tell which service is its own

- timestamp: 2026-11-28T05:00:00Z
  checked: go test -race ./internal/desktop/ (full) and ./internal/daemon/ (full); go test -race ./internal/screencap/ with -skip 'TestEncoder_|TestEncoderRate_|TestEncoderRefine_|TestOpus_' (no encoder on the host); new tests -race -count=10; go vet ./... and go vet -tags vmdesktop ./...; gofmt -l; go build of the app (default, vmdesktop, wailsassets, wailsassets vmdesktop; macOS 14 target; wailsassets with a throwaway frontend/dist placeholder, removed); tests/check-traceability-paths.sh; tests/check-brand.sh
  found: desktop ok 188.6 s, daemon ok 133.4 s, screencap ok 1.2 s, no race; new tests 10/10; vet, gofmt clean; all four builds link; traceability OK (343 paths); brand gate clean
  implication: no neighbor broken; none of the known load flakes showed

## Resolution

root_cause: "AND-gate. (1) environment: under extreme load the VideoToolbox encoder service instance launchd runs for the daemon (VTEncoderXPCService, ServiceType Application, here hosting ParavirtualizedVideoEncoder) wedged and refused every new session (-12902) until that process ended; the daemon held no bad state (the same pid streamed right after the service was killed). (2) code: the daemon had no recovery for an encoder that keeps refusing: pc_stream_start's encoder failure reached Go as an untyped error, noteStartFailure only classified -3801/-3815, and every later start (each viewer rejoin, each 10 s retry) asked the same wedged service again, forever."
fix: "screencap: pc_stream_start reports an encoder failure through a new out flag and Start returns it as screencap.ErrEncoderStart (same text as before: 'screencap: starting the encoder failed (code N)'); new ResetEncoderService SIGKILLs only the VTEncoderXPCService* process whose responsible pid is this process (pc_encoder_service_pids: proc_listallpids + proc_pidpath under VideoToolbox's XPCServices + responsibility_get_pid_responsible_for_pid looked up with dlsym), refusing while this process has a running encoder (gEnc) and under tests. desktop: encoderRecovery (guarded by smu) counts consecutive ErrEncoderStart starts, a good start clears, other failures neither count nor clear, and it says reset at the third, at most once a minute (a clock stepped back never blocks); Manager.encoderOutcome runs it from startSource right after Start (no source runs, no other start can begin), puts 'encoder: ...' in Status.Reason (GET /desktop reason) until a start works, and logs one line per reset: ended pids, not found, or failed. Deps.ResetEncoder is the seam, defaulting to screencap.ResetEncoderService. The daemon is never restarted. Software fallback (candidate b) not added: unproven against this wedge and VideoToolbox may already fall back on its own (hardware preferred, not required)."
verification:
  target_test: { result: pass, detail: "RED before the fix (policy: 'reset false, want true'; rig: no Status.Reason, no not-found line; screencap: untyped error, lookup missing); GREEN after: internal/desktop encoder_recovery_test.go (5), encoder_wedge_test.go (2, one with 2 subtests), internal/screencap encoder_service_darwin_test.go (4); -race -count=10 of the desktop ones: ok" }
  mutation_check: { result: pass, detail: "14 of 15 mutants killed (threshold, success clear, error filter, backoff, clock-step guard, reset clears count, call from startSource, reason set, reason cleared, default seam wired, startError flag, ObjC flag, ObjC gEnc guard, ObjC responsible check). Survivor: the missing-SPI code -1 -> 0, which only changes the log wording when the private call is absent and cannot be reached on a macOS that has it" }
  no_op_deletion: { result: pass, deletion_justified_by_rca: true, detail: "additive only; no assertion or existing behavior removed (Start's error text for an encoder failure is unchanged)" }
  adjacent_tests: { result: pass, suites_run: ["go test -race ./internal/desktop/ : ok 188.6 s", "go test -race ./internal/daemon/ : ok 133.4 s", "go test -race -skip encoder/Opus ./internal/screencap/ : ok 1.2 s", "go vet ./... and -tags vmdesktop: clean", "gofmt -l: clean", "go build . (default, vmdesktop, wailsassets, wailsassets vmdesktop): ok", "tests/check-traceability-paths.sh: OK", "tests/check-brand.sh: clean"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true, fixed_on_reapply: true, detail: "the RED run is the reverted state (stubs only): every new test failed; the mutant that drops the startSource call (the wiring revert) fails the rig test" }
  guardrail_verdict: accepted
not_verified: "No real wedge with the fix (VM M-222 pending): whether the reset alone clears a wedged ParavirtualizedVideoEncoder service as the manual pkill did, and whether a LaunchAgent daemon's encoder service reports the daemon as its responsible pid (inferred from the ExtensionKit sample on the host). Not covered: a wedge where sessions are created but every output fails (-19092) without starts failing (no frames, the watchdog restarts; if starts keep succeeding the policy never counts); a daemon started by the detached-spawn fallback (not its own responsible process: the lookup finds nothing and logs that); a pid reused between the lookup and the kill (microseconds, same user)."
oracle_type: specified (U-load evidence: the manual kill of the daemon's encoder service restored joins on the same daemon pid; the policy's thresholds are this fix's contract)
commit: 7af74813
files_changed: [internal/screencap/screencap.go, internal/screencap/media_darwin.go, internal/screencap/media_darwin.m, internal/screencap/encoder_service_darwin_test.go, internal/desktop/encoder_recovery.go, internal/desktop/manager.go, internal/desktop/encoder_recovery_test.go, internal/desktop/encoder_wedge_test.go, TESTING.md]
vm_recheck: "M-222 on portty-test (macOS 27.2, paravirtualized encoder), build with this fix (tags wailsassets vmdesktop). Before the wedge: after one streamed join, `pgrep -f VTEncoderXPCService` gives one pid; `sudo launchctl procinfo <pid>` shows the responsible pid = the daemon pid (if not, recovery will log 'no encoder service of this process was found': record and stop). Reproduce U-load (guest load 100-160 with a viewer streaming) until joins fail with `capture start failed err=\"screencap: starting the encoder failed (code -12902)\"`; stop the load; keep the home page open, kill nothing by hand. Expect in daemon.log: `desktop: encoder failed err=...` once, `capture start failed` x3 (each followed by `join failed` when the start came from a join), then `desktop: the encoder keeps failing to start; ended this process's encoder service so macOS starts a fresh one pids=[<old service pid>]`; `pgrep` then shows a new pid (after the next start); the next join logs `viewer joined` and the page streams (framesDecoded rising); GET /desktop reason contains 'encoder:' while it fails and is empty after; the daemon pid is unchanged. If -12902 continues after the reset line, the reset did not clear it: record whether the new service pid differs, and that is the trigger to try candidate b (software fallback). No second reset may appear within a minute of the first."
vm_result: "2026-11-28, build 23.10.4 (9ee3d660, carries 7af74813), EW-01 RESULT=PARTIAL. Done: responsible-pid premise holds: VTEncoderXPCService 17907 has responsible pid 17724 = the daemon (sudo launchctl procinfo), responsible path the Portty binary. Wedge reproduced in about 3.5 min at load average 204-233 (motion plus two yes loops plus lock/unlock cycles): 04:53:03 still refresh -19092, 04:53:55 capture error -19092, 04:54:08 viewer left, 04:54:09 `desktop: encoder failed ... (code -12902)` and one `capture start failed`; GET /desktop reason `encoder: screencap: starting the encoder failed (code -12902)` from then on (still at 04:57:43). Not observed: the 2nd and 3rd start failures, the reset line, a new service pid and a recovered join. Reason: the office VM's tailscaled exited at 04:53:56 under the load (launchd last exit 1, not restarted), so no join reached the daemon after the first failure, and the daemon does not retry a start with no viewer; count stayed at 1, service pid still 17907, daemon pid unchanged. Run stopped by the off-tailnet rule; nothing killed by hand. Evidence: phase 23 evidence/EW-01.txt; row EW-01 in 23-VM-CHECKS.md"
