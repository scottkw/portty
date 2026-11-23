---
phase: 11-file-preview
plan: 10
subsystem: verification
status: complete
tags: [vm-check, gate, testing-md, security-review, preview, upgrade, safari, csp]
requires: [11-01, 11-02, 11-03, 11-04, 11-05, 11-06, 11-07, 11-08, 11-09]
provides:
  - "The Phase 11 gate, run once and green"
  - "11-VM-CHECKS.md: P-00..P-07, P-08a..P-08f, P-10, U-01..U-03, all PASS (P-08c entered by the user over Screen Sharing)"
  - "TESTING.md: Phase 11 Section 2 note and counts, 30 PREV rows in Section 4, Category BA (M-121..M-127)"
  - "11-SECURITY-REVIEW.md: VM evidence and verdict (approved)"
affects: [ROADMAP Phase 11 SC1-SC5 evidence, /gsd-verify-work]
tech-stack:
  added: []
  patterns:
    - "Rendering judged from the AX tree (desktop) and WebDriver DOM reads (Safari); screencapture over SSH does not draw web content"
    - "Tab clicks scroll the tab strip first (Scroll tabs left/right) when the tab is out of view"
key-files:
  created:
    - .planning/phases/11-file-preview/11-VM-CHECKS.md
  modified:
    - TESTING.md
    - .planning/phases/11-file-preview/11-SECURITY-REVIEW.md
decisions:
  - "P-07's one 200 (GET /app/sessions/<id>/preview) is the /app/* SPA fallback, byte-identical to GET /app/ (11-04 Deviation 1); recorded as PASS with the evidence"
  - "hostile-external.svg is refused by portty open (DOCTYPE internal subset); its fetch vectors were tested with the same file minus the DOCTYPE block"
  - "P-08c: the run has no VM password, so the user typed it into the prompt over Screen Sharing; the orchestrator verified the link and `portty list`; PASS"
requirements-completed: [PREV-02, PREV-04]  # added by Phase 16 (16-03); delivered across 11-06, 11-07 and 11-09 and proven at this gate; 11-VERIFICATION.md truths 3, 5 and 8 (passed; PREV-02 desktop look: deferred, accepted)
metrics:
  duration: "~55 min"
  completed: 2026-11-21
actuals:
  tokens: 21500    # chars/4 over the realized diff (86,108 chars)
  tasks: 3         # all complete (P-08c after the checkpoint)
  commits: 6
plan_head_before: 7042f5ce3a8bb1afc3d62c89f13b0a3b326a8fd1
plan_head_after: affbc988c22736900b2330ec181dcaa5ffcd2c9c
---

# Phase 11 Plan 10: Phase gate, TESTING.md and VM checks Summary

The merged Phase 11 passed the full gate in one run, and every VM row but one passed on the two Tart VMs over the real tailnet: previews in both GUIs and Safari, all seven types in light and dark, hostile files silent against loopback beacons, a 19.9 MB preview to four viewers with a ticking terminal, over-cap refusal, route absence, the PATH link, the install menu's warnings and cancel, and three upgrade rows. P-08c (the actual install through the administrator prompt) also passed, once the user typed the VM password over Screen Sharing. Every row is PASS.

## Gate (host, once, 17:39:40-17:42:09 UTC, `GOTOOLCHAIN=go1.26.11`)

Logs in the session scratchpad (`gate/*.log`):

- `go build ./...` and `go vet ./...`: clean.
- `go test -race -short -count=1 ./...`: 13 packages `ok`, 0 FAIL lines. **`--- SKIP` lines: none.** No DATA RACE or panic. No flake reruns.
- `pnpm install --frozen-lockfile`: clean.
- `pnpm vitest run`: 104 files, 1510 tests passed.
- `tsc --noEmit`: clean.
- check-brand, check-share-removal and check-traceability (188 paths): clean.
- `tests/build-script.test.sh`: 40 passed, 0 failed.
- `frontend/src/wailsjs`: clean afterwards.

## TESTING.md counts (final)

Go 157, vitest 104, Playwright 7, build-script 1, total 269. The plan expected vitest 103. The 104th file is the 11-07 fix's parseMarkdownInWorker.test.ts. Section 4 has 30 PREV rows (PREV-01..PREV-05). Section 5 has Category BA, M-121..M-127. M-126's last-run line records P-08c as PASS.

## Builds (ad-hoc, `Signature=adhoc`, `TeamIdentifier=not set`)

