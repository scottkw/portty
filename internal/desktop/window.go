package desktop

// window.go: what a window share permits. CGEvent posts are global, so without
// a gate a viewer typing into a shared window could type into an app it cannot
// see. A press posts only while a live Accessibility read says the shared
// window's app has focus and its focused window is the shared one; otherwise
// the press starts a raise (off the post path) and is dropped. Presses are also
// confined to the window: a click outside the freshly read frame, or on a point
// another window covers, is dropped, and moves are clamped to the frame.

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

const (
	// frameTolerance is how far (in points) two reads of one window's frame may
	// differ and still be the same window.
	frameTolerance = 2
	// raiseTimeout is how long a raise may run before viewers are told it failed.
	raiseTimeout = 300 * time.Millisecond
	// frameRefresh is the cadence at which the frame is re-read while viewers can send input.
	frameRefresh = 200 * time.Millisecond
)

var errRaisePanic = errors.New("desktop: window raise panicked")

// WindowBridge is what the window gate reads and does through the native layer;
// screencap.WindowBridge is the real one and tests inject a fake. Every real
// method refuses under tests, so no test touches Accessibility on the host.
type WindowBridge interface {
	// Focus reads, live, the focused application's pid and its focused window's
	// frame; ok is false when there is no focused window.
	Focus() (pid int32, x, y, w, h float64, ok bool, err error)
	// KeyFocus reads, live, the owner pid of the element that actually holds
	// keyboard focus and the frame of that element's window; ok is false when no
	// element or window is found. It differs from Focus when a non-activating
	// panel (Spotlight, Alfred, Raycast) takes key focus without changing the
	// focused application (Phase 20 review M-05).
	KeyFocus() (pid int32, x, y, w, h float64, ok bool, err error)
	// WindowInfo reads a window's owner and frame and whether it is on screen.
	WindowInfo(id uint32) (pid int32, x, y, w, h float64, onScreen bool, err error)
	// Minimized reports whether the window with this frame (title breaks a tie) is minimized.
	Minimized(pid int32, x, y, w, h float64, title string) (bool, error)
	// Raise brings the window to the front and returns the rung that confirmed it.
	Raise(pid int32, x, y, w, h float64, title string) (rung int, err error)
	// TopmostAt returns the topmost on-screen window at a global point.
	TopmostAt(x, y float64) (screencap.TopWindow, error)
}

// rect is a window frame in global points.
type rect struct{ x, y, w, h float64 }

// near reports whether two frames are the same window within frameTolerance.
func (r rect) near(o rect) bool {
	return math.Abs(r.x-o.x) <= frameTolerance && math.Abs(r.y-o.y) <= frameTolerance &&
		math.Abs(r.w-o.w) <= frameTolerance && math.Abs(r.h-o.h) <= frameTolerance
}

func (r rect) contains(x, y float64) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

func (r rect) target() inputTarget { return inputTarget{x: r.x, y: r.y, w: r.w, h: r.h, ok: true} }

// windowState is the window gate's state; mu is a leaf lock (no bridge call under it).
type windowState struct {
	mu      sync.Mutex
	frame   rect   // the last on-screen read of the shared window
	have    bool   // frame is valid for the current window
	raising bool   // a raise is running; one at a time
	gen     uint64 // counts raises and surface changes, so a late raise result is ignored

	done chan struct{} // closed when windowLoop has ended; set by Start under Manager.mu
}

// windowShared reports whether the share shows a window.
func (m *Manager) windowShared() bool { return m.surfaceSel().Kind == screencap.SurfaceWindow }

func (m *Manager) cachedFrame() (rect, bool) {
	m.win.mu.Lock()
	defer m.win.mu.Unlock()
	return m.win.frame, m.win.have
}

// windowFrame returns the shared window's last read frame.
func (m *Manager) windowFrame() (x, y, w, h float64, ok bool) {
	f, ok := m.cachedFrame()
	return f.x, f.y, f.w, f.h, ok
}

// resetWindow forgets the previous surface's frame and state, and disowns a raise in flight.
func (m *Manager) resetWindow() {
	m.win.mu.Lock()
	m.win.have = false
	m.win.gen++
	m.win.mu.Unlock()
	m.setControlWindow("")
}

// clearWindowState clears the window control value when it is state.
func (m *Manager) clearWindowState(state string) {
	if _, cur := m.ControlState(); cur == state {
		m.setControlWindow("")
	}
}

