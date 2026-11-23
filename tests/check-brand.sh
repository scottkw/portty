#!/usr/bin/env bash
# tests/check-brand.sh
# Brand gate for Portty. Fails if tracked files outside .planning/ still carry:
#   - the old product name, in content or in a file name   (BRND-05)
#   - a github.com/scottkw/ link to any repo but portty    (BRND-06)
#   - a mention of the macOS package manager, outside the
#     Tailscale PATH code in internal/daemon/path.go       (DIST-03)
#   - the VM-only vmpreview or vmwhois build tag in .github, build.sh or
#     scripts, tracked or untracked: release builds must never carry
#     the fake Tailscale used for the Tart VM GUI check    (SHAR-01)
#     the fake update feed of the M-92 VM update checks    (UPD)
#     or the faked other-account WhoIs of the VM checks    (MOB-05)
#   - an import of the WhoIs fake package (webservertest) by a non-test Go
#     file outside the Playwright fixture and the package itself (MOB-05)
# Every pattern spells its word with one letter in a bracket class, so the
# pattern never matches its own text and this script never flags itself.
#
# Exit: 0 clean, 1 findings, 2 git error.
# Run: bash tests/check-brand.sh    (from anywhere in the repo)

set -uo pipefail # not -e: git grep exits 1 for "no match", which is expected

# Pathspecs and the packaging/ check are root-relative.
root=$(git rev-parse --show-toplevel) && cd "$root" || { echo "ERROR: not a git repo"; exit 2; }

FINDINGS=0

# report <prefix> <git output>: print each non-empty line with the prefix.
report() {
  local prefix=$1 out=$2 line
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    echo "${prefix}: ${line}"
    FINDINGS=$((FINDINGS + 1))
  done <<< "$out"
}

# check_status <status>: git grep/ls-files status above 1 is a real failure.
check_status() {
  if (( $1 > 1 )); then
    echo "ERROR: git grep failed (status $1)"
    exit 2
  fi
}

OLD_NAME='agent[h]ub'

out=$(git grep -n -i -I "$OLD_NAME" -- ':!.planning'); rc=$?
check_status "$rc"
report content "$out"

files=$(git ls-files); rc=$?
check_status "$rc"
report filename "$(grep -i -E "$OLD_NAME" <<< "$files")"

out=$(git grep -n -i -I -E 'home[b]rew|[b]rew (install|tap)' -- ':!.planning' \
  ':!internal/daemon/path.go' ':!internal/daemon/path_test.go'); rc=$?
check_status "$rc"
PM_PREFIX="home""brew" # split so this file does not spell the word it checks for
report "$PM_PREFIX" "$out"
if [[ -e packaging ]]; then
  report "$PM_PREFIX" "packaging/ still exists"
fi

out=$(git grep -n -I -o -E 'github\.com/scottkw/[A-Za-z0-9_.-]+' -- ':!.planning'); rc=$?
check_status "$rc"
report link "$(grep -v -E ':github\.com/scottkw/portty$' <<< "$out")"

out=$(git grep -n -I --untracked -E 'vmpr[e]view|vmwh[o]is' -- .github build.sh scripts); rc=$?
check_status "$rc"
report vm-tag "$out"

out=$(git grep -n -I 'internal/webserver/webserver[t]est' -- '*.go' ':!*_test.go' \
  ':!cmd/playwright-fixture/' ':!internal/webserver/webservertest/'); rc=$?
check_status "$rc"
report fake-whois "$out"

if (( FINDINGS > 0 )); then
  echo "FAIL: ${FINDINGS} finding(s)"
  exit 1
fi
echo "OK: brand gate clean"
