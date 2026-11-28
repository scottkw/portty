---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 11
subsystem: desktop
tags: [hardening, soak, upgrade, handoff, relock, vm-rig, sampler]
requires:
  - phase: 23-10
    provides: p23-rig.sh unlock and lock helpers, the re-lock rows, 23-VM-CHECKS.md and the VM permissions
  - phase: 19-15
    provides: the F5 upgrade procedure (GUI quit only, app replaced, GUI start takes the daemon over) and the tick-gap reading
provides:
  - "scripts/desktop-soak.sh: 60 s daemon sampler (pid, rss, phys_footprint, fds, cpu, goroutines, viewers, state, audio, quality step and target) for the VM and the office-Mac soaks"
  - "H-00..H-03 results with evidence: a two-build upgrade with a live viewer and live sessions, re-lock surviving the upgrade, a one-hour soak"
  - "rig subcommands: build-pair, tick-start, term-open, state-h, h01-watch, upgrade, h01-run, relock-watch, soak, soak-collect, soak-motion, soak-verdict"
affects: [23-12, 23-14, 23-15, 23-16]
tech-stack:
  added: []
  patterns:
    - "footprint (phys_footprint) and lsof counted next to RSS: RSS alone hides compressed and shared pages"
    - "two version stamps of one tree as the N and N+1 of an upgrade rig"
    - "pass rule computed from the evidence CSV by a rig subcommand, not by eye"
key-files:
  created:
    - scripts/desktop-soak.sh
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/js/readterm23.js
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/js/audpoll.js
  modified:
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/p23-rig.sh
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/23-VM-CHECKS.md
key-decisions:
  - "The upgrade is the Phase 19 F5/S5 path (GUI quit only, app replaced, GUI started), not the fake update feed: the builds carry the desktop tag, and the feed needs the preview tag"
  - "Both guests were stepped to NTP once (898.8 s behind) before any measured row, so no clock step can land inside a row"
  - "H-02 was run with the Portty window left frontmost as the GUI start leaves it; the chord locked the Mac, so no Finder-in-front rerun was needed"
duration: 3h
completed: 2026-11-28
status: complete
estimate_tokens: 70000
actuals:
  tokens: 70000
  tasks: 3
  commits: 3
plan_head_before: 84f2bf6da42eed82bf393d14ff5c24e81b459751
plan_head_after: b45678df6778d0e6a569e10ad6249bab98f1d814
---

# Phase 23 Plan 11: Hardening rehearsal in the VMs Summary

**An upgrade with a viewer streaming and two shells ticking keeps both shells, their history and counter and the viewer's link (reconnect about 5 s, sound back), an armed re-lock survives it, and an hour of motion and idle with a viewer ends on the same daemon pid with flat resources (RSS ratio 1.036, footprint 1.059 against the 1.15 limit, goroutines 65 to 71). All four rows PASS.**

## Row results

| Row | Result | Numbers |
|-----|--------|---------|
| H-00 | PASS | One sampler run in portty-test with a viewer: pid 60566, rss 68144 kB, footprint 24.8 MB, 44 fds, cpu 3.0, goroutines 65, viewers 1, streaming, audio ok, step 0, target 8000000 |
| H-01 | PASS | Shell pids 61124 and 61153 before and after; terminal page buffer HIST 1 to 300 and TICK 1 to 197 with breaks 0 on 200 of 200 reads; longest tick gap 2 s (epoch seconds); desktop `reconnecting` on 7 reads (about 5 to 6 s), same page (load origin constant), muted false throughout, audio packets back at about 50 a second; handoff moved 2 sessions in 165 ms |
| H-02 | PASS | `relock_armed` true before, during and after the swap (daemon.log: re-lock was armed before this start; it is armed again); viewer closed 07:19:49, `relocked` line and loginwindow lock event 31 s later; the daemon's locked reading flipped at 41 s (10 s lag, see findings) |
| H-03 | PASS | 62 samples, 07:23:29 to 08:24:37 UTC, pid 64837 throughout, streaming and viewers 1 at all 62; RSS ratio 1.036 and footprint ratio 1.059 (limit 1.15); fds 43 to 46, goroutines 65 to 71 (warm-up ranges 43 to 45 and 65 to 67); no WARN or ERROR, one join and no leave in daemon.log |

## What was built

