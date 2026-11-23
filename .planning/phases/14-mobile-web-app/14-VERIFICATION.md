---
phase: 14-mobile-web-app
verified: 2026-11-22T21:45:00Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - ".planning/phases/14-mobile-web-app/14-01-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-01-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-02-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-02-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-03-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-03-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-04-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-04-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-05-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-05-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-06-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-06-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-07-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-07-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-08-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-08-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-09-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-09-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-10-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-10-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-11-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-11-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-12-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-12-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-13-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-13-SUMMARY.md"
  - ".planning/phases/14-mobile-web-app/14-14-PLAN.md"
  - ".planning/phases/14-mobile-web-app/14-14-SUMMARY.md"
  - "CHANGELOG.md"
  - "README.md"
  - "TESTING.md"
  - "build/appicon-fullbleed.png"
  - "cmd/playwright-fixture/main.go"
  - "cmd_share.go"
  - "cmd_share_test.go"
  - "docs/launcher-app.md"
  - "frontend/e2e/mobile-iphone.spec.ts"
  - "frontend/e2e/selection-offset.spec.ts"
  - "frontend/e2e/web-csp.spec.ts"
  - "frontend/e2e/web-same-user.spec.ts"
  - "frontend/e2e/web-share-scope.spec.ts"
  - "frontend/index.html"
  - "frontend/playwright.config.ts"
  - "frontend/public/apple-touch-icon.png"
  - "frontend/public/icon-192.png"
  - "frontend/public/icon-512.png"
  - "frontend/public/manifest.json"
  - "frontend/src/App.tsx"
  - "frontend/src/__tests__/homeScreen.test.ts"
  - "frontend/src/components/ErrorBoundary.tsx"
  - "frontend/src/components/KeyBar.tsx"
  - "frontend/src/components/PdfPreview.tsx"
  - "frontend/src/components/SessionEndedBanner.tsx"
  - "frontend/src/components/SettingsSheet.tsx"
  - "frontend/src/components/TabBar.tsx"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/UserRefused.tsx"
  - "frontend/src/components/WebShareSessionView.tsx"
  - "frontend/src/components/__tests__/App.webLink.test.tsx"
  - "frontend/src/components/__tests__/ErrorBoundary.test.tsx"
  - "frontend/src/components/__tests__/KeyBar.test.tsx"
  - "frontend/src/components/__tests__/PreviewRenderers.test.tsx"
  - "frontend/src/components/__tests__/SessionEndedBanner.test.tsx"
  - "frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx"
  - "frontend/src/components/__tests__/SharePopover.test.tsx"
  - "frontend/src/components/__tests__/TabBar.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.pinch.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.touchscroll.test.tsx"
  - "frontend/src/components/__tests__/UserRefused.test.tsx"
  - "frontend/src/components/__tests__/WebShareSessionView.font.test.tsx"
  - "frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx"
  - "frontend/src/components/__tests__/WebShareSessionView.test.tsx"
  - "frontend/src/components/__tests__/style.keybar.test.ts"
  - "frontend/src/components/__tests__/style.mobile.test.ts"
  - "frontend/src/lib/__tests__/closeReasons.test.ts"
  - "frontend/src/lib/__tests__/ctrlMap.test.ts"
  - "frontend/src/lib/__tests__/phoneQuery.test.ts"
  - "frontend/src/lib/__tests__/pinchZoom.test.ts"
  - "frontend/src/lib/__tests__/visualViewportInset.test.ts"
  - "frontend/src/lib/closeReasons.ts"
  - "frontend/src/lib/ctrlMap.ts"
  - "frontend/src/lib/phoneQuery.ts"
  - "frontend/src/lib/pinchZoom.ts"
  - "frontend/src/lib/relayClient.test.ts"
  - "frontend/src/lib/relayClient.ts"
  - "frontend/src/lib/shareCopy.ts"
  - "frontend/src/lib/visualViewportInset.ts"
  - "frontend/src/style.css"
  - "internal/daemon/share_test.go"
  - "internal/daemon/share_vmwhois.go"
  - "internal/daemon/share_vmwhois_test.go"
  - "internal/relay/oscabsorb.go"
  - "internal/relay/oscabsorb_relay_test.go"
  - "internal/relay/oscabsorb_test.go"
  - "internal/tailnet/tailnet.go"
  - "internal/webserver/app_static_test.go"
  - "internal/webserver/capability_helpers_test.go"
  - "internal/webserver/csp_integration_test.go"
  - "internal/webserver/csp_mw.go"
  - "internal/webserver/csp_mw_test.go"
  - "internal/webserver/inject_test.go"
  - "internal/webserver/origin_mw_test.go"
  - "internal/webserver/oscabsorb_relay_test.go"
  - "internal/webserver/server.go"
  - "internal/webserver/server_test.go"
  - "internal/webserver/session_ended_test.go"
  - "internal/webserver/webservertest/whois.go"
  - "internal/webserver/whois.go"
  - "internal/webserver/whois_gate_test.go"
  - "launcher_copy_test.go"
  - "release_guard_test.go"
  - "tests/check-brand.sh"
  - "web/assets/dashboard.css"
  - "web/embed.go"
  - "web/service-retry.html"
  - "web/user-refused.html"
