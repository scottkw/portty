package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/webserver"
)

// desktop_share_api.go: the click-to-share (Phase 22) as the daemon runs it.
// One guest share at a time, in memory only. Everything the sharer can do to it
// (start, stop, admit, deny, rights, disconnect) exists only here, on the
// daemon's own unix socket; the share server carries no such route. The GUI's
// events stream is the liveness signal: with no GUI holding it no share may
// exist, because then nobody sees the indicator (DSHR-06).

// Error codes of the share routes, beside the Tailscale refusals (ShareErr*).
const (
	shareErrGUIRequired = "gui_required"
	shareErrGUIBusy     = "gui_busy"
	shareErrNoRequest   = "no_request"
	shareErrNoViewer    = "no_viewer"
	shareErrOwnViewer   = "own_viewer"
	shareErrBadRequest  = "bad_request"
)

// Line types of the events stream.
const (
	shareLineState     = "state"
	shareLineEvent     = "event"
	shareLineHeartbeat = "heartbeat"
)

// Timing of the events stream. Vars so tests shorten them.
var (
	// shareHeartbeat is the gap between heartbeat lines.
	shareHeartbeat = 15 * time.Second
	// shareStateGap is the least gap between two state lines: at most 10 a second.
	shareStateGap = 100 * time.Millisecond
	// sharePollEvery re-reads the facts no event announces (Secure Event Input,
	// whether anyone controls).
	sharePollEvery = time.Second
)

// shareEventQueue bounds the event lines waiting for the stream. A state line
// carries the same facts, so a dropped event costs a notification, not a fact.
const shareEventQueue = 256

// DesktopShareViewer is one viewer or waiting guest as the GUI sees it. There is
// deliberately no user ID, tailnet address or token here: the names are
// sanitized (desktop.NewIdentity) and the numeric identifiers stay in the daemon
// and its audit log.
type DesktopShareViewer struct {
	ID          string `json:"id"`
	Mode        string `json:"mode"`
	DisplayName string `json:"displayName"`
	LoginName   string `json:"loginName"`
	NodeName    string `json:"nodeName"`
	Control     bool   `json:"control"`
	Clipboard   bool   `json:"clipboard"`
	Since       string `json:"since"`
}

// DesktopSharePending is a guest waiting at Admit; ID is the request ID the
// admit and deny routes take.
type DesktopSharePending DesktopShareViewer

// DesktopShareState is the first line of the events stream and every later
// change: whether a guest share is active, who is watching and who is waiting.
type DesktopShareState struct {
	Type          string                `json:"type"`
	Active        bool                  `json:"active"`
	Viewers       []DesktopShareViewer  `json:"viewers"`
	Pending       []DesktopSharePending `json:"pending"`
	SecureInput   bool                  `json:"secureInput"`
	AnyController bool                  `json:"anyController"`
}

// DesktopShareLine is any line of the events stream, for the reader: a state
// line fills the state fields, an event line (admit_request, join, leave,
// share_stop) fills Event, Viewer and Reason, a heartbeat only Type.
type DesktopShareLine struct {
	Type          string                `json:"type"`
	Active        bool                  `json:"active,omitempty"`
	Viewers       []DesktopShareViewer  `json:"viewers,omitempty"`
	Pending       []DesktopSharePending `json:"pending,omitempty"`
	SecureInput   bool                  `json:"secureInput,omitempty"`
	AnyController bool                  `json:"anyController,omitempty"`
	Event         string                `json:"event,omitempty"`
	Viewer        *DesktopShareViewer   `json:"viewer,omitempty"`
	Reason        string                `json:"reason,omitempty"`
}

// DesktopShareStart answers POST /desktop/share. FirstShare is true exactly when
// this call recorded the Certificate Transparency acknowledgement.
type DesktopShareStart struct {
	URL        string `json:"url"`
	FirstShare bool   `json:"firstShare,omitempty"`
}

// DesktopRightsRequest is the PUT /desktop/share/viewers/{id}/rights body.
type DesktopRightsRequest struct {
	Control   bool `json:"control"`
	Clipboard bool `json:"clipboard"`
}

// DesktopShareStatus answers GET /desktop/share: the state, plus the link while
// a share is active (the link never travels in the events stream).
type DesktopShareStatus struct {
	DesktopShareState
	URL string `json:"url,omitempty"`
}

