---
phase: "12"
slug: "file-path-click"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-22"
---

# Phase 12 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 12-01..12-09-PLAN.md: T-12-01..T-12-50, plus T-12-SC, which every plan repeats and is counted once here (51 threats). The audited code is dfff923d..c3d472ef, which includes the WR-02 fix 30ac9abb. The working tree held no product changes at audit time. Config: enforcement on, ASVS 1, block on `high`. The high-severity threats were traced to the boundary where each control sits. The phase's own narrative review, with the VM evidence and verdict, is 12-SECURITY-REVIEW.md.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Viewer → host (path click 0x13) | A viewer sends clicked text, up to 1024 bytes, over its session stream | Untrusted text; the host must check it was printed, the viewer can write, and the file passes the previewfile checks |
| Program output → cwd log (OSC 7) | Any program in the session can emit OSC 7 | Untrusted directory claims; parsed and capped, not a security boundary (AR-12-01) |
| Host → viewers (preview, notice) | File bytes go out as a preview; notices go to the clicker only | Bytes of a file the user can read |
| Daemon → user shell startup (zsh shim) | The daemon writes ZDOTDIR files under its config dir | Shell startup integrity |
| Peer tab ← this Mac's config | A desktop peer tab uses this Mac's webLinks/webLinksConfig only | Link toggles; no CORS, CSP unchanged |

---

## Threat Register

Evidence (file:line) for every row is in the audit return of 2026-11-22. Key controls: `resolvePathClick` runs findPrinted → resolvePath → previewfile.Open in that order (internal/relay/pathclick.go:91-103). The writer-only gate sits in both read pumps (relay/server.go:207-213, webserver/server.go:1007-1013) and again in the hub (pathclick.go:58, 65-69). previewfile does a stat, an O_NONBLOCK open, a descriptor re-check, a bounded read and a type allowlist (previewfile.go:48-90). Notices go to the clicker only (pathclick.go:124-129).

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-12-01 | Tampering | previewfile move | medium | mitigate | CLI check order moved verbatim; TestOpen_Order | closed |
| T-12-02 | EoP | previewfile importers | high | mitigate | Only cmd_open and relay/pathclick import it; source gate test | closed |
| T-12-03 | Tampering | preview name | low | mitigate | CleanPreviewName shared | closed |
| T-12-04 | DoS | zsh shim write | high | mitigate | Refuses bad dir, read-back before ZDOTDIR is set, base env fallback | closed |
| T-12-05 | Tampering | zsh shim files | medium | mitigate | 0700 dir, 0600 file, atomic rename, embedded script | closed |
| T-12-06 | Spoofing | inherited ZDOTDIR var | medium | mitigate | Always overrides PORTTY_ZSH_ZDOTDIR | closed |
| T-12-07 | Tampering | OSC 7 emission | medium | mitigate | Subshell return, writes to /dev/tty | closed |
| T-12-08 | DoS | shim script | medium | mitigate | emulate -L zsh, quoted builtins, always block | closed |
| T-12-09 | Tampering | user dotfiles | high | mitigate | Writes only under configDir/zsh | closed |
| T-12-10 | Spoofing | OSC 7 forgery | low | accept | AR-12-01 | closed |
| T-12-11 | Tampering | OSC 7 parse | medium | mitigate | parseOSC7: file scheme, host, abs, caps, control chars | closed |
| T-12-12 | DoS | cwd log growth | medium | mitigate | Caps, dedupe, prune, RIS clear, alt-screen ignored | closed |
| T-12-13 | Tampering | cwd log handoff | medium | mitigate | Validated on import | closed |
| T-12-14 | Info disclosure | cwd log | low | mitigate | Memory only; directories never logged | closed |
| T-12-15 | Info disclosure | read-only click | high | mitigate | Pump skip + hub refusal + WR-02 joined/not-revoked check | closed |
| T-12-16 | Info disclosure | unprinted path | high | mitigate | Printed-text search on an ANSI-stripped copy, whole tokens only | closed |
| T-12-17 | Info disclosure | notices | medium | mitigate | Clicker only, fixed codes | closed |
| T-12-18 | Info disclosure | logs | medium | mitigate | Session, conn, outcome, bytes only | closed |
| T-12-19 | DoS | click flood | medium | mitigate | 1024-byte cap, one resolve at a time, drops while busy | closed |
| T-12-20 | Tampering | FIFO / special file | medium | mitigate | O_NONBLOCK + descriptor re-check | closed |
| T-12-21 | Tampering | grid ownership | medium | mitigate | FocusSub only, no claim | closed |
| T-12-22 | EoP | relay file access | high | mitigate | No fs/exec imports in pathclick.go | closed |
| T-12-23 | Spoofing | printed path | low | accept | AR-12-02 | closed |
| T-12-24 | Info disclosure | writer reads files | low | accept | AR-12-03 | closed |
| T-12-25 | EoP | share read cap | high | mitigate | Signed-cap readonly flag, pump skip, hub refusal | closed |
| T-12-26 | Spoofing | API version | low | mitigate | Per-connection capability bit | closed |
| T-12-27 | Repudiation | PREV-03 amendment | medium | mitigate | 12-SECURITY-REVIEW.md amendment + verdict | closed |
| T-12-28 | Info disclosure | review doc | medium | mitigate | No caps, links, IPs or paths | closed |
| T-12-29 | Tampering | notice text | medium | mitigate | Exact codes, fixed copy | closed |
| T-12-30 | Tampering | click without modifier | medium | mitigate | Modifier re-checked at activate | closed |
| T-12-31 | Spoofing | URL as path | medium | mitigate | URL-shaped text excluded | closed |
| T-12-32 | Tampering | queued click | low | mitigate | Dropped unless socket open | closed |
| T-12-33 | EoP | read-only links | high | mitigate | No links without host bit; host refusal | closed |
| T-12-34 | Tampering | host app during VM run | high | mitigate | Procedural; host app untouched | closed |
| T-12-35 | Info disclosure | VM docs | high | mitigate | Secret greps 0 | closed |
| T-12-36 | Info disclosure | signer | high | mitigate | Signer and key copies removed | closed |
| T-12-37 | DoS | ephemeral VMs | high | mitigate | Procedural; VMs never stopped | closed |
| T-12-38 | Tampering | VM state | medium | mitigate | Procedural; restored and compared | closed |
| T-12-39 | Tampering | oh-my-zsh download | low | accept | AR-12-04 | closed |
| T-12-40 | Tampering | synthetic re-hover | high | mitigate | Non-bubbling mousemove on .xterm-screen; no mouse report | closed |
| T-12-41 | Info disclosure | peer config fetch | medium | mitigate | Remote tabs never self-fetch | closed |
| T-12-42 | EoP | CORS widening | high | mitigate | No Access-Control-Allow-* anywhere | closed |
| T-12-43 | Spoofing | peer URL links | medium | mitigate | Existing scheme, modifier and confirm gates | closed |
| T-12-44 | Tampering | Linkifier internals | low | mitigate | Field guard + pin test | closed |
| T-12-45 | Info disclosure | read-only after refresh | high | mitigate | Host bit still gates | closed |
| T-12-46 | Tampering | host app during re-run | high | mitigate | Procedural | closed |
| T-12-47 | Info disclosure | re-run docs | high | mitigate | Secret greps 0 | closed |
| T-12-48 | Info disclosure | re-run signer | high | mitigate | No worktree or temp dir left | closed |
| T-12-49 | DoS | ephemeral VMs | high | mitigate | Procedural; VMs running | closed |
| T-12-50 | Tampering | VM state | medium | mitigate | Procedural; cmp clean | closed |
| T-12-SC | Tampering | supply chain | high | mitigate | go.mod, go.sum, package.json, lockfile unchanged | closed |

