# Phase 11 VM Checks: file preview (PREV-01..PREV-05)

Evidence for plan 11-10 from real app builds in the two Tart VMs: ROADMAP Phase 11 SC1 (previews in the office local tab and the home peer tab over the real tailnet, the focus rule, close, CLI errors), SC2 (every type in light and dark, hostile fixtures against loopback beacons), SC3 (a near-cap stream with a ticking terminal and four viewers, over-cap, route probes), SC4 (the Safari share viewer under the reviewed CSP), SC5 (`portty` on the PATH and Install Command-Line Tool…), and an upgrade sanity check (API 3, a mutating route under the handoff freeze gate, a new session environment). Result is exactly `PASS` or `FAIL`.

- **Date:** 2026-11-21, from 17:40 VM clock (UTC). Claude ran every row over SSH from the host.
- **Base commit:** builds from `f43f20bb` (gsd/v1.2 `7042f5ce`, all of 11-01..11-09 and the 11-07 Markdown worker fix `b0785383`/`7c643a0a`, plus this plan's TESTING.md commit); product code is `7042f5ce`'s. A is the pre-phase base `7dbe384e`.
- **VMs:** office `portty-test` (192.168.64.108, tailnet `portty-vm-office` 100.91.251.68) and home `portty-test-2` (192.168.64.109, tailnet `portty-vm-home` 100.112.33.35), both macOS 27.2 arm64, both already running and confirmed with `tart ip`. Before the run each ran the `dev` build `90541495583f…` (the 11-01 restore) with 0 sessions: office GUI 30453 / daemon 30457, home GUI 46152 / daemon 46156, each `/health` `{"status":"ok","version":"dev","api":2,"handoff":1}`. The VMs reach each other through the DERP relay (`relay "dfw"`).
- **Ephemeral-node rule:** both VMs are ephemeral tailnet nodes. They were never stopped, suspended or rebooted; `tailscale down`, `tailscale logout` and auth keys were never used. No row drops the network.
- **Install method:** bundles staged in `~/ptest/stage/<name>` (tar over SSH), then `rm -rf ~/ptest/Portty.app` and `ditto` (`~/p907/inst.sh`); launch with `open --stderr /tmp/<name>.err ~/ptest/Portty.app`; stop with `~/p907/stopall.sh`. P-08 alone uses `/Applications/Portty.app` (SC5 names it); before the run the office had **no** `/Applications/Portty.app` and **no** `/usr/local/bin/portty` (`/usr/local/bin` held only `tailscale` and `tailscaled`), and the home VM had neither either.
- **Monitoring:** office and home `~/Library/Application Support/portty/daemon.log` lines (`viewer-diag: preview accepted …`, `preview start …`, `preview end … status=…`, `join`, `leave`, `replay … mode=`, `heartbeat timeout`, `proxy heartbeat timeout`, `too slow`, `grid …`, `cli link:`, `cli install:`, `handoff`); AX dumps of the GUIs (`~/ptest/axdump2`: tab list, selected tab, panes, dialogs, `AXPDFPluginSubrole`/`AXPage`); WebDriver DOM reads for Safari; screenshots (`screencapture -x`, copied to the host and inspected, not committed); session tails through `curl --unix-socket … /sessions/<id>/tail`; `stty -f /dev/<tty> size`; beacon logs; file sizes and sha256.
- **Helpers:** the 09-07 rig in `~/p907` and `~/ptest` and the Phase 10 helpers in `~/p10` (unchanged), plus new Phase 11 helpers in `~/p11` on each VM: `beacon.pl` (loopback listener on 127.0.0.1:8765, one line per connection in `~/p11/beacon.log`; self-test `GET /selftest` = 204 and one line), `samples.sh` (office: the samples below), `p.sh` (AX tab list, typing into the Portty terminal through System Events, daemon log marks, session tail, appearance switch, screenshots, beacon count).
- **No secrets:** share links went from the office VM to the home VM in 0600 files over an SSH pipe and were never printed; share checks record HTTP codes only; CSP evidence records directive names only (the `connect-src` host is written `<host>`).
- **Administrator prompt rule:** the prompt runs only in the office VM. The VM password is never written to a file, log, command line or this evidence.
- **Nothing on the host** but the gate, the builds, `tar`, `shasum`, `codesign -dv` and SSH. Every install, kill, click, key event, appearance switch and app launch ran inside a VM; the host's own Portty and `/usr/local/bin` were not touched.

## Gate (host, run once, 17:39:40 to 17:42:09 UTC)

`GOTOOLCHAIN=go1.26.11`: `go build ./...` and `go vet ./...` clean; `go test -race -short -count=1 ./...` all 13 packages `ok`, 0 FAIL lines, 0 `--- SKIP` lines, no DATA RACE or panic; `pnpm install --frozen-lockfile` clean; `pnpm vitest run` 104 files / 1510 tests passed; `tsc --noEmit` clean; check-brand, check-share-removal and check-traceability (188 paths) clean; `tests/build-script.test.sh` 40 passed, 0 failed. No flake reruns. `frontend/src/wailsjs` clean afterwards.

## Builds

Host: A from a `git archive` of `7dbe384e` (`pnpm install --frozen-lockfile --prefer-offline`, `pnpm run build`); B, B2 and HEAD in this checkout after one `pnpm run build`. Each with `GOTOOLCHAIN=go1.26.11 wails build -s -platform darwin/arm64 -tags wailsassets` and `-ldflags "-X main.Version=<v>"` (none for HEAD); each logged `Generating bindings`. `codesign -dv`: `Signature=adhoc`, `TeamIdentifier=not set` for all four (no Developer ID). `<bin> version` printed the version below. The sha256 values matched on the host and in the VMs (`~/ptest/stage/<name>`: A, B, B2 and HEAD to office; A, B and HEAD to home). `frontend/src/wailsjs` was restored (`git checkout`, `git clean`, the generated `wailsjs/wailsjs/go/main` removed); `build/bin`, `frontend/dist` and the A source tree were removed; `git status --porcelain --ignored` showed nothing but `frontend/node_modules`.

| App | Version | Binary sha256 |
|-----|---------|---------------|
| A (pre-phase, `7dbe384e`) | v1.2.0-vmA | `558d653f89959dd6b69886a4fd6fa691c3e71e4c1e6fec0de08d2f75bee3f5eb` |
| B | v1.2.0-vmB | `248f596870e413f69ce7c96256f4b603f23b59e41f2f31e37ecf5305b972d6f8` |
| B2 (B's tree, only the version changed) | v1.2.0-vmB2 | `f517f65311c2a946f9b9ec25c3eb2289c1590094e9a61852ab54d0e6a2323fd0` |
| HEAD (deployed on both at the end) | dev | `4230ab8a7900e3b2032d49b8020750dc41f9f60ed3c5df7d6c9b0fec9e24787b` |

## Samples (office `~/p11/s`)

| File | Bytes | sha256 (first 16) | Made with |
|------|-------|-------------------|-----------|
| sample.png | 60,905 | `9718c3c2628b4f0e` | `screencapture -x`, `sips -Z 1600` (1600x900) |
| sample.jpg | 46,083 | `93524cc0abedaf0c` | `sips -s format jpeg` |
| sample.gif | 20,196 | `3ad3f04b8b690ecd` | `sips -s format gif` |
| sample.webp | 1,440 | `e38e230dead989cc` | `sips` cannot write WebP here; a 320x200 WebP made on the host (`magick`, `cwebp`) and written from base64; `sips -g format` reads `webp` 320x200 |
| sample.svg | 306 | `ea00071d428687b6` | hand-written, 240x120 |
| sample.pdf | 18,965 | `a899f70f28507635` | `cupsfilter` of 150 text lines (3 pages) |
| sample.md | 457 | `53d645d93fb5902b` | headings, lists, a table with alignment, code, a quote, a link, an image reference, tasks |
| hostile.md, hostile-script.svg, hostile-external.svg | 3,724 / 913 / 1,888 | `2fd3fbb1fd7ab525` / `5d52f511423b04f6` / `fc235f6a88383af1` | the repo fixtures with `BEACON` replaced by `127.0.0.1:8765` |
| mdslow.md | 81,000 | `0fdabaf4d25e13c2` | `'*a **b _c'` x 9000, no separator (11-07-FIX.md: must stop at the 3 s worker limit) |
| damaged.pdf | 30,400 | `04204096b5f53297` | the first 400 bytes of sample.pdf, then 30,000 random bytes (11-06's known limit) |
| near.jpg | 19,900,000 | `1bba7ae3b3c9ce13` | sample.jpg padded with random bytes after its end marker |
| over.png | 20,000,001 | `732a4d0ecd7aff2b` | sample.png padded with random bytes |
| notes.txt | 17 | `e2ea404d1cce4e1b` | plain text |

## Rows

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
