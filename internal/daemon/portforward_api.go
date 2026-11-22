package daemon

// POST /peers/ports (unix socket only): a click on a remote session's
// `localhost` URL asks the viewing daemon to open a loopback forward on the
// same port. The daemon asks the host for a port grant first, then asks the
// user (consent, per Mac and port), then binds. See portforward.go for the
// forward itself and the Phase 13 interface contract in 13-02-PLAN.md.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
)

// PortOpenRequest is the body of POST /peers/ports.
type PortOpenRequest struct {
	SessionID string `json:"sessionId"`
	URL       string `json:"url"`
	Consent   bool   `json:"consent"`
}

// PortOpenResponse is the 200 body: open (URL set, a forward is listening) or
// consent (nothing bound). Host is the MagicDNS short label of the Mac the
// daemon will contact.
type PortOpenResponse struct {
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

// PortOpenErrorResponse is the error body.
type PortOpenErrorResponse struct {
	Error string `json:"error"`
}

// PortOpenResponse.Status values.
const (
	PortStatusOpen    = "open"
	PortStatusConsent = "consent"
)

// PortOpenErrorResponse.Error codes. PortErrDaemonOutdated is for the client
// (13-04); this route never sends it.
const (
	PortErrBadURL         = "bad_url"
	PortErrNoCap          = "no_cap"
	PortErrRefused        = "refused"
	PortErrHostOutdated   = "host_outdated"
	PortErrPortInUse      = "port_in_use"
	PortErrUnreachable    = "unreachable"
	PortErrDaemonOutdated = "daemon_outdated"
)

// portErrStatus maps an error code to its HTTP status.
var portErrStatus = map[string]int{
	PortErrBadURL:       http.StatusBadRequest,
	PortErrNoCap:        http.StatusNotFound,
	PortErrRefused:      http.StatusForbidden,
	PortErrHostOutdated: http.StatusConflict,
	PortErrPortInUse:    http.StatusConflict,
	PortErrUnreachable:  http.StatusBadGateway,
}

// portOutcome maps an error code to its log outcome.
var portOutcome = map[string]string{
	PortErrBadURL:       "bad-url",
	PortErrNoCap:        "no-cap",
	PortErrRefused:      "refused",
	PortErrHostOutdated: "host-outdated",
	PortErrPortInUse:    "port-in-use",
	PortErrUnreachable:  "unreachable",
}

// logPortOutcome writes the one line per outcome. It never carries a request
// target (Vite's HMR URL holds a token), a capability, a port token or a host.
func logPortOutcome(sid string, port int, outcome string) {
	log.Printf("port forward: session=%s port=%d outcome=%s", relay.DiagID(sid), port, outcome)
}

// classifyLoopbackURL is the shared click classifier (pinned by
// frontend/src/lib/__tests__/fixtures/localhost-url.json). It accepts
// http://{localhost,127.0.0.1,[::1],0.0.0.0,[::]}:<port> plus rest, after
// trimming trailing punctuation a terminal prints around a URL.
func classifyLoopbackURL(raw string) (port int, rest string, ok bool) {
	for raw != "" {
		last := raw[len(raw)-1]
		if strings.IndexByte("]}>,;.'\"", last) >= 0 ||
			(last == ')' && strings.Count(raw, "(") < strings.Count(raw, ")")) {
			raw = raw[:len(raw)-1]
			continue
		}
		break
	}
	s, ok := strings.CutPrefix(raw, "http://")
	if !ok {
		return 0, "", false
	}
	matched := false
	for _, h := range []string{"localhost", "127.0.0.1", "[::1]", "0.0.0.0", "[::]"} {
		if after, cut := strings.CutPrefix(s, h+":"); cut {
			s, matched = after, true
			break
		}
	}
	if !matched {
		return 0, "", false
	}
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	if n == 0 || n > 5 || s[0] == '0' {
		return 0, "", false
	}
	port, _ = strconv.Atoi(s[:n])
	if port < 1 || port > 65535 {
		return 0, "", false
	}
	rest = s[n:]
	if rest != "" && !strings.ContainsRune("/?#", rune(rest[0])) {
		return 0, "", false
	}
	for _, r := range rest {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return 0, "", false
		}
	}
	return port, rest, true
}

