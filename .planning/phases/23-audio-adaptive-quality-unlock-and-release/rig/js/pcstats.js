// Counts only: receive-side RTP numbers, the negotiated codecs with their channel counts, whether the
// viewer's own offer carried the transport-wide-cc extension, and the video element's sound state.
const pc = window.__pc
if (!pc) return 'no-pc'
const rep = await pc.getStats()
const out = { codecs: [] }
rep.forEach((s) => {
  if (s.type === 'inbound-rtp') {
    out[s.kind] = { lost: s.packetsLost, recv: s.packetsReceived, jitter: s.jitter, fd: s.framesDecoded, fw: s.frameWidth, energy: s.totalAudioEnergy }
  }
  if (s.type === 'codec') out.codecs.push([s.mimeType, s.channels, s.clockRate])
})
const sdp = (pc.localDescription && pc.localDescription.sdp) || ''
out.twcc_in_offer = /transport-wide-cc|transport-cc/.test(sdp)
out.extmap_uris = [...new Set((sdp.match(/a=extmap:\d+ \S+/g) || []).map((l) => l.split(' ')[1]))]
const rsdp = (pc.remoteDescription && pc.remoteDescription.sdp) || ''
out.opus_stereo_in_answer = /opus\/48000\/2/.test(rsdp)
const v = document.querySelector('video')
out.element = v ? { muted: v.muted, paused: v.paused, audioTracks: v.srcObject ? v.srcObject.getAudioTracks().length : 0 } : null
return JSON.stringify(out)
