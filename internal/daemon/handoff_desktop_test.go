package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
)

// desktopHandoff is a handoff scene with the always-available desktop on in the
// old daemon: loopback share server, synthetic capture source, one media port
// shared by both daemons (so the takeover really has to wait for it).
type desktopHandoff struct {
	*handoffScene
	env *desktopEnv
}

// newDesktopHandoff starts the scene, turns the desktop on in the old daemon
// when on is set and returns the link it answered with ("" when off).
func newDesktopHandoff(t *testing.T, on bool, scripts ...string) (*desktopHandoff, string) {
	t.Helper()
	desktop.AllowLoopbackForTest()
	useTestAppBundle(t)
	s := newHandoffScene(t, scripts...)
	d := &desktopHandoff{handoffScene: s, env: &desktopEnv{media: freeUDPPort(t)}}
	s.old.SetDesktopSeamsForTest(loopbackAddrs, d.env.newSource, d.env.media)
	if !on {
		return d, ""
	}
	resp, err := s.client.SetDesktopAlwaysAvailable(true)
	if err != nil {
		t.Fatalf("SetDesktopAlwaysAvailable(true): %v", err)
	}
	return d, resp.URL
}

// takeoverAPI is a takeover daemon with the same desktop seams and a short poll.
func (d *desktopHandoff) takeoverAPI() *API {
	n := d.newAPI(true)
	n.SetDesktopSeamsForTest(loopbackAddrs, d.env.newSource, d.env.media)
	n.desktopPollEvery = 100 * time.Millisecond
	d.mu.Lock()
	d.takeovers = append(d.takeovers, n)
	d.mu.Unlock()
	return n
}

// runTakeover is handoffScene.takeover with the desktop seams on the new API.
func (d *desktopHandoff) runTakeover(build string) (*API, int) {
	n := d.takeoverAPI()
	code := runTakeover(context.Background(), takeoverDeps{socketPath: d.socket, lockPath: d.lockPath, api: n, build: build})
	d.event(fmt.Sprintf("new:returned %d", code))
	if code != 0 && !d.holdPeer {
		d.closePeer()
	}
	return n, code
}

func (a *API) desktopLink(t *testing.T) string {
	t.Helper()
	a.mu.RLock()
	defer a.mu.RUnlock()
	link, err := a.desktopLinkLocked()
	if err != nil {
		t.Fatalf("desktop link: %v", err)
	}
	return link
}

func (a *API) hasDesktopMgr() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.desktopMgr != nil
}

// ROADMAP SC3 and SC5: with always-available on, two live sessions and one
// desktop viewer survive an upgrade: same PIDs and history, the share listener
// passes with zero terminal shares, the same link answers on the new daemon,
// the old viewer gets 1012 and a fresh one on the same link gets a keyframe.
func TestHandoffDesktop_SessionsAndLinkSurvive(t *testing.T) {
	d, link := newDesktopHandoff(t, true, tickScript(false), tickScript(true))
	s := d.handoffScene
	old := connectViewer(t, link)
	old.waitIDR(t, 10*time.Second)
	before := make([][]int, len(s.ids))
	for i, id := range s.ids {
		before[i] = ringTicks(s.old, id)
	}

	n, code := d.runTakeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0 (events %v)", code, s.events)
	}
	s.waitExit(s.oldExits, "old daemon", 0)

	for i, id := range s.ids {
		sess, ok := n.engine.registry.Get(id)
		if !ok || sess.Pid() != s.pids[i] {
			t.Errorf("session %d pid after = %v, want %d", i, sess, s.pids[i])
		}
		if err := syscall.Kill(s.pids[i], 0); err != nil {
			t.Errorf("shell %d (pid %d) is gone: %v", i, s.pids[i], err)
		}
		if after := ringTicks(n, id); len(after) < len(before[i]) || !slices.Equal(after[:len(before[i])], before[i]) {
			t.Errorf("session %d history after = %v, before %v", i, after, before[i])
		}
	}

	select {
	case <-old.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the old desktop viewer was never closed")
	}
	if got := old.code.Load(); got != int32(websocket.StatusServiceRestart) {
		t.Errorf("old desktop viewer closed with %d, want 1012", got)
	}

	eventually(t, 10*time.Second, "the takeover's Manager", n.hasDesktopMgr)
	if got := n.desktopLink(t); got != link {
		t.Errorf("link after the handoff differs from the link before it")
	}
	if code, _ := desktopPageStatus(t, link); code != 200 {
		t.Errorf("desktop page after the handoff = %d, want 200", code)
	}
	fresh := connectViewer(t, link)
	fresh.waitIDR(t, 5*time.Second)
}

