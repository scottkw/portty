package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

func healthyTailnetForTest(context.Context) webserver.TailscaleHealth {
	return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: "127.0.0.1"}
}

// loopbackShareServerForTest stands in for the tailnet share server on 7444.
func loopbackShareServerForTest(_ webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
	cfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		return nil, err
	}
	ws, err := webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", Port: 0, FQDN: "127.0.0.1", TLSConfig: cfg}, m)
	if err != nil {
		return nil, err
	}
	ws.SetWhoIsClientForTest(webservertest.SameUser())
	return ws, nil
}

// testShareSetup is testSetup with a bootstrapped signing key and share seams
// that serve shares on a loopback server. It returns the API so tests can
// swap the health seam.
func testShareSetup(t *testing.T) (*daemon.API, *daemon.DaemonClient) {
	t.Helper()
	tmpCfg := t.TempDir()
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", tmpCfg)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmpCfg)
	}
	engine := daemon.NewSessionEngine()
	engine.SetSpawnCommandForTest("/bin/cat", nil)
	api := daemon.NewAPI(engine)
	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatalf("BootstrapCapabilityState: %v", err)
	}
	api.SetShareSeamsForTest(healthyTailnetForTest, loopbackShareServerForTest, func(context.Context, string) error { return nil })
	socketPath := fmt.Sprintf("/tmp/ahs%d_%d.sock", os.Getpid(), time.Now().UnixNano()%100000)
	t.Cleanup(func() { os.Remove(socketPath) })
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { api.Stop() })
	time.Sleep(10 * time.Millisecond)
	return api, daemon.NewDaemonClient(socketPath)
}

func TestResolveSession(t *testing.T) {
	sessions := []daemon.SessionInfo{
		{ID: "aaaa1111-0000", Name: "work"},
		{ID: "bbbb2222-0000", Name: "dup"},
		{ID: "bbbb3333-0000", Name: "dup"},
		{ID: "cccc4444-0000", Name: "aaaa1111-0000"}, // a name that equals another session's ID
	}
	cases := []struct {
		arg, wantID, wantErr string
	}{
		{arg: "work", wantID: "aaaa1111-0000"},
		{arg: "aaaa1111-0000", wantID: "cccc4444-0000"}, // exact name beats exact ID
		{arg: "bbbb2222-0000", wantID: "bbbb2222-0000"},
		{arg: "cccc", wantID: "cccc4444-0000"},
		{arg: "dup", wantErr: `portty: more than one session is named "dup". Use an ID prefix instead: bbbb2222, bbbb3333.`},
		{arg: "bbbb", wantErr: `portty: no session named "bbbb". Run 'portty list' to see your sessions.`},
		{arg: "nope", wantErr: `portty: no session named "nope". Run 'portty list' to see your sessions.`},
	}
	for _, tc := range cases {
		got, err := resolveSession(sessions, tc.arg)
		if tc.wantErr != "" {
			if err == nil || err.Error() != tc.wantErr {
				t.Errorf("resolveSession(%q) err = %v; want %q", tc.arg, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got.ID != tc.wantID {
			t.Errorf("resolveSession(%q) = %q, %v; want %q", tc.arg, got.ID, err, tc.wantID)
		}
	}
}

func TestCmdShare_PrintsLinkAndCTOnFirstShare(t *testing.T) {
	_, client := testShareSetup(t)
	if _, err := client.CreateSession("alpha", "/tmp", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateSession("beta", "/tmp", 0, 0); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdShare(client, []string{"alpha"}, &buf); err != nil {
		t.Fatalf("cmdShare: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("first share printed %d lines; want 4:\n%s", len(lines), buf.String())
	}
	if lines[0] != `Shared "alpha".` || !strings.HasPrefix(lines[1], "https://127.0.0.1:") || !strings.Contains(lines[1], "?cap=") ||
		lines[2] != "Treat this link like a password. It only opens on devices signed in to your Tailscale account." ||
		lines[3] != "The first share gets an HTTPS certificate for 127.0.0.1. That name is published in public Certificate Transparency logs." {
		t.Fatalf("first share output:\n%s", buf.String())
	}

	buf.Reset()
	if err := cmdShare(client, []string{"beta"}, &buf); err != nil {
		t.Fatalf("cmdShare beta: %v", err)
	}
	if n := strings.Count(buf.String(), "\n"); n != 3 || strings.Contains(buf.String(), "Certificate Transparency") {
		t.Fatalf("second share output (want 3 lines, no CT line):\n%s", buf.String())
	}
}

func TestCmdShare_Errors(t *testing.T) {
	api, client := testShareSetup(t)
	if err := cmdShare(client, nil, &bytes.Buffer{}); err == nil || err.Error() != "usage: portty share <session>" {
		t.Errorf("no argument: %v", err)
	}
	if _, err := client.CreateSession("alpha", "/tmp", 0, 0); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		health webserver.TailscaleHealth
		want   string
	}{
		{webserver.TailscaleHealth{}, "portty: Tailscale isn't connected. Remote access and sharing are off until it is."},
		{webserver.TailscaleHealth{Connected: true, IP: "127.0.0.1"}, "portty: couldn't start sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again."},
	}
	for _, tc := range cases {
		h := tc.health
		api.SetShareSeamsForTest(func(context.Context) webserver.TailscaleHealth { return h }, loopbackShareServerForTest, func(context.Context, string) error { return nil })
		if err := cmdShare(client, []string{"alpha"}, &bytes.Buffer{}); err == nil || err.Error() != tc.want {
			t.Errorf("health %+v: err = %v; want %q", h, err, tc.want)
		}
	}
	api.SetShareSeamsForTest(healthyTailnetForTest, loopbackShareServerForTest, func(context.Context, string) error { return fmt.Errorf("boom") })
	want := "portty: couldn't start sharing: couldn't get an HTTPS certificate: boom. Check that Tailscale is connected, then try again."
	if err := cmdShare(client, []string{"alpha"}, &bytes.Buffer{}); err == nil || err.Error() != want {
		t.Errorf("warm-up failure: err = %v; want %q", err, want)
	}
}

func TestCmdUnshare(t *testing.T) {
	_, client := testShareSetup(t)
	if err := cmdUnshare(client, nil, &bytes.Buffer{}); err == nil || err.Error() != "usage: portty unshare <session>" {
		t.Errorf("no argument: %v", err)
	}
	if _, err := client.CreateSession("alpha", "/tmp", 0, 0); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdUnshare(client, []string{"alpha"}, &buf); err != nil || buf.String() != "\"alpha\" isn't shared.\n" {
		t.Fatalf("unshare when not shared = %q, %v", buf.String(), err)
	}
	if err := cmdShare(client, []string{"alpha"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := cmdUnshare(client, []string{"alpha"}, &buf); err != nil || buf.String() != "Stopped sharing \"alpha\". Remote viewers were disconnected.\n" {
		t.Fatalf("unshare = %q, %v", buf.String(), err)
	}
	sessions, _ := client.ListSessions()
	for _, s := range sessions {
		if s.WebEnabled {
			t.Fatalf("session %s still shared after unshare", s.Name)
		}
	}
}
