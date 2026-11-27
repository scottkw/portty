#!/usr/bin/env bash
# g09.sh: row G-09. The Portty GUI process alone is terminated during a share: the daemon ends the share
# (share_stop, reason gui_gone), the guest page says Sharing has ended, the daemon keeps running. The GUI is
# started again afterwards. Usage: g09.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-09 "quitting the GUI ends the share (gui_gone)"

step "pre: a new share with one admitted guest streaming"
stop_leftover
ensure_admitted
close_share_sheet
ev "framesDecoded $(guest_stat framesDecoded) then $(sleep 2; guest_stat framesDecoded)"
ev "share-status: $(r share-status | jq -c '{active, viewers: (.viewers | length)}')"

step "1: SIGTERM to the GUI process only (timed inside the office)"
r gui-quit-timed >> "$EV" 2>&1
ev "guest page: $(wait_guest_has 'Sharing has ended' 12)"
ev "guest-text: $(r guest-text | jq -r .text | cut -c 150-330)"
ev "daemon answers on its socket after the GUI died (permissions route): $(r p20 perm)"
ev "share-status from the daemon: $(r share-status)"
ev "audit (last 6 lines, ts event mode reason): "
r audit-dump 6 | tail -6 >> "$EV"

step "2: GUI started again"
evr "start" start 30
sleep 5
evr "tray" tray items
step "done"
cat "$EV"
