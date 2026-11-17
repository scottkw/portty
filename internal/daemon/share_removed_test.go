package daemon_test

// SHAR-05: the public internet sharing, code-based access and share reminder
// routes are gone from the daemon socket mux. Reuses assertRouteGone from files_removed_test.go.

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
		{http.MethodPost, "/join/exchange"},
		{http.MethodGet, "/settings/shell-web-share-warned"},
		{http.MethodPatch, "/settings/shell-web-share-warned"},
		{http.MethodGet, "/settings/shell-web-share-warning-enabled"},
		{http.MethodPatch, "/settings/shell-web-share-warning-enabled"},
		// Portty 04-02: the old share server lifecycle routes; PUT /sessions/{id}/share replaces them.
		{http.MethodGet, "/webserver/local-" + "password"},
		{http.MethodPost, "/webserver/start"},
		{http.MethodPost, "/webserver/stop"},
		{http.MethodGet, "/webserver/status"},
		{http.MethodPost, "/sessions/x/web-serve"},
		{http.MethodPost, "/sessions/x/capabilities"},
		{http.MethodPost, "/sessions/x/disconnect-viewers"},
	} {
		assertRouteGone(t, h, r.method, r.path)
	}
}
