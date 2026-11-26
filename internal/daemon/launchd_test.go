package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// pidLineMissing as a fake job's pid prints a running job with no pid line, as
// a changed `launchctl print` format would.
const pidLineMissing = -1

// fakeLaunchd is a fake launchd domain behind the launchctl seam: it records
// every call, tracks loaded jobs (label -> pid) and disabled labels, and can
// fail one subcommand. It never touches the real launchd.
type fakeLaunchd struct {
	mu          sync.Mutex
	calls       [][]string
	jobs        map[string]int // loaded label -> pid
	disabled    map[string]bool
	failOn      map[string]error // subcommand -> error
	onKickstart func()
	bootouts    []bootoutRec
}

// bootoutRec is a booted-out job and the pid it had at the time.
type bootoutRec struct {
	label string
	pid   int
}

func newFakeLaunchd(t *testing.T) *fakeLaunchd {
	t.Helper()
	f := &fakeLaunchd{jobs: map[string]int{}, disabled: map[string]bool{}, failOn: map[string]error{}}
	t.Cleanup(SetLaunchctlForTest(f.run))
	return f
}

func (f *fakeLaunchd) run(_ context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(args))
	if err := f.failOn[args[0]]; err != nil {
		return []byte("simulated failure"), err
	}
	label := func(target string) string { return target[strings.LastIndex(target, "/")+1:] }
	switch args[0] {
	case "print":
		pid, ok := f.jobs[label(args[1])]
		if !ok {
			return []byte(`Could not find service "` + label(args[1]) + `" in domain for user gui: 501`), errors.New("exit status 113")
		}
		if pid == pidLineMissing {
			return fmt.Appendf(nil, "%s = {\n\tstate = running\n}\n", label(args[1])), nil
		}
		if pid > 0 {
			return fmt.Appendf(nil, "%s = {\n\tstate = running\n\tpid = %d\n}\n", label(args[1]), pid), nil
		}
		return []byte("job = {\n\tstate = not running\n}\n"), nil
	case "print-disabled":
		out := "disabled services = {\n\t\"com.example.other\" => enabled\n"
		for l, d := range f.disabled {
			if d {
				out += fmt.Sprintf("\t%q => disabled\n", l)
			}
		}
		return []byte(out + "}\n"), nil
	case "bootstrap":
		f.jobs[strings.TrimSuffix(filepath.Base(args[2]), ".plist")] = 0
	case "bootout":
		f.bootouts = append(f.bootouts, bootoutRec{label(args[1]), f.jobs[label(args[1])]})
		delete(f.jobs, label(args[1]))
	case "kickstart":
		if f.onKickstart != nil {
			f.onKickstart()
		}
	}
	return nil, nil
}

// mutating returns the recorded calls that are not print or print-disabled.
func (f *fakeLaunchd) mutating() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out [][]string
	for _, c := range f.calls {
		if c[0] != "print" && c[0] != "print-disabled" {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeLaunchd) subcommands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c[0])
	}
	return out
}

func (f *fakeLaunchd) reset() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

func (f *fakeLaunchd) setJob(label string, pid int) {
	f.mu.Lock()
	f.jobs[label] = pid
	f.mu.Unlock()
}

func (f *fakeLaunchd) hasJob(label string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.jobs[label]
	return ok
}

func kickstartTarget() string { return fmt.Sprintf("gui/%d/%s", os.Getuid(), LaunchdLabel) }

const testExe = "/Applications/Portty.app/Contents/MacOS/Portty"

// plistDict reads a plist's top-level dict into key -> value, where a value
// is a string, bool, []string (array) or a nested map.
func plistDict(t *testing.T, data []byte) map[string]any {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var parseValue func(start xml.StartElement) any
	parseDict := func() map[string]any {
		m := map[string]any{}
		var key string
		for {
			tok, err := dec.Token()
			if err != nil {
				t.Fatalf("plist: %v", err)
			}
			switch e := tok.(type) {
			case xml.StartElement:
				if e.Name.Local == "key" {
					var k string
					_ = dec.DecodeElement(&k, &e)
					key = k
				} else {
					m[key] = parseValue(e)
				}
			case xml.EndElement:
				return m
			}
		}
	}
	parseValue = func(start xml.StartElement) any {
		switch start.Name.Local {
		case "true":
			_ = dec.Skip()
			return true
		case "false":
			_ = dec.Skip()
			return false
		case "string":
			var s string
			_ = dec.DecodeElement(&s, &start)
			return s
		case "array":
			var out []string
			for {
				tok, _ := dec.Token()
				switch e := tok.(type) {
				case xml.StartElement:
					var s string
					_ = dec.DecodeElement(&s, &e)
					out = append(out, s)
				case xml.EndElement:
					return out
				}
			}
		case "dict":
			return parseDict()
		}
		t.Fatalf("plist: unexpected element %s", start.Name.Local)
		return nil
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("plist has no dict: %v", err)
		}
		if e, ok := tok.(xml.StartElement); ok && e.Name.Local == "dict" {
			return parseDict()
		}
	}
}

