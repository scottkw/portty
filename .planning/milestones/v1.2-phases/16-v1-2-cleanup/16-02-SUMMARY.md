---
phase: 16-v1-2-cleanup
plan: 02
subsystem: relay
tags: [go, relay, pty, webserver, daemon, vitest, review-cleanup]
status: complete

requires:
  - phase: 15-v1-2-hardening
    provides: 15-REVIEW IN-01, IN-02, IN-03, IN-06 and the DoS-bound test todo
provides:
  - "ED 3 inside an alt-screen round trip no longer double-counts main-screen scrollback, so a line printed before a later cwd report keeps its own directory"
  - "killSession has one early return for no process or a reaped shell, with the IN-01 sentence recorded"
  - "dialResumePing, the ping-hook dial helper dialResume wraps; joinPasteViewer uses it"
  - "PathClickAPI constant removed; APIVersion stays 5"
  - "Two DoS-bound vitest cases carry an explicit 30 s per-test timeout"
affects: [16-04 dispositions and TESTING.md, v1.2 milestone close]

actuals:
  tokens: 2300
  tasks: 3
  commits: 3
plan_head_before: d4bfe08b779edbd1c4d41a0cc87e310b72c6d8ae
plan_head_after: 7aa3fbb306c90256729e693ce674379807feb801

tech-stack:
  added: []
  patterns:
    - "Go test helper pair: dialResume wraps dialResumePing the way dialWebServerWS wraps dialWebServerWSPing"

key-files:
  created: []
  modified:
    - internal/relay/hub.go
    - internal/relay/cwd_test.go
    - internal/pty/cleanup.go
    - internal/webserver/resume_test.go
    - internal/webserver/input_queue_test.go
    - internal/daemon/types.go
    - internal/daemon/client_test.go
    - frontend/src/lib/__tests__/localhostUrl.test.ts
    - frontend/src/lib/__tests__/markdown.test.ts

key-decisions:
  - "IN-03: guard History.Skip with !emu.IsAltScreen() only; h.history.Clear() left as is (does not touch the row counter)"
  - "IN-06: Go has no optional parameters, so the optional ping hook lives in dialResumePing and dialResume wraps it with a nil hook; signature and five callers unchanged"
  - "DoS tests: 30 000 ms per-test timeout, inputs and assertions untouched (about 60x headroom over the 0.5 s alone time)"

requirements-completed: []

coverage:
  - id: D1
    description: "ED 3 written while the alternate screen is active no longer adds the main screen's undrained scrollback to History.Pushed (15-REVIEW IN-03)"
    verification:
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_AltScreenScrollbackEraseKeepsRows"
        status: pass
      - kind: unit
        ref: "go test -race -short ./internal/relay/... (full package)"
        status: pass
    human_judgment: false
  - id: D2
    description: "killSession merged early return plus IN-01 doc sentence, behaviour-identical (15-REVIEW IN-01, IN-02)"
    verification:
      - kind: unit
        ref: "internal/pty/adopt_test.go#TestKillSession_ReapedSkipsGroupSignal"
        status: pass
      - kind: unit
        ref: "internal/pty/adopt_test.go#TestKillSession_RunningSignalsGroup"
        status: pass
    human_judgment: false
  - id: D3
    description: "joinPasteViewer dials through dialResumePing; no second copy of the Origin, URL and read-limit setup (15-REVIEW IN-06)"
    verification:
      - kind: integration
        ref: "internal/webserver/input_queue_test.go#TestShare_PasteBacklogKeepsViewer"
        status: pass
      - kind: integration
        ref: "internal/webserver/resume_test.go#TestWSS_ResumeDeltaAfterReconnect"
        status: pass
    human_judgment: false
  - id: D4
    description: "PathClickAPI removed from every .go file; /health still reports APIVersion 5"
    verification:
      - kind: unit
        ref: "internal/daemon/client_test.go#TestHealthInfo"
        status: pass
      - kind: other
        ref: "git grep PathClickAPI -- '*.go' exits 1 (no match)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The two DoS-bound vitest tests keep hostile inputs and assertions and carry a 30 s per-test timeout; pass alone and beside a full vitest run"
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/localhostUrl.test.ts and markdown.test.ts: 3 runs alone, 1 run beside full pnpm test"
        status: pass
    human_judgment: false
---

# Phase 16 Plan 02: Review Cleanup and DoS-Test Timeouts Summary

**ED 3 inside the alternate screen no longer double-counts main-screen scrollback (path-click rows), plus the IN-02/IN-06 tidy-ups, the unused PathClickAPI constant removed, and 30 s timeouts on two load-sensitive vitest cases.**

## Accomplishments

