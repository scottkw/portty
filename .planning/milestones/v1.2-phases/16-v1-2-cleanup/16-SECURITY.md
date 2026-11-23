---
phase: "16"
slug: "v1-2-cleanup"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 16 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register from the plan-time `<threat_model>` blocks in 16-01..16-04-PLAN.md; evidence verified at HEAD of `gsd/v1.2` on 2026-11-23 (targeted `-race` tests, the 16-04 single full run, post-review fixes 0e08afca/c8158c94, and M-156 PASS in the Tart VM).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| macOS launch services → app binary | Finder, Dock, `open` and login items start the binary with no arguments or launch-services arguments; `launchMode` decides GUI or CLI | argv |
| CLI user → daemon unix socket | `portty kill`/`rename` turn a typed argument into a DELETE or PATCH on `/sessions/{id}` | Session name / ID / prefix |
| PTY output → hub row numbering | Shell and TUI output (ED 3, alt-screen switches) drives the absolute rows the cwd log and path clicks rely on | Terminal output |
| viewer → host (MsgPathClick) | A viewer sends text; the host checks the session printed it, resolves it against that row's cwd, then applies previewfile checks | Clicked text |
| planning artifacts → milestone audit | The audit trusts SUMMARY frontmatter for completed requirements | requirements-completed lists |
| regression home → CI / milestone close | build.yml runs the traceability check; the audit trusts the single clean run and dispositions | TESTING.md, logs, dispositions |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-16-01 | Denial of Service | launchMode (main.go) | high | mitigate | No args and `-psn_`/`-NS`/`-Apple` shapes stay GUI launches, pinned by TestLaunchMode's 18 rows incl. near miss `-psn`; M-156 PASS 2026-11-23: Finder, Dock reopen, cold Dock launch, `open -a` and both `--args` forms all opened the window | closed |
| T-16-02 | Elevation of Privilege | cmdKill/cmdRename name resolution (cmd_cli.go, cmd_share.go) | medium | mitigate | resolveSession: exact name, exact ID, then unique prefix; duplicate name refused; post-review 0e08afca adds lookupSessionStrict (ID prefix ≥ 8 chars for kill/rename) and c8158c94 rejects an empty argument; TestCmdKill_ByName, TestCmdRename_ByName, TestCmdKillRename_ShortPrefixRefused, TestResolveSession; M-156 step 3 confirmed in the VM | closed |
| T-16-03 | Tampering | DaemonClient.KillSession/RenameSession URL path | low | mitigate | Only an ID taken from the daemon's own ListSessions reaches `/sessions/{id}`; a typed name containing `/` or `..` never does | closed |
| T-16-04 | Information Disclosure | ED 3 row counting (internal/relay/hub.go) | low | mitigate | `!emu.IsAltScreen()` guard keeps printed paths resolving against the directory they were printed in; TestCwd_AltScreenScrollbackEraseKeepsRows (failed before the guard, passes after; reviewer re-confirmed by removing it) | closed |
| T-16-05 | Denial of Service | killSession merge (internal/pty/cleanup.go) | low | mitigate | Merged early return has the same body for the same conditions; reaped sessions never signalled; TestKillSession_ReapedSkipsGroupSignal, TestKillSession_RunningSignalsGroup | closed |
| T-16-06 | Spoofing | API version reporting (internal/daemon/types.go) | low | accept | See Accepted Risks Log | closed (accepted) |
| T-16-07 | Denial of Service | DoS-bound test coverage (frontend tests) | low | mitigate | Only a 30 s per-test timeout added; hostile input sizes and assertions unchanged (verifier diff check); both files pass (70 tests) | closed |
| T-16-08 | Repudiation | SUMMARY requirements-completed claims | low | mitigate | Each added ID carries an inline comment citing its phase's passed VERIFICATION.md (PREV-02 keeps "desktop look: deferred, accepted"); changed-line gates held at 4 and 3; summary-extract reads back INPT-01..06, PREV-01..05, MOB-01..05 | closed |
| T-16-09 | Tampering | TESTING.md traceability | low | mitigate | `bash tests/check-traceability-paths.sh` → OK: 228 (re-run after every TESTING.md edit) | closed |
| T-16-10 | Repudiation | "clean run" claim | low | mitigate | Each suite ran once (action script); logs kept; 14 ok lines = 14 go-list packages, no FAIL; vitest 1826/1826; post-review delta re-tested (root package `-race` ok) | closed |
| T-16-11 | Denial of Service | launch paths on the remote Mac (D-01) | high | mitigate | M-156 PASS 2026-11-23 in `portty-test`, build v1.2.0-vm17 (recorded in TESTING.md Category BG) | closed |
| T-16-SC | Tampering | npm/pip/cargo installs | high | accept | See Accepted Risks Log (no package added in any plan; `git diff cc595307..HEAD` touches no package.json, lockfile, go.mod or go.sum) | closed (accepted) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-16-01 | T-16-06 | Only the unused documentation constant `PathClickAPI` was removed; `/health` keeps APIVersion 5, viewers follow the per-connection pathClick bit, MinDaemonAPI stays 1; TestHealthInfo pins the reported values | Planner (16-02 threat model), user-approved plan | 2026-11-23 |
| AR-16-02 | T-16-SC | No package is installed or added in this phase, so there is no supply-chain surface to mitigate | Planner (all four plans) | 2026-11-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Notes (non-blocking)

- Review info items IN-01 (exact name shadows an exact full ID), IN-02 (ED 3 in the alternate screen still clears hub history) and IN-03 (dead `arg != ""` check) stay open in 16-REVIEW-DISPOSITION.md; none is a security exposure at L1.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 12 | 12 | 0 | /gsd-secure-phase 16 (orchestrator, L1 short-circuit: register authored at plan time, threats_open 0) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
