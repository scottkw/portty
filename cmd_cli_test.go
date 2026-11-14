package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/agenthub/internal/daemon"
	"github.com/scottkw/agenthub/internal/relay"
	"github.com/scottkw/agenthub/internal/webserver"
)

// testSetup creates a real daemon API on a short socket path and returns a DaemonClient.
func testSetup(t *testing.T) *daemon.DaemonClient {
	t.Helper()
	// Isolate config directory so settings.json from other tests doesn't leak.
	tmpCfg := t.TempDir()
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", tmpCfg) // os.UserConfigDir uses ~/Library/Application Support
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmpCfg)
	}
	engine := daemon.NewSessionEngine()
	engine.SetSpawnCommandForTest("/bin/cat", nil) // deterministic echo process instead of a login shell
	api := daemon.NewAPI(engine)
	// Use short socket path to avoid macOS 103-char sun_path limit.
	socketPath := fmt.Sprintf("/tmp/aht%d_%d.sock", os.Getpid(), time.Now().UnixNano()%10000)
	t.Cleanup(func() { os.Remove(socketPath) })
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { api.Stop() })
	// Brief sleep to let server goroutine start accepting.
	time.Sleep(10 * time.Millisecond)
	return daemon.NewDaemonClient(socketPath)
}

// TestCmdList_Empty verifies that the header row is printed even when no sessions exist.
func TestCmdList_Empty(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdList(client, nil, &buf)
	if err != nil {
		t.Fatalf("cmdList returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "HOST") {
		t.Errorf("expected header row with %q, got %q", "HOST", out)
	}
	if !strings.Contains(out, "ID") {
		t.Errorf("expected header row with %q, got %q", "ID", out)
	}
}

// TestCmdList_WithSessions verifies that sessions appear in the list output.
func TestCmdList_WithSessions(t *testing.T) {
	client := testSetup(t)
	// Create a session via the client directly.
	id, err := client.CreateSession("mytest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var buf bytes.Buffer
	if err := cmdList(client, nil, &buf); err != nil {
		t.Fatalf("cmdList: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, id) {
		t.Errorf("expected session ID %q in list output, got:\n%s", id, out)
	}
	if !strings.Contains(out, "mytest") {
		t.Errorf("expected session name %q in list output, got:\n%s", "mytest", out)
	}
}

// TestCmdList_JSON_Empty verifies that --json with no sessions produces a listOutput with empty local array.
func TestCmdList_JSON_Empty(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdList(client, []string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("cmdList --json returned error: %v", err)
	}
	var output listOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if len(output.Local) != 0 {
		t.Errorf("expected 0 local sessions, got %d", len(output.Local))
	}
}

// TestCmdList_JSON_WithSessions verifies that --json with sessions produces valid JSON.
func TestCmdList_JSON_WithSessions(t *testing.T) {
	client := testSetup(t)
	id, err := client.CreateSession("jsontest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var buf bytes.Buffer
	if err := cmdList(client, []string{"--json"}, &buf); err != nil {
		t.Fatalf("cmdList --json: %v", err)
	}
	var output listOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if len(output.Local) != 1 {
		t.Fatalf("expected 1 local session, got %d", len(output.Local))
	}
	if output.Local[0].ID != id {
		t.Errorf("expected session ID %q, got %q", id, output.Local[0].ID)
	}
	if output.Local[0].Name != "jsontest" {
		t.Errorf("expected name %q, got %q", "jsontest", output.Local[0].Name)
	}
}

// TestCmdKill_Success creates a session, kills it, and verifies it is removed.
func TestCmdKill_Success(t *testing.T) {
	client := testSetup(t)
	id, err := client.CreateSession("killtest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := cmdKill(client, []string{id}); err != nil {
		t.Fatalf("cmdKill: %v", err)
	}
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after kill, got %d", len(sessions))
	}
}

// TestCmdRename_Success creates a session, renames it, and verifies the new name.
func TestCmdRename_Success(t *testing.T) {
	client := testSetup(t)
	id, err := client.CreateSession("oldname", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := cmdRename(client, []string{id, "newname"}); err != nil {
		t.Fatalf("cmdRename: %v", err)
	}
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "newname" {
		t.Errorf("expected name %q, got %q", "newname", sessions[0].Name)
	}
}

// TestCmdRename_MissingArgs verifies that missing args returns an error with "usage".
func TestCmdRename_MissingArgs(t *testing.T) {
	client := testSetup(t)
	err := cmdRename(client, []string{"onlyone"})
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected error to contain %q, got %q", "usage", err.Error())
	}
}

// testSetupWithWebServer creates a daemon with an injected running WebServer.
// Returns client and a cleanup-registered web server reference.
// Used for cmdServe/cmdUnserve tests that require a running web server.
func testSetupWithWebServer(t *testing.T) *daemon.DaemonClient {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	engine := daemon.NewSessionEngine()
	engine.SetSpawnCommandForTest("/bin/cat", nil) // deterministic echo process instead of a login shell
	api := daemon.NewAPI(engine)
	socketPath := fmt.Sprintf("/tmp/aht%d_%d.sock", os.Getpid(), time.Now().UnixNano()%10000)
	t.Cleanup(func() { os.Remove(socketPath) })
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}

	// Create and inject a test web server (no TLS, localhost, random port).
	ws, err := webserver.NewWebServer(webserver.Config{
		BindIP: "127.0.0.1",
		Port:   0,
		FQDN:   "localhost",
	}, relay.NewHubManager())
	if err != nil {
		t.Fatalf("NewWebServer: %v", err)
	}
	ws.SetSessionResolver(func(id string) (string, string, string, string) {
		return id, "", "", ""
	})
	if err := ws.Start(); err != nil {
		t.Fatalf("ws.Start: %v", err)
	}
	api.SetWebServerForTest(ws)

	t.Cleanup(func() {
		_ = ws.Stop()
		api.Stop()
	})
	time.Sleep(10 * time.Millisecond)
	return daemon.NewDaemonClient(socketPath)
}

// TestCmdWebStatus_NotRunning verifies web status prints "running:" and "false" when not running.
func TestCmdWebStatus_NotRunning(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdWebStatus(client, nil, &buf)
	if err != nil {
		t.Fatalf("cmdWebStatus returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "running:") {
		t.Errorf("expected output to contain %q, got %q", "running:", out)
	}
	if !strings.Contains(out, "false") {
		t.Errorf("expected output to contain %q, got %q", "false", out)
	}
}

// TestCmdWebStatus_JSON verifies --json produces valid JSON for web status.
func TestCmdWebStatus_JSON(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdWebStatus(client, []string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("cmdWebStatus --json returned error: %v", err)
	}
	var resp daemon.WebServerStatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if resp.Running {
		t.Error("expected Running=false")
	}
}

// TestCmdWebStop verifies cmdWebStop returns no error (server may or may not be running).
func TestCmdWebStop(t *testing.T) {
	client := testSetup(t)
	// Stopping when not running should return no error (server returns 204).
	err := cmdWebStop(client)
	if err != nil {
		t.Errorf("cmdWebStop returned unexpected error: %v", err)
	}
}

// TestCmdServe_Success creates a session and enables web serving for it.
func TestCmdServe_Success(t *testing.T) {
	client := testSetupWithWebServer(t)
	id, err := client.CreateSession("servetest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := cmdServe(client, []string{id}); err != nil {
		t.Fatalf("cmdServe returned error: %v", err)
	}
}

// TestCmdServe_MissingArgs verifies that missing args returns an error containing "usage".
func TestCmdServe_MissingArgs(t *testing.T) {
	client := testSetup(t)
	err := cmdServe(client, []string{})
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected error to contain %q, got %q", "usage", err.Error())
	}
}

// TestCmdUnserve_Success creates a session and disables web serving for it.
func TestCmdUnserve_Success(t *testing.T) {
	client := testSetupWithWebServer(t)
	id, err := client.CreateSession("unservetest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := cmdUnserve(client, []string{id}); err != nil {
		t.Fatalf("cmdUnserve returned error: %v", err)
	}
}

// TestCmdHealth_OutputFormat verifies cmdHealth prints exactly 5 key-value lines.
func TestCmdHealth_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	err := cmdHealth(nil, &buf)
	if err != nil {
		t.Fatalf("cmdHealth returned error: %v", err)
	}
	out := buf.String()
	// Verify all 5 labels are present.
	for _, label := range []string{"installed:", "connected:", "has-certs:", "ip:", "domain:"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected output to contain %q, got:\n%s", label, out)
		}
	}
	// Count non-empty lines — should be exactly 5.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 output lines, got %d:\n%s", len(lines), out)
	}
}

