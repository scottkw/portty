//go:build go1.27

// Portty pins Go 1.26 (go.mod toolchain line): tailscale.com v1.98.3's JSON
// decoding breaks on Go 1.27's encoding/json (tailcfg's `format:nano` tag), and
// GOTOOLCHAIN=auto never downgrades a newer local Go. This file stops a Go 1.27+
// build of the app and CLI; build.sh sets GOTOOLCHAIN from go.mod, or run
// `export GOTOOLCHAIN=go1.26.11`. Remove this file and the go.mod toolchain line
// together when tailscale.com is upgraded, after TestTailscaleJSON_FormatTagCanary
// passes under the new Go.

package main

var _ = buildPorttyWithGo1_26__export_GOTOOLCHAIN_from_go_mod
