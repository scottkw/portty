---
status: awaiting_human_verify
trigger: "Goroutine leak in the daemon desktop share: GET /desktop/stats showed 32 goroutines at daemon start and 1020 with no viewer connected after about 25 viewer joins and leaves over cross-VM WebRTC (relay path, dummynet, quality ladder stepping); ~16-20 per join/leave plus slow idle drift. No goroutine dump taken. (23-09-SUMMARY finding 4)"
created: 2026-11-27T00:00:00Z
updated: 2026-11-27T02:20:00Z
---

## Current Focus

hypothesis: CONFIRMED (mechanism) - the daemon goroutine growth is the desktop health poller, not viewer join/leave. Every 10 s while the desktop is on, desktopTick -> desktopTailnet -> webserver.CheckHealth builds a fresh tailscale local.Client, whose private http.Transport parks one idle keep-alive LocalAPI connection that tailscaled never closes: +2 goroutines (persistConn readLoop/writeLoop) and +1 socket per tick, forever.
bug_class: Bohrbug (deterministic, time-driven: one leaked connection per health check)
test: static guard over the repo's non-test Go files (no local.Client value built inside a function) RED before / GREEN after; mechanism test against a fake LocalAPI (shared client 1 connection for 20 calls, client-per-call 20); join/leave cycle test stays as the H-03 guard that no per-join residue exists
expecting: guard fails on tailscale.go:76, tailnet.go:96, share.go:45 before the fix and passes after
next_action: human verify in the VM: daemon with always-available on, no viewer, GUI closed, read GET /desktop/stats goroutines at t=0 and t=10 min (pre-fix build: about +120; fixed build: flat within a few)

reasoning_checkpoint:
  hypothesis: "CheckHealth (internal/webserver/tailscale.go:76) builds a new local.Client per call; local.Client lazily builds its own http.Transport (tailscale client/local/local.go:140-146); after the two LocalAPI requests the connection parks idle in that transport, and since tailscaled's LocalAPI http.Server has no IdleTimeout (ipn/ipnserver/server.go:533-540) it is never closed, so its readLoop and writeLoop goroutines stay rooted. The daemon calls CheckHealth every 10 s (desktopPollEvery) while the desktop is on, giving +2 goroutines per 10 s."
  confirming_evidence:
    - "Fake LocalAPI experiment (httptest, Dial+OmitAuth so nothing real is contacted): 20 checkHealth calls with a fresh local.Client each: goroutines 3 -> 63 (+3 per call in-process: 2 client persistConn + 1 server conn.serve; +2 in the daemon where tailscaled is another process). One shared client: 3 -> 6 total."
    - "VM finding text: 'about 2 more every 10 s while idle' = exactly one leaked connection per 10 s tick; (1020-32)/2 = 494 ticks = ~82 min of desktop-on time in a ~2 h session; 89 with one viewer = 32 + ~31 harness per-viewer cost + ~24 (~2 min of ticks)."
    - "Join/leave cycles leave nothing: desktop harness 5 leave modes (clean, abrupt ws, late pc, black-holed media path then ws, black hole + late pc) and the daemon-level harness (5 cycles through real routes/hooks/stats) all return to baseline."
  falsification_test: "If a daemon running the fix with always-available on (and no viewer, no GUI peer polling) still gains goroutines at ~0.2/s, or if the goroutine dump taken next time shows growth outside net/http.(*persistConn).readLoop/writeLoop, the hypothesis does not explain (all of) the growth."
  fix_rationale: "Share one local.Client per process (tailnet.LocalAPI) so every LocalAPI call reuses the same transport and its one keep-alive connection: the poller's cost becomes constant instead of linear in uptime. Same fix at the other per-call sites (DiscoverPeers on each tailnet cache refresh, defaultWarmCert) because they are the same root cause."
  blind_spots: "Not run in the VM (no Portty, no tailscaled allowed): the production rate is inferred from code + the VM note, not from a dump. Production-only per-capture-run costs are untested: native callback threads (SCK/VT/audio GCD threads that called goScreencap* exports) count in runtime.NumGoroutine until they exit (go1.26 needm/dropm with pthread-key dropm). They should drain when GCD reaps idle threads, but that is unverified. GUI-process leaks (app.go:1711 CheckHealth polling) are fixed by the same change but not measured."
  candidate_causes:
    - "code (daemon, time-driven): fresh local.Client per CheckHealth on the 10 s desktop poller - CONFIRMED mechanism"
    - "code (desktop, per-join): pion PC / RTCP readers / quality / audio / ws reader not released on leave - ELIMINATED by harness (5 leave modes, 2 layers)"
    - "environment (native): capture-run native threads counted as goroutines via extra Ms - untested, bounded by GCD reaping"
    - "config/env (tailscaled): LocalAPI server without IdleTimeout - the amplifier that makes the stranded connections permanent"
  and_gate: "yes: needs (1) a fresh client per call (our code) AND (2) a server that never closes idle connections (tailscaled). Either alone is harmless; we can only change (1)."