func agentPlistOnDisk(t *testing.T) (string, []byte) {
	t.Helper()
	p, err := AgentPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, data
}

// Tracer (DSHR-01): a start writes and loads Portty's own LaunchAgent, then
// ensureDaemon starts the daemon through launchd and never through the
// detached fallback, and the daemon reports it is launchd-managed.
func TestLaunchd_StartTracer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	ctx := context.Background()

	// EnsureAgent: plist written 0644 with every key, job bootstrapped once.
	state, err := EnsureAgent(ctx, testExe)
	if err != nil || state != AgentReady {
		t.Fatalf("EnsureAgent = %v, %v; want AgentReady, nil", state, err)
	}
	path, data := agentPlistOnDisk(t)
	if filepath.Base(path) != "app.portty.Portty.daemon.plist" {
		t.Errorf("plist path %s", path)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Errorf("plist mode %v, want 0644", fi.Mode().Perm())
	}
	d := plistDict(t, data)
	if d["Label"] != "app.portty.Portty.daemon" || d["RunAtLoad"] != true || d["AbandonProcessGroup"] != true {
		t.Errorf("plist = %v", d)
	}
	if args, _ := d["ProgramArguments"].([]string); !slices.Equal(args, []string{testExe, "daemon"}) {
		t.Errorf("ProgramArguments = %v", d["ProgramArguments"])
	}
	if ka, _ := d["KeepAlive"].(map[string]any); len(ka) != 1 || ka["SuccessfulExit"] != false {
		t.Errorf("KeepAlive = %v, want {SuccessfulExit false}", d["KeepAlive"])
	}
	if _, has := d["ProcessType"]; has || strings.Contains(string(data), "ProcessType") {
		t.Error("plist carries a ProcessType key")
	}
	if got := f.subcommands(); !slices.Equal(got, []string{"print", "print-disabled", "bootstrap"}) {
		t.Errorf("first EnsureAgent calls = %v, want print, print-disabled, bootstrap", got)
	}

	// A second EnsureAgent for the same exe writes nothing and bootstraps nothing.
	before, _ := os.Stat(path)
	f.reset()
	time.Sleep(10 * time.Millisecond)
	if state, err := EnsureAgent(ctx, testExe); err != nil || state != AgentReady {
		t.Fatalf("second EnsureAgent = %v, %v", state, err)
	}
	if after, _ := os.Stat(path); !after.ModTime().Equal(before.ModTime()) {
		t.Error("the second EnsureAgent rewrote an unchanged plist")
	}
	if got := f.subcommands(); !slices.Equal(got, []string{"print", "print-disabled"}) {
		t.Errorf("second EnsureAgent calls = %v, want print, print-disabled only", got)
	}

	// ensureDaemon: no daemon answers; the kickstart brings one up.
	sock := refusingSocket(t, "tracer")
	f.reset()
	f.onKickstart = func() { serveDaemonAfter(t, sock, 0) }
	var fallbacks int
	t.Cleanup(SetFallbackSpawnForTest(func(string) (<-chan struct{}, func(), error) {
		fallbacks++
		return nil, nil, errors.New("fallback must not run")
	}))
	if err := ensureDaemon(ctx, sock, EnsureOptions{Start: StartGUI}, testHooks(nil)); err != nil {
		t.Fatalf("ensureDaemon: %v", err)
	}
	want := [][]string{{"kickstart", kickstartTarget()}}
	if got := f.mutating(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("launchctl calls = %v, want exactly %v", got, want)
	}
	if fallbacks != 0 {
		t.Errorf("the detached fallback ran %d times with the agent loaded", fallbacks)
	}

	// The daemon's managed flag: XPC_SERVICE_NAME, and the /health body.
	if ManagedByLaunchd(ctx) {
		t.Error("ManagedByLaunchd with no XPC_SERVICE_NAME and another pid")
	}
	t.Setenv("XPC_SERVICE_NAME", LaunchdLabel)
	if !ManagedByLaunchd(ctx) {
		t.Error("ManagedByLaunchd = false with XPC_SERVICE_NAME set to the label")
	}
	old := launchdManaged
	launchdManaged = true
	t.Cleanup(func() { launchdManaged = old })
	rec := httptest.NewRecorder()
	NewAPI(NewSessionEngine()).handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var h HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &h); err != nil || !h.Launchd || h.PID != os.Getpid() {
		t.Errorf("health = %s (%v), want launchd true and pid %d", rec.Body.String(), err, os.Getpid())
	}
	if !strings.Contains(rec.Body.String(), `"launchd":true`) {
		t.Errorf("health body %s lacks \"launchd\":true", rec.Body.String())
	}
}

