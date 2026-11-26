package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// siRelease is a fake Runner that answers like a genuine release for every tool the full install
// pipeline calls, so later plans add steps without editing this file. fail names one step to
// break: a tool base name (ditto, hdiutil, spctl), dvv-app (the app's codesign -dvv), detach
// (every hdiutil detach), xattr-list (the follow-up xattr -lr), or smoke (the --version run);
// failOut is the output returned with that failure. Every call is recorded.
type siRelease struct {
	mu      sync.Mutex
	calls   []string
	fail    string
	failOut string
	version string // the version the mounted app carries; "" means 9.9.9
}

func (f *siRelease) ver() string {
	if f.version == "" {
		return "9.9.9"
	}
	return f.version
}

func (f *siRelease) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// called counts recorded calls whose command line contains all of subs.
func (f *siRelease) called(subs ...string) int {
	n := 0
	for _, c := range f.recorded() {
		if !slices.ContainsFunc(subs, func(s string) bool { return !strings.Contains(c, s) }) {
			n++
		}
	}
	return n
}

func (f *siRelease) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	f.mu.Unlock()
	last := ""
	if len(args) > 0 {
		last = args[len(args)-1]
	}
	broken := func(step string) ([]byte, error) {
		if f.fail == step {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return nil, nil
	}
	switch base := filepath.Base(name); {
	case base == "codesign" && slices.Contains(args, "-dvv"):
		if strings.HasSuffix(last, ".dmg") {
			return []byte("Identifier=" + strings.TrimSuffix(filepath.Base(last), ".dmg") + "\nTeamIdentifier=" + TeamID + "\n"), nil
		}
		if f.fail == "dvv-app" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return []byte("Identifier=app.portty.Portty\nTeamIdentifier=" + TeamID + "\n"), nil
	case base == "codesign":
		return broken("codesign")
	case base == "spctl":
		if f.fail == "spctl" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return []byte(last + ": accepted\nsource=Notarized Developer ID\n"), nil
	case base == "xattr":
		if slices.Contains(args, "-lr") {
			if f.fail == "xattr-list" {
				return []byte(last + ": com.apple.quarantine\n"), nil
			}
			return nil, nil
		}
		return broken("xattr")
	case base == "hdiutil" && args[0] == "attach":
		if f.fail == "hdiutil" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		i := slices.Index(args, "-mountpoint")
		app := filepath.Join(args[i+1], BundleName)
		if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(app, "version"), []byte(f.ver()), 0o644)
	case base == "hdiutil" && args[0] == "detach":
		// fail "detach" is an hdiutil that reports an error although the volume is gone.
		err := os.RemoveAll(filepath.Join(last, BundleName))
		if f.fail == "detach" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return nil, err
	case base == "ditto":
		if f.fail == "ditto" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return nil, os.CopyFS(args[len(args)-1], os.DirFS(args[len(args)-2]))
	case base == "plutil":
		switch {
		case slices.Contains(args, "CFBundleShortVersionString"):
			return []byte(f.ver() + "\n"), nil
		case slices.Contains(args, "LSMinimumSystemVersion"):
			return []byte("14.0\n"), nil
		}
		return nil, nil
	case base == "sw_vers":
		return []byte("27.2\n"), nil
	case strings.HasSuffix(name, "/"+BundleExecutable) && slices.Contains(args, "--version"):
		if f.fail == "smoke" {
			return []byte(f.failOut), errors.New("exit status 1")
		}
		return []byte("v" + f.ver() + "\n"), nil
	case base == "open" && slices.Contains(args, "-R"): // Reveal; every other open must go through the installer's env
		return broken("open")
	case base == "osascript": // Notify
		return broken("osascript")
	}
	return nil, errors.New("siRelease: unexpected call " + name)
}

// siBundle makes a fake app at dir/Portty.app whose version marker holds version.
func siBundle(t *testing.T, dir, version string) string {
	t.Helper()
	app := filepath.Join(dir, BundleName)
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "version"), []byte(version), 0o644); err != nil {
		t.Fatal(err)
	}
	return app
}

