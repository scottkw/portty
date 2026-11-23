---
phase: 16-v1-2-cleanup
verified: 2026-11-23T14:05:00Z
status: passed
score: 6/6 must-haves verified
covered_files:
  - ".planning/phases/11-file-preview/11-SECURITY.md"
  - ".planning/phases/16-v1-2-cleanup/16-01-PLAN.md"
  - ".planning/phases/16-v1-2-cleanup/16-01-SUMMARY.md"
  - ".planning/phases/16-v1-2-cleanup/16-02-PLAN.md"
  - ".planning/phases/16-v1-2-cleanup/16-02-SUMMARY.md"
  - ".planning/phases/16-v1-2-cleanup/16-03-PLAN.md"
  - ".planning/phases/16-v1-2-cleanup/16-03-SUMMARY.md"
  - ".planning/phases/16-v1-2-cleanup/16-04-PLAN.md"
  - ".planning/phases/16-v1-2-cleanup/16-04-SUMMARY.md"
  - "TESTING.md"
  - "cmd_cli.go"
  - "cmd_cli_test.go"
  - "cmd_share.go"
  - "cmd_share_test.go"
  - "dispatch_test.go"
  - "frontend/src/lib/__tests__/localhostUrl.test.ts"
  - "frontend/src/lib/__tests__/markdown.test.ts"
  - "internal/daemon/client_test.go"
  - "internal/daemon/types.go"
  - "internal/pty/cleanup.go"
  - "internal/relay/cwd_test.go"
  - "internal/relay/hub.go"
  - "internal/webserver/input_queue_test.go"
  - "internal/webserver/resume_test.go"
  - "main.go"
covered_digest: "v4:sha256:801ffd49cb8ecbc07b6baca429293370f94cf3e1ceb2022d058abaaae10e8632"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 5/6
  gaps_closed:
    - "SC5: `/gsd-secure-phase 11` produces 11-SECURITY.md"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "M-156 (TESTING.md Category BG): launch Portty from Finder, Dock, `open -a Portty`, `open -a Portty --args -NSDocumentRevisionsDebugMode YES` and `-ApplePersistenceIgnoreState YES` in the Tart VM `portty-test`; over SSH run `Portty --version`, `-v`, `--bogus` with `echo $?` and `pgrep -x Portty` before and after; rename and kill by name with `portty rename`/`portty kill`, including a duplicate name and a 4-character ID prefix"
    expected: "Every launch shows the window; --version and -v print the version and exit 0; --bogus prints `portty: unknown flag \"--bogus\"` plus usage and exits 2; no new Portty process for any CLI call; rename and kill by name act on the named session; a duplicate name is refused and both sessions remain; a 4-character ID prefix is refused with `no session named` and the session remains"
    why_human: "Real LaunchServices launches of a signed bundle cannot run in process or on the user's Mac. launchMode is table-tested in process, but the entry point for Finder, Dock, open and login items on the remote Mac is a costly-to-get-wrong path (D-01, rock-solid priority). Recorded in TESTING.md as not yet run; must PASS before ship."
---

# Phase 16: v1.2 Cleanup Verification Report

