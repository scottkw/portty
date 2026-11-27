package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// The audit log is desktop-audit.jsonl in the config directory (DSHR-07). These
// tests read it back as a file, the way the sharer would.

func (e *desktopEnv) auditPath() string {
	return filepath.Join(e.api.engine.configDir, "desktop-audit.jsonl")
}

// auditRecords reads every line written so far.
func (e *desktopEnv) auditRecords(t *testing.T) []desktop.AuditRecord {
	t.Helper()
	f, err := os.Open(e.auditPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []desktop.AuditRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r desktop.AuditRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("audit line %q is not JSON: %v", sc.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func auditEvents(recs []desktop.AuditRecord) []string {
	var out []string
	for _, r := range recs {
		out = append(out, r.Event)
	}
	return out
}

// waitAuditEvent waits until the log holds an event named name.
func (e *desktopEnv) waitAuditEvent(t *testing.T, name string) {
	t.Helper()
	eventually(t, 5*time.Second, "an audit line "+name, func() bool {
		return slices.Contains(auditEvents(e.auditRecords(t)), name)
	})
}

func TestShareAudit_GuestSession(t *testing.T) {
	e, _ := guestEnv(t)
	gui, start := startedShare(t, e)
	g, gid := admittedGuest(t, e, gui, start.URL)

	for _, r := range []struct{ control, clipboard bool }{{true, false}, {true, true}, {false, true}} {
		if err := e.client.SetDesktopViewerRights(gid, r.control, r.clipboard); err != nil {
			t.Fatalf("SetDesktopViewerRights(%v): %v", r, err)
		}
	}
	if err := e.client.DisconnectDesktopViewer(gid); err != nil {
		t.Fatal(err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4432 {
		t.Fatalf("revoked guest closed with %d, want 4432", code)
	}
	e.waitAuditEvent(t, "leave") // the leave follows the socket, so wait before stopping
	if err := e.client.StopDesktopShare(); err != nil {
		t.Fatal(err)
	}

	recs := e.auditRecords(t)
	want := []string{"share_start", "admit_request", "admitted", "join", "control_granted", "clipboard_granted", "control_revoked", "revoked", "leave", "share_stop"}
	if got := auditEvents(recs); !slices.Equal(got, want) {
		t.Fatalf("audit events = %v\nwant           %v", got, want)
	}
	shareID := shareIDOf(t, start.URL)
	colleague := webservertest.DetailsOf(webservertest.Colleague)
	for _, r := range recs {
		if r.V != 1 || r.Mode != "guest" || r.ShareID != shareID {
			t.Errorf("%s: v=%d mode=%q share_id=%q; want 1, guest and the share's ID", r.Event, r.V, r.Mode, r.ShareID)
		}
		if r.Event == "share_start" || r.Event == "share_stop" {
			continue
		}
		if r.UserID != int64(colleague.UserID) || r.LoginName != colleague.LoginName || r.DisplayName != colleague.DisplayName ||
			r.NodeName != colleague.NodeName || r.NodeStableID != colleague.StableID {
			t.Errorf("%s: identity = %+v; want the Colleague's", r.Event, r)
		}
		if r.TailnetIP != "127.0.0.1" {
			t.Errorf("%s: tailnet_ip = %q; want the connection's address", r.Event, r.TailnetIP)
		}
	}
	if recs[8].Reason != "revoked" {
		t.Errorf("leave reason = %q, want revoked", recs[8].Reason)
	}
	if recs[9].Reason != "share_stopped" {
		t.Errorf("share_stop reason = %q, want share_stopped", recs[9].Reason)
	}

	// No line can hold the token, the link, typed text or signalling data.
	raw, err := os.ReadFile(e.auditPath())
	if err != nil {
		t.Fatal(err)
	}
	token := strings.SplitN(start.URL, "cap=", 2)[1]
	for _, bad := range []string{"cap=", token, "v=0", "candidate", "sdp", "/desktop/guest"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("the audit log contains %q", bad)
		}
	}
	if fi, err := os.Stat(e.auditPath()); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("audit file mode = %v, %v; want 0600", fi.Mode().Perm(), err)
	}
}

func TestShareAudit_OwnAccount(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	e.waitAuditEvent(t, "join")
	if err := own.conn.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Logf("close: %v", err)
	}
	e.waitAuditEvent(t, "leave")

	self := webservertest.DetailsOf(webservertest.Same)
	recs := e.auditRecords(t)
	if got := auditEvents(recs); !slices.Equal(got, []string{"join", "leave"}) {
		t.Fatalf("audit events = %v; want join, leave", got)
	}
	for _, r := range recs {
		if r.Mode != "own" || r.ShareID != "" {
			t.Errorf("%s: mode=%q share_id=%q; want own and no share ID", r.Event, r.Mode, r.ShareID)
		}
		if r.UserID != int64(self.UserID) || r.LoginName != self.LoginName || r.DisplayName != self.DisplayName ||
			r.NodeName != self.NodeName || r.NodeStableID != self.StableID {
			t.Errorf("%s: identity = %+v; want the Same identity's names and stable ID", r.Event, r)
		}
	}
	if recs[1].Reason != "viewer_closed" {
		t.Errorf("leave reason = %q, want viewer_closed", recs[1].Reason)
	}
}

func TestShareAudit_DeniedKnock(t *testing.T) {
	e, _ := guestEnv(t)
	gui, start := startedShare(t, e)
	g := connectGuest(t, start.URL)
	knock := gui.until(t, 5*time.Second, "the admit_request event", gui.event("admit_request"))
	if err := e.client.DenyDesktopGuest(knock.Viewer.ID); err != nil {
		t.Fatal(err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4430 {
		t.Errorf("denied guest closed with %d, want 4430", code)
	}
	e.waitAuditEvent(t, "denied")
	recs := e.auditRecords(t)
	if got := auditEvents(recs); !slices.Equal(got, []string{"share_start", "admit_request", "denied"}) {
		t.Fatalf("audit events = %v; want share_start, admit_request, denied", got)
	}
	if recs[2].Reason != "denied" {
		t.Errorf("denied reason = %q, want denied", recs[2].Reason)
	}
}

func TestShareAudit_TimedOutKnock(t *testing.T) {
	e, _ := guestEnv(t)
	// Every reading of this clock is six minutes later, so the knock is already
	// past its five-minute wait when the daemon starts waiting.
	var n atomic.Int64
	base := time.Now()
	e.api.guestClock = func() time.Time { return base.Add(time.Duration(n.Add(1)) * 6 * time.Minute) }
	_, start := startedShare(t, e)
	g := connectGuest(t, start.URL)
	if code := g.waitClosed(t, 5*time.Second); code != 4431 {
		t.Errorf("timed-out guest closed with %d, want 4431", code)
	}
	e.waitAuditEvent(t, "denied")
	recs := e.auditRecords(t)
	if got := auditEvents(recs); !slices.Equal(got, []string{"share_start", "admit_request", "denied"}) {
		t.Fatalf("audit events = %v; want share_start, admit_request, denied", got)
	}
	if recs[2].Reason != "timeout" {
		t.Errorf("denied reason = %q, want timeout", recs[2].Reason)
	}
}

func TestShareAudit_UnknownRequestWritesNothing(t *testing.T) {
	e, _ := guestEnv(t)
	startedShare(t, e)
	if err := e.client.AdmitDesktopGuest("nope"); !isShareErr(err, "no_request") {
		t.Errorf("admit of an unknown request = %v; want no_request", err)
	}
	if err := e.client.DenyDesktopGuest("nope"); !isShareErr(err, "no_request") {
		t.Errorf("deny of an unknown request = %v; want no_request", err)
	}
	if got := auditEvents(e.auditRecords(t)); !slices.Equal(got, []string{"share_start"}) {
		t.Errorf("audit events = %v; want only share_start (404s write no line)", got)
	}
}

func TestShareAudit_FailedWriteNeverBlocks(t *testing.T) {
	t.Run("file cannot be opened", func(t *testing.T) {
		e, _ := guestEnv(t)
		// A directory where the file must go: the audit cannot open.
		if err := os.Mkdir(e.auditPath(), 0o700); err != nil {
			t.Fatal(err)
		}
		gui, start := startedShare(t, e)
		_, gid := admittedGuest(t, e, gui, start.URL)
		if err := e.client.SetDesktopViewerRights(gid, true, true); err != nil {
			t.Errorf("a grant was refused with the audit unavailable: %v", err)
		}
	})
	t.Run("write fails", func(t *testing.T) {
		e, _ := guestEnv(t)
		gui, start := startedShare(t, e)
		e.waitAuditEvent(t, "share_start")
		e.api.audit.mu.Lock()
		_ = e.api.audit.log.Close() // later writes fail
		e.api.audit.mu.Unlock()
		_, gid := admittedGuest(t, e, gui, start.URL)
		if err := e.client.SetDesktopViewerRights(gid, true, true); err != nil {
			t.Errorf("a grant was refused with the audit writes failing: %v", err)
		}
		if got := auditEvents(e.auditRecords(t)); !slices.Equal(got, []string{"share_start"}) {
			t.Errorf("audit events = %v; want nothing after the writes began failing", got)
		}
	})
}

func TestShareAudit_DaemonExitLogsLeaves(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	who.Set(webservertest.Colleague)
	gui, start := startedShare(t, e)
	_, _ = admittedGuest(t, e, gui, start.URL)

	e.api.stopDesktop()

	leaves, stops := 0, 0
	for _, r := range e.auditRecords(t) {
		switch r.Event {
		case "leave":
			leaves++
			if r.Reason != "daemon_exit" {
				t.Errorf("leave (%s) reason = %q, want daemon_exit", r.Mode, r.Reason)
			}
		case "share_stop":
			stops++
			if r.Reason != "daemon_exit" {
				t.Errorf("share_stop reason = %q, want daemon_exit", r.Reason)
			}
		}
	}
	if leaves != 2 || stops != 1 {
		t.Errorf("daemon exit wrote %d leaves and %d share_stop; want 2 and 1", leaves, stops)
	}
}

// joinedNoMedia is admittedGuest, except the guest never applies the answer, so
// ICE never connects and no media identity check runs. Two devices of one
// person can then watch under the fake WhoIs, which answers every lookup
// (signalling and media alike) with the one current identity.
func joinedNoMedia(t *testing.T, e *desktopEnv, gui *shareStreamHolder, link string) (*guest, string) {
	t.Helper()
	g := connectGuest(t, link)
	g.waitMsg(t, "admission", "waiting")
	knock := gui.until(t, 5*time.Second, "the admit_request event", gui.event("admit_request"))
	if err := e.client.AdmitDesktopGuest(knock.Viewer.ID); err != nil {
		t.Fatalf("AdmitDesktopGuest: %v", err)
	}
	g.waitMsg(t, "admission", "admitted")
	pc, _ := newPionViewer(t)
	off, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(off); err != nil {
		t.Fatal(err)
	}
	<-gathered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	writeWS(ctx, t, g.conn, map[string]any{"type": "offer", "sdp": pc.LocalDescription().SDP})
	g.waitMsg(t, "answer", "")
	join := gui.until(t, 10*time.Second, "the join event", gui.event("join"))
	return g, join.Viewer.ID
}

// TestShareAudit_PersonCutLogsEveryDevicesLeave: a disconnect or a deny cuts
// the person on every device, and each device's leave line says why.
func TestShareAudit_PersonCutLogsEveryDevicesLeave(t *testing.T) {
	for _, tc := range []struct{ name, reason string }{{"disconnect", "revoked"}, {"deny", "denied"}} {
		t.Run(tc.name, func(t *testing.T) {
			e, who := guestEnv(t)
			gui, start := startedShare(t, e)
			who.Set(webservertest.Other)
			laptop, laptopID := joinedNoMedia(t, e, gui, start.URL)
			who.Set(webservertest.OtherSecondNode)
			want := 1 // a denied device never joined, so only the laptop leaves
			if tc.name == "disconnect" {
				want = 2
				desk, _ := joinedNoMedia(t, e, gui, start.URL)
				if err := e.client.DisconnectDesktopViewer(laptopID); err != nil {
					t.Fatal(err)
				}
				if code := desk.waitClosed(t, 5*time.Second); code != 4432 {
					t.Errorf("the person's second device closed %d, want 4432", code)
				}
			} else {
				desk := connectGuest(t, start.URL)
				desk.waitMsg(t, "admission", "waiting")
				knock := gui.until(t, 5*time.Second, "the admit_request event", gui.event("admit_request"))
				if err := e.client.DenyDesktopGuest(knock.Viewer.ID); err != nil {
					t.Fatal(err)
				}
				if code := desk.waitClosed(t, 5*time.Second); code != 4430 {
					t.Errorf("the denied device closed %d, want 4430", code)
				}
			}
			if code := laptop.waitClosed(t, 5*time.Second); code != 4432 {
				t.Errorf("the watching device closed %d, want 4432", code)
			}
			var leaves []desktop.AuditRecord
			eventually(t, 5*time.Second, "every leave line", func() bool {
				leaves = leaves[:0]
				for _, r := range e.auditRecords(t) {
					if r.Event == "leave" {
						leaves = append(leaves, r)
					}
				}
				return len(leaves) == want
			})
			for _, r := range leaves {
				if r.Reason != tc.reason {
					t.Errorf("leave of %s: reason %q, want %q", r.NodeName, r.Reason, tc.reason)
				}
			}
		})
	}
}
