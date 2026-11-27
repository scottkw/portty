#!/usr/bin/env bash
# g06.sh: row G-06. Terminal frontmost with Secure Keyboard Entry on and a controlling guest: the
# Typing-at-this-Mac banner shows in Portty within 5 s, remote keys still reach Terminal, and the banner
# clears when Secure Keyboard Entry is off. Usage: g06.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-06 "Secure Event Input banner while a guest controls"

step "pre: a share with one admitted guest holding control"
ensure_admitted
ensure_control
# An open sheet makes the rest of the window inert, so Portty's AX tree shows nothing behind it: close it first.
close_share_sheet
ev "banner before (Typing at this Mac in Portty's tree): $(r ax-has 'Typing at this Mac')"

step "1: Terminal frontmost, Secure Keyboard Entry on (timed inside the office: toggle start to banner seen)"
evr "Terminal frontmost, Secure Keyboard Entry off" secure-input off
r p20 ax setframe com.apple.Terminal 100 80 640 420 >/dev/null
evr "timed toggle" secure-input-timed on
r ax-find 'Typing at this Mac' | cut -c1-260 >> "$EV"
ev "share-status: $(r share-status | jq -c '{secureInput, anyController}')"

step "2: the guest clicks into Terminal and types echo p22ske"
guest_click_text
k0=$(counter key)
r p20 safari-type 'echo p22ske' >/dev/null
sleep 2
k1=$(counter key)
ev "key counter $k0 -> $k1"
ev "Terminal text lines holding the typed string (count): $(r p20 ax text com.apple.Terminal | grep -c 'echo p22ske')"

step "3: Secure Keyboard Entry off (timed: toggle start to banner gone)"
evr "timed toggle" secure-input-timed off
ev "banner now: $(r ax-has 'Typing at this Mac'); share-status $(r share-status | jq -c '{secureInput, anyController}')"
step "done"
cat "$EV"
