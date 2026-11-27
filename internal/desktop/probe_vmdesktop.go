//go:build vmdesktop

// VM-only probe for the Phase 20 Tart VM rows (20-09). It drives the real
// bridges (input, window, pasteboard) of a running Manager from the daemon
// socket, so real macOS behaviour (CGEvent posting from a launchd daemon, the
// raise ladder, the pasteboard privacy alert) can be measured without a
// viewer. Input goes through the same parse, queue, gate and post path a
// viewer's data channel uses, as a pseudo-viewer; nothing here is a shortcut
// around a gate. Never in a release build: the vmdesktop tag must not appear in
// .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/screencap"
)

// DebugVM20Request is one probe call. Op picks the fields that matter.
type DebugVM20Request struct {
	Op       string   `json:"op"`
	Code     string   `json:"code,omitempty"`     // key
	Down     bool     `json:"down,omitempty"`     // key
	Mask     int      `json:"mask,omitempty"`     // key: the modifier mask the key carries
	Codes    []string `json:"codes,omitempty"`    // chord
	S        string   `json:"s,omitempty"`        // text, pbwrite
	DX       float64  `json:"dx,omitempty"`       // scroll, in the CoreGraphics sign
	DY       float64  `json:"dy,omitempty"`       // scroll, in the CoreGraphics sign
	Kind     string   `json:"kind,omitempty"`     // select: display or window
	ID       uint32   `json:"id,omitempty"`       // select
	WindowID uint32   `json:"windowID,omitempty"` // raise, minimized
	X        float64  `json:"x,omitempty"`        // topmost: a global point
	Y        float64  `json:"y,omitempty"`
}

// DebugVM20Response is what a probe call answers: a JSON object of the facts.
type DebugVM20Response map[string]any

var (
	vm20Mu     sync.Mutex
	vm20Viewer *viewer // the pseudo-viewer, made on first use (one Manager per process)
)

// vm20Pseudo returns the pseudo-viewer: control and surface rights, and it is
// never in Manager.viewers, so it has no dead-man's release: a held key stays
// held until a probe op lets go.
func vm20Pseudo() *viewer {
	vm20Mu.Lock()
	defer vm20Mu.Unlock()
	if vm20Viewer == nil {
		vm20Viewer = &viewer{
			Viewer: Viewer{ID: "probe-vm20", Rights: RightsControl | RightsSurface | RightsClipboard},
			in:     &viewerInput{},
		}
	}
	return vm20Viewer
}

func (m *Manager) vm20Counts(v *viewer) map[string]uint64 {
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	out := make(map[string]uint64, len(v.in.counts))
	for k, n := range v.in.counts {
		out[k] = n
	}
	return out
}

// vm20Send queues msgs for the input goroutine through parseInput and waits for
// a trailing heartbeat, which the goroutine reaches only after every message
// before it has been gated and posted.
func (m *Manager) vm20Send(v *viewer, msgs ...map[string]any) error {
	before := m.vm20Counts(v)[inHB]
	for _, w := range msgs {
		b, err := json.Marshal(w)
		if err != nil {
			return err
		}
		msg, err := parseInput(b)
		if err != nil {
			return err
		}
		m.enqueueInput(v, msg)
	}
	m.enqueueInput(v, inputMsg{T: inHB})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if m.vm20Counts(v)[inHB] > before {
			return nil
		}
	}
	return errors.New("the input goroutine did not reach the probe's heartbeat in 5 s (is the desktop on?)")
}

func vm20Key(code string, down bool, mask int) map[string]any {
	return map[string]any{"t": inKey, "code": code, "d": down, "m": mask, "r": false, "caps": false, "k": "", "ts": 0}
}

var vm20ModBit = map[string]int{
	"ShiftLeft": 1, "ShiftRight": 1, "ControlLeft": 2, "ControlRight": 2,
	"AltLeft": 4, "AltRight": 4, "MetaLeft": 8, "MetaRight": 8,
}

// vm20Select drives handleSelect as the pseudo-viewer. handleSelect answers a
// refusal on the viewer's socket, so the pseudo-viewer gets a real loopback
// socket whose far end is read for that answer.
func (m *Manager) vm20Select(ctx context.Context, v *viewer, kind string, id uint32) (string, error) {
	srvDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(srvDone)
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		pv := *v
		pv.ws = c
		m.handleSelect(ctx, &pv, kind, id)
		_ = c.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cc, _, err := websocket.Dial(rctx, "ws"+srv.URL[len("http"):], nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = cc.CloseNow() }()
	failure := ""
	for {
		_, b, err := cc.Read(rctx)
		if err != nil {
			break
		}
		var f struct{ Type, Code string }
		if json.Unmarshal(b, &f) == nil && f.Type == typeSelectFailed {
			failure = f.Code
		}
	}
	<-srvDone
	return failure, nil
}

