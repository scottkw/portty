<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/portty-title-logo-dark.png">
    <img src="docs/portty-title-logo.png" alt="Portty" width="400">
  </picture>
</p>

Portty is a macOS terminal with remote access over Tailscale.

## Install

1. Download the DMG from https://portty.app, open it, and drag Portty to Applications.
2. To use `portty` from a terminal, put it on your PATH:

   ```sh
   sudo mkdir -p /usr/local/bin && sudo ln -sf /Applications/Portty.app/Contents/MacOS/Portty /usr/local/bin/portty
   ```

## Use It From Another Mac

1. Install Portty on both Macs.
2. Sign both Macs into the same tailnet in Tailscale.
3. On the first Mac, share a session.
4. On the second Mac, open it from **Other Macs** in the sidebar, or run `portty attach <host>:<session-id>` in any terminal.

## License

MIT. See [LICENSE](LICENSE).