// A v1.2 old daemon (nothing shared) passes no listener while the new daemon's
// desktop.json says on: the handoff commits, and the poller starts the server
// and the Manager once the old daemon has exited. Simulated by turning the
// desktop on in desktop.json after the old API loaded its (empty) setting.
func TestHandoffDesktop_NoListenerFromOldSide(t *testing.T) {
	logs := captureDiag(t)
	d, _ := newDesktopHandoff(t, false, tickScript(false))
	s := d.handoffScene
	gid, err := desktop.NewGrantID()
	if err != nil {
		t.Fatal(err)
	}
	cfg := desktop.Config{SchemaVersion: desktop.ConfigSchemaVersion, AlwaysAvailable: true, GrantID: gid, IAT: time.Now().Unix()}
	if err := desktop.SaveConfig(s.cfg, cfg); err != nil {
		t.Fatal(err)
	}

	n, code := d.runTakeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0 (events %v)", code, s.events)
	}
	s.waitExit(s.oldExits, "old daemon", 0)
	if !strings.Contains(string(logs.bytes()), "always-available server starts after the old daemon exits") {
		t.Errorf("finalize did not take the no-listener branch; log:\n%s", logs.bytes())
	}
	eventually(t, 10*time.Second, "the poller to bring up the server and the Manager", n.hasDesktopMgr)
	link := n.desktopLink(t)
	if code, _ := desktopPageStatus(t, link); code != 200 {
		t.Errorf("desktop page = %d, want 200", code)
	}
	if sess, ok := n.engine.registry.Get(s.ids[0]); !ok || sess.Pid() != s.pids[0] {
		t.Errorf("session pid after = %v, want %d", sess, s.pids[0])
	}
}

// Quick 261126-oq0: with only always-available needing the share server, a
// takeover that cannot build it (newServer fails) or is handed a listener it
// refuses (CheckListener) still commits; the sessions matter more and the
// poller starts the server after the old daemon exits.
func TestHandoffDesktop_UnusableServerStillCommits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		newServer func(*testing.T) func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error)
	}{
		{"newServer fails", func(*testing.T) func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error) {
			return func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error) {
				return nil, errors.New("injected")
			}
		}},
		{"listener refused", func(t *testing.T) func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error) {
			var built atomic.Int32
			return fixedPortShareServer(freeTCPPort(t), &built) // never the received listener's port
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureDiag(t)
			d, _ := newDesktopHandoff(t, true, tickScript(false))
			s := d.handoffScene
			n := d.takeoverAPI()
			n.SetShareSeamsForTest(healthyTailnet, tc.newServer(t), s.warmSeam)
			code := runTakeover(context.Background(), takeoverDeps{socketPath: s.socket, lockPath: s.lockPath, api: n, build: BuildVersion})
			if code != takeoverCommitted {
				t.Fatalf("runTakeover = %d, want 0 (events %v)", code, s.events)
			}
			s.waitExit(s.oldExits, "old daemon", 0)
			if !strings.Contains(string(logs.bytes()), "always-available server starts after the old daemon exits") {
				t.Errorf("finalize did not take the desktop-only branch; log:\n%s", logs.bytes())
			}
			if sess, ok := n.engine.registry.Get(s.ids[0]); !ok || sess.Pid() != s.pids[0] {
				t.Errorf("session pid after = %v, want %d", sess, s.pids[0])
			}
			if err := syscall.Kill(s.pids[0], 0); err != nil {
				t.Errorf("shell (pid %d) is gone: %v", s.pids[0], err)
			}
		})
	}
}

// A handoff that rolls back before COMMIT leaves the old daemon's desktop
// alone: same Manager, viewer still connected and streaming, no 1012.
func TestHandoffDesktop_RollbackKeepsDesktop(t *testing.T) {
	d, link := newDesktopHandoff(t, true, tickScript(false))
	s := d.handoffScene
	v := connectViewer(t, link)
	v.waitIDR(t, 10*time.Second)
	s.old.mu.RLock()
	mgr := s.old.desktopMgr
	s.old.mu.RUnlock()

	s.failAt("old:after-ready-before-commit", errors.New("injected"))
	if _, code := d.runTakeover(BuildVersion); code == takeoverCommitted {
		t.Fatalf("runTakeover committed despite the fault (events %v)", s.events)
	}
	waitFor(t, "the old daemon to resume", 5*time.Second, func() bool { return !s.old.frozen.Load() })

	s.old.mu.RLock()
	same := s.old.desktopMgr == mgr && mgr != nil
	s.old.mu.RUnlock()
	if !same {
		t.Error("the old daemon's desktop Manager changed or was dropped by a rollback")
	}
	select {
	case <-v.done:
		t.Fatalf("the desktop viewer was closed (code %d) by a rollback", v.code.Load())
	case <-time.After(300 * time.Millisecond):
	}
	got, err := s.client.DesktopStatus()
	if err != nil || got.Viewers != 1 || got.State != string(desktop.StateStreaming) {
		t.Errorf("desktop status after rollback = %+v, %v; want streaming with 1 viewer", got, err)
	}
	for len(v.aus) > 0 {
		<-v.aus
	}
	select {
	case <-v.aus:
	case <-time.After(5 * time.Second):
		t.Error("no more video after the rollback")
	}
}

