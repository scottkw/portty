#!/usr/bin/env bash
# tests/check-share-removal.sh
# Share removal gate for Portty Phase 4 (ROADMAP SC4; SHAR-02, SHAR-05).
# Fails if tracked files outside .planning/, TESTING.md, vendored xterm and the
# *_removed_test.go route pins still carry any of:
#   - public internet sharing, its consent UI and the public wr[i]te gate
#   - code-based access and image codes (the old image-code module included)
#   - view-only links, or a view-only capability minted by non-test Go
#   - the LAN server and its password
#   - the one-time share reminder
#   - the old web-server lifecycle bindings and routes
#   - the old CLI subcommands
# It also fails if a removed file still exists, or if TESTING.md Sections 4-5
# still describe a removed feature.
#
# Kept on purpose: the capability write-perm check (relay.Subscriber.ReadOnly
# from HasPerm(claims.Perms, "write")) is the security model, not a link feature,
# so the view-only patterns leave capabilities and viewers alone. A class field
# that happens to be named after the write method is not that gate.
#
# Every pattern spells its word with one letter in a bracket class, and removed
# paths are split literals, so this script never matches its own text.
#
# Exit: 0 clean, 1 findings, 2 git error.
# Run: bash tests/check-share-removal.sh    (from anywhere in the repo)

set -uo pipefail # not -e: git grep exits 1 for "no match", which is expected

# Pathspecs, removed-file checks and TESTING.md are all root-relative.
root=$(git rev-parse --show-toplevel) && cd "$root" || { echo "ERROR: not a git repo"; exit 2; }
[[ -f TESTING.md ]] || { echo "ERROR: TESTING.md missing"; exit 2; }

FINDINGS=0
# The legacy settings fixture spells the old reminder keys so the migration test can pin their removal.
SCOPE=(':!.planning' ':!TESTING.md' ':!web/vendor' ':!*_removed_test.go' ':!tests/fixtures/settings_legacy_v4.json')

# report <prefix> <output>: print each non-empty line with the prefix.
report() {
  local prefix=$1 out=$2 line
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    echo "${prefix}: ${line}"
    FINDINGS=$((FINDINGS + 1))
  done <<< "$out"
}

# check_status <status>: git grep status above 1 is a real failure.
check_status() {
  if (( $1 > 1 )); then
    echo "ERROR: git grep failed (status $1)"
    exit 2
  fi
}

# scan <label> <git grep flags and pattern...>: content check over SCOPE.
scan() {
  local label=$1 out rc
  shift
  out=$(git grep -n -I "$@" -- "${SCOPE[@]}"); rc=$?
  check_status "$rc"
  report "$label" "$out"
}

PUBLIC='f[u]nnel|r[i]sk[ _-]?panel|h[o]ld[ _-]?to[ _-]?confirm|public[ _-]?(l[i]nk|sh[a]re)|public[_-]?wr[i]te|public wr[i]te gate|SetRWG[a]te|isRWG[a]ted|rwG[a]ted'
# The route check skips "_" so the Phase 5 test file path (relay/j[o]in_test.go) in TESTING.md Section 4 is not a hit.
CODES='j[o]in[ _-]?c[o]de|/j[o]in([^a-z_]|$)|q[r][ _-]?code'
VIEWONLY='--r[e]adonly|Get\("r[e]adonly"\)|r[e]adonly[-_]badge|internet[ _-]?r[e]ad|r[e]ad-only (link|share|tab)'
REMINDER='shellw[e]bshare|shell[ _-]w[e]b[ _-]share'

scan public-sharing -i -E -e "$PUBLIC"
scan access-codes -i -E -e "$CODES"
scan view-only -i -E -e "$VIEWONLY"
scan view-only -E -e 'READ O[N]LY'

out=$(git grep -n -I -E 'Perms: *"r[e]ad"' -- '*.go' ':!*_test.go' "${SCOPE[@]}"); rc=$?
check_status "$rc"
report view-only-cap "$out"

scan lan-server -E -e 'startL[o]cal|L[o]calPassword|l[o]calPassword|GetL[A]NIP|BasicAuthM[i]ddleware|upgradeT[o]Tailscale'
scan lan-server -i -E -e 'on your l[o]cal network'
scan reminder -i -E -e "$REMINDER"
scan lifecycle -E -e 'StartW[e]bServer|StopW[e]bServer|ToggleW[e]bServing|IssueC[a]pabilities|W[e]bServerMode|IsW[e]bServerRunning|/w[e]b-serve|/w[e]bserver/(start|stop|status|local-password)'

out=$(git grep -n -I -E 'case "(s[e]rve|uns[e]rve|w[e]b|q[r])"' -- main.go); rc=$?
check_status "$rc"
report cli "$out"
out=$(git grep -n -I -E '^  (s[e]rve|uns[e]rve|w[e]b|q[r]) ' -- cmd_cli.go); rc=$?
check_status "$rc"
report cli "$out"

# Removed files (split literals keep the checked words out of this script).
for path in \
  "internal/webserver/f""unnel_client.go" \
  internal/webserver/auth.go \
  internal/webserver/localip.go \
  "internal/capability/j""oincode.go" \
  "web/j""oin.html" \
  frontend/src/components/Share \
  frontend/src/components/SessionShare \
  "frontend/src/components/ShellW""ebShareBanner.tsx" \
  "frontend/src/components/RemoteJ""oinCodeModal.tsx"; do
  if [[ -e "$path" ]]; then
    report removed-file "$path still exists"
  fi
done

# TESTING.md Sections 4-5 (Section 2 is a dated history log and may name them).
section=$(awk '/^## 4\./,/^## 6\./' TESTING.md)
[[ -n "$section" ]] || { echo "ERROR: TESTING.md Sections 4-6 not found"; exit 2; }
out=$(grep -n -i -E -e "${PUBLIC}|${CODES}|${VIEWONLY}|${REMINDER}" <<< "$section")
report testing-md "$out"

if (( FINDINGS > 0 )); then
  echo "FAIL: ${FINDINGS} finding(s)"
  exit 1
fi
echo "OK: share removal gate clean"
