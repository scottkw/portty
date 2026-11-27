package desktop

// surface.go: which display or window the share shows. A viewer with
// RightsSurface asks for the list ({type:surfaces}) and picks one
// ({type:select, kind, id}); the running capture switches in place (no new
// offer), the input epoch is bumped before the switch starts so no stale click
// lands on the new picture, and every viewer is told with {type:surface}. The
// choice lives only in memory: the last viewer leaving resets it to the main
// display and nothing is written to desktop.json.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// Message types of the surface protocol.
const (
	typeSurfaces     = "surfaces"
	typeSurface      = "surface"
	typeSelect       = "select"
	typeSelectFailed = "select_failed"
)

// Codes of select_failed, and the error of a surfaces reply that has no list.
const (
	selectSwitchFailed = "switch_failed"
	selectNotFound     = "not_found"
	selectBusy         = "busy"
	selectForbidden    = "forbidden"
	listFailed         = "list_failed"
)

// Why a share fell back to the main display (the reason of a surface message).
const (
	ReasonWindowClosed = "window_closed"
	ReasonDisplayGone  = "display_gone"
)

// surfacePoll is how often a shared window or non-main display is checked.
const surfacePoll = time.Second

// listTimeout bounds one surface list; the native request has its own 15 s
// ceiling, this is what a viewer waits at most.
const listTimeout = 5 * time.Second

var errNoSurfaceList = errors.New("desktop: no surface list available")

type surfaceDisplay struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Main bool   `json:"main"`
}

type surfaceWindow struct {
	ID    uint32 `json:"id"`
	App   string `json:"app"`
	Title string `json:"title"`
	Name  string `json:"name"`
}

// msgSurfaces answers a surfaces request: the lists, or only an error.
type msgSurfaces struct {
	Type     string           `json:"type"`
	Displays []surfaceDisplay `json:"displays"`
	Windows  []surfaceWindow  `json:"windows"`
	Error    string           `json:"error,omitempty"`
}

// MarshalJSON leaves the lists out of an error reply, as the golden file has it.
func (s msgSurfaces) MarshalJSON() ([]byte, error) {
	if s.Error != "" {
		return json.Marshal(struct {
			Type  string `json:"type"`
			Error string `json:"error"`
		}{s.Type, s.Error})
	}
	type plain msgSurfaces // no MarshalJSON, so no recursion
	return json.Marshal(plain(s))
}

// msgSurface tells every viewer what is shared now; e is the epoch input must
// carry from here on and reason says why a share fell back ("" for a pick).
type msgSurface struct {
	Type   string `json:"type"`
	E      uint32 `json:"e"`
	Kind   string `json:"kind"`
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Reason string `json:"reason"`
}

type msgSelectFailed struct {
	Type string `json:"type"`
	Code string `json:"code"`
}

// surfaceState is what is shared. mu is a leaf lock: nothing is called under it.
type surfaceState struct {
	mu        sync.Mutex
	sel       screencap.Surface     // what the source is told; the main display is {display, 0}
	info      screencap.SurfaceInfo // its description; a zero Name means the main display, not read yet
	switching bool                  // a switch (a pick or a fallback) is in flight

	done chan struct{} // closed when surfaceLoop has ended; set by Start under Manager.mu
}

// mainInfo describes the main display now.
func (m *Manager) mainInfo() screencap.SurfaceInfo {
	info := screencap.SurfaceInfo{Kind: screencap.SurfaceDisplay, ID: m.d.MainDisplayID(), Name: "Main Display", Main: true, OnScreen: true}
	if x, y, w, h, err := m.d.DisplayBounds(0); err == nil {
		info.X, info.Y, info.W, info.H = x, y, w, h
	}
	return info
}

// currentSurface returns what is shared. The main display is read fresh, so a
// resolution change shows in the next surface message.
func (m *Manager) currentSurface() (screencap.Surface, screencap.SurfaceInfo) {
	m.surf.mu.Lock()
	sel, info := m.surf.sel, m.surf.info
	m.surf.mu.Unlock()
	if info.Name == "" {
		info = m.mainInfo()
	}
	return sel, info
}

