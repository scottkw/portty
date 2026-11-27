package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/webserver"
)

// desktopPollEvery is how often the health poller looks at Tailscale while
// always-available is on: it brings the share server and the Manager up at
// daemon start, retries while Tailscale is down, and follows an address change.
const desktopPollEvery = 10 * time.Second

// Production seams, copied into each API by NewAPI (SetDesktopSeamsForTest).
var (
	// defaultInterfaceAddrs lists the machine's interface addresses; the tailnet
	// address must be one of them for the media socket to bind (not userspace).
	defaultInterfaceAddrs = net.InterfaceAddrs
	// defaultNewDesktopSource is the real ScreenCaptureKit capture; it refuses
	// to run under a test binary.
	defaultNewDesktopSource = screencap.NewDisplaySource
)

// desktopTestDeps, when set (by TestMain only), adjusts the Manager's Deps
// before it is built, so daemon tests never read the host's lock, display or
// power state. nil in production.
var desktopTestDeps func(*desktop.Deps)

// desktopNotifier posts a native notification for the Manager (19-11 sets it
// from the daemon, 19-14 adds the GUI side). Read at call time, so setting it
// after the Manager started still takes effect.
var (
	desktopNotifierMu sync.RWMutex
	desktopNotifier   func(id, title, body string)
)

// SetDesktopNotifier sets the function the desktop uses to post notifications.
func SetDesktopNotifier(fn func(id, title, body string)) {
	desktopNotifierMu.Lock()
	desktopNotifier = fn
	desktopNotifierMu.Unlock()
}

func notifyDesktop(id, title, body string) {
	desktopNotifierMu.RLock()
	fn := desktopNotifier
	desktopNotifierMu.RUnlock()
	if fn != nil {
		fn(id, title, body)
	}
}

// SetDesktopSeamsForTest replaces the interface list, the capture source
// factory and the media UDP port (0 keeps the default). Test-only.
func (a *API) SetDesktopSeamsForTest(
	interfaceAddrs func() ([]net.Addr, error),
	newSource func() screencap.Source,
	mediaPort int,
) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.interfaceAddrs, a.newDesktopSource, a.desktopMediaPort = interfaceAddrs, newSource, mediaPort
}

// DesktopStatusResponse is GET /desktop. Tailscale is the last verdict the
// daemon reached ("ok", "off", "https_off", "userspace"; "" before the first
// check). Launchd is whether launchd started this daemon: false means the
// desktop's unattended guarantee is degraded (a fallback-started daemon), which
// the host shows as needs repair. Viewers are told nothing new.
type DesktopStatusResponse struct {
	AlwaysAvailable  bool   `json:"alwaysAvailable"`
	State            string `json:"state"`
	Viewers          int    `json:"viewers"`
	Locked           bool   `json:"locked"`
	ApproachingLapse bool   `json:"approachingLapse"`
	LapseSeq         int    `json:"lapseSeq"`
	LapseClaimedSeq  int    `json:"lapseClaimedSeq"`
	Tailscale        string `json:"tailscale"`
	Launchd          bool   `json:"launchd"`
	Reason           string `json:"reason,omitempty"`
	// Surface and Control are additive (Phase 20) and only set while viewers are
	// connected: what is shared, and "ok" or "needs_accessibility" for remote
	// control. Older readers ignore both.
	Surface *DesktopSurface `json:"surface,omitempty"`
	Control string          `json:"control,omitempty"`
}

// DesktopSurface is the shared surface as the host shows it: Kind is "display"
// or "window", Name is the picker's name for it.
type DesktopSurface struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// DesktopRequest is the PUT /desktop body.
type DesktopRequest struct {
	AlwaysAvailable bool `json:"alwaysAvailable"`
}

// DesktopResponse answers PUT /desktop. FirstShare is true exactly when this
// call recorded the Certificate Transparency acknowledgement.
type DesktopResponse struct {
	AlwaysAvailable bool   `json:"alwaysAvailable"`
	URL             string `json:"url,omitempty"`
	FirstShare      bool   `json:"firstShare,omitempty"`
}

// serverNeededLocked is the share server's lifetime rule: it runs exactly while
// any terminal share or the always-available desktop needs it. Every place that
// stops the server for lack of use asks this. Caller holds a.mu.
func (a *API) serverNeededLocked() bool {
	return len(a.shares) > 0 || a.desktopCfg.AlwaysAvailable || a.guestShare != nil
}