- `scripts/desktop-soak.sh`: `--pid`, `--samples` (62 in the rehearsal, 60 default), `--interval` (60 default), `--out`, `--no-stats`. CSV header `ts,pid,rss_kb,footprint,fds,cpu,goroutines,viewers,state,audio,step,target_bps`. Footprint is `phys_footprint` from `footprint -p PID --noCategories -f bytes`, fds are `lsof` lines, the stats come from `curl --unix-socket` on `GET /desktop/stats`. It writes counts and states only, exits non-zero when the pid disappears, and writes `-` (never a guess) when the stats route cannot be read. On the host only `bash -n` and a dry run on the test shell's own pid with `--no-stats`.
- Usage line for 23-12 and 23-14 (office Mac): `bash scripts/desktop-soak.sh --samples 62 --interval 60 --out soak.csv` while a viewer is connected and the screen moves 5 minutes then rests 5 minutes; the pass rule is `rig/p23-rig.sh soak-verdict` (warm-up minutes 5 to 15, RSS and footprint at most 1.15 times the median, fds within 5, goroutines within 10, same pid, state streaming at every sample).
- Rig: `build-pair`, `tick-start` (two sessions, one with 300 history lines then a counter, session 1 shared to the home VM), `term-open`, `state-h`, `h01-watch`, `upgrade`, `h01-run`, `relock-watch`, `soak`, `soak-collect`, `soak-motion`, `soak-verdict`; `rig/js/readterm23.js` (xterm buffer reader with history and tick contiguity) and `rig/js/audpoll.js` (audio packet counter).

## Findings

None needed a fix; none blocks the release.

1. **The session lock flag lags the lock by about 10 s with no load (H-02).** loginwindow got the lock event 31.6 s after the leave; `GET /desktop` read locked at 41 s. U-10r saw 11 s under load, U-10 about 2 s. Anything that reads the flag right after the daemon's lock can see unlocked for about 10 s. Not separated: macOS setting the flag late, or the daemon's reading.
2. **`motion off` takes about two minutes** (Terminal quit over osascript), so the soak ran 5 minutes of motion and about 7 without it, five idle windows and six bursts in 61 minutes. A Terminal process also came back after the last off. For an exact 5 and 5, stop the stream with `pkill -x yes`.
3. **The guests were 898.8 s behind NTP at the start** (the host had been paused while the previous cycle ended). Both were stepped once with `sudo sntp -sS` before any measured row. Future plans should read `sntp` in each guest first.
4. **The leaks recorded in 23-09 and 23-10 are gone on this build.** Goroutines held 65 to 71 for an hour with a viewer (23.9.0 left about 16 to 20 per join and leave); footprint 23 to 30 MB.
5. **Upgrade timing.** After the GUI start the new daemon answered /health within 3 s and the launchd job within 6 s; the viewer was unreachable for about 5 to 6 s in H-01 and 3 s in H-02.

## Deviations from Plan

### Auto-fixed Issues

None.

### Choices within the plan

- **Upgrade path.** The plan says "through the updater path the Phase 18 rig uses". That rig's fake feed needs a build tag these builds do not carry (they are built with the desktop tag, and check-brand keeps preview tags out of scripts). The upgrade used the Phase 19 F5b/S5 sequence instead, which is the same sequence the in-app updater ends in (replace the app, start the GUI, take over the daemon). Recorded in the H-01 row.
- **Re-install of N for H-02.** H-01 left N+1 installed, so H-02 started from a Quit All deploy of N (fresh daemon, no sessions) and then upgraded to N+1 again.
- **H-03 schedule.** See finding 2.

## Authentication Gates

None. No permission prompt or system dialog appeared in either VM; nothing was clicked on the user's behalf (recorded under User checkpoint in 23-VM-CHECKS.md). Only the VM's public admin password was typed.

## Known Stubs

None.

## Threat Flags

None. The sampler reads counts and states only (T-23-28); H-01 and H-02 ran the full upgrade with everything live and both pass (T-23-29). `bash tests/check-brand.sh` was clean before each commit.

## VM and tailnet state at the end

portty-test: build AF 23.11.1, daemon pid 64837 (launchd), unlocked, no viewer, no Terminal, no `yes`, permissions `{"sr":true,"ax":true}`, tailscaled Running. portty-test-2: Safari viewer closed, on the tailnet, no dummynet pipe (not used here); its Portty.app stays 23.10.5 (it is only a viewer in this plan). Neither VM was stopped. The host's `caffeinate -i` (its own pid) was killed after the soak.

## Self-Check: PASSED

- scripts/desktop-soak.sh, rig/js/readterm23.js, rig/js/audpoll.js, evidence/H-00.csv, H-01.watch, H-01.stats, H-02.txt, H-03.csv, H-03.verdict: present.
- Commits 229415f6, 9396208a and b45678df are on this branch (`git merge-base --is-ancestor` verified); `grep -cE '^\| H-0[0-3] .*RESULT=PASS'` on 23-VM-CHECKS.md reads 4.
