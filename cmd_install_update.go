package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/updater"
)

// installUpdateCmd is internal, so not in the usage: only the GUI's update spawns it (cmd_daemon.go
// has the same rule for daemon --takeover).
const installUpdateCmd = "install-update"

// runInstallUpdateFunc is a var so the dispatch can be tested without running an installer.
var runInstallUpdateFunc = runInstallUpdate

// runInstallUpdate is `portty install-update JOBFILE`: the detached copy of this binary that
// swaps the staged app in after the GUI quit (UPD-05). Its stderr is update.log. Exit 0 only for
// an updated install.
func runInstallUpdate(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "portty: install-update takes one job file")
		return 2
	}
	jobPath := args[0]
	job, err := updater.ReadJob(jobPath)
	if err != nil {
		log.Printf("install-update: %v", err)
		return 1
	}
	jobDir := filepath.Dir(jobPath)
	// First effect: tells the GUI the installer is running, so it may quit.
	if err := os.WriteFile(filepath.Join(jobDir, updater.StartedMarker), nil, 0o600); err != nil {
		log.Printf("install-update: started marker not written")
		return 1
	}
	// The run folder holds the job and this running copy; remove it only if it is ours.
	removeRunDir := func() {
		if filepath.Dir(jobDir) == updateWorkDir() && strings.HasPrefix(filepath.Base(jobDir), "run-") {
			_ = os.RemoveAll(jobDir)
		}
	}
	sock := daemon.DefaultSocketPath()
	// One client for the whole health poll. The exiting old daemon drops its connections (see
	// answersAfterDrop in internal/daemon/client.go) and the pooled transport then dials the
	// socket's new owner; a stale answer is only the old version, so it delays health, never fakes it.
	dc := daemon.NewDaemonClient(sock)
	env := updater.InstallerEnv{
		Run:     updateRunner,
		Swap:    updater.Swap,
		Open:    openApp,
		Alive:   pidAlive,
		Signal:  syscall.Kill,
		GUIPids: realGUIPids,
		Trash:   trashItem,
		DaemonVersion: func() (string, error) {
			h, err := dc.HealthInfo()
			return h.Version, err
		},
		RemoveRunDir: removeRunDir,
		ResultPath:   installResultPath(),
		Now:          time.Now,
		Sleep:        time.Sleep,
		Logf:         log.Printf,
	}
	ctx, cancel := context.WithTimeout(context.Background(), updater.QuitWait+updater.HealthTimeout+2*time.Minute)
	defer cancel()
	res := updater.RunInstaller(ctx, job, env)
	removeRunDir() // the paths that never reach the mover's DMG Trash; a no-op otherwise
	if res.Outcome == updater.OutcomeUpdated {
		return 0
	}
	return 1
}

// openApp is a plain `open APP`, no -n: LaunchServices never starts a second copy of a running app.
// Each env entry (NAME=VALUE) is passed with open's --env to the launched app.
func openApp(ctx context.Context, app string, env ...string) error {
	if testing.Testing() {
		return errors.New("refusing to open an app from a test binary")
	}
	var args []string
	for _, e := range env {
		args = append(args, "--env", e)
	}
	return exec.CommandContext(ctx, "/usr/bin/open", append(args, app)...).Run()
}

// pidAlive is kill(pid, 0): EPERM still means the process exists.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// realGUIPids lists this user's Portty GUIs running from exe (updater.ParseGUIPids over ps
// output); the installer health-checks and signals only these.
func realGUIPids(exe string) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/bin/ps", "-axww", "-o", "pid=,uid=,args=").Output()
	if err != nil {
		return nil, err
	}
	return updater.ParseGUIPids(out, exe, os.Getuid()), nil
}

func installResultPath() string { return filepath.Join(configDir(), "update-result.json") }

func updateWorkDir() string { return filepath.Join(configDir(), "update") }
