#!/usr/bin/env bash
# g07.sh: row G-07. Notifications while the display is captured: with a guest streaming and TextEdit frontmost
# (Portty not the active app), a second guest knocks (the office's own Safari, a different tailnet node), is
# admitted (join) and leaves. Portty's notification records and Notification Center's banners are read after each
# event; delivered or suppressed is recorded either way. The first guest's picture is checked live at each
# stage: a static screen sends no new frames, so a local key press into TextEdit changes the screen first.
# Usage: g07.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-07 "notifications while the display is captured"

# live LABEL: framesDecoded of the guest VM's page before and after the screen is changed by two untagged key presses.
live() {
  local f1 f2
  r p20 front TextEdit >/dev/null 2>&1; sleep 1
  f1=$(guest_stat framesDecoded)
  r local-key 0 >/dev/null; sleep 0.5; r local-key 0 >/dev/null
  sleep 3
  f2=$(guest_stat framesDecoded)
  ev "guest 1 picture $1: framesDecoded $f1 -> $f2"
}

step "pre: a NEW share (so the office node has not been admitted before) with one admitted guest streaming (guest VM); TextEdit frontmost so Portty is not the active app"
r self-guest-close >/dev/null 2>&1
stop_leftover
ensure_admitted
close_share_sheet
office_textedit
evr "front" p20 ax-front
live "before anything"
ev "notification records and banners before (last 6 records):"
r notif 6 >> "$EV" 2>&1

step "1: a second guest knocks while the display is captured (the office's own Safari, another node)"
r p20 front TextEdit >/dev/null 2>&1
r self-guest-open >/dev/null
ev "pending request: $(r share-status | jq -c '.pending | length')"
ev "Admit sheet in Portty's AX tree: $(wait_office_ax 'Someone Wants to Watch' 20)"
sleep 3
ev "notification records and banners after the knock:"
r notif 6 >> "$EV" 2>&1
live "after the knock"

step "2: Admit pressed (the second guest joins)"
evr "press" press Admit
sleep 8
ev "viewers: $(r share-status | jq -c '[.viewers[] | {nodeName, control}]')"
ev "notification records and banners after the join:"
r notif 6 >> "$EV" 2>&1
live "after the join"

step "3: the second guest leaves (its Safari session ends)"
r self-guest-close >/dev/null
sleep 8
ev "viewers: $(r share-status | jq -c '[.viewers[] | {nodeName, control}]')"
ev "notification records and banners after the leave:"
r notif 6 >> "$EV" 2>&1
live "after the leave"
step "done"
cat "$EV"
