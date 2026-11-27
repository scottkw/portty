#!/usr/bin/env bash
# g02.sh: row G-02. View-only guest changes nothing; Allow control (AX press on the viewer row) gives
# "You have control"; a guest click and typed text reach TextEdit. Usage: g02.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-02 "grant control, then a guest click and typing post"

step "pre: a share from the GUI with one admitted guest (view only)"
ensure_admitted
ev "share-status: $(r share-status)"
ev "guest-text: $(r guest-text)"

step "1: view only: empty TextEdit frontmost, guest clicks the text area and types zz"
office_textedit
ev "counters before: $(r counters)"
guest_click_text
r p20 safari-type zz >/dev/null
sleep 1.5
ev "counters after: $(r counters)"
ev "document after the view-only click and typing: $(doc_text)"
ev "guest-text: $(r guest-text)"

step "2: Allow control pressed on the guest's row in the Share Desktop sheet (AX)"
open_share_sheet
ev "viewer row in the sheet:"
r ax-gui | sed -n '/AXApplicationDialog\] Title=Share Desktop/,/AXButton \[AXCloseButton/p' |
  grep -E 'View only|Controlling|Allow control|Allow clipboard|Disconnect|AXCheckBox|AXSwitch' | cut -c1-200 >> "$EV"
evr "press" press 'Allow control'
ev "guest shows You have control: $(wait_guest_has 'You have control' 20)"
ev "share-status viewers: $(r share-status | jq -c '[.viewers[] | {mode, control, clipboard}]')"
ev "guest-text: $(r guest-text)"

step "3: control granted: empty TextEdit frontmost, guest clicks the text area and types Portty 22"
office_textedit
ev "counters before: $(r counters)"
guest_click_text
r p20 safari-type 'Portty 22' >/dev/null
sleep 1.5
ev "counters after: $(r counters)"
ev "document: $(doc_text)"
step "done"
cat "$EV"
