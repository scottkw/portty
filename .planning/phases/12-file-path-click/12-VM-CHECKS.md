# Phase 12 VM Checks: file path click (PATH-01..PATH-02)

Evidence for plan 12-07 from real app builds in the two Tart VMs: ROADMAP Phase 12 SC1 (the zsh shim keeps shell startup intact, the cwd is reported after every `cd`), SC2 (a Cmd-click on a printed path previews it in the office local tab, in a home peer tab over the real tailnet and in home Safari, and only the clicker switches), SC3 (a missing path or folder is silent, a plain click selects, URLs are unchanged, notices show only in the clicking window), SC4 (a read-only viewer sees no underline and its hand-sent click previews nothing), the UI-SPEC backstops (the modifier-held underline and the notice in light and dark, a mouse-reporting TUI, resize and wrapped paths) and the upgrade check.

- **Date:** 2026-11-21 local (the VM clocks and every log timestamp below are UTC, so they read 2026-11-22 from 01:00). Claude ran every row over SSH from the host.
- **Base commit:** builds from `bd7fcf9c` (gsd/v1.2, all of 12-01..12-06 plus this plan's TESTING.md commit); product code is `bcabec11`'s. A is the pre-phase base `dfff923d` (all Phase 11 code, no Phase 12 code).
- **VMs:** office `portty-test` (192.168.64.108, tailnet `portty-vm-office` 100.91.251.68) and home `portty-test-2` (192.168.64.109, tailnet `portty-vm-home` 100.112.33.35), both macOS 27.2 arm64, both already running and confirmed on the tailnet before the run.
- **Ephemeral-node rule:** both VMs are ephemeral tailnet nodes. They were never stopped, suspended or rebooted; `tailscale down`, `tailscale logout` and auth keys were never used. No row drops the network.
- **Install method:** bundles staged in `~/ptest/stage/<name>` (tar over SSH), then `rm -rf ~/ptest/Portty.app` and `ditto` (`~/p907/inst.sh`); launch with `open --stderr /tmp/<name>.err ~/ptest/Portty.app`; stop with `~/p907/stopall.sh`.
- **Monitoring:** office and home `~/Library/Application Support/portty/daemon.log` lines (`viewer-diag: path click session=… conn=… outcome=… bytes=…`, `zsh shim:`, `preview start`, `preview end … status=`, `join`, `leave`, `replay … mode=`, `grid`, `handoff`); AX dumps of the GUIs (`~/ptest/axdump2`: tab list, selected tab, banner text, `DOMClassList`); WebDriver DOM reads for Safari; the session tail through `curl --unix-socket … /sessions/<id>/tail` on the VM; file sizes and sha256.
- **Helpers:** the Phase 9 to 11 rig in `~/p907`, `~/p10`, `~/p11` (unchanged) plus new Phase 12 helpers in `~/p12` on each VM: `p12ui` (real CGEvents compiled on the host: typed text, a click, a Cmd-click and a Cmd-hold at a screen point; no System Events), `appear` (light/dark through SkyLight), `p.sh` (daemon log marks, AX tab and notice reads, terminal cell geometry, session tail) and the sample scripts. No helper holds a secret.
- **No secrets:** share links went from the office VM to the home VM in 0600 files and were never printed; share checks record HTTP codes only; the read capability was minted inside the office VM by a throwaway signer whose output stayed in a 0600 file, and the signer, its output and its worktree were deleted after C-05.
- **Dotfiles rule:** the office VM user's `~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.zlogin` and `~/.config/zsh` are backed up as `~/p12/dotbak.tgz` (sha256 recorded) before C-08 and restored with the sha256 compared before any later row.
- **Signer rule:** a detached worktree at `/tmp/p12-sign` on the host holds a `main` package that imports `internal/capability`; it is never committed and is deleted after C-05.
- **Nothing on the host** but the gate, the builds, the signer build, `tar`, `shasum`, `codesign -dv` and SSH. Every install, kill, click, key event, appearance switch and app launch ran inside a VM; the host's own Portty, dotfiles and `/usr/local/bin` were not touched.

## Gate (host, run once, 01:13:06 to 01:17:13 UTC)

`GOTOOLCHAIN=go1.26.11`: `go build ./...` and `go vet ./...` clean; `go test -race -short -count=1 ./...` all 17 package lines `ok` (0 `--- FAIL` or `^FAIL` lines, no DATA RACE or panic; the run is not `-v`, so skipped tests print nothing, and the zsh shim tests are the 12-02 ones that ran on this Mac under `/bin/zsh`); `pnpm install --frozen-lockfile` clean; `pnpm vitest run` 106 files / 1576 tests passed; `tsc --noEmit` clean; `check-brand`, `check-share-removal` and `check-traceability-paths` (196 paths) clean; `tests/build-script.test.sh` 40 passed, 0 failed. `git status --porcelain -- frontend/src/wailsjs` empty afterwards.

## Builds

Host: A from a detached worktree at `dfff923d` (`pnpm install --frozen-lockfile --prefer-offline`, `pnpm run build`, removed afterwards); B, B2 and HEAD in this checkout after one `pnpm run build`. Each with `GOTOOLCHAIN=go1.26.11 wails build -s -platform darwin/arm64 -tags wailsassets` and `-ldflags "-X main.Version=<v>"` (none for HEAD); `frontend/src/wailsjs` restored after each. `codesign -dv` on all four: `Signature=adhoc`, `TeamIdentifier=not set`.

| App | Version | Binary sha256 |
|-----|---------|---------------|
| A (pre-phase, `dfff923d`) | v1.2.0-vmA | `0341d0f803bbb353fe610db1bf05c75efa2f34b92fd4485209b9c6c5f2f04a1f` |
| B | v1.2.0-vmB | `1dedd51f190b0c7d2a840d4e7f35edfab1a8373efd98c2055e613d18a4298d1f` |
| B2 (B's tree, only the version changed) | v1.2.0-vmB2 | `84df0188281d361db9c78da3100aac014c214c98f4fb69365084e41ea820b0cb` |
| HEAD (deployed on both at the end) | dev | `bfddb9cfedf1be1697464e93c148dadb138c85b827bde3c94f968dcb980feceb` |

## Evidence

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
