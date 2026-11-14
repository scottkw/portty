package webserver

import (
	"os"
	"os/exec"
)

// tailscaleWellKnownPaths returns the ordered list of well-known macOS binary
// paths. Detection order: custom path -> these -> PATH.
func tailscaleWellKnownPaths() []string {
	return []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/opt/homebrew/bin/tailscale",
		"/usr/local/bin/tailscale",
	}
}

// detectTailscaleBinary finds the tailscale binary. Detection order:
//  1. customPath (non-empty, must exist on disk)
//  2. Well-known platform paths (tailscaleWellKnownPaths)
//  3. PATH (exec.LookPath)
//
// Returns the resolved path or "" if not found.
func detectTailscaleBinary(customPath string) string {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath
		}
	}
	for _, p := range tailscaleWellKnownPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	return ""
}
