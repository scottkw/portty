//go:build vmdesktop

// VM-only routes for the Tart VM rows (19-12, row V1): POST and DELETE
// /desktop/debug/lapse on the daemon socket start and end a simulated approval
// lapse (desktop.Manager.SimulateLapse), so the daemon's own notification
// sender can be watched on real macOS. Never in a release build: the
// vmdesktop tag must not appear in .github, build.sh or scripts
// (tests/check-brand.sh enforces this in CI).

package daemon

import "net/http"

func init() {
	extraDesktopRoutes = append(extraDesktopRoutes, func(a *API, mux *http.ServeMux) {
		mux.HandleFunc("POST /desktop/debug/lapse", a.handleDebugLapse(true))
		mux.HandleFunc("DELETE /desktop/debug/lapse", a.handleDebugLapse(false))
	})
}

func (a *API) handleDebugLapse(start bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.mu.RLock()
		mgr := a.desktopMgr
		a.mu.RUnlock()
		if mgr == nil {
			http.Error(w, "the desktop is not running", http.StatusConflict)
			return
		}
		if start {
			mgr.SimulateLapse()
		} else {
			mgr.ClearSimulatedLapse()
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
