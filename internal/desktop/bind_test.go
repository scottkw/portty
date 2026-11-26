package desktop

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// loopbackHook sets the test-only loopback allowance to on for one test and
// restores the previous value after it.
func loopbackHook(t *testing.T, on bool) {
	t.Helper()
	prev := allowLoopbackForTest.Load()
	allowLoopbackForTest.Store(on)
	t.Cleanup(func() { allowLoopbackForTest.Store(prev) })
}

// TestCheckBind_Boundaries pins the tailnet-only bind rule at both ends of the
// CGNAT block and at the addresses that look like it.
func TestCheckBind_Boundaries(t *testing.T) {
	loopbackHook(t, false)
	for _, tc := range []struct {
		addr string
		ok   bool
	}{
		{"100.64.0.0", true},
		{"100.100.100.100", true},
		{"100.127.255.255", true}, // the last address before 100.128.0.0
		{"100.63.255.255", false}, // the address just before 100.64.0.0
		{"100.128.0.0", false},
		{"0.0.0.0", false},
		{"192.168.1.5", false},
		{"::", false},
		{"fd7a:115c:a1e0::1", false},
		{"::ffff:100.64.0.1", false}, // IPv4-mapped is still not a plain IPv4
		{"127.0.0.1", false},
	} {
		err := checkBind(netip.MustParseAddr(tc.addr))
		if tc.ok && err != nil {
			t.Errorf("checkBind(%s) = %v, want accepted", tc.addr, err)
		}
		if !tc.ok && !errors.Is(err, ErrBindRefused) {
			t.Errorf("checkBind(%s) = %v, want ErrBindRefused", tc.addr, err)
		}
	}
	if err := checkBind(netip.Addr{}); !errors.Is(err, ErrNoTailnetAddr) {
		t.Errorf("checkBind(zero) = %v, want ErrNoTailnetAddr", err)
	}

	loopbackHook(t, true)
	if err := checkBind(netip.MustParseAddr("127.0.0.1")); err != nil {
		t.Errorf("loopback with the test hook on: %v, want accepted", err)
	}
	// The hook admits loopback and nothing else.
	if err := checkBind(netip.MustParseAddr("192.168.1.5")); !errors.Is(err, ErrBindRefused) {
		t.Errorf("192.168.1.5 with the test hook on: %v, want ErrBindRefused", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	return c.LocalAddr().(*net.UDPAddr).Port
}

func testDeps(bind netip.Addr, port int) Deps {
	return Deps{Bind: bind, MediaPort: port, NewSource: func() screencap.Source { return screencap.NewSyntheticSource(1280, 800) }}
}

// TestManager_StartRefusesWithoutTailnetAddr: no tailnet address (or a
// non-tailnet one) means no socket at all.
func TestManager_StartRefusesWithoutTailnetAddr(t *testing.T) {
	loopbackHook(t, false)
	port := freePort(t)

	if _, err := NewManager(testDeps(netip.Addr{}, port)); !errors.Is(err, ErrNoTailnetAddr) {
		t.Fatalf("NewManager with the zero address: %v, want ErrNoTailnetAddr", err)
	}
	m, err := NewManager(testDeps(netip.MustParseAddr("192.168.1.5"), port))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); !errors.Is(err, ErrBindRefused) {
		t.Fatalf("Start on a non-tailnet address: %v, want ErrBindRefused", err)
	}

	// Nothing holds the port: a fresh bind on it succeeds.
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatalf("port %d was left bound: %v", port, err)
	}
	_ = c.Close()
	_ = m.Close()
}

// shortRetry shrinks listenMedia's retry timing for one test.
func shortRetry(t *testing.T, every, window time.Duration) {
	t.Helper()
	pe, pw := bindRetryEvery, bindRetryWindow
	bindRetryEvery, bindRetryWindow = every, window
	t.Cleanup(func() { bindRetryEvery, bindRetryWindow = pe, pw })
}

// TestListenMedia_RetriesWhileBusy: the upgrade case, where the old daemon
// still holds the port for a moment.
func TestListenMedia_RetriesWhileBusy(t *testing.T) {
	loopbackHook(t, true)
	lo := netip.MustParseAddr("127.0.0.1")
	hold := func() (*net.UDPConn, int) {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		return c, c.LocalAddr().(*net.UDPAddr).Port
	}

	shortRetry(t, 50*time.Millisecond, 5*time.Second)
	held, port := hold()
	go func() {
		time.Sleep(600 * time.Millisecond)
		_ = held.Close()
	}()
	start := time.Now()
	c, err := listenMedia(context.Background(), lo, port)
	if err != nil {
		t.Fatalf("listenMedia after the holder released: %v", err)
	}
	_ = c.Close()
	if d := time.Since(start); d < 500*time.Millisecond {
		t.Errorf("bound after %v, before the holder released", d)
	}

	shortRetry(t, 50*time.Millisecond, 300*time.Millisecond)
	held, port = hold()
	defer func() { _ = held.Close() }()
	if _, err := listenMedia(context.Background(), lo, port); !errors.Is(err, syscall.EADDRINUSE) {
		t.Errorf("a never-released port: %v, want EADDRINUSE", err)
	}

	// A refused address never retries.
	shortRetry(t, 50*time.Millisecond, 5*time.Second)
	start = time.Now()
	if _, err := listenMedia(context.Background(), netip.MustParseAddr("192.168.1.5"), port); !errors.Is(err, ErrBindRefused) {
		t.Errorf("refused address: %v, want ErrBindRefused", err)
	}
	if time.Since(start) > time.Second {
		t.Error("a refused address was retried")
	}
}
