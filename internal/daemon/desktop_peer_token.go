package daemon

import "net/url"

// DesktopToken is the standing always-available desktop token for the same-user
// peer route (POST /api/peer/desktop/cap). It is the cap query value of the Copy
// Link link, taken from desktopLinkLocked so the claims are signed in exactly one
// place and the peer token is always the Copy Link token. It is false while
// always-available is off or the share server is not running.
func (d desktopAdapter) DesktopToken() (string, bool) {
	d.a.mu.RLock()
	link, err := d.a.desktopLinkLocked()
	d.a.mu.RUnlock()
	if err != nil {
		return "", false
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", false
	}
	tok := u.Query().Get("cap")
	return tok, tok != ""
}
