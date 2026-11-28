#!/usr/bin/env bash
# p23-rig.sh: VM rig for the Phase 23 rows (23-09 audio and adaptive quality, then 23-10 and 23-11),
# built on p22-rig.sh, p20-rig.sh and p19-rig.sh.
#
# Roles: office = portty-test (the shared Mac: Portty GUI and daemon, afplay, the stats route);
# home = portty-test-2 (the viewer: Safari through safaridriver, Chrome, the Portty app for the tab row).
# Every capture, playback, AX call, input post and traffic-shaping call runs inside one of the two VMs
# through ssh; the host only builds (compile only), generates a test tone file, copies and writes scrubbed
# evidence. Nothing here contacts a 100.x address or the host's tailscaled, and nothing stops a VM.
# The desktop link travels office -> home through the p19 enable-desktop pipe into ~/p19/link.txt (0600)
# and is never printed.
#
# Build and install:
#   build VERSION OUTDIR     p22 build22 (tags "wailsassets vmdesktop", Developer ID); compile only on the host
#   deploy APP               office: quit GUI and daemon, replace /Applications/Portty.app
#   deploy-home APP          home: quit Portty, replace /Applications/Portty.app (the tab row needs the app there)
#   start                    office: open the GUI, wait for /health
#   tools23                  install the AX helper ~/p22/axp22 and the key poster ~/p22/key in the home VM (compiled on the host, compile only)
#   ax-home VERB BUNDLE WORDS  the AX helper in the home VM (find, press, has), scrubbed
# Office:
#   stats                    GET /desktop/stats on the daemon socket, compact
#   stats-log SECS ROW       one stats line a second for SECS seconds, saved as evidence/ROW.stats
#   stats-key                the stats key set (closed list check input)
#   tone-make OUT            host: a 5 s left-only 440 Hz 48 kHz stereo WAV (a file, nothing is played)
#   tone-copy FILE           copy a WAV into ~/p23/tone.wav in the office
#   tone-play                afplay ~/p23/tone.wav in the office, in the background; prints its start epoch
#   motion on|off            Terminal printing a line stream (a moving screen) in the office
#   audio-fault              restart coreaudiod in the office (sudo inside the VM)
#   still-timed              stop the motion stream, poll `still` every 0.25 s, print the delay
#   clock                    the office's epoch seconds
# Home (the viewer VM) and the office's own Safari (WHO = home|office):
#   open-viewer WHO          enable-desktop (link to both VMs), then Safari at link#debug (session kept)
#   close-viewer WHO         end WHO's Safari session
#   vstats WHO               the page's debug stats once
#   js WHO NAME [MS]         run rig/js/NAME.js (an async function body) in WHO's viewer page; MS replaces MS
#   vlog WHO SECS ROW        one line a second: status, frames, width, Reconnecting or Slow connection text, load origin
#   hook WHO                 capture the page's RTCPeerConnection (window.__pc) through the debug poll
#   pcstats WHO              receive-side counts, codec channels, transport-cc in the offer
#   levels WHO MS            left and right energy of the received sound over MS ms
#   page WHO                 title and visible text of the viewer page, scrubbed
#   click-label WHO LABEL    click the button whose aria-label is LABEL (a real WebDriver element click)
#   label-has WHO LABEL      yes|no: a button with that aria-label exists
#   chrome-open              home: Chrome (throwaway profile, web AX tree on) on the desktop link (#debug)
#   chrome-close             home: quit Chrome and remove its throwaway profile
#   ax-safari [ROW]          home: Safari's accessibility tree, scrubbed (also saved as evidence/ROW.ax)
# Shaping (home VM only, dummynet through pf anchor com.apple/p23, tailnet address resolved in the VM):
#   shape on KBIT            a pipe of KBIT kbit/s on traffic from the office to the home VM
#   shape off                flush the anchor and the pipes, print dnctl list (empty) and pf state
#   shape list               dnctl list and the anchor's rules
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(dirname "$HERE")"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
P19="$ROOT/.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh"
P20="$ROOT/.planning/phases/20-remote-control/rig/p20-rig.sh"
P22="$ROOT/.planning/phases/22-click-to-share-for-tailnet-people/rig/p22-rig.sh"
VM_SH="${P23_VM_SH:-$HOME/.local/share/portty-vm-tools/vm.sh}"
VM2_NAME="${P23_VM2_NAME:-portty-test-2}"
SOCKQ='"$HOME/Library/Application Support/portty/daemon.sock"'

