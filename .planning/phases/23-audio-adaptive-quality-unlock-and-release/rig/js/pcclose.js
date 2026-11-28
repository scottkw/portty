// Z-01: close only the viewer's RTCPeerConnection (needs hook first); the page and its signalling socket stay open.
// Returns the page clock at the close so it can be set against the office's leave line.
const pc = window.__pc
if (!pc) return 'no-pc'
const t = Date.now()
pc.close()
return JSON.stringify({ closed_at_ms: t, pc_state: pc.connectionState, page_open: document.visibilityState })
