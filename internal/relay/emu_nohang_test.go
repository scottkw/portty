package relay

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"time"
)

// unstrippedQueries elicit a vt reply but slip past StripTerminalQueries
// (CR-01). Each step is one PTY read.
var unstrippedQueries = []struct {
	name  string
	steps []string
}{
	{"8-bit CSI DA1", []string{"\x9bc"}},
	{"DA1 split across reads", []string{"hello\x1b", "[c"}},
	{"2048 inside a mode list", []string{"\x1b[?2004;2048h"}},
	{"2 MiB random blob", randomChunks(2<<20, 32<<10)},
}

// randomChunks returns n seeded random bytes in reads of at most chunk bytes.
// Seed 1 holds DA1 replies vt would answer (CR-01's random-binary case).
func randomChunks(n, chunk int) []string {
	b := make([]byte, n)
	rand.New(rand.NewSource(1)).Read(b)
	var out []string
	for len(b) > 0 {
		k := min(len(b), chunk)
		out = append(out, string(b[:k]))
		b = b[k:]
	}
	return out
}

// within fails the test if fn does not return in d (it then leaks fn's goroutine).
func within(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %v: the live emulator is wedged", what, d)
	}
}

// TestHub_UnstrippedQueryNeverWedges: a reply-eliciting query the strip misses
// must not block the live emulator, so output keeps flowing and Join and a
// host resize still return (CR-01).
func TestHub_UnstrippedQueryNeverWedges(t *testing.T) {
	for _, c := range unstrippedQueries {
		t.Run(c.name, func(t *testing.T) {
			// Not runJoinHub: its cleanup waits for Run, which a wedge never ends.
			r, w := io.Pipe()
			hub := NewHub("nohang", r, w, DefaultScrollbackBytes, nil)
			go hub.Run()
			t.Cleanup(func() { _ = w.Close() })
			resizeHub(t, hub, 80, 24)
			sub := &Subscriber{Msgs: make(chan []byte, 1024), CloseSlow: func() {}}
			hub.Subscribe(sub)
			defer hub.Unsubscribe(sub)

			within(t, 10*time.Second, "PTY writes", func() {
				for _, s := range c.steps {
					_, _ = io.WriteString(w, s)
				}
				_, _ = io.WriteString(w, "after-marker")
			})
			within(t, 5*time.Second, "broadcast of the next output", func() {
				for f := range sub.Msgs {
					if bytes.Contains(f, []byte("after-marker")) {
						return
					}
				}
			})
			within(t, 5*time.Second, "Join", func() { join(t, hub, 0, false) })
			within(t, 5*time.Second, "host resize", func() { resizeHub(t, hub, 100, 30) })
		})
	}
}