// CurrentSurface describes what the share shows now (the host GUI and the
// window gate of a later plan read it).
func (m *Manager) CurrentSurface() screencap.SurfaceInfo {
	_, info := m.currentSurface()
	return info
}

func (m *Manager) setSurface(sel screencap.Surface, info screencap.SurfaceInfo) {
	if sel == (screencap.Surface{}) {
		info = screencap.SurfaceInfo{} // the main display is read fresh, never cached
	}
	m.surf.mu.Lock()
	changed := m.surf.sel != sel
	m.surf.sel, m.surf.info = sel, info
	m.surf.mu.Unlock()
	if changed {
		m.resetWindow()
	}
}

// surfaceSel is what the source is told to capture.
func (m *Manager) surfaceSel() screencap.Surface {
	m.surf.mu.Lock()
	defer m.surf.mu.Unlock()
	return m.surf.sel
}

// resetSurface returns to the main display when the last viewer has left, and
// bumps the epoch so nothing stamped for the old surface can land on the next.
func (m *Manager) resetSurface() {
	m.setSurface(screencap.Surface{}, screencap.SurfaceInfo{})
	m.bumpEpoch()
}

func kindName(k screencap.SurfaceKind) string {
	if k == screencap.SurfaceWindow {
		return "window"
	}
	return "display"
}

// surfaceMsg is the message that tells viewers what is shared at this epoch.
func (m *Manager) surfaceMsg(reason string) msgSurface {
	_, info := m.currentSurface()
	return msgSurface{
		Type: typeSurface, E: m.currentEpoch(), Kind: kindName(info.Kind), ID: info.ID, Name: info.Name,
		W: int(math.Round(info.W)), H: int(math.Round(info.H)), Reason: reason,
	}
}

// target is the surface input maps onto, read fresh for every press and move:
// a display by its bounds (id 0 is the main display), a window by the frame the
// window gate last read (none until a press or move has read one; the gate then
// decides). An unreadable display leaves the geometry unknown.
func (m *Manager) target() inputTarget {
	m.surf.mu.Lock()
	sel := m.surf.sel
	m.surf.mu.Unlock()
	t := inputTarget{epoch: m.currentEpoch()}
	if sel.Kind == screencap.SurfaceWindow {
		t.x, t.y, t.w, t.h, t.ok = m.windowFrame()
		t.ok = t.ok && t.w >= 1 && t.h >= 1
		return t
	}
	x, y, w, h, err := m.d.DisplayBounds(sel.ID)
	if err != nil || w < 1 || h < 1 {
		return t
	}
	t.x, t.y, t.w, t.h, t.ok = x, y, w, h, true
	return t
}