// DebugVM20 runs one probe call. Ops: list, key, chord, text, scroll, release,
// select, raise, minimized, topmost, focus, behavior, pbread, pbwrite, control.
func (m *Manager) DebugVM20(ctx context.Context, req DebugVM20Request) (DebugVM20Response, error) {
	v := vm20Pseudo()
	out := DebugVM20Response{"op": req.Op}
	started := time.Now()
	defer func() { out["elapsed_ms"] = time.Since(started).Milliseconds() }()
	switch req.Op {
	case "list":
		list, err := m.listSurfaces(ctx)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(list))
		for _, s := range list {
			rows = append(rows, map[string]any{
				"kind": kindName(s.Kind), "id": s.ID, "pid": s.PID, "app": s.App, "title": s.Title,
				"x": s.X, "y": s.Y, "w": s.W, "h": s.H, "main": s.Main, "on_screen": s.OnScreen,
			})
		}
		out["surfaces"] = rows
	case "key":
		if err := m.vm20Send(v, vm20Key(req.Code, req.Down, req.Mask)); err != nil {
			return nil, err
		}
	case "chord":
		var msgs []map[string]any
		mask := 0
		for _, c := range req.Codes {
			mask |= vm20ModBit[c]
			msgs = append(msgs, vm20Key(c, true, mask))
		}
		for i := len(req.Codes) - 1; i >= 0; i-- {
			mask &^= vm20ModBit[req.Codes[i]]
			msgs = append(msgs, vm20Key(req.Codes[i], false, mask))
		}
		if err := m.vm20Send(v, msgs...); err != nil {
			return nil, err
		}
	case "text":
		if err := m.vm20Send(v, map[string]any{"t": inText, "s": req.S}); err != nil {
			return nil, err
		}
		out["bytes"] = len(req.S)
	case "scroll":
		// The wire carries the browser's sign and the daemon inverts it, so the
		// probe takes the CoreGraphics sign (what the Mac receives) and negates it.
		msg := map[string]any{"t": inWheel, "dx": -req.DX, "dy": -req.DY, "e": m.currentEpoch()}
		if err := m.vm20Send(v, msg); err != nil {
			return nil, err
		}
	case "release":
		if err := m.vm20Send(v, map[string]any{"t": inRelease}); err != nil {
			return nil, err
		}
	case "select":
		code, err := m.vm20Select(ctx, v, req.Kind, req.ID)
		if err != nil {
			return nil, err
		}
		cur := m.CurrentSurface()
		out["select_failed"] = code
		out["surface"] = map[string]any{"kind": kindName(cur.Kind), "id": cur.ID, "w": cur.W, "h": cur.H, "epoch": m.currentEpoch()}
	case "raise", "minimized":
		list, err := m.listSurfaces(ctx)
		if err != nil {
			return nil, err
		}
		found := false
		for _, s := range list {
			if s.Kind != screencap.SurfaceWindow || s.ID != req.WindowID {
				continue
			}
			found = true
			if req.Op == "minimized" {
				mini, err := m.d.Window.Minimized(s.PID, s.X, s.Y, s.W, s.H, s.Title)
				out["minimized"], out["error"] = mini, errText(err)
				break
			}
			rung, err := m.d.Window.Raise(s.PID, s.X, s.Y, s.W, s.H, s.Title)
			out["rung"], out["error"] = rung, errText(err)
		}
		out["found"] = found
	case "topmost":
		id, err := m.d.Window.TopmostAt(req.X, req.Y)
		out["window_id"], out["error"] = id, errText(err)
	case "focus":
		pid, x, y, w, h, ok, err := m.d.Window.Focus()
		out["pid"], out["ok"], out["error"] = pid, ok, errText(err)
		out["frame"] = []float64{x, y, w, h}
	case "behavior":
		var b interface{ Behavior() (string, error) } = screencap.NewPasteboard()
		if pb, ok := m.d.Pasteboard.(interface{ Behavior() (string, error) }); ok {
			b = pb
		}
		s, err := b.Behavior()
		out["behavior"], out["error"] = s, errText(err)
	case "pbread":
		// The same bounded wait handleClipGet uses: a read still blocked after
		// clipReadTimeout is reported as waiting_approval, and the daemon keeps serving.
		type res struct {
			text string
			out  screencap.ReadOutcome
			err  error
		}
		done := make(chan res, 1)
		go func() {
			defer recoverLog()
			var r res
			r.text, r.out, r.err = m.d.Pasteboard.ReadText(clipMaxBytes)
			done <- r
		}()
		select {
		case r := <-done:
			out["outcome"] = [...]string{"ok", "concealed", "empty", "too_large", "denied"}[r.out]
			out["bytes"], out["text"], out["error"] = len(r.text), r.text, errText(r.err)
		case <-time.After(clipReadTimeout):
			out["outcome"] = clipWaitingApproval
		}
	case "pbwrite":
		out["error"] = errText(m.d.Pasteboard.WriteText(req.S))
	case "control":
		in, win := m.ControlState()
		out["control_input"], out["control_window"] = in, win
		out["counters"] = m.InputCounters()
		out["pseudo_counters"] = m.vm20Counts(v)
	default:
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	if req.Op != "control" {
		out["counters"] = m.InputCounters()
	}
	return out, nil
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
