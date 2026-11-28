// Receive-side health of the viewer's own connection (needs hook first): video frames received, decoded and
// dropped, mean jitter-buffer delay in ms, mean decode time in ms, packets lost; audio packets received.
const pc = window.__pc
if (!pc) return 'no-pc'
const rep = await pc.getStats()
const out = {}
rep.forEach((s) => {
  if (s.type !== 'inbound-rtp') return
  if (s.kind === 'video') {
    const n = s.jitterBufferEmittedCount || 0
    out.video = {
      recv: s.framesReceived, fd: s.framesDecoded, dropped: s.framesDropped, lost: s.packetsLost,
      jb_ms: n ? Math.round((1000 * s.jitterBufferDelay) / n) : null,
      dec_ms: s.framesDecoded ? Math.round((1000 * s.totalDecodeTime) / s.framesDecoded) : null,
      fw: s.frameWidth,
    }
  }
  if (s.kind === 'audio') out.audio = { recv: s.packetsReceived, lost: s.packetsLost }
})
return JSON.stringify(out)
