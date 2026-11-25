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

## Process (looked up 2026-11-24)
- No public request form confirmed. Apple DTS (Quinn, forums thread 816646): escalation path for entitlement requests is developer.apple.com/contact → Development and Technical → Entitlements. Check the signed-in developer account for a request option.
- Reported waits (developers, not Apple): weeks to 2+ months, little correspondence.
- Restricted entitlement is tied to an App ID; each executable signed with it needs its own App ID + embedded provisioning profile (DTS).
- TIMING (user asked 2026-11-24): file AFTER Phase 17 records FEAS-02 (capture host = daemon → app.portty.Portty, or helper → its own bundle ID). Claude drafts the request text then.
- Include: team S2K7P43927, app Portty, bundle ID from FEAS-02, use case (unattended remote-desktop server on the user's own Mac over their tailnet; nobody present to answer the monthly Screen Recording prompt), Developer ID notarized distribution.
