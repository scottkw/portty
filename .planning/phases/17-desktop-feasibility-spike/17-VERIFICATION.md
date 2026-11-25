---
phase: 17-desktop-feasibility-spike
verified: 2026-11-25T19:30:00Z
status: human_needed
score: 4/4 must-haves verified (1 by override)
covered_files:
  - .planning/phases/17-desktop-feasibility-spike/17-01-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-01-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-02-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-02-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-03-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-03-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-04-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-04-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-05-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-05-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-06-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-06-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-07-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-07-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-08-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-08-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-09-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-09-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-10-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-10-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-11-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-11-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-12-PLAN.md
  - .planning/phases/17-desktop-feasibility-spike/17-12-SUMMARY.md
  - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md
  - .planning/phases/17-desktop-feasibility-spike/evidence/cleanup-final.txt
  - .planning/phases/17-desktop-feasibility-spike/evidence/closure-decisions.txt
  - .planning/phases/17-desktop-feasibility-spike/evidence/feas02-rerun.txt
  - .planning/phases/17-desktop-feasibility-spike/evidence/host-run-notes.txt
  - .planning/phases/17-desktop-feasibility-spike/evidence/r01-host-report-safari-lite.json
  - .planning/phases/17-desktop-feasibility-spike/evidence/r01-network.txt
  - .planning/phases/17-desktop-feasibility-spike/evidence/r18-r20-host-timeline-session3.jsonl
  - .planning/phases/17-desktop-feasibility-spike/evidence/sc4-final.txt
  - TESTING.md
  - spike/desktop/.gitignore
  - spike/desktop/README.md
  - spike/desktop/bridge_darwin.go
  - spike/desktop/bundle/Info.plist
  - spike/desktop/bundle/app.portty.spike.daemon.plist
  - spike/desktop/bundle/app.portty.spike.helper.plist
  - spike/desktop/bundle/build.sh
  - spike/desktop/bundle/entitlements.plist
  - spike/desktop/go.mod
  - spike/desktop/go.sum
  - spike/desktop/hostrun.go
  - spike/desktop/hostrun_darwin.m
  - spike/desktop/hostrun_test.go
  - spike/desktop/main.go
  - spike/desktop/media_darwin.m
  - spike/desktop/probe.go
  - spike/desktop/server.go
  - spike/desktop/sys_darwin.m
  - spike/desktop/viewer.html
  - spike/desktop/wailsshell/go.mod
  - spike/desktop/wailsshell/go.sum
  - spike/desktop/wailsshell/main.go
  - spike/desktop/wailsshell/wails.json
covered_digest: "v4:sha256:1446d2baa8a595f6a7683d8df388672c8b9ca496e86b84df4f470b99ee98f01d"
behavior_unverified: 0
overrides_applied: 1
overrides:
  - must_have: "On a real Apple Silicon Mac, a throwaway page opened from another device over the tailnet shows that Mac's desktop moving live (ScreenCaptureKit -> VideoToolbox H.264 -> pion -> browser), with no STUN, TURN or cloud server involved"
    reason: "Live frames from the real MacBook Air are proven (R01-host: 1853 decoded, 0 dropped, 2560 x 1664, ICE connected, server bound only to the tailnet address, iceServers empty, 0 spike packets to 3478/5353/19302). Both test-rig paths ran over a Tailscale DERP relay (tailscale ping 'direct connection not established'), a test-rig limit: the two VMs and VM-home-to-host never establish a direct path. The direct path between the two real Macs is checked in Phase 19 (requirement change on DESK-02, follow-up M-157). R01, R01-vm and R01-host closed PARTIAL by user acceptance."
    accepted_by: "user (17-12 closure checkpoint, reply: ok; evidence/closure-decisions.txt)"
    accepted_at: "2026-11-25T00:00:00Z"
deferred:
  - truth: "SC1 clause 'no STUN, TURN or cloud server' shown on a direct tailnet path between the real office Mac and home Mac"
    addressed_in: "Phase 19"
    evidence: "Phase 19 SC4: 'Desktop media stays on the tailnet. No STUN, TURN or cloud server is contacted ... tests assert both.' (DESK-02); 17-SPIKE-MATRIX.md Requirement changes adds a direct-path success criterion; TESTING.md M-157"
