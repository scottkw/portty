//go:build vmdesktop

// VM-only route for the Phase 20 Tart VM rows (20-09): POST /desktop/debug/vm20
// on the daemon socket runs one probe call against the real input, window and
// pasteboard bridges (desktop.Manager.DebugVM20). The answer is JSON; the route
// never logs a key, text or window title. Never in a release build: the
// vmdesktop tag must not appear in .github, build.sh or scripts
// (tests/check-brand.sh enforces this in CI).

package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/scottkw/portty/internal/desktop"
)

func init() {
	extraDesktopRoutes = append(extraDesktopRoutes, func(a *API, mux *http.ServeMux) {
		mux.HandleFunc("POST /desktop/debug/vm20", a.handleDebugVM20)
	})
}

func (a *API) handleDebugVM20(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	mgr := a.desktopMgr
	a.mu.RUnlock()
	if mgr == nil {
		http.Error(w, "the desktop is not running", http.StatusConflict)
		return
	}
	var req desktop.DebugVM20Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp, err := mgr.DebugVM20(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
