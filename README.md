<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/portty-title-logo-dark.png">
    <img src="docs/portty-title-logo.png" alt="Portty" width="400">
  </picture>
</p>

Portty is a macOS terminal with remote access over Tailscale. It also shows, and can control, another Mac's screen.

## Install

Portty needs a Mac with Apple silicon and macOS 14 or later.

1. Download the DMG from https://portty.app, open it, and drag Portty to Applications.
2. In Portty, choose **Portty ▸ Install Command-Line Tool…** to use `portty` in Terminal and other apps. Inside Portty it already works.

## Use It From Another Mac

1. Install Portty on both Macs.
2. Sign both Macs in to the same Tailscale account.
3. On the first Mac, turn on Share for a session (the Share button in the toolbar, or `portty share <session>`).
4. On the second Mac, open it from **Other Macs** in the sidebar, or run `portty attach <host>:<session>` in any terminal.

Other Macs lists only Macs signed in to your own Tailscale account. A Mac shared into your tailnet from another account is not listed, and Portty refuses to connect to it by name.

Any device signed in to your Tailscale account can also open the session's link in a browser, with full control. A device on a different Tailscale account gets a refusal page, even with the link. Treat the link like a password. Turning Share off disconnects everyone.

**Desktop (other Mac).** If another Mac has Always-available desktop turned on in Settings (under Sharing), its **Desktop** row appears first in its group in the sidebar. Click it to open that Mac's screen in a tab. Click the picture to control it: the label at the top right then reads *Controlling*, and every key goes to that Mac, including Cmd+Q, Cmd+W and Cmd+C. Press Ctrl+Option+Cmd+Shift+Return, click outside the picture or switch tabs to get Portty's shortcuts back. In control mode, Cmd+V pastes the text on your own clipboard into the other Mac, and Cmd+C copies on the other Mac only. Your Mac keeps its system shortcuts, such as Cmd+Tab and Cmd+Space; use Send Keys for those. A Desktop tab disconnects about 2 minutes after Portty's window is hidden, so the other Mac's display can sleep, and it reconnects when you show the window. The first time, macOS asks whether Portty may find devices on local networks. Choose Allow. A small speaker mark on the Desktop tab shows while its sound is on.

**Desktop (always available, any browser).** Turn on **Always-available desktop** in Settings, under Sharing, on the Mac you want to reach. Portty then keeps a background service running under macOS's launchd, so the screen stays reachable with Portty quit, after a restart of Portty and after an upgrade. Choose **Copy Link** and open it in Chrome or Safari on any device signed in to your Tailscale account; the link can control that Mac, so treat it like a password. In a browser, Ctrl+Option+Cmd+Return gives the keyboard back, and the bar has **View Only**, **Send Keys**, **Clipboard** and **Full Screen**. macOS asks you, at that Mac, to approve Screen Recording and Accessibility (**Set Up Permissions…** in Settings), and Screen Recording again about once a month. While the Mac is plugged in, Portty keeps it from sleeping on its own, though the display may still sleep; closing the lid or choosing Sleep still puts it to sleep, and nothing can wake it from here. If the background service stops running under macOS's control, Settings shows **Needs repair** with what to do, and the desktop is then reachable only while Portty is open.

**Share Desktop (guests).** Choose Share Desktop… from Portty's menu bar item or from Settings to get a link that anyone on your tailnet can open in a browser. Each person waits at an **Admit** prompt on your Mac that names them, can only watch until you turn on **Allow control** (and **Allow clipboard**) for them in the **Watching** list, and can be disconnected at any time; **Stop Sharing** ends every link at once, and so does quitting Portty. Your Mac shows a notice when someone asks to join, joins or leaves, and the menu bar icon shows while a share is on. Joins, leaves and control grants are also written to a local log on your Mac. If your tailnet uses access rules, a guest's device must be allowed to reach your Mac on tcp/7444 (the page and its signalling) and udp/7444 (the desktop picture), otherwise the guest sees Can't reach this Mac. Ask the tailnet admin to allow it.

**Sound and unlock.** Sound from the other Mac starts muted every time you open its desktop. Choose Unmute, or tap it on an iPhone, to hear it. The Mac sends sound to everyone watching, including people you share with, and muting only silences your own device. If the other Mac is locked, a viewer signed in to your own Tailscale account shows an Unlock This Mac box: type the Mac's password and choose Unlock. The password is sent once, over the encrypted connection, and is never saved or logged. Unlock allows one try about every 5 seconds and pauses for 15 minutes after 3 failed tries. Each try can check the password at the Mac twice, because Portty types it once more if the Mac is still locked, so the pause comes after at most six checks at the Mac. After you unlock a Mac from here, it locks itself again about 30 seconds after you disconnect, unless someone used its own keyboard or mouse. People you share with can hear the sound but can never unlock the Mac. If your connection is too slow, the picture may freeze and a Slow connection notice appears.

