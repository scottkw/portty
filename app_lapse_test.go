package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
)

// fakeLapseDaemon answers the three calls the tray poll makes: /health,
// /sessions, GET /desktop (the next scripted status each time) and the claim.
type fakeLapseDaemon struct {
	mu      sync.Mutex
	script  []daemon.DesktopStatusResponse
	next    int
	claims  []int  // the seq of every claim received
	answers []bool // answers[i] is the reply to the i-th claim; past the end: false
	guiPoll int    // GET /desktop?gui=1 count
}

func (f *fakeLapseDaemon) serve(t *testing.T) *daemon.DaemonClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("GET /sessions", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	mux.HandleFunc("GET /desktop", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.URL.Query().Get("gui") == "1" {
			f.guiPoll++
		}
		st := f.script[min(f.next, len(f.script)-1)]
		f.next++
		_ = json.NewEncoder(w).Encode(st)
	})
	mux.HandleFunc("POST /desktop/lapse-claim", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LapseSeq int `json:"lapseSeq"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		defer f.mu.Unlock()
		won := len(f.claims) < len(f.answers) && f.answers[len(f.claims)]
		f.claims = append(f.claims, req.LapseSeq)
		_ = json.NewEncoder(w).Encode(map[string]bool{"claimed": won})
	})
	sock := fmt.Sprintf("/tmp/p14_%d_%d.sock", os.Getpid(), testSockSeq.Add(1))
	_ = os.Remove(sock)
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close(); _ = os.Remove(sock) })
	return daemon.NewDaemonClient(sock)
}

type postedNote struct{ id, title, body string }

func lapseStatus(state string, seq, claimed int) daemon.DesktopStatusResponse {
	return daemon.DesktopStatusResponse{AlwaysAvailable: true, State: state, Tailscale: "ok", LapseSeq: seq, LapseClaimedSeq: claimed}
}

func newLapseApp(t *testing.T, f *fakeLapseDaemon) (*App, *[]postedNote) {
	t.Helper()
	var notes []postedNote
	a := &App{
		trayInit: true,
		client:   f.serve(t),
		sendNotificationFunc: func(id, title, body string) {
			notes = append(notes, postedNote{id, title, body})
		},
	}
	return a, &notes
}

// TestRefreshTray_ClaimsAndPostsOnce: the GUI claims a lapse once, posts only
// when it won, never posts on recovery, and a lapse the daemon won posts nothing.
func TestRefreshTray_ClaimsAndPostsOnce(t *testing.T) {
	f := &fakeLapseDaemon{
		script: []daemon.DesktopStatusResponse{
			lapseStatus("streaming", 0, 0),
			lapseStatus("awaiting_approval", 1, 0), // the GUI claims and wins
			lapseStatus("awaiting_approval", 1, 0), // same lapse: no second claim
			lapseStatus("streaming", 1, 1),         // recovery: nothing
			lapseStatus("awaiting_approval", 2, 0), // the daemon wins this one
		},
		answers: []bool{true, false},
	}
	a, notes := newLapseApp(t, f)
	for range f.script {
		a.refreshTrayState()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.claims) != 2 || f.claims[0] != 1 || f.claims[1] != 2 {
		t.Fatalf("claims = %v, want [1 2]", f.claims)
	}
	if f.guiPoll != len(f.script) {
		t.Errorf("gui=1 polls = %d, want %d", f.guiPoll, len(f.script))
	}
	want := postedNote{"portty.desktop-approval-lapse", "Portty", "Portty desktop needs approval at the Mac."}
	if len(*notes) != 1 || (*notes)[0] != want {
		t.Fatalf("notifications = %+v, want exactly [%+v]", *notes, want)
	}
}

// TestRefreshTray_SkipsClaimedLapse: a lapse somebody already claimed is not
// claimed again and posts nothing.
func TestRefreshTray_SkipsClaimedLapse(t *testing.T) {
	f := &fakeLapseDaemon{script: []daemon.DesktopStatusResponse{lapseStatus("awaiting_approval", 3, 3)}}
	a, notes := newLapseApp(t, f)
	a.refreshTrayState()
	a.refreshTrayState()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.claims) != 0 || len(*notes) != 0 {
		t.Fatalf("claims %v, notifications %+v; want none", f.claims, *notes)
	}
}

// WR-07: LapseSeq is per Manager and restarts after a daemon restart or a
// rebuilt Manager. The GUI still claims each new lapse (a lower number, or the
// same number after the state left awaiting_approval) instead of leaving it to
// the daemon's 15 s fallback.
func TestRefreshTray_ClaimsAfterLapseSeqRestarts(t *testing.T) {
	f := &fakeLapseDaemon{
		script: []daemon.DesktopStatusResponse{
			lapseStatus("awaiting_approval", 3, 0), // the GUI claims 3
			lapseStatus("awaiting_approval", 1, 0), // a new daemon's first lapse, seen straight away
			lapseStatus("idle", 0, 0),              // and another new daemon
			lapseStatus("awaiting_approval", 1, 0), // its first lapse: the same number as the last claim
		},
		answers: []bool{true, true, true},
	}
	a, notes := newLapseApp(t, f)
	for range f.script {
		a.refreshTrayState()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if fmt.Sprint(f.claims) != "[3 1 1]" {
		t.Fatalf("claims = %v, want [3 1 1]", f.claims)
	}
	if len(*notes) != 3 {
		t.Fatalf("notifications = %d, want 3", len(*notes))
	}
}