const (
	unsafeTestExe = "/Volumes/Portty/Portty.app/Contents/MacOS/Portty"
	launchctlText = "macOS didn't let Portty start its background service. Check Portty in System Settings, General, Login Items, then try again."
)

func writePlistFile(t *testing.T, label string) string {
	t.Helper()
	p, err := plistPath(label)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// startRig is a start with no daemon answering: a fake launchd, a refusing
// socket, and a fallback spawn recorder; either a kickstart or the fallback
// brings a fake ready daemon up at the socket.
type startRig struct {
	f         *fakeLaunchd
	sock      string
	fallbacks []string
	reasons   []error
	stderr    *bytes.Buffer
}

func newStartRig(t *testing.T, exe string) *startRig {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &startRig{f: newFakeLaunchd(t), sock: refusingSocket(t, "rig"), stderr: &bytes.Buffer{}}
	old := executablePath
	executablePath = func() (string, error) { return exe, nil }
	t.Cleanup(func() { executablePath = old })
	r.f.onKickstart = func() { serveDaemonAfter(t, r.sock, 0) }
	t.Cleanup(SetFallbackSpawnForTest(func(e string) (<-chan struct{}, func(), error) {
		r.fallbacks = append(r.fallbacks, e)
		serveDaemonAfter(t, r.sock, 0)
		return nil, nil, nil
	}))
	return r
}

func (r *startRig) ensure(mode StartMode) error {
	h := testHooks(nil)
	h.stderr = r.stderr
	return ensureDaemon(context.Background(), r.sock, EnsureOptions{Start: mode, OnFallback: func(e error) { r.reasons = append(r.reasons, e) }}, h)
}

// DSHR-01 as amended ("Start it anyway"): launchd first for the GUI and the
// CLI; the detached spawn only after a typed agent error, reported through
// OnFallback, and with the repair sentence on stderr for the CLI.
func TestEnsureDaemon_KickstartFirstFallbackOnTypedError(t *testing.T) {
	cases := []struct {
		name  string
		exe   string
		setup func(t *testing.T, r *startRig)
		want  error // nil: a kickstart and no fallback
	}{
		{"loaded and enabled", testExe, func(t *testing.T, r *startRig) { r.f.setJob(LaunchdLabel, 0) }, nil},
		{"not installed", testExe, func(t *testing.T, r *startRig) {}, ErrAgentMissing},
		{"disabled in Login Items", testExe, func(t *testing.T, r *startRig) { r.f.disabled[LaunchdLabel] = true }, ErrAgentDisabled},
		{"bootstrap refused", testExe, func(t *testing.T, r *startRig) {
			writePlistFile(t, LaunchdLabel)
			r.f.failOn["bootstrap"] = errors.New("bootstrap refused")
		}, ErrAgentRefused},
		{"disk image", unsafeTestExe, func(t *testing.T, r *startRig) { r.f.setJob(LaunchdLabel, 0) }, ErrAgentUnsafePath},
	}
	for _, mode := range []StartMode{StartGUI, StartCLI} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("mode%d/%s", mode, tc.name), func(t *testing.T) {
				r := newStartRig(t, tc.exe)
				tc.setup(t, r)
				if err := r.ensure(mode); err != nil {
					t.Fatalf("ensureDaemon: %v", err)
				}
				kicks := 0
				for _, c := range r.f.mutating() {
					if c[0] == "kickstart" {
						kicks++
					}
				}
				if tc.want == nil {
					if want := fmt.Sprint([][]string{{"kickstart", kickstartTarget()}}); fmt.Sprint(r.f.mutating()) != want {
						t.Errorf("launchctl calls = %v, want %s", r.f.mutating(), want)
					}
					if len(r.fallbacks) != 0 || len(r.reasons) != 0 || r.stderr.Len() != 0 {
						t.Errorf("fallbacks %v, reasons %v, stderr %q; want none", r.fallbacks, r.reasons, r.stderr)
					}
					return
				}
				if len(r.reasons) != 1 || !errors.Is(r.reasons[0], tc.want) {
					t.Fatalf("OnFallback got %v, want one %v", r.reasons, tc.want)
				}
				if !slices.Equal(r.fallbacks, []string{tc.exe}) {
					t.Errorf("fallback spawns = %v, want one for %s", r.fallbacks, tc.exe)
				}
				if kicks != 0 {
					t.Errorf("kickstart ran %d times though the agent is unusable", kicks)
				}
				if got := r.stderr.String(); (mode == StartCLI) != strings.Contains(got, tc.want.Error()) {
					t.Errorf("mode %d stderr %q: the repair sentence %q must appear for the CLI only", mode, got, tc.want)
				}
			})
		}
	}
}

