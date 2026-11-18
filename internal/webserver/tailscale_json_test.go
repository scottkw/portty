package webserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/tailcfg"
)

// LocalAPI bodies shaped like tailscaled v1.98.3's, with the field names of
// ipnstate.Status/PeerStatus, apitype.WhoIsResponse, tailcfg.Node and ipn.Prefs.
const (
	tsSelfJSON = `{
		"ID": "nSelf1CNTRL", "PublicKey": "nodekey:1111111111111111111111111111111111111111111111111111111111111111",
		"HostName": "mac-office", "DNSName": "mac-office.tail1234.ts.net.", "OS": "macOS",
		"UserID": 42, "TailscaleIPs": ["100.64.0.1", "fd7a:115c:a1e0::1"],
		"Addrs": ["192.168.1.10:41641"], "CurAddr": "", "Relay": "nyc", "PeerRelay": "",
		"RxBytes": 0, "TxBytes": 0, "Created": "2026-10-01T12:00:00Z",
		"LastWrite": "0001-01-01T00:00:00Z", "LastSeen": "0001-01-01T00:00:00Z", "LastHandshake": "0001-01-01T00:00:00Z",
		"Online": true, "ExitNode": false, "ExitNodeOption": false, "Active": false,
		"PeerAPIURL": ["http://100.64.0.1:52000"], "TaildropTarget": 0, "NoFileSharingReason": "",
		"Capabilities": ["https://tailscale.com/cap/is-admin"], "CapMap": {"https://tailscale.com/cap/is-admin": null},
		"InNetworkMap": true, "InMagicSock": false, "InEngine": false,
		"KeyExpiry": "2027-04-01T12:00:00Z"
	}`

	tsPeersJSON = `{
		"nodekey:2222222222222222222222222222222222222222222222222222222222222222": {
			"ID": "nHome2CNTRL", "PublicKey": "nodekey:2222222222222222222222222222222222222222222222222222222222222222",
			"HostName": "mac-home", "DNSName": "mac-home.tail1234.ts.net.", "OS": "macOS",
			"UserID": 42, "TailscaleIPs": ["100.64.0.2", "fd7a:115c:a1e0::2"],
			"AllowedIPs": ["100.64.0.2/32", "fd7a:115c:a1e0::2/128"], "Tags": null,
			"Addrs": null, "CurAddr": "203.0.113.5:41641", "Relay": "nyc", "PeerRelay": "",
			"RxBytes": 1024, "TxBytes": 2048, "Created": "2026-10-02T12:00:00Z",
			"LastWrite": "2026-11-17T21:00:00Z", "LastSeen": "0001-01-01T00:00:00Z", "LastHandshake": "2026-11-17T20:59:00Z",
			"Online": true, "ExitNode": false, "ExitNodeOption": false, "Active": true,
			"PeerAPIURL": ["http://100.64.0.2:52000"], "TaildropTarget": 3, "NoFileSharingReason": "",
			"InNetworkMap": true, "InMagicSock": true, "InEngine": true,
			"KeyExpiry": "2027-04-02T12:00:00Z"
		},
		"nodekey:3333333333333333333333333333333333333333333333333333333333333333": {
			"ID": "nSrv3CNTRL", "PublicKey": "nodekey:3333333333333333333333333333333333333333333333333333333333333333",
			"HostName": "build-box", "DNSName": "build-box.tail1234.ts.net.", "OS": "linux",
			"UserID": 7, "TailscaleIPs": ["100.64.0.3"], "Tags": ["tag:server"],
			"Addrs": null, "CurAddr": "", "Relay": "", "PeerRelay": "",
			"RxBytes": 0, "TxBytes": 0, "Created": "2026-09-01T12:00:00Z",
			"LastWrite": "0001-01-01T00:00:00Z", "LastSeen": "2026-11-16T08:30:00Z", "LastHandshake": "0001-01-01T00:00:00Z",
			"Online": false, "ExitNode": false, "ExitNodeOption": false, "Active": false,
			"PeerAPIURL": null, "TaildropTarget": 2, "NoFileSharingReason": "",
			"InNetworkMap": true, "InMagicSock": false, "InEngine": false
		}
	}`

	tsWhoIsJSON = `{
		"Node": {
			"ID": 2, "StableID": "nHome2CNTRL", "Name": "mac-home.tail1234.ts.net.",
			"User": 42, "Key": "nodekey:2222222222222222222222222222222222222222222222222222222222222222",
			"KeyExpiry": "2027-04-02T12:00:00Z",
			"Addresses": ["100.64.0.2/32", "fd7a:115c:a1e0::2/128"],
			"AllowedIPs": ["100.64.0.2/32", "fd7a:115c:a1e0::2/128"],
			"HomeDERP": 1, "Hostinfo": {"OS": "macOS", "Hostname": "mac-home"},
			"Created": "2026-10-02T12:00:00Z", "Cap": 133, "MachineAuthorized": true,
			"LastSeen": "2026-11-17T21:00:00Z", "Online": true
		},
		"UserProfile": {"ID": 42, "LoginName": "user@example.com", "DisplayName": "Example User"},
		"CapMap": {"tailscale.com/cap/file-sharing": null}
	}`

	tsPrefsJSON = `{
		"ControlURL": "https://controlplane.tailscale.com", "RouteAll": false,
		"ExitNodeID": "", "ExitNodeIP": "", "InternalExitNodePrior": "", "ExitNodeAllowLANAccess": false,
		"CorpDNS": true, "RunSSH": false, "RunWebClient": false, "WantRunning": true, "LoggedOut": false,
		"ShieldsUp": false, "AdvertiseTags": null, "Hostname": "", "NotepadURLs": false,
		"AdvertiseRoutes": null, "AdvertiseServices": null, "NoSNAT": false, "NetfilterMode": 2,
		"AutoUpdate": {"Check": true, "Apply": null}, "AppConnector": {"Advertise": false},
		"PostureChecking": false, "NetfilterKind": ""
	}`
)

