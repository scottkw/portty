package daemon

// PutRemoteCapForTest stores a remote capability the way POST /peers/connect
// does, for proxy tests that don't need a fake peer's meta and cap routes.
func (a *API) PutRemoteCapForTest(sessionID, baseURL, capToken string) error {
	return a.remoteCaps.Put(sessionID, baseURL, "", capToken)
}
