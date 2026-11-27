// Left and right energy of the received sound through a WebAudio channel splitter, for MS milliseconds.
// MS is replaced by the rig. The context is resumed first (a WebDriver session is not a user gesture).
const v = document.querySelector('video')
if (!v || !v.srcObject) return 'no-stream'
const tracks = v.srcObject.getAudioTracks()
if (!tracks.length) return 'no-audio-track'
const ctx = new (window.AudioContext || window.webkitAudioContext)()
await ctx.resume().catch(() => {})
const src = ctx.createMediaStreamSource(new MediaStream(tracks))
const split = ctx.createChannelSplitter(2)
const l = ctx.createAnalyser(), r = ctx.createAnalyser()
l.fftSize = r.fftSize = 2048
src.connect(split)
split.connect(l, 0)
split.connect(r, 1)
const bl = new Float32Array(2048), br = new Float32Array(2048)
let el = 0, er = 0, n = 0, peakL = 0, peakR = 0
const t0 = Date.now()
while (Date.now() - t0 < MS) {
  l.getFloatTimeDomainData(bl)
  r.getFloatTimeDomainData(br)
  for (let i = 0; i < 2048; i++) {
    el += bl[i] * bl[i]
    er += br[i] * br[i]
    peakL = Math.max(peakL, Math.abs(bl[i]))
    peakR = Math.max(peakR, Math.abs(br[i]))
  }
  n += 2048
  await new Promise((res) => setTimeout(res, 40))
}
await ctx.close()
return JSON.stringify({ ctx_state_ran: true, rmsL: +Math.sqrt(el / n).toFixed(5), rmsR: +Math.sqrt(er / n).toFixed(5), peakL: +peakL.toFixed(4), peakR: +peakR.toFixed(4), samples: n })
