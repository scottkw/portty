---
phase: 16-v1-2-cleanup
verified: 2026-11-23T13:45:00Z
status: gaps_found
score: 5/6 must-haves verified
covered_files:
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
covered_digest: "v4:sha256:e711919bbb40eb0932f07a94d67d70d88425398e856dfa534304c510eec0daf7"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC5: `/gsd-secure-phase 11` produces 11-SECURITY.md"
    status: failed
    reason: "No 11-SECURITY.md exists in .planning/phases/11-file-preview/ (only the older 11-SECURITY-REVIEW.md). Known and expected: the orchestrator runs the command after this verification. Every other clause of SC5 is met."
    artifacts:
      - path: ".planning/phases/11-file-preview/11-SECURITY.md"
        issue: "missing"
    missing:
      - "Run /gsd-secure-phase 11 (a command, not a replan), then re-verify SC5 only"
human_verification:
  - test: "M-156 (TESTING.md Category BG): launch Portty from Finder, Dock, `open -a Portty`, `open -a Portty --args -NSDocumentRevisionsDebugMode YES` and `-ApplePersistenceIgnoreState YES` in the Tart VM `portty-test`; over SSH run `Portty --version`, `-v`, `--bogus` with `echo $?` and `pgrep -x Portty` before and after; rename and kill by name with `portty rename`/`portty kill`, including a duplicate name"
    expected: "Every launch shows the window; --version and -v print the version and exit 0; --bogus prints `portty: unknown flag \"--bogus\"` plus usage and exits 2; no new Portty process for any CLI call; rename and kill by name act on the named session; a duplicate name is refused and both sessions remain"
    why_human: "Real LaunchServices launches of a signed bundle cannot run in process or on the user's Mac. launchMode is table-tested in process, but the entry point for Finder, Dock, open and login items on the remote Mac is a costly-to-get-wrong path (D-01, rock-solid priority). Recorded in TESTING.md as not yet run; must PASS before ship."
---

# Phase 16: v1.2 Cleanup Verification Report

**Phase Goal:** Clear the doable tech debt from the v1.2 re-audit before the milestone closes (CLI kill-by-name, `--version`/unknown flags, load-robust DoS tests, Phase 15 review info items, planning hygiene, one clean full suite run, TESTING.md).
**Verified:** 2026-11-23
**Status:** gaps_found (one gap, a pending command, not a code defect)
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `portty kill <name>` resolves names like share/rename; a test pins it | VERIFIED | `cmd_cli.go` cmdKill and cmdRename call `lookupSessionStrict` (wraps `lookupSession`; names and full IDs always pass, ID prefixes need 8+ chars). Re-ran under `-race`: `TestCmdKill_ByName`, `TestCmdRename_ByName`, `TestCmdKillRename_ShortPrefixRefused`, `TestResolveSession` all PASS. Todo moved to `todos/completed` with Resolution. |
| 2 | `Portty --version`/`-v` prints the version with no GUI; unknown flags never start the GUI | VERIFIED in process; real launch check pending (see human verification) | `main.go` `launchMode`: `--version`/`-v` map to `[version]` (modeCLI, and `cliNeedsDaemon` skips the daemon for `version`); the allowlist is exactly `-psn_`, `-NS`, `-Apple`; anything else returns modeHelp plus error, and `main` prints it plus usage and exits 2. `TestLaunchMode` (18 rows, including near-miss `-psn` and bare `--`) PASS. I did not run the built binary: any mistake would start the GUI on this host. |
| 3 | The two DoS-bound vitest tests are load-robust without loosening the bound | VERIFIED | `git diff` shows only `, 30_000` and a comment added to each `it`; inputs (`repeat(10000)`, `length: 1_000`) and `expect` lines unchanged; `localhostUrl.ts`/`markdown.ts` untouched. Re-ran the two files: 2 passed, 70 tests. Full run under load (16-04): both pass. |
| 4 | Phase 15 IN-02, IN-03, IN-06 resolved or recorded; `PathClickAPI` removed or consumed | VERIFIED | IN-03: `hub.go` guards `History.Skip` with `!emu.IsAltScreen()`; `TestCwd_AltScreenScrollbackEraseKeepsRows` PASS (RED-first per SUMMARY). IN-02: `cleanup.go` has one merged `if s.cmd == nil \|\| s.cmd.Process == nil \|\| s.reaped.Load()`; IN-01 sentence present; `TestKillSession_*` PASS. IN-06: `dialResumePing` used by `joinPasteViewer`; `TestShare_PasteBacklogKeepsViewer`, `TestWSS_ResumeDeltaAfterReconnect` PASS. `git grep PathClickAPI -- '*.go'` exits 1; `APIVersion` still 5, `TestHealthInfo` PASS. 15-REVIEW-DISPOSITION rows marked fixed. |
| 5 | Planning hygiene: SUMMARY `requirements-completed` covers INPT-01..06, PREV-02/04, MOB-01..05; `/gsd-secure-phase 11` produces 11-SECURITY.md; selection-offset todo triaged | PARTIAL: FAILED on 11-SECURITY.md | Read back through `summary-extract`: Phase 10 union INPT-01..06 (10-01, 10-02, 10-07), Phase 11 gains PREV-02/PREV-04 on 11-10 (PREV-01/03/05 already on 11-03, 11-08), Phase 14 MOB-01..05 on 14-14. Selection-offset todo has a `## Resolution` naming 7a37bfb4 and its evidence. **`.planning/phases/11-file-preview/11-SECURITY.md` does not exist** (only `11-SECURITY-REVIEW.md`). Expected to be produced by the orchestrator next; not done yet. |
| 6 | Full Go -race and vitest pass once, clean; fixed todos move to completed; TESTING.md updated per standing rule | VERIFIED (with a documentation warning) | 16-04 logs (`tmp.sAUD92yTnm`): `t2-done` = "every command exited 0"; 14 `ok` packages, no FAIL; vitest 122 files / 1826 tests passed. That run preceded the post-review fixes (c8158c94, 0e08afca), so I re-ran the full root package `-race -short`: ok (27.8 s); go vet and gofmt clean; `check-traceability-paths.sh`: OK, 228 paths. The three todos are in `todos/completed`; only dompurify-3418 and v12-release-checks remain pending (both out of scope). TESTING.md has the Phase 16 Section 2 note, a corrected PATH-02 row and Category BG M-156. |

