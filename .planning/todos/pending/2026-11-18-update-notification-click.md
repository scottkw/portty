---
created: 2026-11-18
area: updater
source: Phase 6 review IN-05
---
# Clicking the update notification may not show Portty

There is no `didReceiveNotificationResponse` handler, so clicking the "Portty X is available" notification runs no app code. With the window hidden (tray-only), the click likely shows nothing. Check in a Tart VM with a notarized build (ad-hoc builds can't post notifications), then decide: show the window with the banner, or start the download.
