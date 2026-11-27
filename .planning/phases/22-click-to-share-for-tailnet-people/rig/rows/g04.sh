#!/usr/bin/env bash
# g04.sh: row G-04. Stop Sharing with two guest viewers mid-frame (the guest VM and the office's own Safari, two
# different tailnet nodes): both leave within 2 s of the share_stop, both pages say Sharing has ended, and the
# old link no longer admits anyone. Usage: g04.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-04 "Stop Sharing with two guests mid-frame"

step "pre: a share with two admitted guests streaming"
ensure_two_guests
ev "viewers: $(r share-status | jq -c '[.viewers[] | {nodeName, control}]')"
ev "guest VM framesDecoded: $(guest_stat framesDecoded) then $(sleep 2; guest_stat framesDecoded)"
ev "office guest: $(r self-guest-text | jq -c '{title, viewers: .stats.viewers, fd: .stats.framesDecoded}')"
sleep 2
ev "office guest 2 s later: $(r self-guest-text | jq -c '{fd: .stats.framesDecoded}')"
close_share_sheet

step "1: Stop Sharing pressed in the tray"
T1=$(now)
evr "tray press" tray press "Stop Sharing"
T2=$(now)
ev "press began at office epoch $T1 (UTC $(date -u -r "${T1%.*}" +%H:%M:%S)) and returned at $T2 (UTC $(date -u -r "${T2%.*}" +%H:%M:%S))"
sleep 3
ev "daemon log lines for the two leaves (ms): "
r p20 dlog 14 | grep -E "viewer (joined|left)" | tail -4 >> "$EV"
ev "guest VM page: $(r guest-text | jq -r .text | cut -c 150-330)"
ev "office guest page: $(r self-guest-text | jq -r .text | cut -c 150-330)"
evr "share-status" share-status
ev "audit (last 8 lines, ts event mode reason):"
r audit-dump 8 | tail -8 >> "$EV"

step "2: the old link after the stop (both pages reloaded)"
r p20 safari-js 'location.reload(); return "reloaded"' >/dev/null
sleep 5
ev "guest VM after reload: $(r guest-text | jq -r '.text' | cut -c 1-300)"
r self-guest-close >/dev/null
step "done"
cat "$EV"
