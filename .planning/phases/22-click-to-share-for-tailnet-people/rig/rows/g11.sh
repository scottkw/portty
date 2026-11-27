#!/usr/bin/env bash
# g11.sh: row G-11 (Phase 20 M-05 carried in, fixed in 22-05). One TextEdit window shared to a controlling
# guest. Two panels are opened locally by untagged software keys: Spotlight (a window of another process)
# and TextEdit's own Fonts panel (a panel of the shared app that takes keyboard focus). Guest keys must be
# dropped while a panel holds keyboard focus (nothing typed anywhere) and reach the window afterwards.
# Usage: g11.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
row_begin G-11 "window share: keys dropped while a panel holds keyboard focus (M-05)"

# deltas NAME...: the counters before and after are in B0 and B1 (JSON); print name before->after.
deltas() { local n out=""; for n in "$@"; do out="$out $n $(jq -r --arg n "$n" '.counters[$n] // 0' <<<"$B0")->$(jq -r --arg n "$n" '.counters[$n] // 0' <<<"$B1");"; done; echo "$out"; }

step "pre: a share with one admitted guest holding control, TextEdit window shared"
ensure_admitted
ensure_control
office_textedit
ID=$(r probe list | jq -r '[.surfaces[] | select(.app == "TextEdit" and .on_screen == true and .title == "Untitled")][0].id')
ev "TextEdit document window id from the probe list (on screen, titled Untitled): $ID"
# (bash 3.2 mis-parses escaped quotes inside $( ) inside "...": the JSON goes through a variable)
SELJSON="{\"kind\":\"window\",\"id\":$ID}"
SEL=$(r probe select "$SELJSON")
ev "select window: $(jq -c '{select_failed, surface}' <<<"$SEL")"
sleep 5
ev "guest-text: $(r guest-text | jq -c '{title, stats: {status: .stats.status, w: .stats.frameWidth, h: .stats.frameHeight}}')"

step "1: baseline: guest clicks the window and types k1"
r p20 safari-click 0.5 0.5 >/dev/null
r p20 safari-type k1 >/dev/null
sleep 1.5
ev "document: $(doc_text)"
ev "counters: $(r counters)"

step "2: Spotlight opened locally (untagged Cmd+Space through ~/p22/key), then 3 s idle, guest types zz"
r local-key 49 cmd >/dev/null
sleep 3
ev "windows at any layer (the panel owner/layer): $(r p20 ax cgwin x | grep -E 'owner=(Siri|Spotlight)' | head -3)"
ev "frontmost: $(r p20 ax-front | head -1)"
B0=$(r counters)
r p20 safari-type zz >/dev/null
sleep 2
B1=$(r counters)
ev "counter changes (key, window_key_focus, raising, not_held, physical):$(deltas key window_key_focus raising not_held physical)"
ev "guest notice: $(r guest-text | jq -r .text | cut -c 150-400)"
ev "document (no zz expected): $(doc_text)"
r local-key 53 >/dev/null; sleep 1; r local-key 53 >/dev/null; sleep 3
ev "Spotlight windows after two Escapes: $(r p20 ax cgwin x | grep -cE 'owner=(Siri|Spotlight)')"
r p20 front TextEdit >/dev/null 2>&1; sleep 2
B0=$(r counters)
r p20 safari-type k2 >/dev/null
sleep 2
B1=$(r counters)
ev "panel closed, guest types k2: counter changes (key, window_key_focus, raising, physical):$(deltas key window_key_focus raising physical)"
ev "document (k1k2 expected, no zz): $(doc_text)"

step "3: TextEdit's own Fonts panel opened locally (untagged Cmd+T), it takes keyboard focus; guest types zz"
for try in 1 2 3; do
  r p20 front TextEdit >/dev/null 2>&1; sleep 1
  r local-key 17 cmd >/dev/null
  sleep 3
  panel=$(r p20 ax cgwin x | grep -c 'owner=TextEdit pid=[0-9]* layer=3 ')
  ev "Cmd+T try $try: TextEdit layer-3 windows: $panel"
  [ "$panel" -ge 1 ] && break
done
ev "TextEdit windows at any layer (the panel is layer 3): $(r p20 ax cgwin x | grep 'owner=TextEdit' | tr '\n' ';')"
B0=$(r counters)
r p20 safari-type zz >/dev/null
sleep 2
B1=$(r counters)
ev "counter changes (key, window_key_focus, raising, not_held, physical):$(deltas key window_key_focus raising not_held physical)"
ev "document (no zz expected): $(doc_text)"

step "4: TextEdit quit to remove the Fonts panel (Cmd+W did not close it in an earlier run)"
r kill-app TextEdit >/dev/null

step "restore: back to a display share"
DID=$(r probe list | jq -r '[.surfaces[] | select(.kind == "display")][0].id')
SELJSON="{\"kind\":\"display\",\"id\":$DID}"
SEL=$(r probe select "$SELJSON")
ev "select display $DID: $(jq -c '{select_failed, surface}' <<<"$SEL")"
step "done"
cat "$EV"
