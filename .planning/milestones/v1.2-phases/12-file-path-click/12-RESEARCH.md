# Phase 12: File Path Click - Research

**Researched:** 2026-11-21
**Domain:** zsh startup-file shim (OSC 7), PTY-stream cwd tracking in the Go relay hub, a viewer-to-host click message with a host-side "printed" check, xterm.js 6 link providers
**Confidence:** HIGH on the repo mechanics and the zsh shim (read the code and ran real zsh 5.9 on macOS this session); MEDIUM on the xterm live-underline toggle inside Wails' WKWebView (read xterm's source, not yet run in the real web view)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Shell integration**
- zsh emits OSC 7 through a ZDOTDIR shim (the Ghostty / VS Code method): the daemon points ZDOTDIR at a Portty-owned directory whose zsh startup files restore the user's real ZDOTDIR (or unset it), source the user's own files in the normal order, then add a chpwd hook (plus one initial emit) that prints OSC 7 — **Reversibility:** costly — it runs inside every user's shell startup on every session; a mistake breaks shells on an unattended remote Mac, and changing the mechanism later means re-testing every startup-file combination. Must not change behaviour for users with no ZDOTDIR, a custom ZDOTDIR, or a login shell that isn't zsh.

**Security model for path clicks**
- The daemon reads a clicked file only when all hold: (a) the clicked text appears in that session's own retained output; (b) the requesting viewer can write to the session (read-only subscribers are refused, since `ls ~/.ssh` would otherwise hand them files they could not otherwise read); (c) the file passes the same checks as `portty open` (regular file, non-blocking open re-checked on the descriptor, 20,000,000-byte cap, image/PDF/Markdown sniffing). This amends Phase 11's "the daemon never opens a path given to it" rule; the security review must record the amendment and the PREV-03 argument — **Reversibility:** one-way — it is the security contract for host file reads; loosening it later is easy but tightening it after release breaks behaviour users rely on, and the review must sign off on it.

**Click UX**
- Every viewer gets the preview tab, as with `portty open`: the clicking viewer switches to it, other viewers get it as a background tab.
- Cmd-hover underlines any path-like text (no host round trip). Clicking a path that doesn't exist does nothing: no tab, no dialog, nothing typed into the shell (SC3).
- Cmd-clicking a file that exists but can't be previewed (wrong type, over the cap) shows a brief transient notice in the clicking window only (e.g. "Can't preview this file type" / "Too large to preview"). Missing files stay silent.
- A plain click still selects text; URLs keep their existing click behaviour (confirm popover, then browser).

**Scope**
- Path clicks also work in the browser share view, not only app and peer tabs — parity with Phase 11 previews. (Corrected 2026-11-21 after UI research: share links redirect to `/app/?session=…&cap=…` (`internal/webserver/server.go` handleTerminalPage), which mounts the same React `TerminalPanel`, so one implementation covers local, peer and browser. `web/assets/terminal.js` has no preview support and is out of scope; the original pointer to it was the orchestrator's mistake, not the user's choice.) Read-only browser viewers get no clickable paths (or their clicks are refused host-side; host-side refusal is mandatory either way).

**Decided autonomously**
- [CWD tracking] Q: "Where is OSC 7 parsed?" → Selected: "In the daemon, from the PTY output stream; cwd changes are kept interleaved with the retained output history" — internal to the daemon.
- [CWD tracking] Q: "Which cwd does a relative path resolve against?" → Selected: "The cwd in effect when the path was printed, found via the last occurrence of the clicked text in the retained history (ANSI stripped)" — resolver logic only.
- [CWD tracking] Q: "OSC 7 from another host (e.g. inside ssh)?" → Selected: "Ignored unless the hostname is empty, localhost, or the host's own name" — one check.
- [Click detection] Q: "How are paths detected?" → Selected: "An xterm link provider in the frontend; strips a trailing :line[:col]; ~ is expanded on the host" — frontend-only, can be reshaped freely. Note: TerminalPanel.web-links.test.tsx currently asserts the source has no registerLinkProvider call (Phase 95 Plan B for OSC 8); that assertion must be updated deliberately, not deleted blindly.
- [Transport] Q: "How does a click reach the host?" → Selected: "A new relay message on that session's own authenticated stream, gated by an API version bump like PreviewAPI" — same pattern as Phase 11.
- [File read] Q: "Which checks before reading?" → Selected: "Move portty open's file checks into shared code used by both the CLI and the daemon" — code is moved, not reinvented.
- [Shells] Q: "bash and fish?" → Selected: "No injection; tracked only when the user's own setup emits OSC 7. Without OSC 7, absolute paths still work and relative ones do nothing" — matches the PATH-01 wording.

### Claude's Discretion
- Plan split, file names, test names, path-detection regex details, the notice's visual details within the macOS-native look.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PATH-01 | The terminal knows each session's working directory through OSC 7. Portty's zsh setup emits it, and other shells that emit it work too. | zsh `.zshenv`-only ZDOTDIR shim, verified on real zsh (Sec. "zsh shim"); OSC 7 handler registered on the hub's live emulator, absolute-row cwd log (Sec. "cwd tracking"); hostname/URI rules |
| PATH-02 | Cmd-clicking a file path printed in the terminal (absolute, or relative to the working directory) previews it, using the PREV path for local and remote sessions alike. The host previews only paths that session itself printed, so there is still no general file-read API. Paths that don't exist do nothing. | New client frame `MsgPathClick`, `Hub.PathClick` (read-only gate, printed check against History + screen, resolver, shared `previewfile` checks, `SendPreview` with a focus override, unicast notice); xterm path link provider with modifier-gated decorations |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Read: `~/dev/CLAUDE.md`, `CLAUDE.md`, `.claude/CLAUDE.md`. No project skills directory entries exist (`.claude/CLAUDE.md` "Project Skills: No project skills found").

- **TESTING.md is the regression home.** Every phase that adds tests updates Section 2 (manifest counts), Section 4 (traceability rows: path column is a repo-relative `.go/.ts/.tsx/.sh` path only), and Section 5 (a new `M-NN` for anything not automatable). Run `bash tests/check-traceability-paths.sh` before committing. Current state [VERIFIED: TESTING.md:24-32 table and `grep` of Section 5]: Go **157** `*_test.go` files, vitest **104**, Playwright **7**, build-script **1**, total **269**; last category is `BA — File Preview`, last manual item is `M-127`. This phase is **Category BB**, manual items start at **M-128**.
- **Chesterton's Fence / deletion discipline.** `cmd_open.go` file checks are *moved*, not rewritten; `TerminalPanel.web-links.test.tsx` Plan B assertion is *edited with a reason*, not deleted.
- **Security:** keep token-gated tailnet-only share links and the CSP intact. This phase needs **no CSP change** (no new resource types; the preview renderers already exist).
- **Silent fallbacks forbidden** ("Let it crash"): where this research says "silent" (missing file), that is a *product decision* (UI-SPEC), implemented as an explicit refusal path with a logged counter, never as `or {}`/default values. A failed shim write must log and spawn without integration, never half-configure ZDOTDIR.
- **Tech stack:** keep Go/Wails/React/tsnet. No new relay servers or cloud dependencies. macOS only. Go: `ctx`-aware, `go fmt`, golangci-lint. JS/TS: pnpm, ESLint + Prettier.
- **GSD enforcement:** no repo edits outside a GSD workflow. Memory: radical simplicity, rock-solid first, macOS-native look, GUI checks in the Tart VMs (never the host).
- **Executor rules inherited from Phase 11 CONTEXT:** `export GOTOOLCHAIN=go1.26.11`; each full suite at most once per plan (every test binary fires a macOS firewall alert); outputs to files, not piped through tail; VM checks only in `portty-test` (office) and `portty-test-2` (home), which are ephemeral tailnet nodes (never stop them, never `tailscale down/logout`).

## Summary

Everything this phase needs already exists as an extension point; nothing new has to be invented on the transport. **The one structural finding that shapes the plan: the hub already runs a live VT emulator fed from the first PTY byte, with OSC handlers registered in byte order inside `emu.Write` under `emuMu`** (`emu.RegisterOscHandler(8, …)` at `internal/relay/hub.go:1354`). OSC 7 is parsed there, not by a second scan of the output. The "retained output" the printed check searches is **`Hub.history` (rendered lines, 10,000 lines / 4 MiB, soft-wrap flags) plus the live screen**, not the 256 KiB raw ring; History is what survives ring wrap and is what Phase 9 handoff moves. cwd changes are interleaved with that history by recording each OSC 7 against an **absolute row number** (`lines pushed to history so far + emulator scrollback + cursor row`), which is monotonic and survives history trimming.

**The zsh shim is much simpler and safer than the CONTEXT wording suggests.** Ghostty's shim is a single `.zshenv`: zsh reads `$ZDOTDIR/.zshenv` first, the shim restores `ZDOTDIR` right there, sources the user's `.zshenv` by hand, and from then on zsh looks up `.zprofile`, `.zshrc`, `.zlogin` and `/etc/zshrc`'s `HISTFILE` in the user's real directory by itself. No four-file shim, no sourcing-order emulation. I built it and ran it on this Mac's zsh 5.9 (login+interactive, PTY): no `ZDOTDIR`, custom exported `ZDOTDIR`, a `ZDOTDIR` set inside the user's `~/.zshenv`, a syntax-error `~/.zshenv`, a `return 1` `~/.zshenv`, a shim dir containing a space, nested zsh, command substitution, redirected stdout, and a user `.zshrc` that resets `chpwd_functions` all behave correctly (evidence below). **The one catastrophic failure mode is a ZDOTDIR that points at a directory without a readable `.zshenv`: zsh then silently skips the user's `.zprofile`/`.zshrc`/`.zlogin` entirely** (verified: no user file ran, `ZDOTDIR` stayed on the shim dir). The daemon must therefore write and verify the shim *before* it sets `ZDOTDIR`, and set nothing if that fails.

Two things differ from what CONTEXT assumes and must be flagged to the planner: (1) sessions always run `/bin/zsh -l` (`sessionShell = "/bin/zsh"`, `$SHELL` is ignored), so "a login shell that isn't zsh" cannot occur at spawn; a user reaches bash/fish only by running them inside the zsh session, where no shim applies. (2) The shim should also emit from `precmd` (Ghostty, Terminal.app and VS Code all do), which is a small addition to CONTEXT's "chpwd hook plus one initial emit"; it is what heals state after a Phase 9 handoff and after a user rc file resets `chpwd_functions` (both shown below). See Open Question 1.

