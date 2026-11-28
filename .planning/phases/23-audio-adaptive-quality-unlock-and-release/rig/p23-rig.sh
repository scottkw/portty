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
    -e 's#portty-vm-office#OFFICE#g' -e 's#portty-vm-home#HOME#g' | redact
}

# Wrong-password sentinels (23-10): made up per run, kept only in a mode-0700 directory outside the repo
# (P23_SECRETS, default under TMPDIR), never printed, never committed. redact removes them from any output.
SECRETS="${P23_SECRETS:-${TMPDIR:-/tmp}/p23-sentinels}"
sentinel() { # sentinel N: the Nth made-up wrong password, created once
  mkdir -p "$SECRETS" && chmod 700 "$SECRETS"
  [ -s "$SECRETS/s$1" ] || { printf 'Zq%s' "$(openssl rand -hex 5)" > "$SECRETS/s$1"; chmod 600 "$SECRETS/s$1"; }
  cat "$SECRETS/s$1"
}
redact() {
  local f e=()
  for f in "$SECRETS"/s*; do [ -s "$f" ] && e+=(-e "s#$(cat "$f")#SENTINEL-REDACTED#g"); done
  if [ ${#e[@]} -gt 0 ]; then sed "${e[@]}"; else cat; fi
}
# pw_for KIND: admin (the VM's public password), wrong1..wrong6 (sentinels), emoji (a sentinel holding an emoji).
pw_for() {
  case "$1" in
    admin) printf 'admin' ;;
    wrong[1-6]) sentinel "${1#wrong}" ;;
    emoji) [ -s "$SECRETS/s9" ] || { mkdir -p "$SECRETS" && chmod 700 "$SECRETS"; printf 'Zq%s\xf0\x9f\x98\x80' "$(openssl rand -hex 3)" > "$SECRETS/s9"; chmod 600 "$SECRETS/s9"; }; cat "$SECRETS/s9" ;;
    *) die "password kind is admin, wrong1..wrong6 or emoji" ;;
  esac
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

# ---- unlock (23-10) -----------------------------------------------------------------------------
# Everything here runs inside portty-test (office, locked and typed at) or portty-test-2 (home, the viewer);
# the host only writes scrubbed evidence. Only the VM's public admin password and the made-up sentinels
# (pw_for) are ever typed.

# lock: Ctrl+Cmd+Q through the click tool (the 17-06 method), then wait up to 10 s for the daemon to read locked.
cmd_lock() {
  vmsh <<'EOF'
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
# Finder frontmost first: with the Portty window in front the shortcut does not lock (seen in 23-10)
open -a Finder; sleep 1
~/ptest/click2 key 12 cmd ctrl
for i in $(seq 1 40); do
  [ "$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop | jq -r .locked)" = true ] && { echo "lock: locked after about $((i / 4)) s"; exit 0; }
  sleep 0.25
done
echo "lock: NOT locked after 10 s"; exit 1
EOF
}

# unlock-admin: the recovery and cycle-end unlock that is NOT the daemon path: a user-activity wake, then the
# public password as key codes through the click tool (the 19 helper). Waits up to 12 s for unlocked.
cmd_unlock_admin() {
  vmsh <<'EOF'
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
caffeinate -u -t 2; sleep 1
for k in 0 2 46 34 45 36; do ~/ptest/click2 key $k; sleep 0.2; done
for i in $(seq 1 48); do
  [ "$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop | jq -r .locked)" = false ] && { echo "unlock-admin: unlocked"; exit 0; }
  sleep 0.25
done
echo "unlock-admin: STILL locked after 12 s"; exit 1
EOF
}

# lockstate: the daemon's locked reading and the unlock facts of the stats route, compact.
cmd_lockstate() {
  vmsh <<'EOF'
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
echo "lockstate: locked=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop | jq -r .locked) $(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -c '{viewers, own_viewers, goroutines, unlock}')"
EOF
}

