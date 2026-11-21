<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/portty-title-logo-dark.png">
    <img src="docs/portty-title-logo.png" alt="Portty" width="400">
  </picture>
</p>

Portty is a macOS terminal with remote access over Tailscale.

## Install

1. Download the DMG from https://portty.app, open it, and drag Portty to Applications.
2. In Portty, choose **Portty ▸ Install Command-Line Tool…** to use `portty` in Terminal and other apps. Inside Portty it already works.

## Use It From Another Mac

1. Install Portty on both Macs.
2. Sign both Macs in to the same Tailscale account.
3. On the first Mac, turn on Share for a session (the Share button in the toolbar, or `portty share <session>`).
4. On the second Mac, open it from **Other Macs** in the sidebar, or run `portty attach <host>:<session>` in any terminal.

Any device on your tailnet can also open the session's link in a browser, and anyone with the link has full control. Turning Share off disconnects everyone.

## Known Issues

**Stray prompt lines after making the window narrower.** With a multi-line shell prompt (such as Powerlevel10k's two-line style), making the window narrower can leave copies of the prompt's top line in the scrollback. Wider, taller and shorter are fine. Each resize makes zsh redraw the prompt; narrowing wraps the prompt's lines, so the redraw starts one line too low and leaves the old top line behind. The lines are cosmetic and do not affect the session.

- Clear them with `clear` or Ctrl-L.
- Or use a one-line prompt. For Powerlevel10k, run `p10k configure` and pick the one-line style, or set it in `~/.p10k.zsh`.

## License

MIT. See [LICENSE](LICENSE).