**Primary recommendation:** Build in this order: (1) extract `cmd_open.go`'s file checks into a leaf `internal/previewfile` package; (2) OSC 7 handler + absolute-row cwd log on the hub (with HubState export); (3) the daemon-written `.zshenv` shim with verify-before-set; (4) `Hub.PathClick` with the new client frame, read-only gate, printed check, focus override and unicast notice; (5) frontend path link provider (modifier-gated decorations) and notice; (6) security review, TESTING.md, VM checks. Gate the viewer side on a host-authoritative capability bit in the existing `MsgMeta` replay frame rather than on API-version plumbing the peer path does not have (Open Question 2).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Emit the shell's cwd (OSC 7) | zsh process on the host Mac (shim `.zshenv`) | — | Only the shell knows its cwd; `proc_pidinfo` was rejected in discuss (wrong inside ssh) |
| Daemon-owned shim files and spawn env | Daemon (`SessionEngine.CreateSession`) | — | Same place as the PREV-05 PATH env (`cliPathEnv`); a viewer must never influence it |
| Parse OSC 7, keep the cwd log | Relay hub (live emulator handler, under `emuMu`) | Handoff state export | Byte-ordered with the history; survives the raw ring wrapping |
| Detect path-like text, hover/click | Browser / xterm link provider in `TerminalPanel` | — | Pure display; no host round trip for the underline (CONTEXT) |
| Authorise the click (write-capable, printed text) | Relay hub (`Hub.PathClick`) | Both read pumps call it | Hub owns history + subscribers; both pumps (`relay/server.go`, `webserver/server.go`) share it, so there is one check, not two |
| Resolve and open the file | Leaf `internal/previewfile` package | Hub calls it; CLI calls it | One implementation of the `portty open` checks |
| Deliver the preview | `Hub.SendPreview` (paced, existing) | Peer proxy copies frames verbatim | Reused unchanged apart from a focus override |
| Tell only the requester "can't preview" | Hub unicast (`MsgMeta` field to one `Subscriber`) | `TerminalPanel` banner | Never through `broadcast` |
| Read-only / old-host gating of the provider | Host (capability bit per connection) | Frontend honours the bit | The host is the only authority on both; works for local, peer and browser without extra HTTP |

## Standard Stack

### Core
No new packages. Go side: stdlib (`net/url`, `os`, `path/filepath`, `unicode/utf8`, `embed`) plus `github.com/charmbracelet/x/ansi` v0.11.7 (`ansi.Strip`, already a direct dependency and already used on History renders in `replay_fidelity_test.go:478`) and `github.com/charmbracelet/x/vt` (already vendored in the module cache; `RegisterOscHandler` already used). Frontend: `@xterm/xterm` 6.0.0 (`registerLinkProvider`, `ILink.decorations`) and `@xterm/addon-web-links` 0.12.0, both installed [VERIFIED: `frontend/node_modules/@xterm/xterm/package.json` `"version": "6.0.0"`, `…/addon-web-links/package.json` `"version": "0.12.0"`].

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/url` | Go stdlib (toolchain go1.26.11) | Decode `file://host/path` from OSC 7 | Verified behaviour below; handles `%XX`, rejects a raw `%` |
| `github.com/charmbracelet/x/ansi` | v0.11.7 | `ansi.Strip` on History lines | Already used on these exact strings |
| `@xterm/xterm` | 6.0.0 | `Terminal.registerLinkProvider`, live `link.decorations` | The path link provider API |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `.zshenv`-only shim | VS Code's four-file approach | VS Code's script also does not use OSC 7 (it emits its own `OSC 633;P;Cwd=`), and re-sources `.zshrc` by hand; strictly more code and more ways to change startup order |
| OSC 7 in the emulator handler | A second byte scan of each PTY read | Reads straddle sequences (the hub already needed `incompleteTail`/`vtScan` for that); the emulator handler is already byte-exact |
| Absolute-row cwd log | Tagging each History line with a cwd at push time | Lines drain to History after the screen scrolls, possibly after a later `cd`; the tag would be wrong. Row numbers are assigned when the OSC is parsed |
| Capability bit in `MsgMeta` | `APIVersion` check in the frontend | Peer tabs have no `/info` fetch on the proxy path and local tabs have no API binding; the bit rides the stream every viewer already has |

**Installation:** none. **Version verification:** `node v24.22.1`, `pnpm 12.6.0`, zsh 5.9 (arm64-apple-darwin26.0), Go toolchain pinned `go1.26.11` [VERIFIED: `go.mod:5` `toolchain go1.26.11`; probes run this session].

## Package Legitimacy Audit

This phase installs no external package (Go stdlib and modules already in `go.mod`, npm packages already in `frontend/package.json`). The Package Legitimacy Gate is not applicable.

**Packages removed due to SLOP verdict:** none. **Packages flagged SUS:** none.

## Architecture Patterns

### System Architecture Diagram

```
 office Mac (host)                                                           viewers
 ------------------------------------------------------------------------    -----------------------------------
 daemon CreateSession
   ├─ write+verify <configDir>/zsh/.zshenv (atomic), then env:
   │    ZDOTDIR=<configDir>/zsh   PORTTY_ZSH_ZDOTDIR=<user's ZDOTDIR or "">
   └─ spawn /bin/zsh -l   (PORTTY_SESSION_ID, PATH as in PREV-05)
 zsh: /etc/zshenv → shim .zshenv: restore ZDOTDIR, source user .zshenv,
      then hooks: chpwd + precmd + one emit  ─┐  OSC 7 "file:///url-encoded"
                                              ▼  (written to /dev/tty, so $(…) and >/dev/null can't swallow it)
 PTY master ─► Hub.Run ─► recordFrameLocked (emuMu) ─► live emulator Write
                                  │                      ├─ OSC 7 handler (Hub-owned cwd log: absRow → dir)
                                  │                      └─ lines scroll → History.Push (pushed counter ++)
                                  └─► broadcast MsgOutput to viewers (OSC 7 bytes ride along, xterm ignores them)

 viewer: Cmd-click "docs/diagram.png"
   xterm link provider (writable + host said pathClick:true) ──► RelayClient.sendPathClick(text)
        frame [0x13][utf8 text]  ── local WS / peer proxy copyWS verbatim / share server WS ──►
 pump (relay/server.go | webserver/server.go): case MsgPathClick  → !sub.ReadOnly → hub.PathClick(sub, text)
 Hub.PathClick (cheap checks first):
   1 validate text (≤1024 B, UTF-8, no control chars)   2 CAS pathClickBusy, preview not busy
   3 snapshot under emuMu: History lines+wrap flags+pushed, screen rows, cwd log   (release emuMu)
   4 search newest→oldest, ANSI-stripped, wrap-joined, token boundaries → row   (no match = silent)
   5 resolve: abs | "~/" → home | else cwd-at-row (none = silent); filepath.Clean
   6 previewfile.Open(path): stat/regular/cap/NONBLOCK open/fstat/sniff/key
        missing, dir, unreadable ........................ silent
        unsupported type / too large .................... unicast {"notice":…} to THIS sub only
        ok ──► SendPreview(Preview{…, FocusSub: sub}) ──► paced frames to every viewer
                                                           (requester header.focus=true, others false)
 requester's frontend: Phase 11 assembler → preview tab → App.handlePreview focus rule (unchanged)
```

### Recommended Project Structure
```
internal/previewfile/            # NEW leaf package (no relay/daemon import): Open, MIME, Key, CleanName, typed errors
cmd_open.go                      # now calls previewfile (behaviour and CLI copy unchanged)
internal/relay/cwd.go            # NEW: parseOSC7, cwdLog (absRow→dir, dedupe, prune), unit-tested, no emulator needed
internal/relay/pathclick.go      # NEW: Hub.PathClick, printed search, resolver, busy guards, unicast notice
internal/relay/history.go        # + pushed counter, line/wrap accessors for the snapshot
internal/relay/hub.go            # + OSC 7 handler in newLiveEmulator; cwd log field (Hub-owned, like cs/modes/wraps)
internal/relay/protocol.go       # + MsgPathClick; MetaPayload gains PathClick *bool and Notice *string
internal/relay/preview.go        # Preview gains FocusSub *Subscriber (nil = today's owner rule)
internal/relay/handoff_state.go  # + Cwd state (additive gob fields)
internal/relay/server.go, internal/webserver/server.go   # + case MsgPathClick next to MsgInput
internal/daemon/zshshim.go       # NEW: embed + write/verify + env entries; hooks into CreateSession
internal/daemon/zsh_zshenv       # NEW: the shim script (go:embed), a real file so it can be linted/tested
internal/daemon/types.go         # APIVersion 4, PathClickAPI = 4
frontend/src/lib/pathLinks.ts    # NEW: detection (pure), provider factory, decoration toggling
frontend/src/lib/relayClient.ts  # + MSG_PATH_CLICK, sendPathClick, meta pathClick/notice, onCapabilities/onNotice
frontend/src/components/TerminalPanel.tsx, SessionEndedBanner.tsx   # provider lifecycle in the webLinks arm; 2 banner kinds
```

### Pattern 1: The zsh shim (mechanics, verified)

**What:** one file, `<configDir>/zsh/.zshenv`. The daemon sets `ZDOTDIR=<configDir>/zsh` and `PORTTY_ZSH_ZDOTDIR=<value of the daemon's own ZDOTDIR, or empty>`; the shim restores and gets out of the way.

**Why one file is enough [CITED: raw.githubusercontent.com/ghostty-org/ghostty/main/src/shell-integration/zsh/.zshenv]:** "Restore the original ZDOTDIR value if GHOSTTY_ZSH_ZDOTDIR is set. Otherwise, unset the ZDOTDIR that was set during shell injection." zsh reads `$ZDOTDIR/.zshenv` first (after `/etc/zshenv`); once ZDOTDIR is restored there, `.zprofile` (login), `.zshrc` (interactive), `.zlogin` (login) and `/etc/zshrc`'s `HISTFILE=${ZDOTDIR:-$HOME}/.zsh_history` [VERIFIED: `/etc/zshrc` read this session] are all resolved against the user's real directory. VS Code's script by contrast needs `HISTFILE=$USER_ZDOTDIR/.zsh_history` and a manual `. $USER_ZDOTDIR/.zshrc` because it keeps ZDOTDIR on its dir longer [CITED: raw.githubusercontent.com/microsoft/vscode/main/src/vs/workbench/contrib/terminal/common/scripts/shellIntegration-rc.zsh], and it emits `OSC 633;P;Cwd=`, not OSC 7.