// extraDesktopRoutes lets a build-tagged file add routes without touching the
// release code (desktop_vmdesktop.go, VM-only). It is empty in every release build.
var extraDesktopRoutes []func(*API, *http.ServeMux)

func (a *API) registerDesktopRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /desktop", a.handleGetDesktop)
	mux.HandleFunc("PUT /desktop", a.handlePutDesktop)
	mux.HandleFunc("GET /desktop/link", a.handleGetDesktopLink)
	mux.HandleFunc("GET /desktop/permissions", a.handleDesktopPermissions)
	mux.HandleFunc("POST /desktop/test-capture", a.handleDesktopTestCapture)
	mux.HandleFunc("POST /desktop/lapse-claim", a.handleDesktopLapseClaim)
	a.registerDesktopShareRoutes(mux)
	for _, add := range extraDesktopRoutes {
		add(a, mux)
	}
}

func (a *API) handleGetDesktop(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	cfg, cfgErr, mgr, ts := a.desktopCfg, a.desktopErr, a.desktopMgr, a.desktopTS
	a.mu.RUnlock()
	if mgr != nil && r.URL.Query().Get("gui") == "1" {
		mgr.NoteGUISeen() // the GUI's 5 s tray poll: the lapse sender rule reads this
	}
	resp := DesktopStatusResponse{State: string(desktop.StateOff), Tailscale: ts, Launchd: launchdManaged}
	if cfgErr != nil {
		resp.Reason = cfgErr.Error()
	}
	if cfg.AlwaysAvailable {
		resp.AlwaysAvailable = true
		resp.State = string(desktop.StateIdle)
		if mgr != nil {
			st := mgr.Status()
			resp.State, resp.Viewers, resp.Locked = string(st.State), st.Viewers, st.Locked
			resp.ApproachingLapse, resp.LapseSeq = st.ApproachingLapse, st.LapseSeq
			resp.LapseClaimedSeq = st.LapseClaimedSeq
			if st.Reason != "" {
				resp.Reason = st.Reason
			}
			// The control state is measured by effect and only moves while a viewer
			// sends input, so with nobody watching there is nothing true to report.
			if st.Viewers > 0 {
				info := mgr.CurrentSurface()
				resp.Surface = &DesktopSurface{Kind: "display", Name: info.Name}
				if info.Kind == screencap.SurfaceWindow {
					resp.Surface.Kind = "window"
				}
				resp.Control, _ = mgr.ControlState()
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// lapseClaimRequest is the POST /desktop/lapse-claim body.
type lapseClaimRequest struct {
	LapseSeq int `json:"lapseSeq"`
}

// LapseClaimResponse answers it: Claimed is true for exactly one caller per
// lapse, so only that caller posts the notification.
type LapseClaimResponse struct {
	Claimed bool `json:"claimed"`
}

// handleDesktopLapseClaim lets the running GUI take the right to notify a lapse
// (CONTEXT: the GUI when it runs, otherwise the daemon). With no Manager nothing
// is claimed.
func (a *API) handleDesktopLapseClaim(w http.ResponseWriter, r *http.Request) {
	var req lapseClaimRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	a.mu.RLock()
	mgr := a.desktopMgr
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, LapseClaimResponse{Claimed: mgr != nil && mgr.ClaimLapse(req.LapseSeq, "gui")})
}

func (a *API) handlePutDesktop(w http.ResponseWriter, r *http.Request) {
	var req DesktopRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	var (
		resp DesktopResponse
		serr *ShareError
	)
	if req.AlwaysAvailable {
		resp, serr = a.desktopOn(r.Context())
	} else {
		serr = a.desktopOff()
	}
	if serr == errShareFrozen {
		writeFrozen(w)
		return
	}
	if serr != nil {
		writeJSON(w, shareStatus(serr.Code), ShareErrorResponse{Error: serr.Code, Detail: serr.Detail})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleGetDesktopLink(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	link, err := a.desktopLinkLocked()
	a.mu.RUnlock()
	if err != nil {
		writeJSON(w, http.StatusConflict, ShareErrorResponse{Error: "off", Detail: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": link})
}

var errDesktopNoLink = errors.New("the desktop is off or its server is not running")

// desktopLinkLocked is the always-available link. The same grant ID, issued-at
// and signing key give the same text after every restart (HMAC over the
// marshalled claims), so a saved link keeps working. Caller holds a.mu.
func (a *API) desktopLinkLocked() (string, error) {
	ws, cfg := a.webServer, a.desktopCfg
	if ws == nil || !cfg.AlwaysAvailable {
		return "", errDesktopNoLink
	}
	tok, err := capability.SignDesktop(capability.DesktopClaims{
		Kind: capability.DesktopTokenKind, Mode: capability.DesktopModeAlways,
		GrantID: cfg.GrantID, IAT: cfg.IAT, V: 1,
	}, a.CurrentSigningKey())
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(ws.BaseURL(), "/") + "/desktop/?cap=" + url.QueryEscape(tok), nil
}

// desktopAdapter is what the share server's /desktop/ routes see: the daemon's
// current grant, and the current Manager (none while off).
type desktopAdapter struct{ a *API }

func (a *API) desktopBackend() webserver.DesktopBackend { return desktopAdapter{a} }

func (d desktopAdapter) DesktopGrant() (string, bool) {
	d.a.mu.RLock()
	defer d.a.mu.RUnlock()
	c := d.a.desktopCfg
	return c.GrantID, c.AlwaysAvailable && c.GrantID != ""
}

func (d desktopAdapter) manager() *desktop.Manager {
	d.a.mu.RLock()
	defer d.a.mu.RUnlock()
	return d.a.desktopMgr
}

func (d desktopAdapter) ServeDesktopViewer(ctx context.Context, c *websocket.Conn, admit func() bool) {
	m := d.manager()
	if m == nil {
		// On with no Manager yet (a handoff or a poller retry in progress) is not
		// off: 4410 is final for the viewer, 1013 makes it retry.
		if _, on := d.DesktopGrant(); on {
			_ = c.Close(websocket.StatusTryAgainLater, "desktop starting")
			return
		}
		_ = c.Close(desktop.CloseDesktopOff, "desktop off")
		return
	}
	m.ServeDesktopViewer(ctx, c, admit)
}

// Full lets the route answer 429 before any upgrade.
func (d desktopAdapter) Full() bool {
	m := d.manager()
	return m != nil && m.Full()
}

// tailnetAddrLocal reports whether ip is assigned to a local interface. With
// Tailscale in userspace networking it is not, and nothing can bind it.
func tailnetAddrLocal(ifaceAddrs func() ([]net.Addr, error), ip string) (bool, error) {
	want, err := netip.ParseAddr(ip)
	if err != nil {
		return false, err
	}
	addrs, err := ifaceAddrs()
	if err != nil {
		return false, err
	}
	for _, ad := range addrs {
		var got net.IP
		switch v := ad.(type) {
		case *net.IPNet:
			got = v.IP
		case *net.IPAddr:
			got = v.IP
		}
		if g, ok := netip.AddrFromSlice(got); ok && g.Unmap() == want.Unmap() {
			return true, nil
		}
	}
	return false, nil
}

func (a *API) setDesktopTS(ts string) {
	a.mu.Lock()
	a.desktopTS = ts
	a.mu.Unlock()
}

func (a *API) desktopTSValue() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.desktopTS
}

// desktopTailnet runs the health check the desktop needs and records the
// verdict for GET /desktop. The refusals are shareOn's two plus userspace.
func (a *API) desktopTailnet(
	ctx context.Context,
	health func(context.Context) webserver.TailscaleHealth,
	ifaceAddrs func() ([]net.Addr, error),
) (webserver.TailscaleHealth, *ShareError) {
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	h := health(hctx)
	cancel()
	if !h.Connected || h.IP == "" {
		a.setDesktopTS("off")
		return h, &ShareError{ShareErrTailscaleOff, "Tailscale isn't connected"}
	}
	if !h.HasCerts || h.Domain == "" {
		a.setDesktopTS("https_off")
		return h, &ShareError{ShareErrHTTPSOff, "HTTPS certificates are off for your tailnet"}
	}
	local, err := tailnetAddrLocal(ifaceAddrs, h.IP)
	if err != nil {
		return h, &ShareError{ShareErrStartFailed, "couldn't list this Mac's network addresses: " + err.Error()}
	}
	if !local {
		a.setDesktopTS("userspace")
		return h, &ShareError{ShareErrUserspace, "Tailscale is in userspace networking mode, so the desktop can't be reached"}
	}
	a.setDesktopTS("ok")
	return h, nil
}

// desktopOn turns always-available on: writes desktop.json, brings the share
// server and the Manager up on the tailnet address and returns the link. A
// refusal (Tailscale off, HTTPS off, userspace) changes nothing at all. a.mu is
// never held across the health check, the warm-up or any network work.
func (a *API) desktopOn(ctx context.Context) (DesktopResponse, *ShareError) {
	a.mu.RLock()
	health, newServer, warmCert, ifaceAddrs := a.shareHealth, a.newShareServer, a.warmCert, a.interfaceAddrs
	cfgErr, running := a.desktopErr, a.webServer != nil
	a.mu.RUnlock()
	if a.engine.configDir == "" {
		return DesktopResponse{}, &ShareError{ShareErrStartFailed, "no config directory"}
	}
	if cfgErr != nil { // never replaced: the user decides what happens to that file
		return DesktopResponse{}, &ShareError{ShareErrStartFailed, cfgErr.Error()}
	}
	h, serr := a.desktopTailnet(ctx, health, ifaceAddrs)
	if serr != nil {
		return DesktopResponse{}, serr
	}
	if !running { // as shareOn: the first warm-up can take seconds and moves nothing
		if serr := warmShareCert(ctx, warmCert, h.Domain); serr != nil {
			return DesktopResponse{}, serr
		}
		warmCert = func(context.Context, string) error { return nil }
	}
	if !a.enterWrite() {
		return DesktopResponse{}, errShareFrozen
	}
	defer a.writeGate.RUnlock()
	a.startMu.Lock() // lock order: startMu, then a.mu
	defer a.startMu.Unlock()

	// The setting is saved before anything starts, so a terminal share turned
	// off meanwhile finds the server needed (serverNeededLocked).
	a.mu.Lock()
	wasOn := a.desktopCfg.AlwaysAvailable
	if !wasOn {
		id, err := desktop.NewGrantID()
		if err == nil {
			cfg := desktop.Config{SchemaVersion: desktop.ConfigSchemaVersion, AlwaysAvailable: true, GrantID: id, IAT: time.Now().Unix()}
			if err = desktop.SaveConfig(a.engine.configDir, cfg); err == nil {
				a.desktopCfg = cfg
			}
		}
		if err != nil {
			a.mu.Unlock()
			return DesktopResponse{}, &ShareError{ShareErrStartFailed, "couldn't save desktop.json: " + err.Error()}
		}
	}
	a.mu.Unlock()

	if serr := a.bringUpDesktopLocked(ctx, h, newServer, warmCert); serr != nil {
		if !wasOn {
			if err := a.disableDesktop(); err != nil {
				fmt.Fprintf(os.Stderr, "daemon: desktop.json rollback: %v\n", err)
			}
		}
		return DesktopResponse{}, serr
	}
	first := a.recordCTDisclosure()
	a.mu.RLock()
	link, err := a.desktopLinkLocked()
	a.mu.RUnlock()
	if err != nil {
		return DesktopResponse{}, &ShareError{ShareErrStartFailed, err.Error()}
	}
	return DesktopResponse{AlwaysAvailable: true, URL: link, FirstShare: first}, nil
}

// bringUpDesktopLocked makes the share server and the Manager run for h,
// restarting both first when they were built for another address or name.
// Caller holds the write gate and startMu.
func (a *API) bringUpDesktopLocked(
	ctx context.Context,
	h webserver.TailscaleHealth,
	newServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error),
	warmCert func(context.Context, string) error,
) *ShareError {
	a.mu.Lock()
	var old *desktop.Manager
	if a.desktopMovedLocked(h) {
		old, a.desktopMgr = a.desktopMgr, nil
		a.desktopBuiltIP, a.desktopBuiltDomain, a.shareBuiltIP, a.shareBuiltDomain = "", "", "", ""
		a.stopServerLocked()
	}
	haveServer, haveMgr := a.webServer != nil, a.desktopMgr != nil
	a.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	if !haveServer {
		if serr := a.startShareServer(ctx, h, newServer, warmCert); serr != nil {
			return serr
		}
	}
	if !haveMgr {
		return a.startDesktopManager(h)
	}
	return nil
}

// desktopMovedLocked: the Manager or the server was built for an address or
// name other than h's. Unknown (empty) never counts as moved, so a server a
// handoff installed is not restarted. Caller holds a.mu.
func (a *API) desktopMovedLocked(h webserver.TailscaleHealth) bool {
	if a.desktopMgr != nil && (a.desktopBuiltIP != h.IP || a.desktopBuiltDomain != h.Domain) {
		return true
	}
	return a.webServer != nil && a.shareBuiltIP != "" && (a.shareBuiltIP != h.IP || a.shareBuiltDomain != h.Domain)
}

// startDesktopManager builds and starts the Manager on h's tailnet address.
// Caller holds startMu.
func (a *API) startDesktopManager(h webserver.TailscaleHealth) *ShareError {
	addr, err := netip.ParseAddr(h.IP)
	if err != nil {
		return &ShareError{ShareErrStartFailed, "bad tailnet address: " + err.Error()}
	}
	a.mu.RLock()
	newSource, port := a.newDesktopSource, a.desktopMediaPort
	a.mu.RUnlock()
	if port == 0 {
		port = desktop.MediaPortDefault
	}
	// A busy port answers at once. Manager.Start would retry for 30 s (the
	// upgrade case), and it keeps its context as the capture's lifetime, so it
	// cannot be given a short one.
	if c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: addr.AsSlice(), Port: port}); err == nil {
		_ = c.Close()
	} else if errors.Is(err, syscall.EADDRINUSE) {
		return &ShareError{ShareErrPortInUse, fmt.Sprintf("UDP port %d is in use", port)}
	}
	deps := desktop.Deps{
		Bind: addr, MediaPort: port, NewSource: newSource, SourceConfig: screencap.DefaultConfig(),
		ConfigDir: a.engine.configDir, Notify: notifyDesktop,
	}
	a.wireGuestDeps(&deps)
	if desktopTestDeps != nil {
		desktopTestDeps(&deps)
	}
	mgr, err := desktop.NewManager(deps)
	if err != nil {
		return &ShareError{ShareErrStartFailed, err.Error()}
	}
	if err := mgr.Start(a.desktopCtx); err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return &ShareError{ShareErrPortInUse, fmt.Sprintf("UDP port %d is in use", port)}
		}
		return &ShareError{ShareErrStartFailed, err.Error()}
	}
	a.mu.Lock()
	a.desktopMgr, a.desktopBuiltIP, a.desktopBuiltDomain = mgr, h.IP, h.Domain
	a.mu.Unlock()
	return nil
}

// desktopOff turns always-available off. The grant is cleared, every desktop
// viewer is closed with 4410, capture stops, and the share server stops only if
// no terminal share needs it.
func (a *API) desktopOff() *ShareError {
	if !a.enterWrite() {
		return errShareFrozen
	}
	defer a.writeGate.RUnlock()
	a.startMu.Lock()
	defer a.startMu.Unlock()
	a.mu.RLock()
	idle := !a.desktopCfg.AlwaysAvailable && (a.desktopMgr == nil || a.guestShare != nil)
	a.mu.RUnlock()
	if idle { // already off (or a file the user must look at): nothing to clear
		return nil
	}
	if err := a.disableDesktop(); err != nil {
		return &ShareError{ShareErrStartFailed, "couldn't save desktop.json: " + err.Error()}
	}
	return nil
}

// disableDesktop clears the grant in memory and in desktop.json, stops the
// Manager (viewers get 4410) and stops the server if nothing else needs it.
// A failed save is returned but the revoke still happens: off was asked for.
// Caller holds startMu.
func (a *API) disableDesktop() error {
	a.mu.Lock()
	err := desktop.SaveConfig(a.engine.configDir, desktop.Config{})
	a.desktopCfg = desktop.Config{}
	mgr := a.desktopMgr
	guest := a.guestShare != nil
	if !guest { // a running guest share keeps the Manager (and, by serverNeededLocked, the server)
		a.desktopMgr, a.desktopBuiltIP, a.desktopBuiltDomain = nil, "", ""
	}
	a.stopServerIfIdleLocked()
	a.mu.Unlock()
	switch {
	case mgr == nil:
	case guest:
		a.closeOwnViewers(mgr, desktop.CloseDesktopOff, "desktop off")
	default:
		mgr.CloseAll(desktop.CloseDesktopOff, "desktop off")
		_ = mgr.Close()
	}
	return err
}

// stopDesktop ends the poller and the Manager at daemon shutdown. The setting
// stays as it is on disk.
func (a *API) stopDesktop() {
	a.endGuestShare(desktop.ReasonDaemonExit)
	a.desktopCancel()
	a.mu.Lock()
	mgr := a.desktopMgr
	a.desktopMgr = nil
	a.mu.Unlock()
	if mgr != nil {
		_ = mgr.Close()
	}
}

// startDesktopPoller starts runDesktopPoller once, from Start and from the
// takeover daemon's serveOn (the two ways a daemon begins serving).
func (a *API) startDesktopPoller() {
	a.desktopPollOnce.Do(func() { go a.runDesktopPoller(a.desktopCtx) })
}

// runDesktopPoller brings the always-available desktop up at daemon start and
// keeps it on the current tailnet address, with no GUI: an immediate tick, then
// one every desktopPollEvery until ctx ends.
func (a *API) runDesktopPoller(ctx context.Context) {
	a.desktopTick(ctx)
	t := time.NewTicker(a.desktopPollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.desktopTick(ctx)
		}
	}
}

// desktopTick is one poll. With always-available off it only rechecks a
// recorded Tailscale refusal. a.mu is never held across health, cert or network work.
func (a *API) desktopTick(ctx context.Context) {
	defer func() { // a bug here must not take the daemon down
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "daemon: desktop poller recovered: %T\n", r)
		}
	}()
	if a.frozen.Load() || a.committed.Load() || ctx.Err() != nil {
		return
	}
	a.mu.RLock()
	on, running := a.desktopCfg.AlwaysAvailable || a.guestShare != nil, a.webServer != nil
	health, newServer, warmCert, ifaceAddrs := a.shareHealth, a.newShareServer, a.warmCert, a.interfaceAddrs
	a.mu.RUnlock()
	if !on {
		// A refusal recorded at a turn-on attempt (userspace, Tailscale off,
		// HTTPS off) is rechecked so a fixed Tailscale unblocks the switch
		// within one poll; with no refusal recorded, off asks nothing.
		if ts := a.desktopTSValue(); ts != "" && ts != "ok" {
			_, _ = a.desktopTailnet(ctx, health, ifaceAddrs)
		}
		return
	}
	h, serr := a.desktopTailnet(ctx, health, ifaceAddrs)
	if serr != nil {
		return
	}
	if a.desktopUpToDate(h) {
		return
	}
	if !running { // as desktopOn: the warm-up can take seconds, so it runs outside the write gate
		if serr := warmShareCert(ctx, warmCert, h.Domain); serr != nil {
			fmt.Fprintf(os.Stderr, "daemon: desktop not started: %s\n", serr.Code)
			return
		}
		warmCert = func(context.Context, string) error { return nil }
	}
	if !a.enterWrite() {
		return
	}
	defer a.writeGate.RUnlock()
	a.startMu.Lock()
	defer a.startMu.Unlock()
	if !a.desktopWanted() || a.desktopUpToDate(h) { // changed while we waited
		return
	}
	if serr := a.bringUpDesktopLocked(ctx, h, newServer, warmCert); serr != nil {
		fmt.Fprintf(os.Stderr, "daemon: desktop not started: %s\n", serr.Code)
	}
}

