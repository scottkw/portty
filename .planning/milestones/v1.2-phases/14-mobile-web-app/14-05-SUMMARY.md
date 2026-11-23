---
phase: 14-mobile-web-app
plan: 05
subsystem: web-share
tags: [csp, manifest, pwa, home-screen, ios, icon, playwright, vitest]
requires:
  - phase: 14-01
    provides: same-account gate (this plan only adds static assets and one CSP directive)
provides:
  - "/app/manifest.json (name and short_name Portty, display standalone, scope ./, no start_url, no id)"
  - "CSP Amendment 4: manifest-src 'self' (the only delta from Amendment 3)"
  - "Full-bleed opaque icon: build/appicon-fullbleed.png (1024) and 512, 192, 180 px exports"
  - "index.html head tags: manifest, apple-touch-icon, Apple web-app meta, light/dark theme-color, viewport-fit=cover"
affects: [14-07, 14-12, 14-13, 14-14]
status: complete
actuals:
  tokens: 10500
  tasks: 2
  commits: 2
plan_head_before: 1f43716e2d8416bd6b5cba6d2c27a30dc635af20
plan_head_after: ccd0e1b4ebb2e21bbc15c50dedb13afe0528db02
tech-stack:
  added: []
  patterns:
    - "public/ assets are referenced root-absolute in index.html source; Vite (base './') rewrites them to ./ in dist"
key-files:
  created:
    - frontend/public/manifest.json
    - frontend/public/apple-touch-icon.png
    - frontend/public/icon-192.png
    - frontend/public/icon-512.png
    - build/appicon-fullbleed.png
    - frontend/src/__tests__/homeScreen.test.ts
  modified:
    - frontend/index.html
    - internal/webserver/csp_mw.go
    - internal/webserver/csp_mw_test.go
    - internal/webserver/csp_integration_test.go
    - frontend/e2e/web-csp.spec.ts
key-decisions:
  - "TestCSPHeaders_PreviewDelta strips manifest-src before comparing so it keeps pinning Amendment 3 and ManifestDelta owns Amendment 4"
  - "index.html source hrefs are /manifest.json and /apple-touch-icon.png; Vite rewrites them to ./manifest.json and ./apple-touch-icon.png in dist"
requirements-completed: []  # MOB-02 build half done; the launch-URL check is 14-07 spike, 14-13 simulator and 14-14 real phone, so mark-complete was not run
---

# Phase 14 Plan 05: Home Screen manifest, icon and Amendment 4 Summary

The share view now links a static host-served manifest and a full-bleed opaque Portty icon, and the share CSP gained exactly one directive, `manifest-src 'self'` (Amendment 4), proven by Chromium parsing the manifest from `/app/` under the real policy.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.3UR9R6tXpn` (Task 1 Go and checks), `.../tmp.9m26u4enDr` (Task 1 e2e), `.../tmp.DZeHJPJDLW` (Task 2 vitest, build, tsc), `.../tmp.wzdIlnosHh` (final e2e and brand gate).

## Accomplishments

- **Manifest.** `frontend/public/manifest.json`: name, short_name `Portty`, `display: standalone`, `scope: "./"`, background and theme `#1C1C1E`, icons `icon-192.png` and `icon-512.png`. No `start_url`, `id`, `orientation` or `purpose`.
- **CSP Amendment 4.** `manifest-src 'self'; ` inserted after `worker-src 'self'; ` in `cspHeaders`, plus an Amendment 4 block in the package doc (full policy, rationale, 14-SECURITY-REVIEW.md sign-off pointer).
- **Head tags.** Manifest link, apple-touch-icon, `apple-mobile-web-app-capable yes`, `apple-mobile-web-app-title Portty`, two theme-color tags with prefers-color-scheme media, `viewport-fit=cover` appended to the viewport. Wails meta CSP and the splash style untouched. Built `dist/index.html` has `href="./manifest.json"` and `href="./apple-touch-icon.png"`.
- **Icon.** Read once and checked: the glyph spans x 296-728, y 242-782 of 1024, inside the central 60% (205-819).

## Final policy string (`<host>` is the listener host)

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; manifest-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

## Icon commands run (ad hoc, tools not recorded in tracked files)

Work files in the session scratchpad. Gradient ends `#3067FF` top to `#204EE6` bottom, taken from the tile.