## Symptoms

expected: goroutine count returns to roughly the daemon-start baseline after every viewer leaves
actual: 32 at start, 1020 with no viewer connected after ~25 joins/leaves (~16-20 per join/leave plus a slow idle drift)
errors: none (count only, via GET /desktop/stats)
reproduction: cross-VM WebRTC viewer joins and leaves (relay path, some under dummynet, quality ladder stepping); not reproduced on host
started: observed during 23-09 VM rows (build 23.9.0, commit 98ca0390)

## Eliminated

- hypothesis: the stats route itself leaks
  evidence: 50 polls of GET /desktop/stats changed nothing (23-09-SUMMARY finding 4)
  timestamp: 2026-11-27T00:00:00Z

- hypothesis: a viewer join/leave leaves goroutines behind (pion PC not closed, RTCP/TWCC readers, GCC estimator, quality ticker, audio/video fanout, serveAdmitted socket reader, unlock/relock timers)
  evidence: desktop loopback harness, 5 leave modes (clean, socket dropped, client still sending after server close, media path black-holed then socket, black hole + late client close), audio + input channel, returns to baseline; daemon-level harness (real routes, hooks, stats) flat over 5 cycles; a mutant without the deferred pc.Close is caught (11 -> 36 over 3 cycles), so the test can see such a leak
  timestamp: 2026-11-27T01:30:00Z

- hypothesis: launchd relaunching a lost-lock daemon every 10 s
  evidence: plist KeepAlive is SuccessfulExit false (launchd.go:85-110); the lost-lock path exits 0, so it is not relaunched
  timestamp: 2026-11-27T01:32:00Z

## Evidence

- timestamp: 2026-11-27T00:10:00Z
  checked: serveAdmitted (manager.go:740-913) defer order
  found: defers run LIFO: m.leave(v) (+OnViewerLeave) first, then gs.detach, then m.q.unregister, then pc.Close(). The viewer leaves m.viewers BEFORE pc.Close() runs.
  implication: stats can read viewers 0 while a viewer's whole pion stack is still alive if pc.Close() blocks or the serving goroutine is stuck after leave().

- timestamp: 2026-11-27T00:12:00Z
  checked: pion cc interceptor v0.1.51 Close and gcc SendSideBWE (2 goroutines per estimator in delayController); viewerBWE embeds cc.BandwidthEstimator so Close is forwarded
  found: cc.Interceptor.Close -> estimator.Close -> delayController.Close (closes pipes, waits wg). pion closes interceptors in pc.Close.
  implication: the estimator goroutines end iff pc.Close runs to completion.

- timestamp: 2026-11-27T00:20:00Z
  checked: new loopback test TestFanout_JoinLeaveCyclesLeaveNoGoroutines (audio source + audio m-line + input data channel, client closes pc then ws cleanly, 1 warm-up + 5 cycles)
  found: PASS, goroutine count returns to the post-warm-up base
  implication: a clean, prompt client close does not leak. The VM leak needs a leave mode the harness did not drive (silent/abrupt peer, ws closed with media alive, media dead with ws alive, close during handshake, slow link).

- timestamp: 2026-11-27T00:30:00Z
  checked: capture lifecycle (manager.go join/leave/startSource/stopSourceLocked)
  found: the first viewer's join starts a capture run (NewSource + Start), the last viewer's leave stops it. ~25 lone-viewer join/leaves in the VM = ~25 real capture start/stop cycles. The harness uses SyntheticSource/audioFake, so the real DisplaySource is never exercised by any unit test.
  implication: a per-run leak in the real source (Go goroutines or native threads) is invisible to the loopback harness.

