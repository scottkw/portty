---
created: 2026-11-17T17:26:17.244Z
title: Document multi-line prompt resize artifacts in user docs
area: docs
severity: cosmetic
files:
  - frontend/src/components/TerminalPanel.tsx:819
---

## Problem

With a multi-line zsh prompt (seen with Powerlevel10k's two-line style), resizing the
Portty window leaves stray copies of the prompt's top line (e.g. `╭─  ~/dev/temp`)
in the scrollback. Found during Phase 4 two-Mac UAT on 2026-11-17.

It happens only when the window gets smaller (narrower), never when it grows (user
observation, 2026-11-17). Narrowing wraps the prompt lines, adding rows above the
cursor; zsh moves up by the old row count and falls short. Widening adds no rows.

Cause: each resize sends SIGWINCH, zsh redraws the prompt, the terminal reflows old
text and zsh's cursor lands one line off, orphaning the top prompt line. Portty makes
it worse: `TerminalPanel.tsx:819` calls `fitTerminal` straight from a `ResizeObserver`
with no debounce, so one window drag resizes the PTY many times and leaves many lines.

Decision (2026-11-17, user): no code fix for the initial release. Document it instead.

## Solution

Add a "Known issues" entry to the user documentation with these two items:

1. **Known behavior.** Making the window narrower can leave copies of the prompt's top
   line behind; making it bigger never does. The leftover lines are cosmetic and do not affect the session.
   Clear them with `clear` or Ctrl-L. (Portty has no Cmd-K clear shortcut today —
   checked 2026-11-17.) A future release may wait ~100 ms after a drag stops before
   resizing the shell, cutting it to at most one leftover line per drag.
2. **Workaround.** Use a single-line prompt. For Powerlevel10k, run `p10k configure`
   and choose the one-line style (or set it in `~/.p10k.zsh`).

Not planned: prompt-aware reflow via shell integration (OSC 133) — days of work.
