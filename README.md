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

Any device signed in to your Tailscale account can also open the session's link in a browser, with full control. A device on a different Tailscale account gets a refusal page, even with the link. Treat the link like a password. Turning Share off disconnects everyone.

**Desktop (other Mac).** If another Mac has Always-available desktop turned on in Settings, its **Desktop** row appears first in its group in the sidebar. Click it to open that Mac's screen in a tab. Click the picture to control it: the label at the top right then reads *Controlling*, and every key goes to that Mac, including Cmd+Q, Cmd+W and Cmd+C. Press Ctrl+Option+Cmd+Shift+Return, click outside the picture or switch tabs to get Portty's shortcuts back. In control mode, Cmd+V pastes the text on your own clipboard into the other Mac, and Cmd+C copies on the other Mac only. Your Mac keeps its system shortcuts, such as Cmd+Tab and Cmd+Space; use Send Keys for those. A Desktop tab disconnects about 2 minutes after Portty's window is hidden, so the other Mac's display can sleep, and it reconnects when you show the window. The first time, macOS asks whether Portty may find devices on local networks. Choose Allow.

**Share Desktop (guests).** Choose Share Desktop… from Portty's menu bar item or from Settings to get a link that anyone on your tailnet can open in a browser. Each person waits until you let them in, can only watch until you allow control for them, and can be disconnected at any time; Stop Sharing ends every link at once. If your tailnet uses access rules, a guest's device must be allowed to reach your Mac on tcp/7444 (the page and its signalling) and udp/7444 (the desktop picture), otherwise the guest sees Can't reach this Mac. Ask the tailnet admin to allow it.

## Use It From Your iPhone

Open a shared session's link in Safari on an iPhone signed in to the same Tailscale account. The view fits the phone, adds a key bar (Esc, Tab, Ctrl, arrows, `|`), and pinch changes the font size. Add it to the Home Screen to open the session full screen. It picks up again after the phone sleeps or changes network. Step-by-step: [docs/launcher-app.md](docs/launcher-app.md).

**Desktop (iPhone).** Open the always-available desktop link in Safari on an iPhone signed in to the same Tailscale account, or from its Home Screen icon. Tap **Tap to Start** once to begin the picture. One finger moves the pointer like a trackpad, a tap clicks, a double-tap double-clicks, press and hold right-clicks, tap then drag drags, and two fingers scroll. Pinch zooms up to 5x and a two-finger double-tap fits the screen again. **Show Keyboard** opens the iPhone keyboard with a row of Esc, Tab, Ctrl, Opt, Cmd and arrow keys. Start drags away from the screen edge, because iOS uses an edge swipe to go back. It picks up again after the phone sleeps.

## Preview Files

- Run `portty open <file>` in any Portty session to show an image, SVG, PDF or Markdown file in a preview tab beside the terminal. It works in remote sessions and browser viewers too: the file travels over the session's own connection.
- Cmd-click a file path a program printed in the terminal to preview it. Portty resolves relative paths against the directory the shell was in when the path was printed.

## Open Remote localhost Links

In a session from another Mac, Cmd-click a `localhost:PORT` URL the remote Mac printed, such as a dev server. Portty asks once per Mac and port, then opens the page in your normal browser through the tailnet. Only ports you approved, and that the session actually printed, can be reached.

## Updating

Portty checks GitHub for new releases and offers them in the app. Upgrading keeps your sessions: the new background daemon takes over the running shells and their history, and connected viewers carry on by themselves. If that takeover fails, the old daemon keeps serving and Portty asks before restarting anything that would end a session.

## Known Issues

**Stray prompt lines after making the window narrower.** With a multi-line shell prompt (such as Powerlevel10k's two-line style), making the window narrower can leave copies of the prompt's top line in the scrollback. Wider, taller and shorter are fine. Each resize makes zsh redraw the prompt; narrowing wraps the prompt's lines, so the redraw starts one line too low and leaves the old top line behind. The lines are cosmetic and do not affect the session.

- Clear them with `clear` or Ctrl-L.
- Or use a one-line prompt. For Powerlevel10k, run `p10k configure` and pick the one-line style, or set it in `~/.p10k.zsh`.

## License

MIT. See [LICENSE](LICENSE).