- timestamp: 2026-11-27T00:35:00Z
  checked: go1.26.11 runtime/proc.go needm/dropm/oneNewExtraM/gcount
  found: an extra M's goroutine is excluded from gcount via ngsys only while the thread is not in Go. needm does ngsys.Add(-1); on darwin (pthreads) dropm (ngsys.Add(1)) runs only from the pthread-key destructor when the non-Go thread EXITS. So every live native thread that has ever called a cgo export is counted by runtime.NumGoroutine until it exits.
  implication: native threads that call goScreencapAccessUnit / goScreencapStreamError / goScreencapAudioPacket / goScreencapAudioError (internal/screencap) inflate the stats goroutine count for as long as they live. A capture run that leaves such threads alive (or GCD that keeps them parked) shows up as "goroutines" with no Go code to fix.

- timestamp: 2026-11-27T00:50:00Z
  checked: 23-VM-CHECKS.md finding 4 (fuller than the summary)
  found: 32 at start with no viewer, 89 with ONE viewer (+57), 1020 with no viewer after ~25 join/leaves over two hours, "about 2 more every 10 s while idle after viewers came and went". 50 stats polls: 1024 before and after.
  implication: there is a time-based idle drift (~0.2/s) on top of any per-join residue; the "16-20 per join" figure was not separated from the drift. No dump exists.