// DESK-02: the takeover never takes another address or port. While the old
// Manager holds the media socket the takeover's attempt reports port_in_use and
// binds nothing; once the old one is closed the same address and port are
// bound by the new owner.
func TestHandoffDesktop_MediaPortRetry(t *testing.T) {
	d, _ := newDesktopHandoff(t, true, tickScript(false))
	n := d.takeoverAPI() // loads the same desktop.json: on
	ctx := context.Background()

	n.desktopTick(ctx)
	n.desktopTick(ctx)
	if n.hasDesktopMgr() {
		t.Fatal("the takeover bound the media port while the old daemon still held it")
	}
	d.old.closeDesktopForHandoff()

	n.desktopTick(ctx)
	if !n.hasDesktopMgr() {
		t.Fatal("the takeover did not bind the media port once the old daemon released it")
	}
	n.mu.RLock()
	builtIP := n.desktopBuiltIP
	n.mu.RUnlock()
	if builtIP != "127.0.0.1" {
		t.Errorf("Manager built for %q, want the tailnet address 127.0.0.1", builtIP)
	}
	// The one socket it holds is the same address and port the old one had.
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: d.env.media})
	if err == nil {
		_ = c.Close()
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Errorf("binding %d/udp again = %v, want EADDRINUSE (held by the new owner)", d.env.media, err)
	}
}

func readCaptureActive(t *testing.T, dir string) (active bool, exits int) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "desktop-state.json"))
	if err != nil {
		t.Fatalf("read desktop-state.json: %v", err)
	}
	var st struct {
		CaptureActive bool    `json:"captureActive"`
		AbnormalExits []int64 `json:"abnormalExits"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	return st.CaptureActive, len(st.AbnormalExits)
}

// After COMMITTED the old daemon has stopped capture cleanly, so the takeover's
// Manager counts no abnormal exit.
func TestHandoffDesktop_OldSideClearsCaptureActive(t *testing.T) {
	d, link := newDesktopHandoff(t, true, tickScript(false))
	s := d.handoffScene
	v := connectViewer(t, link)
	v.waitIDR(t, 10*time.Second)
	eventually(t, 5*time.Second, "captureActive to be recorded", func() bool {
		a, _ := readCaptureActive(t, s.cfg)
		return a
	})

	n, code := d.runTakeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0 (events %v)", code, s.events)
	}
	s.waitExit(s.oldExits, "old daemon", 0)
	if active, _ := readCaptureActive(t, s.cfg); active {
		t.Error("captureActive is still true after the old daemon handed off")
	}
	eventually(t, 10*time.Second, "the takeover's Manager", n.hasDesktopMgr)
	if active, exits := readCaptureActive(t, s.cfg); active || exits != 0 {
		t.Errorf("state after the takeover's Manager started: captureActive=%v abnormalExits=%d, want false and 0", active, exits)
	}
}

// With no terminal share and always-available off, no share listener is passed
// (as before this phase); either of the two reasons to keep the server passes it.
func TestHandoffDesktop_DesktopOffNoListener(t *testing.T) {
	d, _ := newDesktopHandoff(t, false, tickScript(false))
	s := d.handoffScene
	pass := func() (hsGlobal, int) {
		g, conns, err := (&oldHandoff{a: s.old, frozen: s.old.engine.handoffEntries()}).global()
		if err != nil {
			t.Fatalf("global: %v", err)
		}
		return g, len(conns)
	}
	if g, n := pass(); g.ShareListener || n != 3 || len(g.Shares) != 0 {
		t.Errorf("nothing shared, desktop off: ShareListener=%v fds=%d shares=%d, want false 3 0", g.ShareListener, n, len(g.Shares))
	}
	mustShare(t, s.client, s.ids[0])
	if g, n := pass(); !g.ShareListener || n != 4 || len(g.Shares) != 1 {
		t.Errorf("one terminal share: ShareListener=%v fds=%d shares=%d, want true 4 1", g.ShareListener, n, len(g.Shares))
	}
	if _, err := s.client.SetSessionShared(s.ids[0], false); err != nil {
		t.Fatal(err)
	}
	if g, n := pass(); g.ShareListener || n != 3 {
		t.Errorf("share turned off again: ShareListener=%v fds=%d, want false 3", g.ShareListener, n)
	}
	if _, err := s.client.SetDesktopAlwaysAvailable(true); err != nil {
		t.Fatal(err)
	}
	if g, n := pass(); !g.ShareListener || n != 4 || len(g.Shares) != 0 {
		t.Errorf("desktop on, no share: ShareListener=%v fds=%d shares=%d, want true 4 0", g.ShareListener, n, len(g.Shares))
	}
}
