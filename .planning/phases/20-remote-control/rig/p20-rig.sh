#!/usr/bin/env bash
# p20-rig.sh: VM rig for the Phase 20 remote-control rows (20-09), built on p19-rig.sh.
#
# Roles: office = portty-test (the Mac that is controlled: Portty, TextEdit, the probe route);
# home = portty-test-2 (the viewer: Safari through safaridriver, Chrome with DevTools on loopback).
# Every capture, input, AX call, pasteboard access, launchctl and tccutil call runs inside one
# of the two VMs through ssh; the host only builds, signs, compiles the AX helper and writes
# evidence files. Nothing here contacts a 100.x address or the host's tailscaled, and nothing
# stops either VM (a stopped VM drops off the tailnet); network loss is pf only (p19 pf-block).
#
# Host-side builds and Phase 19 helpers (delegated to p19-rig.sh):
#   build20 VERSION OUTDIR   Developer ID build with tags "wailsassets vmdesktop" (the VM-only
#                            probe route). NOT vmpreview: that tag fakes Tailscale (127.0.0.1)
#                            and the desktop refuses to start on it.
#   p19 ARGS...              deploy APP, start-gui, quit-gui, enable-desktop, lock, unlock, ...
#   tools                    compile ax20.swift on the host, install ~/p20/ax20 in both VMs
# Office, the Mac under control:
#   probe OP [JSON]          POST /desktop/debug/vm20 on the daemon socket (ops in
#                            internal/desktop/probe_vmdesktop.go), JSON merged into {"op":OP}
#   textedit-new [X Y W H]   quit TextEdit, start it with one empty document, place it (points)
#   textedit-read            the document text through AX, then the char counts
#   ax-front                 frontmost application and the window list's topmost TextEdit/Spotlight facts
#   ax ARGS...               ~/p20/ax20 ARGS (text, windows, setframe, charrect, selrange, scroll, find, press)
#   pb-set TEXT [--concealed] [--home]   JavaScript for Automation through NSPasteboard
#   pb-get [--home]          pbpaste as the console user (length and sha1 only unless --show)
#   natural-scroll on|off    com.apple.swipescrolldirection, applied for new events
#   tcc-reset-ax             tccutil reset Accessibility app.portty.Portty (revokes, raises no prompt)
#   perm                     GET /desktop/permissions
#   state ROW                p19 state capture, scrubbed, saved as evidence/ROW.state
#   dlog [N]                 the last N (40) daemon.log lines
# Home, the viewer (safaridriver session kept in ~/p19/safari.sid):
#   safari-open-viewer       open the share link with #debug in Safari and keep the session
#   safari-js BODY           run a JS function body in the page, print the returned value
#   safari-point NX NY       viewport point of a normalized point of the shared surface
#   safari-click NX NY       a real WebDriver pointer click there
#   safari-actions JSON      raw WebDriver actions
#   safari-type TEXT         key actions for ASCII text, Shift held for capitals
#   safari-menu LABEL [key]  move to the bar's reveal zone and click the button or menu item LABEL
#   safari-shot OUT [WIDTH]  the decoded video frame as a PNG (fails in the VMs: no IOSurface)
#   front APP                make a running app frontmost (open -a), office
#   safari-stats [SECS]      the debug stats once a second (session kept)
#   chrome-stats [SECS]      p19 chrome-stats
set -euo pipefail

VM_SH="${P20_VM_SH:-$HOME/.local/share/portty-vm-tools/vm.sh}"
VM2_NAME="${P20_VM2_NAME:-portty-test-2}"
VM_NAME="${P20_VM_NAME:-portty-test}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
P19="$ROOT/.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh"
SOCKQ='"$HOME/Library/Application Support/portty/daemon.sock"'

die() { echo "p20-rig: $*" >&2; exit 1; }
vm() { "$VM_SH" "$@"; }
vmsh() { "$VM_SH" bash -s -- "$@"; }
vm2() {
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$VM2_NAME")" "$@"
}
vm2sh() { vm2 bash -s -- "$@"; }
require_vm() {
  tart list | awk -v n="$VM_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' || die "$VM_NAME is not running"
}
require_vm2() {
  tart list | awk -v n="$VM2_NAME" '$2 == n && $NF == "running" {f = 1} END {exit !f}' || die "$VM2_NAME is not running"
}
# scrub: VM home shows as ~; tailnet addresses, names and capability tokens never survive.
scrub() {
  sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' -e 's#cap=[^& "]*#CAP-REDACTED#g'
}

