package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

func TestCmdDaemon_ServiceActions(t *testing.T) {
	actions := []string{"install", "uninstall", "start", "stop"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			var called string
			old := serviceControlFunc
			serviceControlFunc = func(a string) error {
				called = a
				return nil
			}
			defer func() { serviceControlFunc = old }()

			var buf bytes.Buffer
			err := cmdDaemon([]string{action}, &buf)
			if err != nil {
				t.Fatalf("cmdDaemon(%q) error: %v", action, err)
			}
			if called != action {
				t.Errorf("ServiceControl called with %q, want %q", called, action)
			}
			if !strings.Contains(buf.String(), action) {
				t.Errorf("output %q does not contain %q", buf.String(), action)
			}
		})
	}
}

func TestCmdDaemon_UnknownSubcommand(t *testing.T) {
	var buf bytes.Buffer
	err := cmdDaemon([]string{"bogus"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error %q does not contain 'unknown'", err.Error())
	}
}

func TestCmdDaemon_ServiceControlError(t *testing.T) {
	old := serviceControlFunc
	serviceControlFunc = func(a string) error {
		return fmt.Errorf("mock error")
	}
	defer func() { serviceControlFunc = old }()

	var buf bytes.Buffer
	err := cmdDaemon([]string{"install"}, &buf)
	if err == nil {
		t.Fatal("expected error when ServiceControl fails")
	}
	if !strings.Contains(err.Error(), "mock error") {
		t.Errorf("error %q does not contain 'mock error'", err.Error())
	}
}

// TestCmdDaemon_Status verifies cmdDaemonStatus prints "running:" and "true" with reachable daemon.
func TestCmdDaemon_Status(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdDaemonStatus(client, nil, &buf)
	if err != nil {
		t.Fatalf("cmdDaemonStatus error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "running:") {
		t.Errorf("expected 'running:' in output, got %q", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected 'true' in output, got %q", out)
	}
}

// TestCmdDaemon_Status_JSON verifies --json produces {"running":true} with reachable daemon.
func TestCmdDaemon_Status_JSON(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdDaemonStatus(client, []string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("cmdDaemonStatus --json error: %v", err)
	}
	var resp struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if !resp.Running {
		t.Error("expected running=true for reachable daemon")
	}
}

// TestCmdDaemon_Status_Unreachable verifies cmdDaemonStatus prints "false" for unreachable daemon.
func TestCmdDaemon_Status_Unreachable(t *testing.T) {
	// Create client pointing to non-existent socket.
	client := daemon.NewDaemonClient("/tmp/nonexistent-aht-test.sock")
	var buf bytes.Buffer
	err := cmdDaemonStatus(client, nil, &buf)
	if err != nil {
		t.Fatalf("cmdDaemonStatus error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "false") {
		t.Errorf("expected 'false' for unreachable daemon, got %q", out)
	}
}

// TestCmdDaemon_Status_JSON_Unreachable verifies --json produces {"running":false} for unreachable daemon.
func TestCmdDaemon_Status_JSON_Unreachable(t *testing.T) {
	client := daemon.NewDaemonClient("/tmp/nonexistent-aht-test.sock")
	var buf bytes.Buffer
	err := cmdDaemonStatus(client, []string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("cmdDaemonStatus --json error: %v", err)
	}
	var resp struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if resp.Running {
		t.Error("expected running=false for unreachable daemon")
	}
}

// 09-05: `portty daemon --takeover` is the process EnsureDaemon spawns for a
// handoff. It routes to the RunTakeover seam, and the usage error for an
// unknown subcommand keeps not advertising the internal flag.
func TestCmdDaemon_TakeoverRoutes(t *testing.T) {
	calls := 0
	old := runTakeoverFunc
	runTakeoverFunc = func() { calls++ }
	defer func() { runTakeoverFunc = old }()

	if err := cmdDaemon([]string{"--takeover"}, &bytes.Buffer{}); err != nil || calls != 1 {
		t.Fatalf("cmdDaemon(--takeover): err %v, RunTakeover calls %d; want nil, 1", err, calls)
	}
	err := cmdDaemon([]string{"bogus"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `unknown daemon subcommand "bogus"`) {
		t.Fatalf("cmdDaemon(bogus) = %v, want the usage error", err)
	}
	if strings.Contains(err.Error(), "--takeover") {
		t.Fatalf("usage error advertises the internal flag: %v", err)
	}
	if calls != 1 {
		t.Fatalf("bogus reached RunTakeover (%d calls)", calls)
	}
}

// DSHR-01: install, uninstall, start and stop now drive Portty's own
// LaunchAgent through launchctl (a fake here: HOME is a temp dir and the real
// launchd is never reached). stop is `kill SIGTERM` (the daemon exits 0 and
// launchd leaves it down); uninstall refuses while the job runs.
func TestAgentControl_Actions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls [][]string
	jobPid, loaded := 0, false
	t.Cleanup(daemon.SetLaunchctlForTest(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch args[0] {
		case "print":
			if !loaded {
				return []byte("Could not find service"), errors.New("exit status 113")
			}
			return []byte(fmt.Sprintf("pid = %d\n", jobPid)), nil
		case "print-disabled":
			return []byte("disabled services = {\n}\n"), nil
		case "bootstrap":
			loaded = true
		case "bootout":
			loaded = false
		}
		return nil, nil
	}))
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), daemon.LaunchdLabel)
	agent, err := daemon.AgentPath()
	if err != nil {
		t.Fatal(err)
	}
	last := func() []string { return calls[len(calls)-1] }

	var out bytes.Buffer
	if err := cmdDaemon([]string{"install"}, &out); err != nil || !loaded {
		t.Fatalf("install: %v, job loaded %v", err, loaded)
	}
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("install wrote no plist: %v", err)
	}
	calls = nil
	if err := cmdDaemon([]string{"start"}, &out); err != nil || len(calls) != 1 || !slices.Equal(last(), []string{"kickstart", target}) {
		t.Fatalf("start: %v, calls %v; want one plain kickstart", err, calls)
	}
	calls = nil
	if err := cmdDaemon([]string{"stop"}, &out); err != nil || len(calls) != 1 || !slices.Equal(last(), []string{"kill", "SIGTERM", target}) {
		t.Fatalf("stop: %v, calls %v; want kill SIGTERM", err, calls)
	}

	jobPid = 4242
	calls = nil
	err = cmdDaemon([]string{"uninstall"}, &out)
	if err == nil || !strings.Contains(err.Error(), "Run portty daemon stop first, then uninstall.") {
		t.Fatalf("uninstall of a running job = %v, want the stop-first message", err)
	}
	if _, serr := os.Stat(agent); serr != nil || !loaded {
		t.Fatalf("a running job's uninstall changed things: plist err %v, loaded %v, calls %v", serr, loaded, calls)
	}

	jobPid = 0
	if err := cmdDaemon([]string{"uninstall"}, &out); err != nil || loaded {
		t.Fatalf("uninstall of an idle job: %v, loaded %v", err, loaded)
	}
	if _, serr := os.Stat(agent); serr == nil {
		t.Fatal("uninstall left the plist")
	}
	for _, c := range calls {
		if slices.Contains(c, "-k") {
			t.Fatalf("a launchctl call carries the kill flag: %v", c)
		}
	}
}