**The script I ran (use as the starting point; all rows of the evidence table were produced with it):**
```zsh
# Portty zsh shell integration (OSC 7 working-directory reports).
# ZDOTDIR is restored first, so every later startup file (.zprofile, .zshrc,
# .zlogin) and /etc/zshrc's HISTFILE are looked up exactly as without Portty.
if [[ -n "$PORTTY_ZSH_ZDOTDIR" ]]; then
  'builtin' 'export' ZDOTDIR="$PORTTY_ZSH_ZDOTDIR"
else
  'builtin' 'unset' 'ZDOTDIR'
fi
'builtin' 'unset' 'PORTTY_ZSH_ZDOTDIR'
{
  'builtin' 'typeset' _portty_f=${ZDOTDIR-$HOME}"/.zshenv"
  [[ ! -r "$_portty_f" ]] || 'builtin' 'source' '--' "$_portty_f"
} always {
  'builtin' 'unset' '_portty_f'
  if [[ -o interactive ]]; then
    __portty_osc7() {
      'builtin' 'emulate' '-L' 'zsh'
      (( ZSH_SUBSHELL )) && return 0
      'builtin' 'setopt' 'extendedglob'
      'builtin' 'local' 'LC_ALL=C'
      'builtin' 'printf' '\e]7;file://%s\a' "${PWD//(#m)[^A-Za-z0-9\/._~-]/%${(l:2::0:)$(( [##16] #MATCH ))}}" 2>/dev/null >/dev/tty
      return 0
    }
    # Re-asserted at every prompt: a startup file that resets chpwd_functions, or a
    # handoff that lost the daemon's state, heals at the next prompt.
    __portty_precmd() {
      if (( ! ${+chpwd_functions[(r)__portty_osc7]} )); then chpwd_functions+=(__portty_osc7); fi
      __portty_osc7
    }
    if (( ! ${+chpwd_functions[(r)__portty_osc7]} )); then chpwd_functions+=(__portty_osc7); fi
    if (( ! ${+precmd_functions[(r)__portty_precmd]} )); then precmd_functions+=(__portty_precmd); fi
    __portty_osc7
  fi
}
```
Design points, each earned by a test below: quoted builtins (the file "can get sourced with aliases enabled", Ghostty's comment); `always {}` so a user `.zshenv` that errors or `return`s still gets our hooks; plain array append instead of `autoload add-zsh-hook` (the user's `.zshenv` may have altered `fpath`; Ghostty sources the user's `.zshenv` first for the same reason); `/dev/tty` instead of stdout (Ghostty uses a dedicated TTY fd "even when STDOUT is redirected" [CITED: ghostty-integration]); `ZSH_SUBSHELL` guard (a `cd` inside `$(…)` or `( … )` does not change the parent's cwd); `emulate -L zsh` so the user's options (`nounset`, `errexit`, `warn_create_global`, …) cannot break the function; the host part of the URL is **empty** (`file:///path`, valid per `net/url`), so a hostname change on the Mac (DHCP/Bonjour renames) can never desync the daemon's host check.

**Evidence (zsh 5.9, `/bin/zsh -l -i`, real PTY, `HOME` = a temp dir with marker `.zshenv/.zprofile/.zshrc/.zlogin`):**

| Scenario | Observed |
|----------|----------|
| No user ZDOTDIR | Order `.zshenv`, OSC 7, `.zprofile`, `.zshrc`, `.zlogin`; `ZDOTDIR=[]` inside; `HISTFILE=<home>/.zsh_history`; shim var unset |
| User ZDOTDIR exported before spawn | All four files ran from the custom dir; `HISTFILE=<custom>/.zsh_history` |
| User `~/.zshenv` sets `ZDOTDIR` itself | `.zprofile/.zshrc/.zlogin` ran from the new dir |
| Syntax-error user `.zshenv` | Parse error printed with the file path, then `.zprofile`, `.zshrc`, `.zlogin` ran, OSC 7 still emitted |
| User `.zshenv` does `return 1` | Same: rest of startup unaffected, OSC 7 emitted |
| Shim dir contains a space (`Application Support/portty/zsh`) | Works; `chpwd_functions=[__portty_osc7]` |
| Non-interactive `-l -c` | No hook, no OSC 7 output; user `.zshenv/.zprofile/.zlogin` ran; `ZDOTDIR=[]` |
| Nested `zsh -i` | User files run, no hooks (the shim unsets its own vars); see Pitfall 5 |
| `x=$(cd /tmp && pwd)` and `(cd /usr)` | `captured=[/tmp]` (no escape bytes captured); no OSC 7 from the subshell |
| `cd /tmp >/dev/null; cd /usr 2>&1 >/dev/null` | OSC 7 for `/tmp` and `/usr` still reached the terminal |
| User `.zshrc` does `chpwd_functions=()` | After the first prompt `cd /usr` emitted `file:///usr` again (the precmd re-registered the hook); **without** the precmd re-assert the same `cd` emitted nothing |
| PWD `dir with space é;x%` | `file:///…/dir%20with%20space%20%C3%A9%3Bx%25` (UTF-8 bytes encoded under `LC_ALL=C`) |
| **Shim dir with no `.zshenv` (failure mode)** | **No user startup file ran at all** and `ZDOTDIR` stayed on the shim dir |

**Failure modes and the rule that prevents each (all are "break a shell on an unattended Mac"):**
1. *ZDOTDIR points at a dir without a readable `.zshenv`* → user rc skipped silently. **Rule:** `refreshZshShim()` writes `<configDir>/zsh/.zshenv` via temp file + `os.Rename` in the same directory (mode 0600 file, 0700 dir), re-`Stat`s it, and only then does `CreateSession` add `ZDOTDIR`/`PORTTY_ZSH_ZDOTDIR`. Any error: log (same style as `cli link: %v`), return nil env, session starts with no integration. Mirror `TestCLILink_FailureStillSpawns` (clilink_test.go:240) and `TestCLILink_ConcurrentRefresh` (:162).
2. *Inherited stale `PORTTY_ZSH_ZDOTDIR`* (daemon started from a Portty session, or a crashed shim) → ZDOTDIR restored to a wrong dir. **Rule:** always pass `PORTTY_ZSH_ZDOTDIR=` explicitly (empty when the daemon has no ZDOTDIR) through the `required` arguments of `mergeEnv` (`internal/pty/native.go:69`), so an inherited value is always overwritten. `mergeEnv` cannot delete keys, hence "empty means unset" in the shim (`[[ -n … ]]`). An exported-but-empty user `ZDOTDIR` is treated as unset (zsh itself would look for `/.zshenv`); negligible.
3. *Shim error text on every new session* → a new zsh version that rejects the script prints errors forever. **Rule:** the integration test (below) runs the shim under the real `/bin/zsh` and asserts empty stderr/no `zsh:` lines on a clean HOME; a CI-visible failure beats a field failure.
4. *Non-zsh spawn in tests* (`/bin/cat`, `/usr/bin/env`): only add the ZDOTDIR entries when `cliPath == sessionShell` (`internal/daemon/engine.go:179`), so every existing env-asserting test is untouched.
5. *Spawn path differs from CONTEXT's belief:* sessions run `sessionShell` (`"/bin/zsh"`, `internal/daemon/engine.go:178-179`: "the binary every session runs; $SHELL and settings are ignored") with `[]string{"-l"}` (`:198`). There is no "login shell that isn't zsh" at spawn, so that clause of the locked decision is satisfied vacuously; bash/fish are only reachable by running them inside the session, where nothing is injected (matches the locked bash/fish decision).

**Interaction with PREV-05 (PATH):** the shim touches neither `PATH` nor path_helper. `cliPathEnv()` returns `"PATH="+…` and the login `path_helper` moves it after system dirs (verified in Phase 11 and by `TestCLILink_SurvivesZshLoginPathHelper`, `internal/daemon/clilink_test.go:266`). That test points `ZDOTDIR` at an empty temp dir to disable rc files and stays valid; add one sibling asserting `command -v portty` still resolves **with the real shim as ZDOTDIR**.

**Phase 9 handoff sessions and pre-existing sessions:** a session spawned by an older build has no shim and never will (`ZDOTDIR` is fixed at exec). Its relative paths do nothing, absolute ones work: exactly the locked "no OSC 7" behaviour. A session carried through a handoff keeps its shell; it keeps working if it was shim-enabled, and the hub's cwd log must be exported/imported (Pattern 3) or else healed by the next prompt's `precmd` emit.

### Pattern 2: OSC 7 parsing in the hub (byte-ordered, under `emuMu`)

**Where output flows [VERIFIED: internal/relay/hub.go]:** `Run()` reads the PTY, then under `emuMu` calls `recordFrameLocked(frame, buf[:n])` and `broadcast(frame)`. `recordFrameLocked` appends the frame to the raw ring, builds the emulator on the first frame (eager, M-51), strips terminal queries, feeds `h.liveEmu.Write(clean)`, then `drainScrollbackLocked()` moves lines that scrolled off the screen into `h.history` (`History.Push`, called at `hub.go:1378` and in `resizeLiveEmulator` at `hub.go:1480`).

**Where to hook:** `newLiveEmulator` is the single constructor of the emulator, so a rebuild (resize, alt-screen exit) can never miss a handler (its own comment: "It is the only constructor, so a rebuild … can never miss a setting"). It already does `emu.RegisterOscHandler(8, func(data []byte) bool { h.cs.osc8(data); return false })` (`hub.go:1354-1357`). Add OSC 7 beside it. Verified vt facts [VERIFIED: `~/go/pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615092313-b57e5e6d29bb/handlers.go:135-146,317-321` and `osc.go:106-125`]: handlers run last-registered-first, `data` includes the leading `7;`, returning `false` lets vt's default (`handleWorkingDirectory`) also run, and that default splits on `;` and ignores any payload with more than one `;` — so **do not rely on `Callbacks.WorkingDirectory`**; parse `data` yourself (a path containing `;` is legal, and the shim encodes it as `%3B` anyway, but third-party emitters may not).

