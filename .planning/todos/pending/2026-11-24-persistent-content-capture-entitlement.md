---
created: 2026-11-24
title: File Apple persistent-content-capture entitlement request
area: release
files: []
---

## Problem
macOS 15+ asks monthly to re-approve Screen Recording. On the unattended office Mac nobody can click it, so desktop sharing stops until someone is at the Mac. The restricted entitlement `com.apple.developer.persistent-content-capture` is the only Apple-sanctioned fix; requests reportedly take 2+ months.

## Solution
User files the request from the Apple Developer account (user said "I'll file it now", 2026-11-24). Non-blocking for v2.0; once granted, add the provisioning profile to the signing pipeline (see .planning/research/SUMMARY.md).

## Process (looked up 2026-11-24; corrected 2026-11-25)
- CORRECT FORM (2026-11-25): https://developer.apple.com/contact/request/persistent-content-capture/ (Apple Developer sign-in required). Apple describes the entitlement as for VNC / remote-screen apps. The generic developer.apple.com/contact page has no Entitlements option.
- No public request form confirmed. Apple DTS (Quinn, forums thread 816646): escalation path for entitlement requests is developer.apple.com/contact → Development and Technical → Entitlements. Check the signed-in developer account for a request option.
- Reported waits (developers, not Apple): weeks to 2+ months, little correspondence.
- Restricted entitlement is tied to an App ID; each executable signed with it needs its own App ID + embedded provisioning profile (DTS).
- TIMING (user asked 2026-11-24): file AFTER Phase 17 records FEAS-02 (capture host = daemon → app.portty.Portty, or helper → its own bundle ID). Claude drafts the request text then.
- Include: team S2K7P43927, app Portty, bundle ID from FEAS-02, use case (unattended remote-desktop server on the user's own Mac over their tailnet; nobody present to answer the monthly Screen Recording prompt), Developer ID notarized distribution.

## Request text (drafted 2026-11-25, after FEAS-02 decided in-daemon)

Subject: Request for com.apple.developer.persistent-content-capture — Portty (app.portty.Portty)

Team ID: S2K7P43927
App: Portty
Bundle ID: app.portty.Portty
Distribution: Developer ID, notarized (not Mac App Store)

Use case:
Portty is a macOS terminal and remote-desktop app. A user runs it on their own Mac (for example, an office Mac) and connects from their other devices: a second Mac, a browser, or an iPhone. The connection goes over the user's private Tailscale network only, with no relay or cloud service. The main use is unattended access: the user is away from the shared Mac and nobody is in front of it.

Screen capture uses ScreenCaptureKit inside Portty's background process, a LaunchAgent-started instance of the same signed app binary. The user grants Screen Recording once, in System Settings.

The problem is the periodic Screen Recording re-approval prompt. On an unattended Mac nobody can answer it, so remote access stops until someone is physically back at the Mac. That defeats the purpose of the feature, and the user can't get in remotely to fix it.

Safeguards:
- Desktop access is limited to the owner's own devices on their own Tailscale network. Each connection is checked against the Tailscale identity of the connecting device.
- Sharing with other people requires an explicit per-person approval on the shared Mac, and is view-only by default.
- macOS shows its screen-capture indicator whenever capture is active, and the shared Mac shows a persistent "shared with N" indicator with a one-click stop.
- Nothing is recorded or stored; frames are encoded and streamed live only.

We'd appreciate the entitlement for app.portty.Portty, or guidance on the right path if a different process applies.
