import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { cleanPreviewName, createPreviewAssembler, PREVIEW_MAX_BYTES, type PreviewEvent } from '../previewAssembler'
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

describe('previewAssembler failure rules (PREV-03, T-11-16)', () => {
  const last = () => events[events.length - 1]

  it('a header over the cap gives start then failed tooLarge, and later chunks are ignored', () => {
    feed(header({ size: PREVIEW_MAX_BYTES + 1 }))
    expect(types()).toEqual(['start', 'failed'])
    expect(last()).toEqual({ type: 'failed', id: 1, reason: 'tooLarge' })
    feed(chunk(5))
    feed(end(0))
    expect(types()).toEqual(['start', 'failed'])
  })

  it('a header exactly at the cap is accepted', () => {
    feed(header({ size: PREVIEW_MAX_BYTES }))
    expect(types()).toEqual(['start'])
  })

  it('bytes past the cap fail tooLarge (a size can only be declared under the cap)', () => {
    feed(header({ size: PREVIEW_MAX_BYTES }))
    const big: PreviewFrame = { kind: 'chunk', id: 1, payload: new Uint8Array(PREVIEW_MAX_BYTES + 1) }
    feed(big)
    expect(last()).toEqual({ type: 'failed', id: 1, reason: 'tooLarge' })
  })

  it('bytes past the declared size (within the cap) fail incomplete; nothing more is accepted', () => {
    feed(header({ size: 4 }))
    feed(chunk(3))
    feed(chunk(2))
    expect(last()).toEqual({ type: 'failed', id: 1, reason: 'incomplete' })
    feed(end(0))
    expect(types()).not.toContain('complete')
  })

  it('end status 2 is tooLarge, status 1 is incomplete', () => {
    feed(header({ size: 2 }))
    feed(chunk(1))
    feed(end(2))
    expect(last()).toEqual({ type: 'failed', id: 1, reason: 'tooLarge' })
    feed(header({ id: 2, size: 2 }))
    feed(chunk(1, 2))
    feed(end(1, 2))
    expect(last()).toEqual({ type: 'failed', id: 2, reason: 'incomplete' })
    expect(types()).not.toContain('complete')
  })

  it('an ok end with fewer bytes than declared is incomplete', () => {
    feed(header({ size: 10 }))
    feed(chunk(9))
    feed(end(0))
    expect(last()).toEqual({ type: 'failed', id: 1, reason: 'incomplete' })
  })

  it('a second header fails the first as incomplete and starts the second', () => {
    feed(header({ id: 1, size: 4 }))
    feed(chunk(2))
    feed(header({ id: 2, size: 1 }))
    expect(events.slice(-2).map((e) => e.type)).toEqual(['failed', 'start'])
    expect(events[events.length - 2]).toEqual({ type: 'failed', id: 1, reason: 'incomplete' })
    feed(chunk(1, 2))
    feed(end(0, 2))
    expect(last()).toMatchObject({ type: 'complete', id: 2 })
  })

  it('chunks and ends for another id are ignored', () => {
    feed(header({ size: 2 }))
    feed(chunk(2, 9))
    feed(end(0, 9))
    expect(types()).toEqual(['start'])
    feed(chunk(2))
    feed(end(0))
    expect(last()).toMatchObject({ type: 'complete', id: 1 })
  })

  it('an unknown MIME emits nothing and warns', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    feed(header({ mime: 'text/html' }))
    feed(chunk(10))
    feed(end(0))
    expect(events).toEqual([])
    expect(warn).toHaveBeenCalled()
  })

  for (const key of ['0123456789ABCDEF0123456789ABCDEF', 'abc', `${KEY}0`, '0123456789abcdef0123456789abcdeg']) {
    it(`a key that is not 32 lowercase hex (${key}) emits nothing`, () => {
      feed(header({ key }))
      expect(events).toEqual([])
    })
  }

  it('progress events are at least 100 ms apart and the last one equals the size', () => {
    feed(header({ size: 100 }))
    for (let i = 0; i < 100; i++) {
      feed(chunk(1))
      vi.advanceTimersByTime(7)
    }
    const prog = events.filter((e) => e.type === 'progress') as Extract<PreviewEvent, { type: 'progress' }>[]
    expect(prog.length).toBeGreaterThan(1)
    expect(prog.length).toBeLessThan(20)
    expect(prog[prog.length - 1].received).toBe(100)
    // Every progress event but the forced last one is throttled.
    const times: number[] = []
    events.length = 0
    const a = createPreviewAssembler((e) => { if (e.type === 'progress') times.push(Date.now()) })
    a.feed(header({ size: 1000 }))
    for (let i = 0; i < 999; i++) {
      a.feed(chunk(1))
      vi.advanceTimersByTime(3)
    }
    for (let i = 1; i < times.length; i++) expect(times[i] - times[i - 1]).toBeGreaterThanOrEqual(100)
  })

  it('cleanPreviewName strips controls and bidi overrides, caps at 255 characters and maps empty to file', () => {
    expect(cleanPreviewName('gnp‮.exe')).toBe('gnp.exe')
    expect(cleanPreviewName('a⁦b\u0007c\u0085d')).toBe('abcd')
    expect(cleanPreviewName('‪‫‬‭⁧⁨⁩\u0000\u001f\u007f\u009f')).toBe('file')
    expect(cleanPreviewName('')).toBe('file')
    expect(cleanPreviewName('x'.repeat(300))).toBe('x'.repeat(255))
    expect(Array.from(cleanPreviewName('\u{1F600}'.repeat(300)))).toHaveLength(255) // code points, never a split pair
  })

  it('the start event carries the cleaned name', () => {
    feed(header({ name: 'evil‮gnp.exe' }))
    expect(events[0]).toMatchObject({ name: 'evilgnp.exe' })
  })
})
