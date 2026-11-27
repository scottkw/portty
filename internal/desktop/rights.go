package desktop

import (
	"context"
	"errors"
)

// rights.go: the sharer's per-person switches. A guest starts view-only and the
// sharer turns control and clipboard on and off while the guest is connected.
// The daemon enforces them: pressAllowed and the clipboard handlers read the
// live bits through rightsOf, so a modified guest page cannot bypass a switch.

// Errors of the per-viewer calls (SetRights here, CloseViewer in revoke.go).
var (
	ErrOwnViewer = errors.New("desktop: an own-account viewer's rights cannot be changed")
	ErrNoViewer  = errors.New("desktop: no such viewer")
)

// rightsOf is v's live rights: the atomic SetRights stores, not the construction-time record.
func (m *Manager) rightsOf(v *viewer) Rights { return Rights(v.rights.Load()) }

// setRightsBits stores r as v's rights (record and atomic).
func (v *viewer) setRightsBits(r Rights) { v.rights.Store(uint32(r)) }

// SetRights sets a guest's control and clipboard rights, each independent of the
// other. Surface is never granted to a guest. Turning control off lets go of
// everything the guest holds, from the input goroutine so a press that was
// already being handled cannot leave a button down. The admission keeps the new bits, so a reconnect from the same node
// comes back with them, and the guest is told with a rights message.
func (m *Manager) SetRights(id ViewerID, control, clipboard bool) error {
	m.vmu.Lock()
	v := m.viewers[id]
	m.vmu.Unlock()
	switch {
	case v == nil:
		return ErrNoViewer
	case v.Mode != ModeGuest:
		return ErrOwnViewer
	}
	var r Rights
	if control {
		r |= RightsControl
	}
	if clipboard {
		r |= RightsClipboard
	}
	m.rmu.Lock()
	old := Rights(v.rights.Swap(uint32(r)))
	if v.gs != nil {
		v.gs.SetGrants(AdmissionKey{UserID: v.Identity.UserID, StableID: v.Identity.StableID}, r)
	}
	m.rmu.Unlock()
	if old&RightsControl != 0 && r&RightsControl == 0 {
		m.arb.forget(v.ID)
		m.queueReleaseHeld(v, "grant_revoked")
	}
	m.syncTap() // control on starts the tap, the last controller off stops it
	m.kickLease()
	_ = send(context.Background(), v.ws, msgRights{Type: typeRights, Rights: rightsMap(r)})
	return nil
}

// queueReleaseHeld asks the input goroutine to let go of everything v holds,
// without ending v's input life. It runs there, after whatever press it is
// handling now, so a press that passed the right check just before the change
// is released too.
func (m *Manager) queueReleaseHeld(v *viewer, reason string) {
	m.in.mu.Lock()
	m.in.rels = append(m.in.rels, release{rv: v, why: reason})
	m.in.mu.Unlock()
	m.wakeInput()
}