cmd_build20() {
  [ $# -eq 2 ] || die "usage: build20 VERSION OUTDIR"
  bash "$P19" build "$1" "$2" --tags "wailsassets vmdesktop" --sign devid
}

cmd_tools() {
  require_vm
  require_vm2
  local tmp
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/p20-ax.XXXXXX")
  swiftc -O "$HERE/ax20.swift" -o "$tmp/ax20" 2>&1 | grep -v warning || true
  for who in vm vm2; do
    $who 'mkdir -p ~/p20 && cat > ~/p20/ax20 && chmod +x ~/p20/ax20' < "$tmp/ax20"
  done
  rm -rf "$tmp"
  echo "tools: ax20 installed in both guests"
}

cmd_probe() {
  [ $# -ge 1 ] || die "usage: probe OP [JSON]"
  require_vm
  local op=$1 extra=${2:-'{}'}
  jq -nc --arg op "$op" --argjson f "$extra" '{op:$op}+$f' |
    vm "curl -s -m 30 -X POST --unix-socket $SOCKQ --data-binary @- http://localhost/desktop/debug/vm20" | scrub
  echo
}

cmd_textedit_new() {
  require_vm
  local x=${1:-100} y=${2:-80} w=${3:-640} h=${4:-420}
  vmsh "$x" "$y" "$w" "$h" <<'EOF'
# SIGKILL, and both state folders (sandboxed and not) removed, so no earlier text is restored.
pkill -9 -x TextEdit 2>/dev/null; sleep 1
rm -rf "$HOME/Library/Saved Application State/com.apple.TextEdit.savedState" \
  "$HOME/Library/Containers/com.apple.TextEdit/Data/Library/Saved Application State" \
  "$HOME/Library/Containers/com.apple.TextEdit/Data/Library/Autosave Information"
# TextEdit's own autocapitalisation would upper-case a lower-case first letter (it did, in T1): not Portty's doing.
defaults write com.apple.TextEdit NSAutomaticCapitalizationEnabled -bool false
open -a TextEdit
for i in $(seq 1 30); do
  [ "$(~/p20/ax20 windows com.apple.TextEdit 2>/dev/null | grep -c '^window')" -ge 1 ] && break
  sleep 0.5
done
if [ "$(~/p20/ax20 windows com.apple.TextEdit 2>/dev/null | grep -c '^window')" -lt 1 ]; then
  ~/ptest/axmenu com.apple.TextEdit File New >/dev/null; sleep 1
fi
~/p20/ax20 setframe com.apple.TextEdit "$1" "$2" "$3" "$4"
sleep 0.5
~/p20/ax20 windows com.apple.TextEdit
~/p20/ax20 front x
~/p20/ax20 text com.apple.TextEdit | tail -1 | sed 's/^/start document: /'
exit 0
EOF
}

cmd_ax() { require_vm; vm "~/p20/ax20 $*" | scrub; }
cmd_textedit_read() { cmd_ax text com.apple.TextEdit; }

cmd_ax_front() {
  require_vm
  vmsh <<'EOF'
echo "ax-front epoch=$(date +%s) $(~/p20/ax20 front x)"
~/p20/ax20 cgwin x | grep -v -E 'owner=(Window Server|Dock|Finder|Control Center|SystemUIServer|Portty|Notification Center|TextInputMenuAgent)'
exit 0
EOF
}

# front APP: make a running app frontmost the way the Dock would (open -a), inside the office VM.
cmd_front() { require_vm; vm "open -a '$1'; sleep 1; ~/p20/ax20 front x"; }

cmd_pb_set() {
  [ $# -ge 1 ] || die "usage: pb-set TEXT [--concealed] [--home]"
  local text=$1 conceal=0 who=vm
  shift
  for o in "$@"; do
    case "$o" in --concealed) conceal=1 ;; --home) who=vm2 ;; *) die "pb-set: unknown option $o" ;; esac
  done
  $who 'mkdir -p ~/p20 && cat > ~/p20/pbset.js' < "$HERE/pbset.js"
  # The text travels base64 encoded so it never meets a shell on the way.
  $who "osascript -l JavaScript ~/p20/pbset.js $(printf '%s' "$text" | base64) $conceal"
}

cmd_pb_get() {
  local who=vm show=0
  for o in "$@"; do
    case "$o" in --home) who=vm2 ;; --show) show=1 ;; *) die "pb-get: unknown option $o" ;; esac
  done
  if [ $show = 1 ]; then
    $who 'pbpaste'
  else
    $who 'c=$(pbpaste); echo "pbpaste bytes=${#c} sha1=$(printf %s "$c" | shasum | cut -c1-12)"'
  fi
}

cmd_natural_scroll() {
  require_vm
  case "${1:-}" in
    on) v=true ;; off) v=false ;; *) die "usage: natural-scroll on|off" ;;
  esac
  vmsh "$v" <<'EOF'
