#!/usr/bin/env bash
# g08.sh: row G-08. The tray menu line and the menu-bar icon variant while someone watches. Starts from no
# share (stops a leftover one) so the not-sharing icon can be captured first. Usage: g08.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-08 "tray line and icon variant while someone watches"
EVD="$PHASE_DIR/evidence"

step "pre: no share, Share Desktop sheet closed"
stop_leftover
r guest-close >/dev/null 2>&1
sleep 2
step "1: not sharing: tray menu and the menu-bar icon"
evr "tray" tray items
evr "status item" tray icon
r menubar-shot "$EVD/G-08-normal.png" | sed 's/^/menubar-shot normal: /' >> "$EV"

step "2: sharing, nobody connected yet"
ev "start from the tray, sheet shows Copy Link: $(start_share_from_gui)"
close_share_sheet
evr "tray" tray items
sleep 2
r menubar-shot "$EVD/G-08-nobody.png" | sed 's/^/menubar-shot nobody: /' >> "$EV"

step "3: one guest connected (admitted)"
r share-link >/dev/null
r guest-open >/dev/null
ev "Admit sheet: $(wait_office_ax 'Someone Wants to Watch' 40)"
sleep 1.5
evr "press Admit" press Admit
ev "guest shows View only: $(wait_guest_has 'View only' 60)"
close_share_sheet
sleep 3
evr "tray (one watching)" tray items
evr "status item" tray icon
r menubar-shot "$EVD/G-08-sharing.png" | sed 's/^/menubar-shot sharing: /' >> "$EV"

step "4: pixel comparison of the three crops (cmp: identical or differ)"
for pair in "normal nobody" "normal sharing" "nobody sharing"; do
  set -- $pair
  if cmp -s "$EVD/G-08-$1.png" "$EVD/G-08-$2.png"; then ev "cmp $1 vs $2: identical"; else ev "cmp $1 vs $2: differ"; fi
done
step "done"
cat "$EV"
