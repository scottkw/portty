package tailnet

import (
	"errors"
	"os"
	"testing"
)

// ErrLiveTailnetInTest is returned instead of touching tailscaled or a peer
// from a test binary, so no test can probe the user's real tailnet by
// forgetting a seam (WR-07). Set PORTTY_LIVE_TAILNET_TESTS=1 for a
// deliberate integration run.
var ErrLiveTailnetInTest = errors.New("tailnet: live tailscaled/peer access disabled in tests")

// LiveAllowed reports whether code may reach tailscaled or a tailnet peer.
// Always true in a real binary: testing.Testing() is false outside go test.
func LiveAllowed() bool {
	return !testing.Testing() || os.Getenv("PORTTY_LIVE_TAILNET_TESTS") == "1"
}
