#!/usr/bin/env bash
# common.sh: helpers sourced by the Phase 22 row scripts (rig/rows/g*.sh). A row script writes one
# scrubbed evidence file, evidence/ROW.txt, through ev/evcmd; every command goes through p22-rig.sh,
# so every capture, input and AX call runs in a VM and every output is scrubbed.
set -uo pipefail
ROWS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RIG_DIR="$(dirname "$ROWS_DIR")"
PHASE_DIR="$(dirname "$RIG_DIR")"
RIG="$RIG_DIR/p22-rig.sh"
r() { bash "$RIG" "$@"; }

row_begin() { # ROW TITLE
  ROW=$1
  EV="$PHASE_DIR/evidence/$ROW.txt"
  mkdir -p "$PHASE_DIR/evidence"
  : > "$EV"
  ev "$ROW $2"
  ev "office=portty-test guest=portty-test-2; build 22.12.0 of HEAD dfa1a208 (tags wailsassets vmdesktop, Developer ID); times are the office VM's epoch seconds"
}
ev() { printf '%s\n' "$*" >> "$EV"; }
now() { r clock; }
step() { ev "--- $* [office $(now)]"; }
# evcmd LABEL CMD...: run CMD (a p22-rig subcommand), put its output under LABEL.
evcmd() { local label=$1; shift; ev "$label: $("$@" 2>&1)"; }
# evr LABEL RIG-ARGS...
evr() { local label=$1; shift; ev "$label: $(r "$@" 2>&1)"; }
sleep_ev() { ev "(wait $1 s)"; sleep "$1"; }
# counter NAME: the Manager counter NAME now (0 when absent).
counter() { r counters | jq -r --arg n "$1" '(.counters[$n] // 0)'; }
# guest_stat FIELD: one field of the guest page's debug stats.
guest_stat() { r guest-text | jq -r '.stats.'"$1"' // "none"' 2>/dev/null; }
guest_has() { r guest-text | jq -r '.text' | grep -qF -- "$1"; }
# wait_guest_has TEXT SECS: poll the guest page until TEXT shows; prints seconds waited or "timeout".
wait_guest_has() {
  local t=0
  while [ $t -lt "$2" ]; do
    if guest_has "$1"; then echo "seen after ${t}s"; return 0; fi
    sleep 1; t=$((t + 1))
  done
  echo "timeout after ${2}s"; return 1
}
# ensure_admitted: a share running from the GUI and one guest admitted (view only). Starts one when needed.
ensure_admitted() {
  local st
  st=$(r share-status)
  if [ "$(jq -r '.active and (.viewers | length) >= 1' <<<"$st")" = true ]; then return 0; fi
  stop_leftover
  r guest-close >/dev/null 2>&1
  start_share_from_gui >/dev/null
  r share-link >/dev/null
  r guest-open >/dev/null
  wait_office_ax 'Someone Wants to Watch' 40 >/dev/null
  sleep 1.5
  r press Admit >/dev/null
  wait_guest_has 'View only' 60 >/dev/null
}
# ensure_two_guests: the guest VM admitted, plus the office's own Safari admitted as a second guest (the
# office is its own tailnet node, a different node from the guest VM; two windows of one device cannot hold
# two steady viewers, see evidence/X-same-node-windows.txt).
ensure_two_guests() {
  ensure_admitted
  if [ "$(r share-status | jq -r '.viewers | length')" -ge 2 ]; then return 0; fi
  r self-guest-close >/dev/null 2>&1
  r self-guest-open >/dev/null
  local i st
  for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    st=$(r share-status)
    [ "$(jq -r '.viewers | length' <<<"$st")" -ge 2 ] && return 0
    # a node admitted earlier in this share is re-admitted without a knock; a new one waits at Admit
    if [ "$(jq -r '.pending | length' <<<"$st")" -gt 0 ]; then sleep 1.5; r press Admit >/dev/null; fi
    sleep 2
  done
  return 1
}
# open_share_sheet: the Share Desktop sheet through the tray line, when it is not open already.
open_share_sheet() {
  if [ "$(r ax-has 'Allow clipboard')" != yes ]; then r tray press "Desktop shared with" >/dev/null; sleep 2; fi
}
# ensure_control: the admitted guest holds control (the Allow control switch pressed in the sheet).
ensure_control() {
  # The daemon's own answer decides (the switch is a toggle: pressing it with control already on would revoke it).
  if [ "$(r share-status | jq -r '[.viewers[] | select(.mode == "guest")][0].control')" = true ]; then return 0; fi
  open_share_sheet
  r press 'Allow control' >/dev/null
  wait_guest_has 'You have control' 20 >/dev/null
}
# office_textedit: an empty TextEdit document at 100,80 640x420 and frontmost (display 1280x720 points).
office_textedit() { r p20 textedit-new 100 80 640 420 >/dev/null 2>&1; r p20 front TextEdit >/dev/null 2>&1; }
# guest_click_text: a guest click at office point (400,300), inside TextEdit's text area.
guest_click_text() { r p20 safari-click 0.3125 0.4167 >/dev/null; }
doc_text() { r p20 textedit-read | tail -2 | tr '\n' ' '; }
# close_share_sheet: close the Share Desktop sheet the way a person does (Not Now or Done), when it is open.
close_share_sheet() {
  if [ "$(r ax-has 'Not Now')" = yes ]; then r press 'Not Now' >/dev/null
  elif [ "$(r ax-has 'Share Desktop')" = yes ] && [ "$(r ax-has 'Done')" = yes ]; then r press 'Done' >/dev/null; fi
  sleep 1
}
# stop_leftover: end a share that an earlier row left running.
stop_leftover() {
  if [ "$(r share-status | jq -r .active)" = true ]; then r tray press "Stop Sharing" >/dev/null; sleep 3; fi
  close_share_sheet
}
# start_share_from_gui: tray item Share Desktop... (sheet starts the share when the CT notice was acknowledged).
start_share_from_gui() {
  r tray press "Share Desktop" >/dev/null
  wait_office_ax 'Copy Link' 25
}
wait_office_ax() { # WORDS SECS
  local t=0
  while [ $t -lt "$2" ]; do
    if [ "$(r ax-has "$1")" = yes ]; then echo "seen after ${t}s"; return 0; fi
    sleep 1; t=$((t + 1))
  done
  echo "timeout after ${2}s"; return 1
}
