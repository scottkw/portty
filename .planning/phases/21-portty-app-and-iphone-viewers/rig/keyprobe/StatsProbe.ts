// THROWAWAY Phase 21-11 stats probe (scratch copy only, never the shipped tree). Imported first by
// main.tsx by `apply.sh --stats`. It turns on desktopClient's existing debug tick, then forwards
// counts to ProbeLog (~/p21/keyprobe.log in the VM): the tick's __desktopStats numbers, and every
// RTCPeerConnection's ICE, connection and gathering state changes. Counts and states only: no
// token, no SDP, no candidate, no address.
import { ProbeLog } from './wailsjs/go/main/App'

const log = (s: string) => { void ProbeLog(s) }

try {
  window.localStorage.setItem('portty:desktop-debug', '1')
} catch {
  log('STATS storage-blocked')
}

const Orig = window.RTCPeerConnection
let pcSeq = 0
if (Orig) {
  class ProbedPC extends Orig {
    constructor(...args: ConstructorParameters<typeof RTCPeerConnection>) {
      super(...args)
      const id = ++pcSeq
      log(`PC new id=${id}`)
      this.addEventListener('iceconnectionstatechange', () => log(`ICE id=${id} ${this.iceConnectionState}`))
      this.addEventListener('connectionstatechange', () => log(`PCSTATE id=${id} ${this.connectionState}`))
      this.addEventListener('icegatheringstatechange', () => log(`ICEGATHER id=${id} ${this.iceGatheringState}`))
    }
  }
  window.RTCPeerConnection = ProbedPC
}

// App.tsx re-dispatches the native occlusion event as this window event (21-08).
window.addEventListener('portty:window-hidden', (e) => log(`WINHIDDEN hidden=${(e as CustomEvent).detail}`))

let last: unknown = null
window.setInterval(() => {
  const s = (window as unknown as { __desktopStats?: Record<string, unknown> }).__desktopStats
  if (!s || s === last) return
  last = s
  log(`STATS status=${s.status} state=${s.state} viewers=${s.viewers} fd=${s.framesDecoded} fw=${s.frameWidth} fh=${s.frameHeight} bytes=${s.bytesReceived} csp=${s.cspViolations}`)
}, 1000)
log('PROBE ready stats')
