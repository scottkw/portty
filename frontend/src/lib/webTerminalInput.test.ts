import { readFileSync } from 'fs'
import { resolve } from 'path'

// HARD-02: the browser share viewer (web/assets/terminal.js) is a plain script with no
// module exports, so load its pure framing prelude (everything before the URL parsing).
const src = readFileSync(resolve(__dirname, '../../../web/assets/terminal.js'), 'utf-8')
const prelude = src.slice(0, src.indexOf('// Extract session ID from URL path'))
const { makeInputFrames, MsgInput } = new Function(
  `${prelude}\nreturn { makeInputFrames, MsgInput };`,
)() as { makeInputFrames: (text: string) => Uint8Array[]; MsgInput: number }

const payloads = (frames: Uint8Array[]) => frames.flatMap((f) => Array.from(f.subarray(1)))

describe('web terminal.js makeInputFrames (HARD-02)', () => {
  it('short input is one MsgInput frame', () => {
    const frames = makeInputFrames('ls\r')
    expect(frames).toHaveLength(1)
    expect(Array.from(frames[0])).toEqual([MsgInput, 0x6c, 0x73, 0x0d])
  })

  it('a 100,000-byte paste becomes 4 ordered frames of at most 32768 wire bytes', () => {
    let text = ''
    for (let i = 0; text.length < 100_000; i++) text += `${i},`
    text = text.slice(0, 100_000)
    const frames = makeInputFrames(text)
    expect(frames).toHaveLength(4)
    for (const f of frames) {
      expect(f.length).toBeLessThanOrEqual(32768)
      expect(f[0]).toBe(MsgInput)
    }
    expect(payloads(frames)).toEqual(Array.from(new TextEncoder().encode(text)))
  })

  it('a multibyte character split across frames reassembles', () => {
    const text = '€'.repeat(20_000) // 60,000 bytes; 32767 is not a multiple of 3
    const frames = makeInputFrames(text)
    expect(frames).toHaveLength(2)
    expect(frames[1][1] & 0xc0).toBe(0x80) // frame 2 starts mid-character
    expect(new TextDecoder().decode(new Uint8Array(payloads(frames)))).toBe(text)
  })
})