covered_digest: "v4:sha256:dfc69f3a6bd872bb4dbb75de1509187e2c85fd9c2e1af9c6f4252a7d5d1804ed"
behavior_unverified: 0
overrides_applied: 0
accepted_limits: # Recorded in the user's own words in 14-VM-CHECKS.md "User decisions"; not gaps
  - "S-02: the gap above the keyboard is iOS's own form-assist bar; Portty's key bar sits directly on it"
  - "UI-A: another account shown with the vmwhois build on the VMs; tagged and shared-in nodes pinned by tests"
  - "UI-B: WhoIs kept uncached (82.8 ms per cold load against the 300 ms rule)"
  - "UI-C: simulator rows accepted as evidence, backed by the real-phone checks"
known_flakes:
  - "G-1 internal/pty TestKillClean_NoOrphans (accepted in Phases 12 and 13)"
  - "pathLinks.linkifier.test.ts reconnect flake (todo: .planning/todos/pending/2026-11-22-linkifier-reconnect-flake.md); passed in this run"
post_run_fixes: # 14-REVIEW-FIX.md, committed after every VM, simulator and real-phone run
  - id: WR-01
    new_device_evidence_needed: false
    reason: "The heal was never observed on a device before or after the fix; the fix only stops it firing on a landscape rotation. Unit tests pin both cases. Residual: the heal stays an [ASSUMED] belt for an iOS standalone bug."
  - id: WR-02
    new_device_evidence_needed: false
    reason: "Pure input-transform logic with no platform dependence; unit test plus the iPhone-emulation Ctrl tests (I-05, I-06) pass."
  - id: WR-03
    new_device_evidence_needed: false
    reason: "For touches on the terminal the effective touch-action was pan-y before and after (the container's own pan-y already excluded pinch-zoom), and the non-passive two-touch touchmove handler is unchanged, so S-03 and the real-phone run still describe the terminal's behaviour. Computed-style Playwright test and unit tests pass."
  - id: IN-02
    new_device_evidence_needed: false
    reason: "Cleanup-only (cancel the heal frame, never leave .app hidden); unit-tested."
---

# Phase 14: Mobile Web App Verification Report