# wake VK: the vmdesktop probe's wake op (the daemon's own Wake step) with virtual key VK.
cmd_wake() {
  [ $# -eq 1 ] || die "usage: wake VK"
  bash "$P20" probe wake "{\"vk\":$1}"
}

# lw-log SINCE: loginwindow's unified log from SINCE ("YYYY-MM-DD HH:MM:SS", guest time) to now, reduced to the
# lock-screen lines (field state, Return, the authorization result). The lines carry a length, never a password.
cmd_lw_log() {
  [ $# -ge 1 ] || die "usage: lw-log 'YYYY-MM-DD HH:MM:SS' ['YYYY-MM-DD HH:MM:SS' end]"
  local a=${1// /_} b=${2:-}
  vmsh "$a" "${b// /_}" <<'EOF' | scrub
S=${1/_/ }; END=(); [ -n "$2" ] && END=(--end "${2/_/ }")
/usr/bin/log show --start "$S" "${END[@]}" --predicate 'process == "loginwindow"' --style compact 2>/dev/null |
  grep -E 'keyPressed:\] \|[[:space:]]+View|loginPressed|Attempt #|uthorization (succeeded|failed)|INCORRECT|return in secure|showPasswordFieldMakingFirstResponder|hidePasswordField\] \||Clearing password field|Event: (Will|Did) (Sleep|Wake)|kAELockScreenEvent|_numFailedAuthAttempts|timeout timer fired' |
  sed -E 's/^[0-9-]+ ([0-9:.]+) [A-Za-z]+ +loginwindow\[[^]]*\] \[[^]]*\] /\1 /' | cut -c1-170
echo "lw-log: end"
EOF
}

# u01-tight VK DELAY ROW [click]: U-01's measurement (rig/u01tight.sh runs inside the office). Lock, wait DELAY s
# (3 = display awake; 45 = the lock UI has idled and the display slept), post candidate VK alone through the probe's
# wake op (or the click tool with "click", or nothing with VK "none" as the control), then 0.8 s later ONE letter key
# through the click tool as the diagnostic. loginwindow logs "keyPressed ... View is hidden password field length = 0"
# for a key that reaches its reveal handler and nothing for a key the already-revealed field takes itself, so a
# candidate reveals the field only if the diagnostic letter leaves no such line. The letter is deleted and is never
# followed by Return; the public password then unlocks the VM through unlock-admin.
cmd_u01_tight() {
  [ $# -ge 3 ] || die "usage: u01-tight VK DELAY ROW [click]"
  mkdir -p "$PHASE_DIR/evidence"
  { vmsh "$1" "$2" "${4:-probe}" < "$HERE/u01tight.sh" | scrub; cmd_unlock_admin; } 2>&1 | tee "$PHASE_DIR/evidence/$3.txt"
}

# office-now: the office's local time as lw-log wants it.
cmd_office_now() { vm "date '+%Y-%m-%d %H:%M:%S'"; }

# unlock-try KIND DUR ROW: on the home viewer, type the password KIND into the Unlock panel by page script (native
# setter plus an input event, as typing does), click the Unlock button with a real WebDriver click, then read the
# panel's status line four times a second for DUR seconds. At the same time the office's stats route is read twice a
# second. Afterwards loginwindow's lines and the new audit lines are printed. Saved as evidence/ROW.txt.
cmd_unlock_try() {
  [ $# -eq 3 ] || die "usage: unlock-try KIND DUR ROW"
  local kind=$1 dur=$2 row=$3 pw t0 tmp ac
  pw=$(pw_for "$kind")
  mkdir -p "$PHASE_DIR/evidence"
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/p23-try.XXXXXX")
  t0=$(cmd_office_now)
  ac=$(vm 'wc -l < "$HOME/Library/Application Support/portty/desktop-audit.jsonl" 2>/dev/null || echo 0' | tr -d ' ')
  {
    echo "unlock-try $row: kind=$kind dur=${dur}s office-start=$t0"
    echo "office before: $(cmd_lockstate)"
    ( vmsh "$dur" <<'EOF' > "$tmp/office.txt"
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
prev=""
for i in $(seq 1 $(( $1 * 2 ))); do
  s=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -c '{unlock, viewers}')
  l=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop | jq -r .locked)
  cur="locked=$l $s"
  [ "$cur" != "$prev" ] && echo "t=$(echo "$i" | awk '{printf "%.1f", ($1-1)/2}') $cur"
  prev=$cur
  sleep 0.5
done
EOF
    ) &
    local bg=$!
    vm2sh "$pw" "$dur" <<'EOF' | scrub
PW=$1; DUR=$2
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
find_el() { curl -s -m 20 -X POST $D/session/$SID/element -d "$(jq -nc --arg x "$1" '{using:"xpath",value:$x}')" | jq -r '.value|objects|to_entries[]|select(.key|startswith("element-"))|.value'; }
now() { perl -MTime::HiRes=time -e 'printf q(%.2f), time'; }
PANEL='const g=[...document.querySelectorAll("[role=group]")].find(e=>/Unlock This Mac/.test(e.innerText||"")); if(!g) return "panel absent; notice: "+((document.body.innerText.replace(/\s+/g," ").match(/Mac unlocked[^.]*\.?/)||["none"])[0]); const st=g.querySelector(".desktop-unlock__status"); return st?st.innerText.replace(/\s+/g," "):"no status"'
PBODY=$(jq -nc --arg s "$PANEL" '{script:$s,args:[]}')
panel() { curl -s -m 8 -X POST $D/session/$SID/execute/sync -d "$PBODY" | jq -r '.value|tostring'; }
T0=$(now)
echo "home before: $(panel)"
eid=$(find_el '//input[@aria-label="Mac password"]')
[ -n "$eid" ] || { echo "home: no Mac password field in the page"; exit 3; }
FILL=$(jq -nc --arg id "$eid" --arg pw "$PW" '{script:"const el=arguments[0]; Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,\"value\").set.call(el,arguments[1]); el.dispatchEvent(new Event(\"input\",{bubbles:true})); return \"set\"",args:[{"element-6066-11e4-a52e-4f735466cecf":$id},$pw]}')
curl -s -m 15 -X POST $D/session/$SID/execute/sync -d "$FILL" | jq -c '.value' | sed 's/^/home fill: /'
unset FILL PW
bid=$(find_el '//button[normalize-space()="Unlock"]')
[ -n "$bid" ] || { echo "home: no Unlock button"; exit 3; }
tc=$(now)
curl -s -m 20 -X POST $D/session/$SID/element/$bid/click -d '{}' | jq -c '.value' | sed 's/^/home click: /'
prev=""
end=$(perl -e "print $tc + $DUR")
while [ "$(perl -e "print (($(now) < $end) ? 1 : 0)")" = 1 ]; do
  cur=$(panel)
  [ "$cur" != "$prev" ] && echo "t=+$(perl -e "printf q(%.2f), $(now) - $tc") panel: $cur"
  prev=$cur
  sleep 0.2
done
exit 0
EOF
    wait "$bg" 2>/dev/null || true
    echo "office transitions:"; cat "$tmp/office.txt" | scrub
    echo "loginwindow:"; cmd_lw_log "$t0"
    echo "new audit lines:"
    # Only the event, mode, reason and the key names of each line: the lines also name the viewer (a person), which
    # never goes into evidence.
    vm "tail -n +$((ac + 1)) \"\$HOME/Library/Application Support/portty/desktop-audit.jsonl\" 2>/dev/null | jq -c '{ts, event, mode, reason, keys: keys}'" | scrub
    echo "office after: $(cmd_lockstate)"
  } 2>&1 | tee "$PHASE_DIR/evidence/$row.txt"
  rm -rf "$tmp"
}

# unlock-seq ROW STEP...: a run of submits on the home viewer, each STEP being a password KIND (pw_for) or wait:SECS.
# After a submit the panel is read until its status line changed and reached a settled state (a failure, a rate-limit
# or pause line, a refusal, or the panel gone), then the next step starts at once. Per step the panel's transitions
# are printed with the office clock beside them; afterwards loginwindow's lines, the stats transitions and the new
# audit lines. Saved as evidence/ROW.txt.
cmd_unlock_seq() {
  [ $# -ge 2 ] || die "usage: unlock-seq ROW STEP..."
  local row=$1 s dur=20 tmp t0 ac
  shift
  local steps=()
  for s in "$@"; do
    case "$s" in
      wait:*) steps+=("W:${s#wait:}"); dur=$((dur + ${s#wait:})) ;;
      *) steps+=("P:$s:$(pw_for "$s")"); dur=$((dur + 28)) ;;
    esac
  done
  mkdir -p "$PHASE_DIR/evidence"
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/p23-seq.XXXXXX")
  t0=$(cmd_office_now)
  ac=$(vm 'wc -l < "$HOME/Library/Application Support/portty/desktop-audit.jsonl" 2>/dev/null || echo 0' | tr -d ' ')
  {
    echo "unlock-seq $row: steps=$* office-start=$t0"
    echo "office before: $(cmd_lockstate)"
    ( vmsh "$dur" <<'EOF' > "$tmp/office.txt"
SOCK="$HOME/Library/Application Support/portty/daemon.sock"
prev=""
for i in $(seq 1 $(( $1 * 2 ))); do
  s=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop/stats | jq -c '{unlock, viewers}')
  l=$(curl -s -m 3 --unix-socket "$SOCK" http://localhost/desktop | jq -r .locked)
  cur="locked=$l $s"
  [ "$cur" != "$prev" ] && echo "$(date +%H:%M:%S) $cur"
  prev=$cur
  sleep 0.5
done
EOF
    ) &
    local bg=$!
    vm2sh "${steps[@]}" <<'EOF' | scrub
D=http://127.0.0.1:4444; SID=$(cat ~/p19/safari.sid)
find_el() { curl -s -m 20 -X POST $D/session/$SID/element -d "$(jq -nc --arg x "$1" '{using:"xpath",value:$x}')" | jq -r '.value|objects|to_entries[]|select(.key|startswith("element-"))|.value'; }
now() { perl -MTime::HiRes=time -e 'printf q(%.2f), time'; }
PANEL='const g=[...document.querySelectorAll("[role=group]")].find(e=>/Unlock This Mac/.test(e.innerText||"")); if(!g) return "panel absent; notice: "+((document.body.innerText.replace(/\s+/g," ").match(/Mac unlocked[^.]*\.?/)||["none"])[0]); const st=g.querySelector(".desktop-unlock__status"); return st?st.innerText.replace(/\s+/g," "):"no status"'
PBODY=$(jq -nc --arg s "$PANEL" '{script:$s,args:[]}')
panel() { curl -s -m 8 -X POST $D/session/$SID/execute/sync -d "$PBODY" | jq -r '.value|tostring'; }
for step in "$@"; do
  case "$step" in
    W:*) echo "step wait ${step#W:} s"; sleep "${step#W:}"; continue ;;
  esac
  rest=${step#P:}; kind=${rest%%:*}; PW=${rest#*:}
  pre=$(panel)
  echo "step submit $kind (office clock check: panel before: $pre)"
  eid=$(find_el '//input[@aria-label="Mac password"]')
  [ -n "$eid" ] || { echo "home: no Mac password field in the page"; exit 3; }
  FILL=$(jq -nc --arg id "$eid" --arg pw "$PW" '{script:"const el=arguments[0]; Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,\"value\").set.call(el,arguments[1]); el.dispatchEvent(new Event(\"input\",{bubbles:true})); return \"set\"",args:[{"element-6066-11e4-a52e-4f735466cecf":$id},$pw]}')
  curl -s -m 15 -X POST $D/session/$SID/execute/sync -d "$FILL" | jq -c '.value' | sed 's/^/  fill: /'
  unset FILL PW
  bid=$(find_el '//button[normalize-space()="Unlock"]')
  [ -n "$bid" ] || { echo "home: no Unlock button"; exit 3; }
  tc=$(now)
  curl -s -m 20 -X POST $D/session/$SID/element/$bid/click -d '{}' | jq -c '.value' | sed 's/^/  click: /'
  prev=""; settled=0
  end=$(perl -e "print $tc + 30")
  while [ "$(perl -e "print (($(now) < $end) ? 1 : 0)")" = 1 ]; do
    cur=$(panel)
    if [ "$cur" != "$prev" ]; then
      echo "  t=+$(perl -e "printf q(%.2f), $(now) - $tc") panel: $cur"
      if [ "$cur" != "$pre" ] && ! printf '%s' "$cur" | grep -qE 'Unlocking|Trying once more' && printf '%s' "$cur" | grep -qE "didn.t work|Try again in|paused|panel absent|can.t type|Caps Lock|Couldn.t unlock|needs Accessibility"; then settled=1; fi
    fi
    prev=$cur
    [ "$settled" = 1 ] && break
    sleep 0.2
  done
done
echo "end: $(panel)"
exit 0
EOF
    wait "$bg" 2>/dev/null || true
    echo "office transitions (office clock):"; cat "$tmp/office.txt" | scrub
    echo "loginwindow:"; cmd_lw_log "$t0"
    echo "new audit lines:"
    vm "tail -n +$((ac + 1)) \"\$HOME/Library/Application Support/portty/desktop-audit.jsonl\" 2>/dev/null | jq -c '{ts, event, mode, reason, keys: keys}'" | scrub
    echo "office after: $(cmd_lockstate)"
  } 2>&1 | tee "$PHASE_DIR/evidence/$row.txt"
  rm -rf "$tmp"
}

# sentinel-hits ROW: U-12. In the office, count the wrong sentinels (the files in SECRETS, passed as arguments and never
# printed) in daemon.log, desktop-audit.jsonl and the unified log: the Portty process, then every process. Also the audit
# lines' key sets for unlocked and unlock_failed, and the lines' modes. Counts only; saved as evidence/ROW.txt.
cmd_sentinel_hits() {
  [ $# -eq 1 ] || die "usage: sentinel-hits ROW"
  local f s=() n=0
  for f in "$SECRETS"/s*; do [ -s "$f" ] && { s+=("$(cat "$f")"); n=$((n + 1)); }; done
  [ "$n" -gt 0 ] || die "no sentinel was ever created"
  mkdir -p "$PHASE_DIR/evidence"
  vmsh "${s[@]}" <<'EOF' | scrub | tee "$PHASE_DIR/evidence/$1.txt"
D="$HOME/Library/Application Support/portty"
echo "sentinels checked: $#"
i=0
for s in "$@"; do
  i=$((i + 1))
  # grep -F reads the sentinel as data; only counts are printed
  d=$(grep -cF -- "$s" "$D/daemon.log" 2>/dev/null); a=$(grep -cF -- "$s" "$D/desktop-audit.jsonl" 2>/dev/null)
  # the older rotated log, if any
  r=$(cat "$D"/daemon.log.* 2>/dev/null | grep -cF -- "$s")
  p=$(/usr/bin/log show --last 3h --predicate 'process == "Portty"' --style compact 2>/dev/null | grep -cF -- "$s")
  e=$(/usr/bin/log show --last 3h --predicate "eventMessage CONTAINS \"$s\"" --style compact 2>/dev/null | grep -F -- "$s" | grep -vcE "^[^ ]+ [^ ]+ +[A-Za-z]+ +log\[")
  echo "sentinel $i: daemon.log=$d rotated-daemon-logs=$r desktop-audit.jsonl=$a unified-log(Portty process)=$p unified-log(any process, message text)=$e"
done
F="$D/desktop-audit.jsonl"
echo "audit file: mode=$(stat -f %Lp "$F") lines=$(wc -l < "$F" | tr -d ' ')"
echo "events unlocked / unlock_failed / relocked / relock_failed: $(jq -r 'select(.event=="unlocked" or .event=="unlock_failed" or .event=="relocked" or .event=="relock_failed") | .event' "$F" | sort | uniq -c | awk '{printf "%s=%s ", $2, $1}')"
echo "modes on those lines: $(jq -r 'select(.event=="unlocked" or .event=="unlock_failed" or .event=="relocked" or .event=="relock_failed") | .mode' "$F" | sort | uniq -c | awk '{printf "%s=%s ", $2, $1}')"
echo "key sets on those lines:"
jq -c 'select(.event=="unlocked" or .event=="unlock_failed" or .event=="relocked" or .event=="relock_failed") | {event, keys: (keys)}' "$F" | sort | uniq -c
echo "any line with a key naming length, password or secret: $(jq -r 'select(.event=="unlocked" or .event=="unlock_failed") | keys[] | select(test("len|pass|secret|pw"; "i"))' "$F" | wc -l | tr -d ' ')"
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
  lock) cmd_lock ;;
  unlock-admin) cmd_unlock_admin ;;
  lockstate) cmd_lockstate ;;
  wake) cmd_wake "$@" ;;
  lw-log) cmd_lw_log "$@" ;;
  u01-tight) cmd_u01_tight "$@" ;;
  office-now) cmd_office_now ;;
  unlock-try) cmd_unlock_try "$@" ;;
  unlock-seq) cmd_unlock_seq "$@" ;;
  sentinel-hits) cmd_sentinel_hits "$@" ;;
  panel) cmd_js "${1:-home}" unlockpanel ;;
  audit-dump) bash "$P22" audit-dump "$@" ;;
  guest-open) bash "$P22" guest-open ;;
  guest-text) bash "$P22" guest-text ;;
  guest-close) bash "$P22" guest-close ;;
  stand-in-input) bash "$P22" stand-in-input "$@" ;;
  p22) bash "$P22" "$@" ;;
  p20) bash "$P20" "$@" ;;
  p19) bash "$P19" "$@" ;;
  *) die "usage: $0 build|deploy|deploy-home|start|stats|stats-log|stats-key|tone-make|tone-copy|tone-play|motion|audio-fault|clock|open-viewer|close-viewer|vstats|js|hook|pcstats|levels|page|click-label|label-has|ax-safari|shape|p22|p20|p19" ;;
esac