die() { echo "p23-rig: $*" >&2; exit 1; }
vm() { "$VM_SH" "$@"; }
vmsh() { "$VM_SH" bash -s -- "$@"; }
vm2() {
  ssh -i "$HOME/.ssh/transom_demo_ed25519" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o BatchMode=yes -o ConnectTimeout=10 -o LogLevel=ERROR "admin@$(tart ip "$VM2_NAME")" "$@"
}
vm2sh() { vm2 bash -s -- "$@"; }
who_run() { case "$1" in home) shift; vm2 "$@" ;; office) shift; vm "$@" ;; *) die "WHO is home or office" ;; esac; }
scrub() {
  sed -E -e 's#/Users/[A-Za-z0-9._-]*#~#g' -e 's#100\.[0-9]+\.[0-9]+\.[0-9]+#100.x.x.N#g' \
    -e 's#[A-Za-z0-9.-]+\.ts\.net#TAILNET-NAME#g' -e 's#cap=[^& "]*#CAP-REDACTED#g' \
    -e 's#[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+#LOGIN@REDACTED#g' \
    -e 's#portty-vm-office#OFFICE#g' -e 's#portty-vm-home#HOME#g'
}

cmd_build() { [ $# -eq 2 ] || die "usage: build VERSION OUTDIR"; GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.11}" bash "$P22" build22 "$1" "$2"; }
cmd_deploy() { [ $# -eq 1 ] || die "usage: deploy APP"; bash "$P22" install "$1"; }
cmd_start() { bash "$P22" start "${1:-40}"; }

cmd_deploy_home() {
  [ $# -eq 1 ] && [ -d "$1/Contents" ] || die "usage: deploy-home APP"
  vm2 'pkill -x Portty; sleep 2; rm -rf /Applications/Portty.app'
  COPYFILE_DISABLE=1 tar -C "$(dirname "$1")" -czf - "$(basename "$1")" | vm2 'tar xzf - -C /Applications'
  vm2 'plutil -extract CFBundleShortVersionString raw /Applications/Portty.app/Contents/Info.plist'
}

# ---- office -------------------------------------------------------------------------------------

cmd_stats() {
  vm "curl -s -m 5 --unix-socket $SOCKQ http://localhost/desktop/stats" | jq -c . | scrub
}

cmd_stats_log() {
  [ $# -eq 2 ] || die "usage: stats-log SECS ROW"
  mkdir -p "$PHASE_DIR/evidence"
  vmsh "$1" <<'EOF' | scrub | tee "$PHASE_DIR/evidence/$2.stats"
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
for i in $(seq 1 $1); do
  echo "t=$i $(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -c '{viewers, own_viewers, state, audio, still, target_bps: .quality.target_bps, step: .quality.step, fps: .quality.fps, long_edge: .quality.long_edge, slow: .quality.slow, no_feedback: .quality.no_feedback}')"
  sleep 1
done
exit 0
EOF
}

cmd_stats_key() {
  vm "curl -s -m 5 --unix-socket $SOCKQ http://localhost/desktop/stats" | jq -r '[paths(type != "object") | map(tostring) | join(".")] | sort | join(" ")'
}

# tone-make OUT: 5 s, 48 kHz, 16-bit stereo; left = 440 Hz at half scale, right = silence. A file only.
cmd_tone_make() {
  [ $# -eq 1 ] || die "usage: tone-make OUT"
  perl -e '
    my $rate = 48000; my $n = $rate * 5; my $data = "";
    for my $i (0 .. $n - 1) { $data .= pack("s<s<", int(16000 * sin(2 * 3.14159265358979 * 440 * $i / $rate)), 0); }
    my $len = length $data;
    print "RIFF", pack("V", 36 + $len), "WAVEfmt ", pack("VvvVVvv", 16, 1, 2, $rate, $rate * 4, 4, 16), "data", pack("V", $len), $data;
  ' > "$1"
  echo "tone-make: $(wc -c < "$1" | tr -d ' ') bytes at $1"
}
cmd_tone_copy() { [ $# -eq 1 ] || die "usage: tone-copy FILE"; vm 'mkdir -p ~/p23 && cat > ~/p23/tone.wav && ls -l ~/p23/tone.wav | awk "{print \$5}"' < "$1"; }
cmd_tone_play() {
  vmsh <<'EOF'
nohup afplay ~/p23/tone.wav >/dev/null 2>&1 &
echo "tone-play: afplay started at office epoch $(perl -MTime::HiRes=time -e 'printf q(%.2f), time')"
exit 0
EOF
}

cmd_motion() {
  case "${1:-}" in
    on)
      vm "osascript -e 'tell application \"Terminal\" to activate' -e 'tell application \"Terminal\" to do script \"yes portty-motion-line-for-the-quality-rows\"' >/dev/null; echo motion on" ;;
    off)
      vm 'pkill -x yes; osascript -e "tell application \"Terminal\" to quit" >/dev/null 2>&1; pkill -x Terminal; echo motion off' ;;
    *) die "usage: motion on|off" ;;
  esac
}

# still-timed: with a viewer connected and the Terminal line stream running (motion on), stop only the stream
# (pkill -x yes: the screen stops changing at once, nothing closes) and poll `still` every 0.25 s inside the
# office until it reads true (up to 30 s); prints the stats just before the stop and the delay in seconds.
cmd_still_timed() {
  vmsh <<'EOF'
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
now() { perl -MTime::HiRes=time -e 'printf q(%.3f), time'; }
st() { curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -c '{viewers, still, step: .quality.step, target_bps: .quality.target_bps}'; }
echo "still-timed: before the stop: $(st)"
t0=$(now)
pkill -x yes
echo "still-timed: motion stopped at office epoch $t0"
for i in $(seq 1 120); do
  s=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -r '.still')
  tn=$(now)
  if [ "$s" = true ]; then echo "still-timed: still true $(perl -e "printf q(%.2f), $tn - $t0") s after the stop (poll $i)"; echo "still-timed: after: $(st)"; exit 0; fi
  sleep 0.25
done
echo "still-timed: still did not read true within 30 s; last: $(st)"
exit 0
EOF
}

cmd_audio_fault() {
  vmsh <<'EOF'
su() { echo admin | sudo -S -p '' "$@"; }
echo "audio-fault: coreaudiod pid before: $(pgrep -x coreaudiod)"
su killall coreaudiod
sleep 2
echo "audio-fault: coreaudiod pid after: $(pgrep -x coreaudiod)"
exit 0
EOF
}
cmd_clock() { vm "perl -MTime::HiRes=time -e 'printf q(%.2f), time'"; echo; }

# tools23: compile the Phase 22 AX helper on the host (compile only) and install it as ~/p22/axp22 in the home VM.
cmd_tools23() {
  local tmp
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/p23-tools.XXXXXX")
  swiftc -O "$(dirname "$P22")/axp22.swift" -o "$tmp/axp22" 2>&1 | grep -v warning || true
  swiftc -O "$ROOT/.planning/phases/21-portty-app-and-iphone-viewers/rig/key.swift" -o "$tmp/key" 2>&1 | grep -v warning || true
  vm2 'mkdir -p ~/p22 && cat > ~/p22/axp22 && chmod +x ~/p22/axp22' < "$tmp/axp22"
  vm2 'cat > ~/p22/key && chmod +x ~/p22/key' < "$tmp/key"
  rm -rf "$tmp"
  echo "tools23: axp22 and key installed in the home VM"
}

# ---- viewers ------------------------------------------------------------------------------------

cmd_open_viewer() {
  [ $# -eq 1 ] || die "usage: open-viewer home|office"
  bash "$P19" enable-desktop >/dev/null
  case "$1" in
    home) bash "$P20" safari-open-viewer | tail -1 | scrub ;;
    office) P19_SAFARI_ON=office bash "$P19" safari-stats 3 keep | tail -1 | scrub ;;
    *) die "WHO is home or office" ;;
  esac
}

cmd_close_viewer() {
  [ $# -eq 1 ] || die "usage: close-viewer home|office"
  who_run "$1" 'D=http://127.0.0.1:4444; [ -f ~/p19/safari.sid ] && curl -s -m 10 -X DELETE $D/session/$(cat ~/p19/safari.sid) >/dev/null; pkill -x safaridriver; rm -f ~/p19/safari.sid; echo close-viewer: done'
}

cmd_vstats() {
  [ $# -eq 1 ] || die "usage: vstats WHO"
  who_run "$1" 'D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid); curl -s -m 15 -X POST $D/session/$SID/execute/sync -d "{\"script\":\"return JSON.stringify(window.__desktopStats||null)\",\"args\":[]}" | jq -r ".value|tostring"' | scrub
}

# js WHO NAME [MS]: rig/js/NAME.js is an async function body that returns a string or a JSON value.
cmd_js() {
  [ $# -ge 2 ] || die "usage: js WHO NAME [MS]"
  local who=$1 name=$2 ms=${3:-0} body wrapped
  body=$(cat "$HERE/js/$name.js")
  body=${body//MS/$ms}
  wrapped="const cb=arguments[arguments.length-1]; (async()=>{ $body
})().then(v=>cb(typeof v==='string'?v:JSON.stringify(v)),e=>cb('ERR '+e))"
  jq -nc --arg s "$wrapped" '{script:$s,args:[]}' |
    who_run "$who" 'D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid); curl -s -m 120 -X POST $D/session/$SID/execute/async --data-binary @-' |
    jq -r '.value|tostring' | scrub
}
# vlog WHO SECS ROW: once a second for SECS seconds, inside WHO's VM, the viewer page's status, decoded frames,
# frame width, whether its text holds Reconnecting or Slow connection, and the page's load time origin (a
# reload would change it); saved as evidence/ROW.viewer-WHO.
cmd_vlog() {
  [ $# -eq 3 ] || die "usage: vlog WHO SECS ROW"
  mkdir -p "$PHASE_DIR/evidence"
  who_run "$1" bash -s -- "$2" <<'EOF' | scrub | tee "$PHASE_DIR/evidence/$3.viewer-$1"
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
JS='const s=window.__desktopStats||{}; const t=document.body.innerText; return JSON.stringify({status:s.status,fd:s.framesDecoded,fw:s.frameWidth,reconnecting:/Reconnecting/.test(t),slow:/Slow connection/.test(t),origin:Math.round(performance.timeOrigin)})'
BODY=$(jq -nc --arg s "$JS" '{script:$s,args:[]}')
for i in $(seq 1 $1); do
  echo "t=$i epoch=$(date +%s) $(curl -s -m 8 -X POST $D/session/$SID/execute/sync -d "$BODY" | jq -r '.value|tostring')"
  sleep 1
done
exit 0
EOF
}
cmd_hook() { cmd_js "$1" hook; }
cmd_pcstats() { cmd_js "$1" pcstats; }
cmd_levels() { [ $# -eq 2 ] || die "usage: levels WHO MS"; cmd_js "$1" levels "$2"; }

cmd_page() {
  jq -nc '{script:"return JSON.stringify({title: document.title, text: document.body.innerText.replace(/\\s+/g, \" \").slice(0, 700)})",args:[]}' |
    who_run "$1" 'D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid); curl -s -m 30 -X POST $D/session/$SID/execute/sync --data-binary @-' | jq -r '.value|tostring' | scrub
}

cmd_click_label() {
  [ $# -eq 2 ] || die "usage: click-label WHO LABEL"
  who_run "$1" bash -s -- "$2" <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid); label=$1
find_el() { curl -s -m 20 -X POST $D/session/$SID/element -d "$(jq -nc --arg x "$1" '{using:"xpath",value:$x}')" | jq -r '.value|objects|to_entries[]|select(.key|startswith("element-"))|.value'; }
eid=$(find_el "$(printf '//button[@aria-label="%s"]' "$label")")
[ -n "$eid" ] || { echo "click-label: no button [$label]"; exit 1; }
curl -s -m 20 -X POST $D/session/$SID/element/$eid/click -d '{}' | jq -c '.value'
echo "click-label: clicked [$label]"
exit 0
EOF
}

cmd_label_has() {
  [ $# -eq 2 ] || die "usage: label-has WHO LABEL"
  who_run "$1" bash -s -- "$2" <<'EOF'
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
n=$(curl -s -m 20 -X POST $D/session/$SID/elements -d "$(jq -nc --arg x "$(printf '//button[@aria-label="%s"]' "$1")" '{using:"xpath",value:$x}')" | jq '.value|length')
[ "$n" -gt 0 ] && echo yes || echo no
EOF
}

# chrome-open: Chrome in the home VM with a throwaway profile and its web accessibility tree switched on, on the
# desktop link with #debug (the link is read on the VM, never printed). Chrome's page scripts could not be driven:
# AppleScript JavaScript runs in an isolated world (the page's own RTCPeerConnection is not visible), and a raw
# DevTools socket client got no answer from Chrome's loopback HTTP endpoint. Chrome is therefore read through the
# AX tree (ax-home) and through the daemon's stats route.
cmd_chrome_open() {
  vm2sh <<'REMOTE'
pkill -x "Google Chrome" 2>/dev/null; sleep 2
rm -rf /tmp/p23-chrome
open -na "Google Chrome" --args --user-data-dir=/tmp/p23-chrome --force-renderer-accessibility \
  --no-first-run --no-default-browser-check "$(cat ~/p19/link.txt)#debug"
sleep 8
echo "chrome-open: Chrome pid $(pgrep -x 'Google Chrome' | head -1)"
exit 0
REMOTE
}

cmd_chrome_close() { vm2 'pkill -x "Google Chrome"; sleep 1; rm -rf /tmp/p23-chrome; echo chrome-close: done'; }

cmd_ax_home() {
  [ $# -ge 3 ] || die "usage: ax-home find|press|has BUNDLE WORDS"
  vm2 "~/p22/axp22 $*" | scrub
}

cmd_ax_safari() {
  if [ -n "${1:-}" ]; then
    mkdir -p "$PHASE_DIR/evidence"
    vm2 '~/ptest/axdump2 com.apple.Safari 2>&1' | scrub | tee "$PHASE_DIR/evidence/$1.ax"
  else
    vm2 '~/ptest/axdump2 com.apple.Safari 2>&1' | scrub
  fi
}

# ---- shaping (home VM only) ---------------------------------------------------------------------

# shape on KBIT | off | list, in the home VM only: the pipe holds traffic from the office's tailnet address to the
# home VM (the viewer's inbound video). A pipe on the office's own loopback viewer was tried and matched nothing
# (pf does not see that traffic), so the loopback viewer stays unshaped.
cmd_shape() {
  case "${1:-}" in
    on) [ $# -eq 2 ] || die "usage: shape on KBIT" ;;
    off | list) ;;
    *) die "usage: shape on KBIT | off | list" ;;
  esac
  vm2sh "$1" "${2:-}" <<'EOF' | scrub
su() { echo admin | sudo -S -p '' "$@" 2>&1 | grep -v -E 'ALTQ|^$' || true; }
act=$1; kbit=$2
case "$act" in
  on)
    ip=$(/usr/local/bin/tailscale ip -4 portty-vm-office | head -1)
    rule="dummynet in quick from $ip to any pipe 1"
    [ -n "$ip" ] || { echo "shape: could not resolve the tailnet address"; exit 1; }
    su dnctl pipe 1 config bw "${kbit}Kbit/s" delay 20 queue 30
    printf '%s\n' "$rule" > /tmp/p23.rules
    su pfctl -q -f /etc/pf.conf
    su pfctl -q -a com.apple/p23 -f /tmp/p23.rules
    su pfctl -E 2>&1 | awk '/Token/{print $NF}' >> /tmp/p23.tokens
    echo "shape: pipe of ${kbit} kbit/s at guest epoch $(date +%s)"
    ;;
  off)
    su pfctl -q -a com.apple/p23 -F all
    for t in $(cat /tmp/p23.tokens 2>/dev/null); do su pfctl -q -X "$t"; done
    su dnctl -q flush
    rm -f /tmp/p23.tokens /tmp/p23.rules
    echo "shape: off at guest epoch $(date +%s)"
    ;;
esac
echo "shape: dnctl list: [$(echo admin | sudo -S -p '' dnctl list 2>/dev/null | tr '\n' ' ')]"
echo "shape: anchor dummynet rules: $(echo admin | sudo -S -p '' pfctl -a com.apple/p23 -s dummynet 2>/dev/null | grep -c dummynet)"
echo "shape: $(echo admin | sudo -S -p '' pfctl -s info 2>/dev/null | grep Status)"
exit 0
EOF
}

sub=${1:-}
[ $# -gt 0 ] && shift
case "$sub" in
  build) cmd_build "$@" ;;
  deploy) cmd_deploy "$@" ;;
  deploy-home) cmd_deploy_home "$@" ;;
  start) cmd_start "$@" ;;
  tools23) cmd_tools23 ;;
  stats) cmd_stats ;;
  stats-log) cmd_stats_log "$@" ;;
  stats-key) cmd_stats_key ;;
  tone-make) cmd_tone_make "$@" ;;
  tone-copy) cmd_tone_copy "$@" ;;
  tone-play) cmd_tone_play ;;
  motion) cmd_motion "$@" ;;
  audio-fault) cmd_audio_fault ;;
  still-timed) cmd_still_timed ;;
  clock) cmd_clock ;;
  open-viewer) cmd_open_viewer "$@" ;;
  close-viewer) cmd_close_viewer "$@" ;;
  vstats) cmd_vstats "$@" ;;
  js) cmd_js "$@" ;;
  vlog) cmd_vlog "$@" ;;
  hook) cmd_hook "$@" ;;
  pcstats) cmd_pcstats "$@" ;;
  levels) cmd_levels "$@" ;;
  page) cmd_page "$@" ;;
  click-label) cmd_click_label "$@" ;;
  label-has) cmd_label_has "$@" ;;
  chrome-open) cmd_chrome_open ;;
  chrome-close) cmd_chrome_close ;;
  ax-home) cmd_ax_home "$@" ;;
  ax-safari) cmd_ax_safari "$@" ;;
  shape) cmd_shape "$@" ;;
  p22) bash "$P22" "$@" ;;
  p20) bash "$P20" "$@" ;;
  p19) bash "$P19" "$@" ;;
  *) die "usage: $0 build|deploy|deploy-home|start|stats|stats-log|stats-key|tone-make|tone-copy|tone-play|motion|audio-fault|clock|open-viewer|close-viewer|vstats|js|hook|pcstats|levels|page|click-label|label-has|ax-safari|shape|p22|p20|p19" ;;
esac