**Phase Goal:** The user can drive a shared session from an iPhone. The share view fits the phone, adds the keys iOS lacks, installs to the Home Screen and picks up again after the phone sleeps or changes network. The host Mac serves all of it over the tailnet, and portty.app/app is only a static launcher page.
**Verified:** 2026-11-22
**Status:** passed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC5 | Built first: a browser share link opens only for devices on the host's own Tailscale account; another user's WhoIs is refused with "This link only works on your own devices" before any terminal data; page, info and WebSocket routes all gated; fails closed when WhoIs errors (MOB-05) | VERIFIED | `internal/webserver/server.go`: every session-bearing route is wrapped: `GET /api/sessions` :596, `/info` :602, plugin-config :608 and :613 with `requireSameUser(requireCapability(...))`; `/sessions/{id}` :623 and `/app/` :691 with `cspHeaders(requireSameUserPage(...))`; `/sessions/{id}/ws` :634 with `requireAllowedOrigin(requireSameUser(requireCapability(...)))`. Only `/`, `/dashboard` and `/assets/` stay open (no session data), as decided. `whois.go:87-109` `sameUserStatus`: 3 s timeout, status error or nil Self gives 503, WhoIs error gives 503, not-found or another user gives 403. `requireSameUserPage` serves the embedded `web/user-refused.html` (403 + marker) or `web/service-retry.html` (503). Tests run now: `go test -race ./internal/webserver/` ok (includes `whois_gate_test.go`), Playwright `web-same-user.spec.ts` 3/3 on chromium and webkit. Real tailnet: P-01 (same account, no prompt), P-02 (vmwhois other account: 403 + marker on page, info, WS; no join logged), P-03 (fault page). |
| SC1 | On an iPhone the share view fits the phone's width, keeps the cursor line above the keyboard, shows a key bar (Esc, Tab, Ctrl, arrows, `\|`) whose keys reach the shell; Ctrl then C stops a running command; pinching changes the font; desktop browser view unchanged (MOB-01) | VERIFIED | `KeyBar.tsx` rendered by `WebShareSessionView.tsx:268` only when `phone && isActive && canWrite !== false && !useProxy`; keys go through `TerminalPanel.tsx:593` `term.input(seq, true)` (the typing path); `ctrlMap.ts` maps Ctrl; `inputTransform` applies an armed Ctrl to keyboard input. `pinchZoom.ts` attached at `TerminalPanel.tsx:855`. `visualViewportInset.ts` drives `--vv-height`/`--vv-top` and `keyboardUp` (App.tsx:116, passed at :1356). Lone Esc: `oscabsorb.go:154` end-of-frame flush. Tests run now: Playwright iphone project `mobile-iphone.spec.ts` 12/12 (Esc 1b, Tab 09, pipe 7c, Up, Ctrl+Up CSI 1;5A, Ctrl then c gives 03, 44 pt fit, chrome, terminal-only touch-action); `web-share-scope.spec.ts` (desktop: no "Terminal keys" toolbar) on chromium and webkit; full vitest 122 files / 1813 tests. Device: S-01, S-02 (accepted iOS limit), S-03, P-05 (lone Esc acts at once in vim through all three viewers), P-06 (desktop unchanged), PHONE-1 (real iPhone, standalone, key bar directly above iOS's form-assist bar). "Stops a running command": 0x03 reaches the PTY input on the same pump the VM rows exercised; the tty line discipline turns it into SIGINT. |
| SC2 | Add to Home Screen gives a Portty icon that opens the session full screen, every asset from the host; token still gates it (icon shows Link No Longer Works after sharing stops); CSP changes only as approved (MOB-02) | VERIFIED | `frontend/public/manifest.json`: name/short_name Portty, `display: standalone`, `scope: ./`, no `start_url`, no `id`, local icons only. `frontend/index.html:9-15` head tags; built `dist/index.html` uses `./manifest.json` and `./apple-touch-icon.png` (relative under `/app/`). `csp_mw.go`: Phase 14 commit f26647f8 adds exactly one line, `manifest-src 'self'`; `default-src 'none'` kept. Playwright `web-csp.spec.ts` (Chromium parses the manifest under the share CSP; zero violations on chromium and webkit) passed now. Device: H-01 and S-04 (query kept on Home Screen launch), S-05 (safe areas), S-06 (Open PDF Done), S-07 (share off: stopped banner, Reload, Link No Longer Works, relaunch shows it too), PHONE-1 (real iPhone: icon on a Home Screen page, full screen, ticks counting). CSP Amendment 4 approved by the user (14-VM-CHECKS.md User decisions; 14-SECURITY-REVIEW.md). |
| SC3 | Human check (real iPhone): after sleep or a Wi-Fi/cellular switch the view reconnects on its own and resumes with no missing or duplicated output; recorded as an M-NN item; reconnect logic unit-tested (MOB-03) | VERIFIED | `relayClient.ts:310` pageshow wake (removed at :649), `onConnectStuck` at :590; Reload on stopped/ended/unreachable/stuck banners (`TerminalPanel.tsx:1412`, `:1429`), ErrorBoundary web copy with Reload. Unit tests: `relayClient.test.ts:962` (pageshow) and `:1090` (iOS-like suspend with no visibility or pageshow event still probes and redials), in the passing vitest run. Recorded as M-148 in TESTING.md Category BE. PHONE-2 PASS on the user's real iPhone 2026-11-22: 10-minute lock, Wi-Fi off and on, 2-minute app switch: "All three worked, no gaps". |
| SC4 | This repo supplies the copy and link format for portty.app/app; that page holds no token, opens no WebSocket and has no shell access; nothing in the origin check, CSP or token handling trusts portty.app (MOB-04) | VERIFIED | `docs/launcher-app.md` (header comment: static, no inputs, no token, no WebSocket; link format `https://<your-mac>.<your-tailnet>.ts.net:7444/sessions/…`; Messages/iCloud warning; Add to Home Screen steps). `launcher_copy_test.go` `TestLauncherCopy` passed now. `grep portty.app` over internal/, cmd/, frontend/src and root Go sources finds only `main.go:158 helpURL` (a Help-menu link), nothing in origin, CSP or capability code. P-07: launcher steps match the real Share popover. |

