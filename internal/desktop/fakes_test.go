package desktop_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// world is the fake outside world a Manager reads: lock, display, power,
// permissions, plus counters of what the Manager did to it. Its clock is the
// Manager's Deps.Now.
type world struct {
	clock *fakeClock

	mu                 sync.Mutex
	locked, asleep, ac bool
	lockErr, asleepErr error
	probeErr           error
	holdDisplayErr     error
	holdSystemErr      error
	perms              screencap.Permissions
	lockReads          int
	probes             int
	nudges             int
	declares           int
	holdDisplays       []string // names, in order taken
	holdSystems        []string
	released           int
	notifies           []notification
	nudgeFn            func() // runs inside Nudge (a test makes the source emit)
	approvalDate       time.Time
	approvalErr        error
}

type notification struct{ id, title, body string }

func newWorld() *world {
	return &world{clock: newFakeClock(), perms: screencap.Permissions{SR: true, AX: true, PE: true}, approvalErr: errors.New("store unreadable")}
}

func (w *world) set(f func()) {
	w.mu.Lock()
	f()
	w.mu.Unlock()
}

// opt wires every Deps seam to the world and shortens the state loop to 10 ms
// (the decisions still compare the fake wall clock).
func (w *world) opt() tracerOpt {
	return func(env *tracerEnv, d *desktop.Deps) {
		if env != nil {
			env.preStart = func(m *desktop.Manager) { m.SetTicksForTest(10*time.Millisecond, 10*time.Millisecond) }
		}
		d.Now = w.clock.Now
		d.Lock = func() (bool, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.lockReads++
			return w.locked, w.lockErr
		}
		d.DisplayAsleep = func() (bool, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.asleep, w.asleepErr
		}
		d.OnAC = func() (bool, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.ac, nil
		}
		d.Probe = func(context.Context) (screencap.Permissions, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			w.probes++
			return w.perms, w.probeErr
		}
		d.Nudge = func() (bool, error) {
			w.mu.Lock()
			w.nudges++
			fn := w.nudgeFn
			w.mu.Unlock()
			if fn != nil {
				fn()
			}
			return true, nil
		}
		d.HoldDisplay = func(name string) (func() error, error) { return w.hold(&w.holdDisplays, w.holdDisplayErr, name) }
		d.HoldSystem = func(name string) (func() error, error) { return w.hold(&w.holdSystems, w.holdSystemErr, name) }
		d.DeclareActivity = func(string) error {
			w.mu.Lock()
			w.declares++
			w.mu.Unlock()
			return nil
		}
		d.Notify = func(id, title, body string) {
			w.mu.Lock()
			w.notifies = append(w.notifies, notification{id, title, body})
			w.mu.Unlock()
		}
		d.ApprovalStore = func(context.Context) (time.Time, error) {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.approvalDate, w.approvalErr
		}
	}
}

func (w *world) hold(names *[]string, failWith error, name string) (func() error, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if failWith != nil {
		return nil, failWith
	}
	*names = append(*names, name)
	return func() error {
		w.mu.Lock()
		w.released++
		w.mu.Unlock()
		return nil
	}, nil
}

func (w *world) notified() []notification {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]notification(nil), w.notifies...)
}

func (w *world) counts() (nudges, declares, released int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nudges, w.declares, w.released
}

// fakeSource is a screencap.Source the test drives by hand: it emits only what
// emit gives it, so a stalled stream is a source that is simply quiet.
type fakeSource struct {
	startErr error

	mu       sync.Mutex
	frames   chan screencap.AccessUnit
	errs     chan error
	started  bool
	stopped  bool
	forced   int
	hwKnown  bool // HardwareEncoder is exposed only when set
	hardware bool
}

func (s *fakeSource) Start(context.Context, screencap.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startErr != nil {
		return s.startErr
	}
	s.started = true
	s.frames = make(chan screencap.AccessUnit, 16)
	s.errs = make(chan error, 4)
	return nil
}

func (s *fakeSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started && !s.stopped {
		s.stopped = true
		close(s.frames)
	}
	return nil
}

func (s *fakeSource) Frames() <-chan screencap.AccessUnit { return s.frames }
func (s *fakeSource) Errors() <-chan error                { return s.errs }
func (s *fakeSource) SelectSurface(screencap.Surface) error {
	return nil
}
func (s *fakeSource) Rebuild(int, int) error { return nil }

func (s *fakeSource) ForceKeyframe() {
	s.mu.Lock()
	s.forced++
	s.mu.Unlock()
}

// emit sends one access unit (a keyframe when kf is true).
func (s *fakeSource) emit(kf bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || s.stopped {
		return
	}
	au := screencap.AccessUnit{Width: 1280, Height: 800, Duration: 33 * time.Millisecond, Data: []byte{0, 0, 0, 1, 0x41, 1}}
	if kf {
		au.Keyframe = true
		au.Data = []byte{0, 0, 0, 1, 0x67, 0x42, 0xe0, 0x1f, 0, 0, 0, 1, 0x68, 0xce, 0x3c, 0x80, 0, 0, 0, 1, 0x65, 0x88}
	}
	select {
	case s.frames <- au:
	default:
	}
}

// fail reports err on the source's Errors channel.
func (s *fakeSource) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errs != nil {
		s.errs <- err
	}
}

// HardwareEncoder mimics screencap.DisplaySource's accessor for the type
// assertion; fakeHWSource wraps fakeSource with it.
type fakeHWSource struct{ *fakeSource }

func (s fakeHWSource) HardwareEncoder() (hardware, known bool) { return s.hardware, s.hwKnown }

// sources records every source the Manager's factory built.
type sources struct {
	mu       sync.Mutex
	all      []*fakeSource
	startErr error // Start error of the next sources, nil for none
	hw       bool  // wrap new sources as software-encoder reporters
}

func (r *sources) factory() func() screencap.Source {
	return func() screencap.Source {
		s := &fakeSource{}
		r.mu.Lock()
		s.startErr = r.startErr
		r.all = append(r.all, s)
		r.mu.Unlock()
		if r.hw {
			s.hwKnown, s.hardware = true, false
			return fakeHWSource{s}
		}
		return s
	}
}

func (r *sources) setStartErr(err error) {
	r.mu.Lock()
	r.startErr = err
	r.mu.Unlock()
}

func (r *sources) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.all)
}

func (r *sources) last() *fakeSource {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.all) == 0 {
		return nil
	}
	return r.all[len(r.all)-1]
}

func (r *sources) opt() tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.NewSource = r.factory() }
}