// Any launchctl failure on the start path falls back, wrapping the cause
// under ErrAgentLaunchctl; only the fallback spawn failing fails the start.
func TestEnsureDaemon_LaunchctlErrorsFallBack(t *testing.T) {
	boom := errors.New("launchctl exploded")
	for _, sub := range []string{"print", "print-disabled", "kickstart"} {
		t.Run(sub, func(t *testing.T) {
			r := newStartRig(t, testExe)
			r.f.setJob(LaunchdLabel, 0)
			r.f.failOn[sub] = boom
			if err := r.ensure(StartGUI); err != nil {
				t.Fatalf("ensureDaemon: %v", err)
			}
			if len(r.reasons) != 1 || !errors.Is(r.reasons[0], ErrAgentLaunchctl) || !errors.Is(r.reasons[0], boom) {
				t.Fatalf("OnFallback got %v, want ErrAgentLaunchctl wrapping the cause", r.reasons)
			}
			if len(r.fallbacks) != 1 {
				t.Errorf("fallback spawns = %d, want 1", len(r.fallbacks))
			}
		})
	}
	t.Run("fallback fails", func(t *testing.T) {
		r := newStartRig(t, testExe)
		r.f.failOn["print"] = boom
		spawnErr := errors.New("cannot exec")
		t.Cleanup(SetFallbackSpawnForTest(func(string) (<-chan struct{}, func(), error) { return nil, nil, spawnErr }))
		if err := r.ensure(StartGUI); !errors.Is(err, spawnErr) {
			t.Fatalf("ensureDaemon = %v, want the fallback's error", err)
		}
	})
}

// A Quit All that cancels while launchctl runs makes that call fail; it is no
// agent error, so nothing is spawned.
func TestEnsureDaemon_CancelDuringLaunchctlSpawnsNothing(t *testing.T) {
	r := newStartRig(t, testExe)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(SetLaunchctlForTest(func(context.Context, ...string) ([]byte, error) {
		cancel()
		return nil, errors.New("signal: killed")
	}))
	err := ensureDaemon(ctx, r.sock, EnsureOptions{OnFallback: func(e error) { r.reasons = append(r.reasons, e) }}, testHooks(nil))
	if !errors.Is(err, context.Canceled) || len(r.fallbacks) != 0 || len(r.reasons) != 0 {
		t.Fatalf("err %v, fallbacks %v, reasons %v; want a cancel and no spawn", err, r.fallbacks, r.reasons)
	}
}

// T-19-09: with no recorder injected, the detached spawn refuses from a test
// binary, and so does the launchctl seam: no process and no launchd call.
func TestEnsureDaemon_FallbackRefusesUnderTest(t *testing.T) {
	if _, _, err := fallbackSpawn(os.Args[0]); err == nil || !strings.Contains(err.Error(), "test binary") {
		t.Fatalf("fallbackSpawn from a test binary = %v, want the refusal", err)
	}
	t.Setenv("HOME", t.TempDir())
	err := ensureDaemon(context.Background(), refusingSocket(t, "refuse"), EnsureOptions{}, testHooks(nil))
	if err == nil || !strings.Contains(err.Error(), "refusing to spawn a daemon from a test binary") {
		t.Fatalf("ensureDaemon in a test binary = %v, want the spawn refusal", err)
	}
}