func siMarker(t *testing.T, app string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(app, "version"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// siFix is a Service wired to siRelease and a fake installed bundle, with the installer run
// in-process by StartInstaller.
type siFix struct {
	svcFix
	rel      *siRelease
	root     string // holds the installed bundle and, beside it, any staging folder
	target   string
	envDo    func(env *InstallerEnv) // lets a test adjust the installer's env before it runs
	started  int
	result   Result
	resPath  string
	opened   []string
	mu       sync.Mutex
	daemonV  string
	startErr error
}

func newSiFix(t *testing.T) *siFix {
	t.Helper()
	f := &siFix{svcFix: newSvc(t), rel: &siRelease{}, root: t.TempDir(), daemonV: "v1.0.0"}
	f.target = siBundle(t, f.root, "1.0.0")
	f.resPath = filepath.Join(t.TempDir(), "update-result.json")
	f.svc.Run = f.rel.run
	f.svc.Bundle = f.target
	f.svc.StartInstaller = func(j Job) error {
		f.started++
		if f.startErr != nil {
			return f.startErr
		}
		jobPath := filepath.Join(t.TempDir(), "job.json")
		if err := WriteJob(jobPath, j); err != nil {
			return err
		}
		j, err := ReadJob(jobPath)
		if err != nil {
			return err
		}
		now := time.Unix(1000, 0)
		env := InstallerEnv{
			Run:  f.rel.run,
			Swap: Swap,
			Open: func(_ context.Context, app string, _ ...string) error {
				f.mu.Lock()
				defer f.mu.Unlock()
				f.opened = append(f.opened, app)
				f.daemonV = "v" + j.NewVersion
				return nil
			},
			Alive:  func(int) bool { return false },
			Signal: func(int, syscall.Signal) error { return nil },
			GUIPids: func(string) ([]int, error) {
				return []int{4242}, nil
			},
			DaemonVersion: func() (string, error) {
				f.mu.Lock()
				defer f.mu.Unlock()
				return f.daemonV, nil
			},
			Trash:      func(string) error { return nil },
			ResultPath: f.resPath,
			Now:        func() time.Time { return now },
			Sleep:      func(d time.Duration) { now = now.Add(d) },
			Logf:       func(string, ...any) {},
		}
		if f.envDo != nil {
			f.envDo(&env)
		}
		f.result = RunInstaller(context.Background(), j, env)
		return nil
	}
	f.gh.set(f.gh.rel("9.9.9"), 0)
	return f
}

// download checks for and downloads the 9.9.9 release.
func (f *siFix) download(t *testing.T) Status {
	t.Helper()
	if _, err := f.svc.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, _ := f.svc.Download()
	return st
}

func siOrdered(states string, want ...string) bool {
	rest := strings.Split(states, ",")
	for _, w := range want {
		i := slices.Index(rest, w)
		if i < 0 {
			return false
		}
		rest = rest[i+1:]
	}
	return true
}

func TestSelfInstall_UpdateHappyPath(t *testing.T) {
	f := newSiFix(t)
	f.download(t)

	if got := f.ev.states(); !siOrdered(got, "downloading", "verifying", "installing", "relaunching") {
		t.Errorf("emitted %s", got)
	}
	if got := siMarker(t, f.target); got != "9.9.9" {
		t.Errorf("installed marker = %q, want 9.9.9", got)
	}
	// The swap leaves the old bundle in the staging folder it came from, for 18-05 to trash.
	old, _ := filepath.Glob(filepath.Join(f.root, StagingPrefix+"*", BundleName))
	if len(old) != 1 || siMarker(t, old[0]) != "1.0.0" {
		t.Errorf("old bundle aside = %v", old)
	}
	if m, _ := filepath.Glob(filepath.Join(f.dir, "*.dmg")); len(m) != 0 {
		t.Errorf("DMG left in the cache: %v", m)
	}
	if m, _ := filepath.Glob(filepath.Join(f.dir, "mnt-*")); len(m) != 0 {
		t.Errorf("mount folder left: %v", m)
	}
	if f.result.Outcome != OutcomeUpdated || f.result.OldVersion != "1.0.0" || f.result.NewVersion != "9.9.9" {
		t.Errorf("result = %+v", f.result)
	}
	if len(f.opened) != 1 || f.opened[0] != f.target {
		t.Errorf("Open calls = %v", f.opened)
	}
	if f.rel.called("hdiutil", "detach") != 1 {
		t.Errorf("image not detached exactly once: %v", f.rel.recorded())
	}
}

func TestSwap_ExchangesBundles(t *testing.T) {
	root := t.TempDir()
	a := siBundle(t, filepath.Join(root, "a"), "new")
	b := siBundle(t, filepath.Join(root, "b"), "old")
	if err := Swap(a, b); err != nil {
		t.Fatal(err)
	}
	if siMarker(t, b) != "new" || siMarker(t, a) != "old" {
		t.Errorf("after swap: target %q, staged %q", siMarker(t, b), siMarker(t, a))
	}
	missing := filepath.Join(root, "c", BundleName)
	if err := os.MkdirAll(filepath.Dir(missing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Swap(b, missing); err != nil {
		t.Fatal(err)
	}
	if siMarker(t, missing) != "new" {
		t.Errorf("plain rename marker = %q", siMarker(t, missing))
	}
	if _, err := os.Lstat(b); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("staged path still exists: %v", err)
	}
}

func TestSafeRunner_RefusesUnderTest(t *testing.T) {
	ran := false
	base := func(context.Context, string, ...string) ([]byte, error) { ran = true; return nil, nil }
	run := SafeRunner(base)
	for _, tool := range []string{"/usr/bin/hdiutil", "/usr/bin/open", "/usr/bin/ditto", "/usr/bin/osascript"} {
		if _, err := run(context.Background(), tool, "x"); err == nil || !strings.Contains(err.Error(), filepath.Base(tool)) {
			t.Errorf("%s: err = %v", tool, err)
		}
	}
	if ran {
		t.Error("a refused tool reached the base runner")
	}
	if _, err := run(context.Background(), "/usr/bin/codesign", "x"); err != nil || !ran {
		t.Errorf("codesign: err = %v, ran = %v", err, ran)
	}
}

// siLeftovers lists .Portty-update-* folders beside the target and mnt-* folders under the cache.
func siLeftovers(f *siFix) []string {
	var left []string
	for _, pat := range []string{filepath.Join(f.root, StagingPrefix+"*"), filepath.Join(f.dir, "mnt-*")} {
		m, _ := filepath.Glob(pat)
		left = append(left, m...)
	}
	return left
}

func TestSelfInstall_StageFailureReveals(t *testing.T) {
	for _, c := range []struct{ name, fail, out string }{
		{"attach", "hdiutil", ""},
		{"app team check", "dvv-app", "Identifier=app.portty.Portty\n"},
		{"ditto", "ditto", ""},
		{"detach", "detach", ""},
		{"quarantine still listed", "xattr-list", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			detachPause = 0
			f := newSiFix(t)
			f.rel.fail, f.rel.failOut = c.fail, c.out
			st := f.download(t)
			if st.State != "revealed" || st.Error == "" {
				t.Errorf("status = %+v, want revealed with an error text", st)
			}
			if f.started != 0 {
				t.Errorf("StartInstaller ran %d times", f.started)
			}
			if n := f.rel.called("/usr/bin/open", "-R"); n != 1 {
				t.Errorf("Reveal ran %d times", n)
			}
			if n := f.rel.called("xattr", "-w", "com.apple.quarantine"); n != 2 { // Verify, then the fallback
				t.Errorf("quarantine written %d times, want 2", n)
			}
			if left := siLeftovers(f); len(left) != 0 {
				t.Errorf("left behind: %v", left)
			}
			if m, _ := filepath.Glob(filepath.Join(f.dir, "*.dmg")); len(m) != 1 {
				t.Errorf("DMG in the cache = %v, want the kept download", m)
			}
			if siMarker(t, f.target) != "1.0.0" {
				t.Error("installed app changed")
			}
			if c.fail == "dvv-app" && f.rel.called("xattr", "-dr", StagingPrefix) != 0 {
				t.Errorf("an app that failed its check was un-quarantined: %v", f.rel.recorded())
			}
		})
	}
}

func TestSelfInstall_StartFailureReveals(t *testing.T) {
	f := newSiFix(t)
	f.startErr = errors.New("spawn failed")
	st := f.download(t)
	if st.State != "revealed" || st.Error != installFailureText(errInstallerStart, true) {
		t.Errorf("status = %+v", st)
	}
	if f.started != 1 {
		t.Errorf("StartInstaller ran %d times", f.started)
	}
	if left := siLeftovers(f); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	if n := f.rel.called("xattr", "-w", "com.apple.quarantine"); n != 2 {
		t.Errorf("quarantine written %d times, want 2", n)
	}
}

func TestSelfInstall_RevealFailureAvailable(t *testing.T) {
	f := newSiFix(t)
	f.startErr = errors.New("spawn failed")
	f.rel.fail = "open"
	st := f.download(t)
	if st.State != "available" || st.Error != "Portty couldn't show the download in Finder." {
		t.Errorf("status = %+v", st)
	}
	if n := f.rel.called("xattr", "-w", "com.apple.quarantine"); n != 2 {
		t.Errorf("quarantine written %d times, want 2 (it goes back before Reveal)", n)
	}
}

func TestSelfInstall_SecondDownloadIsNoOp(t *testing.T) {
	for _, stage := range []string{"installing", "relaunching"} {
		t.Run(stage, func(t *testing.T) {
			f := newSiFix(t)
			entered, release := make(chan struct{}), make(chan struct{})
			block := func() { close(entered); <-release }
			if stage == "installing" {
				var once sync.Once
				f.svc.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
					if strings.HasSuffix(name, "hdiutil") && args[0] == "attach" {
						once.Do(block)
					}
					return f.rel.run(ctx, name, args...)
				}
			} else {
				f.svc.StartInstaller = func(Job) error { f.started++; block(); return nil }
			}
			if _, err := f.svc.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
			done := make(chan Status)
			go func() { st, _ := f.svc.Download(); done <- st }()
			<-entered
			checks := len(f.gh.checks())
			want := f.svc.Status()
			if want.State != stage {
				t.Fatalf("state = %s, want %s", want.State, stage)
			}
			if st, _ := f.svc.Download(); st != want {
				t.Errorf("second Download = %+v, want %+v", st, want)
			}
			if st, err := f.svc.Check(context.Background()); st != want || err != nil || len(f.gh.checks()) != checks {
				t.Errorf("Check during install = %+v, %v (GitHub asked: %v)", st, err, len(f.gh.checks()) != checks)
			}
			close(release)
			<-done
			if stage == "relaunching" && f.started != 1 {
				t.Errorf("StartInstaller ran %d times", f.started)
			}
			if n := f.rel.called("hdiutil", "attach"); n > 1 {
				t.Errorf("image attached %d times", n)
			}
		})
	}
}

// siJobFix makes a valid job on disk: an installed bundle, a staging sibling and a DMG name.
func siJobFix(t *testing.T) (Job, string) {
	t.Helper()
	root := t.TempDir()
	target := siBundle(t, root, "1.0.0")
	staged := siBundle(t, filepath.Join(root, StagingPrefix+"abc123"), "9.9.9")
	j := Job{V: JobVersion, Kind: KindUpdate, OldPID: os.Getpid(), Target: target, Staged: staged,
		OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: filepath.Join(root, "updates", "portty-v9.9.9-darwin-universal.dmg")}
	return j, root
}

// siWriteFile writes b to a fresh file at path with exactly mode.
func siWriteFile(t *testing.T, path string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestJob_RoundTripAndRejects(t *testing.T) {
	j, root := siJobFix(t)
	path := filepath.Join(t.TempDir(), "job.json")
	if err := WriteJob(path, j); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadJob(path); err != nil || got != j {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
	if err := WriteJob(path, j); err == nil {
		t.Error("WriteJob overwrote an existing job file")
	}

	field := func(k string, v any) func(Job, string) ([]byte, os.FileMode) {
		return func(j Job, _ string) ([]byte, os.FileMode) {
			var m map[string]any
			b, _ := json.Marshal(j)
			_ = json.Unmarshal(b, &m)
			m[k] = v
			b, _ = json.Marshal(m)
			return b, 0o600
		}
	}
	valid := func(mutate func(*Job)) func(Job, string) ([]byte, os.FileMode) {
		return func(j Job, _ string) ([]byte, os.FileMode) {
			mutate(&j)
			b, _ := json.Marshal(j)
			return b, 0o600
		}
	}
	sub := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }

	for _, c := range []struct {
		name string
		make func(j Job, root string) ([]byte, os.FileMode)
		pre  func(t *testing.T) // changes the disk before the job is read
	}{
		{"mode 0644", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o644 }, nil},
		{"mode 0660", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o660 }, nil},
		{"empty", func(Job, string) ([]byte, os.FileMode) { return nil, 0o600 }, nil},
		{"over 64 KiB", func(j Job, _ string) ([]byte, os.FileMode) {
			b, _ := json.Marshal(j)
			return append(b, bytes.Repeat([]byte(" "), 64<<10)...), 0o600
		}, nil},
		{"not json", func(Job, string) ([]byte, os.FileMode) { return []byte("{"), 0o600 }, nil},
		{"unknown field", field("extra", "x"), nil},
		{"v 2", field("v", 2), nil},
		{"kind mover", field("kind", "mover"), nil},
		{"oldPid 1", field("oldPid", 1), nil},
		{"oldPid 0", field("oldPid", 0), nil},
		{"relative target", valid(func(j *Job) { j.Target = "Portty.app" }), nil},
		{"relative staged", valid(func(j *Job) { j.Staged = "x/Portty.app" }), nil},
		{"unclean target", valid(func(j *Job) { j.Target = root + "/x/../" + BundleName }), nil},
		{"unclean staged", valid(func(j *Job) {
			j.Staged = root + "/" + StagingPrefix + "abc123/../" + StagingPrefix + "abc123/" + BundleName
		}), nil},
		{"target base", valid(func(j *Job) { j.Target = sub("Other.app") }), func(t *testing.T) { siBundle2(t, sub("Other.app")) }},
		{"staged base", valid(func(j *Job) { j.Staged = sub(StagingPrefix+"abc123", "Other.app") }), func(t *testing.T) { siBundle2(t, sub(StagingPrefix+"abc123", "Other.app")) }},
		{"staged not beside target", valid(func(j *Job) { j.Staged = sub("elsewhere", StagingPrefix+"abc123", BundleName) }), func(t *testing.T) { siBundle(t, sub("elsewhere", StagingPrefix+"abc123"), "9.9.9") }},
		{"staging dir prefix", valid(func(j *Job) { j.Staged = sub("stage", BundleName) }), func(t *testing.T) { siBundle(t, sub("stage"), "9.9.9") }},
		{"target missing", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o600 }, func(t *testing.T) { _ = os.RemoveAll(sub(BundleName)) }},
		{"target symlink", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o600 }, func(t *testing.T) {
			real := t.TempDir()
			_ = os.RemoveAll(sub(BundleName))
			if err := os.Symlink(real, sub(BundleName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"staged missing", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o600 }, func(t *testing.T) { _ = os.RemoveAll(sub(StagingPrefix + "abc123")) }},
		{"staged symlink", func(j Job, _ string) ([]byte, os.FileMode) { b, _ := json.Marshal(j); return b, 0o600 }, func(t *testing.T) {
			real := t.TempDir()
			_ = os.RemoveAll(sub(StagingPrefix+"abc123", BundleName))
			if err := os.Symlink(real, sub(StagingPrefix+"abc123", BundleName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"old version", valid(func(j *Job) { j.OldVersion = "one" }), nil},
		{"new version", valid(func(j *Job) { j.NewVersion = "9.9" }), nil},
		{"new version equal", valid(func(j *Job) { j.NewVersion = j.OldVersion }), nil},
		{"new version older", valid(func(j *Job) { j.NewVersion = "0.9.0" }), nil},
		{"relative dmg", valid(func(j *Job) { j.DMG = "portty-v9.9.9-darwin-universal.dmg" }), nil},
		{"dmg name", valid(func(j *Job) { j.DMG = sub("evil.dmg") }), nil},
		{"dmg not a dmg", valid(func(j *Job) { j.DMG = sub("portty-v9.9.9-darwin-universal.zip") }), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			j, root = siJobFix(t) // sub reads root: every row gets a fresh tree
			if c.pre != nil {
				c.pre(t)
			}
			b, mode := c.make(j, root)
			p := filepath.Join(t.TempDir(), "job.json")
			siWriteFile(t, p, b, mode)
			if _, err := ReadJob(p); !errors.Is(err, ErrJob) {
				t.Errorf("ReadJob = %v, want ErrJob", err)
			}
		})
	}

	t.Run("symlinked job file", func(t *testing.T) {
		j, _ := siJobFix(t)
		real := filepath.Join(t.TempDir(), "real.json")
		if err := WriteJob(real, j); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(t.TempDir(), "job.json")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadJob(link); !errors.Is(err, ErrJob) {
			t.Errorf("ReadJob = %v, want ErrJob", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		if _, err := ReadJob(t.TempDir()); !errors.Is(err, ErrJob) {
			t.Errorf("ReadJob = %v, want ErrJob", err)
		}
	})
	t.Run("foreign owner", func(t *testing.T) {
		j, _ := siJobFix(t)
		p := filepath.Join(t.TempDir(), "job.json")
		if err := WriteJob(p, j); err != nil {
			t.Fatal(err)
		}
		defer func(old func() int) { wantUID = old }(wantUID)
		wantUID = func() int { return os.Getuid() + 1 }
		if _, err := ReadJob(p); !errors.Is(err, ErrJob) {
			t.Errorf("ReadJob = %v, want ErrJob", err)
		}
	})
}

// siBundle2 makes an empty directory at path (a bundle-shaped directory with another name).
func siBundle2(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResult_TakeOnceAndRejects(t *testing.T) {
	good := Result{V: 1, Outcome: OutcomeUpdated, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: "/x/portty-v9.9.9-darwin-universal.dmg"}
	path := filepath.Join(t.TempDir(), "update-result.json")
	if r, ok, err := TakeResult(path); ok || err != nil || r != (Result{}) {
		t.Fatalf("missing file = %+v, %v, %v", r, ok, err)
	}
	if err := WriteResult(path, good); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("result file = %v, %v; want 0600", fi, err)
	}
	if r, ok, err := TakeResult(path); !ok || err != nil || r != good {
		t.Fatalf("TakeResult = %+v, %v, %v", r, ok, err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("result file not removed")
	}
	if _, ok, _ := TakeResult(path); ok {
		t.Error("result returned twice")
	}

	body := func(mutate func(m map[string]any)) []byte {
		b, _ := json.Marshal(good)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		mutate(m)
		b, _ = json.Marshal(m)
		return b
	}
	for _, c := range []struct {
		name  string
		write func(t *testing.T, p string)
	}{
		{"symlink", func(t *testing.T, p string) {
			real := filepath.Join(t.TempDir(), "real.json")
			siWriteFile(t, real, body(func(map[string]any) {}), 0o600)
			if err := os.Symlink(real, p); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory", func(t *testing.T, p string) { siBundle2(t, p) }},
		{"group readable", func(t *testing.T, p string) { siWriteFile(t, p, body(func(map[string]any) {}), 0o640) }},
		{"other readable", func(t *testing.T, p string) { siWriteFile(t, p, body(func(map[string]any) {}), 0o604) }},
		{"empty", func(t *testing.T, p string) { siWriteFile(t, p, nil, 0o600) }},
		{"not json", func(t *testing.T, p string) { siWriteFile(t, p, []byte("nope"), 0o600) }},
		{"v 2", func(t *testing.T, p string) { siWriteFile(t, p, body(func(m map[string]any) { m["v"] = 2 }), 0o600) }},
		{"unknown outcome", func(t *testing.T, p string) {
			siWriteFile(t, p, body(func(m map[string]any) { m["outcome"] = "exploded" }), 0o600)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "update-result.json")
			c.write(t, p)
			if _, ok, err := TakeResult(p); ok || !errors.Is(err, ErrJob) {
				t.Errorf("TakeResult = %v, %v; want an ErrJob error", ok, err)
			}
			if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("refused file not removed: %v", err)
			}
		})
	}
	t.Run("foreign owner", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "update-result.json")
		if err := WriteResult(p, good); err != nil {
			t.Fatal(err)
		}
		defer func(old func() int) { wantUID = old }(wantUID)
		wantUID = func() int { return os.Getuid() + 1 }
		if _, ok, err := TakeResult(p); ok || !errors.Is(err, ErrJob) {
			t.Errorf("TakeResult = %v, %v", ok, err)
		}
	})
}

func TestSelfInstall_DMGAlreadyGone(t *testing.T) {
	f := newSiFix(t)
	f.envDo = func(env *InstallerEnv) {
		open := env.Open
		env.Open = func(ctx context.Context, app string, launchEnv ...string) error {
			if m, _ := filepath.Glob(filepath.Join(f.dir, "*.dmg")); len(m) != 1 {
				t.Errorf("DMG should still be there when the app opens: %v", m)
			}
			_ = os.Remove(filepath.Join(f.dir, DMGAssetName("9.9.9"))) // eject-and-delete by hand, mid-install
			return open(ctx, app, launchEnv...)
		}
	}
	f.download(t)
	if f.result.Outcome != OutcomeUpdated {
		t.Errorf("result = %+v, want updated", f.result)
	}
	if siMarker(t, f.target) != "9.9.9" {
		t.Error("new version not installed")
	}
}

func TestSelfInstall_HealthMissWritesFailed(t *testing.T) {
	f := newSiFix(t)
	f.envDo = func(env *InstallerEnv) {
		env.Open = func(context.Context, string, ...string) error { return nil } // the daemon stays at the old version
	}
	f.download(t)
	r := f.result // 18-03: a health miss rolls back
	if r.Outcome != OutcomeRolledBack || r.Reason != "health" || r.OldVersion != "1.0.0" || r.NewVersion != "9.9.9" {
		t.Errorf("result = %+v", r)
	}
	if m, _ := filepath.Glob(filepath.Join(f.dir, "*.dmg")); len(m) != 1 {
		t.Errorf("DMG should be kept when health fails: %v", m)
	}
	if got, ok, err := TakeResult(f.resPath); !ok || err != nil || got != r {
		t.Errorf("result file = %+v, %v, %v", got, ok, err)
	}
}

func TestStage_DetachIdempotent(t *testing.T) {
	detachPause = 0
	var calls []string
	mk := func(failFirst int, force bool) Runner {
		n := 0
		return func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			n++
			if n <= failFirst || (!force && slices.Contains(args, "-force")) {
				return nil, errors.New("exit status 1")
			}
			return nil, nil
		}
	}
	ctx := context.Background()

	gone := filepath.Join(t.TempDir(), "mnt-gone")
	if err := Detach(ctx, mk(0, true), gone); err != nil || len(calls) != 0 {
		t.Errorf("missing mount dir: err = %v, hdiutil calls = %v", err, calls)
	}

	dir := t.TempDir()
	calls = nil
	if err := Detach(ctx, mk(2, true), dir); err != nil || len(calls) != 3 {
		t.Errorf("fails twice then succeeds: err = %v, calls = %v", err, calls)
	}

	dir = t.TempDir()
	calls = nil
	err := Detach(ctx, mk(100, false), dir)
	if !errors.Is(err, ErrInstall) || !strings.Contains(err.Error(), "detach") {
		t.Errorf("keeps failing: err = %v", err)
	}
	if len(calls) != 4 || !strings.Contains(calls[3], "-force") || strings.Contains(strings.Join(calls[:3], ""), "-force") {
		t.Errorf("want 3 plain tries then one -force, got %v", calls)
	}
}
