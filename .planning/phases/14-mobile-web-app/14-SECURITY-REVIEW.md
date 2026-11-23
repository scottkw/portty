# Phase 14 Security Review: Mobile Web App

## Scope and parts

Scope: what Phase 14 (MOB-01..MOB-05) changes at the security boundary. A share link opened on an iPhone is now a first-class client: the host Mac serves a phone-sized view with a key bar, a Home Screen manifest and icon, and every session-bearing request is checked against the host's own Tailscale account. Two changes are security-relevant in themselves and need a decision from the user: the same-account gate (new) and one added CSP directive, `manifest-src 'self'` (an amendment to the share-page policy).

This file is written in two parts:

- **14-14 Task 1 (this part, written before the user's pause):** the gate contract, the check order, why it holds, the discovery probe and its accepted cost, the CSP delta put up for sign-off, the Home Screen token storage, the lone-Esc change, the fakes and their release guards, the launcher's static scope, the verdict-cache decision basis, the Open PDF return control, the early Home Screen spike, the simulator and real-phone fidelity caveat, the threat register and the tests that pin each point.
- **14-14 Task 3 (appended after the user answers):** `## Evidence and verdict`, with the VM, emulation and simulator rows, the user's results for the real phone, their answers on the accepted limits and the CSP delta, and the verdict.

No share link, capability, token, tailnet address or tailnet host appears in this file. Hosts are written as `<host>`. Where a page's query parameters matter they are named in words (the `session` and `cap` query parameters of the share link).

## What Phase 14 changes at the boundary

- **On the host Mac (the one sharing):** a same-account check (`requireSameUser`, WhoIs by numeric user ID, fail closed) now wraps every route that carries session data, and a page variant (`requireSameUserPage`) wraps the two browser-facing routes. Another account gets a fixed static refusal page; a tailscaled fault gets a fixed static retry page. Neither names an account or a host.
- **On the share page:** a static manifest and three icon sizes are served with the SPA from `/app/`, and the CSP gains `manifest-src 'self'`. The page also gains the key bar, the visual-viewport sizing, the pinch font size and Reload controls. None of these adds a route or a network call.
- **In the input path:** `InputAbsorber.Filter` now forwards a lone ESC at the end of a frame instead of holding it (a deliberate change of Phase 9 behaviour).
- **In the repository:** three fakes that exist only for tests and VM checks (the WhoIs fake package, the `vmwhois` build tag and the `playwrightfixture` build tag) with guards that stop them reaching a release.
- **Not changed:** the tailnet-only listener, the capability token and its HMAC, the origin allow-list on the WebSocket, the other CSP directives, `portty.app` (a static launcher page that holds no link and opens no connection).

## The gate contract

The host serves session data to a caller only when the caller's tailnet node is logged in as the same Tailscale user as the host. The check is the numeric user ID, not a login name. Tagged nodes, nodes shared in from another tailnet, a tagged or shared-in host, and any missing data are refused (`sameUser` in `internal/webserver/whois.go`).

| Route | Wrapper order, outermost first | Refusal (other account, no peer, tagged, shared-in) | Refusal (host's tailscaled cannot answer) | Pinned by |
|-------|--------------------------------|-----------------------------------------------------|-------------------------------------------|-----------|
| `GET /app/` (the SPA, manifest, icons, hashed assets under `/app/assets/`) | `cspHeaders`, `requireSameUserPage` | 403 with `X-Portty: 1`, the static refusal page `web/user-refused.html` | 503, no marker, the static retry page `web/service-retry.html` | `TestSameUserGate_AppPage`, `TestSameUserGate_PagesNameNoAccount`, `frontend/e2e/web-same-user.spec.ts`; VM P-02, P-03 |
| `GET /sessions/{id}` (link page) | `cspHeaders`, `requireSameUserPage`, `requireCapabilityPage` | same refusal page | same retry page | `TestSameUserGate_LinkPage`, `TestSameUserGate_PagesNameNoAccount` |
| `GET /api/sessions/{id}/info` | `requireSameUser`, `requireCapability` | 403 with `X-Portty: 1`, plain body | 503, no marker | `TestSameUserGate_SessionInfo` |
| `GET /api/plugin-config`, `GET /api/plugin-config/stream` | `requireSameUser`, `requireCapability` | 403 with marker | 503, no marker | `TestSameUserGate_APIRoutes` |
| `GET /sessions/{id}/ws` (WebSocket) | `requireAllowedOrigin`, `requireSameUser`, `requireCapability` | 403 with marker, no upgrade, no subscriber joins | 503, no marker | `TestSameUserGate_WebSocket` (order rows), `TestSecurity_OriginCheckShortCircuitsBeforeCapability` |
| `GET /api/sessions` (the discovery probe) | `requireSameUser`, `porttyMarker`, `requireCapability` | 403 with marker | 503, no marker | `TestSameUserGate_DiscoveryProbe` |
| `GET /api/sessions/meta`, `POST /api/peer/sessions/{id}/cap` | `requireSameUser` (unchanged since Phase 12 and 13) | 403 with marker | 503, no marker | `TestWhoIs_MetaRefused`, `TestWhoIs_LocalFaultIs503`, `TestPeerCap_Routes` |
| `POST /api/peer/sessions/{id}/ports`, `.../ports/{port}` | `requireSameUser` then the write capability (Phase 13, unchanged) | 403 with marker | 503, no marker | Phase 13 review |
| `GET /dashboard`, `GET /`, `GET /assets/`, `GET /assets/xterm/` | none (open on purpose: static, no session data; the refusal pages load `/assets/dashboard.css`) | n/a | n/a | `TestSameUserGate_OpenRoutes` |

The marker (`X-Portty: 1`) is on 403 refusals only. A 503 from a local fault never carries it, so a viewer reads it as "unreachable", never as "another account" (WR-08).

## Check order

The order is fixed and no check moves after a piece of capability work.

1. **WebSocket:** Origin allow-list, then the same-account check, then the capability (signature, session, grant, session enabled), then `Accept`. A cross-site Origin is refused before any WhoIs call; another account is refused before any HMAC work; a refused caller never subscribes. Pinned by the order rows of `TestSameUserGate_WebSocket`.
2. **Pages (`/app/`, `/sessions/{id}`):** CSP headers first (so even the refusal page carries the policy), then the same-account check (the refusal or retry page), then, for the link page, the capability page check ("Link No Longer Works").
3. **API routes (info, plugin config, probe):** the same-account check first, then the capability. The unauthenticated 401 a same-account peer's discovery gets is still marked, because `porttyMarker` sits outside the capability check; the 403 for another account and the bare 503 come from the outer gate.
4. **Inside the check:** the host asks its own tailscaled for `StatusWithoutPeers` (its own user) and `WhoIs` of the request's source address, under a 3 s timeout. Any error other than "peer not found" is a 503 and never calls the next handler; "peer not found" and a user mismatch are a 403.

## Why the gate holds

1. **A leaked link is useless off the host's account.** The link still needs a valid capability, but the same-account check runs first, so a link forwarded to another person's device gets the refusal page and no session data, no WebSocket and no `/info` request. Pinned by `TestSameUserGate_SessionInfo`, `_WebSocket`, `_AppPage`, `_LinkPage` and, on real machines, VM row P-02 (another-account refusal on every gated route, byte-identical refusal page, no join in the host's log).
2. **It fails closed.** Every status or WhoIs error that is not "peer not found" gives 503 and never calls `next`. StatusFault and WhoIsFault rows exist in every gate test; VM row P-03 shows the real fault page and the 503 on `/app/` and the probe.
3. **Identity is the numeric user ID.** Login names are display-only and are not compared. Tagged, shared-in and alt-sharer nodes are refused.
4. **The pages say nothing.** The refusal and retry pages are static embedded bytes; nothing from the request is rendered; no account or host is named. Pinned by `TestSameUserGate_PagesNameNoAccount`; in P-02 and P-03 the served body's sha256 equals the file's.
5. **The gate cannot be reached around the SPA.** The whole `/app/` bundle (index, chunks, manifest, icons) is behind the page gate, so another account cannot even read the manifest. The open `/assets/` and `/dashboard` carry no session data.
6. **The source address is trusted as tailscaled trusts it.** A WireGuard-authenticated tailnet address. This is the same trust the Phase 12 and 13 peer routes already place in tailscaled (T-14-02, accepted).
7. **This Mac's own node passes.** The host resolves its own tailnet address to itself with the same user (VM row P-04, `is-self-node` true, same user true), so the host's own browser still opens its own link (P-01, P-02 office Safari).

## The discovery probe

User decision, 2026-11-22 ("Gate it"): `GET /api/sessions` returns the session name, status and hostname, so it sits behind the same-account check.

- **A same-account Mac still finds the host.** Its unauthenticated probe gets 401 with the marker, which the viewing Mac's discovery accepts as "this is Portty". Pinned by the `same-no-cap` row of `TestSameUserGate_DiscoveryProbe` (`tailnet.IsPorttyProbeResponse` is true); real machines: VM row P-01 (401 with `X-Portty: 1`, the home sidebar lists the office's session).
- **Another account gets 403 with the marker,** which the probe does not count as a Portty peer, so another account's Mac cannot list the host's sessions. Rows `other`, `tagged`, `shared-in`, `not-found`; VM row P-02 (the home sidebar had no office group at all).
- **Accepted cost (T-14-43).** While the host's own tailscaled cannot answer, the probe gets a 503, which the probe rejects, so peers drop the host's group until their next refresh (5 to 30 s). Measured on real machines in P-03: with the fault mode on, the home sidebar dropped the office group between about 21 and 26 s later, and after the fault ended the group returned 31 s after the mode was cleared (26 s the first time). No session is lost and no link stops working; the sidebar entry is briefly absent. This is the only availability cost of the gate that the user will notice, and it needs the host's tailscaled to be failing at that moment.

## CSP delta and sign-off

**Approval requested at the checkpoint (Task 2). Nothing here changes the policy any further.**

The share-page policy before Phase 14 (Amendment 3, Phase 11 PREV-04):

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

The policy now (Amendment 4, Phase 14 MOB-02):

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; manifest-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

The delta is exactly one directive: `manifest-src 'self'`.

- **Why it is needed.** Add to Home Screen reads `/app/manifest.json`. Chromium blocks the manifest fetch under `default-src 'none'` (reproduced in 14-RESEARCH.md, Q2), and WebKit also checks manifests against the page CSP. Without the directive the manifest is refused and the icon and standalone mode are lost.
- **Why it is narrow.** `'self'` names no network origin. The manifest is a static same-origin file with no `start_url` and no `id`, so the launch URL stays the exact share link (WebKit rejects a cross-origin `start_url` or `scope` anyway). The manifest's icons load under the existing `img-src 'self'`. `default-src` stays `'none'`; no other directive changes; `'unsafe-eval'` stays absent.
- **What stays pinned.** `TestCSPHeaders_ManifestDelta` pins the exact policy string. `TestCSPHeaders_NoUnsafeEvalToken_TokenAware` keeps the `unsafe-eval` guard. `TestCSPHeaders_PreviewDelta` compares the policy minus the new directive so Amendment 3 stays pinned on its own. `TestCSPHeaderStrict_App` checks the directive on `/app/`. In a real Chromium, `frontend/e2e/web-csp.spec.ts` reads `Page.getAppManifest` under the live policy with no errors and no CSP console message (the e2e fails if the directive is removed, mutation-checked in 14-05).
- **What it does not allow.** No remote manifest, no remote script, image, style, font, frame or connection. A page that could inject a manifest link could only point it at this same origin.
- **Not covered by a test in this phase:** that WebKit on iOS accepts the manifest under the policy. The iOS evidence is the simulator's Add to Home Screen flow (S-04) and the real-phone look, in part 2.
- **The decision requested:** approve Amendment 4 as above, or refuse it (in which case Home Screen install is dropped and MOB-02 stays open).

## Home Screen token storage

T-14-04, accepted residual. Add to Home Screen stores the page URL, with its `session` and `cap` query parameters, in the Home Screen icon (the design is locked: no `start_url`, the token stays in the page URL).

- **What limits it.** The same-account gate makes a leaked copy useless off the host's account. The icon stops working when sharing is turned off (observed in the spike and in S-07; the launcher copy also names a regenerated key and a daemon restart, which were not separately exercised); it then shows "Link No Longer Works" and the user adds a new one from the new link. The launcher copy tells the user to treat the link like a password and not to send it in Messages (it syncs to iCloud).
- **The residual.** A same-account device that syncs the bookmark or the Home Screen could open it. That device is, by definition, on the host's Tailscale account.
- **Observed.** In the early spike (H-01) and the full simulator run (S-04) the launch kept both query parameters; after sharing stopped and started again (a new token) the old icon showed "Link No Longer Works" (spike extra check, S-07).
- **A real phone can differ.** Where the icon lands and whether the query survives on current iOS is the real-phone look (PHONE-1).

## Lone Esc

T-14-09, accepted cost, deliberate behaviour change. `InputAbsorber.Filter` (`internal/relay/oscabsorb.go`) used to hold a bare ESC at the end of a frame until the next frame, so a lone Esc (the key bar's Esc, "Esc to interrupt", leaving vim's Insert mode) did nothing until the next key. It now forwards that ESC at once.

- **Why it is safe to flush.** xterm.js sends every key and every reply to a terminal query in one `onData` call, so a frame never ends between the ESC and the introducer of a reply (only the 32767-byte input chunking splits a frame, and only pastes are that large). Absorption of colour and device replies split after the introducer is unchanged (the OSC and CSI states persist across frames).
- **The pinned cost.** A raw terminal behind `portty attach` that splits a reply exactly between ESC and `]` (or `[`) now leaks the rest of the reply to the PTY as text. This is a cosmetic fault in a program the user chose to run; it is not a security boundary and the leaked bytes were already the user's own terminal's replies. Pinned and named by `TestInputAbsorber_EscBracketSplitLeaks`.
- **What stays.** A lone Esc does not claim the grid (`IsUserInput` is false for it), so INPT-06 stays conservative. Pinned by `TestInputAbsorber_LoneEscEndsFrame`, `TestInputAbsorber_P9_BareEscThenColon` (rewritten on purpose), S1 to S5, and the relay and share-server end-to-end rows `TestRelay_handleSession_LoneEscReachesPTY`, `TestRelay_LoneEscReachesPTY`, `TestRelay_KeystrokesStillForwarded`, `TestRelay_OSC11SplitAcrossTwoFrames`.
- **Real machines.** The before rows E-00 and E-01 reproduced the hold in a local tab and a peer tab; the after row P-05 and M-150 show it gone in a local tab, a peer tab and the browser share view. These are in part 2.

## Fakes never ship

T-14-11, T-14-22, T-14-23. Three constructs exist only so tests and the VM checks can answer "which account is this?" without a second Tailscale login.

- **The WhoIs fake package** (`internal/webserver/webservertest`). It must not compile into any release. `TestNoFakeWhoIsInRelease` runs `go list -deps` over three release-like graphs (no tags, `wailsassets`, `vmwhois`) and asserts the fake package is absent, with a positive control that the Playwright fixture's graph does contain it (so a broken search cannot pass vacuously; `TestGoListDeps_ReportsFailure` pins that a failing `go list` is an error, never "absent"). The release guard is in `release_guard_test.go`.
- **The `check-brand.sh` import guard** (the `fake-whois` line). It fails when any non-test Go file outside the Playwright fixture and the package itself imports `webservertest`. The check-brand gate ran clean in 14-12.
- **The `vmwhois` build tag** (`//go:build vmwhois`). It rewrites the real WhoIs answer so one VM looks like another account. `check-brand.sh` (the `vm-tag` line) fails when the tag word appears in `.github`, `build.sh` or `scripts`, tracked or untracked; the mode wrapper fails closed on unknown mode content (`TestVMWhoIs_Modes`); the release graph is checked with the tag absent. The vmwhois build (`W14`) was installed only on the office VM for P-02 and P-03 and was removed by the 14-12 cleanup.
- **The `playwrightfixture` build tag.** The fixture's admin listener is bound to `127.0.0.1`; the CA it prints is a throwaway per-run certificate whose key is never written. It installs the WhoIs fake before it starts, so a test binary or fixture can never ask the host's live tailscaled (T-14-14, `TestLiveTailnetGuard_Webserver` pins the default).

## The launcher's static scope

T-14-21. `portty.app/app` is a static page rendered by the website repo from `docs/launcher-app.md`. It explains the steps (copy the link from the Share popover, move it to the phone, Add to Home Screen) and warns that the icon stops working when sharing stops and that Messages keeps a copy in iCloud.

- **It holds no link and opens no connection.** The page has no input, form, script, token or WebSocket and no `?session=` link form; `TestLauncherCopy` fails if any of `<input`, `<form`, `<script`, `wss://` or a query form appears, and fails if any required instruction is missing.
- **Nothing trusts it.** No capability, CSP or webserver code names `portty.app` (checked by the 14-09 verify scan). The iPhone talks straight to the host Mac.
- **The copy matches the product.** The warning sentence is the same one the CLI and the Share popover print (pinned in `cmd_share_test.go`, `SharePopover.test.tsx` and `launcher_copy_test.go`); VM row P-07 checked the launcher's step names and link form against the real Share popover. AirDrop and Universal Clipboard cannot run in the VMs, so step 2 of the launcher is checked on the real phone (PHONE-1).

## Verdict cache (not built)

T-14-07, accepted. Every gated request makes a `StatusWithoutPeers` and a `WhoIs` call to the host's own tailscaled. A positive-only per-IP verdict cache with a short TTL was considered and not built, because a cache adds a stale-allow window (a node removed from the account would pass until the TTL ends) that the security review would have to approve.

- **The rule fixed beforehand:** "a positive-only verdict cache is worth discussing when the estimate exceeds 300 ms per cold load or either p95 exceeds 50 ms".
- **The numbers (VM row P-04, 200 calls each, HEAD, real tailnet):** against a peer, status p50 4.43 ms, p95 6.88 ms; WhoIs p50 3.90 ms, p95 4.95 ms. A cold `/app/` load makes 7 gated requests, so the estimate is 7 x (6.88 + 4.95) = 82.8 ms. The gate's own share, measured by curl through the real relay path, was about 20 ms per request against an open route. Neither condition of the rule holds.
- **The decision is the user's (UI-B):** keep uncached, or add a cache in a gap plan with the TTL signed off in this review.

## Open PDF in a Home Screen app

T-14-13 and T-14-41. In a Safari tab, Open PDF opens the PDF in a new tab and Safari's own chrome gives the way back. In a Home Screen web app there is no browser chrome, so a top-level navigation to the PDF would strand the user with no way back and no way to close it.

- **What was built (14-08).** In standalone mode Open PDF opens an in-app layer with a visible Done control and Esc, no top-level navigation; the full document stays reachable through "Open full PDF". Pinned by `PreviewRenderers.test.tsx` (both paths).
- **Why this is a security review item.** It is a denial-of-service-by-UX risk on a shell the user may only be able to reach from the phone ("reliability first": the user may not be able to reach the host Mac to fix a stuck view). Reload (T-14-42) is user-initiated only, never automatic, and keeps the same token URL.
- **The iOS evidence is S-06 (14-13),** recorded in part 2: Done visible below the status bar and returning without a relaunch.

## The early Home Screen spike

T-14-40. The Home Screen design is locked on one assumption: that iOS keeps the share link's query string when a Home Screen icon is added from it. If iOS dropped it, 14-09, 14-10 and 14-11 would have been built on a launch URL that does not work. Plan 14-07 therefore ran the question first, in a headless iOS 26.5 simulator against a fixture (H-00, H-01), with a blocking checkpoint offering a redesign route; the user's decision was "continue" (recorded in 14-SIM-CHECKS.md). The full run repeated it on the finished UI (S-04). The real phone repeats it once more (PHONE-1). If S-04 or PHONE-1 shows the query dropped, the checkpoint's redesign option sends the Home Screen design back to discussion; MOB-02 stays open.

## Simulator and real-phone fidelity

What the simulator rows prove, and what they cannot.

- **Real in the simulator:** iOS 26.5's WebKit and SpringBoard, Safari's Add to Home Screen flow, the standalone web-app process (`com.apple.webapp`), the software keyboard and Safari's chrome, safe areas and light and dark appearance, XCUITest-driven taps, pinches and measurements.
- **Not real in the simulator:** real suspension and resume, cellular and Wi-Fi roaming, the Tailscale app's VPN and its reconnection, real thumb and finger pinches (the simulator's pinch-in with whole-view fingers opened Safari's tab overview), and where a real phone puts a new Home Screen icon (the simulator put it only in the App Library).
- **Known WebKit bugs that only a real phone shows.**
  - **WebKit bug 308073** (filed 2026-02-17, iOS 26.3, open): after a background resume the old WebSocket claims OPEN and then closes 1006, and every new `WebSocket` stays CONNECTING while `fetch` works; only a page reload fixes it, in a Safari tab and in a Home Screen app. The mitigations built are a 1 s timer-gap wake detector, `pageshow`, a stuck-connect escalation that reports and never changes backoff (T-14-18), and a user-initiated Reload (T-14-42). They are pinned by unit tests; the real-phone sleep and network-change check (PHONE-2, MOB-03) is the only evidence that they work on iOS.
  - **WebKit bugs 218983 and 254861** (standalone visualViewport): `visualViewport.height` can be wrong in a Home Screen app and the viewport can stay shrunk after the keyboard closes. The keyboard inset and the "heal" toggle in `visualViewportInset.ts` are marked [ASSUMED] in the code until a device shows them; the simulator measured the key bar's position above the keyboard (S-02, S-05, results in part 2).
- **Playwright's WebKit is not iOS Safari.** The I-rows (an iPhone 15 device descriptor on WebKit, plus desktop Chromium, Firefox and WebKit) prove layout, byte mapping and the gate's page behaviour; they cannot show the keyboard, touch hardware or Safari chrome.
- **Consequence for the verdict.** The simulator rows are the evidence for the key bar, the keyboard inset, pinch and safe areas; MOB-03's reconnect is evidenced only by the user's own real-phone result.

## Accepted risks

Each with its source.

- **The Home Screen icon stores the share URL (T-14-04).** See above. Source: 14-CONTEXT (Home Screen locked design), 14-RESEARCH Security Domain.
- **While the host's tailscaled is failing, peers drop its sidebar group for 5 to 30 s plus the refresh (T-14-43).** Source: user decision 2026-11-22; measured in P-03.
- **No verdict cache (T-14-07).** Each gated request costs one status and one WhoIs call on the host's tailscaled; measured at 82.8 ms per cold load.
- **The source address is trusted as tailscaled trusts it (T-14-02).**
- **A raw terminal behind `portty attach` that splits a reply between ESC and `]` leaks text to the shell (T-14-09).** Not a boundary.
- **A same-account holder of a write link controls the shell, as before.** The key bar adds a way to type; it is hidden and inert without write permission (T-14-10), and the server still discards input from read-only subscribers.
- **"Another account" is shown on real machines only through the `vmwhois` build,** which rewrites the real WhoIs answer; tagged and shared-in nodes are pinned only by tests (UI-A, decided at the checkpoint).

## What stays unchanged

- **The tailnet-only listener.** No new listener on the host; no relay, cloud service or `portty.app` dependency.
- **The capability token and its HMAC,** the origin allow-list on the WebSocket, `Referrer-Policy: no-referrer` and `no-store` from `cspHeaders`.
- **Every CSP directive except `manifest-src`.** `default-src 'none'` and the absence of `'unsafe-eval'` are pinned.
- **The desktop CSP and desktop behaviour.** The desktop shows no key bar and no pinch font change (`WebShareSessionView.font.test.tsx`, and the desktop no-key-bar assertions in `e2e/web-share-scope.spec.ts`); VM row P-06 checks the host GUI launches with no manifest or CSP error.
- **Phase 13's port routes** keep their own gate and tests.

## Threat register

Rows from the threat models of 14-01 to 14-14. T-14-SC (package installs) is in every plan: no package was installed in any of them (`pnpm install --frozen-lockfile` and `playwright install` of the lockfile-pinned browser only), so the package-legitimacy gate never triggered. T-14-37 and T-14-38 belong to this plan; their results are in part 2.

| ID | Threat | Severity | Disposition | Pinned by |
|----|--------|----------|-------------|-----------|
| T-14-01 | Spoofing / Elevation: another account's device presents a valid link on the probe, info, plugin-config, page and WebSocket routes | high | mitigate | `TestSameUserGate_SessionInfo`, `TestSameUserGate_APIRoutes`, `TestSameUserGate_DiscoveryProbe`, `TestSameUserGate_WebSocket`, `TestSameUser`; VM P-01, P-02 |
| T-14-02 | Spoofing: source address used for WhoIs | low | accept | none (accepted; the same trust in tailscaled as the peer routes) |
| T-14-03 | Elevation: a tailscaled fault turning the gate open | high | mitigate | StatusFault and WhoIsFault rows in every gate test; `TestWhoIs_LocalFaultIs503`; VM P-03 |
| T-14-04 | Information disclosure: the token in the Home Screen URL | medium | accept | gate tests above; `TestLauncherCopy` (the warning); S-04 and S-07 (icon dies with the share) |
| T-14-05 | Tampering: share CSP widening | medium | mitigate | `TestCSPHeaders_ManifestDelta`, `TestCSPHeaders_NoUnsafeEvalToken_TokenAware`, `TestCSPHeaders_PreviewDelta`, `TestCSPHeaderStrict_App`, `frontend/e2e/web-csp.spec.ts`; sign-off in this file |
| T-14-06 | Information disclosure: refusal and retry pages | medium | mitigate | `TestSameUserGate_PagesNameNoAccount`; P-02 and P-03 body sha256 equals the embedded file |
| T-14-07 | Denial of service / Elevation: per-request WhoIs cost; a verdict cache's stale allow | medium | accept | no cache built; VM P-04 numbers against the rule; decision UI-B |
| T-14-08 | Spoofing: WebSocket check order | high | mitigate | `TestSameUserGate_WebSocket` order rows; `TestSecurity_OriginCheckShortCircuitsBeforeCapability` |
| T-14-09 | Tampering: lone-Esc flush in `InputAbsorber.Filter` | low | mitigate | `TestInputAbsorber_EscBracketSplitLeaks`, `TestInputAbsorber_LoneEscEndsFrame`, `TestInputAbsorber_P9_BareEscThenColon`, `TestInputAbsorber_S1_SplitAfterIntroducer`; VM E-00, E-01, P-05 |
| T-14-10 | Elevation: the key bar for a capability without write | medium | mitigate | `frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx` (the bar is not rendered without write); the server's read-only input discard is existing behaviour, not a Phase 14 test |
| T-14-11 | Elevation: the WhoIs fake compiled into a release | high | mitigate | `TestNoFakeWhoIsInRelease`, `TestGoListDeps_ReportsFailure`; `tests/check-brand.sh` (fake-whois) |
| T-14-12 | Tampering: a stale manifest or icon from a cache | low | mitigate | `TestAppStatic_CacheHeaders`, `TestAppBundle_HashedAsset_CacheableNotNoStore` |
| T-14-13 | Denial of service (UX): Open PDF in a standalone web app | medium | mitigate | `frontend/src/components/__tests__/PreviewRenderers.test.tsx`; S-06 |
| T-14-14 | Elevation / Information disclosure: fixture and test binaries asking the host's live tailscaled | high | mitigate | `TestLiveTailnetGuard_Webserver`; the fixture installs the fake before it starts; `TestNoFakeWhoIsInRelease` (positive control) |
| T-14-15 | Denial of service / Information disclosure: the 14-02 VM before rows | high | mitigate | the 14-02 cleanup record in 14-VM-CHECKS.md (no network drop, settings and dotfiles compared) |
| T-14-16 | Spoofing / Elevation: `/app/` and `/sessions/{id}` | high | mitigate | `TestSameUserGate_AppPage`, `TestSameUserGate_LinkPage`, `frontend/e2e/web-same-user.spec.ts` |
| T-14-17 | Spoofing / Repudiation: the SPA's marker branch | medium | mitigate | `frontend/src/components/__tests__/App.webLink.test.tsx`, `frontend/src/lib/__tests__/closeReasons.test.ts` |
| T-14-18 | Denial of service: the stuck-connect escalation | low | mitigate | `frontend/src/lib/relayClient.test.ts` |
| T-14-19 | Tampering: the client-only close reason | low | mitigate | `frontend/src/lib/__tests__/closeReasons.test.ts` |
| T-14-20 | Information disclosure (user error): share copy | medium | mitigate | `frontend/src/components/__tests__/SharePopover.test.tsx`, `TestLauncherCopy`, the CLI's `cmd_share_test.go`; VM P-07 |
| T-14-21 | Spoofing / Information disclosure: the portty.app launcher page | medium | mitigate | `TestLauncherCopy` |
| T-14-22 | Elevation: the `vmwhois` build reaching a release | high | mitigate | `tests/check-brand.sh` (vm-tag), `TestVMWhoIs_Modes`, `TestNoFakeWhoIsInRelease` (the `vmwhois` graph); the 14-12 cleanup (W14 installed nowhere) |
| T-14-23 | Elevation / Information disclosure: fixture admin routes and CA output | low | mitigate | the `playwrightfixture` tag and loopback bind; `TestNoFakeWhoIsInRelease` (positive control) |
| T-14-24 | Tampering: Ctrl and arrow mapping | low | mitigate | `frontend/src/lib/__tests__/ctrlMap.test.ts` |
| T-14-25 | Denial of service (UX): double firing, or a bar on the desktop | low | mitigate | `frontend/src/components/__tests__/KeyBar.test.tsx`, `frontend/src/components/__tests__/style.keybar.test.ts`, `frontend/e2e/web-share-scope.spec.ts` |
| T-14-26 | Denial of service: a pinch claiming the grid repeatedly | medium | mitigate | `frontend/src/components/__tests__/TerminalPanel.pinch.test.tsx`, `frontend/src/lib/__tests__/pinchZoom.test.ts`; S-03 |
| T-14-27 | Denial of service (UX): viewport sizing hiding the cursor line or the bar | low | mitigate | `frontend/src/lib/__tests__/visualViewportInset.test.ts`, `frontend/e2e/mobile-iphone.spec.ts`; S-02 and S-05 (see part 2) |
| T-14-28 | Tampering: the host machine (14-12) | high | mitigate | the 14-12 Cleanup record |
| T-14-29 | Information disclosure: 14-VM-CHECKS.md and TESTING.md (14-12) | high | mitigate | the 14-12 verify greps for link, token, tailnet URL and address patterns |
| T-14-30 | Denial of service: ephemeral VM tailnet nodes | high | mitigate | the 14-12 Cleanup record (VMs never stopped, no `tailscale down`) |
| T-14-31 | Tampering: VM settings and dotfiles after the run | medium | mitigate | the 14-12 Cleanup record (`cmp` clean, sha256 equal to P-00) |
| T-14-32 | Tampering: temporary worktree and helper source left behind | medium | mitigate | the 14-12 Cleanup record (0 helper worktrees) |
| T-14-33 | Elevation: the vmwhois build left installed; the phone-check share left running | medium | mitigate | the 14-12 Cleanup record; the phone-check share is ended by 14-14 Task 3 |
| T-14-34 | Tampering: host GUI and preferences (14-13) | medium | mitigate | the 14-SIM-CHECKS.md Cleanup (headless boot only, `pgrep -x Simulator` empty at start and end) |
| T-14-35 | Elevation: the fixture CA trusted beyond the run | medium | mitigate | the 14-SIM-CHECKS.md Cleanup (throwaway device deleted) |
| T-14-36 | Tampering / Information disclosure: runner project, link files, screenshots | low | mitigate | the 14-SIM-CHECKS.md Cleanup and Screenshots (sha256 list; screenshots deleted by 14-14) |
| T-14-37 | Information disclosure / Repudiation: this review, TESTING.md and the decision record | high | mitigate | the Task 1 and Task 3 greps for link, token, tailnet URL and address patterns; accepted limits only in the user's own words; result in part 2 |
| T-14-38 | Elevation: the phone-check share and its link file left behind | medium | mitigate | 14-14 Task 3 ends the share and deletes the link file and the screenshot directories in every outcome; result in part 2 |
| T-14-39 | Tampering / Elevation: host GUI, preferences and trust store (14-07) | medium | mitigate | the 14-SIM-CHECKS.md Spike cleanup |
| T-14-40 | Repudiation (design risk): building on a launch URL that iOS drops | medium | mitigate | H-01 and the spike decision; S-04; PHONE-1 |
| T-14-41 | Denial of service (UX): the standalone PDF layer on iOS WebKit | medium | mitigate | S-06 (part 2) |
| T-14-42 | Denial of service: Reload | low | mitigate | `frontend/src/components/__tests__/SessionEndedBanner.test.tsx`, `frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx`; S-07, S-08 |
| T-14-43 | Denial of service: the gated discovery probe during a host tailscaled blip | low | accept | `TestSameUserGate_DiscoveryProbe`; VM P-03 timings |
| T-14-SC | Tampering: package installs | high | mitigate | no package installed in any Phase 14 plan |

## Tests that pin it

- Same-account gate (host): `TestSameUser`, `TestSameUserGate_SessionInfo`, `TestSameUserGate_APIRoutes`, `TestSameUserGate_WebSocket`, `TestSameUserGate_DiscoveryProbe`, `TestSameUserGate_OpenRoutes`, `TestSameUserGate_AppPage`, `TestSameUserGate_LinkPage`, `TestSameUserGate_PagesNameNoAccount` (`internal/webserver/whois_gate_test.go`, `internal/webserver/whois_test.go`), `TestWhoIs_MetaSameUser`, `TestWhoIs_MetaRefused`, `TestWhoIs_LocalFaultIs503`, `TestPeerCap_Routes`
- Origin before WhoIs: `TestSecurity_OriginCheckShortCircuitsBeforeCapability` (`internal/webserver/origin_integration_test.go`)
- Live tailnet guard: `TestLiveTailnetGuard_Webserver` (`internal/webserver/tailscale_test.go`)
- CSP and static serving: `TestCSPHeaders_ManifestDelta`, `TestCSPHeaders_NoUnsafeEvalToken_TokenAware`, `TestCSPHeaders_PreviewDelta` (`internal/webserver/csp_mw_test.go`), `TestCSPHeaderStrict_App`, `TestAppBundle_HashedAsset_CacheableNotNoStore` (`internal/webserver/csp_integration_test.go`), `TestAppStatic_CacheHeaders` (`internal/webserver/app_static_test.go`), `frontend/e2e/web-csp.spec.ts`
- Lone Esc: `TestInputAbsorber_LoneEscEndsFrame`, `TestInputAbsorber_EscBracketSplitLeaks`, `TestInputAbsorber_P9_BareEscThenColon`, `TestInputAbsorber_S1_SplitAfterIntroducer` (`internal/relay/oscabsorb_test.go`), `TestRelay_handleSession_LoneEscReachesPTY` (`internal/relay/oscabsorb_relay_test.go`), `TestRelay_LoneEscReachesPTY`, `TestRelay_KeystrokesStillForwarded`, `TestRelay_OSC11SplitAcrossTwoFrames` (`internal/webserver/oscabsorb_relay_test.go`)
- Fakes and release guards: `TestNoFakeWhoIsInRelease`, `TestGoListDeps_ReportsFailure` (`release_guard_test.go`), `TestVMWhoIs_Modes` (`internal/daemon/share_vmwhois_test.go`), `tests/check-brand.sh`
- Launcher and share copy: `TestLauncherCopy` (`launcher_copy_test.go`), `frontend/src/components/__tests__/SharePopover.test.tsx`
- Browser: `frontend/e2e/web-same-user.spec.ts`, `frontend/e2e/mobile-iphone.spec.ts`, `frontend/e2e/web-share-scope.spec.ts`
- Key bar, pinch, viewport, phone chrome: `frontend/src/components/__tests__/KeyBar.test.tsx`, `frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx`, `frontend/src/components/__tests__/WebShareSessionView.font.test.tsx`, `frontend/src/components/__tests__/style.keybar.test.ts`, `frontend/src/components/__tests__/style.mobile.test.ts`, `frontend/src/components/__tests__/TerminalPanel.pinch.test.tsx`, `frontend/src/lib/__tests__/ctrlMap.test.ts`, `frontend/src/lib/__tests__/pinchZoom.test.ts`, `frontend/src/lib/__tests__/phoneQuery.test.ts`, `frontend/src/lib/__tests__/visualViewportInset.test.ts`
- Marker branch, close reasons, reconnect, Reload, Open PDF: `frontend/src/components/__tests__/App.webLink.test.tsx`, `frontend/src/components/__tests__/UserRefused.test.tsx`, `frontend/src/lib/__tests__/closeReasons.test.ts`, `frontend/src/lib/relayClient.test.ts`, `frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx`, `frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx`, `frontend/src/components/__tests__/SessionEndedBanner.test.tsx`, `frontend/src/components/__tests__/PreviewRenderers.test.tsx`
- Home Screen head tags and manifest: `frontend/src/__tests__/homeScreen.test.ts`
- Not automated (VM, simulator and real-phone rows): E-00, E-01, P-00..P-07 (`14-VM-CHECKS.md`), H-00, H-01, S-00..S-08 (`14-SIM-CHECKS.md`), PHONE-1 and PHONE-2 (the checkpoint)