// openedURL is the URL the click opens: this Mac's localhost on the same port.
func openedURL(port int, rest string) (string, error) {
	if rest == "" {
		rest = "/"
	}
	u, err := url.Parse("http://localhost:" + strconv.Itoa(port) + rest)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// requestPortGrant asks the host for a port token. A non-empty code is the
// PortErr* answer.
func (a *API) requestPortGrant(ctx context.Context, base *url.URL, fqdn, ip, sid, capToken string, port int) (token, code string) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]int{"port": port})
	u := "https://" + base.Host + "/api/peer/sessions/" + url.PathEscape(sid) + "/ports"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", PortErrUnreachable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Portty-Cap", capToken)
	client := a.peerClient(fqdn, ip)
	defer client.CloseIdleConnections() // per-request client: don't leave its keep-alive open on both Macs
	resp, err := client.Do(req)
	if err != nil {
		return "", PortErrUnreachable
	}
	defer resp.Body.Close()
	marked := resp.Header.Get(tailnet.MarkerHeader) == tailnet.MarkerValue
	switch {
	case resp.StatusCode == http.StatusOK:
		var out struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&out); err != nil || out.Token == "" {
			return "", PortErrUnreachable
		}
		return out.Token, ""
	case resp.StatusCode == http.StatusForbidden && marked:
		return "", PortErrRefused
	case (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed) && !marked:
		return "", PortErrHostOutdated
	}
	return "", PortErrUnreachable
}

func writePortErr(w http.ResponseWriter, sid string, port int, code string) {
	logPortOutcome(sid, port, portOutcome[code])
	writeJSON(w, portErrStatus[code], PortOpenErrorResponse{Error: code})
}

// handlePortOpen answers POST /peers/ports. Order: classify, held capability,
// grant from the host, drop a stale approval, port taken by another Mac, then
// open (live approval) or ask (no approval). The grant runs before consent so
// the sheet is never shown for a port the host would refuse. `open` is only
// answered with a live approval, recorded after its forward is installed.
func (a *API) handlePortOpen(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req PortOpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePortErr(w, req.SessionID, 0, PortErrBadURL)
		return
	}
	sid := req.SessionID
	port, rest, ok := classifyLoopbackURL(req.URL)
	if !ok {
		writePortErr(w, sid, 0, PortErrBadURL)
		return
	}
	opened, err := openedURL(port, rest)
	if err != nil {
		writePortErr(w, sid, port, PortErrBadURL)
		return
	}
	baseURL, ip, capToken, held := a.remoteCaps.Get(sid)
	if !held {
		writePortErr(w, sid, port, PortErrNoCap)
		return
	}
	base, err := url.Parse(baseURL)
	if err != nil { // RemoteCapStore.Put already checked it; this is a corrupted entry
		writePortErr(w, sid, port, PortErrUnreachable)
		return
	}
	fqdn := base.Hostname()
	host := strings.ToLower(fqdn)
	label := magicDNSLabel(fqdn)

	token, code := a.requestPortGrant(r.Context(), base, fqdn, ip, sid, capToken, port)
	if code != "" {
		writePortErr(w, sid, port, code)
		return
	}

	pf := a.portForwards
	key := approvalKey{host: host, port: port}
	ap := approval{sid: sid, capFP: sha256.Sum256([]byte(capToken))}

	pf.mu.Lock()
	defer pf.mu.Unlock()
	live := pf.dropStaleLocked(a, key)
	f := pf.forwards[port]
	if f != nil && f.host != host {
		writePortErr(w, sid, port, PortErrPortInUse)
		return
	}
	if f != nil && live {
		f.rebind(sid, token)
		pf.approvals[key] = ap
		logPortOutcome(sid, port, "open")
		writeJSON(w, http.StatusOK, PortOpenResponse{Status: PortStatusOpen, URL: opened, Host: label, Port: port})
		return
	}
	if f != nil { // a forward with no live approval cannot be trusted: close it and start over
		pf.closeLocked(f)
	}
	if portFree(port) != nil {
		writePortErr(w, sid, port, PortErrPortInUse)
		return
	}
	if !live && !req.Consent {
		logPortOutcome(sid, port, "consent")
		writeJSON(w, http.StatusOK, PortOpenResponse{Status: PortStatusConsent, Host: label, Port: port})
		return
	}
	lns, err := bindLoopback(port)
	if err != nil {
		writePortErr(w, sid, port, PortErrPortInUse)
		return
	}
	nf := a.newPortForward(pf, port, host, base, fqdn, ip, label, sid, token)
	nf.serve(lns)
	pf.forwards[port] = nf
	pf.approvals[key] = ap
	logPortOutcome(sid, port, "open")
	writeJSON(w, http.StatusOK, PortOpenResponse{Status: PortStatusOpen, URL: opened, Host: label, Port: port})
}
