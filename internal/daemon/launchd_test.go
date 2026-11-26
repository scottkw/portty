package daemon

import (
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
	"testing"
	"time"
)

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
