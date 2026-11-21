# Phase 12: File Path Click - Context

**Gathered:** 2026-11-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Portty tracks each shell's working directory through OSC 7 (zsh emits it with no user setup; bash/fish when the user's own setup emits it). Cmd-clicking a file path printed in the terminal (absolute, or relative to the working directory) previews it through the Phase 11 preview path, in local tabs, peer tabs over the tailnet, and the browser share view. The host reads only a path that the session itself printed, only for a viewer that can write to the session, so there is still no general file-read API.

</domain>

<decisions>
## Implementation Decisions

### Shell integration
- zsh emits OSC 7 through a ZDOTDIR shim (the Ghostty / VS Code method): the daemon points ZDOTDIR at a Portty-owned directory whose zsh startup files restore the user's real ZDOTDIR (or unset it), source the user's own files in the normal order, then add a chpwd hook (plus one initial emit) that prints OSC 7 — **Reversibility:** costly — it runs inside every user's shell startup on every session; a mistake breaks shells on an unattended remote Mac, and changing the mechanism later means re-testing every startup-file combination. Must not change behaviour for users with no ZDOTDIR, a custom ZDOTDIR, or a login shell that isn't zsh.

### Security model for path clicks
- The daemon reads a clicked file only when all hold: (a) the clicked text appears in that session's own retained output; (b) the requesting viewer can write to the session (read-only subscribers are refused, since `ls ~/.ssh` would otherwise hand them files they could not otherwise read); (c) the file passes the same checks as `portty open` (regular file, non-blocking open re-checked on the descriptor, 20,000,000-byte cap, image/PDF/Markdown sniffing). This amends Phase 11's "the daemon never opens a path given to it" rule; the security review must record the amendment and the PREV-03 argument — **Reversibility:** one-way — it is the security contract for host file reads; loosening it later is easy but tightening it after release breaks behaviour users rely on, and the review must sign off on it.

### Click UX
- Every viewer gets the preview tab, as with `portty open`: the clicking viewer switches to it, other viewers get it as a background tab.
- Cmd-hover underlines any path-like text (no host round trip). Clicking a path that doesn't exist does nothing: no tab, no dialog, nothing typed into the shell (SC3).
- Cmd-clicking a file that exists but can't be previewed (wrong type, over the cap) shows a brief transient notice in the clicking window only (e.g. "Can't preview this file type" / "Too large to preview"). Missing files stay silent.
- A plain click still selects text; URLs keep their existing click behaviour (confirm popover, then browser).

### Scope
- Path clicks also work in the browser share view, not only app and peer tabs — parity with Phase 11 previews. (Corrected 2026-11-21 after UI research: share links redirect to `/app/?session=…&cap=…` (`internal/webserver/server.go` handleTerminalPage), which mounts the same React `TerminalPanel`, so one implementation covers local, peer and browser. `web/assets/terminal.js` has no preview support and is out of scope; the original pointer to it was the orchestrator's mistake, not the user's choice.) Read-only browser viewers get no clickable paths (or their clicks are refused host-side; host-side refusal is mandatory either way).

### Decided autonomously
- [CWD tracking] Q: "Where is OSC 7 parsed?" → Selected: "In the daemon, from the PTY output stream; cwd changes are kept interleaved with the retained output history" — internal to the daemon.
- [CWD tracking] Q: "Which cwd does a relative path resolve against?" → Selected: "The cwd in effect when the path was printed, found via the last occurrence of the clicked text in the retained history (ANSI stripped)" — resolver logic only.
- [CWD tracking] Q: "OSC 7 from another host (e.g. inside ssh)?" → Selected: "Ignored unless the hostname is empty, localhost, or the host's own name" — one check.
- [Click detection] Q: "How are paths detected?" → Selected: "An xterm link provider in the frontend; strips a trailing :line[:col]; ~ is expanded on the host" — frontend-only, can be reshaped freely. Note: TerminalPanel.web-links.test.tsx currently asserts the source has no registerLinkProvider call (Phase 95 Plan B for OSC 8); that assertion must be updated deliberately, not deleted blindly.
- [Transport] Q: "How does a click reach the host?" → Selected: "A new relay message on that session's own authenticated stream, gated by an API version bump like PreviewAPI" — same pattern as Phase 11.
- [File read] Q: "Which checks before reading?" → Selected: "Move portty open's file checks into shared code used by both the CLI and the daemon" — code is moved, not reinvented.
- [Shells] Q: "bash and fish?" → Selected: "No injection; tracked only when the user's own setup emits OSC 7. Without OSC 7, absolute paths still work and relative ones do nothing" — matches the PATH-01 wording.

### Claude's Discretion
- Plan split, file names, test names, path-detection regex details, the notice's visual details within the macOS-native look.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd_open.go` `cmdOpen`: file checks (stat, regular file, O_NONBLOCK re-check, `relay.PreviewMaxBytes`, `previewMIME`, `previewKey`) to move into shared code.
- `internal/daemon/client.go` `SendPreview` + `Hub.SendPreview` (`internal/relay/`): paced per-subscriber preview delivery, one in-flight preview per hub, typed errors.
- `relay.Subscriber.ReadOnly` (`internal/relay/hub.go:32`): read-only flag already enforced for input frames (MC-03); use it to refuse path-click requests.
- `TerminalPanel.tsx` WebLinksAddon lifecycle (`frontend/src/components/TerminalPanel.tsx:199`, `:865`) and `web/assets/terminal.js` `applyPluginConfig` (L567-737) for the browser viewer's link handling.

### Established Patterns
- Sessions spawn `$SHELL -l` with a fixed argv (`internal/daemon/engine.go:198`, T-01-15) and an env built by `e.cliPathEnv()` (PREV-05 PATH symlink); `PORTTY_SESSION_ID` is set per session. The ZDOTDIR shim joins this env.
- New daemon features are gated by an `APIVersion` bump plus a named constant (`PreviewAPI`).
- Retained output history exists for viewer replay (Phase 9 delta resume); the "printed" check and cwd-at-print lookup should search it.

### Integration Points
- `SessionEngine.CreateSession` (`internal/daemon/engine.go:206`) — env for ZDOTDIR shim.
- `relay.Server.handleSession` (`internal/relay/server.go:46`) — new inbound message type from viewers; read-only check sits next to the MC-03 input discard.
- Phase 11 preview tab handling in the frontend and `web/assets/terminal.js`.

</code_context>

<specifics>
## Specific Ideas

- Example from the roadmap: `cd ~/proj`, `ls` prints `docs/diagram.png`, Cmd-click opens it in a preview tab — local tab, and a home-VM peer tab on an office-VM session over the real tailnet.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
