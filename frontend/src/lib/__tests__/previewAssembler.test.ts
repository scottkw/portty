import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPreviewAssembler, PREVIEW_MAX_BYTES, type PreviewEvent } from '../previewAssembler'
import type { PreviewFrame } from '../relayClient'
import previewFixture from './fixtures/preview-frames.json'

const KEY = '0123456789abcdef0123456789abcdef'
const header = (o: Partial<Extract<PreviewFrame, { kind: 'header' }>> = {}): PreviewFrame =>
  ({ kind: 'header', id: 1, name: 'shot.png', mime: 'image/png', size: 10, key: KEY, focus: true, ...o })
const chunk = (n: number, id = 1): PreviewFrame => ({ kind: 'chunk', id, payload: new Uint8Array(n).fill(7) })
const end = (status = 0, id = 1): PreviewFrame => ({ kind: 'end', id, status })

let events: PreviewEvent[]
let feed: (f: PreviewFrame) => void
let reset: () => void
beforeEach(() => {
  vi.useFakeTimers()
  events = []
  const a = createPreviewAssembler((e) => events.push(e))
  feed = a.feed
  reset = a.reset
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
const types = () => events.map((e) => e.type)

describe('previewAssembler happy path (PREV-01)', () => {
  it("the cap equals the Go fixture's maxBytes", () => {
    expect(PREVIEW_MAX_BYTES).toBe(previewFixture.maxBytes)
  })

  it('header, chunks and an ok end give start, progress and a Blob of the declared size and the viewer type', async () => {
    feed(header({ size: 10 }))
    feed(chunk(4))
    feed(chunk(6))
    feed(end(0))
    expect(types()[0]).toBe('start')
    expect(types()).toContain('progress')
    expect(types()[events.length - 1]).toBe('complete')
    expect(events[0]).toEqual({ type: 'start', id: 1, name: 'shot.png', kind: 'image', mime: 'image/png', size: 10, key: KEY, focus: true })
    const done = events[events.length - 1] as Extract<PreviewEvent, { type: 'complete' }>
    expect(done.blob.size).toBe(10)
    expect(done.blob.type).toBe('image/png')
    expect(new Uint8Array(await done.blob.arrayBuffer())).toEqual(new Uint8Array(10).fill(7))
  })

  for (const [mime, kind] of [['image/jpeg', 'image'], ['image/gif', 'image'], ['image/webp', 'image'], ['image/svg+xml', 'svg'], ['application/pdf', 'pdf'], ['text/markdown', 'markdown']] as const) {
    it(`${mime} maps to kind ${kind} and a Blob typed ${mime}`, () => {
      feed(header({ mime, size: 1 }))
      feed(chunk(1))
      feed(end(0))
      expect(events[0]).toMatchObject({ kind, mime })
      expect((events[events.length - 1] as Extract<PreviewEvent, { type: 'complete' }>).blob.type).toBe(mime)
    })
  }

  it('reset mid-stream fails the open preview as incomplete', () => {
    feed(header())
    feed(chunk(3))
    reset()
    expect(events[events.length - 1]).toEqual({ type: 'failed', id: 1, reason: 'incomplete' })
    reset()
    expect(types().filter((t) => t === 'failed')).toHaveLength(1)
  })
})