human_verification:
  - test: "Prohibition sign-off: no committed evidence file or matrix cell contains a frame of the real host desktop, a token, the tailnet domain, a 100.x address, or the user's account or host name"
    expected: "Confirm. Verifier's non-authoritative check: grep over evidence/, the matrix and SUMMARYs found no 100.64/10 address, no t=<32 hex> token, no *.ts.net name, no home path or account name; committed frames inspected are VM desktops or the test chart; host-run frames were never committed"
    why_human: "unverified-prohibition, judgment tier (no verification field); grep cannot rule out a name or desktop content inside an image"
  - test: "Prohibition sign-off: no executor-run program contacted the host's tailscaled or a 100.x address outside the user-gated 17-11 host-run window"
    expected: "Confirm from your recollection of the sessions. Verifier found no evidence of a violation; the only record is executor self-report (e.g. r01-network.txt: 'The host never contacted its own tailscaled or a tailnet address')"
    why_human: "unverified-prohibition, judgment tier; a past process action leaves no artifact the codebase can prove"
  - test: "Prohibition sign-off: the spike never typed, stored or logged a real password on the user's Mac"
    expected: "Confirm. Verifier's non-authoritative check: both host-run timelines start with allow-input false, empty type-keycodes and allow-keycodes, and contain 0 key events; every typed password in the matrix is the public VM password, in the VM only"
    why_human: "unverified-prohibition, judgment tier"
  - test: "Prohibition sign-off: no spike process, LaunchAgent, SMAppService registration or TCC grant for app.portty.spike / app.portty.spike.helper remains on the personal Mac"
    expected: "Confirm. Verifier's read-only check today: pgrep -f Spike.app rc 1, launchctl print gui/<uid>/app.portty.spike.daemon and .helper rc 113, no spike plist in ~/Library/LaunchAgents. TCC state rests on the 17-11 session 3 revoke (tccutil rc 0 x3, preflight all false), which you accepted in place of a 17-12 re-run"
    why_human: "unverified-prohibition, judgment tier; the verifier is not permitted to read or reset host TCC state"
---

# Phase 17: Desktop Feasibility Spike Verification Report

**Phase Goal:** Before any production desktop code is written, a throwaway end-to-end path on a real Mac shows the desktop live over the tailnet and settles each risky assumption with a recorded go or no-go. The capture-host architecture is decided from that evidence, so every later phase plans against what was proven rather than what was hoped.
**Verified:** 2026-11-25T19:30:00Z
**Status:** human_needed (the only open items are sign-offs on 4 judgment-tier prohibitions; every success criterion holds)
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP success criteria) | Status | Evidence |
|---|----------------------------------|--------|----------|
| 1 | Real Apple Silicon Mac's desktop live in a page on another device over the tailnet (SCK -> VT H.264 -> pion -> browser), no STUN, TURN or cloud server (FEAS-01) | PASSED (override) | R01-host report: ICE connected 1770 ms, 1853 frames decoded, 0 dropped, 2560 x 1664, H264 42e01f, remote port 47801 host type. Code: `viewer.html` `new RTCPeerConnection({ iceServers: [] })`, `server.go` `webrtc.Configuration{}` with ICE-lite + UDP mux, `media_darwin.m` SCStream and VTCompressionSession. r01-network.txt: lsof shows only the tailnet address, 0 spike packets to 3478/5353/19302. The path was a Tailscale DERP relay on both rigs. Override: test-rig limit, user-accepted at the 17-12 closure checkpoint (evidence/closure-decisions.txt); direct path is deferred to Phase 19 (M-157) |
| 2 | Recorded go/no-go matrix gives a measured result with evidence for every FEAS-01 question | VERIFIED | 17-SPIKE-MATRIX.md: 55 result rows (24 base, 30 sub/reference, T0), 0 PENDING or ROUTED. Results: 45 GO, 4 NO-GO, 6 PARTIAL. Every evidence path in the matrix exists (scripted check, 0 missing). SC2 coverage section maps each bullet: permission holder R02 to R10; Wails web view R13 (SpikeShell substitution, user-accepted in CONTEXT); Chrome R12, Safari R11, iPhone R14 with lite and full ICE each; lock/CGEvent R15 to R17; sleep/wake R18; re-approval R19 (PARTIAL context-unanswerable, which CONTEXT Timebox allows, M-160); CPU/latency/SSIM R20-host (hardware true, 29.15 fps, 12.85 ms mean, 27.0% of a core, SSIM 0.9993); Opus R21 |
| 3 | Capture-host decision written with evidence before Phase 19 is planned; every no-go names the requirement it changes or cuts (FEAS-02) | VERIFIED | Matrix `## FEAS-02 decision`: Status DECIDED, Capture host in-daemon, Decided by row 7, a term-by-term walk citing evidence files. The decision rule section was committed in 40d35b80, an ancestor of the first R00 evidence commit a5466a8e, and `git log -L` shows no later edit to it. `## Requirement changes`: R06 changes DSHR-01/DPRM-01, R18 changes DPRM-04, R01 changes DESK-02, R08-D changes DSHR-01, R19 changes DPRM-03, plus UNLK-01 ship and DVWR-03 tap-to-play. Phase 19 is still `Plans: TBD` |
| 4 | No spike code reaches the shipping build; release builds and the full terminal test suite unchanged | VERIFIED | Independently re-checked: `spike/desktop/go.mod` (module portty-spike-desktop) and `wailsshell/go.mod` are nested modules, there is no go.work, `go list ./...` and `go list -deps ./...` show 0 spike packages, and no root .go file imports a spike path. `git diff b21e0e7c HEAD` outside spike/, .planning/ and TESTING.md is empty (b21e0e7c is the parent of the first spike commit 9c923f82). .github/workflows, build/, go.mod, go.sum and frontend/ are unchanged. `go build ./...` OK, brand gate clean, 228 traceability paths OK. `spike/desktop/build/` is gitignored. The TESTING.md diff is docs only (Phase 17 note + Category BH M-157..M-164) |

