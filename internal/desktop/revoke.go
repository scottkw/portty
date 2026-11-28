package desktop

import (
	"context"
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
// person is deny-listed for the rest of the share and cut on every device (the
// person is the unit, as the sharer's copy says); an own-account device is
// closed 1001 disconnected and may reconnect through always-available. Calling
// it twice, or while the viewer is leaving, is safe.
func (m *Manager) CloseViewer(id ViewerID) error {
	v := m.lookupViewer(id)
	if v == nil {
		return ErrNoViewer
	}
	if v.Mode == ModeGuest {
		// Deny first and unconditionally: a row already cut another way (a
		// replace, a media cut) would skip anything run inside cutViewer.
		if v.gs != nil {
			v.gs.RevokeUser(v.Identity.UserID)
		}
		m.closeGuestUser(v.Identity.UserID)
	} else {
		m.cutViewer(v, websocket.StatusGoingAway, "disconnected")
	}
	return nil
}

// DenyGuest answers the waiting request reqID no. Deny is for the person
// (GuestShare.Deny deny-lists the user), so the person's devices already
// watching are cut too, 4432 revoked: the deny-list and the live connections
// agree, and a watching device is not left to fail at its next reconnect.
func (m *Manager) DenyGuest(gs *GuestShare, reqID string) error {
	var uid int64
	for _, p := range gs.Pending() {
		if p.ID == reqID {
			uid = p.Ident.UserID
		}
	}
	if err := gs.Deny(reqID); err != nil {
		return err
	}
	m.closeGuestUser(uid)
	return nil
}

// closeGuestUser cuts every live guest viewer of userID, 4432 revoked. The
// caller has deny-listed the user first, so a device still joining fails its
// re-check in serveAdmitted.
func (m *Manager) closeGuestUser(userID int64) {
	m.vmu.Lock()
	var vs []*viewer
	for _, v := range m.viewers {
		if v.Mode == ModeGuest && v.Identity.UserID == userID {
			vs = append(vs, v)
		}
	}
	m.vmu.Unlock()
	for _, v := range vs {
		m.cutViewer(v, CloseGuestRevoked, guestReasonRevoked)
	}
}

// CloseViewerWith is CloseViewer with the close code and reason chosen by the
// caller, and no deny-list entry.
func (m *Manager) CloseViewerWith(id ViewerID, code websocket.StatusCode, reason string) error {
	v := m.lookupViewer(id)
	if v == nil {
		return ErrNoViewer
	}
	m.cutViewer(v, code, reason)
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
			m.cutViewer(v, code, reason)
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
// after), then the arbiter forgets it, then media and finally the socket.
func (m *Manager) cutViewer(v *viewer, code websocket.StatusCode, reason string) {
	if !v.cut.CompareAndSwap(false, true) {
		return
	}
	v.setRightsBits(RightsViewOnly)
	m.endViewerInput(v, "revoked")
	m.flushInput()
	m.arb.forget(v.ID)
	if v.pc != nil {
		_ = v.pc.Close()
	}
	m.syncTap()
	go closeSocket(v.ws, v.drop, code, reason)
}

// closeSocket closes c with code and reason, and drops it without the handshake
// once closeCeiling has passed. drop ends the viewer's blocked read, which
// closes c at once: in coder/websocket v1.8.15 CloseNow after Close has started
// only waits, and Close waits about 5 s for the lock that read holds.
func closeSocket(c *websocket.Conn, drop context.CancelFunc, code websocket.StatusCode, reason string) {
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
		drop()
		_ = c.CloseNow()
		<-done
	}
}
