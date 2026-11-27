package desktop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// auditFields is the Phase 22 field set of an audit line: an unlock line carries
// these and nothing else, so no field can hold a password, its length, a key
// code or a character.
var auditFields = []string{"v", "ts", "event", "share_id", "mode", "user_id", "login_name", "display_name", "node_name", "node_stable_id", "tailnet_ip", "reason"}

// auditClean reads the real desktop-audit.jsonl the rig wrote and fails unless
// its events are exactly want (in order), every line has only the Phase 22
// fields, mode own and a fixed reason, and neither the sentinel, its length nor
// any of its characters' key codes appears anywhere in the file.
func auditClean(t *testing.T, what string, rr *relockRig, want ...string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(rr.dir, "desktop-audit.jsonl"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if s := string(raw); strings.Contains(s, sentinelPassword) || strings.Contains(s, "xk29") || lengthField.MatchString(s) {
		t.Errorf("%s: the audit file holds the password or its length:\n%s", what, s)
	}
	lines := rr.audit(t)
	var got []string
	for _, m := range lines {
		got = append(got, m["event"].(string))
		for k := range m {
			if !slices.Contains(auditFields, k) {
				t.Errorf("%s: audit line has a field %q outside the Phase 22 set: %v", what, k, m)
			}
		}
		if m["mode"] != "own" || m["share_id"] != "" {
			t.Errorf("%s: unlock line %v is not an own-account line", what, m)
		}
		if reason, ok := m["reason"]; ok && reason != "still_locked" && reason != "error" {
			t.Errorf("%s: audit reason %v is not one of the fixed ones", what, reason)
		}
		for k, v := range m {
			if n, ok := v.(float64); ok && k != "v" && k != "user_id" {
				t.Errorf("%s: numeric audit field %q = %v could carry a length or a code", what, k, n)
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: audit events %v, want %v", what, got, want)
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
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "unlocked")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "success", log, r, v.surfViewer)
		auditClean(t, "success", rr, "unlocked")
	})

	t.Run("retry succeeds", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.unlockOnReturn = 2
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "unlocked")
		noLeak(t, "retry", log, r, v.surfViewer)
		auditClean(t, "retry", rr, "unlocked")
	})

	t.Run("three failures pause", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.unlockOnReturn = 0
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		failSubmit(t, r, v, "unlocking", "retrying", "failed")
		failSubmit(t, r, v, failedTwice...)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, append(append([]string(nil), failedTwice...), "unlocking", "retrying", "paused")...)
		r.clk.Advance(5 * time.Second)
		// Refused while paused: nothing typed, nothing audited. The counter is 2: the
		// third failure's pause, then this refusal.
		v.submit(t, sentinelPassword)
		eventually(t, 5*time.Second, "the refusal while paused", func() bool { return r.mgr.InputCounters()["unlock_paused"] == 2 })
		noLeak(t, "pause", log, r, v.surfViewer)
		auditClean(t, "pause", rr, "unlock_failed", "unlock_failed", "unlock_failed")
	})

	t.Run("failed", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.unlockOnReturn = 0
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "failed", log, r, v.surfViewer)
		auditClean(t, "failed", rr, "unlock_failed")
	})

	t.Run("refused caps lock", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.caps = true
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "refused")
		noLeak(t, "caps lock", log, r, v.surfViewer)
		auditClean(t, "caps lock", rr) // nothing was typed: nothing is audited
	})

	t.Run("refused unmappable", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.mapErr = desktop.ErrUnmappable
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "refused")
		eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
		noLeak(t, "unmappable", log, r, v.surfViewer)
		auditClean(t, "unmappable", rr)
	})

	t.Run("rate limited", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.unlockOnReturn = 0
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed")
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed", "rate_limited")
		noLeak(t, "rate limited", log, r, v.surfViewer)
		auditClean(t, "rate limited", rr, "unlock_failed") // the refused second submit typed nothing
	})

	t.Run("forbidden from a guest", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
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
		auditClean(t, "guest", rr)
	})

	t.Run("bridge panics mid-type", func(t *testing.T) {
		log := captureLog(t)
		rr := newRelockRig(t, nil)
		r := rr.unlockRig
		r.fb.mu.Lock()
		r.fb.typePanic = true
		r.fb.mu.Unlock()
		v := r.ownUnlockViewerAs(ctx, t, relockWho)
		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "error")
		eventually(t, 5*time.Second, "the buffer to be zeroed after the panic", r.fb.bufferZeroed)
		eventually(t, 5*time.Second, "the recovered line", func() bool { return strings.Contains(log.String(), "desktop: recovered") })
		noLeak(t, "panic", log, r, v.surfViewer)
		auditClean(t, "panic", rr, "unlock_failed")
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
