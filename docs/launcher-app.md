<!-- Source copy for portty.app/app, rendered by the website repo. Keep it static: no inputs, no token, no WebSocket. It only explains the steps. -->

# Portty on your iPhone

Portty is a Mac terminal app. A session you start on one Mac can be opened from another Mac, or from your iPhone, over your Tailscale network. The session keeps running when you close the lid or walk away.

This page only explains the steps. It never sees your link and never connects to your Mac. Your iPhone talks straight to your Mac.

## Before you start

- Portty is running on your Mac, and the Mac is signed in to Tailscale.
- The Tailscale app is on your iPhone, signed in to the **same Tailscale account** as the Mac, and connected.

## 1. Get the link from your Mac

1. In Portty, open the session you want, then click **Share** in the toolbar.
2. Turn on **Share this session**.
3. Click **Copy**.

The link looks like this:

`https://<your-mac>.<your-tailnet>.ts.net:7444/sessions/…`

Treat this link like a password. It only opens on devices signed in to your Tailscale account.

## 2. Move the link to your iPhone

- **Universal Clipboard.** With Handoff on for both devices, copy on the Mac, then paste into Safari's address bar on the iPhone.
- **AirDrop.** Paste the link into Notes on the Mac, select it, choose **Share**, then **AirDrop**, and pick your iPhone. Tap the link on the iPhone to open it in Safari.

Don't send the link to yourself in Messages. Messages keeps a copy in iCloud.

## 3. Add it to your Home Screen

1. With the session open in Safari, tap the **More** button (the three dots), then **Share**.
2. In the share sheet, tap **View More**, then **Add to Home Screen**.
3. Leave **Open as Web App** on, keep the name **Portty**, and tap **Add**.

The Portty icon now opens your session full screen. On a new iPhone the icon may not appear on your Home Screen. If you can't find it, swipe left past your last Home Screen page to the App Library and look in the **Webclips** folder.

## When the icon stops working

The icon stops working when you turn Share off, make a new key, or restart Portty's background service on the Mac. It then shows **Link No Longer Works**. Share the session again, copy the new link, and repeat steps 2 and 3. Delete the old icon first.

If the icon shows **This link only works on your own devices**, the iPhone is signed in to a different Tailscale account than the Mac.

## Using it

The row of keys above the keyboard has Esc, Tab, Ctrl, the arrow keys and `|`. Tap **Ctrl**, then another key, to send a Control key such as Ctrl-C. Pinch the terminal to change the text size.
