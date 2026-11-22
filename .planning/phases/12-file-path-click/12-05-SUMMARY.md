---
phase: 12-file-path-click
plan: 05
subsystem: webserver
tags: [go, share-server, path-click, api-version, security-review]

requires:
  - phase: 12-file-path-click
    provides: "12-04 Hub.PathClick, MsgPathClick, the replay-meta pathClick bit, TestPathClick_SourceGate"
provides:
  - "internal/webserver/server.go: the share read pump's MsgPathClick arm, read-only subscribers skipped"
  - "internal/webserver/pathclick_test.go: write cap previews with focus, read cap gets no bit, read cap refused with a write-cap positive control"
  - "APIVersion 4 and PathClickAPI 4 (PreviewAPI 3, MinDaemonAPI 1 unchanged)"
  - "12-SECURITY-REVIEW.md: the PREV-03 amendment, argument, honest scope, accepted risks, shim rules, unchanged CSP, threat register T-12-01..T-12-28, test pins"
affects: [12-06 viewer, 12-07 TESTING.md consolidation and review verdict]

actuals:
  tokens: 5500
  tasks: 2
  commits: 3
plan_head_before: 12b90fd332be5a4d67c4fcb0087c38f93b83d966
plan_head_after: 8972b9e518c9d5e64921b3e54e88c42a5109cecb

tech-stack:
  added: []
  patterns:
    - "MC-03 shape in a second pump: `if !sub.ReadOnly` before the hub call, with the hub refusing again"

key-files:
  created:
    - internal/webserver/pathclick_test.go
    - .planning/phases/12-file-path-click/12-SECURITY-REVIEW.md
  modified:
    - internal/webserver/server.go
    - internal/daemon/types.go
    - internal/daemon/client_test.go
    - internal/daemon/api_test.go

key-decisions:
  - "No per-click log line in the share pump (a link holder can send these at line rate); the hub logs one outcome line per resolve"
  - "Read-only is enforced in the share pump, the relay pump and Hub.PathClick on purpose: every minted share is read,write today, so a single check could rot unnoticed"
  - "PathClickAPI is documentation for /health and /info readers; viewers follow the replay-meta bit"

requirements-completed: [PATH-02]

coverage:
  - id: D1
    description: "A write-capable share viewer's click on a printed path previews the file with focus for it, and its replay meta carries pathClick:true"
    requirement: PATH-02
    verification:
      - kind: integration
        ref: "internal/webserver/pathclick_test.go#TestWebPathClick_WriteCapPreviews"
        status: pass
    human_judgment: false
  - id: D2
    description: "A read-only capability gets no capability bit, and its click reaches nothing while a write capability's identical click on the same hub previews"
    requirement: PATH-02
    verification:
      - kind: integration
        ref: "internal/webserver/pathclick_test.go#TestWebPathClick_ReadCapNoCapabilityBit"
        status: pass
      - kind: integration
        ref: "internal/webserver/pathclick_test.go#TestWebPathClick_ReadCapRefused"
        status: pass
    human_judgment: false
  - id: D3
    description: "The daemon reports API 4 and PathClickAPI 4; MinDaemonAPI stays 1"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/daemon/client_test.go#TestHealthInfo"
        status: pass
      - kind: unit
        ref: "internal/daemon/api_test.go#TestAPIHealth"
        status: pass
    human_judgment: false
  - id: D4
    description: "The security review records the amendment, argument and accepted risks, with no secrets, and the share CSP keeps frame-ancestors 'none'"
    requirement: PATH-02
    verification:
      - kind: other
        ref: "12-05 gate: review section greps, secret-pattern grep, csp_mw.go frame-ancestors check, check-brand.sh, check-traceability-paths.sh"
        status: pass
    human_judgment: true
    rationale: "Whether the argument is persuasive is a sign-off judgment; 12-07 adds the VM evidence and the verdict"

duration: 25min
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 05: Share server path clicks, API 4 and the security review Summary

**Browser and peer viewers with a write capability can now Cmd-click printed paths through the share server's read pump, a read-only capability's click reaches nothing, the daemon reports API 4, and 12-SECURITY-REVIEW.md records the PREV-03 amendment and its argument.**

## Accomplishments