func (a *API) desktopWanted() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.desktopCfg.AlwaysAvailable || a.guestShare != nil
}

// desktopUpToDate: the server and the Manager run and match h.
func (a *API) desktopUpToDate(h webserver.TailscaleHealth) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.webServer != nil && a.desktopMgr != nil && !a.desktopMovedLocked(h)
}

// Onboarding endpoints (19-11). Both run in the daemon, the process that holds
// the Screen Recording grant: the GUI's own preflight and capture would answer
// for the GUI's bundle, not for the one that streams.

const (
	// testCaptureMaxLongEdge bounds the Test Capture thumbnail (it crosses the
	// socket and is shown at 256 x 160).
	testCaptureMaxLongEdge = 512
	// testCaptureCeiling is how long a Test Capture may take: a macOS 27.2 dialog
	// can hold a capture, and the sheet must not wait for ever.
	testCaptureCeiling = 15 * time.Second
)

// Seams, package-level so the tests restore them (SetDesktopPermsForTest and
// SetDesktopCaptureForTest).
var (
	// desktopProber is the shared fresh-process reader: overlapping asks (the
	// sheet polling, the state loop) share one child spawn.
	desktopProber = desktop.DefaultProber()
	// defaultCaptureStill is the real JPEG still; it refuses under a test binary.
	defaultCaptureStill = screencap.CaptureStill
	captureCeiling      = testCaptureCeiling
)