func viewerView(v desktop.Viewer) DesktopShareViewer {
	return DesktopShareViewer{
		ID: string(v.ID), Mode: v.Mode,
		DisplayName: v.Identity.DisplayName, LoginName: v.Identity.LoginName, NodeName: v.Identity.NodeName,
		Control:   v.Rights&desktop.RightsControl != 0,
		Clipboard: v.Rights&desktop.RightsClipboard != 0,
		Since:     v.JoinedAt.UTC().Format(time.RFC3339),
	}
}

func pendingView(r desktop.Request) DesktopSharePending {
	return DesktopSharePending{
		ID: r.ID, Mode: desktop.ModeGuest,
		DisplayName: r.Ident.DisplayName, LoginName: r.Ident.LoginName, NodeName: r.Ident.NodeName,
		Since: r.Since.UTC().Format(time.RFC3339),
	}
}

// shareStream is the one GUI stream's mailbox: event lines wait here until the
// handler writes them, and wake tells it there is something to write.
type shareStream struct {
	wake chan struct{}
	mu   sync.Mutex
	q    []DesktopShareLine
}

func (s *shareStream) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *shareStream) push(l DesktopShareLine) {
	s.mu.Lock()
	if len(s.q) >= shareEventQueue {
		s.q = s.q[1:]
	}
	s.q = append(s.q, l)
	s.mu.Unlock()
	s.poke()
}

func (s *shareStream) drain() []DesktopShareLine {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.q
	s.q = nil
	return out
}

func (a *API) registerDesktopShareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /desktop/share/events", a.handleDesktopShareEvents)
	mux.HandleFunc("POST /desktop/share", a.handleStartDesktopShare)
	mux.HandleFunc("GET /desktop/share", a.handleGetDesktopShare)
	mux.HandleFunc("POST /desktop/share/requests/{id}/admit", a.handleShareAdmit)
	mux.HandleFunc("POST /desktop/share/requests/{id}/deny", a.handleShareDeny)
	mux.HandleFunc("DELETE /desktop/share", a.handleStopDesktopShare)
	mux.HandleFunc("PUT /desktop/share/viewers/{id}/rights", a.handleShareRights)
	mux.HandleFunc("DELETE /desktop/share/viewers/{id}", a.handleShareDisconnect)
}

// writeShareErr answers with the {"error": code} body the client turns back
// into a *ShareError.
func writeShareErr(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, ShareErrorResponse{Error: code, Detail: detail})
}

func desktopShareStatus(code string) int {
	switch code {
	case shareErrGUIRequired, shareErrGUIBusy, shareErrOwnViewer:
		return http.StatusConflict
	case shareErrNoRequest, shareErrNoViewer:
		return http.StatusNotFound
	case shareErrBadRequest:
		return http.StatusBadRequest
	}
	return shareStatus(code)
}

// --- the GUI's events stream -------------------------------------------------

// takeGUI gives the one GUI slot to a new stream, or reports it taken.
func (a *API) takeGUI() (*shareStream, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.guiStream != nil {
		return nil, false
	}
	st := &shareStream{wake: make(chan struct{}, 1)}
	a.guiStream = st
	return st, true
}

// releaseGUI frees the slot st held and ends the guest share at once: with no
// GUI there is no indicator on screen, so nobody may keep watching (DSHR-06).
// The slot is freed first, so a start that races this sees no GUI.
func (a *API) releaseGUI(st *shareStream) {
	a.mu.Lock()
	if a.guiStream == st {
		a.guiStream = nil
	}
	a.mu.Unlock()
	a.endGuestShare(desktop.ReasonGuiGone)
}

func (a *API) guiHeld() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.guiStream != nil
}

// pushShareEvent queues one event line for the GUI, if one is connected.
func (a *API) pushShareEvent(event string, v *DesktopShareViewer, reason string) {
	a.mu.RLock()
	st := a.guiStream
	a.mu.RUnlock()
	if st != nil {
		st.push(DesktopShareLine{Type: shareLineEvent, Event: event, Viewer: v, Reason: reason})
	}
}

// pokeShare tells the stream something changed, so it sends a fresh state line.
func (a *API) pokeShare() {
	a.mu.RLock()
	st := a.guiStream
	a.mu.RUnlock()
	if st != nil {
		st.poke()
	}
}

