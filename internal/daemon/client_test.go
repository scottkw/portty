package daemon

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientHealth(t *testing.T) {
	_, client, _ := testDaemon(t)
	if err := client.Health(); err != nil {
		t.Errorf("Health: unexpected error: %v", err)
	}
}

// UPGR-04: HealthInfo decodes /health; a body without "api" (v1.0, v1.1)
// reads as API 1, and this build's daemon reports APIVersion.
func TestHealthInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"v1.1.0"}`)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	h, err := (&DaemonClient{http: srv.Client(), base: srv.URL}).HealthInfo()
	if err != nil || h.Version != "v1.1.0" || h.Status != "ok" {
		t.Fatalf("v1.1 health = %+v, %v", h, err)
	}
	if got := DaemonAPI(h); got != 1 {
		t.Fatalf("DaemonAPI(v1.1 body) = %d, want 1", got)
	}
	if MinDaemonAPI != 1 {
		t.Fatalf("MinDaemonAPI = %d, want 1 (a v1.1 daemon kept with Not Now keeps working)", MinDaemonAPI)
	}

	_, client, _ := testDaemon(t)
	h, err = client.HealthInfo()
	if err != nil || DaemonAPI(h) != 4 || APIVersion != 4 || PreviewAPI != 3 || PathClickAPI != 4 {
		t.Fatalf("this build's health = %+v, %v; want API 4 (PreviewAPI 3, PathClickAPI 4)", h, err)
	}
}

// 09-04's handoff freeze answers mutating routes with 503 and Retry-After: 1.
// Mutating calls and /shutdown wait it out within frozenRetryBudget, then
// return the 503 as an error; a GET is never retried (reads still work).
func TestDaemonClient_Waits503(t *testing.T) {
	serve := func(t *testing.T, frozen int) (*DaemonClient, *atomic.Int64) {
		t.Helper()
		path := shortProcTestSocket(t, "frozen")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		var calls atomic.Int64
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
			if n := int(calls.Add(1)); frozen < 0 || n <= frozen { // frozen < 0: forever
				w.Header().Set("Retry-After", "1")
				http.Error(w, "frozen for a handoff", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		return NewDaemonClient(path), &calls
	}

	for name, call := range map[string]func(*DaemonClient) error{
		"shutdown": (*DaemonClient).ShutdownDaemon,
		"delete":   func(c *DaemonClient) error { return c.KillSession("a") },
	} {
		t.Run(name+" waits out two 503s", func(t *testing.T) {
			c, calls := serve(t, 2)
			start := time.Now()
			if err := call(c); err != nil {
				t.Fatalf("after two 503s: %v", err)
			}
			if d := time.Since(start); calls.Load() != 3 || d < 1900*time.Millisecond || d > 4*time.Second {
				t.Fatalf("calls %d in %v; want 3 in about 2 s", calls.Load(), d)
			}
		})
		t.Run(name+" gives up after the budget", func(t *testing.T) {
			old := frozenRetryBudget
			frozenRetryBudget = 1500 * time.Millisecond
			t.Cleanup(func() { frozenRetryBudget = old })
			c, _ := serve(t, -1)
			start := time.Now()
			if err := call(c); err == nil || !strings.Contains(err.Error(), "503") {
				t.Fatalf("a daemon frozen forever: err %v, want the 503", err)
			}
			if d := time.Since(start); d > 3*time.Second {
				t.Fatalf("took %v, want the 1.5 s budget", d)
			}
		})
	}

	t.Run("GET is not retried", func(t *testing.T) {
		c, calls := serve(t, 2)
		if err := c.Health(); err == nil || calls.Load() != 1 {
			t.Fatalf("GET: err %v, calls %d; want the 503 at once", err, calls.Load())
		}
	})
}

func TestClientListSessions(t *testing.T) {
	_, client, _ := testDaemon(t)
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions == nil {
		t.Fatal("ListSessions returned nil, want empty slice")
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestClientCreateSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("test-tab", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession returned empty ID")
	}
	t.Cleanup(func() { client.KillSession(id) })
}

func TestClientKillSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("kill-me", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := client.KillSession(id); err != nil {
		t.Errorf("KillSession: %v", err)
	}
}

func TestClientRenameSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("original", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { client.KillSession(id) })

	if err := client.RenameSession(id, "renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after rename: %v", err)
	}
	var found SessionInfo
	for _, s := range sessions {
		if s.ID == id {
			found = s
			break
		}
	}
	if found.ID == "" {
		t.Fatal("session not found after rename")
	}
	if found.Name != "renamed" {
		t.Errorf("name after rename: got %q, want %q", found.Name, "renamed")
	}
}

func TestClientGetRelayPort(t *testing.T) {
	api, client, _ := testDaemon(t)
	port, err := api.StartRelay()
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	if port <= 0 {
		t.Fatalf("StartRelay returned invalid port: %d", port)
	}

	got, err := client.GetRelayPort()
	if err != nil {
		t.Fatalf("GetRelayPort: %v", err)
	}
	if got <= 0 {
		t.Errorf("GetRelayPort: want > 0, got %d", got)
	}
	if got != port {
		t.Errorf("GetRelayPort: got %d, want %d", got, port)
	}
}

func TestClientGetSessionShare_NotShared(t *testing.T) {
	_, client, _ := testDaemon(t)
	resp, err := client.GetSessionShare("no-such-session")
	if err != nil {
		t.Fatalf("GetSessionShare: %v", err)
	}
	if resp.Shared || resp.URL != "" {
		t.Errorf("GetSessionShare = %+v; want not shared", resp)
	}
}

func TestShutdownDaemon(t *testing.T) {
	// Create a test HTTP server that records the shutdown call
	var shutdownCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/shutdown" {
			shutdownCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	// Create client pointing to test server (TCP, not Unix socket)
	client := &DaemonClient{
		http: srv.Client(),
		base: srv.URL,
	}
	err := client.ShutdownDaemon()
	if err != nil {
		t.Fatalf("ShutdownDaemon returned error: %v", err)
	}
	if !shutdownCalled {
		t.Error("expected /shutdown endpoint to be called")
	}
}

// IN-05: a dropped /shutdown is not proof the sessions' daemon stopped:
// during a handoff the exiting old daemon drops connections while a takeover
// keeps the sessions. If a daemon still answers /health on a fresh
// connection, /shutdown goes to it once more; if none answers, it stopped.
func TestShutdownDaemon_DroppedThenRechecked(t *testing.T) {
	serve := func(t *testing.T, stillAnswers bool) (*DaemonClient, *atomic.Int64) {
		t.Helper()
		path := shortProcTestSocket(t, "drop")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		var shutdowns atomic.Int64
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
			switch r.URL.Path {
			case "/health":
				w.Write([]byte(`{"status":"ok","version":"v1.2.0-new"}`)) //nolint:errcheck
			case "/shutdown":
				if shutdowns.Add(1) == 1 { // the old daemon leaving mid-request
					if !stillAnswers {
						ln.Close()
					}
					conn, _, _ := w.(http.Hijacker).Hijack()
					conn.Close()
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		return NewDaemonClient(path), &shutdowns
	}
	t.Run("a takeover still answers", func(t *testing.T) {
		c, shutdowns := serve(t, true)
		if err := c.ShutdownDaemon(); err != nil {
			t.Fatalf("ShutdownDaemon: %v", err)
		}
		if shutdowns.Load() != 2 {
			t.Fatalf("/shutdown sent %d times, want 2 (the daemon that still answers is stopped too)", shutdowns.Load())
		}
	})
	t.Run("nothing answers", func(t *testing.T) {
		c, shutdowns := serve(t, false)
		if err := c.ShutdownDaemon(); err != nil {
			t.Fatalf("ShutdownDaemon: %v", err)
		}
		if shutdowns.Load() != 1 {
			t.Fatalf("/shutdown sent %d times, want 1", shutdowns.Load())
		}
	})
	// IN-03 (it2): the second /shutdown gets what is left of one budget, not
	// a fresh one. 503 at 0 s, the drop at 1 s, 503s at 1 s and 2 s, then
	// 2 s + 1 s is past the 2.5 s budget: 4 posts (a fresh budget makes 5).
	t.Run("the second post shares the budget", func(t *testing.T) {
		old := frozenRetryBudget
		frozenRetryBudget = 2500 * time.Millisecond
		t.Cleanup(func() { frozenRetryBudget = old })
		path := shortProcTestSocket(t, "drop")
		ln, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		var shutdowns atomic.Int64
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
			switch {
			case r.URL.Path == "/health":
				w.Write([]byte(`{"status":"ok","version":"v1.2.0-new"}`)) //nolint:errcheck
			case shutdowns.Add(1) == 2:
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
			default:
				w.Header().Set("Retry-After", "1")
				http.Error(w, "frozen for a handoff", http.StatusServiceUnavailable)
			}
		}))
		err = NewDaemonClient(path).ShutdownDaemon()
		if err == nil || !strings.Contains(err.Error(), "503") || shutdowns.Load() != 4 {
			t.Fatalf("err %v after %d posts; want the 503 after 4", err, shutdowns.Load())
		}
	})
}

// TestClientFullLifecycle tests the full round-trip: create -> list -> rename -> list -> kill -> list.
func TestClientFullLifecycle(t *testing.T) {
	_, client, _ := testDaemon(t)

	// Create.
	id, err := client.CreateSession("tab-one", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// List — should have 1 session.
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "tab-one" {
		t.Errorf("name: got %q, want %q", sessions[0].Name, "tab-one")
	}

	// Rename.
	if err := client.RenameSession(id, "tab-renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	// List — should show new name.
	sessions, err = client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after rename: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "tab-renamed" {
		t.Errorf("after rename: got %+v", sessions)
	}

	// Kill.
	if err := client.KillSession(id); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	// Give the session a moment to be removed.
	time.Sleep(50 * time.Millisecond)

	// List — should be empty.
	sessions, err = client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after kill: %v", err)
	}
	for _, s := range sessions {
		if s.ID == id {
			t.Errorf("killed session %q still in list", id)
		}
	}
}

type previewCall struct {
	path, name, mime, key, query string
	body                         []byte
}

// previewServer answers the n-th request (from 1) with answer(n, w) on a unix
// socket and records each request's body and headers.
func previewServer(t *testing.T, answer func(n int, w http.ResponseWriter)) (*DaemonClient, func() []previewCall) {
	t.Helper()
	path := shortProcTestSocket(t, "prev")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var calls []previewCall
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, previewCall{r.URL.Path, r.Header.Get("X-Portty-Name"), r.Header.Get("X-Portty-Mime"), r.Header.Get("X-Portty-Key"), r.URL.RawQuery, body})
		n := len(calls)
		mu.Unlock()
		answer(n, w)
	}))
	return NewDaemonClient(path), func() []previewCall {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(calls)
	}
}

func retryAfter(w http.ResponseWriter, status int) {
	w.Header().Set("Retry-After", "1")
	http.Error(w, "wait", status)
}

func accepted(w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"viewers":2}`)) //nolint:errcheck
}

