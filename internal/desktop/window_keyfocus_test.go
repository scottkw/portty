package desktop_test

import (
	"errors"
	"testing"

	"github.com/scottkw/portty/internal/screencap"
)

// M-05 (Phase 20 review): a non-activating panel (Spotlight, Alfred, Raycast)
// can hold keyboard focus while the focused application stays the shared
// window's, so keys that pass the focused-app check would land in a panel the
// viewer cannot see. The key gate also reads the element that really holds key
// focus and drops keys unless it is the shared window.

const keyDownA = `{"t":"key","code":"KeyA","d":true,"m":0}`

// panelHoldsKeyFocus makes the key-focus element another process's panel.
func panelHoldsKeyFocus(f *fakeWindow) {
	f.keyOverride, f.keyPID, f.keyFrame, f.keyOK, f.keyErr = true, otherPID, [4]float64{400, 300, 600, 60}, true, nil
}

// sharedHoldsKeyFocus makes the key-focus element the shared window, at the frame offset dx.
func sharedHoldsKeyFocus(dx float64) func(f *fakeWindow) {
	return func(f *fakeWindow) {
		fr := f.infoFrame
		f.keyOverride, f.keyPID, f.keyFrame, f.keyOK, f.keyErr = true, winPID, [4]float64{fr[0] + dx, fr[1], fr[2], fr[3]}, true, nil
	}
}

// TestWindowKeyFocus_PanelHoldsFocusDropsKeys: with a panel holding key focus,
// keys and text drop as window_key_focus, nothing posts, and no raise starts (a
// panel on top is the user's own local action); a click still posts, since the
// pointer is gated by where it lands, not by key focus.
func TestWindowKeyFocus_PanelHoldsFocusDropsKeys(t *testing.T) {
	s, fw := newWindowSync(t)
	fw.set(panelHoldsKeyFocus)
	s.do(t, keyDownA)
	s.do(t, `{"t":"text","s":"hi"}`)
	s.do(t, `{"t":"key","code":"KeyA","d":true,"r":true,"m":0}`)
	if n := counter(s.env, "window_key_focus"); n != 3 {
		t.Fatalf("window_key_focus drops = %d, want 3", n)
	}
	if n := len(s.fb.snapshot()); n != 0 {
		t.Fatalf("%d bridge calls while a panel held key focus, want 0: %+v", n, s.fb.snapshot())
	}
	if n := fw.raiseCount(); n != 0 {
		t.Errorf("a raise started for a panel on top (%d), want none", n)
	}
	s.do(t, "%s", press(0.5, 0.5))
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseDown {
		t.Errorf("a click while a panel held key focus = %+v, want it to post", c)
	}
}

// TestWindowKeyFocus_SharedWindowFocusedPasses: the shared window holds key
// focus, as the usual case and as an explicit read within the frame tolerance.
func TestWindowKeyFocus_SharedWindowFocusedPasses(t *testing.T) {
	s, fw := newWindowSync(t)
	s.do(t, keyDownA)
	s.do(t, `{"t":"key","code":"KeyA","d":false,"m":0}`)
	s.do(t, `{"t":"text","s":"hi"}`)
	fw.set(sharedHoldsKeyFocus(1)) // 1 pt off is the same window
	s.do(t, `{"t":"key","code":"KeyB","d":true,"m":0}`)
	if n := counter(s.env, "window_key_focus"); n != 0 {
		t.Fatalf("window_key_focus drops = %d, want 0", n)
	}
	if got := len(s.fb.snapshot()); got != 4 {
		t.Errorf("%d bridge calls, want 4 (down, up, text, down)", got)
	}
}

// TestWindowKeyFocus_FailsClosed: an error, no focused element, another
// process, and another window of the same app all drop the key.
func TestWindowKeyFocus_FailsClosed(t *testing.T) {
	cases := map[string]func(f *fakeWindow){
		"error": func(f *fakeWindow) {
			f.keyOverride, f.keyPID, f.keyFrame, f.keyOK, f.keyErr = true, winPID, f.infoFrame, true, errors.New("ax hung")
		},
		"no element": func(f *fakeWindow) {
			f.keyOverride, f.keyPID, f.keyFrame, f.keyOK, f.keyErr = true, winPID, f.infoFrame, false, nil
		},
		"other process": panelHoldsKeyFocus,
		"other process same frame": func(f *fakeWindow) {
			f.keyOverride, f.keyPID, f.keyFrame, f.keyOK, f.keyErr = true, otherPID, f.infoFrame, true, nil
		},
		"same app other window": sharedHoldsKeyFocus(60),
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			s, fw := newWindowSync(t)
			fw.set(set)
			s.do(t, keyDownA)
			if n := counter(s.env, "window_key_focus"); n != 1 {
				t.Errorf("window_key_focus drops = %d, want 1", n)
			}
			if n := len(s.fb.snapshot()); n != 0 {
				t.Errorf("%d bridge calls, want 0", n)
			}
		})
	}
}

// TestWindowKeyFocus_ReleaseOfHeldKeyStillPosts: a panel takes key focus while a
// key is held; the key-up still posts so nothing sticks, and the next key-down drops.
func TestWindowKeyFocus_ReleaseOfHeldKeyStillPosts(t *testing.T) {
	s, fw := newWindowSync(t)
	s.do(t, keyDownA)
	if n := len(s.fb.snapshot()); n != 1 {
		t.Fatalf("%d bridge calls after the key-down, want 1", n)
	}
	fw.set(panelHoldsKeyFocus)
	s.do(t, `{"t":"key","code":"KeyA","d":false,"m":0}`)
	calls := s.fb.snapshot()
	if len(calls) != 2 || calls[1].Op != "key" || calls[1].Down {
		t.Fatalf("calls after the release = %+v, want the key-up to post", calls)
	}
	s.do(t, keyDownA)
	if n := len(s.fb.snapshot()); n != 2 {
		t.Errorf("%d bridge calls, want 2 (the new key-down drops)", n)
	}
	if n := counter(s.env, "window_key_focus"); n != 1 {
		t.Errorf("window_key_focus drops = %d, want 1", n)
	}
}

// TestWindowKeyFocus_DisplayShareNeverReadsIt: only a window share reads key focus.
func TestWindowKeyFocus_DisplayShareNeverReadsIt(t *testing.T) {
	s, fw := newWindowSync(t)
	fw.set(panelHoldsKeyFocus)
	s.env.mgr.SetSurfaceForTest(screencap.Surface{}, screencap.SurfaceInfo{})
	s.do(t, keyDownA)
	if n := len(s.fb.snapshot()); n != 1 {
		t.Errorf("%d bridge calls on a display share, want 1", n)
	}
	fw.mu.Lock()
	reads := fw.keyReads
	fw.mu.Unlock()
	if reads != 0 {
		t.Errorf("key focus read %d times on a display share, want 0", reads)
	}
}
