//go:build !vmpreview

package main

// BRND-06 / UPD-01: the release build's update endpoint, allowlist and cadence. Not in the
// VM-only vmpreview build, whose updates_vmpreview.go points them at cmd/fakegh.

import (
	"net/url"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/updater"
)

func TestUpdateDefaults_ReleaseBuild(t *testing.T) {
	if updateAPI != updater.GitHubAPI || updater.GitHubAPI != "https://api.github.com" {
		t.Errorf("updateAPI = %q", updateAPI)
	}
	if updateFirstDelay != 10*time.Second || updateTick != time.Hour || updateEvery != 24*time.Hour {
		t.Errorf("cadence %v / %v / %v", updateFirstDelay, updateTick, updateEvery)
	}
	ok, _ := url.Parse("https://github.com/scottkw/portty/releases/download/v1.2.3/portty-v1.2.3-darwin-universal.dmg")
	bad, _ := url.Parse("http://127.0.0.1:8080/dl/portty-v1.2.3-darwin-universal.dmg")
	if !updateAllow(ok) || updateAllow(bad) {
		t.Errorf("updateAllow(github) = %v, updateAllow(loopback) = %v", updateAllow(ok), updateAllow(bad))
	}
}
