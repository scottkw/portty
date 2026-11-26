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
	sock := daemon.DefaultSocketPath()
	env := updater.InstallerEnv{
		Run:     updateRunner,
		Swap:    updater.Swap,
		Open:    openApp,
		Alive:   pidAlive,
		Signal:  syscall.Kill,
		GUIPids: realGUIPids,
		Trash:   trashItem,
		DaemonVersion: func() (string, error) {
			h, err := daemon.NewDaemonClient(sock).HealthInfo()
			return h.Version, err
		},
		ResultPath: installResultPath(),
		Now:        time.Now,
		Sleep:      time.Sleep,
		Logf:       log.Printf,
	}
	ctx, cancel := context.WithTimeout(context.Background(), updater.QuitWait+updater.HealthTimeout+2*time.Minute)
	defer cancel()
	res := updater.RunInstaller(ctx, job, env)
	// The run folder holds the job and this running copy; remove it only if it is ours.
	if filepath.Dir(jobDir) == updateWorkDir() && strings.HasPrefix(filepath.Base(jobDir), "run-") {
		_ = os.RemoveAll(jobDir)
	}
	if res.Outcome == updater.OutcomeUpdated {
		return 0
	}
	return 1
}

// openApp is a plain `open APP`, no -n: LaunchServices never starts a second copy of a running app.
func openApp(ctx context.Context, app string) error {
	if testing.Testing() {
		return errors.New("refusing to open an app from a test binary")
	}
	return exec.CommandContext(ctx, "/usr/bin/open", app).Run()
}

// pidAlive is kill(pid, 0): EPERM still means the process exists.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// realGUIPids lists the Portty GUIs running from exe (updater.ParseGUIPids over ps output); the
// installer signals only these.
func realGUIPids(exe string) ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/bin/ps", "-axww", "-o", "pid=,args=").Output()
	if err != nil {
		return nil, err
	}
	return updater.ParseGUIPids(out, exe), nil
}

func installResultPath() string { return filepath.Join(configDir(), "update-result.json") }

func updateWorkDir() string { return filepath.Join(configDir(), "update") }