// listSurfaces asks the host for every display and window.
func (m *Manager) listSurfaces(ctx context.Context) ([]screencap.SurfaceInfo, error) {
	if m.d.ListSurfaces == nil {
		return nil, errNoSurfaceList
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	return m.d.ListSurfaces(ctx)
}

// handleSurfaces answers a surfaces request with every display and every
// window that is on screen. Window titles go only to a viewer that may pick.
func (m *Manager) handleSurfaces(ctx context.Context, v *viewer) {
	reply := msgSurfaces{Type: typeSurfaces, Displays: []surfaceDisplay{}, Windows: []surfaceWindow{}}
	if m.rightsOf(v)&RightsSurface == 0 {
		_ = send(ctx, v.ws, msgSurfaces{Type: typeSurfaces, Error: selectForbidden})
		return
	}
	list, err := m.listSurfaces(ctx)
	if err != nil {
		slog.Warn("desktop: surface list failed")
		_ = send(ctx, v.ws, msgSurfaces{Type: typeSurfaces, Error: listFailed})
		return
	}
	for _, s := range list {
		switch {
		case s.Kind == screencap.SurfaceDisplay:
			reply.Displays = append(reply.Displays, surfaceDisplay{ID: s.ID, Name: s.Name, W: int(math.Round(s.W)), H: int(math.Round(s.H)), Main: s.Main})
		case s.OnScreen:
			reply.Windows = append(reply.Windows, surfaceWindow{ID: s.ID, App: s.App, Title: s.Title, Name: s.Name})
		}
	}
	_ = send(ctx, v.ws, reply)
}

// beginSwitch claims the one switch slot; false means one is already running.
func (m *Manager) beginSwitch() bool {
	m.surf.mu.Lock()
	defer m.surf.mu.Unlock()
	if m.surf.switching {
		return false
	}
	m.surf.switching = true
	return true
}

func (m *Manager) endSwitch() {
	m.surf.mu.Lock()
	m.surf.switching = false
	m.surf.mu.Unlock()
}

func failSelect(ctx context.Context, v *viewer, code string) {
	_ = send(ctx, v.ws, msgSelectFailed{Type: typeSelectFailed, Code: code})
}

// handleSelect switches the share to the display or window the viewer picked.
func (m *Manager) handleSelect(ctx context.Context, v *viewer, kind string, id uint32) {
	if m.rightsOf(v)&RightsSurface == 0 {
		failSelect(ctx, v, selectForbidden)
		return
	}
	if !m.beginSwitch() {
		failSelect(ctx, v, selectBusy)
		return
	}
	defer m.endSwitch()

	want := screencap.SurfaceDisplay
	switch kind {
	case "display":
	case "window":
		want = screencap.SurfaceWindow
	default:
		failSelect(ctx, v, selectNotFound)
		return
	}
	list, err := m.listSurfaces(ctx)
	if err != nil {
		slog.Warn("desktop: surface list failed")
		failSelect(ctx, v, selectSwitchFailed)
		return
	}
	var info screencap.SurfaceInfo
	found := false
	for _, s := range list {
		// Display id 0 always means the main display.
		if s.Kind == want && (s.ID == id || (want == screencap.SurfaceDisplay && id == 0 && s.Main)) {
			info, found = s, true
			break
		}
	}
	if !found {
		failSelect(ctx, v, selectNotFound)
		return
	}
	sel := screencap.Surface{Kind: want, ID: info.ID}
	if info.Main {
		sel.ID = 0
	}
	if err := m.applySurface(ctx, sel, info, ""); err != nil {
		failSelect(ctx, v, selectSwitchFailed)
	}
}

// applySurface switches the share to sel and tells every viewer. The epoch is
// bumped first, so input already in flight carries a stale one and drops from
// here on; held input is let go; then a running source is switched (a source
// that is not running only records the choice). A failed switch keeps the old
// surface and still tells every viewer, with the new epoch, so nobody stays
// paused. A non-empty reason marks a fallback: the old surface is gone, so the
// new one is recorded even when the switch fails, and the source is restarted
// on it (M-01).
func (m *Manager) applySurface(ctx context.Context, sel screencap.Surface, info screencap.SurfaceInfo, reason string) error {
	m.bumpEpoch()
	m.releaseHeldAll("surface")
	m.mu.Lock()
	var err error
	if m.src != nil {
		err = m.src.SelectSurface(sel)
	}
	if err == nil || reason != "" {
		m.setSurface(sel, info)
	}
	m.mu.Unlock()
	switch {
	case err != nil && reason != "":
		// The source is still on the vanished surface: restart it on the one
		// just recorded, so the picture and the input mapping agree.
		slog.Warn("desktop: fallback switch failed, restarting the source", "kind", kindName(sel.Kind), "err", err)
		_ = m.restartSource() // a failure was classified and delays the next try
	case err != nil:
		slog.Warn("desktop: surface switch failed", "kind", kindName(sel.Kind), "err", err)
	default:
		slog.Info("desktop: surface switched", "kind", kindName(sel.Kind), "viewers", m.Viewers())
	}
	m.broadcast(ctx, m.surfaceMsg(reason))
	return err
}

// flushInput waits (at most flushWait) until the input goroutine has done
// everything queued before this call.
func (m *Manager) flushInput() {
	flush := make(chan struct{})
	m.in.mu.Lock()
	running := m.in.done != nil
	if running {
		m.in.rels = append(m.in.rels, release{flush: flush})
	}
	m.in.mu.Unlock()
	if !running {
		return
	}
	m.wakeInput()
	select {
	case <-flush:
	case <-time.After(flushWait):
		slog.Warn("desktop: queued input not posted in time")
	}
}

// releaseHeldAll lets go of everything every viewer holds, without ending any
// viewer's input life (they keep sending, with the new epoch). The first flush
// lets an item that passed the epoch gate just before the bump finish and
// record what it holds, so the snapshot after it misses nothing.
func (m *Manager) releaseHeldAll(reason string) {
	m.flushInput()
	m.vmu.Lock()
	vs := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vs = append(vs, v)
	}
	m.vmu.Unlock()
	n := 0
	m.in.mu.Lock()
	for _, v := range vs {
		rels := m.takeHeldLocked(v.in)
		m.in.rels = append(m.in.rels, rels...)
		n += len(rels)
	}
	m.in.mu.Unlock()
	if n == 0 {
		return
	}
	slog.Debug("desktop: input released", "reason", reason, "buttons", n)
	m.wakeInput()
	m.flushInput()
}

