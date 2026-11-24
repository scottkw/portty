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
