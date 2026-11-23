# Phase 16: v1.2 Cleanup - Context

**Gathered:** 2026-11-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Clear the doable tech debt from the v1.2 re-audit (`.planning/v1.2-MILESTONE-AUDIT.md`, 2026-11-23) before the milestone closes: CLI kill-by-name, `--version`/unknown-flag handling, two load-sensitive DoS-bound vitest tests, Phase 15 review info items IN-02/IN-03/IN-06 plus the unused `PathClickAPI` constant, planning-artifact hygiene (SUMMARY `requirements-completed`, 11-SECURITY.md, selection-offset todo triage), one clean full Go -race + vitest run, and TESTING.md per the standing rule.

Out of scope: real-Mac/release-gated checks (v12-release-checks, M-93 notarized delivery), dompurify-3418 (blocked until 2026-11-28), the user-accepted limits (--status-top overdraw, passive CLI watch). The sidebar dot stays green for a reachable Mac with nothing shared (user decision 2026-11-23).

</domain>

<decisions>
## Implementation Decisions

### Unknown leading flags (GUI vs CLI launch)
- An unknown leading flag prints the error plus usage to stderr and exits 2; it never starts the GUI. A narrow allowlist of macOS launch-service argument shapes (`-psn_*`, `-NS*`, `-Apple*`) still launches the GUI. `--help`/`-h` and the daemon flags keep their current handling. — **Reversibility:** costly — `launchMode` is the entry point for Finder, Dock, `open` and login-item launches on the remote Mac; a wrong rejection means the app cannot be opened there and nobody can get in to fix it (rock-solid priority), so the allowlist must be pinned by a `launchMode` table test covering each shape.

### Decided autonomously
- [CLI] Q: "How does kill resolve a session name?" → Selected: "cmdKill resolves its argument with lookupSession/resolveSession (exact name, exact ID, unique prefix) and kills the resolved ID; cmdRename gets the same treatment because it also passes the raw argument to the daemon as an ID; a test pins kill-by-name" — two CLI call sites, no daemon API change
- [CLI] Q: "How is --version / -v handled?" → Selected: "launchMode maps --version and -v to the version subcommand, which already skips the daemon (cliNeedsDaemon)" — one switch case
- [Tests] Q: "How are the DoS-bound tests made load-robust?" → Selected: "Keep input sizes and assertions unchanged; give each test an explicit per-test timeout sized to its input (or count work instead of wall time if the test measures time); never shrink the hostile input" — test-only change
- [Review] Q: "IN-02 duplicate killSession branches?" → Selected: "Merge into one condition per the review's exact fix; add the IN-01 doc-comment sentence (leftover group members of a reaped shell are intentionally left running) while there" — behavior-identical local refactor
- [Review] Q: "IN-03 ED 3 History.Skip during alt screen?" → Selected: "Guard with !emu.IsAltScreen() and add a cwd_test.go case" — one guard line plus a test
- [Review] Q: "IN-06 joinPasteViewer and the unused PathClickAPI constant?" → Selected: "Give dialResume an optional ping hook and have joinPasteViewer use it; remove PathClickAPI and its client_test.go assertion unless a production consumer is found (check graft callers first)" — test helper and an unused constant
- [Hygiene] Q: "Triage the selection-offset-and-copy todo?" → Selected: "Already fixed at 7a37bfb4 and filed in todos/completed; record the triage as done, no code change" — docs only

### Claude's Discretion
- Plan split and wave order are the planner's call; the planning-hygiene items (SUMMARY frontmatter, `/gsd-secure-phase 11`) can run alongside the code items.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `resolveSession` / `lookupSession` (`cmd_share.go:15-64`), already used by share/unshare/attach; `noSessionError` gives the friendly no-match message. `TestResolveSession` (`cmd_share_test.go:65`) shows the table-test shape.
- `cliNeedsDaemon` already skips the daemon for `version`, `health`, `open`, `--help`.
- `unknownCommandMessage` and `usage()` in `main.go` for the error text.

### Established Patterns
- `launchMode` (`main.go:73-90`) returns modeGUI/modeHelp/modeCLI; flags before a command go through `flagFirstArgs` (WR-04).
- `cmdKill` (`cmd_cli.go:314`) and `cmdRename` (`cmd_cli.go:325`) pass `args[0]` straight to the daemon as an ID, which is why kill-by-name 500s.
- `killSession` cleanup branches: `internal/pty/cleanup.go:30-46`.
- ED 3 hook: `internal/relay/hub.go:1301-1305`.
- `dialResume` (`internal/webserver/resume_test.go:117-124`), `joinPasteViewer` (`internal/webserver/input_queue_test.go:46-50`).
- `PathClickAPI` (`internal/daemon/types.go:70-74`) is only read by `client_test.go:50`.
- DoS tests: `frontend/src/lib/localhostUrl.test.ts` (classifyLoopbackUrl hostile input), `frontend/src/lib/markdown.test.ts` (renderMarkdown DoS bounds, indentation-nested list).

### Integration Points
- TESTING.md Sections 2/4/5 (standing rule); `bash tests/check-traceability-paths.sh` before committing.
- SUMMARY frontmatter `requirements-completed` in phases 10, 11, 14; `/gsd-secure-phase 11` writes `11-SECURITY.md`.
- Fixed todos move from `.planning/todos/pending/` to `completed/`.

</code_context>

<specifics>
## Specific Ideas

- The todo says `portty rename <name>` worked in the VM, but `cmdRename` passes the raw argument as an ID too; verify rename-by-name before claiming it works and fix both the same way.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>