- **The arm:** `case relay.MsgPathClick` in the share read pump of `internal/webserver/server.go`, beside `MsgInput`; it calls `hub.PathClick(sub, payload)` only when `!sub.ReadOnly`. No per-click log. The "no case and no default arm" comment for retired bytes is untouched.
- **API values:** `APIVersion = 4` (comment extended: "4 adds path clicks (PathClickAPI)"), `PathClickAPI = 4` beside `PreviewAPI = 3`, `MinDaemonAPI = 1` unchanged. Pins updated in `internal/daemon/client_test.go` (TestHealthInfo) and `internal/daemon/api_test.go` (TestAPIHealth, `"api":4`).
- **Tests** (`internal/webserver/pathclick_test.go`, real share server, real hub, real capability tokens, a real PNG under a short `/tmp` directory so the path never soft-wraps): the write capability's replay meta has `pathClick:true` and its click gets a `MsgPreview` header (shot.png, image/png, focus true); the read capability's replay meta has no `pathClick` key; the read capability's click yields no preview and no notice in 500 ms, and then a write capability's identical click on the same hub previews.
- **RED evidence:** with the pump call replaced by a no-op, `TestWebPathClick_WriteCapPreviews` failed (timed out waiting for a header); restored and committed.
- **The review** (`12-SECURITY-REVIEW.md`, sections in order): Scope and parts; PREV-03 amended (amendment word for word, conditions (a)(b)(c) with enforcement point and test); Why the contract holds (9 points); Honest scope; Accepted risks; Shell integration; What stays unchanged; Threat register (T-12-01 to T-12-28, T-12-SC noted); Tests that pin it. 12-07 appends "VM evidence and verdict".

## Test names (for 12-07's TESTING.md rows)

New file needing a Section 4 traceability row (PATH-02): `internal/webserver/pathclick_test.go` with `TestWebPathClick_WriteCapPreviews`, `TestWebPathClick_ReadCapNoCapabilityBit`, `TestWebPathClick_ReadCapRefused`. Changed pins: `TestHealthInfo` (`internal/daemon/client_test.go`), `TestAPIHealth` (`internal/daemon/api_test.go`). TESTING.md was not edited, per the plan.

## Gate results

- `go build ./...`, `go vet ./...`: pass.
- One full `go test -race -short -count=1 ./...` (log `/tmp/p12-05-gate-go.log`): 14 packages `ok`, zero `--- FAIL` or `^FAIL` lines; `internal/webserver` and `internal/daemon` both `ok`.
- Review greps, secret-pattern grep (0 matches), `frame-ancestors 'none'` still in `csp_mw.go`, `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh`: pass.
- **`--- SKIP` lines in the gate: none** (the gate runs without `-v`, which prints none).

## Task Commits

1. **Task 1: share pump arm and tests** - `8b07fdc0` (feat)
2. **Task 2a: API 4 and PathClickAPI with pins** - `79cadf84` (feat)
3. **Task 2b: security review** - `8972b9e5` (docs)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, instruction conflict] Commit attribution trailer**
- **Issue:** The plan says commits end with `Co-Authored-By: Claude Opus 5.5`; the session's attribution instruction specifies `Claude Sonnet 5.5`.
- **Fix:** Used the session attribution line (it takes precedence). Same as 12-04.
- **Files modified:** none (commit messages only)

**2. [Plan wording] Gate script run from a file with `|| true` on the `go test` line**
- The verify block's full suite line was run through a script so it could run in the background (about 6 minutes); `|| true` lets the next line count `--- FAIL|^FAIL`, which is the same check the plan makes. No test failed.

**3. [Plan wording] Task order of the Task 2 commits**
- API change and review are committed separately as the plan's action lists.

**Total deviations:** 1 instruction conflict, 2 wording differences. **Impact:** none on deliverables.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the one in the plan's threat model (T-12-25 to T-12-28): one pump arm gated on the capability, a version constant, and a document.

## Next Phase Readiness

12-06 builds the viewer (links only when the replay meta says `pathClick:true`; the host now serves that bit and the frame to browser and peer viewers). 12-07 appends the VM evidence and verdict to `12-SECURITY-REVIEW.md`, adds the TESTING.md rows above, and runs manual checks.

## Self-Check: PASSED

- Created files present: internal/webserver/pathclick_test.go, .planning/phases/12-file-path-click/12-SECURITY-REVIEW.md.
- Commits 8b07fdc0, 79cadf84, 8972b9e5 are ancestors of HEAD; `git rev-list --count` from the ledger base gives 3.
