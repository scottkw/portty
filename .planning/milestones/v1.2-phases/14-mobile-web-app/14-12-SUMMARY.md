---
phase: 14-mobile-web-app
plan: 12
subsystem: verification
tags: [testing-md, full-gate, playwright-iphone, tart-vm, tailnet, whois, lone-esc, vm-checks]
requires:
  - phase: 14
    provides: plans 14-01..14-11 (gate, pages, key bar, pinch, manifest, vmwhois build, copy)
provides:
  - TESTING.md Section 2 counts and Phase 14 note, Section 4 MOB-01..MOB-05 rows (traceability gate green)
  - the full gate run once with every step's exit code, two flakes re-run alone
  - 14-VM-CHECKS.md gate, builds, 24 Playwright emulation rows (I-01..I-24), VM rows P-00..P-07, cleanup, phone-check note
  - one shared `phone` session left running on the office VM for 14-14
affects: [14-13, 14-14]
status: complete
actuals:
  tokens: 13300
  tasks: 2
  commits: 4
plan_head_before: a0f88200d78bc0e6574ed983cca887463c48b7e4
plan_head_after: 16e643289c1aee916b82fb63c74cc8681dc37863
tech-stack:
  added: []
  patterns:
    - "real-tailnet checks only inside the two Tart VMs; links travel in 0600 files by SSH pipe; evidence holds codes, header names and sha256"
key-files:
  created:
    - .planning/phases/14-mobile-web-app/14-12-SUMMARY.md
  modified:
    - TESTING.md
    - .planning/phases/14-mobile-web-app/14-VM-CHECKS.md
key-decisions:
  - "Nothing decided here: both gate flakes, the P-04 cache rule and the vim timing note go to 14-14's checkpoint"
requirements-completed: []  # evidence plan; 14-14's checkpoint decides, so requirements.mark-complete was not run (plan rule)
---

# Phase 14 Plan 12: Full gate, TESTING.md, iPhone emulation rows and Tart VM rows Summary

The merged phase ran the full gate once (two known flakes, both green when re-run alone), TESTING.md now lists every Phase 14 test (Go 174, vitest 122, Playwright 10, total 307), and all eight VM rows P-00..P-07 plus 24 emulation rows are PASS on real machines over the real tailnet, with one shared `phone` session left running for 14-14.

