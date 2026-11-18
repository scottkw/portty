//go:build vmpreview

// VM-only fake update feed for the M-92 update checks in the Tart VMs: the
// update checks talk to cmd/fakegh on 127.0.0.1:8788 instead of GitHub, with
// a short cadence. Never in a release build: the vmpreview tag must not appear
// in .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).
// The vmpreview build also fakes Tailscale and sharing, which the update
// checks do not use.

package main

import (
	"net/url"
	"strings"
	"time"
)

func init() {
	updateAPI = "http://127.0.0.1:8788"
	updateAllow = func(u *url.URL) bool {
		return u.Scheme == "http" && u.Host == "127.0.0.1:8788" && u.User == nil &&
			strings.HasPrefix(u.Path, "/download/") && !strings.Contains(u.Path, "..")
	}
	updateTick = 15 * time.Second
	updateEvery = 2 * time.Minute
}