// TestCmdHealth_JSON verifies --json produces valid JSON for health.
func TestCmdHealth_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := cmdHealth([]string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("cmdHealth --json returned error: %v", err)
	}
	var h webserver.TailscaleHealth
	if err := json.Unmarshal(buf.Bytes(), &h); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	// Just verify it deserialized without error — actual values depend on system state.
}

// TestCmdQR_WebNotRunning verifies cmdQR returns an error when web server is not running.
func TestCmdQR_WebNotRunning(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdQR(client, []string{"some-id"}, &buf)
	if err == nil {
		t.Fatal("expected error when web server not running, got nil")
	}
	if !strings.Contains(err.Error(), "web server not running") {
		t.Errorf("expected error to contain %q, got %q", "web server not running", err.Error())
	}
}

// TestCmdQR_MissingArgs verifies that missing args returns an error containing "usage".
func TestCmdQR_MissingArgs(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdQR(client, []string{}, &buf)
	if err == nil {
		t.Fatal("expected error for missing args, got nil")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected error to contain %q, got %q", "usage", err.Error())
	}
}

// TestCmdWeb_MissingSubcommand verifies that missing subcommand returns an error with "usage: agenthub web".
func TestCmdWeb_MissingSubcommand(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdWeb(client, []string{}, &buf)
	if err == nil {
		t.Fatal("expected error for missing subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "usage: agenthub web") {
		t.Errorf("expected error to contain %q, got %q", "usage: agenthub web", err.Error())
	}
}

// TestCmdSettings_Basic verifies cmdSettings prints all three label lines.
func TestCmdSettings_Basic(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	err := cmdSettings(client, &buf)
	if err != nil {
		t.Fatalf("cmdSettings returned error: %v", err)
	}
	out := buf.String()
	for _, label := range []string{"socket-path:", "relay-port:", "cli-paths:"} {
		if !strings.Contains(out, label) {
			t.Errorf("expected output to contain %q, got:\n%s", label, out)
		}
	}
}

// TestCmdSettings_SocketPath verifies the output contains the default socket path value.
func TestCmdSettings_SocketPath(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	if err := cmdSettings(client, &buf); err != nil {
		t.Fatalf("cmdSettings error: %v", err)
	}
	expected := daemon.DefaultSocketPath()
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected socket path %q in output, got:\n%s", expected, buf.String())
	}
}