**Score:** 5/5 roadmap success criteria verified (0 present-but-behavior-unverified).

### Plan must-have truths

All 14 plans' frontmatter truths were checked against the code and the evidence files. Highlights beyond the SCs:

| Plan | Truth (abridged) | Status | Evidence |
|------|------------------|--------|----------|
| 14-01 | Every test binary and the fixture install a WhoIs fake; fixture identity switch and input drain | VERIFIED | `webservertest/whois.go`, `capability_helpers_test.go`, fixture `/__test__/whois` and `/__test__/input`; key link `SetWhoIsClientForTest` verified |
| 14-02 | Bare ESC at the end of a MsgInput frame is forwarded at once through both pumps; P9 changed on purpose; OSC/CSI absorption kept | VERIFIED | `oscabsorb.go:154`; `go test -race ./internal/relay/` ok; E-00/E-01 before, P-05 after |
| 14-03 | Static refusal and retry pages; no-cache for `/app/` files outside assets/ | VERIFIED | `web/user-refused.html`, `web/service-retry.html`, `app_static_test.go` `TestAppStatic_CacheHeaders` (webserver package passed) |
| 14-04 | SPA tells a marked 403 apart; reconnect-probe refusal is final; pageshow wake; stuck signal | VERIFIED | `WebShareSessionView.tsx:174` `shareInfoVerdict(res.status, X-Portty === '1')`; `App.tsx:1265`/`:1349` `UserRefused`/`onOtherAccount`; relayClient tests |
| 14-06 | vmwhois fake never in a release graph; check-brand guard | VERIFIED | `TestNoFakeWhoIsInRelease` passed now; `TestVMWhoIs_Modes` (tag vmwhois) passed now; `tests/check-brand.sh` "OK: brand gate clean" |
| 14-08 | Phone query module; standalone Open PDF with Done | VERIFIED | `phoneQuery.ts` (`(pointer: coarse) and (hover: none)`); `PdfPreview.tsx:76` `isStandalone()` picks the in-app view; S-06 |
| 14-09 | Share copy true in the GUI and CLI | VERIFIED | `shareCopy.ts` and `cmd_share.go` both carry "Treat this link like a password. It only opens on devices signed in to your Tailscale account."; README:22 updated |
| 14-10/11 backstops | Armed Ctrl readable in light/dark on real WebKit; keyboard inset and safe areas in standalone; page zoom blocked on iOS 26 | VERIFIED (device) | S-05 (light/dark identical frames, armed Ctrl readable), S-03, PHONE-1; UI-C accepted by the user |
| 14-12 | Full gate once; TESTING.md Sections 2 and 4; VM rows P-00..P-07 | VERIFIED | 14-VM-CHECKS.md rows all PASS; `tests/check-traceability-paths.sh` "OK: 228 traceability paths exist" |
| 14-14 | Category BE M-145..M-151; accepted limits only in the user's words | VERIFIED | TESTING.md:1797-1839; 14-VM-CHECKS.md "User decisions" |

### Required Artifacts

`gsd-tools verify.artifacts` on all 14 plans: 40/40 artifacts exist and are substantive (exit 0 on every plan).

### Key Link Verification

`gsd-tools verify.key-links`: 26/31 verified by the tool. The other 5 failed only because their patterns contain an unescaped `(` (not valid RE2); each was checked literally with `grep -F` and is WIRED:

| From | To | Pattern | Status |
|------|----|---------|--------|
| server.go | whois.go | `ws.requireSameUser(ws.requireCapability(` | WIRED (:602, :608, :613, :634) |
| server.go | whois.go | `ws.cspHeaders(ws.requireSameUserPage(` | WIRED (:623, :691) |
| WebShareSessionView.tsx | closeReasons.ts | `shareInfoVerdict(res.status` | WIRED (:174) |
| KeyBar.tsx | TerminalPanel.tsx | `term.input(` | WIRED (:593) |
| pinchZoom.ts | TerminalPanel.tsx | `attachPinchZoom(` | WIRED (:855) |