**Score:** 4/4 truths verified (1 by override; 0 present, behavior-unverified)

Plan-level must_haves were merged under these four. `verify.artifacts` passes on 10 of 12 plans and `verify.key-links` on 12 of 12. The two artifact misses are naming drift, not missing work (see Anti-Patterns).

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Direct (non-DERP) tailnet path with no STUN/TURN/cloud between the real office and home Macs | Phase 19 | Phase 19 SC4 (DESK-02); matrix Requirement changes; TESTING.md M-157 |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `spike/desktop/` (main.go, probe.go, server.go, hostrun.go, *.m, viewer.html) | Throwaway SCK -> VT -> pion -> viewer pipeline and permission probes | VERIFIED | 4,935 lines of real code, separate module, no debt markers, archived README |
| `spike/desktop/bundle/build.sh` | Developer ID signing, inside out, never ad-hoc | VERIFIED | Refuses "-" and foreign teams; `--options runtime --timestamp` per binary |
| `spike/desktop/wailsshell/` | Wails v2 shell for R13 | VERIFIED | Own nested module |
| `17-SPIKE-MATRIX.md` | Rows, rule, decision, requirement changes | VERIFIED | Every row closed with existing evidence |
| `evidence/` | Raw artifacts per row | VERIFIED | 86 files. Spot-checked R01-host JSON, r01-network.txt, host-run-notes R20 numbers, host timeline (36 perf events, 0 key events), and the R12 frame (VM desktop) |
| `TESTING.md` Category BH | M-157..M-164 follow-ups | VERIFIED | Present; traceability check passes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Matrix rows | evidence files | Evidence cell paths | WIRED | 0 missing paths |
| FEAS-02 decision | decision rule | first-match table walk | WIRED | Row 7; terms cite R04, R02/R03, R09-B, R07-B, R17-A/B |
| NO-GO/PARTIAL rows | requirements | `## Requirement changes` | WIRED | Each names a requirement; 17-12 SUMMARY lists them as Required user edits |
| Root module | spike | (must not exist) | NOT LINKED (correct) | Nested go.mod; 0 packages in `go list -deps ./...` |

### Data-Flow Trace (Level 4)

Not applicable. Spike tooling, no product rendering.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Root module excludes spike | `go list ./... \| grep -c spike` | 0 | PASS |
| Root module builds | `go build ./...` | BUILD_OK | PASS |
| Brand gate | `bash tests/check-brand.sh` | OK: brand gate clean | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 228 traceability paths exist | PASS |
| Nothing outside allowed paths changed | `git diff --stat b21e0e7c HEAD -- . ':(exclude)spike' ':(exclude).planning' ':(exclude)TESTING.md'` | empty | PASS |
| Host residue (read-only) | `pgrep -f Spike.app`; `launchctl print gui/<uid>/app.portty.spike.{daemon,helper}` | rc 1; rc 113 / 113 | PASS |
| Full Go suite | not re-run | Diff outside spike/ is empty, so the suite cannot have changed; 17-12 recorded 14 ok / 0 failed (sc4-final.txt) | SKIP (by design) |

The Spike binary, tccutil and anything that reaches the host's tailscaled or 100.x addresses were not run, as instructed.

### Probe Execution

None. The phase declares no `scripts/*/tests/probe-*.sh`. The spike's "probes" are runtime verbs of the Spike binary, which this verification must not run.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| FEAS-01 | 17-01..17-11 | End-to-end path live over tailnet with go/no-go per question | SATISFIED (with R01 override) | Truths 1 and 2 |
| FEAS-02 | 17-06, 17-12 | Capture-host decision recorded with evidence before the capture host is built | SATISFIED | Truth 3 |