*Status: open · closed · open — below high threshold (non-blocking)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-12-01 | T-12-10 | A program in the session, or an ssh peer, can forge OSC 7; the cwd log is not a security boundary | plan (12-CONTEXT) | 2026-11-21 |
| AR-12-02 | T-12-23 | A program in the session can print any path; it already runs as the user. The printed check limits viewer clicks only | plan (12-CONTEXT) | 2026-11-21 |
| AR-12-03 | T-12-24 | A writer can read any file the user can (SHAR-02, same as typing `cat`) | plan (12-CONTEXT) | 2026-11-21 |
| AR-12-04 | T-12-39 | oh-my-zsh downloaded only into a disposable VM test user and removed in cleanup | plan (12-07) | 2026-11-21 |
| AR-12-05 | — | Read-only viewers receive previews that writable viewers trigger | user | 2026-11-21 |
| AR-12-06 | — | Paths split by a real newline, and hard-wrapped TUI paths, are never clickable | user | 2026-11-21 |
| AR-12-07 | — | F-1 (row-0 relative path resolves against the start dir) and F-2 (first click after returning to the same cell): todos, not blockers | user | 2026-11-22 |
| AR-12-08 | — | Desktop underline, dark repaint, `ssh localhost`, Claude Code: manual checks on the user's Macs (M-131..M-133) | user | 2026-11-22 |
| AR-12-09 | — | WR-01: a relative path resolves against the latest print of that text, so clicking old output can open a same-named file in a newer directory (12-CONTEXT "last occurrence"); the printed check is not loosened | user ("accept") | 2026-11-22 |

WR-01 accepted by the user on 2026-11-22: see AR-12-09.

---

## Unregistered Flags

- Commit 7a37bfb4 (selection fix, outside the plans): `compensatePointerScale` wraps xterm's private mouse coordinate methods, and `macOptionClickForcesSelection: true`. Neither creates new input: it only remaps real pointer events, and the host still checks every clicked text. Non-blocking (12-REVIEW IN-06).

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-22 | 51 | 51 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