func TestAgentErrors_RepairText(t *testing.T) {
	for err, want := range map[error]string{
		ErrAgentLaunchctl:  launchctlText,
		ErrAgentRefused:    launchctlText,
		ErrAgentRunning:    "Portty's background service is running. Run portty daemon stop first, then uninstall.",
		ErrAgentMissing:    "Portty's background service isn't installed. Open Portty.app from Applications once to install it.",
		ErrAgentDisabled:   "Portty's background service is turned off in Login Items. Turn on Portty in System Settings, General, Login Items, then try again.",
		ErrAgentUnsafePath: "Portty is running from a disk image. Move it to Applications, then open it again.",
	} {
		if err.Error() != want {
			t.Errorf("%q, want %q", err.Error(), want)
		}
	}
	f := newFakeLaunchd(t)
	boom := errors.New("kickstart exploded")
	f.failOn["kickstart"] = boom
	err := Kickstart(context.Background())
	if !errors.Is(err, ErrAgentLaunchctl) || !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), launchctlText+" (") || !strings.Contains(err.Error(), "kickstart exploded") {
		t.Errorf("Kickstart error = %q, want the sentence then the wrapped error", err)
	}
}

// UPGR with a launchd job: the CLI moves a stale daemon's sessions by asking
// launchd (the job's daemon re-homes them); with no usable agent it prints the
// repair sentence and uses today's takeover; the GUI keeps its takeover.
func TestEnsureDaemon_CLIHandoffAsksLaunchd(t *testing.T) {
	setup := func(t *testing.T) (*fakeDaemon, *fakeLaunchd) {
		useBuild(t, "v2")
		t.Setenv("HOME", t.TempDir())
		fd := startFakeDaemon(t, HandoffVersion, twoLive)
		f := newFakeLaunchd(t)
		f.setJob(LaunchdLabel, 0)
		f.onKickstart = func() { fd.setVersion("v2") } // the job's daemon took the sessions over
		return fd, f
	}
	run := func(fd *fakeDaemon, mode StartMode, takeover func(string) (int, <-chan int, error)) (asked *[]RestartQuestion, stderr string, err error) {
		policy, asked := recordPolicy(false)
		var buf bytes.Buffer
		h := testHooks(takeover)
		h.stderr = &buf
		err = ensureDaemon(context.Background(), fd.path, EnsureOptions{Restart: policy, Start: mode}, h)
		return asked, buf.String(), err
	}

	t.Run("usable agent", func(t *testing.T) {
		fd, f := setup(t)
		var spawns atomic.Int64
		asked, _, err := run(fd, StartCLI, fd.takeoverAfter(0, 0, "v2", &spawns))
		if err != nil || spawns.Load() != 0 || len(*asked) != 0 || fd.shutdown.Load() {
			t.Fatalf("err %v, takeover spawns %d, asked %v, shutdown %v; want nil, 0, none, false", err, spawns.Load(), *asked, fd.shutdown.Load())
		}
		if want := fmt.Sprint([][]string{{"kickstart", kickstartTarget()}}); fmt.Sprint(f.mutating()) != want {
			t.Errorf("launchctl calls = %v, want %s", f.mutating(), want)
		}
	})
	t.Run("agent disabled", func(t *testing.T) {
		fd, f := setup(t)
		f.disabled[LaunchdLabel] = true
		var spawns atomic.Int64
		asked, stderr, err := run(fd, StartCLI, fd.takeoverAfter(0, 0, "v2", &spawns))
		if err != nil || spawns.Load() != 1 || len(*asked) != 0 {
			t.Fatalf("err %v, takeover spawns %d, asked %v; want nil, 1, none", err, spawns.Load(), *asked)
		}
		if !strings.Contains(stderr, ErrAgentDisabled.Error()) {
			t.Errorf("stderr %q lacks the repair sentence", stderr)
		}
		if len(f.mutating()) != 0 {
			t.Errorf("launchctl mutated with the agent disabled: %v", f.mutating())
		}
	})
	t.Run("GUI keeps its takeover", func(t *testing.T) {
		fd, f := setup(t)
		var spawns atomic.Int64
		if _, _, err := run(fd, StartGUI, fd.takeoverAfter(0, 0, "v2", &spawns)); err != nil || spawns.Load() != 1 {
			t.Fatalf("err %v, takeover spawns %d; want nil, 1", err, spawns.Load())
		}
		if len(f.subcommands()) != 0 {
			t.Errorf("the GUI handoff called launchctl: %v", f.subcommands())
		}
	})
	t.Run("the old daemon is the job", func(t *testing.T) {
		fd, f := setup(t)
		fd.launchd.Store(true)
		var spawns atomic.Int64
		asked, _, err := run(fd, StartCLI, fd.takeoverAfter(0, 0, "v2", &spawns))
		if err != nil || spawns.Load() != 0 || len(*asked) != 1 || (*asked)[0].MoveFailed == "" {
			t.Fatalf("err %v, spawns %d, asked %+v; want nil, 0, one question with a move-failed reason", err, spawns.Load(), *asked)
		}
		if len(f.mutating()) != 0 {
			t.Errorf("kickstart cannot move a running job's sessions, yet launchctl mutated: %v", f.mutating())
		}
	})
}