func TestSendPreview_RetriesFrozenWithFreshBody(t *testing.T) {
	data := []byte("\x89PNG fake image bytes")
	c, calls := previewServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			retryAfter(w, http.StatusServiceUnavailable)
			return
		}
		accepted(w)
	})
	resp, err := c.SendPreview("s1", "ré sumé/x.png", "image/png", previewTestKey, data)
	if err != nil || resp.Viewers != 2 {
		t.Fatalf("SendPreview = %+v, %v; want Viewers 2", resp, err)
	}
	got := calls()
	if len(got) != 2 {
		t.Fatalf("%d requests; want 2", len(got))
	}
	for i, call := range got {
		if !bytes.Equal(call.body, data) {
			t.Errorf("attempt %d body %q; want the full body", i+1, call.body)
		}
		if call.path != "/sessions/s1/preview" || call.query != "" || call.mime != "image/png" || call.key != previewTestKey {
			t.Errorf("attempt %d request %+v", i+1, call)
		}
		if call.name != url.QueryEscape("ré sumé/x.png") {
			t.Errorf("attempt %d name header %q; want it percent-encoded", i+1, call.name)
		}
	}
}

func TestSendPreview_WaitsOutBusy(t *testing.T) {
	c, calls := previewServer(t, func(n int, w http.ResponseWriter) {
		if n <= 2 {
			retryAfter(w, http.StatusTooManyRequests)
			return
		}
		accepted(w)
	})
	if _, err := c.SendPreview("s1", "a.md", "text/markdown", previewTestKey, []byte("# a")); err != nil || len(calls()) != 3 {
		t.Fatalf("after two 429s: err %v, %d requests; want success on the third", err, len(calls()))
	}

	old := frozenRetryBudget
	frozenRetryBudget = 1500 * time.Millisecond
	t.Cleanup(func() { frozenRetryBudget = old })
	c, _ = previewServer(t, func(_ int, w http.ResponseWriter) { retryAfter(w, http.StatusTooManyRequests) })
	start := time.Now()
	if _, err := c.SendPreview("s1", "a.md", "text/markdown", previewTestKey, []byte("# a")); !errors.Is(err, ErrPreviewBusy) {
		t.Fatalf("busy until the budget ends: err %v; want ErrPreviewBusy", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v; want the 1.5 s budget", d)
	}
}

func TestSendPreview_TypedErrors(t *testing.T) {
	for status, want := range map[int]error{
		http.StatusNotFound:              ErrPreviewNoSession,
		http.StatusConflict:              ErrPreviewEnded,
		http.StatusRequestEntityTooLarge: ErrPreviewTooLarge,
		http.StatusUnsupportedMediaType:  ErrPreviewUnsupported,
	} {
		c, _ := previewServer(t, func(_ int, w http.ResponseWriter) { http.Error(w, "no", status) })
		if _, err := c.SendPreview("s1", "a.png", "image/png", previewTestKey, []byte("x")); !errors.Is(err, want) {
			t.Errorf("status %d: err %v; want %v", status, err, want)
		}
	}
	c, _ := previewServer(t, func(_ int, w http.ResponseWriter) { http.Error(w, "invalid key", http.StatusBadRequest) })
	_, err := c.SendPreview("s1", "a.png", "image/png", "bad", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("status 400: err %v; want an error carrying the status", err)
	}
	for _, typed := range []error{ErrPreviewNoSession, ErrPreviewEnded, ErrPreviewTooLarge, ErrPreviewUnsupported, ErrPreviewBusy} {
		if errors.Is(err, typed) {
			t.Errorf("status 400 matched %v", typed)
		}
	}
}