// SetDesktopPermsForTest replaces the permission reader; the returned func restores it.
func SetDesktopPermsForTest(p *desktop.Prober) func() {
	old := desktopProber
	desktopProber = p
	return func() { desktopProber = old }
}

// SetDesktopCaptureForTest replaces the still capture and its ceiling (0 keeps
// the default); the returned func restores both.
func SetDesktopCaptureForTest(fn func(ctx context.Context, maxLongEdge int) ([]byte, error), ceiling time.Duration) func() {
	oldFn, oldCeil := defaultCaptureStill, captureCeiling
	defaultCaptureStill = fn
	if ceiling > 0 {
		captureCeiling = ceiling
	}
	return func() { defaultCaptureStill, captureCeiling = oldFn, oldCeil }
}

// DesktopPermissionsResponse is GET /desktop/permissions. Accessibility is
// reported because the sheet shows it; the pointer-event read stays internal.
type DesktopPermissionsResponse struct {
	SR bool `json:"sr"`
	AX bool `json:"ax"`
}

// Codes of the onboarding endpoints' error bodies, {"code": ...}.
const (
	codeProbeFailed     = "probe_failed"
	codeNeedsPermission = "needs_permission"
	codeTimeout         = "timeout"
	codeCaptureFailed   = "capture_failed"
)