func TestLegacyCleanup_Ordering(t *testing.T) {
	legacyTarget := fmt.Sprintf("gui/%d/%s", os.Getuid(), LegacyLabel)
	cases := []struct {
		name     string
		file     bool
		job      int // -1: not loaded
		holder   int
		boot     bool
		wantDone bool
	}{
		{"legacy holds the lock", true, 77, 77, false, false},
		{"another process holds the lock", true, 77, 88, true, true},
		{"nothing left", false, -1, 88, false, true},
		{"plist without a job", true, -1, 88, false, true},
		{"running job, holder unknown", true, 77, 0, false, false},
		{"idle job", true, 0, 88, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := newFakeLaunchd(t)
			path := ""
			if tc.file {
				path = writePlistFile(t, LegacyLabel)
			}
			if tc.job >= 0 {
				f.setJob(LegacyLabel, tc.job)
			}
			done, err := LegacyCleanup(context.Background(), tc.holder)
			if err != nil || done != tc.wantDone {
				t.Fatalf("LegacyCleanup = %v, %v; want %v, nil", done, err, tc.wantDone)
			}
			if tc.file && fileExists(path) {
				t.Error("the legacy plist is still on disk")
			}
			want := [][]string(nil)
			if tc.boot {
				want = [][]string{{"bootout", legacyTarget}}
			}
			if fmt.Sprint(f.mutating()) != fmt.Sprint(want) {
				t.Errorf("launchctl calls = %v, want %v", f.mutating(), want)
			}
		})
	}
	t.Run("print fails", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		f := newFakeLaunchd(t)
		f.failOn["print"] = errors.New("boom")
		path := writePlistFile(t, LegacyLabel)
		if done, err := LegacyCleanup(context.Background(), 88); err == nil || done {
			t.Errorf("LegacyCleanup = %v, %v; want an error", done, err)
		}
		if !fileExists(path) {
			t.Error("the plist was removed though the job's state is unknown")
		}
	})
}

// Research Pitfall 5: while the legacy job's process holds the lock only the
// file goes; the bootout waits for a start after the holder changed.
func TestLegacyCleanup_EndState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	path := writePlistFile(t, LegacyLabel)
	f.setJob(LegacyLabel, 77)
	ctx := context.Background()

	if done, err := LegacyCleanup(ctx, 77); err != nil || done {
		t.Fatalf("first start: %v, %v; want not done", done, err)
	}
	if fileExists(path) || !f.hasJob(LegacyLabel) {
		t.Fatalf("after the first start: plist exists %v, job loaded %v; want the file gone and the job untouched", fileExists(path), f.hasJob(LegacyLabel))
	}
	if done, err := LegacyCleanup(ctx, 88); err != nil || !done {
		t.Fatalf("later start: %v, %v; want done", done, err)
	}
	if fileExists(path) {
		t.Error("the legacy plist is back")
	}
	if loaded, _, err := jobLoaded(ctx, LegacyLabel); err != nil || loaded {
		t.Errorf("legacy job loaded %v (%v) after the end state, want not found", loaded, err)
	}
}

// T-19-05: no kill flag anywhere, and bootout only for an idle agent job or
// the legacy job under the ordering rule; uninstall refuses while it runs.
func TestLaunchd_NoKillFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	ctx := context.Background()
	t.Cleanup(func() { executablePath = os.Executable })
	executablePath = func() (string, error) { return testExe, nil }

	steps := []func() error{
		func() error { _, err := EnsureAgent(ctx, testExe); return err },
		func() error { _, err := EnsureAgent(ctx, "/Applications/Other.app/Contents/MacOS/Portty"); return err }, // idle: reload
		func() error { f.setJob(LaunchdLabel, 99); _, err := EnsureAgent(ctx, testExe); return err },             // running: deferred
		func() error { return Kickstart(ctx) },
		func() error { return AgentControl("start") },
		func() error { f.setJob(LegacyLabel, 77); _, err := LegacyCleanup(ctx, 77); return err },
		func() error { _, err := LegacyCleanup(ctx, 88); return err },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if err := AgentControl("uninstall"); !errors.Is(err, ErrAgentRunning) { // the agent job still has pid 99
		t.Fatalf("uninstall of a running job = %v, want ErrAgentRunning", err)
	}
	f.setJob(LaunchdLabel, 0)
	if err := AgentControl("uninstall"); err != nil {
		t.Fatalf("uninstall of an idle job: %v", err)
	}

	for _, c := range f.calls {
		for _, a := range c {
			if a == "-k" || strings.HasPrefix(a, "-") {
				t.Errorf("launchctl call %v carries a flag", c)
			}
		}
	}
	for _, b := range f.bootouts {
		if b.pid != 0 && b.label != LegacyLabel {
			t.Errorf("booted out %s while it had pid %d", b.label, b.pid)
		}
	}
	// 1 reload of the idle agent, the legacy job once its holder changed, the idle agent at uninstall.
	if len(f.bootouts) != 3 {
		t.Errorf("bootouts = %+v, want 3", f.bootouts)
	}
}