**Parse rules (pure function, unit-test table):** strip `7;`; `url.Parse`; require `Scheme == "file"`; host (case-insensitive, trailing dot trimmed) must be empty, `localhost`, or equal `os.Hostname()` (also accept the same name with a `.local` suffix difference; planner's call, keep it one small function); `u.Path` must be absolute and free of NUL/control characters (Go decodes `%00` and `%0A` into the path [VERIFIED by running `url.Parse`: `file:///a%00b` → path `"/a\x00b"`; `file://host/x%0Ay` → `"/x\ny"`]); then `filepath.Clean`. A raw unencoded `%` in a directory name makes `url.Parse` fail (`invalid URL escape`) → ignore that report (the previous cwd stays); emitters that do not percent-encode are not supported, a documented limitation. Ignore OSC 7 while `emu.IsAltScreen()` (a TUI is not the shell). Cap payload length (1024 B path).

**cwd log, interleaved with History:** `Hub` gets a Hub-owned `cwds` (like `cs`, `modes`, `wraps`: "Hub-owned so it survives rebuilds … Only touched under emuMu"). Each record is `{row uint64, dir string}` where, in the handler,
`row = h.history.pushed + uint64(emu.ScrollbackLen()) + uint64(emu.CursorPosition().Y)` [`ScrollbackLen`/`CursorPosition` verified at `emulator.go:211,457`]. `History` gains a monotonically increasing `pushed uint64` incremented in `Push` (covers both `Push` call sites and imports). The history line at retained index `i` has absolute row `pushed - Len() + i`; screen row `y` has absolute row `pushed + y` after the drain. This is consistent through: scroll (lines leave the screen → counted), `ED 2` (non-empty rows pushed → counted), resize (`resizeLiveEmulator` pushes the visible rows down to the cursor, so their rows keep their numbers), alt-screen exit rebuild (nothing pushed), history trimming (oldest lines dropped, `pushed` unchanged). Known drift only on `RIS` (`emu.ClearScrollback()` drops un-drained lines uncounted): clear the log on RIS, which is also semantically right (a reset terminal forgot its shell state; the next prompt re-emits).
Append only when `dir` differs from the last record; keep at most ~4096 records; prune records older than the oldest retained row **but keep the newest record at or before it** (the floor), so a long-lived line near the start of History still resolves. After `ESC[3J` (`h.history.Clear()` in the `J` handler) `pushed` is untouched (Clear drops lines, not the counter).

**Why rows, not offsets or per-line tags:** a cwd report that arrives in the same 32 KiB read as the output that follows it (`cd docs && ls` in one burst) must order before that output; the handler is byte-exact inside `Write`, whereas `scrollback.End()` at handler time is the end of the whole frame. And a per-line tag assigned at `Push` time is wrong because lines drain after the screen scrolls, possibly after a later `cd`.

### Pattern 3: Handoff state

`HubState` is gob-encoded and additive fields are safe (new daemon importing an old daemon's state sees zero values; the reverse ignores them) [VERIFIED: `internal/relay/handoff_state.go:62-86` struct and its doc: "Every field is plain data that gob encodes"]. Recommended (small): add `Cwds []CwdRecord` and `HistoryPushed uint64`, set on `Export`, restored on import (an import with no `HistoryPushed` sets `pushed = len(history lines)`), plus one round-trip assertion in the existing handoff state tests. Without it, after an upgrade the log is empty: pre-upgrade relative paths are silent until the next prompt's `precmd` emit; post-upgrade ones work. Either way nothing breaks; see Open Question 3.

### Pattern 4: The click message and `Hub.PathClick`

**Wire (recommended; names are the planner's):**
- Client → host: `MsgPathClick byte = 0x13`, payload = the path text as raw UTF-8 (≤ 1024 bytes). `0x13` is unused [VERIFIED: `internal/relay/protocol.go:17-19` `MsgInput byte = 0x10`, `MsgResize2 byte = 0x11`, `MsgPing byte = 0x12`; the retired range is 0x30-0x37]. Old hosts drop it: both read pumps switch on `MsgInput/MsgResize2/MsgPing` with no default arm [VERIFIED: `internal/relay/server.go:178,192,201` and `internal/webserver/server.go:959,982,1007`], and the comment above each says "no case and no default arm, so old clients' frames are dropped". Read limit is the 32 KiB default on both input readers, so no limit change.
- Host → requester only: extend `MetaPayload` (the "extensible JSON payload", `protocol.go:88`) with `Notice *string` (`"path-unsupported"`, `"path-too-large"`), enqueued to **that subscriber's `Msgs` only** with `queueIfRoom` under `h.mu` (the non-kicking helper in `preview.go`), never `broadcast`/`BroadcastMeta`. Old viewers ignore unknown JSON fields: `handleMeta` destructures only `{ replay, offset }` [VERIFIED: `frontend/src/lib/relayClient.ts:534`].
- Capability announcement: `MetaPayload.PathClick *bool`, set `true` on the `{replay}` meta of `Replay.Frames()` (`hub.go` `Frames()`: `frames = append(frames, MakeMeta(MetaPayload{Replay: &mode}))`) when `!sub.ReadOnly`; `Join` already sees `sub`. A host that predates this phase never sends it and a read-only subscriber never gets it, so the viewer registers no provider in either case: exactly UI-SPEC's "no link provider is registered" rows, with no `/info` or API-version fetch. The peer proxy copies these frames verbatim (`copyWS`: "reads whole messages from src and writes them verbatim to dst … never parses or evaluates frames", `remote_ws_proxy.go:199-214`). Bump `APIVersion` to 4 and add `PathClickAPI = 4` anyway (CONTEXT; `/health` and `/info` report it; `internal/daemon/client_test.go:47` pins `DaemonAPI(h) != 3 || APIVersion != 3 || PreviewAPI != 3` and must change).

**`Hub.PathClick(sub, payload)` order (cheap checks first; every refusal is a counter + a no-reply, never an error frame):**
1. `sub.ReadOnly` → drop (the pumps check it too, like `if !sub.ReadOnly { // MC-03` next to `MsgInput`; test both layers).
2. Payload: 1-1024 bytes, `utf8.Valid`, no control characters (reject NUL/newline/ESC), otherwise drop.
3. `pathClickBusy` (atomic CAS, one resolver per hub) and `!h.previewBusy` — spam from a viewer costs one atomic op; a click during a transfer is silently ignored (UI-SPEC "A preview transfer is already in flight").
4. Run the rest in a goroutine so the read pump (which answers pings: `pongs.Owe()`) never waits on disk.
5. **Printed check** (Pattern 5) → absolute row of the last occurrence, else drop.
6. **Resolve** (Pattern 6) → absolute path, else drop.
7. `previewfile.Open(path)` → `ErrNotExist | ErrNotRegular | read error` = drop; `ErrUnsupported` → unicast `path-unsupported`; `ErrTooLarge` → unicast `path-too-large`; ok → `SendPreview(Preview{Name, MIME, Key, Data, FocusSub: sub})`.
8. Never log the path or text (existing diagnostics log counts only: "input payloads are never logged"). Log outcome codes and sizes.

**Focus (Phase 11 keys it on the last typing viewer):** `SendPreview` sets `Focus: sub == owner` where `owner := h.owner` (`preview.go:202,209`). A click is not typing, and calling `NoteTyping` would *claim the grid* (it sets the owner and the PTY size) — do not. Add `Preview.FocusSub *Subscriber`; when non-nil, `Focus: sub == p.FocusSub`, else today's rule. The viewer side is unchanged: `App.handlePreview` already honours `ev.focus` only when the viewer shows that session's tab or one of its previews (`App.tsx:646-651`). Test: two subscribers, the non-owner clicks, only the clicker's header has `focus:true`; `TestHubSendPreview_NoOwnerNobodyFocuses` still passes untouched.

### Pattern 5: The "printed" check (retained output)

**Storage facts [VERIFIED by reading]:** History = `HistoryMaxLines = 10_000` lines, `HistoryMaxBytes = 4 << 20` (`history.go:10-11`), each line a self-contained rendered ANSI string with a parallel `wrapped []bool` ("A line pushed as wrapped soft-wrapped onto the next one"). The raw ring is `DefaultScrollbackBytes = 256 * 1024` (`scrollback.go:6`); **do not search the raw ring**: it holds OSC/title/escape bytes that are not on screen (a program's `\e]0;/etc/passwd\a` would count as "printed"), it is 16x smaller, and cursor-addressed redraws make its text meaningless. The visible screen is not in History until it scrolls: also search `liveEmu.Render()` rows (split on `\n`, as `screenBodyLocked` does), with `screenRowWrappedLocked` for the screen's wrap joins.

**Algorithm:** under `emuMu` copy (cheap: string headers, ~160 KB for 10,000 lines) `lines[head:]`, `wrapped[head:]`, `pushed`, the screen rows, `IsAltScreen()`, and the cwd log; release `emuMu` (Run holds it while recording and broadcasting; never search under it). Then outside the lock: `ansi.Strip` each row, join soft-wrapped rows into logical lines, iterate **newest to oldest**, and take the first logical line containing the text as a token (below). Row of the match = the absolute row of the logical line's first row. A request for text that is not found (history trimmed, `clear`/`ESC[3J` emptied it, text never printed) is dropped silently.

**Token boundary rule (needed; a bare substring test would let `~/.ssh/id_rsa` pass because `~/.ssh/id_rsa.pub` was printed):** the match must start at the line start or after a character outside the path alphabet, and end at the line end or before one, where "outside" is whitespace, quotes, brackets, `<>|;,=`, and a trailing `:digits(:digits)?` or sentence punctuation. It is a ~15-line function with a table test shared as a fixture with the frontend regex tests (Phase 11 precedent: `frontend/src/lib/__tests__/fixtures/preview-frames.json` is read by both Go and vitest). If the frontend underlines something the host's boundary rule rejects, the click is silently ignored; the shared table keeps them aligned.

**Soft wraps:** the frontend (like xterm's own `WebLinkProvider`) rejoins rows with `isWrapped`; the host rejoins with the wrap flags. The flags come from the hub's hash heuristic (`wraps.go`: "content-identical rows can trade a wrap mark"), so a miss makes a wrapped path unmatchable and the click silent. Mitigation if the VM check shows it: also try joining two adjacent rows when the upper row is full width (`lineFull`). Hard line breaks cannot be rejoined by either side (Pitfall 6).

**After a resize or alt-screen:** rows drained by `resizeLiveEmulator` live in History (it pushes the visible rows before building the empty emulator), so printed text survives a window resize. Alt-screen rows are searched but resolve against the *latest* cwd record (the shell cannot change directory while a TUI runs).

### Pattern 6: Resolver and file open (shared package)

- Absolute (`/…`): as is. `~/…`: `os.UserHomeDir()` (the daemon runs as the user and passes the same `HOME` to the shell). `~user` is not supported. `./…`, `../…`, bare `a/b.png`: `filepath.Join(cwdAtRow, text)`; no record at or before the row means "unknown cwd": drop (the locked "relative ones do nothing"; do **not** seed from the spawn work dir, that would resolve wrongly after the first `cd` in a non-OSC-7 shell). Then `filepath.Clean`; the `:line[:col]` suffix is already stripped by the frontend, and the host also strips a trailing `:digits(:digits)?` before opening so a hand-built frame behaves the same.
- **`..` is not a traversal hole here:** authorisation is "this exact text was printed by the session", not "inside a root". Resolution is deterministic and lexical; symlinks are followed by the OS exactly as for `portty open link.png` (the key is `EvalSymlinks`-based, `cmd_open.go` `previewKey`).
- **Shared package:** move `cmdOpen`'s file section into `internal/previewfile` (leaf: imports neither `relay` nor `daemon`, so `relay` can import it): `Open(path string, maxBytes int64) (File, error)` with `File{Name, MIME, Key string; Data []byte}` and sentinel errors, in the CLI's current order: `os.Stat` (not-exist, directory, not regular, size > cap) → `os.OpenFile(O_RDONLY|syscall.O_NONBLOCK)` → `f.Stat()` regular re-check → `io.ReadAll(io.LimitReader(f, cap+1))` → over-cap-after-read → `previewMIME` → `previewKey`. Also move `hasSVGRoot` and `cleanPreviewName` (currently in `internal/daemon/preview.go:87`). `cmd_open.go` keeps its CLI strings and maps the typed errors to them; `cmd_open_test.go` calls `previewMIME` (`:261`) and `previewKey` (`:359`) directly, so those tests move or call the new package. Behaviour and copy must not change (`TestCmdOpen_*` is the regression).
- **Optimisation worth taking** (a click is user-driven, unlike a typed `portty open`): read the first ~8 KiB first and decide "unsupported" from sniff + extension before reading 20 MB of a `.log`; only `.md/.markdown` needs the whole file (UTF-8 and no NUL over all bytes) and SVG needs the 8 KiB root check. Keep the size-before-type order the CLI has so a 100 MB `.dat` says "too large", not "unsupported".
- **TOCTOU:** nothing to defend beyond what exists. The path text, not an inode, is what was authorised, so a swap between the printed check and the open only changes which file the same printed text names (equivalent to the user re-running the command); a swap to a FIFO/device is caught by the non-blocking open and the descriptor re-check (`cmd_open.go` "Non-blocking, then checked on the descriptor").
- **Source gate:** `TestPreview_SourceGate` (`internal/daemon/preview_test.go:335`) forbids `os`, `io/fs`, `path/filepath` imports in `internal/daemon/preview.go` and `internal/relay/preview.go`. Keep that test exactly as is (those two files stay file-system-free) and **add** a second assertion that the only non-test files importing `os`/`path/filepath` for reads in the click path are `internal/previewfile/*.go` and `internal/relay/pathclick.go` (and that `pathclick.go` calls nothing but `previewfile.Open`). Record the amendment in the security review.

### Pattern 7: Frontend path link provider

**Why `TerminalPanel.web-links.test.tsx:126-133` exists [VERIFIED: file read]:** the test is "Plan B (Wave 0 spike outcome): no OSC 8 secondary registerLinkProvider call", asserting `expect(src).not.toContain('registerLinkProvider')` and `not.toContain('getHyperlinkId')` against the *source text of `TerminalPanel.tsx`*. It guards the Phase 95 decision not to walk OSC 8 hyperlink ids (the public accessor is absent from xterm 6 typings). It is unrelated to path links. **Recommended resolution:** put the provider in `frontend/src/lib/pathLinks.ts` (it calls `term.registerLinkProvider`), keep `registerLinkProvider` out of `TerminalPanel.tsx`, and edit the test deliberately: rename it ("no OSC 8 provider; Phase 12 path links live in lib/pathLinks.ts"), keep `getHyperlinkId` absent, assert `TerminalPanel.tsx` imports `pathLinks`, and assert `registerLinkProvider` appears only in `lib/pathLinks.ts` (a one-file `grep` over `src`). That honours "updated on purpose, naming Phase 12" without weakening the OSC 8 guard.

**xterm 6 provider facts [VERIFIED: `node_modules/@xterm/xterm/src/browser/Linkifier.ts`, `typings/xterm.d.ts`]:**
- `ILinkProvider.provideLinks(bufferLineNumber, callback)`; `ILink { range, text, decorations?, activate(event, text), hover?, leave?, dispose? }` (`xterm.d.ts:1393-1453`). Quote: `decorations` — "this property is tracked and changes made after the link is provided will trigger changes" (`xterm.d.ts:1418-1423`).
- **Live toggle is feasible.** After a link becomes current, xterm replaces `link.decorations` with a tracked object whose `underline`/`pointerCursor` setters re-render while hovered (`Linkifier.ts:272-297`: `linkWithState.link.decorations = {} as ILinkDecorations; Object.defineProperties(…, { pointerCursor: { get…, set: v => { … this._element.classList.toggle('xterm-cursor-pointer', v) } }, underline: { get…, set: v => { … this._fireUnderlineEvent(linkWithState.link, v) } } })`). **Gotcha:** that replacement happens *after* `_linkHover` has called `link.hover(...)` (`:270` then `:273`), so inside `hover()` the object is still your own plain one. Do not cache the original `decorations` object; always write through `link.decorations.underline = …` on later events.
- **Recommended implementation (small):** the provider returns links with `decorations: { underline: modDown, pointerCursor: modDown }`, where `modDown` comes from one panel-level tracker (`keydown`/`keyup` on `window`, `mousemove` events' `metaKey`/`ctrlKey` via `isModifierPressed(event, mode)`, and **reset on `blur`/visibility change**, since a Cmd keyup is lost on Cmd-Tab). On a modifier change, set `link.decorations.underline = pointerCursor = modDown` on the currently hovered link (track it from `hover`/`leave`). `activate` re-checks `isModifierPressed(event, modifier)` (default `'platform'`, `modifier = (cfg?.modifier || 'platform')`, exactly as the URL handler, `TerminalPanel.tsx:848`), so even a wrong underline can never fire a click without the modifier. **Fallback (UI-SPEC-approved):** if the VM check shows `keyup` is not delivered reliably in WKWebView, drop the tracker and keep underline-on-plain-hover with a modifier-gated click, which is today's URL behaviour.
- **Precedence over URLs:** providers are queried in registration order and a lower-priority link intersecting a higher one on the same row is removed (`Linkifier.ts` `_removeIntersectingLinks`; `_checkLinkProviderResult`: "If all providers with higher priority came back undefined…"). `WebLinksAddon` is hot-swapped (disposed and recreated when the toggle flips), so its index is not stable; do **not** rely on order. The path provider must itself skip any range that overlaps a URL match (run the same `urlRegex` the addon is given at `TerminalPanel.tsx` over the row text and exclude overlaps).
- **Lifecycle:** register in the existing `if (pluginConfig?.webLinks) { … } else { … }` arm of the hot-swap effect (`TerminalPanel.tsx:825`), keep an `IDisposable` ref, dispose in the else-branch and in the unmount cleanup (mirror `webLinksAddonRef`). Register once per terminal and make `provideLinks` return `undefined` unless a `pathClickEnabledRef` is true (set from the host's `pathClick` bit, cleared on socket close / reconnect), so connect, reconnect and read-only never churn the provider.
- **Text and ranges:** build per row a char list and a cell-index map from `IBufferLine.getCell(x)` (skip zero-width spacer cells; empty cell → space) so wide characters keep correct ranges, and rejoin `isWrapped` rows up to a 2048-character window, as `WebLinkProvider._getWindowedLineStrings` does (the addon's `LinkComputer` is not exported, so a ~60-line local version is justified: three uses is not yet an abstraction, but it is also not a dependency we can reuse).
- **Mouse-reporting TUIs:** the `Linkifier` listens to `mousemove/mousedown/mouseup` on the xterm element itself (`Linkifier.ts:55-57`), independent of mouse reporting, so hover and activate still run when the app has captured the mouse, but the same click is also forwarded to the app (as it already is for URL links). UI-SPEC makes this a VM backstop: verify Cmd-click on a path in the TUI the user runs; if the app swallows it, report, don't change the modifier.
- **Detection regex (planner's discretion; contract from UI-SPEC):** token alphabet `[\p{L}\p{N}_.~@%+=#\-]` with `/`; anchored forms `/…`, `~/…`, `./…`, `../…`, or a bare form with ≥1 `/` whose last segment has an extension (`docs/diagram.png`); left boundary: not preceded by an alphabet char or `/`; optional `:\d+(:\d+)?` included in the underlined range and stripped from the request; trailing `. , ; : ) ] } ' "` trimmed; skip URL overlaps. Put the pure function in `pathLinks.ts` and test it with a table (incl. wrapped, wide chars, URLs, `a/b.png:12:3`, `(docs/x.png).`, `--out=/tmp/a.png`).

### Pattern 8: Notice and send (frontend)

`RelayClient`: add `MSG_PATH_CLICK = 0x13` and `sendPathClick(text)` that, like `sendInput`, is **dropped unless the state is `open`** (never queued: a click replayed after a reconnect would surprise), plus `handleMeta` branches for `pathClick === true` (`onCapabilities`) and `notice` (`onNotice`), with capability cleared in `ws.onclose` next to `onPreviewReset`. `TerminalPanel` shows the notice through the existing slot: two new `SessionEndedBanner` kinds `path-unsupported` and `path-too-large` with the fixed UI-SPEC copy, a 5000 ms auto-dismiss timer that restarts on a replacement, and the "another banner already owns the slot → drop" rule (the existing render order at `TerminalPanel.tsx:1217-1233` already makes the reconnecting banner and the ended banner win if the path notice is rendered last; add a test). The wire value is mapped to a **fixed** kind by exact string comparison and is never rendered (the `SessionEndedBanner` header's rule: "the wire reason is still never rendered").

### Anti-Patterns to Avoid
- **Searching the raw ring for the printed check:** wrong data (escape bytes, titles), 256 KiB, and unusable after wrap.
- **Parsing OSC 7 by scanning raw PTY reads:** a read can split the sequence; the emulator handler cannot.
- **Calling `NoteTyping` to focus the clicker:** it claims the grid.
- **`broadcast`/`BroadcastMeta` for the notice:** every viewer would see "Can't preview this file type".
- **Doing file I/O on the WebSocket read goroutine:** a slow disk stops pong handling and the viewer is dropped as dead.
- **`autoload add-zsh-hook` / writing OSC 7 to stdout in the shim:** `fpath` may be altered by the user's `.zshenv`; `$(…)` would capture the escape bytes.
- **Setting `ZDOTDIR` before the shim is written and verified.**
- **Trusting the host's wrap flags as if they were exact** (see Pitfall 7).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| zsh startup interposition | A four-file `.zshenv/.zprofile/.zshrc/.zlogin` shim re-implementing sourcing order | Ghostty's single `.zshenv` + restore | zsh itself does the ordering once ZDOTDIR is restored (evidence table) |
| `file://` decoding | A hand-written percent-decoder | `net/url.Parse` then `u.Path` | Verified: decodes UTF-8, rejects a stray `%`; add only NUL/control/absolute checks |
| ANSI stripping of History lines | A regex stripper | `ansi.Strip` (x/ansi v0.11.7) | Handles CSI/OSC incl. OSC 8 with either terminator |
| Preview delivery | A second sender | `Hub.SendPreview` (paced, non-kicking) + `FocusSub` | Backpressure, replaying-viewer skip and one-in-flight already solved |
| File checks | A second implementation in the daemon | Move `cmdOpen`'s checks into `internal/previewfile` | CONTEXT: "code is moved, not reinvented" |
| Link rendering/underline | Custom DOM overlay | xterm `registerLinkProvider` + `link.decorations` | Positions, wrapping, cursor, repaint are xterm's |
| Notice UI | A toast component | `SessionEndedBanner` kinds | UI-SPEC |
| Per-line cwd tags | A parallel array filled at `Push` | Absolute-row cwd log | Tags are assigned too late (Pattern 2) |

**Key insight:** every hard part (byte-ordered OSC parsing, retained rendered history with wrap flags, paced non-kicking delivery, verbatim peer proxy, banner slot) already exists and was built for exactly this shape of problem. The risk in this phase is not missing machinery; it is (a) the shell-startup shim, which can silently disable a user's rc files, and (b) the authorisation contract, which is why the printed check needs token boundaries and the click needs both read-only gates.

## Runtime State Inventory

Not a rename/refactor phase (omitted). One migration-like item: `cmd_open.go` file checks move into `internal/previewfile` (Chesterton's Fence: `cmd_open_test.go` calls `previewMIME`/`previewKey` directly and must keep passing).

## Common Pitfalls

### Pitfall 1: ZDOTDIR left on the shim dir
**What goes wrong:** zsh skips `.zprofile/.zshrc/.zlogin`; the user's PATH, prompt and aliases vanish on an unattended Mac.
**Why:** no readable `.zshenv` in `ZDOTDIR`, or the shim file removed between write and spawn (verified: case 7).
**How to avoid:** write temp + `Rename`, `Stat`, then set env; on any failure set nothing. Test with the shim dir deliberately broken: the spawned shell must still reach the user's `.zshrc` (it will, because no ZDOTDIR was set).
**Warning signs:** `ZDOTDIR` equal to the shim path inside a Portty shell (`echo $ZDOTDIR` must be empty or the user's value).

### Pitfall 2: OSC 7 captured by command substitution or lost to a redirect
**What goes wrong:** `x=$(cd d && pwd)` includes escape bytes in `x`, or `cd d >/dev/null` swallows the report.
**How to avoid:** write to `/dev/tty` with `2>/dev/null` before the redirect (order matters), guard `ZSH_SUBSHELL`. Both verified.

### Pitfall 3: User rc resets the hook
**What goes wrong:** `chpwd_functions=()` in `.zshrc` (rare, real) silences OSC 7.
**How to avoid:** the `precmd` re-assert (verified). If a rc file also clears `precmd_functions`, tracking degrades to "no cwd", i.e. relative paths do nothing, which is the locked fallback.

### Pitfall 4: Relative path resolved against the wrong directory
**What goes wrong:** using the *current* cwd instead of the cwd at print time, or a per-line tag assigned at drain time.
**How to avoid:** absolute-row log (Pattern 2); test `cd a; ls` (prints `x.png`), `cd ../b`, click the old `a/x.png`-style output: it must resolve to `a`. And the same-read case `cd docs && ls docs/*.png` in one burst.

### Pitfall 5: Nested shells and ssh
**What goes wrong:** a nested `zsh` (verified: no hooks) or a bash started inside the session does not emit OSC 7; the daemon's last cwd goes stale and a relative click resolves against the outer shell's directory (the file must still exist, so the failure is usually a silent no-op, but a same-named file could open).
**Why accepted:** locked decision ("tracked only when the user's own setup emits OSC 7"). Document it in the security review and TESTING.md; `ssh host` emitting OSC 7 with a different hostname is ignored by the host check, so the outer cwd stays (same staleness).
**Mitigation available later (not now):** none needed for correctness of the security contract: authorisation is the printed text, and the file must exist and be a previewable type.

### Pitfall 6: Hard-wrapped paths
**What goes wrong:** TUIs that wrap long text with real newlines (`[ASSUMED]` for Claude Code's renderer) split a long path across two rows with a hard break; neither xterm nor the host can rejoin them. Each fragment is underlined as its own link and clicks silently fail.
**How to avoid:** none; list as a known limitation and check what the user's TUI does in the VM.

### Pitfall 7: Wrap flags are heuristic
**What goes wrong:** the host fails to join a soft-wrapped path (`wraps.go` shortcut), so a path the viewer rejoins is "not printed" and the click is silent.
**How to avoid:** the full-row fallback in Pattern 5, and a VM check with a path longer than the window.

### Pitfall 8: Searching under `emuMu`
**What goes wrong:** `Run` holds `emuMu` while recording and broadcasting; a 10,000-line `ansi.Strip` scan under that lock stalls output for every viewer.
**How to avoid:** copy under the lock, search outside (Pattern 5).

### Pitfall 9: Read-only is enforced only in one place
**What goes wrong:** a pump forgets `if !sub.ReadOnly`, or the hub trusts the pump. Today every minted share is `Perms: "read,write"` ("A share always grants full control (SHAR-02)", `internal/daemon/share.go:323-326`), so a read-only viewer exists only for legacy/hand-signed capabilities and in tests; the gate would never fire in normal use and could rot unnoticed.
**How to avoid:** check in both pumps **and** at the top of `Hub.PathClick`; one test per layer using a `Subscriber{ReadOnly: true}`.

### Pitfall 10: Existing MockTerminal tests break
**What goes wrong:** six test files define `class MockTerminal` without `registerLinkProvider` [VERIFIED: `grep -rln "class MockTerminal"` returns 6]; any that mount `TerminalPanel` with `webLinks: true` will throw `registerLinkProvider is not a function`.
**How to avoid:** add `registerLinkProvider = vi.fn(() => ({ dispose: () => {} }))` to those mocks (hot-swap, web-links, `TerminalPanel.test.tsx` first), and test the provider lifecycle there.

### Pitfall 11: Frame spam / DoS from a write-capable viewer
**What goes wrong:** repeated clicks re-run the History search and file reads.
**How to avoid:** the busy CAS plus `previewBusy` drop; payload cap; goroutine off the read pump. A write-capable viewer already has the shell, so this is robustness, not privilege.

### Pitfall 12: Test binaries and the firewall alert
Keep new Go tests in existing packages (`relay`, `daemon`, `webserver`, root); one new tiny package (`internal/previewfile`) adds one test binary: accept it, run each full suite at most once per plan, and prefer in-memory hubs over new `httptest` listeners (inherited from Phase 11).

## Code Examples

### Env wiring (internal/daemon, new `zshshim.go`; shape, not final code)
```go
// Source: modeled on internal/daemon/clilink.go cliPathEnv/refreshCLILink (read this session)
func (e *SessionEngine) zshEnv() []string {
	dir, err := e.refreshZshShim() // writes <configDir>/zsh/.zshenv atomically, then Stat()s it
	if err != nil {
		log.Printf("zsh shim: %v", err) // the session still starts, without cwd tracking
		return nil
	}
	return []string{"ZDOTDIR=" + dir, "PORTTY_ZSH_ZDOTDIR=" + os.Getenv("ZDOTDIR")} // "" = unset in the shim
}
// CreateSession: only when cliPath == sessionShell; concatenate with e.cliPathEnv().
```
`PORTTY_ZSH_ZDOTDIR` must reach `mergeEnv` as an override that wins over an inherited value (`internal/pty/native.go:69` passes `req.Env` as `extra`, and `required` for the fixed ones): the empty value is what makes an inherited stale one harmless. Do not put `PORTTY_ZSH_ZDOTDIR` in `required` unless `pty` is taught about it; `extra` already overrides `base` (`os.Environ()`).

### OSC 7 parse (internal/relay/cwd.go; unit-test table)
```go
// Source: behaviour verified by running net/url on these inputs this session.
func parseOSC7(data []byte, self string) (string, bool) {
	rest, ok := strings.CutPrefix(string(data), "7;")
	if !ok || len(rest) > 2048 { return "", false }
	u, err := url.Parse(rest)
	if err != nil || u.Scheme != "file" { return "", false } // a raw '%' lands here
	if h := strings.ToLower(strings.TrimSuffix(u.Hostname(), ".")); h != "" && h != "localhost" && !sameHost(h, self) {
		return "", false // ssh: another machine's directory
	}
	p := u.Path
	if !filepath.IsAbs(p) || strings.IndexFunc(p, unicode.IsControl) >= 0 { return "", false } // %00, %0A
	return filepath.Clean(p), true
}
```

### Provider skeleton (frontend/src/lib/pathLinks.ts)
```ts
// Source: @xterm/xterm 6.0.0 typings (ILinkProvider/ILink/ILinkDecorations) and Linkifier.ts:272-297
export function registerPathLinks(term: Terminal, deps: {
  enabled: () => boolean            // host said pathClick:true on this socket
  modifier: () => ModifierMode      // webLinksConfigRef.current?.modifier || 'platform'
  isModDown: () => boolean          // panel-level tracker, reset on blur
  onClick: (text: string) => void   // relayClient.sendPathClick
}): IDisposable {
  return term.registerLinkProvider({
    provideLinks(y, cb) {
      if (!deps.enabled()) return cb(undefined)
      cb(findPathLinks(term, y).map((m) => ({
        range: m.range, text: m.text,
        decorations: { underline: deps.isModDown(), pointerCursor: deps.isModDown() },
        activate: (ev, t) => { if (isModifierPressed(ev, deps.modifier())) deps.onClick(stripLineCol(t)) },
        hover: (_ev, _t) => { /* track `m` as the hovered link so a modifier change can set m.decorations.* */ },
        leave: () => { /* clear it */ },
      })))
    },
  })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Terminal.app: report cwd only at `precmd` | Ghostty/kitty: `chpwd` + `precmd` + initial report, written to a dedicated TTY fd | current Ghostty main | `cd x && cmd` ordering plus self-healing; our shim does the same with `/dev/tty` |
| VS Code: own `OSC 633;P;Cwd=` | Standard OSC 7 | n/a | We want OSC 7 so user setups for bash/fish "just work" (CONTEXT) |
| Provider links always underline on hover | `ILink.decorations` tracked live | xterm 5/6 | Modifier-gated underline is possible |

**Deprecated/outdated:** nothing in this repo's inherited stack is deprecated for this use. `Callbacks.WorkingDirectory` in vt exists but drops payloads with a second `;`; do not use it.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | fish (recent versions) emits OSC 7 natively; bash on macOS only via Apple's `/etc/bashrc_Apple_Terminal` (`TERM_PROGRAM=Apple_Terminal`) or user setup | Shell integration | Only affects what "other shells that emit it" means in docs; no code depends on it |
| A2 | macOS TCC attributes a file read by the daemon the same as one read by the CLI inside the shell (same responsible process), so reading `~/Documents` etc. behaves like `portty open` today | Pattern 6 | A TCC prompt could appear on the host Mac for a click; same exposure as `ls ~/Documents` already has |
| A3 | Claude Code's Ink renderer hard-wraps long lines with real newlines, so long paths in its output cannot be rejoined | Pitfall 6 | If it soft-wraps, long paths work (better); if not, document the limit |
| A4 | Wails' WKWebView delivers `keydown`/`keyup` for a bare Cmd press and a `blur` on Cmd-Tab (needed for the live underline) | Pattern 7 | If not, use the UI-SPEC fallback (underline on plain hover, modifier-gated click) |
| A5 | Adding `precmd` emission to the shim is acceptable although CONTEXT says "chpwd hook (plus one initial emit)" | Pattern 1 | Needs the user's/planner's nod (Open Question 1); dropping it costs handoff recovery and rc-reset recovery |
| A6 | A host-authoritative capability bit in `MsgMeta` is acceptable instead of a frontend API-version check | Pattern 4 | UI-SPEC's observable behaviour is identical; if the user wants the API check literally, add `api` plumbing for peer tabs |

## Open Questions (RESOLVED)

1. **Add `precmd` emission to the shim?** **RESOLVED:** yes, chpwd + precmd + initial emit (12-CONTEXT "Decided at planning" [Shell], first item; built in 12-02 Task 1, rc-reset case in 12-02 Task 2).
   - What we know: CONTEXT locks "chpwd hook (plus one initial emit)". Ghostty uses chpwd + precmd + initial; Terminal.app uses precmd. I verified that without precmd a user `.zshrc` doing `chpwd_functions=()` silences tracking, and that precmd also re-publishes state after a handoff.
   - What's unclear: whether the user counts this as within the locked decision.
   - Recommendation: include it (about 6 lines, verified); flag it in the plan as the one deliberate addition. The shim is daemon-written and refreshed per session, so it can be edited later for new sessions.
2. **Gate the viewer on a stream capability bit instead of the API version?** **RESOLVED:** both: the `pathClick` bit on the replay MsgMeta is the viewer-side switch, and APIVersion 4 with `PathClickAPI` is reported (12-CONTEXT "Decided at planning" [Gating]; bit in 12-04 Task 3, API in 12-05 Task 2, viewer gate in 12-06 Task 1).
   - What we know: peer tabs get the host's API only via a fetch the proxy path does not make; local tabs have no API binding; the stream already carries per-connection `MsgMeta`.
   - What's unclear: CONTEXT says "gated by an API version bump like PreviewAPI".
   - Recommendation: do both: bump `APIVersion` to 4 with `PathClickAPI` (reported on `/health` and `/info`), and use the `pathClick` meta bit as the viewer-side switch (it also encodes read-only).
3. **Export the cwd log in `HubState`?** **RESOLVED:** yes, additive gob fields `Cwds` and `HistoryPushed` with a round-trip test (12-CONTEXT "Decided at planning" [Handoff]; 12-03 Task 3).
   - Recommendation: yes (additive gob fields, one round-trip test); otherwise pre-upgrade relative paths are silent until the next prompt.
4. **Hostname equality details** (`os.Hostname()` vs `.local`, case): the shim emits an empty host, so only third-party emitters hit this. Keep to case-insensitive equality plus `.local` tolerance; planner's call. **RESOLVED:** case-insensitive equality with `os.Hostname()` (looked up once), trailing dot trimmed, `.local` suffix difference tolerated, plus empty and `localhost` (12-03 Flagged decisions "parseOSC7"; TestParseOSC7 in 12-03 Task 2).
5. **Fixture sharing:** share one JSON table between Go and vitest for the path-token boundary rule and the `MsgPathClick`/meta frame bytes (Phase 11 precedent). **RESOLVED:** one hand-written fixture, `frontend/src/lib/__tests__/fixtures/path-click.json`, with `tokens`, `detect` and `frames`, written by 12-04 (Flagged decisions "Shared fixture"; Tasks 2 and 3) and read unchanged by 12-06.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `/bin/zsh` | Shim integration tests, spawn | ✓ | zsh 5.9 (arm64-apple-darwin26.0) | — |
| Go toolchain | All Go work | ✓ (pinned) | `toolchain go1.26.11` | — |
| node / pnpm | vitest, tsc | ✓ | node v24.22.1, pnpm 12.6.0 | — |
| Tart + two VMs | VM checks (peer over tailnet, browser, dark mode, TUI) | ✓ | tart 2.41.1; `portty-test` and `portty-test-2` both `running` | — |
| Python 3 (`-I`) | Scratch PTY harness used for this research only | ✓ | 3.15.2 | not needed by the executor; Go tests spawn zsh through the existing PTY backend |

**Missing dependencies with no fallback:** none.

## Test and VM Check Plan

(`workflow.nyquist_validation` is `false` in `.planning/config.json`, so the formal Validation Architecture section is omitted; this section answers the orchestrator's question 8.)

**Frameworks:** Go `testing` (`GOTOOLCHAIN=go1.26.11 go test -race -short ./...`), vitest 4 (`cd frontend && pnpm test`), Playwright unchanged. No new framework.

| Req | Behaviour | Test | File (new unless noted) |
|-----|-----------|------|-------------------------|
| PATH-01 | OSC 7 parser: `%XX`/UTF-8, `;`, spaces, empty/`localhost`/own host, foreign host ignored, raw `%`, `%00`/`%0A`, non-absolute, scheme, length cap | unit table | `internal/relay/cwd_test.go` |
| PATH-01 | cwd log through the real hub: same-read `cd d && ls`, scroll past the screen, resize, `ESC[3J`, history trim keeps the floor record, alt-screen ignored, RIS clears, dedupe | hub test with a fake reader | `internal/relay/cwd_hub_test.go` |
| PATH-01 | handoff export/import keeps the log | extend existing | `internal/relay/handoff_state_test.go` |
| PATH-01 | **zsh shim matrix under real `/bin/zsh`** (no ZDOTDIR, custom ZDOTDIR, ZDOTDIR set in `~/.zshenv`, broken/`return` user `.zshenv`, space in dir, `$(cd)` capture, redirect, rc resets `chpwd_functions`, nested zsh, shim dir missing `.zshenv` is never selected, stale `PORTTY_ZSH_ZDOTDIR` overwritten, empty stderr on clean HOME) | integration (PTY via `NativePTYBackend`, `t.Setenv("HOME", tmp)`) | `internal/daemon/zshshim_test.go` |
| PATH-01 | shim + `path_helper`: `command -v portty` still resolves | sibling of `TestCLILink_SurvivesZshLoginPathHelper` | `internal/daemon/clilink_test.go` |
| PATH-01 | shim writer: atomic, 0700/0600, idempotent under concurrency, failure still spawns | unit | `internal/daemon/zshshim_test.go` |
| PATH-02 | printed search: boundary table (`id_rsa` vs `id_rsa.pub`), wrap joins, ANSI/OSC 8 stripped, newest wins, history/screen split, trimmed history = no match | unit + hub | `internal/relay/pathclick_test.go` |
| PATH-02 | `Hub.PathClick`: read-only refused (hub and both pumps), busy dropped, missing/dir silent, unsupported/too-large unicast to one subscriber only, ok → `SendPreview` with `FocusSub` (only the clicker's header `focus:true`), `~` and `..` resolution, no-cwd relative silent | hub tests with real temp files | `internal/relay/pathclick_test.go` |
| PATH-02 | frames: `MsgPathClick` ignored by an old-host-shaped pump; `MetaPayload` `pathClick`/`notice` wire bytes shared with vitest | fixture test | `internal/relay/protocol_test.go`, `frontend/src/lib/__tests__/fixtures/` |
| PATH-02 | both pumps route `MsgPathClick`; read-only share cap never reaches `PathClick` | server tests | `internal/relay/server_test.go`, `internal/webserver/*_test.go` |
| PATH-02 | source gate extension and `previewfile` | unit | `internal/previewfile/previewfile_test.go`, `internal/daemon/preview_test.go` (`TestPreview_SourceGate` kept as is) |
| PATH-02 | `cmd_open.go` behaviour unchanged after the move | existing `TestCmdOpen_*` | `cmd_open_test.go` (edited call sites only) |
| PATH-02 | path detection table; provider returns no links when disabled; decorations follow the modifier; click gated by `isModifierPressed`; URL overlap excluded; plain click selects | vitest | `frontend/src/lib/__tests__/pathLinks.test.ts` |
| PATH-02 | `sendPathClick` dropped unless open; meta `pathClick`/`notice` handled; capability cleared on close | vitest | `frontend/src/lib/relayClient.test.ts` |
| PATH-02 | notice banner kinds, 5 s auto-dismiss, replacement restarts timer, other banner wins, fixed copy only | vitest | `frontend/src/components/__tests__/SessionEndedBanner.test.tsx`, `TerminalPanel.pathClick.test.tsx` |
| PATH-02 | Plan B test edited: no OSC 8 provider, `registerLinkProvider` only in `lib/pathLinks.ts` | edit in place | `frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx` |

**Sampling:** per task commit, the touched package's tests; per wave, `go build ./... && go vet ./...` plus the affected vitest files; phase gate, full Go suite once, `pnpm vitest run` + `tsc`, `check-brand`, `bash tests/check-traceability-paths.sh`. TESTING.md: new Category BB; Section 2 counts; Section 4 rows (PATH-01/02, file paths only); Section 5 from M-128 for the items below.

**Wave 0 gaps:** `registerLinkProvider` on the six `MockTerminal` classes; a shared fixture file for the token table and frame bytes; `internal/previewfile` package skeleton.

**VM checks (manual, Tart; never the host; never stop the VMs):**
1. Local tab: `cd ~/proj; ls docs/*.png` then Cmd-click: preview tab opens and the clicker switches; plain click selects; URL unchanged.
2. Peer tab: home VM peer tab on an office-VM session over the real tailnet: Cmd-click in the **home** tab, home switches, the office window gets a background tab (clicker is not the last typist: type in the office window first to prove `FocusSub`).
3. Browser share link at `/app/`: same.
4. Missing path silent (no tab/notice/typed text); a `.txt` shows "Can't preview this file type…"; a >20 MB file shows "Too large to preview…", **only in the clicking window**.
5. Read-only: hand-sign a `read` cap (test key) and open it: no underline, and a hand-crafted frame (e.g. a scripted WebSocket client) is refused.
6. Light/dark live repaint of the notice; Cmd-hover underline appears and disappears with the key (or the documented fallback).
7. TUI with mouse reporting (the user's Claude Code, vim `mouse=a`): Cmd-click on a printed path.
8. Shell integrity on a **clean VM user and a user with custom `ZDOTDIR` and an oh-my-zsh `.zshrc`**: prompt, PATH, aliases, history file location, `portty open`, nested `zsh`, `ssh localhost`, `$(cd …)` capture.
9. Upgrade handoff (A to B build) with a shim-enabled session: next prompt restores tracking; pre-upgrade relative clicks follow the chosen Open Question 3 outcome.
10. Window resize after `ls`: the printed path is still clickable (History keeps the rows).
11. Wrapped path longer than the window; hard-wrapped TUI output (documents Pitfalls 6 and 7).

## Security Domain

`security_enforcement` is enabled (absent = enabled; `security_asvs_level: 1`, `security_block_on: high`). This phase **amends PREV-03**; the review must be written (`12-SECURITY-REVIEW.md`, model on `11-SECURITY-REVIEW.md` "PREV-03: no file-by-path route").

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | share/peer WhoIs + capability gate already in front of every WebSocket |
| V3 Session Management | no | — |
| V4 Access Control | **yes** | `ReadOnly` refusal in both pumps and in `Hub.PathClick`; host-authoritative capability bit; no route is added |
| V5 Input Validation | **yes** | click payload: length cap, UTF-8, no control chars; OSC 7: `net/url`, scheme/host/absolute/control checks, length cap; token-boundary check; MIME/name validation reused (`Preview.validate`) |
| V6 Cryptography | no | `previewKey` is an identifier hash, not a security control |
| V8 Data Protection | **yes** | never log paths or click text; History stays memory-only (existing); preview bytes only to current subscribers |
| V12 Files and Resources | **yes** | regular file only, non-blocking open + descriptor re-check, 20,000,000-byte cap, closed MIME allowlist, SVG root/DOCTYPE rules (reused) |
| V13 API / Web Service | **yes** | new client frame is dropped by old hosts; no HTTP surface added; unix-socket-only preview route unchanged |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Read-only viewer harvests files via path click (`ls ~/.ssh`) | Information disclosure / EoP | Refused host-side at pump and hub (locked decision (b)); never registers a provider (capability bit) |
| Viewer asks for a path the session never printed | Information disclosure | Printed check against rendered History + screen with token boundaries; prefix/suffix tricks (`id_rsa` from `id_rsa.pub`) rejected |
| Program spoofs "printed" text or cwd (hidden/concealed text, forged OSC 7) | Spoofing | **Accepted, not mitigated:** any program in the session already runs as the user and can run `portty open <anything>`; the check bounds only *viewer-originated* requests. Matching rendered text, not the raw ring, removes the title/OSC-string vector. ssh peers' OSC 7 ignored by host check (not a security boundary) |
| A write-capable viewer reads any file the user could | Information disclosure | **Accepted and stated:** write capability is full shell control (SHAR-02: "A share always grants full control"); the phase adds no capability beyond what typing `cat` gives. The printed check and file-type allowlist are defence in depth, not the boundary |
| Path traversal via `..` or `~` | Tampering | Not a boundary here (authorisation is the printed text); lexical `Clean`, symlinks followed like `portty open` |
| TOCTOU between check and open | Tampering | Authorisation is by text; FIFO/device swaps caught by `O_NONBLOCK` + `fstat` regular check |
| Very long line / huge text | DoS | 1024-byte payload cap, 32 KiB frame limit, one in-flight resolver, search outside `emuMu`, 10,000-line bound |
| Escape-sequence injection into the shim | Elevation (shell startup) | The shim is daemon-written, 0700 dir / 0600 file, atomic; no viewer input reaches it |
| Notice text injection | Tampering | Wire value mapped to a fixed kind by exact compare, never rendered |
| Frame spam / slow disk stalls the pump | DoS | Resolver off the read goroutine; atomic busy guard |

**Record in the security review (amendment text):** "Phase 11: the daemon never opens a path given to it. Phase 12: the daemon opens a path only when (a) the text is a token in that session's own rendered output, (b) the requesting subscriber can write, (c) it passes the `portty open` checks; there is still no HTTP/route that takes a path; the click rides the session's authenticated WebSocket." Include: the honest "write capability already equals shell access" paragraph, the program-spoofing acceptance, the nested-shell cwd staleness, and the tests that pin each.

## Sources

### Primary (HIGH confidence): code read or run this session
- `internal/relay/hub.go` (Run, recordFrameLocked, newLiveEmulator OSC handlers, drainScrollbackLocked, resizeLiveEmulator, Join/Frames), `history.go`, `scrollback.go`, `preview.go`, `protocol.go`, `server.go`, `oscabsorb.go`, `wraps.go`, `charstate.go`, `handoff_state.go`
- `internal/webserver/server.go` (`handleWSSRelay`, `handleSessionInfo`, `handleTerminalPage`), `internal/daemon/engine.go`, `clilink.go`, `clilink_test.go`, `preview.go`, `preview_test.go`, `types.go`, `peer_connect.go`, `remote_ws_proxy.go`, `share.go`, `internal/pty/native.go`, `internal/capability/capability.go`
- `cmd_open.go`, `cmd_open_test.go`; `frontend/src/components/TerminalPanel.tsx`, `SessionEndedBanner.tsx`, `lib/relayClient.ts`, `lib/openLink.ts`, `App.tsx` (`handlePreview`), `__tests__/TerminalPanel.web-links.test.tsx`
- `~/go/pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615092313-b57e5e6d29bb/{handlers.go,osc.go,callbacks.go,emulator.go}`; `…/x/ansi@v0.11.7/width.go` (`Strip`)
- `frontend/node_modules/@xterm/xterm/{typings/xterm.d.ts,src/browser/Linkifier.ts}`, `@xterm/addon-web-links/src/WebLinkProvider.ts`
- `/etc/zshrc`, `/etc/zprofile`, `/etc/zshrc_Apple_Terminal` (Terminal.app reports cwd with `add-zsh-hook precmd update_terminal_cwd`)
- Experiments in the scratchpad: the shim run under `/bin/zsh -l -i` on a real PTY (table above); `net/url.Parse` on OSC 7 shapes under `go1.26.11`
- Planning docs: `12-CONTEXT.md`, `12-UI-SPEC.md`, `11-CONTEXT.md`, `11-RESEARCH.md`, `11-SECURITY-REVIEW.md`, `REQUIREMENTS.md`, `ROADMAP.md`, `TESTING.md`

### Secondary (MEDIUM confidence, official source code fetched)
- Ghostty `src/shell-integration/zsh/.zshenv` and `ghostty-integration` (raw.githubusercontent.com/ghostty-org/ghostty/main/…): ZDOTDIR restore, `always` block, `chpwd` + `precmd` + initial report, dedicated TTY fd
- VS Code `shellIntegration-rc.zsh` (raw.githubusercontent.com/microsoft/vscode/main/…): `USER_ZDOTDIR`, manual `.zshrc` source, `OSC 633;P;Cwd=` (no OSC 7)

### Tertiary (LOW confidence)
- None used for decisions; A1 and A3 are `[ASSUMED]` and flagged.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new packages; every API used was read in source.
- Architecture: HIGH for hub/cwd/printed/transport (read end to end); MEDIUM for the live-underline toggle in the real WKWebView (source read, not run).
- Shim mechanics: HIGH, run on the target zsh with the failure matrix; remaining risk is other zsh versions (the Mac ships 5.9; VMs run the same OS image).
- Pitfalls: HIGH where verified by experiment (1, 2, 3, 5); MEDIUM for 6 and 7 (VM checks will settle them).

**Research date:** 2026-11-21
**Valid until:** 2026-12-21 (stable stack; re-verify if xterm, vt or the macOS zsh version changes)
