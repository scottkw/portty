# Phase 16: v1.2 Cleanup - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-23
**Phase:** 16-v1.2 Cleanup
**Areas discussed:** Unknown leading flags

---

## Unknown leading flags

| Option | Description | Selected |
|--------|-------------|----------|
| Accept all (Recommended) | Reject unknown flags with usage + exit 2; keep a narrow macOS launch-arg allowlist (-psn_*, -NS*, -Apple*) that still opens the GUI | ✓ |
| Strict reject | Every unknown flag exits 2, including -psn_/-NS args; risks Finder/login-item launches | |
| Only add --version/-v | Other unknown flags keep launching the GUI; misses criterion 2 | |

**User's choice:** Accept all (Recommended)
**Notes:** None

---

## Decided autonomously

[auto] [CLI] — Q: "How does kill resolve a session name?" → Selected: "lookupSession/resolveSession in cmdKill and cmdRename, test pins kill-by-name" (reversible technical — decided autonomously: two CLI call sites, no daemon API change)
[auto] [CLI] — Q: "How is --version / -v handled?" → Selected: "launchMode maps both to the version subcommand" (reversible technical — decided autonomously: one switch case)
[auto] [Tests] — Q: "How are the DoS-bound tests made load-robust?" → Selected: "explicit per-test timeout sized to the input; bound and input unchanged" (reversible technical — decided autonomously: test-only change)
[auto] [Review] — Q: "IN-02 duplicate killSession branches?" → Selected: "merge per the review's fix, add IN-01 doc sentence" (reversible technical — decided autonomously: behavior-identical local refactor)
[auto] [Review] — Q: "IN-03 ED 3 History.Skip during alt screen?" → Selected: "!emu.IsAltScreen() guard plus cwd_test.go case" (reversible technical — decided autonomously: one guard line plus a test)
[auto] [Review] — Q: "IN-06 and PathClickAPI?" → Selected: "ping hook on dialResume; remove PathClickAPI unless a consumer exists" (reversible technical — decided autonomously: test helper and an unused constant)
[auto] [Hygiene] — Q: "Triage the selection-offset-and-copy todo?" → Selected: "already fixed at 7a37bfb4; record triage" (reversible technical — decided autonomously: docs only)

## Claude's Discretion

- Plan split and wave order.

## Deferred Ideas

None