| App | Version | sha256 |
|-----|---------|--------|
| A (7dbe384e) | v1.2.0-vmA | `558d653f89959dd6b69886a4fd6fa691c3e71e4c1e6fec0de08d2f75bee3f5eb` |
| B | v1.2.0-vmB | `248f596870e413f69ce7c96256f4b603f23b59e41f2f31e37ecf5305b972d6f8` |
| B2 | v1.2.0-vmB2 | `f517f65311c2a946f9b9ec25c3eb2289c1590094e9a61852ab54d0e6a2323fd0` |
| HEAD | dev | `4230ab8a7900e3b2032d49b8020750dc41f9f60ed3c5df7d6c9b0fec9e24787b` |

## Rows

| Row | Result | One line |
|-----|--------|----------|
| P-00 | PASS | B on both (`"api":3`), each lists the other in `tailscale status`, WebGL off, beacons self-tested, samples made |
| P-01 | PASS | Office typed: office switched, home got a background tab. Home typed: home switched, office did not. × returned to p1 with the terminal focused and shell 34184 unchanged. The missing and unsupported files printed the exact 11-08 lines |
| P-02 | PASS | All seven types render in both GUIs (AX) in light and after a live dark switch, with no log line and no reconnect. mdslow.md hit Couldn't Show File at about 3 s on the `wails://` page. A damaged PDF shows a blank native viewer (recorded) |
| P-03 | PASS | Both beacons logged 0 connections across three viewers, plus 30 s after. Safari: 0 violations and 0 errors. hostile-external.svg was refused by the CLI, so its vectors were tested without the DOCTYPE |
| P-04 | PASS | Safari previews (PNG, Open PDF opening a `blob:` application/pdf window, Markdown through the same-origin worker). The served CSP equals Amendment 3. A wrong token got 401. 0 violations. The dark repaint was confirmed by computed styles |
| P-05 | PASS | 30 s on a preview and back: no log line, `stty` unchanged. Only the later typing claimed the grid (`why=input`) |
| P-10 | PASS | `Portty open --help` printed the usage with rc 0 and started no GUI or daemon. In a session, `command -v portty` gives `…/portty/bin/portty` |
| P-06 | PASS | near.jpg to four viewers, all `status=ok`. No leave, too-slow or heartbeat line. Every viewer's ticks are gapless |
| P-07 | PASS | over.png printed the too-large line in every viewer, with no tab and no `preview accepted`. Probes: 405/405/404 on the share server, plus the SPA-fallback 200 (byte-identical to `/app/`). The relay answered 404/404 |
| P-08a | PASS | From /Applications, a new session resolves the bin link (pointing to /Applications), and `portty open` works |
| P-08b | PASS | Terminal.app: `command -v portty` gave rc=1 |
| P-08c | PASS | The user typed the VM password over Screen Sharing. `/usr/local/bin/portty` became a root link to `/Applications/Portty.app/Contents/MacOS/Portty`. In a clean `env -i` login zsh, `command -v portty` gave `/usr/local/bin/portty` and `portty list` exited 0. The success dialog text was not captured |
| P-08d | PASS | Prompt appeared. A synthesized Don't Allow click was accepted. No dialog, no link, nothing logged |
| P-08e | PASS | Foreign link: warning naming `/bin/echo`, no prompt, link unchanged. Our own link: already-installed notice, no prompt |
| P-08f | PASS | DMG copy: move-to-Applications warning naming `/Volumes/PorttyTest`, no prompt |
| U-01 | PASS | B's CLI against A's daemon printed "update needed" and A kept running. After the upgrade, 4 of 4 viewers resumed with delta within 2.1 s, ticks gapless, 0 grid lines. The moved session got the not-a-session line. New session u2 previewed |
| U-02 | PASS | B2 handoff mid-preview committed. 3 of 3 viewers resumed with delta within 4.5 s. Both home viewers show Couldn't Receive File. A later open worked under B2 |
| U-03 | PASS | An A peer tab got near.jpg with `status=ok`, no leave, ticks gapless, no error, no tab |

### Near-cap stream times (P-06, 19,900,000 bytes)

Office local tab 168 ms, home peer tab 14.953 s, home attach 19.372 s, home Safari 19.406 s (home over DERP). In U-03, the A peer tab took 36.595 s.

### Reconnects

- **U-01** (A to B): 4 of 4 `mode=delta`.
  - Office local tab: +1.18 s, 69 B.
  - Three home web viewers: +1.91 s, +1.98 s and +2.03 s, 92 B each.
  - Takeover moved in 391 ms.
- **U-02** (B to B2 mid-preview): 3 of 3 `mode=delta`.
  - Office local tab: +1.16 s, 66 B.
  - Safari: +3.42 s, 154 B.
  - Home peer tab: +4.45 s, 220 B.
  - Takeover moved in 1.009 s.
  - The CLI waited 3.9 s before the takeover, most likely the first-launch code check of the new binary.

### Admin-prompt path

