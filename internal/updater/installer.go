package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The job file the GUI writes and the detached installer reads, and the result file the installer
// writes and the next GUI reads once (UPD-05..UPD-07).
const (
	JobVersion = 1
	KindUpdate = "update"

	OutcomeUpdated    = "updated"
	OutcomeFailed     = "failed"
	OutcomeRolledBack = "rolled-back"

	StartedMarker = "started" // the installer's first effect, in the job file's folder

	QuitWait      = 20 * time.Second // how long the installer waits for the old GUI to exit
	HealthTimeout = 60 * time.Second // the new version's daemon must answer within this
	HealthStable  = 3 * time.Second  // and keep answering this long

	StopWait = 5 * time.Second // SIGTERM to a GUI, then this long before SIGKILL
	KillWait = 2 * time.Second // and this long for SIGKILL to land

	// A relaunch right after the old GUI exits can be refused while LaunchServices still holds
	// the dead pid (-600 procNotFound, 18-VM-CHECKS.md row A9), so every relaunch is retried.
	OpenAttempts  = 6
	OpenRetryWait = time.Second
	OpenTimeout   = 10 * time.Second // per try, so a stuck open cannot eat the health window
)

// ErrJob wraps every refused job or result file. Messages never carry a path.
var ErrJob = errors.New("update job refused")

func jobErr(why string) error { return fmt.Errorf("%w: %s", ErrJob, why) }

// Job is the GUI's instruction to the installer. Only Portty writes it (0600, in a 0700 run-*
// folder), and ReadJob refuses anything else before the installer acts on it.
type Job struct {
	V          int    `json:"v"`
	Kind       string `json:"kind"`
	OldPID     int    `json:"oldPid"`
	Target     string `json:"target"`
	Staged     string `json:"staged"`
	OldVersion string `json:"oldVersion"`
	NewVersion string `json:"newVersion"`
	DMG        string `json:"dmg"`
	ImagePath  string `json:"imagePath,omitempty"`
	MountPoint string `json:"mountPoint,omitempty"`
}

// Result is what the installer reports. The next GUI takes it once with TakeResult.
type Result struct {
	V          int    `json:"v"`
	Outcome    string `json:"outcome"`
	OldVersion string `json:"oldVersion"`
	NewVersion string `json:"newVersion"`
	Reason     string `json:"reason"`
	KeepDaemon bool   `json:"keepDaemon"`
	DMG        string `json:"dmg"`
}

// wantUID is the owner a job or result file must have; a var so a test can fake a foreign owner
// without root.
var wantUID = os.Getuid

// WriteJob writes j to a new file at path (0600, never overwriting).
func WriteJob(path string, j Job) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// maxControlFile caps a job or result file; a real one is a few hundred bytes.
const maxControlFile = 64 << 10

// readOwnFile reads path only if Portty could have written it: a regular file (never a symlink or
// a FIFO), owned by wantUID, with no mode bits beyond 0600, between 1 byte and 64 KiB. Errors
// carry the reason and never the path.
func readOwnFile(path string) ([]byte, error) {
	// O_NOFOLLOW refuses a symlink; O_NONBLOCK keeps a FIFO from blocking the open.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, jobErr("unreadable")
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return nil, jobErr("not a regular file")
	}
	if fi.Mode().Perm()&^0o600 != 0 {
		return nil, jobErr("mode")
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != wantUID() {
		return nil, jobErr("owner")
	}
	if fi.Size() == 0 || fi.Size() > maxControlFile {
		return nil, jobErr("size")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxControlFile+1))
	if err != nil || len(b) > maxControlFile {
		return nil, jobErr("unreadable")
	}
	return b, nil
}

// isDir reports whether path is a real directory, not a symlink to one.
func isDir(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir()
}

// ReadJob reads and validates the job file at path (T-18-02): only a job Portty wrote for a real
// Portty.app and the staging folder beside it is returned.
func ReadJob(path string) (Job, error) {
	b, err := readOwnFile(path)
	if err != nil {
		return Job{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var j Job
	if err := dec.Decode(&j); err != nil {
		return Job{}, jobErr("malformed")
	}
	if j.V != JobVersion {
		return Job{}, jobErr("version")
	}
	if j.Kind != KindUpdate {
		return Job{}, jobErr("kind")
	}
	for _, p := range []string{j.Target, j.Staged} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || filepath.Base(p) != BundleName {
			return Job{}, jobErr("bundle path")
		}
	}
	if j.OldPID < 2 {
		return Job{}, jobErr("pid")
	}
	stagingDir := filepath.Dir(j.Staged)
	if filepath.Dir(stagingDir) != filepath.Dir(j.Target) || !strings.HasPrefix(filepath.Base(stagingDir), StagingPrefix) {
		return Job{}, jobErr("staging folder")
	}
	if !isDir(j.Target) || !isDir(j.Staged) {
		return Job{}, jobErr("bundle missing")
	}
	if newer, err := Compare(j.NewVersion, j.OldVersion); err != nil || newer <= 0 {
		return Job{}, jobErr("versions")
	}
	if match, _ := filepath.Match("portty-v*-darwin-*.dmg", filepath.Base(j.DMG)); !match || !filepath.IsAbs(j.DMG) || filepath.Clean(j.DMG) != j.DMG {
		return Job{}, jobErr("dmg")
	}
	return j, nil
}

// WriteResult writes r to path through a temp file and a rename, so a reader never sees half of it.
func WriteResult(path string, r Result) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-result-")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
}

