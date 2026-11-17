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
2. Sign both Macs in to the same Tailscale account.
3. On the first Mac, turn on Share for a session (the Share button in the toolbar, or `portty share <session>`).
4. On the second Mac, open it from **Other Macs** in the sidebar, or run `portty attach <host>:<session>` in any terminal.

Any device on your tailnet can also open the session's link in a browser, and anyone with the link has full control. Turning Share off disconnects everyone.

## License

MIT. See [LICENSE](LICENSE).
