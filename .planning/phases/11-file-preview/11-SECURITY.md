---
phase: "11"
slug: "file-preview"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-23"
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Built retroactively on 2026-11-23 (Phase 16 hygiene) from the plan-time `<threat_model>` blocks in 11-01..11-10-PLAN.md, verified against HEAD of `gsd/v1.2` (4f581e08). Earlier evidence: `11-SECURITY-REVIEW.md`, `11-VERIFICATION.md`, `11-VM-CHECKS.md`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| spike worktree → repository (11-01) | Throwaway CSP-loosening spike code must never be committed | Spike patch with configurable CSP |
| executor → VMs (11-01, 11-10) | Spike builds, installs, admin prompts, upgrades, clicks run only in disposable VMs | App bundles, SSH commands, synthesized input |
| executor → host (11-10) | Only gate, builds and SSH on the host; no daemon, GUI, osascript or /usr/local/bin change | Build and test commands |
| VMs → tailnet (11-10) | VMs are ephemeral tailnet nodes the executor cannot re-enroll | Share and peer traffic |
| evidence → repository (11-01, 11-10) | Share checks, CSP headers, log excerpts committed; links and tokens must stay out | HTTP codes, directive names, log lines |
| daemon → hub (11-02) | Bytes a same-UID local process gave the daemon enter the session fan-out | Preview bytes, name, MIME, key |
| hub → viewers (11-02) | Preview frames share each viewer's 256-slot queue with terminal output | MsgPreview (0x21) header, chunk, end frames |
| daemon → session env (11-03) | The daemon chooses what every new shell inherits | PATH, PORTTY_SESSION_ID |
| config dir → session PATH (11-03) | Every new shell searches a directory the daemon writes | `<configDir>/bin/portty` symlink |
| local process / CLI → daemon socket (11-04, 11-08) | Any same-user process can POST; the 0700 socket dir keeps other users out | `POST /sessions/{id}/preview` body + X-Portty-Name/Mime/Key |
| browser page → loopback relay (11-04) | Web pages can reach 127.0.0.1:<relayPort>, so no content-injecting route may live there | Arbitrary page HTTP/WS; no preview route |
| tailnet → share server (11-04) | Tailnet devices reach the share server with a capability token | Share HTTP/WS with cap token |
| remote host → viewer UI (11-05, 11-09) | A possibly compromised share host controls every preview header, byte, name, arrival and focus bit | MsgPreview frames, names, focus flag |
| viewer memory / focus (11-05, 11-09) | Viewers keep previews for days; an arrival could pull focus from an unattended viewer | Blobs, object URLs, active-tab state |
| remote file content → viewer DOM (11-06) | SVG, images and PDFs from a remote shell are untrusted | `blob:` URLs in `<img>` and `<iframe>` |
| share page CSP (11-06) | The browser viewer's second layer; desktop page has only a frames/worker meta CSP | CSP header and meta |
| remote Markdown → viewer DOM (11-07) | Untrusted Markdown; desktop page has no script CSP | Markdown → sanitized HTML |
| npm registry → frontend bundle (11-07) | Two new third-party packages enter app and share page | marked 18.1.3, dompurify 3.4.17 |
| user's shell → CLI (11-08) | The CLI is the only code that opens a path | File path, file bytes |
| GUI → administrator prompt (11-08) | CLI install runs a privileged command through osascript | Executable and link paths as argv |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-11-01 | Tampering | spike patch (configurable CSP) | high | mitigate | Spike lived only in /tmp/p11-spike (gone); 11-01 commits touch docs only; CSP is a fixed string (`internal/webserver/csp_mw.go:156-178`) | closed |
| T-11-02 | Information Disclosure | 11-SPIKE.md | high | mitigate | Gate regex finds 0 caps/tokens/links; links in 0600 files, deleted | closed |
| T-11-03 | Denial of Service | ephemeral VM tailnet nodes | high | mitigate | Process control: never stopped/rebooted, no tailscale down/logout (11-SPIKE.md:8) | closed |
| T-11-04 | Denial of Service | Hub.SendPreview fan-out | high | mitigate | PreviewWindow 16, non-blocking queueIfRoom, stall aborts without kick (`internal/relay/preview.go:34,264,291-298`); TestHubSendPreview_NearCapNoKick/_StallAborts | closed |
| T-11-05 | Denial of Service | memory per hub | medium | mitigate | ErrPreviewBusy; PreviewMaxBytes 20 MB checked before any frame (`preview.go:27,63,153-160`) | closed |
| T-11-06 | Denial of Service | joining viewers | medium | mitigate | Target snapshot skips replaying/revoked subscribers under h.mu (`preview.go:193-198`); test passes | closed |
| T-11-07 | Information Disclosure | daemon.log | low | mitigate | Logs carry DiagID, sizes, MIME, status only — no file name | closed |
| T-11-08 | Tampering | `<configDir>/bin/portty` symlink | medium | mitigate | 0700 bin dir, EvalSymlinks target, self-target refused, atomic temp+rename (`internal/daemon/clilink.go:16-83`); TestCLILink_* | closed |
| T-11-09 | Spoofing | PORTTY_SESSION_ID | low | mitigate | Required mergeEnv entry overrides inherited value (`internal/pty/native.go:69`); TestSpawnPTY_EnvSessionID | closed |
| T-11-10 | Denial of Service | session creation | medium | mitigate | CLI-link failure logs and spawn proceeds (`clilink.go:34-37`); TestCLILink_FailureStillSpawns | closed |
| T-11-11 | Information Disclosure | daemon / share server routes | high | mitigate | Only route takes bytes, never a path (`internal/daemon/api.go:207`); TestPreview_SourceGate, _NotOnRelayHandler, TestPreviewRoute_NotOnShareServer | closed |
| T-11-12 | Spoofing | preview injection from web page / other user | high | mitigate | Route only on the unix-socket mux; socket dir 0700; no MsgPreview read case in either pump | closed |
| T-11-13 | Denial of Service | request body | high | mitigate | 413 on ContentLength, MaxBytesReader, 429 when busy (`internal/daemon/preview.go:50-68`); TestPreview_Refusals/_BusyIs429 | closed |
| T-11-14 | Denial of Service | Phase 9 handoff drain | high | mitigate | Buffered body, async SendPreview, 202 under freezeGate; TestPreview_HandoffDrainNotBlocked | closed |
| T-11-15 | Tampering | file name shown to viewers | medium | mitigate | Single unescape; CleanPreviewName strips C0/C1 and bidi overrides, caps 255 B; viewers clean again | closed |
| T-11-16 | Denial of Service | previewAssembler | high | mitigate | Header size checked before any byte; overflow/mismatch fail; one assembler per connection | closed |
| T-11-17 | Denial of Service | previewStore | medium | mitigate | 10 tabs / 100 MiB retained; never evicts active tab; URLs revoked on swap/drop/closeAll | closed |
| T-11-18 | Tampering | header MIME and name | high | mitigate | Viewer-side PREVIEW_MIMES table sets Blob type; unknown MIMEs ignored | closed |
| T-11-19 | Tampering | resume offset | medium | mitigate | `case 'preview'` never touches offset (`frontend/src/lib/relayClient.ts:464-466`); test passes | closed |
| T-11-20 | Elevation of Privilege (XSS) | Markdown rendering | critical | mitigate | marked drops raw HTML, images→spans; DOMPurify explicit allowlists, fails closed (`frontend/src/lib/markdownSanitize.ts:18-32`); share CSP script-src 'self'; hostile fixtures pass | closed |
| T-11-21 | Information Disclosure | fetch/beacon via SVG | high | mitigate | SVG only as `<img src=blob:>`; share img-src has no network origin; VM beacon 0 hits | closed |
| T-11-22 | Spoofing | links in Markdown | high | mitigate | All activations preventDefault → isAllowedScheme → getRisk → confirm/openLink with noopener,noreferrer | closed |
| T-11-23 | Tampering | share page CSP drift | high | mitigate | `img-src … blob:` + `worker-src 'self'`, frame-ancestors last; TestCSPHeaders_PreviewDelta pins the whole policy | closed |
| T-11-24 | Elevation of Privilege | SVG opened top-level | high | mitigate | Only window.open in preview code is PdfPreview; ImagePreview has none (grep at HEAD — no regression test, see Notes) | closed |
| T-11-25 | Denial of Service | Markdown parser | medium | mitigate | marked pinned 18.1.3; MAX_DEPTH 20; 2M chars / 50k elements caps; 3 s worker timeout; DoS-bound tests pass | closed |
| T-11-26 | Elevation of Privilege | OS PDF parsing | medium | accept | Accepted risk — see Accepted Risks Log | closed (accepted) |
| T-11-27 | Elevation of Privilege | osascript admin command | high | mitigate | `on run argv` + `quoted form of`, absolute /bin/mkdir and /bin/ln without -f (`app_cli_install.go:55-62`); TestInstallCLI_OsascriptArgv | closed |
| T-11-28 | Tampering | /usr/local/bin/portty | high | mitigate | Lstat before prompt; foreign/dangling link or file never overwritten; /Volumes and AppTranslocation refused; TestInstallCLI_* | closed |
| T-11-29 | Denial of Service | daemon restart via `open` | high | mitigate | cliNeedsDaemon excludes open; daemon flags not parsed for open | closed |
| T-11-30 | Information Disclosure | file path | medium | mitigate | Only filepath.Base and a SHA-256 key leave the CLI; URL holds only the session id | closed |
| T-11-31 | Tampering | type confusion | medium | mitigate | Content detection for raster/PDF; SVG and Markdown need extension plus content check | closed |
| T-11-32 | Denial of Service | oversize reads | medium | mitigate | Stat cap, O_NONBLOCK + regular-file re-check, LimitReader(cap+1); daemon caps again | closed |
| T-11-33 | Spoofing | names in tabs, tooltips, panes | medium | mitigate | Only dangerouslySetInnerHTML is MarkdownPreview; TabBar gate test; PreviewPanel hostile-name test (App.tsx half is grep-only, see Notes) | closed |
| T-11-34 | Denial of Service | focus stealing | medium | mitigate | View switches only on focus for owner/its previews; focus sent only to owner or clicker | closed |
| T-11-35 | Denial of Service | viewer connection during preview | high | mitigate | Web view keyed by sessionTabId, hidden not unmounted; local panels stay mounted | closed |
| T-11-36 | Denial of Service | long-running viewer memory | medium | mitigate | Dropped entries remove tabs; per-close revoke; closeAll on unmount and retryInit | closed |
| T-11-37 | Tampering | host machine | high | mitigate | Process control: host ran only gate, builds, tar, shasum, codesign, SSH (11-VM-CHECKS.md:14) | closed |
| T-11-38 | Information Disclosure | 11-VM-CHECKS.md, 11-SECURITY-REVIEW.md | high | mitigate | Gate regex 0 hits (re-run 2026-11-23); 0600 link files deleted | closed |
| T-11-39 | Information Disclosure | VM password | high | mitigate | Typed by user over Screen Sharing; evidence holds references only | closed |
| T-11-40 | Information Disclosure | Safari data (home VM) | medium | mitigate | Three cleanup passes; token search ended at 0 | closed |
| T-11-41 | Denial of Service | ephemeral VM nodes | high | mitigate | Never stopped/rebooted, tailscale untouched (11-VM-CHECKS.md:9,81) | closed |
| T-11-42 | Tampering | VM state after run | medium | mitigate | CLI link, app and disk image removed; settings cmp clean; HEAD redeployed | closed |
| T-11-43 | Information Disclosure | fetch/beacon via Markdown | high | mitigate | Images render as spans; no img/style/link/meta/base in ALLOWED_TAGS; VM beacon 0 hits | closed |
| T-11-SC | Tampering | npm/pip/cargo installs (all plans) | high | mitigate | Only dompurify 3.4.17 and marked 18.1.3 added, exact pins, integrity hashes match review; go.mod unchanged | closed |
| T-11-CR01 | Elevation of Privilege | PDF link navigates desktop preview iframe → Wails frame-agnostic IPC (11-REVIEW CR-01, registered retroactively) | high | mitigate | `frontend/index.html:8` meta CSP `frame-src blob:; child-src blob:; worker-src 'self'`, pinned by text in PreviewRenderers.test.tsx:193-198; VM row RF-01 passed | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-11-01 | T-11-26 | Untrusted PDFs are parsed by the OS viewer (WKWebView PDF view on desktop, Safari's viewer for the top-level blob: in the browser). Portty enables no plugin or script; a damaged PDF in the VM gave a blank 0-page view. Covers parsing only — PDF-link navigation is T-11-CR01, mitigated | User (11-06 plan threat model; 11-SECURITY-REVIEW.md verdict "approved") | 2026-11-21 |

*Accepted risks do not resurface in future audit runs.*

Related residuals the user accepted outside the register (context, not register threats; source 11-VERIFICATION.md and 11-SECURITY-REVIEW.md:277-285): the `/app/*` SPA-shell 200 on `/app/sessions/<id>/preview` (no file bytes; user override 2026-11-21); the Markdown caps showing Couldn't Show File for oversized files; DOMPurify as the desktop page's only XSS layer (bump todo `2026-11-21-dompurify-3418`, due 2026-11-28); PORTTY_SESSION_ID inherited inside tmux/screen; Wails v2.10.2 `external` handler accepting any frame (upstream).

---

## Notes (non-blocking)

- **VM addresses in evidence:** 11-SPIKE.md and 11-VM-CHECKS.md record ephemeral VM node names, 100.x addresses and a share listener. Not caps, tokens or links (gate regex 0 hits; share server still needs a token), but they contradict those files' "directive names only" wording. Candidate redaction to `<host>`.
- **Grep-only gates:** T-11-24 and the App.tsx half of T-11-33 hold at HEAD by grep but have no persistent regression test.
- **Phase 12 path click** extends PREV-03 (MsgPathClick → host read of session-printed text); owned by `12-file-path-click/12-SECURITY.md`, outside this register.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-23 | 45 | 45 | 0 | gsd-security-auditor (opus) via /gsd-secure-phase 11, at HEAD 4f581e08; targeted Go and vitest (351/351) runs green |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-23