func tsStatusJSON(withPeers bool) string {
	peers := ""
	if withPeers {
		peers = `"Peer": ` + tsPeersJSON + `,`
	}
	return `{
		"Version": "1.98.3-t0123456789-g0123456789", "TUN": true, "BackendState": "Running",
		"HaveNodeKey": true, "AuthURL": "", "TailscaleIPs": ["100.64.0.1", "fd7a:115c:a1e0::1"],
		"Self": ` + tsSelfJSON + `, "Health": [],
		"MagicDNSSuffix": "tail1234.ts.net",
		"CurrentTailnet": {"Name": "user@example.com", "MagicDNSSuffix": "tail1234.ts.net", "MagicDNSEnabled": true},
		"CertDomains": ["mac-office.tail1234.ts.net"], ` + peers + `
		"User": {"42": {"ID": 42, "LoginName": "user@example.com", "DisplayName": "Example User"}},
		"ClientVersion": {"RunningLatest": true}
	}`
}

// fakeLocalAPI serves the four LocalAPI routes Portty calls and returns a
// real local.Client dialing it.
func fakeLocalAPI(t *testing.T) *local.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch {
		case r.URL.Path == "/localapi/v0/status" && r.URL.RawQuery == "":
			body = tsStatusJSON(true)
		case r.URL.Path == "/localapi/v0/status" && r.URL.Query().Get("peers") == "false":
			body = tsStatusJSON(false)
		case r.URL.Path == "/localapi/v0/whois" && r.URL.Query().Get("addr") == "100.64.0.2:51234":
			body = tsWhoIsJSON
		case r.URL.Path == "/localapi/v0/prefs":
			body = tsPrefsJSON
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	return &local.Client{
		OmitAuth: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
}

func TestTailscaleJSON_LocalAPIShapes(t *testing.T) {
	lc := fakeLocalAPI(t)
	ctx := t.Context()

	// Health check: Status (no peers) + Prefs through Portty's own code.
	h := checkHealth(ctx, lc.StatusWithoutPeers, realPrefsFunc(lc))
	want := TailscaleHealth{Connected: true, HasCerts: true, IP: "100.64.0.1", Domain: "mac-office.tail1234.ts.net", AcceptDNS: true}
	if h != want {
		t.Fatalf("checkHealth = %+v, want %+v", h, want)
	}

	// Same-account gate: StatusWithoutPeers + WhoIs through sameUser.
	st, err := lc.StatusWithoutPeers(ctx)
	if err != nil {
		t.Fatalf("StatusWithoutPeers: %v", err)
	}
	if st.Self == nil || st.Self.UserID != 42 || st.Self.Tags != nil || st.Self.AltSharerUserID != 0 {
		t.Fatalf("Self = %+v, want UserID 42, no tags, no sharer", st.Self)
	}
	if len(st.Peer) != 0 {
		t.Fatalf("peers=false status has %d peers", len(st.Peer))
	}
	if st.Self.KeyExpiry == nil || !st.Self.KeyExpiry.Equal(time.Date(2027, 4, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("Self.KeyExpiry = %v", st.Self.KeyExpiry)
	}
	who, err := lc.WhoIs(ctx, "100.64.0.2:51234")
	if err != nil {
		t.Fatalf("WhoIs: %v", err)
	}
	if who.Node == nil || who.Node.User != 42 || who.Node.Sharer != 0 || who.Node.IsTagged() {
		t.Fatalf("WhoIs.Node = %+v, want User 42, untagged, unshared", who.Node)
	}
	if who.UserProfile == nil || who.UserProfile.ID != 42 {
		t.Fatalf("WhoIs.UserProfile = %+v", who.UserProfile)
	}
	if !sameUser(who, st.Self) {
		t.Fatal("sameUser(whois, self) = false for a same-account peer")
	}

	// Peer discovery (internal/tailnet reads Online, HostName, DNSName, OS, TailscaleIPs).
	full, err := lc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if full.CurrentTailnet == nil || full.CurrentTailnet.MagicDNSSuffix != "tail1234.ts.net" {
		t.Fatalf("CurrentTailnet = %+v", full.CurrentTailnet)
	}
	type peerView struct {
		host, dns, os, ips string
		online, tagged     bool
	}
	got := map[string]peerView{}
	for k, p := range full.Peer {
		if p.PublicKey != k {
			t.Errorf("peer %v: PublicKey %v does not match its map key", k, p.PublicKey)
		}
		ips := ""
		for _, ip := range p.TailscaleIPs {
			ips += ip.String() + " "
		}
		if p.HostName == "build-box" && !p.LastSeen.Equal(time.Date(2026, 11, 16, 8, 30, 0, 0, time.UTC)) {
			t.Errorf("offline peer LastSeen = %v", p.LastSeen)
		}
		got[p.HostName] = peerView{p.HostName, p.DNSName, p.OS, ips, p.Online, p.Tags != nil && p.Tags.Len() > 0}
	}
	wantPeers := map[string]peerView{
		"mac-home":  {"mac-home", "mac-home.tail1234.ts.net.", "macOS", "100.64.0.2 fd7a:115c:a1e0::2 ", true, false},
		"build-box": {"build-box", "build-box.tail1234.ts.net.", "linux", "100.64.0.3 ", false, true},
	}
	if len(got) != len(wantPeers) {
		t.Fatalf("peers = %+v, want %+v", got, wantPeers)
	}
	for name, w := range wantPeers {
		if got[name] != w {
			t.Errorf("peer %s = %+v, want %+v", name, got[name], w)
		}
	}
}

// TestTailscaleJSON_FormatTagCanary decodes the tailcfg type whose
// `format:nano` tag Go 1.27's encoding/json rejects (it stalled a tailscaled
// built with Go 1.27.4 before Running). Run it under a new Go before dropping
// the go.mod toolchain pin and toolchain_guard.go.
func TestTailscaleJSON_FormatTagCanary(t *testing.T) {
	var a tailcfg.SSHAction
	if err := json.Unmarshal([]byte(`{"accept":true,"sessionDuration":3600000000000}`), &a); err != nil {
		t.Fatalf("Go toolchain %s cannot decode tailscale.com v1.98.3 JSON (format tag); build and test with the go.mod toolchain: GOTOOLCHAIN=go1.26.11 (%v)", runtime.Version(), err)
	}
	if !a.Accept || a.SessionDuration != time.Hour {
		t.Fatalf("SSHAction = %+v, want Accept and SessionDuration 1h", a)
	}
}
