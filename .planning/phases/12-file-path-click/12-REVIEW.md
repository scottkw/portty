---
phase: 12-file-path-click
reviewed: 2026-11-22T00:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - cmd_open.go
  - frontend/src/App.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/lib/openLink.ts
  - frontend/src/lib/pathLinks.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/terminalScale.ts
  - internal/daemon/engine.go
  - internal/daemon/preview.go
  - internal/daemon/types.go
  - internal/daemon/zshshim.go
  - internal/daemon/zshshim.zsh
  - internal/previewfile/previewfile.go
  - internal/relay/cwd.go
  - internal/relay/handoff_state.go
  - internal/relay/history.go
  - internal/relay/hub.go
  - internal/relay/pathclick.go
  - internal/relay/preview.go
  - internal/relay/protocol.go
  - internal/relay/server.go
  - internal/webserver/server.go
findings:
  critical: 0
  warning: 2
  info: 6
  total: 8
  necessity: 1
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-11-22
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

Reviewed the Phase 12 diff (`git diff dfff923d`) for the 24 source files above. Test files were read only for intent. `go vet` on the touched Go packages, `gofmt -l` and `tsc --noEmit` are clean. The relay, previewfile and webserver tests for path click, cwd, handoff and open pass under `-race`. The frontend ESLint check could not run: the repo has no `eslint.config.*`, which is not a Phase 12 change.

I looked for ways to break the security contract (printed token, writer-only, `previewfile.Open` checks, nothing logged or broadcast) and found none:

- Read-only subscribers are refused in both pumps and again at the top of `Hub.PathClick`.
- `hasToken` and `tokenAt` boundary logic holds against prefix, suffix, URL and scp-style inputs.
- `parseOSC7` rejects control characters, relative paths, foreign hosts and oversize payloads.
- The ZDOTDIR shim restores the user's `ZDOTDIR` before any later startup file is read, falls back to "set nothing" on any write or verify failure, and cannot leak the shim directory into child environments.
- No path through `resolvePathClick` can panic the daemon.

The findings below are a correctness gap in relative-path resolution, a missing liveness check on the requesting subscriber, and smaller maintainability items. No Blockers.

## Warnings

### WR-01: A relative click resolves against the newest print of that text, not the occurrence the user clicked

**File:** `internal/relay/pathclick.go:93-97` (and `firstPass`, `:256-273`)
**Issue:** `findPrinted` returns the row of the newest logical line that contains the clicked token, and `resolvePath` then joins the text to the directory in effect at that row. The click frame carries only the text, so the host cannot tell which occurrence was underlined. When the same relative text appears in two directories, clicking the older one silently previews the file from the newer directory. A common case is `git status` or `ls` in repo A, then `cd` to repo B and the same command printing `src/main.go`. The user sees the wrong file with no notice. This is the "last occurrence" rule from 12-CONTEXT, but the accepted-risk list in 12-SECURITY-REVIEW.md does not name it. It lists nested shells, hard wraps and F-1, which is a different cause.
**Fix:** Either have the viewer send the absolute buffer row of the hovered link and have the host search only the logical line at that row (the host already numbers rows absolutely), or record this as an accepted limit in 12-SECURITY-REVIEW.md and the user docs. A cheap host-only mitigation is to refuse (`pcNoCwd`) when the token is found under more than one distinct directory, instead of silently choosing the newest.

### WR-02: `Hub.PathClick` does not check that the requesting subscriber is still joined and not revoked

**File:** `internal/relay/pathclick.go:56-69`
**Issue:** The cheap checks cover `sub == nil`, `ReadOnly`, payload shape and the hub state. They never confirm `h.subscribers[sub]` or `!sub.revoked`. `pathNotice` (`:125`) does make that check, so the code knows the state can occur. A click frame already read by a pump when a share is revoked or the viewer leaves still triggers a file read, a `SendPreview` broadcast to every remaining viewer, and a log line for a viewer whose access ended. The window is small and the viewer was trusted a moment earlier, but it is an authorization check that is missing on a path that reads host files. The SECURITY-REVIEW states condition (b) as "the requesting subscriber can write".
**Fix:** Do the check inside the existing `h.mu` block:
```go
h.mu.Lock()
_, joined := h.subscribers[sub]
idle := joined && !sub.revoked && !h.closed && !h.freezing && !h.previewBusy
h.mu.Unlock()
```

## Info

### IN-01: Test-only knobs live in production structs

**File:** `internal/relay/server.go:18-21`, `internal/relay/hub.go:101-104`
**Issue:** `Server.testReadOnly` and `Hub.pathClickResolved` are fields that production never sets. `testReadOnly` flips every local subscriber to read-only on a path that guards a security refusal, so a stray assignment would silently disable path clicks. Both are documented, and the unexported names keep them in-package.
**Fix:** Acceptable as is. If it is changed later, prefer a test-file option such as a constructor taking a subscriber-decorating func, so the production type has no mutable test switch.

