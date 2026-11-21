package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// cliExecutable returns the resolved path of the binary `portty` runs inside
// sessions: this build's own executable. Tests point it at a temp file.
var cliExecutable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// cliPathEnv refreshes <configDir>/bin/portty to point at this build and
// returns the env entry that puts that dir first on a new session's PATH
// (PREV-05). It returns nil when the link can't be made: the session starts
// anyway, and the daemon log says why.
//
// A per-user link, not …/Portty.app/Contents/MacOS on PATH: that dir holds
// "Portty" (capital P), which breaks `portty` on case-sensitive volumes and
// tab completion; the link also heals itself after app moves and updates.
// zsh's login path_helper keeps the entry but moves it after the system dirs.
func (e *SessionEngine) cliPathEnv() []string {
	binDir := filepath.Join(e.configDir, "bin")
	if err := refreshCLILink(binDir); err != nil {
		log.Printf("cli link: %v", err)
		return nil
	}
	// A daemon started from a Portty session inherits a PATH that already has binDir: keep one copy (11-REVIEW IN-03).
	path := []string{binDir}
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p != binDir {
			path = append(path, p)
		}
	}
	return []string{"PATH=" + strings.Join(path, string(os.PathListSeparator))}
}

// refreshCLILink makes binDir/portty a symlink to cliExecutable's target. A
// stale link or file is replaced by renaming a new symlink over it, so a
// concurrently spawning shell always finds a working link.
func refreshCLILink(binDir string) error {
	target, err := cliExecutable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := os.MkdirAll(binDir, 0700); err != nil {
		return err
	}
	realBin, err := filepath.EvalSymlinks(binDir)
	if err != nil {
		return err
	}
	if target == realBin || strings.HasPrefix(target, realBin+string(filepath.Separator)) {
		return fmt.Errorf("refusing target %s inside %s (it would link to itself)", target, binDir)
	}
	link := filepath.Join(binDir, "portty")
	if cur, err := filepath.EvalSymlinks(link); err == nil && cur == target {
		return nil
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	// shortcut: a crash between Symlink and Rename leaves a .portty-* link in
	// binDir (harmless, never named portty); sweep them if one ever shows up.
	tmp := filepath.Join(binDir, ".portty-"+hex.EncodeToString(suffix))
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