// shareState reads the current state line.
func (a *API) shareState() DesktopShareState {
	a.mu.RLock()
	gs, mgr := a.guestShare, a.desktopMgr
	a.mu.RUnlock()
	st := DesktopShareState{Type: shareLineState, Active: gs != nil, Viewers: []DesktopShareViewer{}, Pending: []DesktopSharePending{}}
	if mgr != nil {
		for _, v := range mgr.ViewerList() {
			st.Viewers = append(st.Viewers, viewerView(v))
		}
		st.SecureInput, st.AnyController = mgr.SecureInput(), mgr.AnyController()
	}
	if gs != nil {
		for _, r := range gs.Pending() {
			st.Pending = append(st.Pending, pendingView(r))
		}
	}
	return st
}

// handleDesktopShareEvents is the GUI's long-lived stream (NDJSON): a state line
// on connect and on every change (at most 10 a second), event lines for the
// notifications, and a heartbeat. Exactly one GUI may hold it. Its end is the
// liveness signal: the unix-socket EOF is definitive, so there is no grace.
func (a *API) handleDesktopShareEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("gui") != "1" {
		writeShareErr(w, http.StatusBadRequest, shareErrBadRequest, "gui=1 is required")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeShareErr(w, http.StatusInternalServerError, ShareErrStartFailed, "no streaming")
		return
	}
	st, ok := a.takeGUI()
	if !ok {
		writeShareErr(w, http.StatusConflict, shareErrGUIBusy, "")
		return
	}
	defer a.releaseGUI(st)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	writeLine := func(v any) bool {
		b, err := json.Marshal(v)
		if err != nil {
			return false
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	var last string
	sendState := func() bool {
		b, err := json.Marshal(a.shareState())
		if err != nil || string(b) == last {
			return err == nil
		}
		last = string(b)
		if _, err := w.Write(append(b, '\n')); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !sendState() {
		return
	}
	lastSent := time.Now()
	hb := time.NewTicker(shareHeartbeat)
	defer hb.Stop()
	poll := time.NewTicker(sharePollEvery)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.desktopCtx.Done():
			return
		case <-hb.C:
			if !writeLine(DesktopShareLine{Type: shareLineHeartbeat}) {
				return
			}
			continue
		case <-st.wake:
		case <-poll.C:
		}
		if wait := shareStateGap - time.Since(lastSent); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
		for _, l := range st.drain() {
			if !writeLine(l) {
				return
			}
		}
		if !sendState() {
			return
		}
		lastSent = time.Now()
	}
}

// --- start, link, admit, deny -----------------------------------------------

