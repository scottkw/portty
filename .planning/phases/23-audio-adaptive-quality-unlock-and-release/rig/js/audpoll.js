// Keep the received audio's packet count in window.__aud (once a second) so a one-shot page read can see
// whether sound keeps arriving across a reconnect. Needs the hook (window.__pc, which follows a new connection
// because the debug poll's getStats call re-captures it). Counts only.
if (!window.__audpoll) {
  window.__audpoll = setInterval(async () => {
    const pc = window.__pc
    if (!pc) return
    try {
      const rep = await pc.getStats()
      rep.forEach((s) => {
        if (s.type === 'inbound-rtp' && s.kind === 'audio') window.__aud = { recv: s.packetsReceived, at: Date.now() }
      })
    } catch (e) {}
  }, 1000)
}
await new Promise((r) => setTimeout(r, 2500))
return JSON.stringify(window.__aud || null)