Duration about 40 minutes. Gate log directory (host `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.lZuXernbL5` (`gate-*.log`, `rc.txt`, `rerun-*.log`, `build-*.log`).

## Commits

| Commit | What |
|--------|------|
| a121c3fd | TESTING.md Sections 2 and 4 |
| 835f1da7 | 14-VM-CHECKS.md: gate, builds, emulation rows |
| 0cb5a82c | P-00..P-07 |
| 16e64328 | Cleanup and the phone-check note |

## Gate (run once, 01:23:46 to 01:30:32 UTC)

| Step | Exit | Result |
|------|------|--------|
| build, vet | 0, 0 | clean |
| `go test -race -short -count=1 ./...` | **1** | 13 `ok`, one FAIL: `internal/pty` `TestKillClean_NoOrphans` (G-1), re-run alone `-count=5`: passed |
| vmwhois-tagged test | 0 | ok |
| frozen install, tsc, check-brand, check-share-removal, check-traceability (228 paths), build-script (40 passed) | 0 | clean |
| vitest | **1** | 121 of 122 files, 1808 of 1809 tests; the one failure is `pathLinks.linkifier.test.ts` "after a reconnect, another cell of the same row offers the link" (14-08's deferred flake), alone: 9 of 9 passed |
| Playwright (all projects incl. `iphone`) | 0 | 52 passed, 31 skipped, 0 failed; 11 passes in the `iphone` project |

The plan says any gate failure is a finding for the user's decision; per the caller's instruction both were re-run alone as evidence and execution continued (the VM rows depend on neither). `frontend/src/wailsjs` clean at the end. Not patched.

## TESTING.md

Section 2: Go **174**, vitest **122** (the plan expected 121; `WebShareSessionView.font.test.tsx` was added in 14-11's follow-up), Playwright **10**, build-script 1, total **307**, all equal to `git ls-files`; Phase 14 note above the Phase 13 note. Section 4: 40 MOB rows (MOB-01 15, MOB-02 5, MOB-03 7, MOB-04 4, MOB-05 9). `check-traceability-paths.sh`: OK, 228 paths. Section 5 Category BE (M-145..M-151) is 14-14's; the note says so.

## Builds (sha256, `Signature=adhoc`, `TeamIdentifier=not set`)

| App | Version | sha256 |
|-----|---------|--------|
| HEAD | v1.2.0-vm14 | `95ac988e504dd54648779691089b3e468e5db9ccef4f37be003573a2dd3dcdcb` (equal in both VMs) |
| W14 (`vmwhois`) | v1.2.0-vm14w | `e78236d24228a28c9ffdeb8e3165e767dacfa09295decdebeaf838b62a6ed7ae` (equal in the office VM) |
| p14dev | helper | `b3b549a43a08899617d17b4f80b627fb4d49052cc325bd220550ab0a71fc9071` (equal in the office VM) |

The helper worktree and directory were removed (0 `p14-help` worktrees); builds directory and helper source deleted.

## Rows

| Row | Result | Evidence (one line) |
|-----|--------|---------------------|
| I-01..I-24 | PASS | 11 iphone-project tests, 9 web-same-user (chromium, firefox, webkit), the chromium manifest test, 3 desktop no-key-bar assertions, all from the single run |
| P-00 | PASS | both VMs at their IPs and on the tailnet, HEAD installed (sha equal to host), WebGL off, dotfile sha256 recorded |
| P-01 | PASS | home curl: `/app/` 200, info 200, link 302, probe 401 + `X-Portty`, WebSocket 101; home Safari opens (Session tab, no refusal, typed echo reached the office tail); home sidebar lists the office's `p1`; office Safari join from its own tailnet address |
| P-02 | PASS | W14: `/app/` 403 + `X-Portty` with body sha equal to `web/user-refused.html`, page, info, WebSocket and probe 403 + marker, dashboard and css 200, AX heading "This link only works on your own devices", 0 joins from home, office Safari still opens |
| P-03 | PASS | `fault`: AX "Can't check this device right now" + "Try again", `/app/` and probe 503 with no marker, body sha equal to `web/service-retry.html`; group dropped about 21 to 26 s after the fault, returned 31 s after `pass`; Try again recovered (join 64 s later, slow Safari load, cause not investigated) |
| P-04 | PASS | peer: status p50 4.43 / p95 6.88 / max 11.43 ms, WhoIs p50 3.90 / p95 4.95 / max 9.26 ms; own address: status p95 5.77, WhoIs p95 5.83, self node and same user true; cold load 7 gated requests, 2985 to 3114 ms sequential (about 20 ms per request is the gate); estimate 7 x (6.88 + 4.95) = 82.8 ms |
| P-05 | PASS | reader `1b` 3.145 s (local), 3.18 s (peer), 3.05 s (Safari) before `78`; vim InsertLeave 1.116 s after the ESC on plain vim (vim's own `timeoutlen` 1000 ms), 0.211 s with `ttimeoutlen=50`; E-00 and E-01 were held until the next key |
| P-06 | PASS | no dialog, GUI stderr 0 bytes, no manifest or CSP line, local tab works; home Safari desktop view: no "Terminal keys", tab strip present |
| P-07 | PASS | AX: toolbar Share, switch "Share this session", Copy, warning text equals `SHARE_WARNING`; copied link matches the format (grep -c 1) and equals the CLI's |

P-04 rule recorded for 14-14: "a positive-only verdict cache is worth discussing when the estimate exceeds 300 ms per cold load or either p95 exceeds 50 ms". By the numbers neither holds. Nothing decided.

## Cleanup and the phone check

Both VMs: sessions gone, Safari cleaned (token-pattern search 0 and 0, 0 host mentions), dotfile sha256 equal to P-00 (office `.zsh_history` restored from the private copy), settings restored (`cmp` clean), HEAD relaunched (`/health` `v1.2.0-vm14`), W14 installed nowhere, VMs never stopped and still on the tailnet.

**Phone-check share (left running):** session `phone` (tick counter, `unset HISTFILE` so its shell does not touch the dotfiles) shared, running on the office VM `portty-test` (office, 192.168.64.108, HEAD) and open in its GUI. Its link is only in the 0600 file `phone.url` in the host directory `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/p14-phone.9XH3D6uSdM` (also recorded in 14-VM-CHECKS.md). Checked from the home VM: link 302, info 200. 14-14 ends it and deletes the directory.

## Deviations from Plan

**1. [Finding, not fixed] Two gate failures, both flakes.** `internal/pty` `TestKillClean_NoOrphans` (the known G-1: same test and symptom as Phases 12 and 13, no diff in `internal/pty` since `9574fd49`) and vitest `pathLinks.linkifier` (14-08's deferred flake). Each passed alone. Per the caller, execution continued instead of stopping; both go to 14-14.

**2. [Rule 3 - Blocking] Terminal rows are not readable over AX, in Safari either.** The plan's "AX shows terminal rows" and "no `-- INSERT --` within 1 s" cannot be read literally. Evidence used instead: AX shows the `Session` tab, the `xterm-viewport` and `Terminal input` and no refusal text; the terminal works (typed text reaches the office session tail, the join is in the office log); for vim, the tail sha256 and an `InsertLeave` probe with millisecond timestamps (header already records this limit from 14-02).

**3. [Note] P-05 plain vim clears Insert mode 1.116 s after the ESC, just over the plan's 1 s.** Cause shown by the second run: vim reports `nottimeout`, `timeoutlen=1000`; with its own wait set to 50 ms the probe fires at 0.211 s. The reader legs are the strict proof (ESC logged at its send time).

**4. [Rig] Home Safari ESC.** `p12ui type` of U+001B is not an Escape key event in Safari (the reader logged no `1b`); the leg was repeated with a real key code through System Events and passed. Both attempts are in the row.

**5. [Rule 2 - cleanup extension] Safari kept its own records.** Opening links through `.webloc` files left token and host strings in `History.db`, `SafariTabs.db` and its WAL, and WebKit cache records (Phase 13's `safclean.sh` left them). Removed with a VM-only script and `secure_delete` vacuum until the search read 0 on both VMs; recorded in the Cleanup.

**6. [Slip] `strings` on the home VM raised the "Install Command Line Developer Tools" prompt.** The dialog process was killed; no install ran.

**7. [Slip] A share link prefix appeared once in the executor's tool output** (the P-07 AX dump of the Share popover, `p4`). No repository file holds it; the share was ended at cleanup.

**8. [Count] vitest 122 not 121** (see TESTING.md above). The `EXIT` trap did not fire in the tool's shell, so `frontend/src/wailsjs` was restored by hand after the builds (verified clean).

## Known Stubs

None.

## Threat Flags

None. No product code changed. T-14-28..T-14-33 were followed: only the gate, builds, helper build, `tar`, `shasum`, `codesign -dv`, `tart ip` and SSH ran on the host; W14 is installed nowhere; the one running share is recorded.

## Not verified

- A real phone, the iOS Simulator (14-13) and the user's items (14-14).
- Another Tailscale account against a real second account: the `vmwhois` build makes this VM's own tailnet nodes look like another account; the real WhoIs ran for same-account answers and the fault page came from the mode file.
- Why Safari took 64 s to load the page after Try again (curl answered in under a second at the same time); P-01's load took about 14 s.
- The Safari browser view rows were judged by AX structure and typed-text effects, not by pixels (WebKit screenshots are blank in these VMs).
- AirDrop, Universal Clipboard and Handoff (not available in the VMs); a real TUI such as Claude Code (no node in the VMs).

## Self-Check: PASSED

- Files present: TESTING.md, 14-VM-CHECKS.md, 14-12-SUMMARY.md.
- Commits a121c3fd, 835f1da7, 0cb5a82c, 16e64328 are ancestors of HEAD; `git rev-list --count a0f88200..HEAD` = 4 before this SUMMARY's own commit.
- Rows: P-00..P-07 each PASS in 14-VM-CHECKS.md, 24 I-rows, the secret-pattern grep over 14-VM-CHECKS.md is 0, `frontend/src/wailsjs` clean, 0 `p14-help` worktrees.
- The self-check covers files, commits and rows; the two gate failures are open findings, not resolved.
