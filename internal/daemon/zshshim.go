package daemon

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// zshShim is the .zshenv a new session's zsh reads first (via ZDOTDIR). It
// restores the user's ZDOTDIR, sources their own .zshenv, and reports the
// working directory with OSC 7 so the hub can resolve relative file paths.
//
//go:embed zshshim.zsh
var zshShim string

// zshEnv returns the env entries that point a new session's zsh at the shim,
// or nil when the shim can't be installed: the session then starts with the
// user's startup files exactly as without Portty, and the log says why.
//
// ZDOTDIR is set only after .zshenv reads back equal to the embedded script.
// A ZDOTDIR whose .zshenv is missing makes zsh skip every user startup file
// silently, so "set nothing" is the only safe answer to a failed write.
// PORTTY_ZSH_ZDOTDIR is always present (empty when the daemon has no ZDOTDIR)
// so a stale inherited value is overwritten by mergeEnv's extra entries.
func (e *SessionEngine) zshEnv() []string {
	if !filepath.IsAbs(e.configDir) {
		log.Printf("zsh shim: config dir %q is not absolute", e.configDir)
		return nil
	}
	dir := filepath.Join(e.configDir, "zsh")
	if err := refreshZshShim(dir); err != nil {
		log.Printf("zsh shim: %v", err)
		return nil
	}
	return []string{"ZDOTDIR=" + dir, "PORTTY_ZSH_ZDOTDIR=" + os.Getenv("ZDOTDIR")}
}

// refreshZshShim makes dir/.zshenv equal the embedded script. A file that
// already matches is left alone; otherwise a temporary file in dir is renamed
// over it, so a concurrently starting shell reads either the old or the new
// script, never a partial one. The result is read back before returning nil.
func refreshZshShim(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, ".zshenv")
	want := []byte(zshShim)
	if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, want) {
		return nil
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".zshenv-"+hex.EncodeToString(suffix))
	if err := os.WriteFile(tmp, want, 0600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s does not match the embedded script after writing", path)
	}
	return nil
}