No orphaned requirements: REQUIREMENTS.md maps only FEAS-01 and FEAS-02 to Phase 17.

### Prohibitions (ADR-550)

Four distinct prohibitions appear across the plans. All are `status: unresolved` with no `verification` field, so they are treated as judgment tier. The verdicts below are non-authoritative, and each prohibition is flagged as `unverified-prohibition: human review recommended`.

| Prohibition | Non-authoritative verdict | Basis |
|-------------|---------------------------|-------|
| No host-desktop frame, token, tailnet domain, 100.x address or account/host name in committed evidence | Appears honored | Regex greps clean; inspected frame is a VM desktop |
| No executor contact with host tailscaled/100.x outside the 17-11 window | Cannot be verified | Executor self-report only |
| No real password typed, stored or logged on the user's Mac | Appears honored | Host timelines: allow-input false, 0 key events |
| No spike residue on the personal Mac | Appears honored | Read-only checks clean; TCC proof is the user-accepted 17-11 session 3 revoke |

### Decision Coverage

`check.decision-coverage-verify` returned skipped ("No trackable decisions in CONTEXT.md"), so the parser found nothing to count. Manual check: all CONTEXT decisions are honored. These are the Tart VM rig; the host run on the MacBook Air with the user's go; the two permission probes (helper and in-daemon); the SpikeShell substitution (R13); the UI safety gate waiver (no UI-SPEC, as allowed); no timebox with M-NN follow-ups for unanswerable rows; the separate spike module; Developer ID signing; R19 backdating only in the off-tailnet clone spike-r19; and the plain HTML viewer.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| spike/desktop/*, TESTING.md diff, matrix | - | TBD/FIXME/XXX | none found | - |
| 17-11-PLAN.md artifact `evidence/r18-host-timeline.jsonl` | 41 | Name drift | Info | Delivered as `r18-r20-host-timeline.jsonl` (session 2) and `r18-r20-host-timeline-session3.jsonl` (190 lines, 36 perf events); listed in 17-11-SUMMARY |
| 17-06-PLAN.md artifact pattern `Status: PROVISIONAL` | - | Superseded | Info | 17-12 replaced it with `Status: DECIDED`, as the rule intends |
| `.planning/debug/spike-hostrun-sigtrap.md` (untracked) | 2 | `status: awaiting_human_verify` | Info | The nil-font fix was exercised by session 3's full 180 s host run, so the debug session can be closed |
| Matrix FA-1 | - | Flagged assumption | Warning (non-blocking) | All attribution rows (R02 to R09) ran in VMs only. The office-Mac confirmation is M-163, and the decision is revisited if it differs |
| REQUIREMENTS.md / ROADMAP.md | - | Pending requirement edits | Warning (non-blocking) | DESK-02, DSHR-01, DPRM-01, DPRM-03, DPRM-04 must be edited before Phase 19 is planned, UNLK-01 before Phase 23, DVWR-03 before Phase 21 (17-12-SUMMARY "Required user edits"). This is by design: the matrix is the sole source until then |

### Test Quality Audit

No requirement-linked tests in the shipping suite (spike phase; evidence is measured runtime artifacts). `spike/desktop/hostrun_test.go` (5 tests, 0 skipped) lives in the nested spike module and is intentionally outside `go test ./...`. Disabled tests: 0. Circular patterns: 0.

### Human Verification Required

The four prohibition sign-offs listed in the frontmatter `human_verification`. These cover redaction of committed evidence, no out-of-window tailnet contact, no real password typed on the host, and no spike residue on the host. Each has the verifier's non-authoritative check attached. No UI or behavior item is needed: the UI safety gate is waived for this throwaway tooling (CONTEXT), and the remaining real-Mac questions are tracked follow-ups M-157..M-164, not open truths of this phase.

### Gaps Summary

No blocking gaps. The phase goal holds in the codebase and evidence. A real end-to-end SCK -> VideoToolbox -> pion -> browser path ran on the real MacBook Air and on the VMs. Every FEAS-01 question has a measured, evidenced result. FEAS-02 is decided (in-daemon) by a rule committed before any evidence. Every no-go and partial names its requirement change. The spike is a nested module the shipping build cannot see. The single literal shortfall, the DERP relay on the test rigs in SC1, is covered by the user's recorded acceptance and deferred to Phase 19 (M-157). Status is human_needed only because the four judgment-tier prohibitions need explicit sign-off. With that sign-off, the phase passes.

---

_Verified: 2026-11-25T19:30:00Z_
_Verifier: Claude (gsd-verifier)_