// TakeResult reads the result file once and removes it. A missing file is (zero, false, nil).
// A file Portty did not write (see readOwnFile) or one that does not parse is removed and
// returned as an ErrJob error, so it cannot stay and trip every start.
func TakeResult(path string) (Result, bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	}
	b, err := readOwnFile(path)
	_ = os.Remove(path)
	if err != nil {
		return Result{}, false, err
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil || r.V != 1 {
		return Result{}, false, jobErr("result malformed")
	}
	switch r.Outcome {
	case OutcomeUpdated, OutcomeFailed, OutcomeRolledBack:
	default:
		return Result{}, false, jobErr("result outcome")
	}
	return r, true, nil
}

// InstallerEnv is every effect RunInstaller has, so tests run it with fakes. Signal is only ever
// sent to pids GUIPids returned (T-18-14); Trash only to the old bundle after health (18-05).
type InstallerEnv struct {
	Run           Runner
	Swap          func(staged, target string) error
	Open          func(ctx context.Context, app string) error
	Alive         func(pid int) bool
	Signal        func(pid int, sig syscall.Signal) error
	GUIPids       func(exe string) ([]int, error)
	DaemonVersion func() (string, error)
	Trash         func(path string) error
	ResultPath    string
	Now           func() time.Time
	Sleep         func(time.Duration)
	Logf          func(format string, args ...any)
}

const installerPoll = 200 * time.Millisecond

// ParseGUIPids returns the pids in `ps -o pid=,args=` output whose process is a Portty GUI run
// from exe: args exactly exe, or exe followed by a launch-services argument (-psn_, -NS, -Apple,
// the shapes main.launchMode starts the GUI for). The daemon, the installer and CLI clients run
// from the same executable carry a subcommand, so they are never listed and never signalled.
func ParseGUIPids(psOut []byte, exe string) []int {
	var pids []int
	for _, line := range strings.Split(string(psOut), "\n") {
		pidStr, args, ok := strings.Cut(strings.TrimSpace(line), " ")
		pid, err := strconv.Atoi(pidStr)
		if !ok || err != nil || pid < 2 {
			continue
		}
		args = strings.TrimLeft(args, " ")
		if args != exe {
			rest, ok := strings.CutPrefix(args, exe+" ")
			if !ok || !slices.ContainsFunc([]string{"-psn_", "-NS", "-Apple"}, func(p string) bool { return strings.HasPrefix(rest, p) }) {
				continue
			}
		}
		pids = append(pids, pid)
	}
	return pids
}

// relaunch opens app, up to OpenAttempts tries OpenRetryWait apart, each under OpenTimeout. The
// tries ignore ctx's own deadline: skipping a relaunch would leave no GUI, and each try is bounded.
func relaunch(ctx context.Context, env InstallerEnv, app string) error {
	var err error
	for i := 1; i <= OpenAttempts; i++ {
		octx, cancel := context.WithTimeout(context.WithoutCancel(ctx), OpenTimeout)
		err = env.Open(octx, app)
		cancel()
		if err == nil {
			return nil
		}
		env.Logf("installer: open attempt %d failed", i)
		if i < OpenAttempts {
			env.Sleep(OpenRetryWait)
		}
	}
	return err
}

