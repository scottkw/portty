package daemon

// Remote-session client helpers behind the Wails-exposed RegisterRemoteCap and
// RemoteSessionOpenURL methods on *main.App: they deposit and read back the cap
// held for a remote session in the local daemon's RemoteCapStore.

import (
	"fmt"
	"net/http"
	"net/url"
)

// RemoteSessionOpenURL retrieves the cap-bearing open URL for a remote session
// from the local daemon's RemoteCapStore. The daemon composes
// baseURL+/sessions/{id}?cap=TOKEN from the stored entry and returns it as a
// plain string. The cap token never leaves the daemon except inside the
// returned URL (which already carries it by design in the existing deposit→open
// path — T-146-05-01 accept).
//
// Returns an error containing "status 404" when no cap is stored for sessionID;
// the caller treats this as "no cap held for this session".
// Phase 146-05 / GAP-146-A.
func (c *DaemonClient) RemoteSessionOpenURL(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session id is invalid: empty")
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := c.doJSON(http.MethodGet, "/api/remote-files/caps/"+url.PathEscape(sessionID)+"/open-url", nil, &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

// RegisterRemoteCap deposits a (sessionID, baseURL, capToken) tuple into the
// local daemon's RemoteCapStore via POST /api/remote-files/caps. Plan 122-01
// owns the daemon-side handler; this helper provides the client side so the
// Wails binding has a stable target.
func (c *DaemonClient) RegisterRemoteCap(sessionID, baseURL, capToken string) error {
	if sessionID == "" {
		return fmt.Errorf("session id is invalid: empty")
	}
	if baseURL == "" {
		return fmt.Errorf("base URL is invalid: empty")
	}
	if capToken == "" {
		return fmt.Errorf("cap token is invalid: empty")
	}
	body := map[string]string{
		"sessionId": sessionID,
		"baseUrl":   baseURL,
		"capToken":  capToken,
	}
	// doJSON handles JSON encoding, the unix-socket dial, and 4xx→error
	// mapping. When Plan 122-01 lands the daemon handler this call succeeds;
	// before then the daemon returns 404 and doJSON surfaces that as an error
	// containing "status 404" — the modal handler treats this as a hard
	// failure (covered by Task 2 Test 7 — generic error path).
	return c.doJSON(http.MethodPost, "/api/remote-files/caps", body, nil)
}
