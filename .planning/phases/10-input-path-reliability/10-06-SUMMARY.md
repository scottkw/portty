---
phase: 10-input-path-reliability
plan: 06
subsystem: relay hub grid, typed-input classifier, both read pumps, portty attach
status: complete
tags: [inpt-06, grid-owner, typing-claim, classifier, attach, sigwinch]
requires: [10-04]
provides:
  - "relay.IsUserInput(filtered []byte) bool: the typed-input classifier"
  - "Hub.NoteTyping(sub) error with the ownerFast / ownerStale atomic mirrors"
  - "both read pumps call NoteTyping for filtered MsgInput that IsUserInput accepts"
  - "attach.MakeClientClaimFrame; WatchResize sends it on SIGWINCH"
affects: [10-07 (TESTING.md rows, VM greps for why=input, M-117 attach/typing checks)]
tech-stack:
  added: []
  patterns: ["lock-free owner fast path on two atomics, slow path gridMu then mu", "stateless per-frame classifier run on the absorber's output"]
key-files:
  created: []
  modified:
    - internal/relay/oscabsorb.go
    - internal/relay/oscabsorb_test.go
    - internal/relay/hub.go
    - internal/relay/hub_grid_owner_test.go
    - internal/relay/server.go
    - internal/webserver/server.go
    - internal/webserver/claim_test.go
    - internal/daemon/attach_viewers_test.go
    - internal/attach/attach.go
    - internal/attach/attach_unix.go
    - internal/attach/attach_test.go
decisions:
  - "A departed viewer's typing changes nothing: NoteTyping's slow path checks that sub is still subscribed. The share pump's read goroutine can outlive Unsubscribe, and a departed owner would otherwise hold the grid with nobody left to clear it"
  - "ownerStale is stored on every owner report as (reported size != target), so a report back to the grid's own size also returns the owner to the fast path"
  - "A lone trailing ESC and a trailing ESC O are unterminated, so they do not claim. InputAbsorber already holds a bare ESC until the next byte, so the classifier never sees one alone from a live viewer"
  - "The connect-report shortcut comment sits on MakeClientResizeFrame in attach.go, because the plan keeps cmd_attach.go unedited"
metrics:
  duration: "~11 min wall clock from start to gate end (02:52 to 03:03 UTC), about 4 min of it the full Go run"
  completed: 2026-11-21
estimate:
  tokens: 100000
  tasks: 2
actuals:
  tokens: 7000
  tasks: 2
  commits: 4
plan_head_before: 50eb867ddcb37cbc5f620d8af9e1b9e2797bc08c
plan_head_after: 247893a036b8199ddaabdd354a8a26ae0947cc66
---

# Phase 10 Plan 06: Typing and `portty attach` resize claim the grid (INPT-06) Summary

Typing now takes the grid. When a write-capable viewer that has reported its size sends real keystrokes, it becomes the grid owner and the shell reflows to its last reported size. When the owner types after a newer report (for example a sidebar toggle that only reported), that report is applied. Terminal auto-replies never count as typing in any viewer: cursor and status reports, DA, DECRPM, focus, mouse, and OSC/DCS strings. `portty attach` claims on SIGWINCH, and its connect-time size is still only a report.

## Exported API

```go
// internal/relay/oscabsorb.go
func IsUserInput(filtered []byte) bool

// internal/relay/hub.go
func (h *Hub) NoteTyping(sub *Subscriber) error

// internal/attach/attach.go
func MakeClientClaimFrame(cols, rows uint16) []byte // [0x11, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]
```

Unexported: `Hub.setOwnerLocked(sub)` is now the one place that sets `owner`, `ownerFast`, the target and `ownerStale`. ClaimGrid, the bootstrap, NoteTyping and Unsubscribe (with nil) all go through it. `attach.watchResize(ctx, conn, winch <-chan os.Signal, size func() (int, int, error))` is the test seam, and `WatchResize` calls it with `signal.Notify` and `term.GetSize`.

## Classifier rule set as implemented

`IsUserInput` scans the filtered bytes and returns true at the first byte that counts as user input. It returns false if no byte does, including for an empty slice.

