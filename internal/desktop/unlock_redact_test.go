package desktop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
)

// syncBuf is a log sink safe for the many goroutines a Manager logs from.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLog sends every slog line, debug included, into a buffer for the life
// of the test.
func captureLog(t *testing.T) *syncBuf {
	t.Helper()
	buf := &syncBuf{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}

// lengthField matches the password's byte length as a number in a log line or a
// message: len, length, size or bytes next to it.
var lengthField = regexp.MustCompile(`(?i)(len|length|size|bytes)["=:\s]+` + fmt.Sprint(len(sentinelPassword)) + `\b`)

// noLeak fails if the sentinel password, or its length as a number field,
// appears in the captured log, any counter name, or any message any viewer was
// sent.
func noLeak(t *testing.T, what string, log *syncBuf, r *unlockRig, viewers ...*surfViewer) {
	t.Helper()
	var hay strings.Builder
	hay.WriteString(log.String())
	for name := range r.mgr.InputCounters() {
		hay.WriteString("\ncounter " + name)
	}
	for _, v := range viewers {
		hay.WriteString("\n" + v.messageBytes())
	}
	if s := hay.String(); strings.Contains(s, sentinelPassword) || strings.Contains(s, "xk29") {
		t.Errorf("%s: the sentinel password appears in the log, a counter or a message:\n%s", what, s)
	}
	if m := lengthField.FindString(hay.String()); m != "" {
		t.Errorf("%s: the password's length appears as %q", what, m)
	}
}

// TestUnlockRedact_NoLeakThroughTheRealChannel: a sentinel password goes
// through the real input channel in every outcome (success, failed, refused for
// Caps Lock and for a character with no key, rate limited, forbidden from a
// guest, a bridge that panics mid-type). The captured slog output at debug
// level, every counter name, every message sent to any viewer and the
// recovered-panic line contain neither the sentinel nor its byte length, and the
// buffer is zero after every outcome that filled it, the panic included.
func TestUnlockRedact_NoLeakThroughTheRealChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("success", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "unlocked")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "success", log, r, v.surfViewer)
	})

	t.Run("failed", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		r.fb.unlockOnReturn = 0
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "failed", log, r, v.surfViewer)
	})

	t.Run("refused caps lock", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		r.fb.caps = true
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "refused")
		noLeak(t, "caps lock", log, r, v.surfViewer)
	})

	t.Run("refused unmappable", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		r.fb.mapErr = desktop.ErrUnmappable
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "refused")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "unmappable", log, r, v.surfViewer)
	})

	t.Run("rate limited", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		r.fb.unlockOnReturn = 0
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed")
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed", "rate_limited")
		noLeak(t, "rate limited", log, r, v.surfViewer)
	})

	t.Run("forbidden from a guest", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		g := r.joinGuest(ctx, t, 1001, "nodeA")
		if err := r.mgr.SetRights(g.viewerID(), true, true); err != nil {
			t.Fatal(err)
		}
		g.wait(t, "rights", rightsIs(true, true))
		g.submit(t, sentinelPassword)
		eventually(t, 5*time.Second, "unlock_forbidden counted", func() bool {
			return r.mgr.InputCounters()["unlock_forbidden"] == 1
		})
		noLeak(t, "guest", log, r, g.surfViewer)
	})

	t.Run("bridge panics mid-type", func(t *testing.T) {
		log := captureLog(t)
		r := newUnlockRig(t)
		r.fb.mu.Lock()
		r.fb.typePanic = true
		r.fb.mu.Unlock()
		v := r.ownUnlockViewer(ctx, t)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "error")
		eventually(t, 5*time.Second, "the buffer to be zeroed after the panic", r.fb.bufferZeroed)
		eventually(t, 5*time.Second, "the recovered line", func() bool { return strings.Contains(log.String(), "desktop: recovered") })
		noLeak(t, "panic", log, r, v.surfViewer)
		if st := r.mgr.UnlockStats(); st.State != "idle" {
			t.Errorf("UnlockStats = %+v after a panic, want idle (the attempt flag released)", st)
		}
	})
}

// TestUnlockRedact_SecretFormatters: the password holder prints the redaction
// marker and nothing else through every fmt verb, value or pointer, through
// json.Marshal and through slog's text and JSON handlers.
func TestUnlockRedact_SecretFormatters(t *testing.T) {
	const marker = "[redacted]"
	for name, s := range map[string]any{
		"value":   desktop.SecretForTest([]byte(sentinelPassword)),
		"pointer": desktop.SecretPtrForTest([]byte(sentinelPassword)),
	} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%08.3f"} {
			if got := fmt.Sprintf(verb, s); got != marker {
				t.Errorf("%s %s printed %q, want %q", name, verb, got, marker)
			}
		}
		if got := fmt.Sprint(s); got != marker {
			t.Errorf("%s Sprint printed %q", name, got)
		}
		if got := fmt.Sprintf("%v", []any{s}); got != "["+marker+"]" {
			t.Errorf("%s inside a slice printed %q", name, got)
		}
		b, err := json.Marshal(s)
		if err != nil || string(b) != `"`+marker+`"` {
			t.Errorf("%s json.Marshal = %s, %v", name, b, err)
		}
		b, err = json.Marshal(map[string]any{"password": s})
		if err != nil || strings.Contains(string(b), sentinelPassword) {
			t.Errorf("%s json.Marshal of a map = %s, %v", name, b, err)
		}
		for hname, mk := range map[string]func(*bytes.Buffer) slog.Handler{
			"text": func(w *bytes.Buffer) slog.Handler { return slog.NewTextHandler(w, nil) },
			"json": func(w *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(w, nil) },
		} {
			var buf bytes.Buffer
			slog.New(mk(&buf)).Info("x", "password", s, slog.Any("again", s))
			if out := buf.String(); strings.Contains(out, sentinelPassword) || !strings.Contains(out, marker) {
				t.Errorf("%s slog %s handler printed %q", name, hname, out)
			}
		}
	}
}
