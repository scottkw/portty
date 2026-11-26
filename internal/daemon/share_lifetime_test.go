package daemon

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
)

// terminalViewer is a web viewer on a shared session's /sessions/{id}/ws.
type terminalViewer struct {
	conn *websocket.Conn
	out  chan []byte
}

func attachTerminalViewer(t *testing.T, link, id string) *terminalViewer {
	t.Helper()
	u, token := linkParts(t, link)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hdr := http.Header{}
	hdr.Set("Origin", "https://"+u.Host)
	wsURL := "wss://" + u.Host + "/sessions/" + id + "/ws?cap=" + url.QueryEscape(token)
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: insecureNoRedirectClient(), HTTPHeader: hdr})
	if err != nil {
		t.Fatalf("terminal viewer dial: %v", err)
	}
	v := &terminalViewer{conn: conn, out: make(chan []byte, 256)}
	go func() {
		for {
			_, m, err := conn.Read(context.Background())
			if err != nil {
				close(v.out)
				return
			}
			if len(m) > 0 && m[0] == relay.MsgOutput {
				v.out <- m[1:]
			}
		}
	}()
	t.Cleanup(func() { _ = conn.CloseNow() })
	return v
}

// echoes types marker into the session (a cat process echoes it back) and
// waits for it to arrive at the viewer: the viewer is connected and live.
func (v *terminalViewer) echoes(t *testing.T, marker string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := v.conn.Write(ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte(marker+"\n"))); err != nil {
		t.Fatalf("terminal viewer write %q: %v", marker, err)
	}
	var got []byte
	deadline := time.After(5 * time.Second)
	for !bytes.Contains(got, []byte(marker)) {
		select {
		case b, ok := <-v.out:
			if !ok {
				t.Fatalf("terminal viewer was disconnected before %q echoed (output so far %q)", marker, got)
			}
			got = append(got, b...)
		case <-deadline:
			t.Fatalf("no echo of %q within 5s (output so far %q)", marker, got)
		}
	}
}

func currentShareServer(api *API) *webserver.WebServer {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.webServer
}

// drainAUs empties a viewer's access-unit channel.
func (v *desktopViewer) drainAUs() {
	for {
		select {
		case <-v.aus:
		default:
			return
		}
	}
}

// stillStreaming: after the call a fresh access unit arrives within 5 s.
func (v *desktopViewer) stillStreaming(t *testing.T, why string) {
	t.Helper()
	v.drainAUs()
	select {
	case <-v.aus:
	case <-v.done:
		t.Fatalf("desktop viewer was disconnected %s", why)
	case <-time.After(5 * time.Second):
		t.Fatalf("desktop viewer stopped receiving video %s", why)
	}
}

func TestShareLifetime_DesktopOffKeepsTerminal(t *testing.T) {
	e := newDesktopEnv(t)
	id := newShareSession(t, e.client)
	link := mustShare(t, e.client, id)
	tv := attachTerminalViewer(t, link.URL, id)
	tv.echoes(t, "before-desktop")

	e.mustOn(t)
	server := currentShareServer(e.api)
	if server == nil {
		t.Fatal("no share server after desktop on")
	}
	tv.echoes(t, "desktop-on")

	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Fatal(err)
	}
	if currentShareServer(e.api) != server {
		t.Fatal("desktop off replaced or stopped the share server a terminal share still uses")
	}
	tv.echoes(t, "after-desktop-off")
	if code, _ := linkStatus(t, link.URL); code != http.StatusFound {
		t.Errorf("terminal link after desktop off = %d, want 302", code)
	}
}

func TestShareLifetime_TerminalOffKeepsDesktop(t *testing.T) {
	e := newDesktopEnv(t)
	on := e.mustOn(t)
	dv := connectViewer(t, on.URL)
	dv.waitIDR(t, 10*time.Second)

	id := newShareSession(t, e.client)
	mustShare(t, e.client, id)
	dv.stillStreaming(t, "after a terminal share was turned on")
	if _, err := e.client.SetSessionShared(id, false); err != nil {
		t.Fatal(err)
	}
	if !shareServerRunning(e.api) {
		t.Fatal("the last terminal share off stopped the server the desktop link needs")
	}
	dv.stillStreaming(t, "after the last terminal share was turned off")
	if code, _ := desktopPageStatus(t, on.URL); code != http.StatusOK {
		t.Errorf("desktop link after the terminal share went off = %d, want 200", code)
	}
}

