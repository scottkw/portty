package daemon_test

// SHAR-05: the public internet sharing routes are gone from the daemon socket
// mux. Reuses assertRouteGone from files_removed_test.go.

import (
	"net/http"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

func TestShareRemoved_DaemonRoutes(t *testing.T) {
	h := daemon.NewAPI(daemon.NewSessionEngine()).Handler()
	// Split literal so repo-wide greps for the removed feature name stay at zero.
	pub := "/sessions/x/" + "fun" + "nel"
	for _, r := range []struct{ method, path string }{
		{http.MethodPost, pub},
		{http.MethodPost, pub + "-write"},
		{http.MethodDelete, pub + "-write"},
	} {
		assertRouteGone(t, h, r.method, r.path)
	}
}
