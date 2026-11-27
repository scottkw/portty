#!/usr/bin/env bash
# g04b.sh: G-04 repeat, appended to G-04's evidence. Same two guests (guest VM and the office's own Safari), but
# Stop is sent through the route the tray uses (DELETE /desktop/share) and timed inside the office to the
# millisecond, which takes the AX menu's own latency out. Usage: g04b.sh [RUNS] (default 2)
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
EV="$PHASE_DIR/evidence/G-04.txt"
RUNS=${1:-2}
n=1
while [ "$n" -le "$RUNS" ]; do
  step "repeat $n: a new share with two admitted guests streaming, then Stop through DELETE /desktop/share (timed in the office)"
  r self-guest-close >/dev/null 2>&1
  stop_leftover
  ensure_two_guests
  close_share_sheet
  ev "viewers: $(r share-status | jq -c '[.viewers[] | {nodeName}]')"
  ev "guest VM framesDecoded: $(guest_stat framesDecoded) then $(sleep 2; guest_stat framesDecoded); office guest: $(r self-guest-text | jq -c '.stats.framesDecoded')"
  r stop-timed >> "$EV" 2>&1
  ev "guest VM page: $(r guest-text | jq -r .text | cut -c 150-330)"
  ev "office guest page: $(r self-guest-text | jq -r .text | cut -c 150-330)"
  r self-guest-close >/dev/null
  n=$((n + 1))
done
step "repeat done"
