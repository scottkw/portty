# Phase 6 VM Update Checks (M-92)

- **Date:** 2026-11-18
- **Build:** gsd/phase-06-release-updates at 5cfa6b4d plus the 06-04 Task 1 seam (updates_vmpreview.go, cmd/fakegh; committed in the first test(06-04) commit). `wails build -platform darwin/arm64 -tags "wailsassets vmpreview" -ldflags "-X main.Version=v0.9.0"`, ad-hoc signed by Wails (no Developer ID). frontend/src/wailsjs restored after the build.
- **Seam:** updateAPI `http://127.0.0.1:8788`, loopback-only allowlist (`http://127.0.0.1:8788/download/...`), updateFirstDelay 10 s (release value), updateTick 15 s, updateEvery 2 min.
- **Fake GitHub:** cmd/fakegh built `GOOS=darwin GOARCH=arm64`, run in the VM on 127.0.0.1:8788.
- **DMGs:**
  - good.dmg: GIFsmith v1.0.1 `Gifsmith-1.0.1-arm64.dmg` from `gh release download`, sha256 4119ccd13d2ee4a84c28c0d0a630ec4b830c23a712219eeaac7ba0ab32b3faa1; host check: TeamIdentifier=S2K7P43927, spctl "accepted, source=Notarized Developer ID". Served under Portty's asset name.
  - unsigned.dmg: `hdiutil create -format UDZO` of a folder with one text file ("code object is not signed at all").
  - adhoc.dmg: a copy of unsigned.dmg after `codesign -s -` (Signature=adhoc, TeamIdentifier=not set).
- **VMs:** office `portty-test` (192.168.64.108), home `portty-test-2` (192.168.64.109), macOS 27.2. UI read with `~/ptest/axdump`, menu items pressed with `axmenu`, the status item menu read with a small AX helper (`axtray`, scratch only), clicks with `click`.
- **Clean state** before each scenario: Portty killed, `~/Library/Application Support/portty/update_notified` and `~/Library/Caches/app.portty.Portty/updates` removed.
- **Notifications:** this ad-hoc wails build is not allowed to post notifications (`UNErrorDomain Code=1 "Notifications are not allowed for this application"`), so delivery was not observable. Per the plan's fallback, U-01/U-02 rest on Portty dispatching exactly one notification per version; M-93 checks delivery on the notarized release.
- Times are UTC (VM clock). Launch times are when `open` ran; Portty's own startup (and so the 10 s timer) starts about 1-2 s later.

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| U-01 | PASS | SC1 launch (office VM): fakegh newer 9.9.9 with good.dmg, clean state, launch; first check about 10 s after launch, banner with Download, one notification dispatch and one notified line, tray "Update Available…" | launched 17:37:43; fakegh `17:37:55Z GET /repos/scottkw/portty/releases/latest inm=no 200` (+12 s after open); gui.log `17:37:55 update: 9.9.9 available`, `notification: dispatching native notification for identifier portty.update-available` (once), `update: notified 9.9.9` (once); AX `AXGroup [AXApplicationAlert] banner banner--info` > `AXStaticText Value=Portty 9.9.9 is available.`, `AXButton Title=Download`, `AXButton Title=Dismiss`; axtray status item menu `["Open Portty", "Update Available…", "", "Quit Portty"]`; delivery not observable on this ad-hoc build (`notification authorization NOT granted ... Notifications are not allowed for this application`) |
