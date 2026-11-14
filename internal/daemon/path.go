package daemon

import (
	"os"
	"strings"
)

// AugmentServicePath prepends the Homebrew and Tailscale binary directories to
// the process PATH so that tools such as the tailscale CLI are found when the
// daemon runs as a launchd user service (or when the GUI app is launched from
// Finder/Dock), which do not source shell init files. Sessions are login
// shells and rebuild the user's own PATH from /etc/zprofile and dotfiles.
// Called once at startup before any exec.LookPath or session is created.
func AugmentServicePath() {
	candidates := []string{
		"/opt/homebrew/bin", // macOS ARM Homebrew
		"/usr/local/bin",    // macOS Intel Homebrew, Tailscale
	}

	current := os.Getenv("PATH")
	var extra []string
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			extra = append(extra, dir)
		}
	}
	if len(extra) > 0 {
		_ = os.Setenv("PATH", strings.Join(extra, string(os.PathListSeparator))+string(os.PathListSeparator)+current)
	}
}
