# Changelog

Notable changes to Portty are recorded here; the earlier app's history stays in git.

## Unreleased

- Portty is a macOS terminal with remote access over Tailscale.
- Every session runs `/bin/zsh -l` in a background daemon and survives closing the window.
- Tabs, plus a sidebar of this Mac's sessions and sessions shared from other Macs.
- Follows macOS Light/Dark, with an Appearance override in Settings.
- One Share switch per session: Macs signed in to your Tailscale account see it in their sidebar, and you get a link for any browser on your tailnet, always with full control.
- portty share, portty unshare and portty attach <host>:<session>.
- The `portty` CLI.
- Releases ship as a DMG linked from https://portty.app.
