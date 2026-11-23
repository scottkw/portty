# Phase 14 VM Checks: mobile web app (MOB-01..)

Evidence from the two Tart VMs over the real tailnet. This file starts with plan 14-02's before rows: the lone-Esc hold on the build already installed in the VMs, recorded before `InputAbsorber.Filter` changed. Later plans append their rows (14-12 adds the after rows, M-150).

- **Date:** 2026-11-22. The VM clocks and every log timestamp below are UTC. Claude ran every row over SSH from the host.
- **Base commit:** `9b3e6b2a` (gsd/v1.2, after 14-01). No Phase 14 product code is in the installed build: it is Phase 13's HEAD `dev` build, left installed and running on both VMs by 13-VM-CHECKS.md's Cleanup. The filter change comes after these rows.
- **VMs:** office `portty-test` (192.168.64.108, tailnet `portty-vm-office`) and home `portty-test-2` (192.168.64.109, tailnet `portty-vm-home`), both macOS 27.2 arm64, both already running and on the tailnet before the run (`tailscale status` in each lists the other as active). The office is the host Mac (it runs the session and the share server); the home VM is the viewing Mac (its peer tab is the share server path).
- **Installed build (both VMs):** `dev`, `/health` `{"status":"ok","version":"dev","api":5,"handoff":1}`, binary sha256 `a01ae0e147c03409590ef7b7fa8447e036ffba93b12372d0afff44f17d2e0c20` (the HEAD row of 13-VM-CHECKS.md Builds). 0 sessions on each at the start. Nothing was built or installed for these rows.
- **Method:** `~/p12/p12ui` real CGEvents inside the VM (typed text; an Esc is a `p12ui type` of the ESC character) after `p.sh front`/`focus`; the session's tail from the daemon socket (`p.sh tail`); `daemon.log` join lines. The raw key reader is `~/p14/keys.zsh` in the office VM (VM-only, never committed): `stty raw -echo`, a `read -rk1` loop appending `<EPOCHREALTIME> <hex>` per byte to `~/p14/keys.log`, stopping on `q`, then `stty sane`. Before the rows it was checked on the office VM through an `expect` pty (ESC, `x`, `q` 0.3 s apart): it logged `1b`, `78`, `71`, so it reports an ESC byte as `1b`.
- **Terminal rows are not readable over AX in this build.** The plan reads vim's bottom row from the AX tree. In this build the AX dump of the terminal panel holds only the `xterm-viewport`, `xterm-helper-textarea` and `xterm-decoration-container` groups (no accessibility-tree rows, with WebGL off), and `screencapture` of the window comes back as an empty window frame (the web view is not captured). So "`-- INSERT --` still shown" is evidenced two other ways: (a) the session tail's sha256 is identical at the moment of the ESC and 3 s later (vim redraws its mode line when it leaves Insert mode, which would add bytes), and (b) a VM-only probe script (`vim -S ~/p14/probe.vim`, one `InsertLeave` autocmd writing `localtime()` to `~/p14/leave.txt`) shows vim had not left Insert mode at +3 s, and the second at which it did. Vim itself is plain `vim` (no `~/.vimrc` or `~/.vim` in the VM).
- **No secrets:** the share link was written only to a 0600 file in a `mktemp -d` directory in the office VM (never printed, never on a command line); `sharechk.sh` printed HTTP codes only. This file records no link, capability, key or tailnet URL.
- **Ephemeral-node rule:** both VMs are ephemeral tailnet nodes. They were never stopped, suspended or rebooted; `tailscale down`, `tailscale logout` and auth keys were never used. No row drops the network. Only the Portty GUI and daemon inside each VM were restarted (needed to switch WebGL off and back), which does not touch the tailnet.
- **WebGL and dotfile rules:** WebGL was switched off in each VM's `settings.json` for the run (`~/p907/webgl.sh off`, backup first, restored after, checked with `cmp`) even though the AX rows turned out not to be available; the sha256 of each VM user's `~/.zshrc` and `~/.zsh_history` was recorded before the rows (private copies in a `mktemp -d` directory) and compared at the Cleanup.
- **Real machine safety:** the VMs' sidebars also list the user's own Mac under "Other Macs"; no row touched it (it was never clicked, shared to or typed into). Nothing ran on the host except `tart ip`, SSH and file writes in this repo.

**14-12 additions (the after rows, the gate and the builds):**