func TestLaunchd_ReloadDeferredWhileRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	ctx := context.Background()
	if _, err := EnsureAgent(ctx, testExe); err != nil {
		t.Fatal(err)
	}
	f.setJob(LaunchdLabel, 4242)
	f.reset()
	other := "/Applications/Moved.app/Contents/MacOS/Portty"
	state, err := EnsureAgent(ctx, other)
	if err != nil || state != AgentReloadDeferred {
		t.Fatalf("EnsureAgent while running = %v, %v; want AgentReloadDeferred", state, err)
	}
	if len(f.mutating()) != 0 {
		t.Errorf("a running job was touched: %v", f.mutating())
	}
	// Idle again: the changed plist is applied with bootout then bootstrap.
	f.setJob(LaunchdLabel, 0)
	if state, err := EnsureAgent(ctx, other); err != nil || state != AgentReady {
		t.Fatalf("EnsureAgent once idle = %v, %v", state, err)
	}
	if got := f.subcommands(); !slices.Equal(got, []string{"print", "print-disabled", "print", "print-disabled", "bootout", "bootstrap"}) {
		t.Errorf("calls = %v", got)
	}
}

func TestLaunchd_UnsafePath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	for _, exe := range []string{unsafeTestExe, "/private/var/folders/x/AppTranslocation/ABC/d/Portty.app/Contents/MacOS/Portty"} {
		if _, err := EnsureAgent(context.Background(), exe); !errors.Is(err, ErrAgentUnsafePath) {
			t.Errorf("EnsureAgent(%s) = %v, want ErrAgentUnsafePath", exe, err)
		}
	}
	if p, _ := AgentPath(); fileExists(p) || len(f.calls) != 0 {
		t.Errorf("an unsafe executable wrote a plist or called launchctl: %v", f.calls)
	}
	if _, err := EnsureAgent(context.Background(), "relative/Portty"); err == nil {
		t.Error("a relative executable was accepted")
	}
}

// T-19-06: the executable is XML-escaped into the plist, never spliced raw.
func TestRenderAgentPlist_Escapes(t *testing.T) {
	data, err := RenderAgentPlist("/Applications/A&B <x>.app/Contents/MacOS/Portty")
	if err != nil {
		t.Fatal(err)
	}
	if args, _ := plistDict(t, data)["ProgramArguments"].([]string); len(args) != 2 || args[0] != "/Applications/A&B <x>.app/Contents/MacOS/Portty" {
		t.Errorf("ProgramArguments = %v, want the exe round-tripped through XML escaping", args)
	}
	if strings.Contains(string(data), "<x>") {
		t.Error("the executable's markup reached the plist unescaped")
	}
}

func TestLaunchctl_RefusesUnderTest(t *testing.T) {
	if out, err := defaultLaunchctl(context.Background(), "print", "gui/0/none"); err == nil || out != nil {
		t.Fatalf("defaultLaunchctl under a test binary = %q, %v; want a refusal", out, err)
	}
}

func TestAgentPath_RefusesRealHomeUnderTest(t *testing.T) {
	t.Setenv("HOME", "/nonexistent-home/someone")
	f := newFakeLaunchd(t)
	if _, err := AgentPath(); !errors.Is(err, ErrAgentUnsafePath) {
		t.Fatalf("AgentPath with a home outside the temp dir = %v, want ErrAgentUnsafePath", err)
	}
	if _, err := EnsureAgent(context.Background(), testExe); !errors.Is(err, ErrAgentUnsafePath) {
		t.Fatalf("EnsureAgent = %v, want ErrAgentUnsafePath", err)
	}
	if len(f.calls) != 0 || fileExists("/nonexistent-home/someone/Library/LaunchAgents") {
		t.Errorf("nothing may be written or called: %v", f.calls)
	}
	t.Setenv("HOME", "")
	if _, err := AgentPath(); err == nil {
		t.Error("AgentPath with no HOME returned no error")
	}
}