// TestCmdSettings_RelayPort verifies the relay-port label is always present in output.
func TestCmdSettings_RelayPort(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	if err := cmdSettings(client, &buf); err != nil {
		t.Fatalf("cmdSettings error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "relay-port:") {
		t.Errorf("expected 'relay-port:' in output, got:\n%s", out)
	}
}

// TestCmdSettings_CLIPaths_None verifies that when no CLI paths are set, output contains "(none)".
func TestCmdSettings_CLIPaths_None(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	if err := cmdSettings(client, &buf); err != nil {
		t.Fatalf("cmdSettings error: %v", err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Errorf("expected '(none)' for empty CLI paths, got:\n%s", buf.String())
	}
}

// TestCmdSettings_CLIPaths_Set verifies that after setting a CLI path, output contains the path name and value.
func TestCmdSettings_CLIPaths_Set(t *testing.T) {
	client := testSetup(t)
	// Use /bin/sh as the CLI binary — it always exists on macOS and Linux.
	if err := client.UpdateCLIPath("claude", "/bin/sh"); err != nil {
		t.Fatalf("UpdateCLIPath: %v", err)
	}
	var buf bytes.Buffer
	if err := cmdSettings(client, &buf); err != nil {
		t.Fatalf("cmdSettings error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "claude=/bin/sh") {
		t.Errorf("expected CLI path in output, got:\n%s", out)
	}
}

// TestCmdList_WithHostColumn creates a session and verifies "(local)" and "HOST" appear in output.
func TestCmdList_WithHostColumn(t *testing.T) {
	client := testSetup(t)
	_, err := client.CreateSession("hosttest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var buf bytes.Buffer
	if err := cmdList(client, nil, &buf); err != nil {
		t.Fatalf("cmdList: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "HOST") {
		t.Errorf("expected HOST column in output, got:\n%s", out)
	}
	if !strings.Contains(out, "(local)") {
		t.Errorf("expected (local) in output, got:\n%s", out)
	}
}

// TestCmdList_LocalFlag verifies --local skips remote discovery.
func TestCmdList_LocalFlag(t *testing.T) {
	client := testSetup(t)
	_, err := client.CreateSession("localtest", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var buf bytes.Buffer
	// --local should work without error (no peer fetch attempted on test daemon)
	if err := cmdList(client, []string{"--local"}, &buf); err != nil {
		t.Fatalf("cmdList --local: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(local)") {
		t.Errorf("expected (local) in --local output, got:\n%s", out)
	}
	if !strings.Contains(out, "localtest") {
		t.Errorf("expected session name in output, got:\n%s", out)
	}
}

// TestCmdList_JSON_WithHostField verifies JSON output has local array with session objects.
func TestCmdList_JSON_WithHostField(t *testing.T) {
	client := testSetup(t)
	id, err := client.CreateSession("jsonhost", "/tmp", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var buf bytes.Buffer
	if err := cmdList(client, []string{"--json"}, &buf); err != nil {
		t.Fatalf("cmdList --json: %v", err)
	}
	var output listOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw: %s", err, buf.String())
	}
	if len(output.Local) != 1 {
		t.Fatalf("expected 1 local session, got %d", len(output.Local))
	}
	if output.Local[0].ID != id {
		t.Errorf("expected session ID %q, got %q", id, output.Local[0].ID)
	}
}

// TestUsage_RemoteSessionDocs verifies that the usage text documents remote
// session features including the Remote Sessions section, hostname:session-id
// example, --local flag, and updated descriptions (REM-05).
func TestUsage_RemoteSessionDocs(t *testing.T) {
	// Read cmd_cli.go source to verify usage text content.
	// Since usage() writes to os.Stderr directly, we use source inspection.
	src, err := os.ReadFile("cmd_cli.go")
	if err != nil {
		t.Fatalf("failed to read cmd_cli.go: %v", err)
	}
	content := string(src)

	checks := []struct {
		name   string
		needle string
	}{
		{"Remote Sessions section header", "Remote Sessions:"},
		{"hostname:session-id example", "hostname:session-id"},
		{"--local flag in list line", "--local"},
		{"List local and remote sessions", "List local and remote sessions"},
		{"Attach to a local or remote session", "Attach to a local or remote session"},
		{"macbook:abc123 example", "macbook:abc123"},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(content, tc.needle) {
				t.Errorf("cmd_cli.go usage missing %q", tc.needle)
			}
		})
	}
}

// captureStderr redirects os.Stderr to a pipe, runs fn, and returns captured output.
// Restores os.Stderr on completion. Used by the cmdNew tests to assert
// locked stderr error strings (per UI-SPEC §CLI).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan struct{})
	var captured bytes.Buffer
	go func() {
		_, _ = captured.ReadFrom(r)
		close(done)
	}()
	fn()
	w.Close()
	<-done
	os.Stderr = old
	return captured.String()
}

// newSessionInfo returns the single session the daemon holds.
func newSessionInfo(t *testing.T, client *daemon.DaemonClient) daemon.SessionInfo {
	t.Helper()
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected exactly 1 session, got %d: %+v", len(sessions), sessions)
	}
	return sessions[0]
}

// assertSessionID fails unless out is a 32-char lowercase hex session ID.
func assertSessionID(t *testing.T, out string) {
	t.Helper()
	id := strings.TrimSpace(out)
	if len(id) != 32 {
		t.Fatalf("expected 32-char hex session ID, got %q (len=%d)", id, len(id))
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("session ID contains non-hex character %q: %q", c, id)
		}
	}
}

// TestCmdNew_WithDir: `agenthub new <dir>` creates a session in dir named
// after its basename and prints the ID.
func TestCmdNew_WithDir(t *testing.T) {
	client := testSetup(t)
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := cmdNew(client, []string{dir}, &buf); err != nil {
		t.Fatalf("cmdNew: %v", err)
	}
	assertSessionID(t, buf.String())
	info := newSessionInfo(t, client)
	if info.Name != filepath.Base(dir) {
		t.Errorf("session name = %q, want %q", info.Name, filepath.Base(dir))
	}
	want, _ := filepath.EvalSymlinks(dir)
	if info.WorkDir != want {
		t.Errorf("session WorkDir = %q, want %q", info.WorkDir, want)
	}
}

// TestCmdNew_NoDir: `agenthub new` creates a session named "shell"; the daemon
// starts it in the home directory.
func TestCmdNew_NoDir(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	if err := cmdNew(client, nil, &buf); err != nil {
		t.Fatalf("cmdNew: %v", err)
	}
	assertSessionID(t, buf.String())
	if info := newSessionInfo(t, client); info.Name != "shell" {
		t.Errorf("session name = %q, want %q", info.Name, "shell")
	}
}

// TestCmdNew_RejectsUnknownFlag: an unknown flag (including the long-removed
// --shell) fails before the daemon is called.
func TestCmdNew_RejectsUnknownFlag(t *testing.T) {
	client := testSetup(t)
	var buf bytes.Buffer
	var callErr error
	stderr := captureStderr(t, func() {
		callErr = cmdNew(client, []string{"--shell=zsh"}, &buf)
	})
	if callErr == nil {
		t.Fatal("expected non-nil error for unknown flag, got nil")
	}
	if !strings.Contains(stderr, "agenthub new: flag provided but not defined: -shell") {
		t.Errorf("unexpected stderr %q", stderr)
	}
	if sessions, _ := client.ListSessions(); len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// TestCmdNew_DaemonError: a daemon-unreachable error surfaces with the locked
// stderr prefix and a non-nil return.
func TestCmdNew_DaemonError(t *testing.T) {
	client := daemon.NewDaemonClient("/tmp/agenthub-nonexistent-socket-test.sock")
	var buf bytes.Buffer
	var callErr error
	stderr := captureStderr(t, func() {
		callErr = cmdNew(client, nil, &buf)
	})
	if callErr == nil {
		t.Fatal("expected error when daemon socket is unreachable, got nil")
	}
	if !strings.Contains(stderr, "agenthub new: daemon unreachable:") {
		t.Errorf("expected stderr to contain 'agenthub new: daemon unreachable:', got %q", stderr)
	}
}

// TestUsage_DocumentsNewDir: usage shows `new [<path>]` and no agent or
// `new shell` / `--` passthrough forms.
func TestUsage_DocumentsNewDir(t *testing.T) {
	src, err := os.ReadFile("cmd_cli.go")
	if err != nil {
		t.Fatalf("read cmd_cli.go: %v", err)
	}
	content := string(src)
	if !strings.Contains(content, "new [<path>]") {
		t.Error("usage() should document `new [<path>]`")
	}
	for _, gone := range []string{"new <agent>", "new shell", "<extra-args>"} {
		if strings.Contains(content, gone) {
			t.Errorf("usage() still contains removed form %q", gone)
		}
	}
}
