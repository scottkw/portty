//go:build vmpreview

// VM-only fake update feed for the M-92 update checks in the Tart VMs: the
// update checks talk to cmd/fakegh on 127.0.0.1:8788 instead of GitHub, with
// a short cadence. Never in a release build: the vmpreview tag must not appear
// in .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).
// The vmpreview build also fakes Tailscale and sharing, which the update
// checks do not use.
//
// It also carries the VM-only verify bypass (18-08): when $HOME/p18/verify-mode
// holds exactly "adhoc", the codesign and spctl answers are faked so an ad-hoc
// rig build passes the release Team ID and notarization checks. It lives in
// this file on purpose: any binary that carries the bypass also carries the
// 127.0.0.1:8788 feed address that release.yml refuses in a shipped binary.
// Any other file content, or no file, runs the real tools.
//
// And the VM-only fault hooks for the rollback rows (18-03): $HOME/p18/fault holds
// "MODE VERSION" and acts only in the build whose main.Version is exactly VERSION, so the
// rolled-back build never trips. exit-before-daemon and exit-after-daemon exit the GUI (status 3)
// around its startup daemon ensure; hang-before-quit blocks the GUI forever right after the
// installer reports it started, before the GUI quits. A file, not the environment, because
// LaunchServices starts the relaunched GUI without the installer's environment. Same file as the
// feed address for the same reason as the verify bypass: it never reaches a release build.

package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/updater"
)

func init() {
	updateAPI = "http://127.0.0.1:8788"
	updateAllow = func(u *url.URL) bool {
		return u.Scheme == "http" && u.Host == "127.0.0.1:8788" && u.User == nil &&
			strings.HasPrefix(u.Path, "/download/") && !strings.Contains(u.Path, "..")
	}
	updateTick = 15 * time.Second
	updateEvery = 2 * time.Minute
	updateRunner = vmVerifyRunner(updateRunner)
	vmFaultHook = vmFault
}

// vmFault runs the fault $HOME/p18/fault names when its stage comes and its version is this one.
func vmFault(stage string) {
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, "p18", "fault"))
	if err != nil {
		return
	}
	mode, ver, ok := strings.Cut(strings.TrimSpace(string(b)), " ")
	if !ok || ver != Version {
		return
	}
	switch {
	case mode == "exit-before-daemon" && stage == "before-daemon", mode == "exit-after-daemon" && stage == "after-daemon":
		updateLogf("vmpreview: fault %s", mode)
		os.Exit(3)
	case mode == "hang-before-quit" && stage == "before-quit":
		updateLogf("vmpreview: fault %s", mode)
		select {}
	}
}

var vmAdhocOnce sync.Once

// vmVerifyRunner answers codesign and spctl with success when the VM's verify-mode file says
// adhoc, and runs every other tool (hdiutil, ditto, xattr, plutil, open, the staged binary) for
// real. The file is read on every call, so a row can flip the mode without a restart.
func vmVerifyRunner(base updater.Runner) updater.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if b, err := os.ReadFile(vmVerifyModePath()); err == nil && strings.TrimSpace(string(b)) == "adhoc" {
			if out, ok := vmFakeVerify(name, args); ok {
				vmAdhocOnce.Do(func() { log.Printf("vmpreview: verify-mode adhoc") })
				return out, nil
			}
		}
		return base(ctx, name, args...)
	}
}

func vmVerifyModePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "p18", "verify-mode")
}

// vmFakeVerify is the faked answer for one codesign or spctl call; ok is false for anything else.
func vmFakeVerify(name string, args []string) (out []byte, ok bool) {
	if len(args) == 0 {
		return nil, false
	}
	path := args[len(args)-1]
	switch name {
	case "/usr/bin/codesign":
		switch args[0] {
		case "--verify":
			return nil, true
		case "-dvv":
			id := "app.portty.Portty"
			if strings.HasSuffix(path, ".dmg") {
				id = strings.TrimSuffix(filepath.Base(path), ".dmg")
			}
			return []byte("Identifier=" + id + "\nTeamIdentifier=" + updater.TeamID + "\n"), true
		}
	case "/usr/sbin/spctl":
		return []byte(path + ": accepted\nsource=Notarized Developer ID\n"), true
	}
	return nil, false
}