**Phase Goal:** Clear the doable tech debt from the v1.2 re-audit before the milestone closes (CLI kill-by-name, `--version`/unknown flags, load-robust DoS tests, Phase 15 review info items, planning hygiene, one clean full suite run, TESTING.md). Out of scope: real-Mac/release-gated checks, dompurify-3418, user-accepted limits.
**Verified:** 2026-11-23
**Status:** passed (all automated truths verified; M-156 real-bundle launch check PASSED in the Tart VM `portty-test` on 2026-11-23, build v1.2.0-vm17; see TESTING.md Category BG)
**Re-verification:** Yes, after gap closure (SC5 11-SECURITY.md; TESTING.md drift)

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `portty kill <name>` resolves names like share/rename; a test pins it | VERIFIED | `cmd_cli.go` cmdKill (line 320) and cmdRename (337) call `lookupSessionStrict` (`cmd_share.go:77`, wraps `lookupSession`; names and full IDs pass, ID prefixes need `minDestructivePrefix` = 8). Re-run at HEAD under `-race`: `TestCmdKill_ByName`, `TestCmdRename_ByName`, `TestCmdKillRename_ShortPrefixRefused`, `TestResolveSession` PASS (root package ok). Todo in completed. |
| 2 | `Portty --version`/`-v` prints the version with no GUI; unknown flags never start the GUI | VERIFIED in process; real launch pending (M-156) | `TestLaunchMode` PASS at HEAD (18 rows). `main.go` unchanged since the last run. I did not run the built binary: a mistake would start the GUI on this host. |
| 3 | The two DoS-bound vitest tests are load-robust without loosening the bound | VERIFIED | `30_000` timeouts present at `localhostUrl.test.ts:40` and `markdown.test.ts:211`; re-ran the two files: 2 files, 70 tests passed. |
| 4 | Phase 15 IN-02, IN-03, IN-06 resolved or recorded; `PathClickAPI` removed or consumed | VERIFIED | Re-run at HEAD under `-race`: `TestCwd_AltScreenScrollbackEraseKeepsRows` (relay), `TestKillSession_*` (pty), `TestShare_PasteBacklogKeepsViewer`, `TestWSS_ResumeDeltaAfterReconnect` (webserver), `TestHealthInfo` (daemon): all ok. `git grep PathClickAPI -- '*.go'` exits 1 (no hits). |
| 5 | Planning hygiene: SUMMARY `requirements-completed` covers INPT-01..06, PREV-02/04, MOB-01..05; `/gsd-secure-phase 11` produces 11-SECURITY.md; selection-offset todo triaged | VERIFIED | `.planning/phases/11-file-preview/11-SECURITY.md` exists and is tracked in git (commit 0253edd3): frontmatter `status: verified`, `threats_open: 0`, audit trail 45 threats / 45 closed / 0 open; T-11-CR01 registered retroactively (high, mitigate, closed). I spot-checked its evidence at HEAD: `frontend/index.html:8` carries `frame-src blob:; child-src blob:; worker-src 'self'`, pinned verbatim by `PreviewRenderers.test.tsx:197`; `PreviewMaxBytes = 20_000_000` and `PreviewWindow = 16` in `internal/relay/preview.go`. One accepted risk (AR-11-01, T-11-26) is documented with rationale and acceptor. SUMMARY frontmatter read-back: 10-01 INPT-05, 10-02 INPT-04, 10-07 INPT-01..06, 11-10 PREV-02/PREV-04, 14-14 MOB-01..05 all present. Selection-offset todo (`2026-11-21-selection-offset-and-copy.md`) is in `.planning/todos/completed` with a Resolution. |
| 6 | Full Go -race and vitest pass once, clean; fixed todos move to completed; TESTING.md updated per standing rule | VERIFIED | 16-04 logs recorded a clean full run (14 `ok` packages, vitest 1826 passed); the post-review root-package re-run was ok in the earlier verification. At HEAD: `go vet .` clean, `gofmt -l` empty, `bash tests/check-traceability-paths.sh` = OK: 228. Pending todos left: only dompurify-3418 and v12-release-checks (both out of scope). The earlier TESTING.md drift is fixed (see below). |

**Score:** 6/6 truths verified (M-156 is a human check recorded as a must-pass before ship, not a failed truth).

### Gap Closure Re-check

| Prior gap / warning | Resolution | Evidence |
|---|---|---|
| SC5: 11-SECURITY.md missing | CLOSED | File present and committed, `status: verified`, `threats_open: 0`; contents checked as above. |
| WARNING: TESTING.md Phase 16 note omitted post-review tests | CLOSED | Commit 57cb9ca2: the note now lists `TestCmdKillRename_ShortPrefixRefused` under cmd_cli_test.go and `TestResolveSession` empty-argument rejection under cmd_share_test.go. M-156 step 3 adds the 4-character prefix attempt and step 4 the `no session named` expectation plus the 8-character rule. Matches `lookupSessionStrict` (`len(arg) < minDestructivePrefix` returns `noSessionError`). Traceability script OK: 228. |

