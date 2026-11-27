#!/usr/bin/env bash
# g01.sh: row G-01 (tracer). A share started from the Portty GUI on the office; the guest waits at Admit,
# the Admit sheet shows on the office, Admit gives a moving View only picture, Stop Sharing from the tray
# shows Sharing has ended on the guest. Usage: g01.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-01 "tracer: guest waits, is admitted on the office, watches; Stop Sharing from the tray"

step "pre: no share active, Share Desktop sheet closed"
stop_leftover
ev "share-status: $(r share-status)"
r guest-close >/dev/null 2>&1

step "1: the share is started from the GUI (tray item Share Desktop..., AX press; the sheet starts the share)"
evr "tray before" tray items
ev "tray press, then sheet shows Copy Link: $(start_share_from_gui)"
evr "share-status" share-status
ev "sheet text:"
r ax-gui | sed -n '/AXApplicationDialog\] Title=Share Desktop/,/AXButton \[AXCloseButton/p' |
  grep -E 'AXHeading|Value=Link|Copy Link|Watching|Nobody|Stop Sharing|Done' | cut -c1-170 >> "$EV"
evr "tray (sharing, nobody yet)" tray items

step "2: the guest opens the link in Safari on the guest VM (link piped into a 0600 file, never printed)"
evr "link" share-link
evr "guest-open" guest-open
sleep 2
ev "guest-text: $(r guest-text)"

step "3: the Admit sheet on the office (AX tree, names scrubbed)"
ev "sheet appears: $(wait_office_ax 'Someone Wants to Watch' 20)"
r ax-gui | sed -n '/AXApplicationDialog\] Title=Someone Wants/,/AXButton \[AXCloseButton/p' |
  grep -E 'AXApplicationDialog|admit-title|Login name|Device|admit__name|AXButton Title=(Deny|Admit)|admit-p' | cut -c1-200 >> "$EV"

step "4: Admit pressed through AX (after the 600 ms guard)"
sleep 1.5
evr "press" press Admit
ev "guest shows View only: $(wait_guest_has 'View only' 40)"
f1=$(guest_stat framesDecoded); sleep 3; f2=$(guest_stat framesDecoded)
ev "framesDecoded: $f1 then (3 s later) $f2"
ev "guest-text: $(r guest-text)"
evr "tray (one watching)" tray items
evr "share-status" share-status

step "5: Stop Sharing pressed in the tray menu (AX)"
evr "tray press" tray press "Stop Sharing"
ev "guest shows Sharing has ended: $(wait_guest_has 'Sharing has ended' 15)"
ev "guest-text: $(r guest-text)"
evr "share-status" share-status
step "done"
cat "$EV"
