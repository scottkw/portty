package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"
)

// MediaPortDefault is the UDP port media is served on: the same number as the
// share server's TCP port, a different protocol. It travels in the SDP, so it
// is fixed rather than ephemeral.
const MediaPortDefault = 7444

// cgnat is the block Tailscale assigns tailnet IPv4 addresses from.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// Bind errors.
var (
	// ErrBindRefused means the address is not a tailnet address (T-19-10).
	ErrBindRefused = errors.New("desktop: media bind address is not a tailnet address")
	// ErrNoTailnetAddr means the daemon has no tailnet address yet, so there is
	// nothing safe to bind.
	ErrNoTailnetAddr = errors.New("desktop: no tailnet address to bind")
)

// Retry timing for a busy port, variables so a test can shorten them. The
// upgrade case: the old daemon still holds the port while it hands over.
var (
	bindRetryEvery  = 250 * time.Millisecond
	bindRetryWindow = 30 * time.Second
)

// checkBind accepts only a plain IPv4 address inside the tailnet block.
// Loopback is accepted only while a test binary holds the test hook, so
// production can never bind it; unspecified, IPv6 and every other address are
// refused.
func checkBind(a netip.Addr) error {
	switch {
	case !a.IsValid():
		return ErrNoTailnetAddr
	case !a.Is4() || a.IsUnspecified():
		return fmt.Errorf("%w: %s", ErrBindRefused, a)
	case allowLoopbackForTest.Load() && a.IsLoopback():
		return nil
	case !cgnat.Contains(a):
		return fmt.Errorf("%w: %s", ErrBindRefused, a)
	}
	return nil
}

// listenMedia binds the media UDP socket on a (checkBind first), retrying a
// busy port every bindRetryEvery for up to bindRetryWindow, then returning
// the in-use error.
func listenMedia(ctx context.Context, a netip.Addr, port int) (*net.UDPConn, error) {
	if err := checkBind(a); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(bindRetryWindow)
	for {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: a.AsSlice(), Port: port})
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("desktop: listen media socket: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(bindRetryEvery):
		}
	}
}