The prompt appeared and SecurityAgent accepted a synthesized click (Don't Allow, P-08d). This run has no VM password and must not guess or store one, so a checkpoint was raised for P-08c. The user typed the password over Screen Sharing, and the orchestrator verified the result at 18:35 UTC. The success dialog's exact text was not captured.

## Security review

**Verdict: approved.** It rests on P-03, P-04 and P-07, which all passed. The review also records:

- macOS 27.2 shows its own prompt copy, not the app's `with prompt` text.
- The SVG DOCTYPE refusal.
- The desktop page has no CSP, and its defences held.
- The residual risks: the OS PDF viewer, the DOMPurify bump todo, the 3 s Markdown limit, `PORTTY_SESSION_ID` inherited through tmux/screen, and the SPA fallback.

## Commits

- `f43f20bb` docs(11-10): TESTING.md Sections 2 and 4 for Phase 11
- `28132fe1` docs(11-10): VM checks header, gate result and builds
- `e73fb9f5` docs(11-10): VM rows P-00..P-05 and P-10
- `909c7521` docs(11-10): VM rows P-06 and P-07
- `df0e80ed` docs(11-10): VM rows P-08a, P-08b, P-08d..P-08f and U-01..U-03
- `affbc988` docs(11-10): TESTING.md Category BA, security review verdict, VM cleanup (P-08c pending)

## Deviations from Plan

1. **P-08c by checkpoint.** P-08d ran before P-08c: Cancel with no link present, so no `sudo rm` was needed. The user then entered the password for P-08c over Screen Sharing (PASS; the success dialog text was not captured). Afterwards the office was cleaned up: link removed, /Applications/Portty.app gone, settings restored, HEAD `dev` running with 0 sessions, and stage removed (11-VM-CHECKS.md Cleanup, "office (final …)").
2. **P-07 bar.** `GET /app/sessions/<id>/preview` answers 200. That is the `/app/*` SPA fallback, byte-identical to `GET /app/`, which 11-04 recorded as a production behaviour the plan's "never 2xx" can't meet. Recorded as PASS with the body hash. Flagged for the user.
3. **hostile-external.svg.** `portty open` refuses the committed fixture: its DOCTYPE internal subset contains `>`, which hits the 11-08 shortcut. The fetch vectors were tested with `hostile-external-nodtd.svg` (the same file minus the DOCTYPE block).
4. **Rendering evidence.** `screencapture` over SSH draws no web content in these VMs, so screenshots were not usable. Rendering came from AX (desktop) and WebDriver DOM reads (Safari), as 11-01 did. Dark-mode colours were checked in Safari.
5. **P-06 typist.** The near-cap open was typed into the home `portty attach`, so no GUI left its terminal while ticks were sampled. Each GUI and Safari then showed the preview by clicking its tab.
6. **WebP sample.** `sips` can't write WebP in the VM, so a 320x200 WebP was made on the host (`magick`, `cwebp`) and written into the VM from base64.
7. **A build source.** A came from a `git archive` of 7dbe384e in the scratchpad instead of `git worktree add` (no worktree to remove).
8. **U-03 office build.** The office ran B2 (B's tree, version only) from U-02, not a fresh B.
9. **Rig slip (P-03).** A coordinate click on the shrunken tab strip hit p1's × at 17:53:04 and closed the office's p1 tab. The session, shell and previews stayed, matching UI-SPEC's close rule. The tab was reopened from the sidebar and the tab clicker now scrolls the strip first.
10. **Terminal.app (P-08b).** The command was typed with System Events into a new Terminal window, with output to a file, as the plan says.

## Observations for the user

- **macOS 27.2's admin prompt** doesn't show "Portty wants to install the portty command in /usr/local/bin." Instead it shows "Allow administrator access for a script started by “Portty”?" plus Apple's "could not verify this script is free of malware" warning. That is alarming copy for users. UI-SPEC's prompt line can't be met on this macOS.
- **SVGs with an internal DTD subset** (some editors write entity declarations) are refused by `portty open` as an unsupported type.
- **Damaged PDFs** show a blank native viewer on the desktop, not Couldn't Show File.
- **11-09 limits.** The live-region double announce and the type-change icon limit were not exercised. The same name never arrived twice in a row, and no refresh changed type.

## Known Stubs

None (no product code changed).

## Threat Flags

None. No product code changed. Evidence records codes and directive names only, and the secret-pattern greps over 11-VM-CHECKS.md and 11-SECURITY-REVIEW.md are 0.

## Self-Check: PASSED

- FOUND: .planning/phases/11-file-preview/11-VM-CHECKS.md, TESTING.md (Category BA), 11-SECURITY-REVIEW.md (`**Verdict:**`)
- FOUND: f43f20bb, 28132fe1, e73fb9f5, 909c7521, df0e80ed, affbc988 (ancestors of HEAD)
- P-08c recorded as PASS, so Task 3's row check has no pending row.