**Score:** 5/6 truths verified (SC5 partial: the SUMMARY and triage clauses pass, the 11-SECURITY.md clause does not exist yet).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `main.go` launchMode | version mapping, allowlist, usage error | VERIFIED | substantive and wired in `main()` (error print plus exit 2) |
| `cmd_cli.go` / `cmd_share.go` | kill/rename by name | VERIFIED | `lookupSessionStrict` is called by both |
| `dispatch_test.go` | TestLaunchMode table | VERIFIED | 18 rows, passes |
| `internal/relay/hub.go`, `cwd_test.go` | IN-03 guard and test | VERIFIED | |
| `internal/pty/cleanup.go` | IN-02 merge | VERIFIED | |
| `internal/webserver/resume_test.go`, `input_queue_test.go` | dialResumePing | VERIFIED | |
| `internal/daemon/types.go` | PathClickAPI removed | VERIFIED | |
| Two vitest files | 30 s timeouts | VERIFIED | |
| `TESTING.md` | Phase 16 note, M-156 | VERIFIED (see warning) | |
| `.planning/phases/11-file-preview/11-SECURITY.md` | secure-phase output | MISSING | pending orchestrator step |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `main()` | `launchMode` | switch on mode, error to stderr, exit 2 | WIRED |
| `launchMode` | version command | `[]string{"version"}` to `runCLI`, `cliNeedsDaemon("version")` false | WIRED |
| `cmdKill`/`cmdRename` | daemon | resolved `s.ID` only, from the daemon's own list | WIRED |
| `joinPasteViewer` | `dialResumePing` | direct call | WIRED |

### Requirements Coverage

All four PLAN files declare `requirements: []`, matching the ROADMAP ("none new"). No IDs to cross-reference and no orphans: REQUIREMENTS.md maps no ID to Phase 16. Side check: INPT-01..06, PREV-01..05, MOB-01..05 are all `[x]`/Complete in REQUIREMENTS.md and now all appear in some SUMMARY `requirements-completed`.

### Anti-Patterns Found

No TBD/FIXME/XXX/TODO/HACK markers added by the phase's non-planning diff. No stubs.

### Warnings and Info (not blockers)

- **WARNING, TESTING.md drift:** the post-review fixes added `TestCmdKillRename_ShortPrefixRefused` (cmd_cli_test.go) and an empty-argument row in `TestResolveSession` (cmd_share_test.go), but the TESTING.md Phase 16 note lists only `TestCmdKill_ByName` and `TestCmdRename_ByName` for those files, and does not list cmd_share_test.go. The M-156 text also does not mention the 8-character minimum for ID prefixes on kill and rename. The standing rule asks for updates when tests are added; a one-line fix.
- **INFO, REVIEW-FIX inaccuracy:** 16-REVIEW-FIX.md says the redundant `arg != ""` guard in the prefix pass was dropped, but `cmd_share.go` still contains `if arg != "" && strings.HasPrefix(s.ID, arg)`. The review disposition correctly records it as open (harmless).
- **INFO, carried review items:** IN-01 (name shadows full ID), IN-02 (ED 3 in alt screen still clears hub history) stay open with reasons in 16-REVIEW-DISPOSITION.md. IN-05 of Phase 15 stays open deliberately (not in SC4).
- **INFO, pre-existing:** `14-04-SUMMARY.md` frontmatter is not parseable YAML (`summary-extract` errors on it). Not touched by Phase 16, its list is empty, and the Phase 14 union read-back succeeds per file for 14-14. A whole-phase reader that iterates every SUMMARY would trip on it.

### Human Verification Required

1. **M-156, Tart VM launch check**
   **Test:** see frontmatter and TESTING.md Category BG.
   **Expected:** every launch path opens the window, `--version`/`-v` exit 0, `--bogus` exits 2, no new process for CLI calls, kill/rename by name work, duplicate names refused.
   **Why human:** real LaunchServices launches of a signed bundle cannot be driven in process, and must not run on this host.

### Gaps Summary

One gap: `11-SECURITY.md` for Phase 11 does not exist, so SC5 is not fully met. This is the known sequencing, closed by running `/gsd-secure-phase 11` and re-checking that file exists; no replan is needed. Everything else holds against the code: the CLI fixes, launch dispatch, review items and DoS-test timeouts are present, wired and passing when re-run; the hygiene frontmatter reads back correctly; the full suites were clean (the root package re-run after the review fixes). M-156 remains a required human check before ship.

---

_Verified: 2026-11-23_
_Verifier: Claude (gsd-verifier)_