// readWindow reads the shared window's frame now and remembers it when the
// window is on screen (an off-screen frame is not where a click could land).
// An on-screen read ends the minimized state.
func (m *Manager) readWindow(wb WindowBridge, sel screencap.Surface, id uint32) (f rect, onScreen bool, err error) {
	_, x, y, w, h, onScreen, err := wb.WindowInfo(id)
	if err != nil {
		return rect{}, false, err
	}
	f = rect{x, y, w, h}
	if onScreen {
		m.win.mu.Lock()
		if m.surfaceSel() == sel { // the surface may have changed during the read
			m.win.frame, m.win.have = f, true
		}
		m.win.mu.Unlock()
		m.clearWindowState(WindowMinimized)
	}
	return f, onScreen, nil
}

// noteOffScreen tells viewers the window is minimized when it is, and returns
// the frame to raise it by. The frame from the last on-screen read is used: a
// minimized window's own bounds are not reliable.
func (m *Manager) noteOffScreen(wb WindowBridge, info screencap.SurfaceInfo, fresh rect) rect {
	f := fresh
	if c, ok := m.cachedFrame(); ok {
		f = c
	}
	if min, err := wb.Minimized(info.PID, f.x, f.y, f.w, f.h, info.Title); err == nil && min {
		m.setControlWindow(WindowMinimized)
	}
	return f
}

// windowFocused reports whether, right now, the shared window's app has focus
// and its focused window is the shared one.
func windowFocused(wb WindowBridge, pid int32, f rect) bool {
	fpid, x, y, w, h, ok, err := wb.Focus()
	return err == nil && ok && fpid == pid && rect{x, y, w, h}.near(f)
}

// keyFocused reports whether, right now, the element holding keyboard focus is
// in the shared window's app and window. Anything else, an error included, is
// no: a key must never land in a panel the viewer cannot see.
func keyFocused(wb WindowBridge, pid int32, f rect) bool {
	kpid, x, y, w, h, ok, err := wb.KeyFocus()
	return err == nil && ok && kpid == pid && rect{x, y, w, h}.near(f)
}

// windowAllows gates a press, move, wheel or text message while a window is
// shared; pressAllowed calls it only then. Reasons: window_unavailable (no
// bridge or an unreadable window: fail closed), raising (focus was elsewhere or
// the window was off screen: a raise runs and this press is dropped), outside
// (the point is not inside the window), covered (another window is on top at
// the point; the shared app's own pop-ups do not count, see sameAppPopup) and,
// for keys and text, window_key_focus (the element holding keyboard focus is not
// the shared window; no raise starts, a panel on top is the user's own action).
func (m *Manager) windowAllows(msg inputMsg) (bool, string) {
	sel, info := m.currentSurface()
	if sel.Kind != screencap.SurfaceWindow {
		return true, ""
	}
	wb := m.d.Window
	if wb == nil {
		return false, "window_unavailable"
	}
	if msg.T == inMove || msg.T == inRMove {
		// A move needs only a frame to map onto and clamp to; presses refresh it.
		if _, ok := m.cachedFrame(); ok {
			return true, ""
		}
		if _, on, err := m.readWindow(wb, sel, info.ID); err != nil || !on {
			return false, "window_unavailable"
		}
		return true, ""
	}
	m.win.mu.Lock()
	raising := m.win.raising
	m.win.mu.Unlock()
	if raising {
		return false, "raising"
	}
	old, haveOld := m.cachedFrame() // what moves and this press were mapped with
	fresh, on, err := m.readWindow(wb, sel, info.ID)
	if err != nil {
		return false, "window_unavailable"
	}
	if !on {
		m.startRaise(wb, info, m.noteOffScreen(wb, info, fresh))
		return false, "raising"
	}
	if !windowFocused(wb, info.PID, fresh) {
		m.startRaise(wb, info, fresh)
		return false, "raising"
	}
	var px, py float64
	switch msg.T {
	case inBtn:
		if haveOld {
			if ox, oy := mapPoint(old.target(), msg.X, msg.Y); !fresh.contains(ox, oy) {
				return false, "outside"
			}
		}
		px, py = mapPoint(fresh.target(), msg.X, msg.Y)
	case inWheel:
		// A scroll lands where the pointer is, not where the viewer points.
		cx, cy, err := m.d.Input.CursorLocation()
		if err != nil {
			return false, "window_unavailable"
		}
		if !fresh.contains(cx, cy) {
			return false, "outside"
		}
		px, py = cx, cy
	default:
		// Keys and text go to whoever holds keyboard focus. The focused app and
		// window were just confirmed, but a non-activating panel takes key focus
		// without changing them, so key focus is read too. Releases never come
		// here, so a held key is still let go.
		if !keyFocused(wb, info.PID, fresh) {
			return false, "window_key_focus"
		}
		return true, ""
	}
	top, err := wb.TopmostAt(px, py)
	if err != nil {
		return false, "window_unavailable"
	}
	if top.ID != info.ID && !sameAppPopup(top, info.PID) {
		return false, "covered"
	}
	return true, ""
}

