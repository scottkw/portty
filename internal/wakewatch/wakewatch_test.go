package wakewatch

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func cidrs(t *testing.T, ss ...string) []net.Addr {
	t.Helper()
	var out []net.Addr
	for _, s := range ss {
		ip, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		n.IP = ip
		out = append(out, n)
	}
	return out
}

func TestAddrKey_IgnoresLoopbackAndLinkLocal(t *testing.T) {
	a := addrKey(cidrs(t, "127.0.0.1/8", "::1/128", "fe80::1/64", "169.254.1.2/16", "192.168.1.5/24", "10.0.0.2/8"))
	b := addrKey(cidrs(t, "10.0.0.2/8", "fe80::9/64", "192.168.1.5/24"))
	if a != b || a == "" {
		t.Errorf("keys %q and %q; want equal and non-empty (loopback and link-local ignored, order ignored)", a, b)
	}
	if c := addrKey(cidrs(t, "10.0.0.3/8", "192.168.1.5/24")); c == a {
		t.Errorf("a changed address kept the key %q", c)
	}
}

func TestStep(t *testing.T) {
	c := config{interval: 2 * time.Second, slack: 3 * time.Second}
	t0 := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	home := cidrs(t, "192.168.1.5/24")
	office := cidrs(t, "10.0.0.2/8")
	start := state{wall: t0, key: addrKey(home), known: true}
	cases := []struct {
		name    string
		s       state
		gap     time.Duration
		addrs   []net.Addr
		err     error
		want    Reason
		wantKey string
	}{
		{"on-time tick", start, 2100 * time.Millisecond, home, nil, "", addrKey(home)},
		{"wall jump", start, 30 * time.Second, home, nil, Woke, addrKey(home)},
		{"address change", start, 2 * time.Second, office, nil, NetworkChanged, addrKey(office)},
		{"both: wake wins", start, 30 * time.Second, office, nil, Woke, addrKey(office)},
		{"lookup error keeps key", start, 2 * time.Second, nil, errors.New("boom"), "", addrKey(home)},
		{"first good lookup only learns", state{wall: t0}, 2 * time.Second, office, nil, "", addrKey(office)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, r := c.step(tc.s, t0.Add(tc.gap), tc.addrs, tc.err)
			if r != tc.want {
				t.Errorf("reason = %q; want %q", r, tc.want)
			}
			if s.key != tc.wantKey || !s.wall.Equal(t0.Add(tc.gap)) {
				t.Errorf("state = %+v; want key %q and wall advanced", s, tc.wantKey)
			}
		})
	}
}

func TestWatch_OneEventPerJumpNoQueueClosesOnCancel(t *testing.T) {
	var mu sync.Mutex
	clock := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	advance := func(d time.Duration) { mu.Lock(); clock = clock.Add(d); mu.Unlock() }
	tick := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := watch(ctx, config{
		interval: 2 * time.Second, slack: 3 * time.Second,
		now:   func() time.Time { mu.Lock(); defer mu.Unlock(); return clock },
		addrs: func() ([]net.Addr, error) { return cidrs(t, "192.168.1.5/24"), nil },
		tick:  tick,
	})
	send := func(d time.Duration) {
		advance(d)
		select {
		case tick <- time.Time{}:
		case <-time.After(time.Second):
			t.Fatal("watcher did not take a tick")
		}
	}
	send(2 * time.Second)
	send(30 * time.Second) // slept
	send(2 * time.Second)  // taken only after the jump tick was handled
	select {
	case r := <-ch:
		if r != Woke {
			t.Fatalf("event = %q; want %q", r, Woke)
		}
	default:
		t.Fatal("no event after a wall-clock jump")
	}
	select {
	case r := <-ch:
		t.Fatalf("extra event %q after one jump", r)
	default:
	}

	send(30 * time.Second)
	send(30 * time.Second) // a second event while one is pending is dropped
	send(2 * time.Second)
	<-ch
	select {
	case r := <-ch:
		t.Fatalf("pending event was queued behind: %q", r)
	default:
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("event after cancel; want the channel closed")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after ctx ended")
	}
}

func TestWatch_RealTickerStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := Watch(ctx)
	cancel()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("Watch channel not closed after ctx ended")
	}
}
