# Portty brand assets

Drop-in replacements for the AgentHub icon/logo files in scottkw/agenthub. Paths mirror the repo.

## Brand
- Name: **Portty** (CLI: `portty`) — port + tty
- Mark: lowercase "p" with a cursor bar ("p_")
- Colors: blue #2457F5 (icon bg, gradient #3165FF→#1E4DE6), ink #0B1436, light #EEF1FB, dark-mode cursor #6F93FF
- Wordmark font: Bricolage Grotesque 700

## Steps for Claude Code
1. Copy everything in this folder over the repo root.
2. Rename every `*-2x.png` to `*@2x.png` (assets/ and build/AppIcon.iconset/):
   `for f in $(find assets build/AppIcon.iconset -name '*-2x.png'); do mv "$f" "${f%-2x.png}@2x.png"; done`
3. Regenerate macOS icns: `iconutil -c icns build/AppIcon.iconset -o build/darwin/iconfile.icns`
4. Copy `docs/portty-title-logo.png` to `frontend/src/assets/portty-title-logo.png`; update imports of `agenthub-title-logo.png` (README, WelcomeTab, scripts/verify-icons.sh).
5. Delete or stop running `build/gen_progress_icons.go` — progress tray icons are now pre-rendered (ring fills clockwise) and that script would overwrite them with the old bottom-bar style.
6. On macOS, load the @2x tray icons for Retina and keep them as template images (black on transparent).
7. Delete old `docs/agenthub-title-logo.png`.

## Files
- build/appicon.png, assets/appicon.png — 1024 macOS-style icon
- build/AppIcon.iconset/ — 16–512 @1x/@2x
- build/windows/icon.ico — 16/32/48/256
- build/linux/{16,32,48,128,256,512}x*.png — full-bleed squircle
- assets/tray_icon{,_error,_progress_25,_50,_75,_100}.png — 18px template + -2x (36px)
- docs/portty-title-logo{,-dark}.png — 1400×360 transparent