// The portty daemon <action> commands, in-package so they count toward
// launchd.go's coverage.
func TestAgentControl_InPackage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	t.Cleanup(func() { executablePath = os.Executable })
	executablePath = func() (string, error) { return testExe, nil }
	target := kickstartTarget()

	if err := AgentControl("install"); err != nil {
		t.Fatalf("install: %v", err)
	}
	agent, _ := AgentPath()
	if !fileExists(agent) || !f.hasJob(LaunchdLabel) {
		t.Fatalf("install left plist %v, job loaded %v", fileExists(agent), f.hasJob(LaunchdLabel))
	}

	f.reset()
	if err := AgentControl("start"); err != nil || fmt.Sprint(f.mutating()) != fmt.Sprint([][]string{{"kickstart", target}}) {
		t.Fatalf("start = %v, calls %v; want only the plain kickstart", err, f.mutating())
	}
	f.reset()
	if err := AgentControl("stop"); err != nil || fmt.Sprint(f.mutating()) != fmt.Sprint([][]string{{"kill", "SIGTERM", target}}) {
		t.Fatalf("stop = %v, calls %v; want kill SIGTERM", err, f.mutating())
	}
	f.failOn["kill"] = errors.New("no such process")
	if err := AgentControl("stop"); !errors.Is(err, ErrAgentLaunchctl) {
		t.Errorf("a failing stop = %v, want ErrAgentLaunchctl", err)
	}
	delete(f.failOn, "kill")

	// A running job: uninstall refuses and changes nothing.
	legacy := writePlistFile(t, LegacyLabel)
	f.setJob(LaunchdLabel, 321)
	f.reset()
	if err := AgentControl("uninstall"); !errors.Is(err, ErrAgentRunning) {
		t.Fatalf("uninstall of a running job = %v, want ErrAgentRunning", err)
	}
	if len(f.mutating()) != 0 || !fileExists(agent) || !fileExists(legacy) {
		t.Errorf("uninstall changed things while running: calls %v, agent %v, legacy %v", f.mutating(), fileExists(agent), fileExists(legacy))
	}
	// Idle: the job is booted out and both plists go.
	f.setJob(LaunchdLabel, 0)
	f.reset()
	if err := AgentControl("uninstall"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if f.hasJob(LaunchdLabel) || fileExists(agent) || fileExists(legacy) {
		t.Errorf("after uninstall: job loaded %v, agent plist %v, legacy plist %v", f.hasJob(LaunchdLabel), fileExists(agent), fileExists(legacy))
	}
	// Nothing installed: uninstall is a clean no-op.
	if err := AgentControl("uninstall"); err != nil {
		t.Errorf("uninstall with nothing installed: %v", err)
	}
	f.failOn["print"] = errors.New("boom")
	if err := AgentControl("uninstall"); err == nil {
		t.Error("uninstall with an unreadable job state succeeded")
	}
	if err := AgentControl("frobnicate"); err == nil || !strings.Contains(err.Error(), "frobnicate") {
		t.Errorf("unknown action = %v", err)
	}
}

// WR-04: a running job whose `launchctl print` output has no pid line the
// parser understands (a format change in a macOS update) counts as running:
// no reload, uninstall or legacy cleanup boots it out.
func TestLaunchd_UnparsedPidCountsAsRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeLaunchd(t)
	ctx := context.Background()
	if _, err := EnsureAgent(ctx, testExe); err != nil {
		t.Fatal(err)
	}
	writePlistFile(t, LegacyLabel)
	f.setJob(LaunchdLabel, pidLineMissing)
	f.setJob(LegacyLabel, pidLineMissing)
	f.reset()

	if state, err := EnsureAgent(ctx, "/Applications/Moved.app/Contents/MacOS/Portty"); err != nil || state != AgentReloadDeferred {
		t.Errorf("EnsureAgent with a changed plist = %v, %v; want AgentReloadDeferred", state, err)
	}
	if err := uninstallAgent(ctx); !errors.Is(err, ErrAgentRunning) {
		t.Errorf("uninstall = %v, want ErrAgentRunning", err)
	}
	if done, err := LegacyCleanup(ctx, 88); err != nil || done {
		t.Errorf("LegacyCleanup = %v, %v; want not done", done, err)
	}
	if len(f.mutating()) != 0 {
		t.Errorf("a job of unknown liveness was touched: %v", f.mutating())
	}
}
