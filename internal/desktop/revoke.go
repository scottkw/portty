package desktop

import (
	"sync"
	"time"

	"github.com/coder/websocket"
)

// revoke.go: cutting viewers. A cut ends the viewer's input first (what it sent
// before is discarded, what it holds is let go, nothing more is accepted), then
// its media, then its socket. The socket close can wait on a peer that never
// answers, so it runs on its own goroutine with a ceiling after which the
// connection is dropped without the handshake.

// closeCeiling is the longest a graceful close handshake may take before the
// socket is dropped (coder/websocket's Close can wait about 5 s plus 5 s).
const closeCeiling = 2 * time.Second

// CloseViewer cuts one viewer at once. A guest is closed 4432 revoked and its
// person is deny-listed for the rest of the share, from every device; an
// own-account device is closed 1001 disconnected and may reconnect through
// always-available. Calling it twice, or while the viewer is leaving, is safe.
func (m *Manager) CloseViewer(id ViewerID) error {
	v := m.lookupViewer(id)
	if v == nil {
		return ErrNoViewer
	}
	if v.Mode == ModeGuest {
		m.cutViewer(v, CloseGuestRevoked, guestReasonRevoked, true)
	} else {
		m.cutViewer(v, websocket.StatusGoingAway, "disconnected", false)
	}
	return nil
}

// CloseViewerWith is CloseViewer with the close code and reason chosen by the
// caller, and no deny-list entry.
func (m *Manager) CloseViewerWith(id ViewerID, code websocket.StatusCode, reason string) error {
	v := m.lookupViewer(id)
	if v == nil {
		return ErrNoViewer
	}
	m.cutViewer(v, code, reason, false)
	return nil
}

// CloseGuests cuts every guest viewer at once and concurrently; own-account
// viewers stay. The caller ends the GuestShare first, so a guest that is still
// joining fails its re-check. It returns when every guest's input and media are
// cut; the socket closes finish within closeCeiling on their own.
func (m *Manager) CloseGuests(code websocket.StatusCode, reason string) {
	m.vmu.Lock()
	var guests []*viewer
	for _, v := range m.viewers {
		if v.Mode == ModeGuest {
			guests = append(guests, v)
		}
	}
	m.vmu.Unlock()
	var wg sync.WaitGroup
	for _, v := range guests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverLog()
			m.cutViewer(v, code, reason, false)
		}()
	}
	wg.Wait()
}

func (m *Manager) lookupViewer(id ViewerID) *viewer {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	return m.viewers[id]
}

// cutViewer runs the cut once per viewer; later calls return at once. Order
// matters: rights go first so nothing the viewer sends is honoured (clipboard
// included), then its input life ends and the input goroutine is waited for (a
// message it was handling finishes before this returns, and none is started
// after), then the person is deny-listed (a guest
// revoked by the sharer), then media and finally the socket.
func (m *Manager) cutViewer(v *viewer, code websocket.StatusCode, reason string, deny bool) {
	if !v.cut.CompareAndSwap(false, true) {
		return
	}
	v.setRightsBits(RightsViewOnly)
	m.endViewerInput(v, "revoked")
	m.flushInput()
	if deny && v.gs != nil {
		v.gs.RevokeUser(v.Identity.UserID)
	}
	if v.pc != nil {
		_ = v.pc.Close()
	}
	go closeSocket(v.ws, code, reason)
}

// closeSocket closes c with code and reason, and drops it without the handshake
// once closeCeiling has passed.
func closeSocket(c *websocket.Conn, code websocket.StatusCode, reason string) {
	defer recoverLog()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverLog()
		_ = c.Close(code, reason)
	}()
	t := time.NewTimer(closeCeiling)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
		_ = c.CloseNow()
		<-done
	}
}
