# Phase 12: File Path Click - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-21
**Phase:** 12-file-path-click
**Areas discussed:** Shell & security, Click UX, Scope

---

## Shell & security

| Option | Description | Selected |
|--------|-------------|----------|
| Accept all | ZDOTDIR shim for zsh; daemon reads only printed paths, only for write-capable viewers, with portty-open checks | ✓ |
| Change Q1 | proc_pidinfo cwd (no shell changes, ignores OSC 7, wrong inside ssh) or TERM_PROGRAM=Apple_Terminal (pulls in Apple session-restore) | |
| Change Q2 | Also allow read-only viewers | |
| Discuss deeper | One question at a time | |

**User's choice:** Accept all

---

## Click UX

| Question | Options | Selected |
|----------|---------|----------|
| Who gets the preview tab | Every viewer / Only the clicker / You decide | Every viewer |
| What Cmd-hover underlines | Any path-like text / Only real files / You decide | Any path-like text |
| Existing but unpreviewable file | Nothing happens / Brief notice / You decide | Brief notice |

---

## Scope

| Option | Description | Selected |
|--------|-------------|----------|
| App + peer tabs only | Matches roadmap SC; browser reworked in Phase 14 | |
| Browser too | Parity with Phase 11 previews | ✓ |
| You decide | | |

---

## Decided autonomously

[auto] [CWD tracking] — Q: "Where is OSC 7 parsed?" → Selected: "In the daemon, from PTY output, interleaved with retained history" (reversible technical — decided autonomously: internal to the daemon)
[auto] [CWD tracking] — Q: "Which cwd does a relative path resolve against?" → Selected: "The cwd in effect when the path was printed" (reversible technical — decided autonomously: resolver logic only)
[auto] [CWD tracking] — Q: "OSC 7 from another host?" → Selected: "Ignored unless empty, localhost, or own hostname" (reversible technical — decided autonomously: one check)
[auto] [Click detection] — Q: "How are paths detected?" → Selected: "xterm link provider; strip :line[:col]; ~ expanded on host" (reversible technical — decided autonomously: frontend-only)
[auto] [Transport] — Q: "How does a click reach the host?" → Selected: "New relay message on the session stream, API version gate" (reversible technical — decided autonomously: Phase 11 pattern)
[auto] [File read] — Q: "Which checks before reading?" → Selected: "portty open's checks moved into shared code" (reversible technical — decided autonomously: moved, not new)
[auto] [Shells] — Q: "bash and fish?" → Selected: "No injection; user's own OSC 7 only" (reversible technical — decided autonomously: matches PATH-01)

## Claude's Discretion

Plan split, file names, test names, regex details, notice visuals.

## Deferred Ideas

None.
