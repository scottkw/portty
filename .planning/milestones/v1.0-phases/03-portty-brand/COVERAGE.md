# API Coverage — GitHub Releases (via go-selfupdate v1.5.2)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
> Scope: Phase 3 changes an existing integration (the update check) to the scottkw/portty feed and adds a DMG asset filter (plan 03-02). AppKit, also flagged by the detector, is a local macOS framework used for the About panel, not an external service, so it has no rows here.

| capability | decision | reason |
|---|---|---|
| detect-latest-release (list releases for scottkw/portty) | INTEGRATE | |
| match-release-asset-by-name (portty-v…-darwin-universal.dmg filter) | INTEGRATE | |
| not-found-and-404-handling (no release yet, private repo) | INTEGRATE | |
| network-and-rate-limit-error-handling (silent, hourly rate limit kept) | INTEGRATE | |
| draft-releases-as-update-candidates | OPT-OUT | explicitly out of scope — UPD-04 says drafts are handled quietly; go-selfupdate skips them by default |
| prerelease-releases-as-update-candidates | OPT-OUT | explicitly out of scope — UPD-04 says prereleases are handled quietly; go-selfupdate skips them by default |
| download-release-asset (fetch the DMG) | OPT-OUT | not needed yet — Phase 6 UPD-03 owns download, verify and reveal |
| checksum-asset-verification (checksums.txt SHA-256) | OPT-OUT | not needed yet — Phase 6 UPD-03 |
| release-notes-display | OPT-OUT | not needed — the UI-SPEC banner shows only current and latest versions |
| authenticated-api-access (token) | OPT-OUT | explicitly out of scope — UPD-01 requires the public Releases API with no login |
| create-or-publish-release | OPT-OUT | explicitly out of scope for the app — .github/workflows/release.yml publishes releases |