- **Base commit for 14-12:** `a121c3fd` (gsd/v1.2: all of 14-01..14-11 plus this plan's TESTING.md commit; its product code is identical to `a0f88200`). The P rows below run on builds from this tree.
- **Builds:** HEAD (`v1.2.0-vm14`) and W14 (`v1.2.0-vm14w`, the `vmwhois` tag) built on the host in this checkout after one `pnpm run build`, ad-hoc signed; the helper `p14dev` built in a temporary worktree. Versions, sha256 and signatures are in "Builds (14-12)". W14 is installed only on the office VM for P-02 and is removed by the Cleanup.
- **Helper `p14dev`:** `-whois-bench -n <N> -addr <ip>` (VM-only; it never ran on the host). It prints p50, p95 and max for `StatusWithoutPeers` and `WhoIs` against the VM's own tailscaled, the answering node's numeric user ID, the self user ID, and whether that node is the self node. No names, no other output. Source never committed.
- **The key reader:** `~/p14/keys.zsh` in the office VM, rewritten as 14-02 described (14-02's cleanup removed it).
- **Phone-check rule:** the last thing this plan does is leave one shared session `phone` (a tick counter) running on the office VM on HEAD, its link only in a 0600 file in a host `mktemp -d` directory; 14-14 ends it in every outcome. See "Phone check (left running for 14-14)".
- **`vmwhois` mode file:** `$HOME/p14/whois-mode` in the office VM (`other`, `pass`, `fault`; a missing file means `other`).

## Before rows (14-02)

Timings are read on the office VM's clock (the `keys.log` lines); the "sent" times are the sending VM's clock, both UTC, and the two VM clocks agree to within a few ms (row E-01: ESC sent at `23:48:02.137` from the home VM, `x` at `23:48:05.237`, the office log's two bytes at `23:48:05.230`).

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| E-00 | PASS | A lone Esc is held until the next key on the installed build, in a local tab (local relay pump): vim stays in Insert mode for 3 s after an ESC sent alone, and the raw key reader logs the ESC byte only when the next key arrives | <ul><li>Preflight: `tart ip` gave 192.168.64.108 and .109; both `/health` lines and binary sha256 as in the header; dotfile sha256 before: office `~/.zshrc` `e3b0c44298fc1c14…` (the empty file), `~/.zsh_history` `e09568dbf568fc9a…`; home `~/.zshrc` the same empty-file hash, `~/.zsh_history` `6b678cce73175753…` (equal to the 13-VM-CHECKS.md Cleanup values). Office session `e00` made by the installed build's `portty new`/`rename`, opened from the sidebar in a local tab (`join session=35ee8795 conn=1 origin=local`, 23:44:33).</li><li>vim leg: `vim -S ~/p14/probe.vim`, typed `ihello` (Insert mode, text `hello` in the buffer: the session tail shows `-- INSERT --` and `hello`). ESC sent alone at 23:46:41.60 (epoch 1795391201.60). Session tail sha256 (first 12 hex) `8d8116068829` at the ESC and `8d8116068829` at +3 s: no byte came back from vim in the 3 s. `~/p14/leave.txt` absent at +3 s. Then `:q!` typed at 23:46:45.02 (ESC, `:`, `q`, `!`, Return reach the PTY together): `leave.txt` then held `1795391206` (23:46:46), the second at which vim left Insert mode (whole-second resolution, so 3.4 to 5.4 s after the ESC was sent, and only after the next key). A first run of the same leg (probe writing `reltime()`) read tail sha256 `82818a6a8a3a` before and at +3 s and `leave.txt` absent at +3 s too.</li><li>Key reader leg: `zsh ~/p14/keys.zsh` typed, ESC sent alone at 23:47:02.22 (epoch 1795391222.22), `x` sent 3.09 s later at 23:47:05.31, then `q`. `~/p14/keys.log`: `1795391225.2734870911 1b`, `1795391225.2736690044 78`, `1795391226.1628649235 71`. The `1b` line carries the timestamp of the `x` (0.2 ms apart, limit 0.5 s), 3.06 s after the ESC was sent: the ESC was held for the whole gap. This is the hold the filter change removes.</li></ul> |
| E-01 | PASS | The same hold through the share server's pump: a home peer tab on the shared session logs the ESC byte only when the next key arrives | <ul><li>Setup: `portty share e00` on the office with the link written only to a 0600 file in a private directory (315 bytes, never printed); `sharechk.sh`: link 302, info 200, wrong-token link 401, wrong-token info 401. The home VM's sidebar (Other Macs, `portty-vm-office`) row `e00` clicked with `~/p13/open.sh`: tab `e00 · portty-vm-office`; office log `23:47:46.215045 viewer-diag: join session=35ee8795 conn=2 origin=web from=<home tailnet ip> viewers=2 web=1`, then `web resize … cols=85 rows=29 claim=false`. So the peer tab is a web viewer on the share server's WebSocket.</li><li>Key reader started in the peer tab (`zsh ~/p14/keys.zsh` typed from the home VM, running in the office shell). From the home VM: ESC sent alone at 23:48:02.137 (epoch 1795391282.137), `x` at 23:48:05.237, `q` after. Office `~/p14/keys.log`: `1795391285.2300240993 1b`, `1795391285.2305760384 78`, `1795391286.1374220848 71`. The `1b` line is 0.55 ms before the `78` line and 3.09 s after the ESC was sent: the ESC was held until `x`, as in E-00.</li></ul> |

Both rows reproduce the hold, so the reason for the filter change stands and the change goes ahead.

## Cleanup (14-02)

Run after E-01, on both VMs.

- `portty unshare e00` ("Stopped sharing", remote viewers disconnected) and `portty kill` of the session `e00`; `portty list --local` empty; no `vim`, `keys.zsh` or `expect` process left. The share link file, the `mktemp -d` directory holding it and the dotfile copies, `~/p14` (the key reader, probe, scripts and logs: the log lines are copied into the rows above) and `~/p13/.ld` were deleted in both VMs; no `~/p14`, no `.ld` and no `/tmp/*.url` remain. The screenshot files taken to try AX/visual reads were deleted from the VM and the host.
- `~/p907/stopall.sh` (Portty GUI and daemon TERM) then `~/p907/webgl.sh restore`: `settings restored (cmp clean)` on the office; on the home VM `cmp` against the backup was clean before the backup directory `~/ptest/stage` (created for this run) was removed. `"webgl":true` again in both.
- Dotfiles: home `~/.zshrc` and `~/.zsh_history` equal to the values recorded before (`e3b0c44298fc1c14…`, `6b678cce73175753…`). Office `~/.zshrc` equal; `~/.zsh_history` had changed to `d0416f94d46b75359cc298d61ac24680978c642bd455097de8a36b41b56d9c4f` when `e00`'s shell ended, so it was restored from the private copy after every session was gone: now `e09568dbf568fc9a…`, equal to the value before.
- The installed Phase 13 HEAD build (`dev`, sha256 `a01ae0e147c03409…`) relaunched on both VMs with `open --stderr` into a fresh private directory: office GUI 39844 / daemon 39846 and home GUI 32450 / daemon 32452, `/health` `{"status":"ok","version":"dev","api":5,"handoff":1}` on both, GUI stderr 0 bytes, 0 sessions. Both VMs still on the tailnet (`tailscale status` in each lists the other as active).
- The VMs themselves were never stopped, suspended or rebooted; no `tailscale down`, `logout` or auth key. Nothing was built or installed. The two fresh private directories from the final relaunch each hold only a 0-byte `gui.err`.

## Gate (14-12)

Run once on the host, 2026-11-23 01:23:46 to 01:30:32 UTC, `GOTOOLCHAIN=go1.26.11`, nvm node v24.22.1 first on PATH, `NO_COLOR=1`, each step's exit code recorded and no output piped through `tail` or `head`. Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.lZuXernbL5` (`gate-*.log`, `rc.txt`, `rerun-*.log`).

| Step | Exit code | Result |
|------|-----------|--------|
| `go build ./...`, `go vet ./...` | 0, 0 | clean |
| `go test -race -short -count=1 ./...` | **1** | 13 package lines `ok`, one FAIL: `internal/pty` `TestKillClean_NoOrphans` (`native_test.go:287: process group 72715 still exists after Kill`), 0.53 s inside a 21.98 s package run that ran in parallel with the 247 s daemon and 215 s relay packages. 0 `--- SKIP` lines. See "G-1 re-run" below |
| `go test -tags vmwhois -count=1 -run VMWhoIs ./internal/daemon/` | 0 | `ok` |
| `pnpm install --frozen-lockfile` | 0 | clean |
| `pnpm vitest run` | **1** | 121 of 122 files and 1808 of 1809 tests passed; one FAIL: `src/lib/__tests__/pathLinks.linkifier.test.ts` "after a reconnect, another cell of the same row offers the link" (`term.onRender fired: expected 2 to be greater than 2`), the 14-08 deferred flake (`deferred-items.md`). See "Linkifier re-run" below |
| `tsc --noEmit` | 0 | clean |
| `tests/check-brand.sh`, `check-share-removal.sh`, `check-traceability-paths.sh` | 0, 0, 0 | clean; 228 traceability paths exist |
| `tests/build-script.test.sh` | 0 | 40 passed, 0 failed |
| Playwright (`PHASE_93_FIXTURE_REBUILD=1`, all projects including `iphone`) | 0 | 52 passed, 31 skipped, 0 failed; 11 of the passes are the `iphone` project |

`frontend/src/wailsjs` was restored after the build-script test and after the builds (empty `git status`).

**G-1 re-run (evidence).** After the gate, `go test -race -short -count=5 -run TestKillClean_NoOrphans ./internal/pty/` passed (exit 0, `ok ... 3.574s`). This is the same test, assertion and symptom as Phase 12's G-1 and Phase 13's, accepted by the user on 2026-11-22 as a timing flake under load. `internal/pty` has no diff since `9574fd49` (`git log 9574fd49..HEAD -- internal/pty` is empty). Nothing was patched.

**Linkifier re-run (evidence).** `vitest run src/lib/__tests__/pathLinks.linkifier.test.ts` alone passed (9 of 9). Its failing test waits a fixed `sleep(100)` for a real xterm render; it passed 9 of 9 alone when 14-08 hit it, and the file imports nothing Phase 14 touched. Not patched.

Both are findings for 14-14, not decided here. The gate ran once; these two re-runs are evidence only. The VM rows do not depend on either test.

## Builds (14-12)

Host, in this checkout after one `pnpm --dir frontend run build`, in a shell with the `wailsjs` restore trap, `GOTOOLCHAIN=go1.26.11 wails build -s -platform darwin/arm64 -tags wailsassets -ldflags "-X main.Version=<v>"` (W14 adds the `vmwhois` tag). `codesign -dv` on both: `Signature=adhoc`, `TeamIdentifier=not set`. `frontend/src/wailsjs` restored after the builds. p14dev: `GOOS=darwin GOARCH=arm64 go build` in a detached worktree at HEAD (`cmd/p14dev` written into the tree, never committed); the worktree and its directory were removed after the binary was copied out (0 `p14-help` worktrees registered). The VM-side sha256 values are recorded in P-00.

| App | Version | Signature | Host binary sha256 |
|-----|---------|-----------|--------------------|
| HEAD | v1.2.0-vm14 | `Signature=adhoc`, `TeamIdentifier=not set` | `95ac988e504dd54648779691089b3e468e5db9ccef4f37be003573a2dd3dcdcb` |
| W14 (`vmwhois`) | v1.2.0-vm14w | `Signature=adhoc`, `TeamIdentifier=not set` | `e78236d24228a28c9ffdeb8e3165e767dacfa09295decdebeaf838b62a6ed7ae` |
| p14dev (helper, not part of the product) | n/a | n/a | `b3b549a43a08899617d17b4f80b627fb4d49052cc325bd220550ab0a71fc9071` |

## Playwright emulation rows

From the gate's single Playwright run (`gate-e2e.log`, the iPhone 15 device descriptor on WebKit for the `iphone` project; chromium, firefox and webkit for the others). These are emulation, not a phone: no real touch hardware, keyboard or Safari chrome. The simulator and real phone rows are 14-13 and 14-14. The manifest test is chromium only (it reads `Page.getAppManifest`); the same spec skips on firefox and webkit.

| Check | Result | Spec and test | Evidence |
|-------|--------|---------------|----------|
| I-01 | PASS | `e2e/mobile-iphone.spec.ts:28` on `iphone`: key bar: tapping Esc reaches the shell as 1b | gate-e2e.log line 109, test 73, 790ms, listed with the passing mark |
| I-02 | PASS | `e2e/mobile-iphone.spec.ts:33` on `iphone`: key bar: Tab sends 09 | gate-e2e.log line 110, test 74, 474ms, listed with the passing mark |
| I-03 | PASS | `e2e/mobile-iphone.spec.ts:38` on `iphone`: key bar: the pipe key sends 7c | gate-e2e.log line 111, test 75, 443ms, listed with the passing mark |
| I-04 | PASS | `e2e/mobile-iphone.spec.ts:43` on `iphone`: key bar: Up sends 1b5b41 | gate-e2e.log line 112, test 76, 460ms, listed with the passing mark |
| I-05 | PASS | `e2e/mobile-iphone.spec.ts:48` on `iphone`: key bar: Ctrl then Up sends 1b5b313b3541 and Ctrl releases | gate-e2e.log line 113, test 77, 503ms, listed with the passing mark |
| I-06 | PASS | `e2e/mobile-iphone.spec.ts:56` on `iphone`: key bar: Ctrl then typing c on the keyboard sends 03 and Ctrl releases | gate-e2e.log line 114, test 78, 453ms, listed with the passing mark |
| I-07 | PASS | `e2e/mobile-iphone.spec.ts:64` on `iphone`: key bar: the eight keys fit one row at 44 pt or more | gate-e2e.log line 115, test 79, 405ms, listed with the passing mark |
| I-08 | PASS | `e2e/mobile-iphone.spec.ts:82` on `iphone`: phone chrome: with only the session tab, the tab strip is not shown | gate-e2e.log line 116, test 80, 370ms, listed with the passing mark |
| I-09 | PASS | `e2e/mobile-iphone.spec.ts:86` on `iphone`: phone chrome: terminal padding is 4px 8px | gate-e2e.log line 117, test 81, 400ms, listed with the passing mark |
| I-10 | PASS | `e2e/mobile-iphone.spec.ts:91` on `iphone`: phone chrome: the xterm helper textarea is 16px, so iOS does not zoom on focus | gate-e2e.log line 118, test 82, 396ms, listed with the passing mark |
| I-11 | PASS | `e2e/mobile-iphone.spec.ts:96` on `iphone`: phone chrome: the key bar is the last row, below the terminal area | gate-e2e.log line 119, test 83, 392ms, listed with the passing mark |
| I-12 | PASS | `e2e/web-same-user.spec.ts:25` on `chromium`: same-account gate: another account gets the refusal page and no session data | gate-e2e.log line 43, test 19, 139ms, listed with the passing mark |
| I-13 | PASS | `e2e/web-same-user.spec.ts:42` on `chromium`: same-account gate: a tailscaled fault gets the retry page, and Try again reloads the same URL | gate-e2e.log line 44, test 20, 252ms, listed with the passing mark |
| I-14 | PASS | `e2e/web-same-user.spec.ts:58` on `chromium`: same-account gate: the same account opens the share view | gate-e2e.log line 45, test 21, 248ms, listed with the passing mark |
| I-15 | PASS | `e2e/web-same-user.spec.ts:25` on `firefox`: same-account gate: another account gets the refusal page and no session data | gate-e2e.log line 71, test 43, 313ms, listed with the passing mark |
| I-16 | PASS | `e2e/web-same-user.spec.ts:42` on `firefox`: same-account gate: a tailscaled fault gets the retry page, and Try again reloads the same URL | gate-e2e.log line 72, test 44, 548ms, listed with the passing mark |
| I-17 | PASS | `e2e/web-same-user.spec.ts:58` on `firefox`: same-account gate: the same account opens the share view | gate-e2e.log line 73, test 45, 358ms, listed with the passing mark |
| I-18 | PASS | `e2e/web-same-user.spec.ts:25` on `webkit`: same-account gate: another account gets the refusal page and no session data | gate-e2e.log line 99, test 67, 280ms, listed with the passing mark |
| I-19 | PASS | `e2e/web-same-user.spec.ts:42` on `webkit`: same-account gate: a tailscaled fault gets the retry page, and Try again reloads the same URL | gate-e2e.log line 100, test 68, 488ms, listed with the passing mark |
| I-20 | PASS | `e2e/web-same-user.spec.ts:58` on `webkit`: same-account gate: the same account opens the share view | gate-e2e.log line 101, test 69, 371ms, listed with the passing mark |
| I-21 | PASS | `e2e/web-csp.spec.ts:49` on `chromium`: web-csp: Chromium parses /app/manifest.json under the share CSP | gate-e2e.log line 36, test 12, 273ms, listed with the passing mark |
| I-22 | PASS | `e2e/web-share-scope.spec.ts:23` on `chromium`: web-share-scope: web-share /app/ guest sees the scoped session surface, NOT the desktop Sidebar (the test carries the desktop no-key-bar assertion: no toolbar named "Terminal keys") | gate-e2e.log line 46, test 22, 249ms, listed with the passing mark |
| I-23 | PASS | `e2e/web-share-scope.spec.ts:23` on `firefox`: web-share-scope: web-share /app/ guest sees the scoped session surface, NOT the desktop Sidebar (the test carries the desktop no-key-bar assertion: no toolbar named "Terminal keys") | gate-e2e.log line 74, test 46, 435ms, listed with the passing mark |
| I-24 | PASS | `e2e/web-share-scope.spec.ts:23` on `webkit`: web-share-scope: web-share /app/ guest sees the scoped session surface, NOT the desktop Sidebar (the test carries the desktop no-key-bar assertion: no toolbar named "Terminal keys") | gate-e2e.log line 102, test 70, 389ms, listed with the passing mark |

## VM rows (14-12)

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