func writeCode(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code})
}

// handleDesktopPermissions answers the live grants. A probe failure is an
// explicit error, never a default false: "not granted" must mean the OS said so
// (research C1).
func (a *API) handleDesktopPermissions(w http.ResponseWriter, r *http.Request) {
	p, err := desktopProber.Read(r.Context())
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: desktop permission probe failed: %v\n", err)
		writeCode(w, http.StatusServiceUnavailable, codeProbeFailed)
		return
	}
	// The live state follows the sheet's poll, not the 5-minute idle probe, so
	// Settings and the tray stop saying Needs permission once it is granted.
	a.mu.RLock()
	mgr := a.desktopMgr
	a.mu.RUnlock()
	if mgr != nil {
		mgr.RefreshPermissions(r.Context())
	}
	writeJSON(w, http.StatusOK, DesktopPermissionsResponse{SR: p.SR, AX: p.AX})
}

// handleDesktopTestCapture takes one still of the main display and returns it
// as a JPEG. A success records the approval and clears the crash-loop breaker:
// a capture that works is the evidence that clears it. The picture is never
// written to disk or logged (T-19-39).
func (a *API) handleDesktopTestCapture(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), captureCeiling)
	defer cancel()
	jpeg, err := defaultCaptureStill(ctx, testCaptureMaxLongEdge)
	switch {
	case err == nil:
	case errors.Is(err, screencap.ErrPermissionDenied):
		writeCode(w, http.StatusConflict, codeNeedsPermission)
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeCode(w, http.StatusGatewayTimeout, codeTimeout)
		return
	default:
		fmt.Fprintf(os.Stderr, "daemon: desktop test capture failed: %v\n", err)
		writeCode(w, http.StatusInternalServerError, codeCaptureFailed)
		return
	}
	if err := desktop.NoteTestCapture(a.engine.configDir, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: desktop test capture not recorded: %v\n", err)
	}
	a.mu.RLock()
	mgr := a.desktopMgr
	a.mu.RUnlock()
	if mgr != nil {
		_ = mgr.ResetBreaker()
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(jpeg)
}
