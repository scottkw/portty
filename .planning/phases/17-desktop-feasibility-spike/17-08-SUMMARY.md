---
phase: 17-desktop-feasibility-spike
plan: 08
subsystem: desktop-spike
tags: [webrtc, chrome, wails, wkwebview, tailnet, ice-lite, h264, local-network-prompt, feas-01, r12, r13]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-07 guest state: both VMs running and on the tailnet, signed Spike.app in VM office with its grant, the daemon-shape pion serve, viewer.html with the null-frame fix"
provides:
  - "R12 GO: Chrome 158 in VM home decodes the Constrained Baseline stream under the 42e01f SDP over the tailnet, ICE-lite and full ICE alike, 2560 x 1440, no SPS patch needed"
  - "R13 GO: RTCPeerConnection, H.264 receive and the data channel work inside a Wails v2.10.2 web view at wails://wails/, signalling through a Go-side proxy of /offer and /report, ICE-lite and full alike"
  - "Local Network prompt record: Chrome and SpikeShell each raise the macOS prompt under their own name; in the Wails shell, media stays at ICE new while it is unanswered"
  - "spike/desktop/wailsshell (nested module portty-spike-shell) and build.sh --shell: builds, sets bundle id app.portty.spike.shell, signs with the Developer ID identity, removes generated wailsjs"
affects: [17-10, 17-11, 17-12, DESK-02, DVWR-01]

actuals:
  tokens: 10179
  tasks: 3
  commits: 4
plan_head_before: 12c489062adfa70294985c25faed46235b60be2c
plan_head_after: 8d574e8a702e0d948291e103f41e963bb863b766

tech-stack:
  added: []
  patterns: ["a Go-side AssetServer Handler that forwards only POST /offer and /report and appends the token, so the page in the wails:// origin never holds it", "read the page's own state in a GPU-less guest through the accessibility tree (axdump) when the screenshot of a web view is blank"]

key-files:
  created:
    - spike/desktop/wailsshell/main.go
    - spike/desktop/wailsshell/go.mod
    - spike/desktop/wailsshell/go.sum
    - spike/desktop/wailsshell/wails.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r12-chrome.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r12-report-chrome-lite.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r12-report-chrome-full.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r12-frame-chrome-lite.jpg
    - .planning/phases/17-desktop-feasibility-spike/evidence/r13-shell.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r13-report-wails-lite.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r13-report-wails-full.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r13-frame-wails-lite.jpg
  modified:
    - spike/desktop/bundle/build.sh
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R12 ran in VM home, so no row was routed to the host Chrome fallback; 17-11 has no R12 work"
  - "R13 is GO on the proxied wails:// path alone; the --navigate fallback did not run because nothing needed a fallback"
  - "The R13 base row carries the user-accepted SpikeShell substitution and a caveat: the shell's Local Network prompt names SpikeShell, so the Portty app's own prompt text and owner are a Phase 21 (DVWR-01) check"
  - "The pipeline was stopped between Task 1 and Task 3 and started again with a fresh token, instead of being left running through the shell build"

patterns-established:
  - "When a build step fetches new go.sum hashes, compare them with the root go.sum and say which lines are new"

requirements-completed: []  # FEAS-01 stays open: only 17-12 closes it

coverage:
  - id: D1
    description: "R12 Chrome in VM home over the tailnet, ICE-lite and full, with the install check and the Local Network prompt recorded"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 1 verify block: r12-chrome.txt non-empty, report keys (iceConnectionState, framesDecoded, codec), R12-lite and R12-full rows record their shape, R12 has a result, no tailnet address or token in r12-chrome.txt"
        status: pass
    human_judgment: false
  - id: D2
    description: "Signed SpikeShell.app built by build.sh --shell, with the app's CSP meta, nothing generated visible to git or the release tests"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 2 verify block: build.sh --shell, gofmt, golangci-lint 0 issues, CSP present in the shell's index.html, codesign --verify --strict, TeamIdentifier S2K7P43927, bundle id app.portty.spike.shell, no frontend/wailsjs, root go list lists no spike package, release tests TestSER03_NoAutoSavePatterns and TestPRG_OffPath_NoProgressLogic, check-brand"
        status: pass
    human_judgment: false
  - id: D3
    description: "R13 WebRTC in the Wails web view over the tailnet, ICE-lite and full, with the prompt and the SpikeShell substitution recorded"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 3 verify block: Substitution line naming the user acceptance, Not reproduced line, R13 row states the substitution, wails:// in r13-shell.txt, R13-lite and R13-full rows record their shape, lite report framesDecoded > 0, R13 has a result, no tailnet address or token in r13-shell.txt"
        status: pass
    human_judgment: false
  - id: D4
    description: "The two frame images are screenshots or canvas frames of the VM office desktop, nothing of the host"
    verification: []
    human_judgment: true
    rationale: "A person should confirm r12-frame-chrome-lite.jpg and r13-frame-wails-lite.jpg show only the VM office desktop (System Settings Login Items, a Finder window, the command-line-tools dialog) and the guest notification banners; the executor read both and found no host content and no names"