Only docs changed between the previous verification and HEAD (git diff 4f581e08..HEAD: 11-SECURITY.md and TESTING.md); no Go or frontend source moved, and the re-run spot-checks above confirm no regression.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `main.go` launchMode | version mapping, allowlist, usage error | VERIFIED | `TestLaunchMode` passes |
| `cmd_cli.go` / `cmd_share.go` | kill/rename by name | VERIFIED | `lookupSessionStrict` used by both |
| `dispatch_test.go`, `cmd_cli_test.go`, `cmd_share_test.go` | pinning tests | VERIFIED | pass under `-race` |
| `internal/relay/hub.go`, `cwd_test.go` | IN-03 guard and test | VERIFIED | test passes |
| `internal/pty/cleanup.go` | IN-02 merge | VERIFIED | `TestKillSession_*` pass |
| `internal/webserver/resume_test.go`, `input_queue_test.go` | dialResumePing | VERIFIED | pass |
| `internal/daemon/types.go` | PathClickAPI removed | VERIFIED | no hits; `TestHealthInfo` passes |
| Two vitest files | 30 s timeouts | VERIFIED | present, pass |
| `TESTING.md` | Phase 16 note, M-156 | VERIFIED | current with post-review tests |
| `.planning/phases/11-file-preview/11-SECURITY.md` | secure-phase output | VERIFIED | exists, tracked, verified, 0 open |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `main()` | `launchMode` | switch on mode, error to stderr, exit 2 | WIRED (unchanged; table test passes) |
| `cmdKill`/`cmdRename` | `lookupSessionStrict` then `client.KillSession`/`RenameSession(s.ID)` | direct calls | WIRED |
| `joinPasteViewer` | `dialResumePing` | direct call | WIRED |

### Requirements Coverage

All four PLAN files declare `requirements: []`, matching the ROADMAP ("none new"); REQUIREMENTS.md maps no ID to Phase 16, so there are no orphans.

### Anti-Patterns Found

None added by this phase. The only changes since the last run are documentation.

### Info (not blockers)

- 16-REVIEW-FIX.md says a redundant `arg != ""` guard was dropped, but `cmd_share.go` still has it. Harmless, recorded as open in 16-REVIEW-DISPOSITION.md.
- Carried review items IN-01 (name shadows full ID) and IN-02 (ED 3 in alt screen still clears hub history) stay open with reasons in the disposition file.
- `11-SECURITY.md` Notes record two grep-only gates (T-11-24, the App.tsx half of T-11-33) and VM node names/100.x addresses in 11-SPIKE.md and 11-VM-CHECKS.md as candidate redaction. Non-blocking, pre-existing Phase 11 evidence.
- `14-04-SUMMARY.md` frontmatter is not parseable YAML (pre-existing, not touched by Phase 16).

### Human Verification Required

1. **M-156, Tart VM launch check (before ship)**
   **Test:** TESTING.md Category BG, M-156: launch from Finder, Dock, `open -a Portty` (plus the two `--args` forms) in the Tart VM `portty-test`; over SSH run `--version`, `-v`, `--bogus` with exit codes and `pgrep -x Portty`; rename and kill by name, a duplicate name, and a 4-character ID prefix.
   **Expected:** every launch path opens the window; `--version`/`-v` exit 0; `--bogus` exits 2 with the usage error; no new process for CLI calls; kill/rename by name work; duplicate name and short ID prefix are refused and the sessions remain.
   **Why human:** real LaunchServices launches of a signed bundle cannot be driven in process and must not run on this host. TESTING.md records it as "not yet run; must PASS before ship".

### Gaps Summary

No gaps. The one prior gap (11-SECURITY.md) is closed with a committed, verified artifact whose spot-checked evidence holds at HEAD, and the TESTING.md drift warning is fixed. SC1-SC4 and SC6 hold at HEAD (targeted `-race` Go tests, vitest, go vet, gofmt and the traceability script all clean). The status is `human_needed` solely because M-156 is a required real-bundle check that automation cannot perform.

---

_Verified: 2026-11-23_
_Verifier: Claude (gsd-verifier)_

## Human Verification Result

M-156 PASS on 2026-11-23 in `portty-test` (macOS 27.2), build `v1.2.0-vm17` from 57cb9ca2: every launch path (Finder, Dock reopen, cold Dock launch, `open -a`, `--args -NS…`, `--args -Apple…`) showed the window; `--version`/`-v` rc 0 and `--bogus` rc 2 with no Portty process; duplicate name and 4-character ID prefix refused, rename/kill by name and by 8-character prefix worked. Status set to passed per the autonomous human-validation route.
