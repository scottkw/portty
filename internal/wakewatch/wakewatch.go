// Package wakewatch tells a reconnect loop that this Mac just woke from sleep
// or changed networks, so it can redial at once instead of waiting out a dead
// socket's watchdog or a pending backoff (SHAR-07). It is pure Go because
// `portty attach` runs in a terminal with no AppKit run loop, where NSWorkspace
// wake notifications never arrive; the GUI uses the same detector.
//
// Sleep shows up as a jump in the wall clock between two ticks of a short
// ticker. That holds whichever clock Go's darwin ticker runs on: if the
// monotonic clock pauses in sleep, the ticker fires about one interval after
// wake; if it keeps running, the ticker fires at wake. Either way the wall-clock
// gap since the previous tick is the sleep length. A network or access-point
// switch shows up as a change in this Mac's routable interface addresses.
package wakewatch

import (
	"context"
	"net"
	"slices"
	"strings"
	"time"
)

// Reason says why Watch fired.
type Reason string

const (
	Woke           Reason = "wake"    // the Mac slept (or the process was stopped) and resumed
	NetworkChanged Reason = "network" // the routable interface addresses changed
)

const (
	interval = 2 * time.Second
	slack    = 3 * time.Second // a tick later than interval+slack means the machine was asleep
)

type config struct {
	interval, slack time.Duration
	now             func() time.Time // wall clock, no monotonic reading
	addrs           func() ([]net.Addr, error)
	tick            <-chan time.Time
}

// state is what step remembers between ticks.
type state struct {
	wall  time.Time
	key   string
	known bool // key came from a successful lookup
}

// Watch reports wake and network-change events until ctx ends, then closes
// the channel. The channel holds one event: a send never blocks, and an event
// arriving while one is pending is dropped, not queued.
func Watch(ctx context.Context) <-chan Reason {
	t := time.NewTicker(interval)
	context.AfterFunc(ctx, t.Stop)
	return watch(ctx, config{
		interval: interval,
		slack:    slack,
		now:      func() time.Time { return time.Now().Round(0) },
		addrs:    net.InterfaceAddrs,
		tick:     t.C,
	})
}

func watch(ctx context.Context, c config) <-chan Reason {
	out := make(chan Reason, 1)
	s := state{wall: c.now()}
	if a, err := c.addrs(); err == nil {
		s.key, s.known = addrKey(a), true
	}
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.tick:
			}
			a, err := c.addrs()
			var r Reason
			if s, r = c.step(s, c.now(), a, err); r != "" {
				select {
				case out <- r:
				default:
				}
			}
		}
	}()
	return out
}

// step decides one tick. A failed address lookup keeps the previous key;
// a wake and an address change on the same tick report Woke.
func (c config) step(s state, now time.Time, addrs []net.Addr, err error) (state, Reason) {
	var r Reason
	if err == nil {
		key := addrKey(addrs)
		if s.known && key != s.key {
			r = NetworkChanged
		}
		s.key, s.known = key, true
	}
	if now.Sub(s.wall) > c.interval+c.slack {
		r = Woke
	}
	s.wall = now
	return s, r
}

// addrKey is a sorted list of the routable addresses (no loopback, no
// fe80::/10 or 169.254/16, which every interface keeps across networks).
func addrKey(addrs []net.Addr) string {
	var keep []string
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.IsLoopback() || n.IP.IsLinkLocalUnicast() {
			continue
		}
		keep = append(keep, n.String())
	}
	slices.Sort(keep)
	return strings.Join(keep, ",")
}