Not user input:
- A CSI (`ESC [`) whose parameter bytes include a private marker (`<` `=` `>` `?`, anywhere in the parameters), or that has an intermediate byte (0x20-0x2F), or whose final byte is one of `R n c t y I O M m`.
- An X10 mouse report: `ESC [ M` plus three raw bytes, whatever they are.
- An OSC, DCS, SOS, PM or APC string (`ESC ]`, `ESC P`, `ESC X`, `ESC ^`, `ESC _`). It ends at BEL or at ST (`ESC \`). An ESC followed by anything else aborts the string, and scanning resumes at that ESC.
- An unterminated sequence at the end of the frame: a lone `ESC`, `ESC O`, a CSI with no final byte, an unclosed string, or a short X10 report.
- A CSI that is aborted by a C0 byte, ESC or a byte above 0x7E before its final byte. That byte is then classified on its own, so a C0 byte or DEL counts as user input.

User input: any byte other than ESC outside a sequence (text, UTF-8, CR, LF, TAB, other C0 controls, DEL), `ESC` + any other byte (Alt keys), an SS3 key (`ESC O` + byte), and a CSI with no marker, no intermediate and another final byte (arrows `A-D`, `H`, `F`, `P-S`, `Z`, `~` function keys and bracketed-paste markers, kitty `CSI code;mods u`).

The two `shortcut:` limits are in the doc comment. The classifier is stateless per frame, so a reply split across two frames can claim once from its tail. And `CSI 1;5R` (a modified F3) looks like a cursor report, so it does not claim.

**No reply class needed a rule change during testing.** Every planned case passed on the first GREEN run. That covers the table, attach_viewers_test.go's queryReplies, the full concatenation, and a 15 s fuzz run (374k execs, no failure).

## NoteTyping model

- Fast path: `ownerFast.Load() == sub && !ownerStale.Load()` returns with no lock. TestHub_GridTypingOwnerFastPath and TestHub_GridTypingOwnerNewerReport prove it by calling NoteTyping while the test holds `gridMu`.
- Slow path: gridMu then mu, which is the existing order. Nothing changes if sub is read-only, no longer subscribed, or has never reported a size (`sub.Cols <= 0`). Otherwise `setOwnerLocked(sub)` runs. A frozen hub records the change and Thaw applies it. Otherwise `applyGridLocked("input")` runs, which releases mu before the broadcast, the emulator resize and resizeFn, as before.
- `ownerStale` is written under mu in sizeClient's report branch, but only when the reporting viewer is the owner.
- Diag line: `viewer-diag: grid session=%s %dx%d -> %dx%d by=%s client=%q conn=%d why=input` (the 10-04 format with a new `why` value).

## Tests added or extended (for 10-07's TESTING.md)

| File | Tests |
|------|-------|
| internal/relay/oscabsorb_test.go (extended) | TestIsUserInput, FuzzIsUserInput (shared `autoReplies` table, which includes a copy of queryReplies) |
| internal/relay/hub_grid_owner_test.go (extended) | TestHub_GridTypingClaims, TestHub_GridTypingOwnerFastPath, TestHub_GridTypingOwnerNewerReport, TestHub_GridTypingNeedsReport, TestHub_GridTypingReadOnly, TestHub_GridOwnerLeaveClearsFast (helpers: captureDiag, gridLine, typingReturnsUnderGridMu, hubOwner) |
| internal/webserver/claim_test.go (extended) | TestWebTyping_TakesGrid, TestWebTyping_AutoReplyNoClaim, TestWebTyping_NoReportNoClaim |
| internal/daemon/attach_viewers_test.go (extended) | TestCLIAttach_TypingTakesGrid (new); TestCLIAttach_Hypotheses/replayed_query_replies now ends with `s.checkUndisturbed(t, "guiTab", "browser")` |
| internal/attach/attach_test.go (extended) | TestMakeClientClaimFrame, TestWatchResize_SendsClaim; TestMakeClientResizeFrame unchanged (still 5 bytes) |

## TDD Gate Compliance

| Task | RED commit | RED evidence | GREEN commit |
|------|-----------|--------------|--------------|
| 1 (tracer) | f1e48408 | TestIsUserInput: 17 `typing must claim` failures. `B typing: grid [100 30] ... want 80x24`, `the owner's typing after a newer report: grid [100 30]`, `a departed owner's typing still took the lock-free fast path`, and in the daemon `timed out after 2s waiting for host hub grid 80x24 after the CLI typed`. TestHub_GridTypingOwnerFastPath, NeedsReport and ReadOnly passed on RED, because a do-nothing stub already matches the behaviour they pin. | 483920b3 |
| 2 | 62aab71a | `MakeClientClaimFrame(80, 24) = [17 0 80 0 24], want [... 1]` and `frame after SIGWINCH = [17 0 90 0 30], want the claim`. The three TestWebTyping_* cases passed on RED, because Task 1 had already wired the share pump. They pin that wiring. | 247893a0 |

Each RED commit ships compile-only stubs, so it builds and fails on assertions. Task 1's stubs: IsUserInput returns false, NoteTyping does nothing. Task 2's stubs: MakeClientClaimFrame returns the report, and the watchResize seam still sends the report. Tracer gate: after GREEN, Task 1's automated verify ran end to end and passed (all TestCLIAttach_ included) before Task 2 started.

## Gate results

All logs are in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/`.

- Task 1 verify: `p10-06-t1.log` (relay -run set, then all TestCLIAttach_): ok / ok. Extra targeted runs are in `p10-06-t1b.log`: TestHub_Handoff*, TestHub_Resize*, TestRelay*, and the 15 s fuzz. All ok.
- Task 2 targeted runs: `p10-06-t2.log`, ok / ok.
- `go build ./... && go vet ./...`: clean.
- **The single full run** `go test -race -short -count=1 ./...`: exit 0, 0 `--- FAIL` / `^FAIL`, 0 DATA RACE, 0 `device not configured`, 13 packages ok, no reruns. Log: `p10-06-gate-go.log`. It includes Phase 9's TestHub_HandoffRejoinKeepsGrid.
- check-brand: pass (`p10-06-brand.log`). check-traceability-paths: pass (`p10-06-trace.log`).
- Guards: `MaxInputPayload` is still 32767. cmd_attach.go still sends `MakeClientResizeFrame(` on connect and was not edited. `git diff d027e86d` on csp_mw.go, capability_mw.go and tailnet/live.go is empty. No input payload is logged anywhere new.

## Deviations from Plan

1. **[Rule 2 - Correctness] A departed viewer's typing cannot take the grid.** The plan's slow path did not include a subscriber-membership check. The share handler runs `conn.CloseNow` and then `Unsubscribe`, but its read goroutine can still be handling a frame at that point. A NoteTyping that arrived after Unsubscribe would make a dead subscriber the owner forever. NoteTyping now changes nothing for a sub that is not in `h.subscribers`. TestHub_GridOwnerLeaveClearsFast asserts that the owner stays nil. (Commit 483920b3.)
2. **Ledger and logs in the scratchpad.** The plan base is in `.../scratchpad/p10-06-base`, and the gate logs use the scratchpad instead of `/tmp`, as in 10-03/04/05.
3. **Lone trailing ESC / ESC O.** The plan did not classify these explicitly. They are treated as unterminated, so they do not claim. The absorber never emits a bare ESC on its own, so this does not affect the Escape key in practice.

No architectural changes. No package installs. TESTING.md is untouched (10-07 owns it).

## Known Stubs

None. The RED-commit stubs are fully replaced at HEAD.

## Threat Flags

None new. T-10-20 is mitigated: the classifier table, the fuzz run and the replayed-query scene's checkUndisturbed. T-10-21 is mitigated: the read-only discard in the pumps plus TestHub_GridTypingReadOnly, and a viewer with no reported size cannot take ownership (TestHub_GridTypingNeedsReport, TestWebTyping_NoReportNoClaim). T-10-22 is mitigated: the lock-free owner fast path. T-10-23 is accepted as planned.

## Self-Check: PASSED

- All 11 files in key-files exist.
- Commits f1e48408, 483920b3, 62aab71a and 247893a0 are ancestors of HEAD (`git rev-list --count 50eb867d..HEAD` = 4).