1. `magick build/appicon.png -alpha off -crop 460x580+282+222 +repage crop.png` (the glyph block, centred on 512,512)
2. White "p" mask from the red channel: `-channel R -separate +channel -level 25%,100%`; dark underscore mask from the blue channel: `-channel B -separate +channel -negate -level 12%,75%`
3. Both masks upscaled `124.27%` (tile 824 to 1024 px, Lanczos); each coloured layer (`white`, `#0B1436`) takes its mask via `CopyOpacity` and is composited, centred, onto `magick -size 1024x1024 gradient:'#3067FF-#204EE6'`
4. `-alpha off -colorspace sRGB -type TrueColor -strip PNG24:build/appicon-fullbleed.png`
5. Exports: `magick build/appicon-fullbleed.png -filter Lanczos -resize NxN -alpha off -colorspace sRGB -type TrueColor -strip PNG24:<file>` for 512, 192, 180

| File | Size | Alpha (sips) | sha256 |
|------|------|--------------|--------|
| build/appicon-fullbleed.png | 1024 | no | e4ae1bd8e241bf501acea48f2a11f94b9eba34805716a380ac7c1d3b5fcc7369 |
| frontend/public/icon-512.png | 512 | no | 3686d33160333b1e5ee92596d9ea1e7b4f632025b4ae26d3170250cca19515a3 |
| frontend/public/icon-192.png | 192 | no | 18fe6d32b4699cb33d4eccf78d2b459ddc577d9ca98e74ce5dfcb8455c94cfa3 |
| frontend/public/apple-touch-icon.png | 180 | no | 05be46b325e188fa4beb06906d31b9a30eed64fb3e8aace176842596f5c69165 |

## Task Commits

1. **Task 1 (tracer): manifest, manifest link, Amendment 4, Chromium parse** - `f26647f8` (feat)
2. **Task 2: icon and remaining head tags, homeScreen test** - `ccd0e1b4` (feat)

## Test results

- `go test -race -short -count=1 -run CSP -v ./internal/webserver/`: ok, 0 FAIL. `TestCSPHeaders_ManifestDelta`, `TestCSPHeaders_NoUnsafeEvalToken_TokenAware`, `TestCSPHeaders_PreviewDelta` pass. Full Go suite not run here (14-12's gate).
- Playwright `web-csp --project=chromium` (`PHASE_93_FIXTURE_REBUILD=1`, node v24.22.1): 2 passed, 0 failed, run twice (after Task 1 and after the icons were added to the manifest). The new test reads `Page.getAppManifest`: no errors, URL ends `/app/manifest.json`, parsed `display` is `standalone`, no CSP console message.
- Mutation check: with the `manifest-src` line temporarily removed, the new e2e test failed (1 failed) and the Go delta test would fail by construction; line restored, tree checked.
- `vitest run homeScreen`: 11 tests passed (written first, 5 RED before the tags and icons, then GREEN).
- `vite build`: dist references `./manifest.json` and `./apple-touch-icon.png`. `tsc --noEmit`: exit 0. `tests/check-brand.sh`: clean.

## Existing tests edited

- `csp_mw_test.go` `TestCSPHeaders_PreviewDelta`: compares the policy minus `manifest-src 'self'; ` so it still pins Amendment 3.
- `csp_integration_test.go` `TestCSPHeaderStrict_App`: asserts `; manifest-src 'self'; ` is present on `/app/`.

## Deviations from Plan

None to the work. Executor notes:

- The tracer's Task 1 e2e re-run gate was satisfied by the passing verify; no checkpoint was required (the task has no `gate="blocking-human"`).
- `.planning/WINDOWS.md` does not exist, so no windows-ledger entries were appended (there are no stubs).

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. T-14-05 mitigation is in place (exact policy pinned by `TestCSPHeaders_ManifestDelta`, unsafe-eval guard still passing, no start_url or id); the 14-14 security review still owes the Amendment 4 sign-off.

## Not verified

- Whether the `?session=...&cap=...` query survives Add to Home Screen on iOS (M-145): that is 14-07's spike, 14-13 and the real phone.
- WebKit/Safari accepting the manifest under the CSP: only Chromium was run.
- Wails desktop not run with the new head tags (A10, VM row P-06 in 14-12).
- Full vitest and the full Go suite were not run.
- TESTING.md not updated (14-12 and 14-14 own it per the plan).

## Self-Check: PASSED

All created files present (manifest, three exports, 1024 source, homeScreen test); commits `f26647f8` and `ccd0e1b4` are ancestors of HEAD.
