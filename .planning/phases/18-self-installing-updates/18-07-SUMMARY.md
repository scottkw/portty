---
phase: 18-self-installing-updates
plan: 07
subsystem: updater
tags: [macos, tart-vm, rig, rollback, failure-matrix, developer-id, share-link]
status: complete

requires:
  - phase: 18-04
    provides: the Update banner, the Installing/Restarting texts and the warning variant
  - phase: 18-06
    provides: the full installer (mover kinds, finishTrash) on top of 18-02, 18-03, 18-05 and 18-10
provides:
  - "18-VM-CHECKS.md rows R1, R2-A, R2-B, F6, R3, F1, F2, F3, F4, F5, F7 PASS on one final build (e03f4ef1), plus OBSERVED side rows F5b-attempt1, F7-unreadable, F7-race-lost"
  - "evidence/ROW.state captures (and ROW-before.state, ROW-timeline.log) for every row; R3-amfi.log"
  - "internal/daemon/share_vmpreview.go: vmLoopbackWhoIs, so the VM's loopback share link reaches its token check"
  - "HealthTimeout tuning result: 60 s kept (12.1x headroom over the measured 4.95 s)"
affects: [18-09]

tech-stack:
  added: []
  patterns:
    - "Forced failures are made on the destination when the source-side trick is swallowed by the tool (ditto exits 0 on unreadable source files)"

