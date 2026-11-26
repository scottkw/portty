package daemon

import (
	"context"
	"fmt"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
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
