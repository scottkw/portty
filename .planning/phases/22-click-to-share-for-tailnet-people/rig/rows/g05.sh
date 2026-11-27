#!/usr/bin/env bash
# g05.sh: row G-05. Portty's own (tagged) posts never count as physical; one untagged software click
# (~/ptest/click, the honest stand-in for hardware, RESEARCH A.5) makes the controlling guest's typing drop
# with the physical counter up and "Someone is using this Mac" on the guest page; after the lease passes
# typing posts again. Usage: g05.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-05 "physical input wins (untagged stand-in click while a guest controls)"

step "pre: a share with one admitted guest holding control"
ensure_admitted
ensure_control
ev "guest-text: $(r guest-text | jq -r .text | cut -c 150-400)"
ev "share-status viewers: $(r share-status | jq -c '[.viewers[] | {mode, control}] , .anyController')"

step "1: only Portty's tagged posts: guest clicks the text area and types ab"
office_textedit
guest_click_text
p0=$(counter physical); k0=$(counter key)
r p20 safari-type 'ab' >/dev/null
sleep 1.5
p1=$(counter physical); k1=$(counter key)
ev "physical counter $p0 -> $p1 (delta $((p1 - p0))); key counter $k0 -> $k1"
ev "document: $(doc_text)"

step "2: the guest types c 40 times (250 ms apart, about 12 s); an untagged stand-in click at the Mac is fired about 4 s in"
p0=$(counter physical); k0=$(counter key)
# The chip shows for 2-3 s and the rig's calls take seconds, so an in-page observer logs every time the text appears.
OBS='window.__chip = []; new MutationObserver(function () { if (document.body.innerText.indexOf("Someone is using this Mac") >= 0) window.__chip.push(Date.now()); }).observe(document.body, {subtree: true, childList: true, characterData: true, attributes: true}); return "observer on"'
ev "chip observer: $(r p20 safari-js "$OBS")"
ACT=$(jq -nc '{actions:[{type:"key",id:"kb",actions:([range(40)] | map([{type:"keyDown",value:"c"},{type:"keyUp",value:"c"},{type:"pause",duration:250}]) | add)}]}')
r p20 safari-actions "$ACT" >/dev/null &
GP=$!
sleep 4
t1=$(now)
r stand-in-input 650 420 >/dev/null
t2=$(now)
ev "stand-in click fired between office clock $t1 and $t2"
wait $GP
ev "times the guest page showed Someone is using this Mac (observer log, count and span in ms): $(r p20 safari-js 'return JSON.stringify({n: window.__chip.length, spanMs: window.__chip.length ? window.__chip[window.__chip.length - 1] - window.__chip[0] : 0})')"
sleep 1
p1=$(counter physical); k1=$(counter key)
ev "physical counter $p0 -> $p1 (delta $((p1 - p0))); key counter $k0 -> $k1 (40 keys sent = 80 key messages)"
ev "document (fewer than 40 c: the ones inside the physical window were dropped): $(doc_text)"
ev "guest-text: $(r guest-text | jq -r .text | cut -c 150-400)"

step "3: after 4 s without any local input the guest types d"
sleep 4
k0=$(counter key)
r p20 safari-type 'd' >/dev/null
sleep 1.5
k1=$(counter key)
ev "key counter $k0 -> $k1"
ev "document: $(doc_text)"
step "done"
cat "$EV"