func (a *API) handleStartDesktopShare(w http.ResponseWriter, r *http.Request) {
	resp, serr := a.startGuestShare(r.Context())
	if serr == errShareFrozen {
		writeFrozen(w)
		return
	}
	if serr != nil {
		writeShareErr(w, desktopShareStatus(serr.Code), serr.Code, serr.Detail)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleGetDesktopShare(w http.ResponseWriter, _ *http.Request) {
	out := DesktopShareStatus{DesktopShareState: a.shareState()}
	if out.Active {
		a.mu.RLock()
		link, err := a.guestLinkLocked()
		a.mu.RUnlock()
		if err == nil {
			out.URL = link
		}
	}
	writeJSON(w, http.StatusOK, out)
}

var errGuestNoLink = errors.New("no guest share is running")

// guestLinkLocked is the active share's link. The issued-at is kept, so the same
// share and key give the same text each time. Caller holds a.mu.
func (a *API) guestLinkLocked() (string, error) {
	ws, gs := a.webServer, a.guestShare
	if ws == nil || gs == nil {
		return "", errGuestNoLink
	}
	tok, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{ShareID: gs.ID, IAT: a.guestIAT}, a.CurrentSigningKey())
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(ws.BaseURL(), "/") + "/desktop/guest/?cap=" + url.QueryEscape(tok), nil
}

// startGuestShare starts the one click-to-share, or returns the running one's
// link. It follows desktopOn's order (health, certificate warm-up, write gate,
// startMu) but writes nothing to desktop.json: the share is memory only.
func (a *API) startGuestShare(ctx context.Context) (DesktopShareStart, *ShareError) {
	if !a.guiHeld() {
		return DesktopShareStart{}, &ShareError{Code: shareErrGUIRequired, Detail: "the Portty window must be running to share"}
	}
	a.mu.RLock()
	health, newServer, warmCert, ifaceAddrs := a.shareHealth, a.newShareServer, a.warmCert, a.interfaceAddrs
	running := a.webServer != nil
	link, linkErr := a.guestLinkLocked()
	a.mu.RUnlock()
	if linkErr == nil {
		return DesktopShareStart{URL: link}, nil
	}
	if a.engine.configDir == "" {
		return DesktopShareStart{}, &ShareError{ShareErrStartFailed, "no config directory"}
	}
	h, serr := a.desktopTailnet(ctx, health, ifaceAddrs)
	if serr != nil {
		return DesktopShareStart{}, serr
	}
	if !running { // as desktopOn: the first warm-up can take seconds and moves nothing
		if serr := warmShareCert(ctx, warmCert, h.Domain); serr != nil {
			return DesktopShareStart{}, serr
		}
		warmCert = func(context.Context, string) error { return nil }
	}
	if !a.enterWrite() {
		return DesktopShareStart{}, errShareFrozen
	}
	defer a.writeGate.RUnlock()
	a.startMu.Lock() // lock order: startMu, then a.mu
	defer a.startMu.Unlock()

	// Checked again under startMu: the GUI may have left, or another start may
	// have won, while the health check ran. releaseGUI frees the slot before it
	// asks for startMu, so a share created here is always ended afterwards.
	a.mu.Lock()
	if a.guiStream == nil {
		a.mu.Unlock()
		return DesktopShareStart{}, &ShareError{Code: shareErrGUIRequired, Detail: "the Portty window must be running to share"}
	}
	if a.guestShare != nil {
		link, err := a.guestLinkLocked()
		a.mu.Unlock()
		if err != nil {
			return DesktopShareStart{}, &ShareError{ShareErrStartFailed, err.Error()}
		}
		return DesktopShareStart{URL: link}, nil
	}
	var gs *desktop.GuestShare
	gs, err := desktop.NewGuestShare(nil, func(e desktop.GuestEvent) { a.onGuestEvent(gs, e) })
	if err != nil {
		a.mu.Unlock()
		return DesktopShareStart{}, &ShareError{ShareErrStartFailed, err.Error()}
	}
	a.guestShare, a.guestIAT = gs, time.Now().Unix()
	a.mu.Unlock()

	if serr := a.bringUpDesktopLocked(ctx, h, newServer, warmCert); serr != nil {
		a.abandonGuestShare(gs)
		return DesktopShareStart{}, serr
	}
	a.mu.RLock()
	link, err = a.guestLinkLocked()
	a.mu.RUnlock()
	if err != nil {
		a.abandonGuestShare(gs)
		return DesktopShareStart{}, &ShareError{ShareErrStartFailed, err.Error()}
	}
	first := a.recordCTDisclosure()
	a.pokeShare()
	return DesktopShareStart{URL: link, FirstShare: first}, nil
}

// abandonGuestShare undoes a start that failed after the share was installed:
// the share is dropped and a server this start brought up for nothing is
// stopped. Caller holds startMu.
func (a *API) abandonGuestShare(gs *desktop.GuestShare) {
	a.mu.Lock()
	if a.guestShare == gs {
		a.guestShare = nil
	}
	a.stopServerIfIdleLocked()
	a.mu.Unlock()
	gs.End(desktop.ReasonShareStopped)
}

func (a *API) currentGuestShare() *desktop.GuestShare {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.guestShare
}

func (a *API) handleShareAdmit(w http.ResponseWriter, r *http.Request) {
	a.answerRequest(w, r.PathValue("id"), true)
}

func (a *API) handleShareDeny(w http.ResponseWriter, r *http.Request) {
	a.answerRequest(w, r.PathValue("id"), false)
}

// answerRequest admits or denies one waiting guest. An unknown, answered or
// ended request is 404 and changes nothing.
func (a *API) answerRequest(w http.ResponseWriter, id string, admit bool) {
	gs := a.currentGuestShare()
	if gs == nil {
		writeShareErr(w, http.StatusNotFound, shareErrNoRequest, "")
		return
	}
	var err error
	if admit {
		err = gs.Admit(id)
	} else {
		err = gs.Deny(id)
	}
	if err != nil {
		writeShareErr(w, http.StatusNotFound, shareErrNoRequest, "")
		return
	}
	a.pokeShare()
	w.WriteHeader(http.StatusNoContent)
}

// handleStopDesktopShare ends the share. Always 204: ending a share that is not
// running is harmless.
func (a *API) handleStopDesktopShare(w http.ResponseWriter, _ *http.Request) {
	a.endGuestShare(desktop.ReasonShareStopped)
	w.WriteHeader(http.StatusNoContent)
}

// shareViewer finds a connected viewer by ID.
func (a *API) shareViewer(id string) (*desktop.Manager, desktop.Viewer, bool) {
	mgr := desktopAdapter{a}.manager()
	if mgr == nil {
		return nil, desktop.Viewer{}, false
	}
	for _, v := range mgr.ViewerList() {
		if string(v.ID) == id {
			return mgr, v, true
		}
	}
	return mgr, desktop.Viewer{}, false
}

// writeViewerErr maps the Manager's per-viewer errors to the route's answers.
func writeViewerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, desktop.ErrOwnViewer):
		writeShareErr(w, http.StatusConflict, shareErrOwnViewer, "")
	default:
		writeShareErr(w, http.StatusNotFound, shareErrNoViewer, "")
	}
}

