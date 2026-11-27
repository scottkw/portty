// Phase 21-05 — the Desktop tab's connection logic, driven against the real
// Phase 19 createDesktopClient with a fake clock, document, window, transport
// and peer connection: no real socket, timer or network. The fakes are copied
// from desktopClient.test.ts on purpose (a test file is not imported).

import { describe, it, expect } from 'vitest'
import {
  createDesktopClient,
  type ClientStatus,
  type DesktopClientOptions,
  type SignalTransport,
} from '../desktopClient'
import {
  createPeerDesktopOptions,
  type PeerDesktopConnectResult,
} from '../peerDesktop'

class FakeClock {
  t = 0
  private seq = 0
  private timers: { id: number; at: number; fn: () => void }[] = []
  now = () => this.t
  setTimeout = (fn: () => void, ms: number) => {
    const id = ++this.seq
    this.timers.push({ id, at: this.t + ms, fn })
    return id
  }
  clearTimeout = (id: unknown) => {
    this.timers = this.timers.filter((x) => x.id !== id)
  }
  /** Advance the clock, firing due timers in order and letting promises settle after each. */
  async advance(ms: number) {
    const target = this.t + ms
    for (;;) {
      await settle()
      const due = this.timers.filter((x) => x.at <= target).sort((a, b) => a.at - b.at || a.id - b.id)[0]
      if (!due) break
      this.timers = this.timers.filter((x) => x !== due)
      this.t = due.at
      due.fn()
    }
    this.t = target
    await settle()
  }
}

async function settle() {
  for (let i = 0; i < 20; i++) await Promise.resolve()
}

class FakeTransport implements SignalTransport {
  sent: Record<string, unknown>[] = []
  closedWith: number | undefined | 'never' = 'never'
  private openCb: () => void = () => {}
  private msgCb: (m: unknown) => void = () => {}
  private closeCb: (c: number) => void = () => {}
  send(m: object) { this.sent.push(m as Record<string, unknown>) }
  close(code?: number) { this.closedWith = code }
  onOpen(cb: () => void) { this.openCb = cb }
  onMessage(cb: (m: unknown) => void) { this.msgCb = cb }
  onClose(cb: (c: number) => void) { this.closeCb = cb }
  open() { this.openCb() }
  recv(m: object) { this.msgCb(m) }
  dropWith(code: number) { this.closeCb(code) }
}

/** A peer connection that also speaks addEventListener, signalingState and iceConnectionState. */
class FakePC extends EventTarget {
  closed = false
  connectionState = 'new'
  iceConnectionState = 'new'
  signalingState = 'stable'
  iceGatheringState = 'complete'
  localDescription: { sdp: string } | null = null
  ontrack: ((e: unknown) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  addTransceiver() {}
  async createOffer() { return { type: 'offer', sdp: 'fake-offer-sdp' } }
  async setLocalDescription(d: { sdp: string }) {
    this.localDescription = d
    this.setSignaling('have-local-offer')
  }
  async setRemoteDescription() { this.setSignaling('stable') }
  close() {
    this.closed = true
    this.signalingState = 'closed'
  }
  fireTrack() { this.ontrack?.({ streams: [{ id: 'stream' }], track: {} }) }
  setSignaling(s: string) {
    this.signalingState = s
    this.dispatchEvent(new Event('signalingstatechange'))
  }
  setIce(s: string) {
    this.iceConnectionState = s
    this.dispatchEvent(new Event('iceconnectionstatechange'))
  }
}

const LOOPBACK = 'ws://127.0.0.1:41000/api/relay/remote-desktop/office-air.example/ws'
const ok = (url = LOOPBACK): PeerDesktopConnectResult => ({ host: 'office-air.example', url, errorCode: '' })

function setup(connect: (host: string) => Promise<PeerDesktopConnectResult>) {
  const clock = new FakeClock()
  const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  const win = Object.assign(new EventTarget(), { location: { hash: '', host: 'h', search: '' } }) as unknown as Window
  const urls: string[] = []
  const transports: FakeTransport[] = []
  const pcs: FakePC[] = []
  const statuses: ClientStatus[] = []
  const blocked: boolean[] = []
  const peer = createPeerDesktopOptions({
    host: 'office-air.example',
    connect,
    onLocalNetwork: (b) => blocked.push(b),
    clock,
    makeTransport: (url) => {
      urls.push(url)
      const t = new FakeTransport()
      transports.push(t)
      return t
    },
    makePC: () => {
      const p = new FakePC()
      pcs.push(p)
      return p as unknown as RTCPeerConnection
    },
  })
  const client = createDesktopClient({ ...peer.options, doc, win } as DesktopClientOptions)
  client.on('status', (s) => statuses.push(s))
  return { clock, doc, urls, transports, pcs, statuses, blocked, peer, client }
}
type Env = ReturnType<typeof setup>

const lastT = (e: Env) => e.transports[e.transports.length - 1]
const lastPC = (e: Env) => e.pcs[e.pcs.length - 1]

/** Walk the newest transport through hello, offer and answer. */
async function handshake(e: Env) {
  const t = lastT(e)
  t.open()
  t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
  await settle()
  t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
  await settle()
}

describe('tracer: connect, then stream over the loopback url', () => {
  it('takes the real client from connect to streaming with one transport and no token in any url', async () => {
    const e = setup(async () => ok())
    e.client.start()
    await settle()
    expect(e.urls).toEqual([LOOPBACK])
    await handshake(e)
    lastPC(e).fireTrack()
    expect(e.statuses.map((s) => s.kind)).toEqual(['streaming'])
    expect(e.urls).toHaveLength(1)
    for (const u of e.urls) expect(u).not.toMatch(/cap=|token=/)
    e.client.stop()
    e.peer.dispose()
  })
})