### Behavioral Spot-Checks (run by the verifier)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gate, CSP, absorber, tailnet probe | `go test -race -count=1 ./internal/webserver/ ./internal/relay/ ./internal/tailnet/` | ok / ok / ok | PASS |
| vmwhois modes | `go test -tags vmwhois -run TestVMWhoIs_Modes ./internal/daemon/` | PASS | PASS |
| Launcher copy, release guard, share CLI | `go test -run 'TestLauncherCopy\|TestNoFakeWhoIsInRelease\|TestShare' .` | ok | PASS |
| Frontend unit suite (full, once) | `NO_COLOR=1 npx vitest run` (node v24.22.1) | 122 files, 1813 tests passed | PASS |
| iPhone emulation | `npx playwright test --project=iphone e2e/mobile-iphone.spec.ts` | 12 passed | PASS |
| Gate pages, manifest under CSP, desktop scope | `npx playwright test --project=webkit --project=chromium web-same-user web-csp web-share-scope` | 13 passed | PASS |
| Types | `npx tsc --noEmit` | exit 0 | PASS |
| Guards | `bash tests/check-traceability-paths.sh`; `bash tests/check-brand.sh` | OK; OK | PASS |

Nothing touched the host's tailscaled or a 100.x address: every Go test installs the fake, and Playwright serves from the loopback fixture.

### Probe Execution

Not applicable: no `scripts/*/tests/probe-*.sh` is declared by this phase.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| MOB-05 | 14-01, 03, 04, 06, 09, 12, 14 | Browser links only for the host's own Tailscale account, fail closed | SATISFIED | SC5 row |
| MOB-01 | 14-02, 10, 11, 12, 13, 14 | Key bar, width fit, keyboard inset, pinch | SATISFIED | SC1 row |
| MOB-02 | 14-03, 05, 06, 07, 12, 13, 14 | Home Screen manifest and icon, full screen, host-served | SATISFIED | SC2 row |
| MOB-03 | 14-04, 08, 12, 13, 14 | Reconnect after sleep or network change on a real iPhone | SATISFIED | SC3 row |
| MOB-04 | 14-09, 12, 14 | Launcher copy and link format, no token/WebSocket/shell | SATISFIED | SC4 row |

All five IDs in the phase's plans map to REQUIREMENTS.md; REQUIREMENTS.md maps no other ID to Phase 14 (no orphans). The traceability table there still reads "Pending" for MOB-01..05; flipping it is the phase-completion step, not a gap.

### Post-run review fixes: do they need new device evidence?

WR-01, WR-02, WR-03 and IN-02 were committed (21:14-21:16) after every VM, simulator and real-phone run. Verdict: **the tests are enough; no new device run is required.**

- **WR-02** (an armed Ctrl not consumed by terminal replies): input-transform logic with no platform dependence. A unit test covers DA1, DSR and focus reports, and the iPhone-emulation Ctrl tests still pass.
- **WR-03** (page zoom limited to the terminal): for touches on the terminal, the effective `touch-action` is `pan-y` both before and after the fix, because the container's own rule already excluded pinch-zoom. The non-passive two-touch `touchmove` handler is unchanged. `gesturestart` moved to the container, where gestures on the terminal still bubble through. So S-03 and PHONE-1 still describe the terminal's behaviour, and outside the terminal page zoom now works as the fix intended. A computed-style Playwright test and unit tests pin this.
- **WR-01 / IN-02** (heal baseline and cleanup): the stuck-viewport heal was never seen to fire on a device before or after the fix. The fix narrows it so a landscape rotation no longer sets it off, and unit tests pin both the rotation and the cleanup. Residual risk, not a gap: the heal stays an [ASSUMED] safeguard for an iOS standalone bug. If iOS's stuck state ever shrinks `innerHeight` along with `visualViewport`, the heal would not fire, and the fix for the user is a Reload. Recommended, not required: next time you use the phone, close the keyboard once in the Home Screen app and check that the key bar returns to the bottom.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX` and no `TODO`/`HACK`/`PLACEHOLDER` lines were added across the 90 non-planning files changed since `64cf4e69` (the base before Phase 14). The two known shortcuts are commented on purpose (WR-02's "typed arrow or Alt while Ctrl is armed does not release it"; S-02's iOS form-assist bar).

### Human Verification Required

None outstanding. The real-phone items, PHONE-1 (M-145) and PHONE-2 (M-148/MOB-03), passed on the user's iPhone on 2026-11-22. UI-A, UI-B (kept uncached), UI-C and S-02 are limits the user accepted in their own words.

### Gaps Summary

No gaps. Every roadmap success criterion has evidence in code and passing tests, and the iOS-only behaviours (Home Screen launch, keyboard inset, sleep and network-switch reconnect) are backed by simulator rows and the user's real-iPhone checks. The known flakes (G-1, linkifier) are tracked and did not fail in this run.

---

_Verified: 2026-11-22_
_Verifier: Claude (gsd-verifier)_