// handleShareRights sets a guest's control and clipboard rights. An own-account
// viewer's rights are not the sharer's to change (409); an unknown viewer is 404.
func (a *API) handleShareRights(w http.ResponseWriter, r *http.Request) {
	var req DesktopRightsRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeShareErr(w, http.StatusBadRequest, shareErrBadRequest, "invalid request body")
		return
	}
	mgr, v, ok := a.shareViewer(r.PathValue("id"))
	switch {
	case !ok:
		writeShareErr(w, http.StatusNotFound, shareErrNoViewer, "")
		return
	case v.Mode != desktop.ModeGuest:
		writeShareErr(w, http.StatusConflict, shareErrOwnViewer, "")
		return
	}
	if err := mgr.SetRights(v.ID, req.Control, req.Clipboard); err != nil {
		writeViewerErr(w, err)
		return
	}
	a.pokeShare()
	w.WriteHeader(http.StatusNoContent)
}

// handleShareDisconnect cuts one viewer: a guest is revoked for the rest of the
// share, an own-account device is disconnected and may come back.
func (a *API) handleShareDisconnect(w http.ResponseWriter, r *http.Request) {
	mgr, v, ok := a.shareViewer(r.PathValue("id"))
	if !ok {
		writeShareErr(w, http.StatusNotFound, shareErrNoViewer, "")
		return
	}
	if err := mgr.CloseViewer(v.ID); err != nil {
		writeViewerErr(w, err)
		return
	}
	a.pokeShare()
	w.WriteHeader(http.StatusNoContent)
}

// closeOwnViewers closes every own-account viewer with code and leaves the
// guests alone (they belong to the share, not to always-available).
func (a *API) closeOwnViewers(mgr *desktop.Manager, code websocket.StatusCode, reason string) {
	for _, v := range mgr.ViewerList() {
		if v.Mode == desktop.ModeOwn {
			_ = mgr.CloseViewerWith(v.ID, code, reason)
		}
	}
}

// --- how a share ends --------------------------------------------------------

