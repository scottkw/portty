---
created: 2026-11-18T01:30:00Z
title: Pin the Go toolchain to what tailscale.com supports (1.26.x), not Go 1.27
area: build
severity: major
files:
  - go.mod
---

## Problem

While setting up test VMs (2026-11-17), a tailscaled v1.98.3 built with Go 1.27.4 registered fine but could never reach Running: every netmap failed with `json: cannot unmarshal object into Go struct field MapResponse.SSHPolicy.rules.0.action of type tailcfg.SSHAction: Go struct field SessionDuration has unsupported format`. tailscale.com v1.98.3's go.mod says `go 1.26.3`; its vendored json-experiment code is incompatible with Go 1.27's encoding/json changes. Building tailscale's own cmd with Go 1.27 also failed outright (`undefined: json.SkipFunc`).

Portty is built with Go 1.27.4 and imports tailscale.com (client/local, tailcfg/ipnstate types). Any tailscale type Portty decodes from JSON could hit the same silent decode failure.

## Solution

1. Add `toolchain go1.26.3` (or the version tailscale.com pins) to go.mod so builds use it, and match CI (build.yml).
2. Check whether Portty decodes any tailscale struct with duration/format tags (status, whois, peers) and add a test that decodes a real-shaped LocalAPI response.
3. Revisit when upgrading tailscale.com to a release that supports Go 1.27.