// RunInstaller waits for the old GUI to exit (SIGKILL after QuitWait, only when ps proves it is the
// installed Portty's GUI), re-verifies the staged app, swaps it in, relaunches it and waits for a
// GUI plus a daemon steadily at the new version; then it deletes the DMG. If the new version never
// gets there, its GUI exits, or it never opens, its GUI is stopped, the old bundle is swapped back
// and relaunched, and the daemon is left alone (UPD-07). Every outcome is written to
// env.ResultPath before any relaunch of the old app. On success the old bundle goes to the Trash.
// Logs carry step names only.
func RunInstaller(ctx context.Context, j Job, env InstallerEnv) Result {
	r := Result{V: 1, OldVersion: j.OldVersion, NewVersion: j.NewVersion, DMG: j.DMG}
	exe := filepath.Join(j.Target, BundleExecutable)
	finish := func(outcome, reason string) Result {
		r.Outcome, r.Reason = outcome, reason
		if err := WriteResult(env.ResultPath, r); err != nil {
			env.Logf("installer: result not written")
		}
		env.Logf("installer: %s %s", outcome, reason)
		return r
	}
	// finishOpen writes the outcome, then relaunches whatever the target holds. A rollback whose
	// relaunch fails becomes failed (keepDaemon kept), so the next manual launch still says why.
	finishOpen := func(outcome, reason string) Result {
		finish(outcome, reason)
		if err := relaunch(ctx, env, j.Target); err != nil {
			if outcome == OutcomeRolledBack {
				finish(OutcomeFailed, reason)
			}
			env.Logf("installer: relaunch failed, result kept for the next launch")
		}
		return r
	}
	// waitGone polls until GUIPids lists none of pids or d passes, and returns those still listed.
	waitGone := func(pids []int, d time.Duration) []int {
		for by := env.Now().Add(d); ; env.Sleep(installerPoll) {
			if now, err := env.GUIPids(exe); err == nil {
				pids = slices.DeleteFunc(pids, func(p int) bool { return !slices.Contains(now, p) })
				if len(pids) == 0 {
					return nil
				}
			}
			if !env.Now().Before(by) {
				return pids
			}
		}
	}
	rollback := func(reason string) Result {
		// SIGTERM is the GUI-only quit (UPGR-05); a daemon that took over keeps the sessions.
		if pids, err := env.GUIPids(exe); err == nil && len(pids) > 0 {
			for _, p := range pids {
				_ = env.Signal(p, syscall.SIGTERM)
			}
			if left := waitGone(pids, StopWait); len(left) > 0 {
				for _, p := range left {
					_ = env.Signal(p, syscall.SIGKILL)
				}
				if len(waitGone(left, KillWait)) > 0 {
					env.Logf("installer: new GUI still running")
				}
			}
		}
		if err := env.Swap(j.Staged, j.Target); err != nil {
			env.Logf("installer: swap back failed")
			return finishOpen(OutcomeFailed, "swap-back") // the target still holds the verified new bundle
		}
		// The staging folder now holds the failed new copy Portty made, never the user's app.
		if dir := filepath.Dir(j.Staged); strings.HasPrefix(filepath.Base(dir), StagingPrefix) {
			if err := os.RemoveAll(dir); err != nil {
				env.Logf("installer: failed copy not removed")
			}
		}
		r.KeepDaemon = true
		return finishOpen(OutcomeRolledBack, reason)
	}

	quitBy := env.Now().Add(QuitWait)
	for env.Alive(j.OldPID) {
		if ctx.Err() != nil || !env.Now().Before(quitBy) {
			pids, err := env.GUIPids(exe)
			if err != nil || !slices.Contains(pids, j.OldPID) {
				return finish(OutcomeFailed, "quit-timeout") // not provably our GUI: touch nothing
			}
			_ = env.Signal(j.OldPID, syscall.SIGKILL)
			env.Logf("installer: old GUI killed after quit timeout")
			for by := env.Now().Add(KillWait); env.Alive(j.OldPID); env.Sleep(installerPoll) {
				if !env.Now().Before(by) {
					return finish(OutcomeFailed, "quit-timeout")
				}
			}
			break
		}
		env.Sleep(installerPoll)
	}
	if err := VerifyApp(ctx, env.Run, j.Staged); err != nil {
		env.Logf("installer: %v", err)
		return finishOpen(OutcomeFailed, "reverify")
	}
	if err := env.Swap(j.Staged, j.Target); err != nil {
		env.Logf("installer: swap failed")
		return finishOpen(OutcomeFailed, "swap")
	}
	if err := relaunch(ctx, env, j.Target); err != nil {
		env.Logf("installer: open failed")
		return rollback("open")
	}

	want := "v" + j.NewVersion
	deadline := env.Now().Add(HealthTimeout)
	var since time.Time // when a GUI and the new daemon were first both up, without a break
	seenGUI, noGUI := false, 0
	for {
		pids, perr := env.GUIPids(exe)
		gui := perr == nil && len(pids) > 0
		switch {
		case gui:
			seenGUI, noGUI = true, 0
		case perr == nil && seenGUI:
			if noGUI++; noGUI >= 2 {
				return rollback("gui-exited")
			}
		}
		if v, err := env.DaemonVersion(); err == nil && v == want && gui {
			if since.IsZero() {
				since = env.Now()
			}
			if env.Now().Sub(since) >= HealthStable {
				break
			}
		} else {
			since = time.Time{}
		}
		if ctx.Err() != nil || !env.Now().Before(deadline) {
			return rollback("health")
		}
		env.Sleep(installerPoll)
	}
	// Healthy: the old bundle (at j.Staged since the swap) goes to the Trash, recoverable by hand,
	// never deleted. Only a Portty.app inside a staging folder beside the target is trashed; a
	// refused Trash keeps it aside for the next GUI start's sweep to retry.
	if dir := filepath.Dir(j.Staged); env.Trash != nil && strings.HasPrefix(filepath.Base(dir), StagingPrefix) && filepath.Dir(dir) == filepath.Dir(j.Target) {
		if err := env.Trash(j.Staged); err != nil {
			env.Logf("installer: old copy kept aside: %v", err)
		} else if err := os.Remove(dir); err != nil {
			env.Logf("installer: staging folder not removed")
		}
	}
	if err := os.Remove(j.DMG); err != nil && !errors.Is(err, os.ErrNotExist) {
		env.Logf("installer: dmg not removed")
	}
	return finish(OutcomeUpdated, "")
}
