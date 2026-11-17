//go:build vmpreview

// VM-only fake for the native GUI check in the Tart VM, which has no
// Tailscale. Never in a release build: the vmpreview tag must not appear in
// .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).
// Shares are served on 127.0.0.1:7444 with a self-signed certificate.

package daemon

import (
	"context"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
)

func init() {
	defaultShareHealth = func(context.Context) webserver.TailscaleHealth {
		return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: "127.0.0.1"}
	}
	defaultNewShareServer = func(_ webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		cfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
		if err != nil {
			return nil, err
		}
		return webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", Port: tailnet.DefaultProbePort, FQDN: "127.0.0.1", TLSConfig: cfg}, m)
	}
	defaultWarmCert = func(context.Context, string) error { return nil }
}