- timestamp: 2026-11-27T01:00:00Z
  checked: harness server-side cost of one viewer (pprof label side=client on the test's client calls, count unlabeled)
  found: idle 10, one viewer 41 (+31: ~22 pion PC goroutines, ws handler, fanOut, audioOut, quality loop, source goroutines), after leave 10.
  implication: production adds ~26 more per viewer+run than the harness (real DisplaySource incl. native-thread extra Ms, daemon layer). The harness pion stack is fully released on leave.

- timestamp: 2026-11-27T01:05:00Z
  checked: leave mode pc_only (client pc.Close, ws kept open), PION_LOG_INFO=all
  found: pion logs "Closing PeerConnection from DTLS CloseNotify" and the server pc goes to state closed; serveAdmitted's OnConnectionStateChange handles only Connected and Failed, so the socket is not closed and the viewer stays joined (60 s+, capture kept running) until the socket drops. All goroutines are released once the socket closes.
  implication: a real zombie-viewer defect (DTLS-closed viewer kept until its socket drops), but NOT the leak: viewers would read >0, and the real page (frontend/src/lib/desktopClient.ts:534-556 teardown) closes socket and pc together.

- timestamp: 2026-11-27T01:15:00Z
  checked: TestFanout_JoinLeaveCyclesLeaveNoGoroutines modes clean, ws_abrupt, ws_then_pc_late (client keeps sending 3 s after server pc closed), blackhole_then_ws (media path silently dropped both ways, then socket closed), blackhole_ws_then_late_pc; each 1 warm-up + 3 cycles, audio + input channel
  found: all PASS, back to the post-warm-up count within the settle window
  implication: no per-join leak in desktop.Manager + webserver ws route + pion under prompt, abrupt, or silent-path leaves.

- timestamp: 2026-11-27T01:25:00Z
  checked: daemon-level probe (newDesktopEnv + mustOn + connectViewer, 1 warm-up + 5 join/leave cycles through the real daemon routes, hooks and stats)
  found: base 15, 16 during each cycle, final 15
  implication: the daemon layer (desktopAdapter, OnViewerJoin/Leave, audit, share stream) adds no per-join residue either.

- timestamp: 2026-11-27T01:35:00Z
  checked: 10 s cadences on the daemon side
  found: internal/daemon/desktop_api.go:30 desktopPollEvery = 10 s; runDesktopPoller (desktop_api.go:628) -> desktopTick (644) -> desktopTailnet(ctx, a.shareHealth, ...) (370-376) runs every 10 s whenever always-available or a guest share is on (it was on throughout 23-09). a.shareHealth = defaultShareHealth = webserver.CheckHealth (daemon/share.go:33, api.go:154). CheckHealth (internal/webserver/tailscale.go:71-78) declares `var lc local.Client` per call and makes two LocalAPI requests (StatusWithoutPeers, GetPrefs).
  implication: one fresh local.Client per 10 s tick.

- timestamp: 2026-11-27T01:40:00Z
  checked: tailscale.com v1.98.3 client/local/local.go:138-146 DoLocalRequest
  found: each local.Client value lazily builds its own &http.Transport{DialContext: lc.dialer()} (keep-alives on, no IdleConnTimeout). After a response is read the connection parks idle in THAT transport; an abandoned transport's idle persistConn keeps its readLoop and writeLoop goroutines (GC roots) until the server closes the connection.
  implication: CheckHealth strands one idle LocalAPI connection = 2 goroutines per call = 2 per 10 s while the desktop is on. This is exactly the VM's "about 2 more every 10 s while idle". It also accounts for the whole count: (1020-32)/2 = 494 ticks = ~82 min of always-available-on time, and 89 with one viewer = 32 + ~31 (harness per-viewer cost) + ~24 (~2 min of ticks).

- timestamp: 2026-11-27T01:45:00Z
  checked: tailscale.com v1.98.3 ipn/ipnserver/server.go:533-540 (tailscaled's LocalAPI http.Server)
  found: no IdleTimeout and no ReadTimeout
  implication: the stranded connections are never closed: the leak is unbounded (daemon: 2 goroutines + 1 socket per tick; tailscaled: 1 goroutine + 1 socket per tick). H-03's open-files bound would fail for the same reason.

- timestamp: 2026-11-27T01:50:00Z
  checked: fake-LocalAPI experiment (internal/webserver, Dial + OmitAuth, nothing real contacted), 20 checkHealth calls
  found: fresh local.Client per call 3 -> 63 goroutines (+3 per call in-process: 2 client persistConn + 1 server conn.serve); one shared client 3 -> 6 total
  implication: mechanism reproduced on the exact CheckHealth request pair.

- timestamp: 2026-11-27T02:00:00Z
  checked: TestLocalAPI_NoClientPerCall before the fix
  found: FAIL at internal/daemon/share.go:45, internal/tailnet/tailnet.go:96, internal/webserver/tailscale.go:76 (the three per-call sites); PASS after
  implication: RED -> GREEN on the root cause.

## Resolution

root_cause: "Not viewer join/leave. The daemon's 10 s desktop health poller (desktopTick -> desktopTailnet -> a.shareHealth = webserver.CheckHealth) built a fresh tailscale local.Client on every call (internal/webserver/tailscale.go:76). Each local.Client owns an http.Transport; after its two LocalAPI requests the keep-alive connection parked idle in a transport nobody referenced, and because tailscaled's LocalAPI server never closes idle connections, its persistConn readLoop and writeLoop goroutines (and the socket) lived forever: +2 goroutines and +1 fd per 10 s while the desktop is on. The same per-call pattern sat in tailnet.DiscoverPeers (each 30 s tailnet-cache refresh) and the daemon's defaultWarmCert. AND-gate: needs our per-call client AND a server without an idle timeout."
oracle_type: derived (connection count at a fake LocalAPI per N calls; static contract: no local.Client built inside a function in shipped code)
fix: "One shared client per process, tailnet.LocalAPI (package var, zero value usable, safe for concurrent use); CheckHealth, DiscoverPeers and defaultWarmCert use it, so every LocalAPI call reuses the same transport and its one keep-alive connection."
verification:
  red_then_green: "TestLocalAPI_NoClientPerCall failed on the 3 sites before the fix, passes after"
  mechanism: "TestLocalAPI_OneClientKeepsOneConnection: shared client 1 connection for 20 calls, client per call 20"
  join_leave_guard: "TestFanout_JoinLeaveCyclesLeaveNoGoroutines (3 leave modes) passes -race -count=2; detects a removed pc.Close mutant (11 -> 36)"
  suites: "go test -race ./internal/tailnet ./internal/webserver ./internal/desktop (181 s) ./internal/daemon (141 s) all pass; go vet . (root) clean"
  static: "go vet (default and -tags vmwhois) and gofmt clean"
  not_verified: "no VM rerun (not allowed here): the production rate is inferred, not measured; GUI-process CheckHealth polling (app.go:1711) is fixed by the same change but not measured"
  guardrail_verdict: accepted
files_changed: [internal/tailnet/tailnet.go, internal/webserver/tailscale.go, internal/daemon/share.go, internal/tailnet/localapi_test.go, internal/desktop/goroutine_leak_test.go]
