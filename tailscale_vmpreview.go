//go:build vmpreview

// VM-only fake for the native GUI check in the Tart VM, which has no
// Tailscale. Never in a release build: the vmpreview tag must not appear in
// .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).

package main

import (
	"context"

	"github.com/scottkw/portty/internal/webserver"
)

func init() {
	defaultTailscaleCheck = func(context.Context) webserver.TailscaleHealth {
		return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: "127.0.0.1"}
	}
}