## Use It From Your iPhone

Open a shared session's link in Safari on an iPhone signed in to the same Tailscale account. The view fits the phone, adds a key bar (Esc, Tab, Ctrl, arrows, `|`), and pinch changes the font size. Add it to the Home Screen to open the session full screen. It picks up again after the phone sleeps or changes network. Step-by-step: [docs/launcher-app.md](docs/launcher-app.md).

**Desktop (iPhone).** Open the always-available desktop link in Safari on an iPhone signed in to the same Tailscale account, or from its Home Screen icon. Tap **Tap to Start** once to begin the picture; sound starts muted, so tap **Unmute** in the bottom bar to hear it. One finger moves the pointer like a trackpad, a tap clicks, a double-tap double-clicks, press and hold right-clicks, tap then drag drags, and two fingers scroll. Pinch zooms up to 5x and a two-finger double-tap fits the screen again. **Show Keyboard** opens the iPhone keyboard with a row of Esc, Tab, Ctrl, Opt, Cmd and arrow keys. Start drags away from the screen edge, because iOS uses an edge swipe to go back. If the other Mac is locked, the Unlock This Mac box works here too. It picks up again after the phone sleeps.

## Preview Files

- Run `portty open <file>` in any Portty session to show an image, SVG, PDF or Markdown file in a preview tab beside the terminal. It works in remote sessions and browser viewers too: the file travels over the session's own connection.
- Cmd-click a file path a program printed in the terminal to preview it. Portty resolves relative paths against the directory the shell was in when the path was printed.

## Open Remote localhost Links

In a session from another Mac, Cmd-click a `localhost:PORT` URL the remote Mac printed, such as a dev server. Portty asks once per Mac and port, then opens the page in your normal browser through the tailnet. Only ports you approved, and that the session actually printed, can be reached.

## Updating

Portty checks GitHub for new releases and offers them in the app. Choose **Update** in the banner: Portty downloads the new version, verifies it, replaces itself in Applications and restarts. If Portty can't replace itself from where it is running, it says so (for example, move it to the Applications folder first), and if the install is refused it shows the verified download in Finder for you to drag in. Upgrading keeps your sessions: the new background daemon takes over the running shells and their history, and connected viewers carry on by themselves. If that takeover fails, the old daemon keeps serving and Portty asks before restarting anything that would end a session.

Releases are for Apple silicon Macs with macOS 14 or later, so each release holds one build. It is published as `portty-vX.Y.Z-darwin-arm64.dmg`, and the same file again as `portty-vX.Y.Z-darwin-universal.dmg`: the universal name is not a universal build, it is kept so copies of Portty from before v2.0 can still find the update.

## Known Issues

**Chrome may offer to save the Mac's password.** After you use Unlock in Chrome, it can show a Save password? bubble. Click **Never**. Chrome saves nothing unless you click Save, and Safari shows no such bubble. The field stays a password field on purpose, so that macOS protects what you type and screen readers do not read it out.

**Unlock covers a locked Mac you are still logged in on.** It does not unlock the login screen after a restart or with FileVault. It types the password as keys, so a password with a character a keyboard can't type, such as an emoji, must be typed at the Mac, and with Caps Lock on at the Mac it asks you to turn it off there first. Wrong tries count toward the Mac's own limit.

**A slow link can freeze the picture.** Portty lowers the picture size and rate to keep up and shows Slow connection to the viewer on the slow link. It may not show that on a very light picture over a link slower than about 200 kbit/s, and sharpness returns when the screen is still or the link improves.

**Stray prompt lines after making the window narrower.** With a multi-line shell prompt (such as Powerlevel10k's two-line style), making the window narrower can leave copies of the prompt's top line in the scrollback. Wider, taller and shorter are fine. Each resize makes zsh redraw the prompt; narrowing wraps the prompt's lines, so the redraw starts one line too low and leaves the old top line behind. The lines are cosmetic and do not affect the session.

- Clear them with `clear` or Ctrl-L.
- Or use a one-line prompt. For Powerlevel10k, run `p10k configure` and pick the one-line style, or set it in `~/.p10k.zsh`.

## License

MIT. See [LICENSE](LICENSE).