defaults write -g com.apple.swipescrolldirection -bool "$1"
/System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings -u 2>&1 | head -2
echo "natural scrolling pref now: $(defaults read -g com.apple.swipescrolldirection)"
exit 0
EOF
}

cmd_tcc_reset_ax() {
  require_vm
  vm 'tccutil reset Accessibility app.portty.Portty'
}

cmd_perm() { require_vm; vm "curl -s -m 5 --unix-socket $SOCKQ http://localhost/desktop/permissions"; echo; }

cmd_state() {
  local row=${1:-}
  [ -n "$row" ] || die "usage: state ROW"
  mkdir -p "$PHASE_DIR/evidence"
  bash "$P19" state - | scrub > "$PHASE_DIR/evidence/$row.state"
  echo "state: wrote evidence/$row.state"
}

cmd_dlog() {
  require_vm
  vm "tail -n ${1:-40} \"\$HOME/Library/Application Support/portty/daemon.log\"" | scrub | cut -c1-240
}

# ---- the viewer in the home VM -----------------------------------------------------------------

cmd_safari_open_viewer() { bash "$P19" safari-stats 3 keep; }

cmd_safari_js() {
  [ $# -eq 1 ] || die "usage: safari-js BODY"
  require_vm2
  vm2sh "$(printf '%s' "$1" | base64)" <<'EOF' | jq -r '.value|tostring' | scrub
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
body=$(printf '%s' "$1" | base64 -D)
curl -s -m 60 -X POST $D/session/$SID/execute/sync -d "$(printf '%s' "$body" | jq -Rsc '{script:., args:[]}')"
EOF
}

# The shared surface's picture inside the video element (object-fit: contain), viewport CSS px.
POINT_JS='const v=document.querySelector("video"); const r=v.getBoundingClientRect(); const ar=v.videoWidth/v.videoHeight; let w=r.width,h=r.height; if(w/h>ar){w=h*ar}else{h=w/ar}; const x0=r.x+(r.width-w)/2, y0=r.y+(r.height-h)/2; return JSON.stringify([Math.round(x0+NX*w), Math.round(y0+NY*h)]);'

cmd_safari_point() {
  [ $# -eq 2 ] || die "usage: safari-point NX NY"
  local js=${POINT_JS//NX/$1}
  js=${js//NY/$2}
  cmd_safari_js "$js"
}

safari_actions() {
  require_vm2
  vm2sh "$(printf '%s' "$1" | base64)" <<'EOF' | scrub
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
printf '%s' "$1" | base64 -D | curl -s -m 60 -X POST $D/session/$SID/actions --data-binary @-
echo
EOF
}
cmd_safari_actions() { [ $# -eq 1 ] || die "usage: safari-actions JSON"; safari_actions "$1"; }

cmd_safari_click() {
  [ $# -ge 2 ] || die "usage: safari-click NX NY [BUTTON]"
  local pt x y
  pt=$(cmd_safari_point "$1" "$2")
  x=$(jq -r '.[0]' <<<"$pt"); y=$(jq -r '.[1]' <<<"$pt")
  echo "safari-click viewport=$x,$y"
  safari_actions "$(jq -nc --argjson x "$x" --argjson y "$y" --argjson b "${3:-0}" \
    '{actions:[{type:"pointer",id:"mouse",parameters:{pointerType:"mouse"},actions:[
      {type:"pointerMove",duration:60,x:$x,y:$y,origin:"viewport"},{type:"pointerDown",button:$b},{type:"pause",duration:60},{type:"pointerUp",button:$b}]}]}')" >/dev/null
}

# safari-type TEXT: ASCII only. A capital is sent as Shift down, the letter, Shift up, so the
# key message carries the shift bit (WebDriver's bare "P" would carry none). A space is the
# WebDriver Space key: Safari sends code "Unidentified" for a bare " ", which the daemon drops.
cmd_safari_type() {
  [ $# -eq 1 ] || die "usage: safari-type TEXT"
  safari_actions "$(jq -nc --arg s "$1" '
    {actions:[{type:"key",id:"kb",actions:(
      [$s|explode[]|[.]|implode|
        if test("[A-Z]") then [{type:"keyDown",value:""},{type:"keyDown",value:.},{type:"keyUp",value:.},{type:"keyUp",value:""}]
        elif . == " " then [{type:"keyDown",value:""},{type:"keyUp",value:""}]
        else [{type:"keyDown",value:.},{type:"keyUp",value:.}] end] | add)}]}')" >/dev/null
}

cmd_safari_menu() {
  [ $# -ge 1 ] || die "usage: safari-menu LABEL [key]"
  require_vm2
  # The bar reveals when the pointer is in the top-centre zone; move there, then click the button
  # (an element click) by its visible text or aria-label.
  safari_actions '{"actions":[{"type":"pointer","id":"mouse","parameters":{"pointerType":"mouse"},"actions":[{"type":"pointerMove","duration":50,"x":300,"y":80,"origin":"viewport"},{"type":"pointerMove","duration":80,"x":400,"y":10,"origin":"viewport"},{"type":"pause","duration":300},{"type":"pointerMove","duration":80,"x":420,"y":14,"origin":"viewport"}]}]}' >/dev/null
  vm2sh "$(printf '%s' "$1" | base64)" "${2:-}" <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid); label=$(printf '%s' "$1" | base64 -D)
sleep 0.4
find_el() { curl -s -m 20 -X POST $D/session/$SID/element -d "$(jq -nc --arg x "$1" '{using:"xpath",value:$x}')" | jq -r '.value|objects|to_entries[]|select(.key|startswith("element-"))|.value'; }
base='(//button|//*[@role="menuitem"]|//*[@role="menuitemradio"])'
eid=$(find_el "$(printf '%s[normalize-space(.)="%s" or @aria-label="%s"]' "$base" "$label" "$label")")
[ -n "$eid" ] || eid=$(find_el "$(printf '%s[contains(normalize-space(.),"%s")]' "$base" "$label")")
[ -n "$eid" ] || { echo "safari-menu: no element for [$label]"; exit 1; }
echo "safari-menu: click at guest epoch $(perl -MTime::HiRes=time -e 'printf q(%.2f), time')"
if [ "${2:-}" = key ]; then
  # The Clipboard trigger opens on mousedown and a WebDriver click's click event closes it again: Enter on the focused button opens it once.
  curl -s -m 20 -X POST $D/session/$SID/element/$eid/value -d '{"text":""}' | jq -c '.value'
else
  curl -s -m 20 -X POST $D/session/$SID/element/$eid/click -d '{}' | jq -c '.value'
fi
echo "safari-menu: clicked [$label]"
exit 0
EOF
}

# safari-shot OUT.png [WIDTH]: the viewer's decoded video frame drawn to a canvas (WebKit
# screenshots are blank in the VMs, the video element is not), scaled to WIDTH (640) px wide.
cmd_safari_shot() {
  [ $# -ge 1 ] || die "usage: safari-shot OUT.png [WIDTH]"
  local out=$1 w=${2:-640} url
  url=$(cmd_safari_js "const v=document.querySelector('video'); const c=document.createElement('canvas'); c.width=$w; c.height=Math.round($w*v.videoHeight/v.videoWidth); c.getContext('2d').drawImage(v,0,0,c.width,c.height); return c.toDataURL('image/png')")
  printf '%s' "${url#data:image/png;base64,}" | base64 -D > "$out"
  echo "safari-shot: wrote $(wc -c < "$out" | tr -d ' ') bytes"
}

cmd_safari_stats() { bash "$P19" safari-stats "${1:-10}" reuse; }
cmd_chrome_stats() { bash "$P19" chrome-stats "${1:-20}"; }

sub=${1:-}
[ $# -gt 0 ] && shift
case "$sub" in
  build20) cmd_build20 "$@" ;;
  p19) bash "$P19" "$@" ;;
  tools) cmd_tools ;;
  probe) cmd_probe "$@" ;;
  textedit-new) cmd_textedit_new "$@" ;;
  textedit-read) cmd_textedit_read ;;
  ax-front) cmd_ax_front ;;
  front) cmd_front "$@" ;;
  ax) cmd_ax "$@" ;;
  pb-set) cmd_pb_set "$@" ;;
  pb-get) cmd_pb_get "$@" ;;
  natural-scroll) cmd_natural_scroll "$@" ;;
  tcc-reset-ax) cmd_tcc_reset_ax ;;
  perm) cmd_perm ;;
  state) cmd_state "$@" ;;
  dlog) cmd_dlog "$@" ;;
  safari-open-viewer) cmd_safari_open_viewer ;;
  safari-js) cmd_safari_js "$@" ;;
  safari-point) cmd_safari_point "$@" ;;
  safari-click) cmd_safari_click "$@" ;;
  safari-actions) cmd_safari_actions "$@" ;;
  safari-type) cmd_safari_type "$@" ;;
  safari-menu) cmd_safari_menu "$@" ;;
  safari-shot) cmd_safari_shot "$@" ;;
  safari-stats) cmd_safari_stats "$@" ;;
  chrome-stats) cmd_chrome_stats "$@" ;;
  *) die "usage: $0 build20|p19|tools|probe|textedit-new|textedit-read|ax-front|ax|pb-set|pb-get|natural-scroll|tcc-reset-ax|perm|state|dlog|safari-open-viewer|safari-js|safari-point|safari-click|safari-actions|safari-type|safari-menu|safari-shot|safari-stats|chrome-stats|front" ;;
esac
