#!/usr/bin/env bash
# apply.sh TREE: apply the THROWAWAY Phase 21-02 key probe to a scratch export of the tree.
# Refuses anything that is not a scratch export made by p21-rig.sh build-probe (it carries the
# .p21-scratch marker) or that is inside the repo that holds these rig files: the probe must
# never reach the shipped source tree (T-21-05).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
die() { echo "apply.sh: $*" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: apply.sh SCRATCH_TREE"
T="$(cd "$1" && pwd -P)"
ROOT="$(git -C "$HERE" rev-parse --show-toplevel)"
ROOT="$(cd "$ROOT" && pwd -P)"
case "$T" in "$ROOT" | "$ROOT"/*) die "$T is inside the repo; refusing" ;; esac
[ -f "$T/.p21-scratch" ] || die "$T has no .p21-scratch marker (not a scratch export)"
[ ! -e "$T/.git" ] || die "$T is a git checkout, not an export; refusing"

cp "$HERE/KeyProbe.tsx" "$T/frontend/src/KeyProbe.tsx"
cp "$HERE/probe_darwin.go" "$T/probe_darwin.go"
cp "$HERE/probe_darwin.m" "$T/probe_darwin.m"

# Mount the probe at the top of App's render (exactly one match, or fail).
APP="$T/frontend/src/App.tsx"
[ "$(grep -c '^    <div className="app">$' "$APP")" -eq 1 ] || die "App.tsx render anchor not found exactly once"
perl -0pi -e 's|^    <div className="app">\n|    <div className="app">\n      <KeyProbe />\n|m' "$APP"
perl -0pi -e "s|(import \{ EventsOn \} from './wailsjs/wailsjs/runtime/runtime'\n)|\$1import { KeyProbe } from './KeyProbe'\n|" "$APP"
grep -q "<KeyProbe />" "$APP" || die "KeyProbe mount not applied"
grep -q "from './KeyProbe'" "$APP" || die "KeyProbe import not applied"

# Stub bindings (wails build regenerates them from the Go methods; this keeps tsc happy before that).
J="$T/frontend/src/wailsjs/go/main/App.js"
D="$T/frontend/src/wailsjs/go/main/App.d.ts"
cat >> "$J" <<'EOF'

// THROWAWAY 21-02 key probe
export const ProbeLog = (line) => Call('main.App.ProbeLog', [line])
export const ProbeMode = () => Call('main.App.ProbeMode', [])
export const ProbeBlankMenuKeys = (on) => Call('main.App.ProbeBlankMenuKeys', [on])
export const ProbeRebuildMenu = (on) => Call('main.App.ProbeRebuildMenu', [on])
EOF
cat >> "$D" <<'EOF'

export function ProbeLog(line: string): Promise<void>
export function ProbeMode(): Promise<string>
export function ProbeBlankMenuKeys(on: boolean): Promise<void>
export function ProbeRebuildMenu(on: boolean): Promise<void>
EOF
echo "apply.sh: probe applied to scratch tree $T"