- **IN-03 (real fix, tracer):** the CSI `J` handler in `internal/relay/hub.go` now calls `h.history.Skip(emu.ScrollbackLen())` only when `!emu.IsAltScreen()`. vt's ED 3 clears the current screen's scrollback while `ScrollbackLen` is always the main screen's, so with the alternate screen active nothing is dropped and counting it pushed rows past their real values. `TestCwd_AltScreenScrollbackEraseKeepsRows` was written first and failed on the unfixed code (`MARK-A resolves to "/b" (found true), want "/a"`), then passed with the guard. Commit 0af0cd46.
- **IN-02 and IN-01:** `killSession` has one `if s.cmd == nil || s.cmd.Process == nil || s.reaped.Load() {` branch (same body); `closePTY()` calls in the file went from 4 to 3. The doc comment records that leftover members of a reaped shell's process group are intentionally left running. Commit 97611082.
- **IN-06:** `dialResumePing` holds the old `dialResume` body but dials through `dialWebServerWSPing`; `dialResume` is a one-line wrapper with a nil hook. `joinPasteViewer` calls `dialResumePing(t, ws, client, sid, token, "", onPing)`; the now-unused `net/http` import was dropped from `input_queue_test.go`. Commit 97611082.
- **PathClickAPI:** the constant, its doc comment, the mention in APIVersion's comment (now "viewers gate them on the replay meta's pathClick bit") and the `TestHealthInfo` clause are gone. `APIVersion` is still 5; `TestHealthInfo` still pins APIVersion 5, PortProxyAPI 5 and PreviewAPI 3. Commit 97611082.
- **DoS-bound tests:** `classifyLoopbackUrl` hostile-input case and the indentation-nested list case got `, 30_000` as the `it` timeout plus a one-line comment each. No input or expect line changed; `localhostUrl.ts` and `markdown.ts` are untouched. Commit 7aa3fbb3.

## Measurements

- Alone on 2026-11-23: localhostUrl hostile case about 519 ms, indentation-nested list case about 319 ms; the failing run hit vitest's 5 s default under parallel load (more than 10x slower). 30 000 ms gives about 60x headroom and matches the repo's existing `10_000` per-test style.
- Proof: three runs of the two files alone each "2 passed, 70 tests"; one run with `--reporter=verbose` beside a full `pnpm --dir frontend test` passed ("2 passed, 70 tests"), and the full run itself exited 0 (122 files, 1826 tests). The full run is load, not the phase gate (16-04).
- Go: `go test -race -short -count=1 ./internal/relay/...` passed (200.8 s). Targeted pty, webserver and daemon runs passed (see below). Logs live in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.PgrOvfBJmQ` (t1-red.log is the RED run, t1-*, t2-*, t3-*).

## Observations (no change in this phase)

- `trimLinkTail` in `frontend/src/lib/localhostUrl.ts` slices and splits the whole remaining string once per trailing character, so it is quadratic in a run of `)`; that is the 519 ms. Terminal lines bound the input in practice, and CONTEXT makes this item test-only.

## Deviations from Plan

None - plan executed exactly as written. Two harmless execution hiccups, neither a deviation: a BSD `sed -i` call failed before any edit (the import was removed with a Python one-liner instead), and the first Task 2 verify attempt ran with an unset log dir (read-only-filesystem errors); it was rerun in full with the log dir set.

## Known Stubs

None.

## Threat Flags

None. T-16-04 (cwd row correctness) mitigated by the IN-03 guard and test; T-16-05 (killSession merge) mitigated, both kill tests pass; T-16-06 accepted (documentation constant only); T-16-07 mitigated (grep gates on inputs and assertions).

## Commits

| Task | Commit | Description |
| ---- | ------ | ----------- |
| 1 | 0af0cd46 | ED 3 alt-screen guard on History.Skip plus RED-first cwd test |
| 2 | 97611082 | killSession merge, dialResumePing, PathClickAPI removal |
| 3 | 7aa3fbb3 | 30 s timeouts on the two DoS-bound vitest cases |

## Self-Check: PASSED

- All nine modified files present and committed; commits 0af0cd46, 97611082, 7aa3fbb3 are ancestors of HEAD (3 commits since plan_head_before d4bfe08b).
- Acceptance greps verified: IsAltScreen guard directly above Skip; merged killSession condition (1), `closePTY()` count 3, "intentionally left running" (1), `func dialResumePing(` (1), `dialResumePing(t, ws, client, sid, token, "", onPing)` (1), no `dialWebServerWSPing(` in input_queue_test.go, `const APIVersion = 5` (1), `git grep PathClickAPI -- '*.go'` exit 1, `}, 30_000)` in both vitest files, `repeat(10000)` x3, `length: 1_000 }` and `toBeLessThanOrEqual(MAX_DEPTH)` unchanged, empty `git diff` for localhostUrl.ts and markdown.ts.
