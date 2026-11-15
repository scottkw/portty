package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAugmentServicePath_HomebrewTailscaleOnly: existing Homebrew/Tailscale
// dirs are prepended, the original PATH is kept as the suffix, and agent
// install dirs under $HOME are never added.
func TestAugmentServicePath_HomebrewTailscaleOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	original := "/usr/bin:/bin"
	t.Setenv("PATH", original)

	agentDirs := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".nvm", "versions", "node", "v20.0.0", "bin"),
	}
	for _, d := range agentDirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	AugmentServicePath()

	got := os.Getenv("PATH")
	if !strings.HasSuffix(got, original) {
		t.Errorf("PATH should end with original %s, got %s", original, got)
	}
	if strings.Contains(got, home) {
		t.Errorf("PATH must not contain agent dirs under %s, got %s", home, got)
	}
	origIdx := strings.LastIndex(got, original)
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		_, err := os.Stat(dir)
		idx := strings.Index(got, dir+string(os.PathListSeparator))
		if err == nil && (idx == -1 || idx > origIdx) {
			t.Errorf("existing %s should be prepended, got %s", dir, got)
		}
		if err != nil && idx != -1 {
			t.Errorf("non-existent %s should not be added, got %s", dir, got)
		}
	}
}
