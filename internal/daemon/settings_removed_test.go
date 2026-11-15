package daemon_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Portty 01-10 (CONTEXT Amendment 2, T-01-35): tabs always close on exit, so the
// auto-close setting and its routes are gone from the daemon socket mux.
func TestSettingsRemoved_AutoCloseSessionRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/auto-close-session")
	assertRouteGone(t, h, http.MethodPatch, "/settings/auto-close-session")

	// A kept sibling settings route still answers, so the 404/405 above is the
	// removal and not a broken handler.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/plugins", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /settings/plugins = %d; want 200", rec.Code)
	}
}