key-files:
  created:
    - .planning/phases/18-self-installing-updates/evidence/R1.state (+ R1-before.state, R1-timeline.log; the same set for F1, F2, F3, F4, F5a, F5b, F7, R3)
    - .planning/phases/18-self-installing-updates/evidence/F6.state (+ F6-before.state, F6-killed.state)
    - .planning/phases/18-self-installing-updates/evidence/F5.state
    - .planning/phases/18-self-installing-updates/evidence/R3-amfi.log
    - .planning/phases/18-self-installing-updates/evidence/R2-A-1803.state (+ -before, -timeline; same for R2-B: 18-03's captures, kept under new names)
    - .planning/phases/18-self-installing-updates/evidence/F5b-attempt1.state, F7-unreadable.state, F7-race-lost.state (+ -before, -timeline)
  modified:
    - internal/daemon/share_vmpreview.go
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md
    - .planning/phases/18-self-installing-updates/evidence/R2-A.state (+ -before, -timeline; same for R2-B: now the final-build captures)

key-decisions:
  - "HealthTimeout stays 60 s: the larger click-to-healthy of R1 (4.95 s) and R2-B (2.67 s) is under half of it; installer.go unchanged"
  - "The vmpreview loopback share server answers the MOB-05 WhoIs gate for 127.0.0.1 callers (VM-only build tag), so R1 checks the real link-token path; release builds are unchanged"
  - "F5(b) forces the staging refusal on ~/Applications, because SIP refuses chflags on /Applications in the guest"
  - "F7 forces the copy failure on the destination (kqueue helper makes the fresh staging folder read-only), because ditto skips unreadable source files and exits 0"
  - "CopyApp is not changed to fail on ditto output: a release build already stops an incomplete copy at the installer's reverify, and failing on any stderr would turn a future ditto warning into failed updates (left to the orchestrator)"

requirements-completed: [UPD-05, UPD-06, UPD-07, UPD-09]

estimate:
  tokens: 65000
actuals:
  tokens: 33500
  tasks: 2
  commits: 3
plan_head_before: 870e549683de9540c6d4f13b03408cfc5bd5a556
plan_head_after: 6e604f5fa47c32d75446636bc3b1dfe3b3aecb17

duration: about 50 min (05:05 to 05:55 UTC)
completed: 2026-11-26
---

# Phase 18 Plan 07: Final VM matrix on one build Summary

**On one final build (`e03f4ef1`), every update-path, rollback and failure row in the VM passes. One click installs 1.8.1 with two ticking sessions: their shells and their 300-line history survive, the share link still answers 302/200 with its token and 401 with a wrong one, and the old app goes to the Trash. A broken 1.8.1 is rolled back in about 3.5 s. A SIGKILLed installer leaves a working Portty, and the sweep cleans up after it. A Developer ID build updates itself to the real notarized v1.2.0 with no grant and no dialog. Each install failure reveals the DMG with its own sentence while Portty keeps running.**

## Result

| Row | Verdict | Key evidence |
| --- | --- | --- |
| R1 | PASS | Click to new GUI 3.34 s, to daemon `v1.8.1` 4.95 s. Same session ids and shell pids. Fresh viewer: `HIST-s1-1`, `HIST-s1-300`, 300 distinct HIST lines; `TICK 0..49`, 0 gaps. Share link 302 / followed 200 / wrong token 401, before and after. `Portty.app 1.8.0` in the Trash. Capture: 1.8.1, 1 GUI, `v1.8.1`, cache_dmgs=0, staging_dirs=0, run_dirs=0. No dialog (0 of 26 axdump samples). |
| R2-A | PASS | W=100.3 s. Rolled back (gui-exited), click-to-rollback 3.52 s. Daemon pid unchanged at `v1.8.0`, same shells. The rolled-back banner stayed (did not revert to `available`). Capture: 1.8.0, `v1.8.0`, 1 GUI. |
| R2-B | PASS | W=100.3 s. Takeover committed to `v1.8.1` (123 ms), then rolled back: click-to-rollback 3.23 s, click-to-healthy 2.67 s. 1.8.0 runs against the 1.8.1 daemon with the same shells. No DEBT-08 dialog. Capture: 1.8.0, `v1.8.1`, 1 GUI. |
| F6 | PASS | The installer was SIGKILLed at +1.76 s, after the swap, while the daemon still answered `v1.8.0`. It had just hit A9 and died before retrying, so no GUI was running. `open` started 1.8.1, which took the sessions over on the same shells. The sweep left both leftovers while they were fresh. With their mtimes set 2 h back, the next launch's sweep trashed the old copy and removed the run folder (`sweep trashed 1, ... run dirs removed 1`). Capture: 1.8.1, 1 GUI, staging_dirs=0, run_dirs=0. |
| R3 | PASS | identity=found, un-granted (App Management `No Items`). Developer ID 1.1.90 to the real v1.2.0: the installer ran from `update/run-*/Portty.app`, and the kernel registered the same-Team AppBundles exception. No `Code=-420` and no AMFI kill. The installer relaunched v1.2.0 itself, and the daemon `v1.2.0` kept the same shells (handoff 9 ms). No dialog, no tccd prompt. Capture: 1.2.0, `v1.2.0`, 1 GUI, run_dirs=0. |
| F1 | PASS | `Portty doesn't have permission to replace itself from this account. The download is shown in Finder.` |
| F2 | PASS | `There isn't enough free disk space to install the update. The download is shown in Finder.` |
| F3 | PASS | `Portty couldn't open the download. The download is shown in Finder.` 0 `mnt-*` left. |
| F4 | PASS | `The new version didn't pass the safety check, so it wasn't installed. The download is shown in Finder.` |
| F5 | PASS | (a) uchg on the bundle: `failed swap`, and the installer relaunched 1.8.0 with sessions and daemon untouched. Banner `Portty couldn't install version 1.8.1. ...`. (b) uchg on `~/Applications`: the **permission** text. This is A2 evidence: an immutable folder already fails `access(W_OK)`. The row also restates the real App Management evidence from P2a, P2d-bundle and R3 (no refusal in any run). |
| F7 | PASS | `Portty couldn't copy the new version. The download is shown in Finder.` No staging dir and no `mnt-*` left. |
| F5b-attempt1 | OBSERVED | SIP refused `chflags uchg /Applications`, so nothing was forced and the click ran a plain update (kept as a record). |
| F7-unreadable | OBSERVED | `ditto` skips an unreadable source file and exits 0. The ad-hoc rig installed the incomplete copy. Real codesign shows that a release build stops it (see Open decisions). |
| F7-race-lost | OBSERVED | The zsh poller lost the race against ditto, so the result was a swap failure. Superseded by F7. |

In every F row, the DMG was revealed: Finder's `updates` window came to the front 0.6 to 1.6 s after the click, and the banner shows "shown in Finder" only after `open -R` succeeded. Its quarantine was back on (`0081;...;Portty;`), and gui_pid_count=1.

**HealthTimeout (O6):** the larger click-to-healthy time of R1 and R2-B is 4.95 s, under 30 s (half of 60 s). The headroom is 12.1x, so HealthTimeout stays at 60 s and `internal/updater/installer.go` is unchanged.

## Task Commits

1. Rule 3 rig fix: `e03f4ef1` fix(18-07): vmpreview loopback share server answers the same-account gate
2. Task 1: `e7499443` docs(18-07): final VM rows R1, R2-A, R2-B, F6 and R3 PASS on e03f4ef1
3. Task 2: `6e604f5f` docs(18-07): final VM failure rows F1-F5 and F7 PASS on e03f4ef1

## Builds (for 18-09, "same build")

`e03f4ef1`, built with `GOTOOLCHAIN=go1.26.11` through `p18-rig.sh build` (`-tags "wailsassets vmpreview"`). The frontend was built first in the worktree with `pnpm install --frozen-lockfile` and `pnpm run build`. The builds are kept in the session scratchpad (`p1807/rig/`, outside the repo):
- `n180/Portty.app` (1.8.0, exec sha256 `8d9d5682f68015f0...`) and `n181/Portty.app` (1.8.1, `646cb1713d6d4b87...`)
- `n1190/` (1.1.90 ad-hoc) and `n1190-devid/` (Developer ID signed by `p18-rig.sh sign`)
- `n181-pv189/` (F4's mismatched build)
- `fakegh`

The VM-side helpers are in the session scratchpad (`p1807/vm/`) (`r7.sh`, `f6.sh`, `fchk.sh`, `f7race.sh`, `poll.sh`, `wins.js`), along with the `f7kq` helper.

## Verification

- Task 1's verify block passed in full: 4 PASS rows, the R1 history check, W and click-to-rollback on both R2 rows, and every capture's header and key lines. R3 PASS shows identity=found, un-granted, 1.2.0, `v1.2.0` and run_dirs=0. There is no 100.x address. `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/updater/` is ok.
- Task 2's verify block passed in full: 6 PASS rows, every capture's header, gui_pid_count=1, installed 1.8.0 for F3, F4, F5 and F7, staging_dirs=0 for F3, F4 and F7, and no 100.x address.
- Extra scan of the rows and every evidence file: no home path, no tailnet DNS name, no `cap=` token and no signer name. The share link was only ever written as `https://127.0.0.1:7444/sessions/ID?cap=TOKEN`, and R3's `spctl` origin line is redacted to `NAME`.
- For the rig fix: `go vet -tags vmpreview` and `-tags 'vmpreview vmwhois'` on `./internal/daemon/` are clean, `go test -run Share ./internal/daemon/` passes with and without `vmpreview`, and `tests/check-brand.sh` is clean.

## Deviations from Plan

**1. [Rule 3 - Blocking] The vmpreview loopback share link could never answer 200.** Since Phase 14's MOB-05 gate, every share route asks tailscaled WhoIs first. A 127.0.0.1 caller is never a tailnet peer, so the link answered 403 for both the right and the wrong token, and the token was never checked. `share_vmpreview.go` now installs `vmLoopbackWhoIs`: a loopback caller counts as this Mac's own user, and any other caller is not found. The capability check is untouched, which R1's wrong-token 401 proves. The file only builds with the vmpreview tag, which check-brand keeps out of release paths. It has no unit test of its own: R1 exercises both outcomes, and a new test file would change 18-09's "+8" Go test count. This was committed before the final builds were made, so every row runs on `e03f4ef1`.

**2. [Rule 3] F5(b) mechanism.** SIP refuses `chflags uchg /Applications` even as root (row F5b-attempt1). The same refusal was forced on `~/Applications` with N running from there. A second, idle 1.8.0 copy left at `/Applications` by `deploy-n` is what the F5 capture's installed_version reads; the row says so.

**3. [Rule 1 finding, Rule 3 workaround] F7 mechanism.** The plan's unreadable file does not fail the copy: `ditto` prints `Permission denied`, skips the file and exits 0 (F7-unreadable). The copy failure was forced on the destination instead, with a kqueue helper that makes the new staging folder read-only, so `ditto` exits 1. CopyApp's code is unchanged (see Open decisions).

**4. [Process] 18-03's R2-A and R2-B captures were overwritten** by the final-build captures, which the plan's verify requires at `evidence/R2-A.state`. The originals were restored from git as `R2-?-1803*` files, and the 18-03 rows now cite those names.

**5. [Process] My capture mistakes.** In R2-B, the helper's 1.8.0 CLI `list --local` without `--keep-daemon` asked the 1.8.1 daemon for a takeover twice. The downgrade wall refused both and the sessions were unaffected; the helper now passes `--keep-daemon`. One `python3` call in the VM raised the macOS "Install Command Line Developer Tools" dialog, which I killed (`kill -9`) right away, between rows; it was not open during any row. The rig wrote the F5b-attempt1, F7-unreadable and F7-race-lost captures as F5b and F7, and I renamed the files with their contents unchanged.

**6. [Process] Banner text.** `axdump` cuts values at 80 characters, so the F rows quote full sentences read with `~/ptest/axdump2`.

## Open decisions and findings for the orchestrator (no STOP)

1. **ditto exits 0 on unreadable source files (F7-unreadable).** CopyApp trusts ditto's exit status, so a staged copy can be incomplete without ErrCopy. A release build is still protected:
   - The installer's reverify runs real `codesign --verify --deep --strict` on the staged copy. A copy of the real v1.2.0 with one resource removed fails it with `a sealed resource is missing or invalid`.
   - An unreadable resource in the mounted app already fails VerifyApp, with `Permission denied`.
   - So the user gets the safety or "couldn't install" sentence instead of "couldn't copy". Nothing is corrupted.

   Options:
   - (a) Keep as is. **Recommended**: the release path is already safe, and this is a wording difference only.
   - (b) Make CopyApp fail when ditto prints anything. This catches skips early, but a future macOS that adds a ditto deprecation warning (as 27.2 did for `hdiutil attach -mountpoint`) would fail every update.
   - (c) After ditto, compare the file counts of the source and the copy (small code, no stderr dependency).
2. **The guest has Gatekeeper assessments disabled** (`spctl --status`: `assessments disabled`, already noted in 17-08). R3's "no Gatekeeper dialog" still holds by design, because the installed copy has no quarantine. 18-02's requested `spctl -a -t exec` on the staged Developer ID copy answered `accepted` and `source=Notarized Developer ID`, but also `override=security disabled`, so "accepted" is not an enforcement result in this VM. **18-09 should know** before R4-f (quarantined ad-hoc mover with Open Anyway): with assessments off, the Open Anyway path may not appear as planned. Real enforcement is M-169 on the office Mac.
3. **F6 leaves one cached DMG.** A killed installer never deletes its DMG, the startup sweep does not look in the update cache, and `updater.Clean` runs only before the next download. One DMG (about 8 MB) stays cached until then. This is deferred.
4. **R3 spends 14.0 s before the installer starts**, against P2d-bundle's 3.9 s before 18-02. The difference is 18-02's CheckStagedApp (real spctl plus the `--version` smoke run, capped at 10 s) and real DMG verification. The banner says "Installing Portty 1.2.0…" the whole time, and no deadline is near.
5. **Put Back (18-05 carry-over):** this was not checked. Driving Finder needs an Automation grant: `osascript` to Finder returned -1743 in this VM, and no grant was made. It remains a manual-check candidate for 18-09.

## Host commands run

- `pnpm install --frozen-lockfile` and `pnpm run build` (frontend), plus `gofmt`, `go vet` and `go test` with `GOTOOLCHAIN=go1.26.11`, and `tests/check-brand.sh`.
- `p18-rig.sh build` (1.8.0, 1.8.1, 1.1.90) and a copy of its build step for F4 (productVersion 1.8.9). wails.json and wailsjs were restored each time, and `git status` was clean.
- `p18-rig.sh sign` on a scratch copy only, and `codesign --verify` / `-dvv` and `plutil` on scratch builds.
- `shasum -c` of the scratch v1.2.0 download from 18-08, and `clang` of the f7kq helper.
- `p18-rig.sh` deploy-n, dmg, feed, state, quit-gui and clean (every effect ran inside the VM through vm.sh), and tar/cat streams into the VM's `~/p18`.

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. No host tailscaled or 100.x address was contacted, and there was no host GUI automation. The v1.2.0 daemon in R3 warmed its certificate against the VM's own tailscaled; that ran inside the guest, and nothing on the host was involved.

## VM end state

- portty-test was never stopped, and portty-test-2 was not touched.
- `/Applications/Portty.app` is the final 1.8.1 rig build of `e03f4ef1`, running with 1 GUI, daemon `v1.8.1` and no sessions.
- Trash and Downloads hold no Portty items. `~/p18` is removed and fakegh is stopped.
- There is no uchg flag on `/Applications` or the bundle, no `/Applications/Locked`, no `~/small` and no attached image except the PKI asset.
- The helper temp files, including the share link file, were removed from `/tmp`. An empty `~/Applications` folder remains.
- Finder's `updates` window is still open.

## Known Stubs

None.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: auth-bypass (VM-only) | internal/daemon/share_vmpreview.go | The vmpreview build answers the MOB-05 WhoIs gate as "same user" for 127.0.0.1 callers. It only exists under the vmpreview tag, which check-brand keeps out of .github, build.sh and scripts, and release builds keep the real WhoIs gate. Same pattern as the accepted vmwhois build (Phase 14 UI-A). |

TESTING.md was not touched. 18-09 owns the Phase 18 manifest. No test file was added or removed here.

## Self-Check: PASSED

- Found: internal/daemon/share_vmpreview.go, 18-VM-CHECKS.md, and evidence/{R1,R2-A,R2-B,F6,R3,F1,F2,F3,F4,F5,F5a,F5b,F7}.state with their -before and -timeline files, plus evidence/F6-killed.state, evidence/R3-amfi.log, the R2-?-1803 files and the three OBSERVED side-row sets.
- Commits e03f4ef1, e7499443 and 6e604f5f are ancestors of HEAD.
- STATE.md, ROADMAP.md and .planning/state.json were not changed (orchestrator instruction).
