---
created: 2026-11-23
area: frontend / sidebar
source: Phase 15 UI review (15-UI-REVIEW.md fix 3)
---
A reachable peer with nothing shared shows the green `sidebar__dot--on` and "Reachable" beside "Nothing shared" (Sidebar.tsx:201,208,218), which can read as "connected and sharing". Pre-existing, but Phase 15-07 makes the state more common (a host whose last share ended). Decide: keep the green dot, or use a neutral dot for reachable-but-empty. Design decision for the user.