### IN-02: Generality without instantiation: `PathClickAPI` is read by nothing but a test

**File:** `internal/daemon/types.go:69-73`
**Category:** necessity
**Issue:** `PathClickAPI` has no non-test reader. The comment says viewers do not read it and that it "documents the feature". `APIVersion` carries the same value of 4, and the capability bit on the replay meta is the real gate. `PreviewAPI` is read by `cmd_open.go` (`PreviewAPI` gate), which is why that constant earns its place; this one does not. The premise is that the constant is not consulted at any call site, shown by `grep -rn PathClickAPI` (only `types.go` and `client_test.go`). CONTEXT records it as an explicit planning decision ("APIVersion 4 with a PathClickAPI constant"), so this rests on judgment.
**Fix:** Keep it if the project wants the symmetry, or drop the constant and keep `APIVersion = 4` with its comment.

### IN-03: `stripLineCol` in `pathLinks.ts` is exported and used only by its test

**File:** `frontend/src/lib/pathLinks.ts:21-24`
**Issue:** `findPathLinks` strips the suffix through its `LINE_COL` regex, and the host strips it again in Go. The exported TypeScript helper has no production caller. A second, differently written definition of "line:col suffix" (`:\d+(?::\d+)?$` here, a digit-trim loop in Go) is one more place for the two sides to drift.
**Fix:** Remove the export, or have `findPathLinks` use it so the test pins live code.

### IN-04: `~/` expansion has no guard for a service-mode `$HOME` of `/` or `.`

**File:** `internal/relay/pathclick.go:152-156`, `internal/relay/cwd.go:143`
**Issue:** `resolvePath` accepts any absolute `os.UserHomeDir()`. `engine.go` (WR-01 comment) already recognises that a service-mode daemon can have `$HOME` of `/` or `.`. With `/`, a clicked `~/notes/a.md` resolves to `/notes/a.md`, which is a wrong or missing file rather than a security problem, since the token was printed. With `.` it is refused by the `IsAbs` check.
**Fix:** Mirror the engine guard: `if err != nil || home == "/" || !path.IsAbs(home) { return "", false }`.

### IN-05: A ref is written during render in `TerminalPanel`

**File:** `frontend/src/components/TerminalPanel.tsx` (`otherBannerRef.current = ...`, in the pathNotice block)
**Issue:** `otherBannerRef.current` is assigned in the render body from props and state. The render condition `pathNotice && isActive && !otherBannerRef.current` and the effect then read it. This works today, but the assignment is a render side effect (it is not safe under concurrent rendering or StrictMode double render), and `showPathNotice` closes over the ref from a mount-time callback.
**Fix:** Compute `const otherBanner = !!pasteNotice || reconnecting || (isGuestRef.current && sessionEnded.ended)` as a plain value for the render condition, and mirror it into the ref in a `useEffect` for the callback's use.

### IN-06: Unrelated pointer and selection changes ride along, and one reaches into xterm internals

**File:** `frontend/src/lib/terminalScale.ts:84-116`, `frontend/src/components/TerminalPanel.tsx` (`macOptionClickForcesSelection: true`, `compensatePointerScale(term)`)
**Issue:** `compensatePointerScale` replaces `getCoords` and `getMouseReportCoords` on xterm's private `_core._mouseService` for every terminal. It also changes TUI mouse-report coordinates, not just link hover. `macOptionClickForcesSelection` changes Option-click behaviour for mouse-tracking TUIs. Both are outside path click (the first is the "selection-offset" fix, the second a copy aid). `pathLinks.ts` also relies on private `_core.linkifier` fields, but that one is guarded and pinned by a test. For `compensatePointerScale`, a failed lookup only logs a warning once per page (`warnedNoMouseService` is module-global, so only the first terminal reports it).
**Fix:** Keep, but make sure the selection-offset and Option-click changes have their own entries in TESTING.md and the manual checklist. Consider a per-terminal warning flag so a second terminal that hits a renamed internal is not silent.

### IN-07: Viewer and host token alphabets differ for numeric non-decimal characters

**File:** `frontend/src/lib/pathLinks.ts:15` vs `internal/relay/pathclick.go:309`
**Issue:** The viewer's `RUN` uses `\p{N}`, which includes `Nl` and `No` (for example `²`, `½`, `Ⅷ`). The host's `isPathChar` uses `unicode.IsDigit`, which is `Nd` only. A path adjacent to such a character is tokenised differently on each side, so the host can refuse (`not-printed`) a link the viewer underlined. The error direction is safe: it never loosens the printed check. `path-click.json` does not pin this class.
**Fix:** Use `unicode.IsNumber` in `isPathChar` so it matches `\p{N}`, and add a fixture row such as `{ "line": "/tmp/a²/b.png", ... }`.

---

_Reviewed: 2026-11-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
