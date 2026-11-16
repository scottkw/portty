---
phase: "03"
slug: "portty-brand"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-16"
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Tailnet peer → webserver `GET /api/sessions` | Peer discovery probe on port 7444 | Status code + `X-Portty: 1` header only; sessions stay behind the capability check |
| GitHub Releases API → updater | Update check against `scottkw/portty` | Release metadata (asset names, versions); no download or self-update |
| Go → Objective-C (cgo) | Tray icons, About panel, notifications | Copied byte buffers, constant strings, session counts |
| Local filesystem | Config, capability key, daemon socket | `~/Library/Application Support/portty` (0700) |
| CI tag → release workflow shell | Release build and DMG signing | Tag name via `$GITHUB_REF_NAME`, regex-checked, passed to jq with `--arg` |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | EoP | config/socket dirs | medium | mitigate | `os.MkdirAll(dir, 0700)` in app.go:808, engine.go:66, socket.go:20 | closed |
| T-03-02 | Info Disclosure | old app data | low | mitigate | No path to the old directory; no migration code | closed |
| T-03-03 | Spoofing | LaunchAgent label | low | accept | See AR-01 | closed |
| T-03-04 | EoP | X-Portty marker | high | mitigate | `porttyMarker(ws.requireCapability(...))` at server.go:828; marker grants nothing; probe_marker_test.go | closed |
| T-03-05 | Spoofing | peer marker forgery | low | accept | See AR-02 | closed |
| T-03-06 | Info Disclosure | marker scope | low | mitigate | One call site (server.go:828); test checks other routes lack it. Residual: visible via Funnel until Phase 4 removes Funnel | closed |
| T-03-07 | DoS | probe body read | low | mitigate | `isPorttyProbeResponse` reads header and status only (tailnet.go:95-100) | closed |
| T-03-08 | Spoofing | update feed | low | mitigate | Single `updateSlug`; anchored DMG filter (updater.go:36); no download/self-update; silent errors | closed |
| T-03-09 | Tampering | tray icon buffers | medium | mitigate | Buffers copied with `dataWithBytes:` before `dispatch_async` (tray_objc_darwin.m:131,170) | closed |
| T-03-10 | Tampering | tray art | low | accept | See AR-03 | closed |
| T-03-11 | Spoofing | bundle ID / entitlements | low | mitigate | Entitlements unchanged (network client/server only); bundle `app.portty.Portty` | closed |
| T-03-12 | DoS | About panel threading | medium | mitigate | Only `dispatch_async` to main queue (about_darwin.m:14) | closed |
| T-03-13 | Tampering | About link | low | accept | See AR-04 | closed |
| T-03-14 | Info Disclosure | quit notification | low | accept | See AR-05 | closed |
| T-03-15 | Tampering | web pages CSP | low | mitigate | `cspHeaders` on /dashboard and /join (server.go:796,801); brand_pages_test asserts CSP | closed |
| T-03-16 | Spoofing | local Basic auth | low | mitigate | Only realm string changed (auth.go:21); comparison unchanged; tests pass | closed |
| T-03-17 | Info Disclosure | localStorage key | low | accept | See AR-06 | closed |
| T-03-18 | Tampering | DMG build | medium | mitigate | `hdiutil create` + `codesign --timestamp` (release.yml:211-212); actions pinned to SHAs | closed |
| T-03-19 | Repudiation | release provenance | medium | mitigate | Build provenance attestation, `gh attestation verify`, sha256 checksums, release attestation (release.yml) | closed |
| T-03-20 | EoP | README `sudo ln` line | low | accept | See AR-07 | closed |
| T-03-21 | Spoofing | repo links | low | mitigate | check-brand.sh flags any non-portty `github.com/scottkw/` link; runs in CI. Residual: case-sensitive (IN-09) | closed |
| T-03-22 | Info Disclosure | RELEASE_PUBLISH_TOKEN | low | accept | See AR-08; superseded by WR-04 (workflow uses GITHUB_TOKEN) | closed |
| T-03-SC | Tampering | supply chain (x6 plans) | high | mitigate | go.sum and pnpm-lock unchanged; go.mod changes only the module line; no package-manager installs in workflows; verify-icons uses built-in tools | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-03-03 | LaunchAgent label `portty-daemon` (service.go:55) is per-user; another user-level agent could reuse the label but gains nothing beyond its own user's rights | orchestrator (plan disposition) | 2026-11-16 |
| AR-02 | T-03-05 | `X-Portty` is an identity hint, not authentication (tailnet.go:31-33); a forging peer is only listed, and attach still requires a capability | orchestrator (plan disposition) | 2026-11-16 |
| AR-03 | T-03-10 | Tray art is embedded at build time; sizes checked by TestTrayIconAsset and verify-icons.sh (27/27) | orchestrator (plan disposition) | 2026-11-16 |
| AR-04 | T-03-13 | About link is a compile-time constant (`https://portty.app`, about_darwin.m:32); no input reaches the panel | orchestrator (plan disposition) | 2026-11-16 |
| AR-05 | T-03-14 | Quit notification body carries only a session count (`quitToMenuBarBody`) | orchestrator (plan disposition) | 2026-11-16 |
| AR-06 | T-03-17 | `portty:lastWorkDir` stores a folder path in the app's own WebKit storage; no old key is read | orchestrator (plan disposition) | 2026-11-16 |
| AR-07 | T-03-20 | README's `sudo mkdir -p /usr/local/bin && sudo ln -sf …` uses fixed paths and takes no input; the user runs it knowingly | orchestrator (plan disposition) | 2026-11-16 |
| AR-08 | T-03-22 | Workflow no longer reads the secret (WR-04); the stale repo secret still needs manual deletion in GitHub settings | orchestrator (plan disposition) | 2026-11-16 |

---

## Notes

- **Unregistered path (no gap found):** the WR-02 "Stamp bundle version from tag" step (release.yml:83-88) reads `$GITHUB_REF_NAME`, regex-checks `^[0-9]+\.[0-9]+\.[0-9]+$` and passes it with `jq --arg`. IN-12 removed the last `${{ github.ref_name }}` from a `run:` script.
- **Carried to Phase 4:** the Funnel loopback listener serves the same mux, so the marker is internet-visible while Funnel is on (Phase 4 removes Funnel).
- `scripts/grep-gate.sh` is not run in CI; it passes today.
- `build/darwin/Info.dev.plist` uses the release bundle ID (IN-04, informational).

---

## Security Audit 2026-11-16

| Metric | Count |
|--------|-------|
| Threats found | 23 |
| Closed | 23 |
| Open | 0 |

Auditor verdict: SECURED at ASVS L1 (block_on: high). 15 mitigations verified with file:line evidence and passing tests (`go build`/`vet`, targeted `go test -race`, verify-icons, grep-gate, check-brand); 8 accepted risks logged above.