// endGuestShare ends the guest share, if there is one, for reason (a
// desktop.Reason* value): the share ID is dropped first, so the link stops
// verifying, then the share is ended, the guests are cut (they see 4433
// share_ended), and the Manager and the server stop only when nothing else
// needs them. Own-account viewers are untouched. Safe to call twice; it reports
// whether there was a share to end. Lock order: startMu, then a.mu.
func (a *API) endGuestShare(reason string) bool {
	a.startMu.Lock()
	defer a.startMu.Unlock()
	a.mu.Lock()
	gs, mgr := a.guestShare, a.desktopMgr
	if gs == nil {
		a.mu.Unlock()
		return false
	}
	a.guestShare = nil
	a.mu.Unlock()

	gs.End(reason)
	if mgr != nil {
		mgr.CloseGuests(desktop.CloseShareEnded, "share_ended")
	}

	a.pushShareEvent(desktop.AuditShareStop, nil, reason)
	a.pokeShare()

	// Always-available keeps the Manager; otherwise it was only here for the share.
	a.mu.Lock()
	var stop *desktop.Manager
	if a.desktopMgr != nil && a.desktopMgr == mgr && !a.desktopCfg.AlwaysAvailable {
		stop, a.desktopMgr = mgr, nil
		a.desktopBuiltIP, a.desktopBuiltDomain = "", ""
	}
	a.stopServerIfIdleLocked()
	a.mu.Unlock()
	if stop != nil {
		// CloseGuests returns once the guests' input and media are cut; their
		// socket closes run on their own goroutines. Closing the Manager now would
		// put its own close code ahead of share_ended, so wait for the guests to
		// leave (a silent peer is dropped after the Manager's 2 s ceiling).
		waitGuestsGone(stop, guestLeaveWait)
		stop.CloseAll(desktop.CloseDesktopOff, "desktop off")
		_ = stop.Close()
	}
	return true
}

// guestLeaveWait bounds waitGuestsGone: longer than the Manager's close ceiling.
const guestLeaveWait = 3 * time.Second

// waitGuestsGone waits until no guest is connected to mgr, or for up to d.
func waitGuestsGone(mgr *desktop.Manager, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		gone := true
		for _, v := range mgr.ViewerList() {
			if v.Mode == desktop.ModeGuest {
				gone = false
				break
			}
		}
		if gone {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// --- what the guest routes see -----------------------------------------------

// GuestShareID is the running share's ID; none means the guest link is off.
func (d desktopAdapter) GuestShareID() (string, bool) {
	gs := d.a.currentGuestShare()
	if gs == nil {
		return "", false
	}
	return gs.ID, true
}

// ServeGuestViewer runs one guest's signalling session. No share or no Manager
// is the share having ended: the guest is told so and the socket closes.
func (d desktopAdapter) ServeGuestViewer(ctx context.Context, c *websocket.Conn, admit func() bool, who webserver.PeerIdentity) {
	gs, m := d.a.currentGuestShare(), d.manager()
	if gs == nil || m == nil {
		_ = c.Close(desktop.CloseShareEnded, "share_ended")
		return
	}
	m.ServeGuestViewer(ctx, c, admit, gs, desktop.NewIdentity(who.UserID, who.LoginName, who.DisplayName, who.NodeName, who.StableID, who.IP))
}

// --- Manager hooks -----------------------------------------------------------

// wireGuestDeps hands the Manager what the guest share needs from the daemon:
// the join and leave hooks and the node identity of a media peer.
func (a *API) wireGuestDeps(d *desktop.Deps) {
	d.OnViewerJoin, d.OnViewerLeave, d.PeerIdentity = a.onViewerJoin, a.onViewerLeave, a.peerIdentity
}

func (a *API) onViewerJoin(v desktop.Viewer) {
	view := viewerView(v)
	a.pushShareEvent(desktop.AuditJoin, &view, "")
	a.pokeShare()
}

func (a *API) onViewerLeave(v desktop.Viewer) {
	view := viewerView(v)
	a.pushShareEvent(desktop.AuditLeave, &view, "")
	a.pokeShare()
}

// onGuestEvent receives every admission event of share gs.
func (a *API) onGuestEvent(gs *desktop.GuestShare, e desktop.GuestEvent) {
	if e.Kind == desktop.AuditAdmitRequest {
		req := desktop.Request{ID: e.RequestID, Ident: e.Ident, Since: time.Now()}
		for _, p := range gs.Pending() {
			if p.ID == e.RequestID {
				req = p
			}
		}
		view := DesktopShareViewer(pendingView(req))
		a.pushShareEvent(desktop.AuditAdmitRequest, &view, "")
	}
	a.pokeShare()
}

// peerIdentity is Deps.PeerIdentity: tailscaled's answer for a media peer's
// address, through the share server's WhoIs.
func (a *API) peerIdentity(ctx context.Context, addr string) (int64, string, error) {
	a.mu.RLock()
	ws := a.webServer
	a.mu.RUnlock()
	if ws == nil {
		return 0, "", errors.New("the share server is not running")
	}
	return ws.LookupPeer(ctx, addr)
}
