// Package wakewatch tells a reconnect loop that this Mac just woke from sleep
// or changed networks, so it can redial at once instead of waiting out a dead
// socket's watchdog or a pending backoff (SHAR-07).
package wakewatch

import (
	"context"
	"net"
	"time"
)

// Reason says why Watch fired.
type Reason string

const (
	Woke           Reason = "wake"    // the Mac slept (or the process was stopped) and resumed
	NetworkChanged Reason = "network" // the routable interface addresses changed
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

// Watch reports wake and network-change events until ctx ends.
func Watch(ctx context.Context) <-chan Reason {
	return watch(ctx, config{})
}

func watch(ctx context.Context, c config) <-chan Reason {
	out := make(chan Reason, 1)
	go func() { <-ctx.Done(); close(out) }()
	return out
}

func (c config) step(s state, now time.Time, addrs []net.Addr, err error) (state, Reason) {
	return s, "" // RED scaffold
}

func addrKey(addrs []net.Addr) string { return "" }
