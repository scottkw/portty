#!/usr/bin/env bash
# desktop-soak.sh: sample the Portty daemon into CSV for a desktop soak (23-11 H-03, the office-Mac soak 23-14).
#
#   desktop-soak.sh [--pid PID] [--samples N] [--interval SECS] [--out FILE] [--no-stats]
#
# One CSV line per interval: ts (UTC), pid, rss_kb (ps), footprint (phys_footprint in bytes, the number that
# matches Activity Monitor; RSS alone hides compressed and shared pages), fds (lsof lines), cpu (ps %cpu),
# and from GET /desktop/stats on the daemon socket: goroutines, viewers, state, audio, quality step and target.
# Counts and states only: no address, link, token or name is ever read or written.
# Defaults: 60 samples, 60 s apart, to stdout. The daemon pid is found from the running Portty executable's
# `daemon` process unless --pid is given. Exits non-zero if the pid disappears (the daemon restarted or crashed).
# --no-stats leaves the stats columns empty (a process that is not the daemon, or a dry run).
# An unreadable stats route writes "-" in those columns and a note on stderr; it never writes a guess.
set -euo pipefail

pid="" samples=60 interval=60 out=/dev/stdout stats=1
while [ $# -gt 0 ]; do
  case "$1" in
    --pid) pid=${2:?--pid needs a value}; shift 2 ;;
    --samples) samples=${2:?--samples needs a value}; shift 2 ;;
    --interval) interval=${2:?--interval needs a value}; shift 2 ;;
    --out) out=${2:?--out needs a value}; shift 2 ;;
    --no-stats) stats=0; shift ;;
    *) echo "usage: $0 [--pid PID] [--samples N] [--interval SECS] [--out FILE] [--no-stats]" >&2; exit 2 ;;
  esac
done

if [ -z "$pid" ]; then
  pid=$(ps -axww -o pid=,args= | awk '$2 ~ /\/Portty$/ && $3 == "daemon" {print $1; exit}')
  [ -n "$pid" ] || { echo "desktop-soak: no Portty daemon process found; pass --pid" >&2; exit 1; }
fi
sock="$HOME/Library/Application Support/portty/daemon.sock"

echo "ts,pid,rss_kb,footprint,fds,cpu,goroutines,viewers,state,audio,step,target_bps" > "$out"
for i in $(seq 1 "$samples"); do
  kill -0 "$pid" 2>/dev/null || { echo "desktop-soak: pid $pid is gone at sample $i" >&2; exit 1; }
  rss=$(ps -o rss= -p "$pid" | tr -d ' ')
  cpu=$(ps -o %cpu= -p "$pid" | tr -d ' ')
  fp=$(footprint -p "$pid" --noCategories -f bytes 2>/dev/null | sed -nE 's/^ *phys_footprint: ([0-9]+) B.*/\1/p' | head -1)
  fds=$(lsof -n -P -p "$pid" 2>/dev/null | tail -n +2 | wc -l | tr -d ' ')
  g="" v="" s="" a="" st="" t=""
  if [ "$stats" = 1 ]; then
    if j=$(curl -s -m 5 --unix-socket "$sock" http://localhost/desktop/stats) && [ -n "$j" ]; then
      read -r g v s a st t < <(printf '%s' "$j" | jq -r '[.goroutines, .viewers, .state, .audio, .quality.step, .quality.target_bps] | map(tostring) | join(" ")')
    else
      echo "desktop-soak: stats route unreadable at sample $i" >&2
      g=- v=- s=- a=- st=- t=-
    fi
  fi
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ),$pid,$rss,${fp:--},$fds,$cpu,$g,$v,$s,$a,$st,$t" >> "$out"
  [ "$i" -lt "$samples" ] && sleep "$interval"
done
exit 0
