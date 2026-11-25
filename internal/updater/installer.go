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
	OutcomeRolledBack = "rolled-back" // produced from 18-03

	StartedMarker = "started" // the installer's first effect, in the job file's folder

	QuitWait      = 20 * time.Second // how long the installer waits for the old GUI to exit
	HealthTimeout = 60 * time.Second // the new version's daemon must answer within this
	HealthStable  = 3 * time.Second  // and keep answering this long
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

// ReadJob reads and validates the job file at path.
func ReadJob(path string) (Job, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return Job{}, jobErr("unreadable")
	}
	defer func() { _ = f.Close() }()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return Job{}, jobErr("not a regular file")
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return Job{}, jobErr("unreadable")
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
	if newer, err := Compare(j.NewVersion, j.OldVersion); err != nil || newer <= 0 {
		return Job{}, jobErr("versions")
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
func TakeResult(path string) (Result, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	}
	_ = os.Remove(path)
	if err != nil {
		return Result{}, false, jobErr("result unreadable")
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil || r.V != 1 {
		return Result{}, false, jobErr("result malformed")
	}
	return r, true, nil
}

// InstallerEnv is every effect RunInstaller has, so tests run it with fakes. Signal, GUIPids and
// Trash have no caller until 18-03 (rollback) and 18-05 (Trash the old bundle); they are declared
// now so the one fake env TestSelfInstall_UpdateHappyPath builds serves every later plan.
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

// RunInstaller waits for the old GUI to exit, re-verifies the staged app, swaps it in, relaunches
// it and waits for its daemon to report the new version; then it deletes the DMG. Every outcome
// is written to env.ResultPath. The old bundle stays at j.Staged. Logs carry step names only.
func RunInstaller(ctx context.Context, j Job, env InstallerEnv) Result {
	r := Result{V: 1, OldVersion: j.OldVersion, NewVersion: j.NewVersion, DMG: j.DMG}
	finish := func(outcome, reason string) Result {
		r.Outcome, r.Reason = outcome, reason
		if err := WriteResult(env.ResultPath, r); err != nil {
			env.Logf("installer: result not written")
		}
		env.Logf("installer: %s %s", outcome, reason)
		return r
	}
	reopen := func() { // the old GUI is gone and the old bundle is still in place: bring it back
		if err := env.Open(ctx, j.Target); err != nil {
			env.Logf("installer: reopen failed")
		}
	}

	quitBy := env.Now().Add(QuitWait)
	for env.Alive(j.OldPID) {
		if ctx.Err() != nil || !env.Now().Before(quitBy) {
			return finish(OutcomeFailed, "quit-timeout")
		}
		env.Sleep(installerPoll)
	}
	if err := VerifyApp(ctx, env.Run, j.Staged); err != nil {
		env.Logf("installer: %v", err)
		reopen()
		return finish(OutcomeFailed, "reverify")
	}
	if err := env.Swap(j.Staged, j.Target); err != nil {
		env.Logf("installer: swap failed")
		reopen()
		return finish(OutcomeFailed, "swap")
	}
	if err := env.Open(ctx, j.Target); err != nil {
		env.Logf("installer: open failed")
		return finish(OutcomeFailed, "open")
	}

	want := "v" + j.NewVersion
	deadline := env.Now().Add(HealthTimeout)
	var since time.Time // when the daemon first answered with the new version, without a break
	for {
		if v, err := env.DaemonVersion(); err == nil && v == want {
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
			return finish(OutcomeFailed, "health") // 18-03 turns this into a rollback
		}
		env.Sleep(installerPoll)
	}
	if err := os.Remove(j.DMG); err != nil && !errors.Is(err, os.ErrNotExist) {
		env.Logf("installer: dmg not removed")
	}
	return finish(OutcomeUpdated, "")
}