// TestShareLifetime_AllOrders: for each order of turning the terminal share (T)
// and the desktop (D) on and off, the server runs exactly while at least one is
// on, and its listener is closed after the last one goes off.
func TestShareLifetime_AllOrders(t *testing.T) {
	orders := []struct {
		name string
		ops  []string
	}{
		{"T on, D on, T off, D off", []string{"T+", "D+", "T-", "D-"}},
		{"T on, D on, D off, T off", []string{"T+", "D+", "D-", "T-"}},
		{"D on, T on, T off, D off", []string{"D+", "T+", "T-", "D-"}},
		{"D on, T on, D off, T off", []string{"D+", "T+", "D-", "T-"}},
	}
	for _, o := range orders {
		t.Run(o.name, func(t *testing.T) {
			e := newDesktopEnv(t)
			id := newShareSession(t, e.client)
			var host string
			termOn, deskOn := false, false
			for i, op := range o.ops {
				switch op {
				case "T+":
					host = mustShareHost(t, e, id)
					termOn = true
				case "T-":
					if _, err := e.client.SetSessionShared(id, false); err != nil {
						t.Fatal(err)
					}
					termOn = false
				case "D+":
					u, _ := linkParts(t, e.mustOn(t).URL)
					host = u.Host
					deskOn = true
				case "D-":
					if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
						t.Fatal(err)
					}
					deskOn = false
				}
				want := termOn || deskOn
				if got := shareServerRunning(e.api); got != want {
					t.Fatalf("after step %d (%s): server running = %v, want %v", i+1, op, got, want)
				}
				if want {
					if c, err := insecureNoRedirectClient().Get("https://" + host + "/"); err != nil {
						t.Fatalf("after step %d (%s): server not answering: %v", i+1, op, err)
					} else {
						_ = c.Body.Close()
					}
				} else {
					assertNotListening(t, host)
				}
			}
		})
	}
}

func mustShareHost(t *testing.T, e *desktopEnv, id string) string {
	t.Helper()
	u, _ := linkParts(t, mustShare(t, e.client, id).URL)
	return u.Host
}

func TestShareLifetime_SessionExitWithDesktopOn(t *testing.T) {
	e := newDesktopEnv(t)
	on := e.mustOn(t)
	id := newShareSession(t, e.client)
	mustShare(t, e.client, id)

	e.api.runSessionExitCleanupForTest(id)
	if got, _ := e.client.GetSessionShare(id); got.Shared {
		t.Fatalf("exited session still shared: %+v", got)
	}
	if !shareServerRunning(e.api) {
		t.Fatal("the shared session's exit stopped the server the desktop link needs")
	}
	if code, _ := desktopPageStatus(t, on.URL); code != http.StatusOK {
		t.Errorf("desktop link after the shared session exited = %d, want 200", code)
	}
}

// TestShareLifetime_ConcurrentStartsShareOneServer: a terminal share and the
// desktop turned on at the same instant go through one start path, so one
// server is built and both succeed.
func TestShareLifetime_ConcurrentStartsShareOneServer(t *testing.T) {
	e := newDesktopEnv(t)
	var built atomic.Int32
	e.api.SetShareSeamsForTest(healthyTailnet, fixedPortShareServer(freeTCPPort(t), &built), noWarm)
	id := newShareSession(t, e.client)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := e.client.SetSessionShared(id, true)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := e.client.SetDesktopAlwaysAvailable(true)
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent turn-on failed: %v", err)
		}
	}
	if n := built.Load(); n != 1 {
		t.Errorf("share server built %d times, want 1", n)
	}
	link, err := e.client.DesktopLink()
	if err != nil || !strings.Contains(link, "/desktop/?cap=") {
		t.Fatalf("DesktopLink = %q, %v", link, err)
	}
	if code, _ := desktopPageStatus(t, link); code != http.StatusOK {
		t.Errorf("desktop link = %d, want 200", code)
	}
}
