package desktop

// window.go: what a window share permits. CGEvent posts are global, so without
// a gate a viewer typing into a shared window could type into an app it cannot
// see. A press posts only while a live Accessibility read says the shared
// window's app has focus and its focused window is the shared one; otherwise
// the press starts a raise (off the post path) and is dropped.

import (
	"log/slog"
	"math"
	"sync"

	"github.com/scottkw/portty/internal/screencap"
)

// frameTolerance is how far (in points) two reads of one window's frame may
// differ and still be the same window.
const frameTolerance = 2

// WindowBridge is what the window gate reads and does through the native layer;
// screencap.WindowBridge is the real one and tests inject a fake. Every real
// method refuses under tests, so no test touches Accessibility on the host.
type WindowBridge interface {
	// Focus reads, live, the focused application's pid and its focused window's
	// frame; ok is false when there is no focused window.
	Focus() (pid int32, x, y, w, h float64, ok bool, err error)
	// WindowInfo reads a window's owner and frame and whether it is on screen.
	WindowInfo(id uint32) (pid int32, x, y, w, h float64, onScreen bool, err error)
	// Minimized reports whether the window with this frame (title breaks a tie) is minimized.
	Minimized(pid int32, x, y, w, h float64, title string) (bool, error)
	// Raise brings the window to the front and returns the rung that confirmed it.
	Raise(pid int32, x, y, w, h float64, title string) (rung int, err error)
	// TopmostAt returns the ID of the topmost on-screen window at a global point.
	TopmostAt(x, y float64) (windowID uint32, err error)
}

// rect is a window frame in global points.
type rect struct{ x, y, w, h float64 }

// near reports whether two frames are the same window within frameTolerance.
func (r rect) near(o rect) bool {
	return math.Abs(r.x-o.x) <= frameTolerance && math.Abs(r.y-o.y) <= frameTolerance &&
		math.Abs(r.w-o.w) <= frameTolerance && math.Abs(r.h-o.h) <= frameTolerance
}

// windowState is the window gate's state; mu is a leaf lock (no bridge call under it).
type windowState struct {
	mu      sync.Mutex
	frame   rect // the last on-screen read of the shared window
	have    bool // frame is valid for the current window
	raising bool // a raise is running; one at a time
}

// windowShared reports whether the share shows a window.
func (m *Manager) windowShared() bool { return m.surfaceSel().Kind == screencap.SurfaceWindow }

// windowFrame returns the shared window's last read frame.
func (m *Manager) windowFrame() (x, y, w, h float64, ok bool) {
	m.win.mu.Lock()
	defer m.win.mu.Unlock()
	f := m.win.frame
	return f.x, f.y, f.w, f.h, m.win.have
}

// resetWindow forgets the frame of the previous surface.
func (m *Manager) resetWindow() {
	m.win.mu.Lock()
	m.win.have = false
	m.win.mu.Unlock()
}

// readWindow reads the shared window's frame now and remembers it when the
// window is on screen (an off-screen frame is not where a click could land).
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
	}
	return f, onScreen, nil
}

// windowFocused reports whether, right now, the shared window's app has focus
// and its focused window is the shared one.
func windowFocused(wb WindowBridge, pid int32, f rect) bool {
	fpid, x, y, w, h, ok, err := wb.Focus()
	return err == nil && ok && fpid == pid && rect{x, y, w, h}.near(f)
}

// windowAllows gates a press, move, wheel or text message of a viewer while a
// window is shared; it is called from pressAllowed only then. The reasons it
// gives: window_unavailable (no bridge or an unreadable window: fail closed),
// raising (focus was elsewhere, a raise runs or has just started).
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
		// A move needs only a frame to map onto; presses refresh it.
		if _, _, _, _, ok := m.windowFrame(); ok {
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
	fresh, on, err := m.readWindow(wb, sel, info.ID)
	if err != nil {
		return false, "window_unavailable"
	}
	if !on || !windowFocused(wb, info.PID, fresh) {
		m.startRaise(wb, info, fresh)
		return false, "raising"
	}
	return true, ""
}

// startRaise raises the shared window on its own goroutine, one at a time (a
// hung app must never stall the input path).
func (m *Manager) startRaise(wb WindowBridge, info screencap.SurfaceInfo, f rect) {
	m.win.mu.Lock()
	if m.win.raising {
		m.win.mu.Unlock()
		return
	}
	m.win.raising = true
	m.win.mu.Unlock()
	go func() {
		defer recoverLog()
		rung, err := wb.Raise(info.PID, f.x, f.y, f.w, f.h, info.Title)
		m.win.mu.Lock()
		m.win.raising = false
		m.win.mu.Unlock()
		if err != nil {
			slog.Warn("desktop: window raise failed")
			return
		}
		slog.Debug("desktop: window raised", "rung", screencap.RungName(rung))
	}()
}