duration: about 35min (the start time was not recorded at launch; taken from the first guest action at 05:29 GMT to the last evidence at 05:39 GMT plus reading and the build)
completed: 2026-11-24
status: complete
---

# Phase 17 Plan 08: Chrome and the Wails web view over the tailnet Summary

**Chrome 158 and a throwaway Wails v2.10.2 shell in VM home both decode the VM office desktop over the tailnet with ICE-lite and full ICE alike; the Wails page runs at wails://wails/ and signals only through a Go-side proxy, and both browsers raise a macOS Local Network prompt under their own name.**

## Performance

- **Duration:** about 35 min (see the frontmatter note)
- **Completed:** 2026-11-25T05:45:00Z (approximate; guest clocks read GMT)
- **Tasks:** 3 of 3
- **Files modified:** 14 (the shell module's four files, build.sh, the matrix, eight evidence files)

## Accomplishments

- **R12 GO (lite and full).** Chrome connected and decoded in both modes: connect 933 ms against 418 ms, first frame 2378 ms against 889 ms, 7 and 6 frames at 2560 x 1440, 0 dropped. The codec entry is `video/H264` with `profile-level-id=42e01f` in both, no profile mismatch, no SPS patch. The timing gap is one run each on a DERP-relayed path with an OS prompt pending on the first, so the row says it is not evidence of an ICE-mode effect. Chrome reports candidate addresses (Safari reported null): prflx to host, UDP port 47801.
- **Chrome install check.** `codesign --verify --strict` fails on the app inside the mounted DMG (extended-attribute detritus), while `--verify --deep` passes there. A `ditto --norsrc --noextattr` copy verified strict and deep, TeamIdentifier EQHXZ8M8AV, notarized; that copy was installed. The check ran before Chrome did.
- **R13 GO (lite and full).** The shell's page ran at `wails://wails/` (the proxy logged host `wails` and referer `wails://wails/` on every call). Final runs: connect 430 ms and 434 ms, first frame 767 ms and 875 ms, 5 frames each at 2560 x 1440, the same H.264 entry as Chrome and Safari, data-channel RTT median 59 ms (so ping and pong replies came back). The custom-scheme POST body reached the Go proxy intact, so the `--navigate` fallback did not run.
- **Local Network prompt, both owners recorded.** Chrome: `Allow "Google Chrome" to find devices on local networks?`; connection and decode did not wait for the answer. SpikeShell: `Allow "SpikeShell" to find devices on local networks?`, body "This will allow the app to discover, connect to, and collect data from devices on your networks." In the shell, with the prompt unanswered, signalling through the Go proxy worked and the report was stored, but ICE stayed at `new` with 0 frames; after Allow and a fresh page load it connected. The grant survived a rebuild and swap of the shell with the same signing identity (another data point for R10).
- **The shell and `build.sh --shell`.** `spike/desktop/wailsshell` is a nested module (`portty-spike-shell`, wails v2.10.2) so neither the root nor the spike module's `./...` compiles it; the proxy serves only POST /offer and /report (4 MiB cap, 10 s timeout, 404 elsewhere). `--shell` copies viewer.html with the app's CSP meta read from `frontend/index.html` (it exits non-zero if the element is missing), builds, sets `app.portty.spike.shell`, signs with the Developer ID identity, and removes `frontend/wailsjs` on exit. Generated output is under git-ignored paths; `git status` lists only the four source files.
- **Cleanup.** Pipeline stopped, LaunchAgent plist and daemon log deleted, token removed from the guest config and the session scratchpad, no spike or shell process in either guest.

## Task Commits

1. **Task 1: R12 Chrome GO for ICE-lite and full** - `bd474250`
2. **Task 2: Wails shell, Go proxy, build.sh --shell** - `418b3724`
3. **Task 2 follow-up: proxy logs host and referer (the origin proof)** - `981a84c0`
4. **Task 3: R13 GO and the substitution record** - `8d574e8a`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `codesign --verify --strict` fails on the mounted Chrome DMG**
- **Found during:** Task 1, install check
- **Issue:** the plan's strict check on `/tmp/chromedmg/Google Chrome.app` returned "resource fork, Finder information, or similar detritus not allowed" in the framework subcomponent. This is the mounted volume's extended attributes, not a bad signature: `--verify --deep` on the same app passed and `codesign -dv` gave TeamIdentifier EQHXZ8M8AV.
- **Fix:** `ditto --norsrc --noextattr` to a staging copy, then strict and deep verification of the copy (passed, notarized), and install of that verified copy. Recorded in r12-chrome.txt and the R12 row. `spctl` said "override=security disabled" because the guest has Gatekeeper assessment off, so its "accepted" is not an enforcement result; the file says so.
- **Files modified:** none (guest only), evidence text in r12-chrome.txt

**2. [Rule 3 - Blocking] `go mod tidy` cannot finish offline for the shell module**
- **Found during:** Task 2
- **Issue:** `go mod tidy` with `GOPROXY=off` fails on test-only dependencies of Wails' dependencies that are not in the module cache.
- **Fix:** `go mod tidy -e` offline, then `wails build` fetched the missing hashes from the Go module proxy, checked against the Go checksum database. The shell's `go.sum` has 81 lines; 16 are not in the root `go.sum` (go-spew, dbus v5.1.0, gorilla/websocket v1.5.3, go-difflib, testify v1.10.0 and their `/go.mod` lines), because the shell resolves Wails' own minimum versions where the root module has newer ones. T-17-SC's "same hashes as the root module" therefore holds for wails v2.10.2 and its direct dependency set but not for these four modules.
- **Files modified:** `spike/desktop/wailsshell/go.mod`, `go.sum`
- **Commit:** `418b3724`

**3. [Rule 2 - Missing critical evidence] Origin of the page was not provable from the first build**
- **Found during:** Task 3, first run
- **Issue:** the proxy logged only the Origin header, which is empty for a same-origin request on the custom scheme, so nothing in the evidence showed the page ran at wails://.
- **Fix:** the proxy log line now includes `r.Host` and `r.Referer()`; rebuilt, redeployed, and the final lite and full runs are from that build.
- **Files modified:** `spike/desktop/wailsshell/main.go`
- **Commit:** `981a84c0`

### Other notes

- **Extra evidence files.** `r12-frame-chrome-lite.jpg` (the viewer's own canvas frame, Chrome's readback works) and `r13-frame-wails-lite.jpg` (a screenshot of VM home, because the shell's readback sent no frame) are not in the plan's file list. Both show the VM office desktop only.
- **Report fields of the Wails runs.** The page at `wails://wails/` has no query string, so its JSON reports `ice: lite`, `row: adhoc`, `tag: viewer` even for the full run. The proxy sets the real values on the calls it forwards, so the file names and the server log (`ice=full`) carry them. The matrix row and r13-shell.txt both say so; I did not edit the report.
- **Prompt owner read from a screenshot, not axdump.** The plan names axdump for the prompt's owner; the prompt is drawn by the system, so I read its text and app name from the guest screenshot. axdump was used for the page state ("done, report stored").
- **Pipeline restart.** The plan allowed leaving the pipeline running from Task 1 into Task 3. Task 2 sits between them and takes minutes, so I stopped it after Task 1 and started it again with a fresh token for Task 3 (T-17-41).
- **Chrome and the first run.** I left the Local Network prompt unanswered for both Chrome runs, so both reports were taken with it pending. A run after Allow, or after Don't Allow, was not made; the row says Don't Allow is not measured.
- **Aside.** The first R12 lite run's JPEG, the matrix and every evidence file were scanned for the token, `ts.net`, 100.x addresses and account names before each commit; none found.

**Total deviations:** 3 auto-fixed (2 Rule 3, 1 Rule 2), 5 notes. **Impact:** none on the plan's answers; the go.sum deviation is a supply-chain point, not a functional one.

## Authentication Gates

None. Both guests were already Running, Online and signed; no tailnet skip was recorded and no re-key checkpoint was raised.

## Issues Encountered

- The first Wails run (prompt pending) looked like a failure: ICE stayed at `new`. The accessibility tree showed the page had negotiated and stored its report, which separated "signalling through the proxy works" from "media blocked behind the prompt".
- Both Chrome and the shell reach VM office through DERP(mia) (RTT about 55 ms), as in 17-07, so the SC1 clause stays open in R01-vm and is not touched here.

## Known Stubs

None.

## Threat Flags

None. The shell's Go handler is the boundary the plan's threat model lists (T-17-19): one configured server, POST /offer and /report only, 4 MiB cap, 10 s timeout, 404 elsewhere.

## Guest end state

Both VMs are running and on the tailnet (do not stop them). VM office: no spike or tcpdump process, no LaunchAgent plist, `agent-B.json` holds a placeholder instead of a token. VM home: `/Applications/Google Chrome.app` (installed, quit), throwaway profile `/tmp/chrome-spike`, `~/ptest/SpikeShell.app` (not running) with its Local Network grant; all three are removed in 17-12 cleanup. The token lived only in the session scratchpad and has been deleted.

## Next Phase Readiness

Ready for 17-10. R12 and R13 are filled (both GO), so 17-11's host Chrome fallback is not needed. FEAS-01 and FEAS-02 stay open: only 17-12 closes them. 17-12 still has to repair the misplaced R15 to R17 rows (deferred-items.md; this plan edited only `## Rows` entries) and should add an M-NN for the real Portty app's Local Network prompt under its own bundle id (Phase 21, DVWR-01).

## Self-Check: PASSED

- Files exist: wailsshell main.go, go.mod, go.sum, wails.json; r12-chrome.txt, r12-report-chrome-lite.json, r12-report-chrome-full.json, r12-frame-chrome-lite.jpg, r13-shell.txt, r13-report-wails-lite.json, r13-report-wails-full.json, r13-frame-wails-lite.jpg.
- Commits `bd474250`, `418b3724`, `981a84c0` and `8d574e8a` are on the branch.