// popUpMenuLayer is kCGPopUpMenuWindowLevel: menus, pop-up menus and their
// popovers sit at this layer or above.
const popUpMenuLayer = 101

// sameAppPopup reports whether a window on top of the shared one is the shared
// app's own menu or pop-up (layer 101 and up). Nothing else is carved out
// (T-20-21): the app's sheets, floating and modal panels (layers 0, 3, 8),
// utility windows and other document windows, negative layers and every other
// app's windows still cover, since a single-window capture does not show them.
func sameAppPopup(top screencap.TopWindow, pid int32) bool {
	return top.PID == pid && top.Layer >= popUpMenuLayer
}

// startRaise raises the shared window on its own goroutine, one at a time (a
// hung app must never stall the input path). A raise that fails, or has not
// finished within raiseTimeout, tells viewers raise_failed; the next press
// retries once it has ended. A success clears raise_failed.
func (m *Manager) startRaise(wb WindowBridge, info screencap.SurfaceInfo, f rect) {
	m.win.mu.Lock()
	if m.win.raising {
		m.win.mu.Unlock()
		return
	}
	m.win.raising = true
	m.win.gen++
	gen := m.win.gen
	m.win.mu.Unlock()
	timer := time.AfterFunc(raiseTimeout, func() {
		defer recoverLog()
		m.win.mu.Lock()
		defer m.win.mu.Unlock()
		if m.win.raising && m.win.gen == gen {
			m.setControlWindow(WindowRaiseFailed)
		}
	})
	go func() {
		defer recoverLog()
		rung, err := safeRaise(wb, info, f)
		timer.Stop()
		m.win.mu.Lock()
		defer m.win.mu.Unlock()
		m.win.raising = false
		if m.win.gen != gen { // the surface changed meanwhile
			return
		}
		if err != nil {
			slog.Warn("desktop: window raise failed")
			m.setControlWindow(WindowRaiseFailed)
			return
		}
		slog.Debug("desktop: window raised", "rung", screencap.RungName(rung))
		m.clearWindowState(WindowRaiseFailed)
	}()
}

// safeRaise calls Raise and turns a panic into an error, so the raising flag is always released.
func safeRaise(wb WindowBridge, info screencap.SurfaceInfo, f rect) (rung int, err error) {
	defer func() {
		if r := recover(); r != nil {
			rung, err = 0, errRaisePanic
		}
	}()
	return wb.Raise(info.PID, f.x, f.y, f.w, f.h, info.Title)
}

// refreshWindow re-reads the frame of the shared window, best effort (the
// release path uses it to clamp to where the window is now).
func (m *Manager) refreshWindow() {
	if !m.windowShared() || m.d.Window == nil {
		return
	}
	sel, info := m.currentSurface()
	_, _, _ = m.readWindow(m.d.Window, sel, info.ID)
}

// clampToWindow keeps a point inside the shared window's last read frame; with
// no window shared or no frame known it returns the point unchanged.
func (m *Manager) clampToWindow(x, y float64) (float64, float64) {
	if !m.windowShared() {
		return x, y
	}
	f, ok := m.cachedFrame()
	if !ok {
		return x, y
	}
	return math.Max(f.x, math.Min(f.x+f.w-1, x)), math.Max(f.y, math.Min(f.y+f.h-1, y))
}

// windowLoop re-reads the shared window's frame about 5 times a second while a
// window is shared and some viewer has an input channel open, so moves map to
// where the window is and a minimized window is reported without a press.
func (m *Manager) windowLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	t := time.NewTicker(frameRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.windowTick()
		}
	}
}

// windowTick is one refresh pass; a panic costs the pass, never the loop.
func (m *Manager) windowTick() {
	defer recoverLog()
	wb := m.d.Window
	if wb == nil || !m.windowShared() || len(m.inputChannels()) == 0 {
		return
	}
	sel, info := m.currentSurface()
	if fresh, on, err := m.readWindow(wb, sel, info.ID); err == nil && !on {
		m.noteOffScreen(wb, info, fresh)
	}
}
