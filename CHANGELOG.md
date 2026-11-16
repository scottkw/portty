# Changelog

Notable changes to Portty are recorded here; the earlier app's history stays in git.

## Unreleased

- Portty is a macOS terminal with remote access over Tailscale.
- Every session runs `/bin/zsh -l` in a background daemon and survives closing the window.
- Tabs, plus a sidebar of this Mac's sessions and sessions shared from other Macs.
- Follows macOS Light/Dark, with an Appearance override in Settings.
- Web and peer sharing on port 7444 over the tailnet.
- The `portty` CLI.
- Releases ship as a DMG linked from https://portty.app.