// anyButtonHeld reports whether any viewer holds a mouse button.
func (m *Manager) anyButtonHeld() bool {
	m.vmu.Lock()
	vs := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vs = append(vs, v)
	}
	m.vmu.Unlock()
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	for _, v := range vs {
		if len(v.in.held) > 0 {
			return true
		}
	}
	return false
}

// surfaceLoop checks, once a second while viewers watch a window or a display
// other than the main one, that the shared surface still exists and still has
// its size. A panic costs one pass, never the loop or the stream.
func (m *Manager) surfaceLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	t := time.NewTicker(surfacePoll)
	defer t.Stop()
	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.surfaceTick(ctx, &failing)
		}
	}
}

// sameSurface finds the shared surface's entry in a fresh list.
func sameSurface(list []screencap.SurfaceInfo, sel screencap.Surface, info screencap.SurfaceInfo) (screencap.SurfaceInfo, bool) {
	for _, s := range list {
		if s.Kind == sel.Kind && s.ID == info.ID {
			return s, true
		}
	}
	return screencap.SurfaceInfo{}, false
}

func (m *Manager) surfaceTick(ctx context.Context, failing *bool) {
	defer recoverLog()
	sel, info := m.currentSurface()
	if m.Viewers() == 0 || sel == (screencap.Surface{}) {
		return
	}
	list, err := m.listSurfaces(ctx)
	if err != nil {
		if !*failing {
			slog.Warn("desktop: surface poll failed")
		}
		*failing = true
		return
	}
	*failing = false
	cur, found := sameSurface(list, sel, info)
	var reason string
	switch {
	case found && (math.Abs(cur.W-info.W) >= 1 || math.Abs(cur.H-info.H) >= 1):
		// Resized: the same surface at its new size. Applying it releases held
		// input, so it waits while a button is held (a drag-resize of the shared
		// window); a later pass applies it once the button is up (M-03).
		if m.anyButtonHeld() {
			return
		}
	case found:
		// A move or a minimize changes only the recorded frame, never the epoch.
		if cur != info && m.surfaceSel() == sel {
			m.setSurface(sel, cur)
		}
		return
	case sel.Kind == screencap.SurfaceWindow:
		reason = ReasonWindowClosed
	default:
		reason = ReasonDisplayGone
	}
	// A pick in flight wins; the next pass looks again.
	if !m.beginSwitch() {
		return
	}
	defer m.endSwitch()
	if m.surfaceSel() != sel {
		return
	}
	if reason == "" {
		_ = m.applySurface(ctx, sel, cur, "")
		return
	}
	// The share falls back to the main display and keeps running.
	main := m.mainInfo()
	for _, s := range list {
		if s.Kind == screencap.SurfaceDisplay && s.Main {
			main = s
		}
	}
	_ = m.applySurface(ctx, screencap.Surface{}, main, reason)
}
