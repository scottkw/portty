---
created: 2026-11-21
area: release
due: v1.2 release
audit_acknowledged:
  milestone: v1.2
  at: 2026-11-23
---
Deferred from Phase 11 verification (user, 2026-11-21):
1. Look at the preview tabs and panes (image, SVG, PDF, Markdown, Receiving / Too Large / Couldn't Show states) in light and dark mode on a real Mac. The VMs can't screenshot web content.
2. Website install step 2 (portty-website `public/index.html:375`) still shows `sudo ln -sf`. Replace it with the menu-item copy in .planning/phases/11-file-preview/11-08-SUMMARY.md. That change is made in the website repo/session.
