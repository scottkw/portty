import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { ILink, Terminal } from '@xterm/xterm'
import { findPathLinks, stripLineCol, registerPathLinks } from '../pathLinks'
import type { ModifierMode } from '../openLink'
import fixture from './fixtures/path-click.json'

describe('findPathLinks against the shared fixture (PATH-02)', () => {
  for (const c of fixture.detect) {
    it(`detects ${JSON.stringify(c.line)}`, () => {
      const got = findPathLinks(c.line).map(({ text, request }) => ({ text, request }))
      expect(got).toEqual(c.links)
    })
  }
})

describe('findPathLinks extra cases', () => {
  const texts = (s: string) => findPathLinks(s).map((l) => l.text)
  it('finds nothing in bare names, lone prefixes, ~user and dates', () => {
    for (const s of ['notes.md', '/', './', '../', '~/', '//', '~user/x.png', '2026/11/21.', 'src/main/']) {
      expect(texts(s), s).toEqual([])
    }
  })
  it('keeps :line:col in the text but not in the request', () => {
    expect(findPathLinks('a/b.png:12')).toEqual([{ start: 0, end: 10, text: 'a/b.png:12', request: 'a/b.png' }])
    expect(stripLineCol('a/b.png:12:3')).toBe('a/b.png')
    expect(stripLineCol('a/b.png')).toBe('a/b.png')
  })
  it('stops at the space of a path with spaces', () => {
    expect(texts('/tmp/my file.png')).toEqual(['/tmp/my'])
  })
  it('never links text inside a URL', () => {
    expect(texts('(https://x.com/a/b.png)')).toEqual([])
    expect(texts('see /tmp/a.png and https://x.com/a/b.png')).toEqual(['/tmp/a.png'])
  })
})

// A fake terminal: rows of cells (wide characters take a width-2 cell then a width-0 spacer).
type Row = { text: string; wrapped?: boolean }
function fakeTerm(rows: Row[], cols = 20) {
  const lineOf = (r: Row) => {
    const cells: { chars: string; width: number }[] = []
    for (const ch of r.text) {
      const wide = ch.codePointAt(0)! > 0x2e80
      cells.push({ chars: ch, width: wide ? 2 : 1 })
      if (wide) cells.push({ chars: '', width: 0 })
    }
    while (cells.length < cols) cells.push({ chars: '', width: 1 })
    return {
      isWrapped: !!r.wrapped,
      length: cells.length,
      getCell: (x: number) => ({ getChars: () => cells[x].chars, getWidth: () => cells[x].width }),
    }
  }
  let provider: { provideLinks: (y: number, cb: (l: ILink[] | undefined) => void) => void } | null = null
  const providerDispose = vi.fn()
  const element = document.createElement('div')
  const term = {
    element,
    buffer: { active: { getLine: (i: number) => (rows[i] ? lineOf(rows[i]) : undefined) } },
    registerLinkProvider: (p: typeof provider) => { provider = p; return { dispose: providerDispose } },
    onResize: () => ({ dispose: () => {} }),
  } as unknown as Terminal
  const provide = (y: number) => {
    let out: ILink[] | undefined
    provider!.provideLinks(y, (l) => { out = l })
    return out
  }
  return { term, provide, element, providerDispose }
}

describe('rows to links (PATH-02)', () => {
  const opts = (over: Partial<Parameters<typeof registerPathLinks>[1]> = {}) => ({
    enabled: () => true, modifier: () => 'platform' as ModifierMode, onClick: vi.fn(), ...over,
  })

  it('gives a path split across two wrapped rows one link, on both rows', () => {
    const { term, provide } = fakeTerm([{ text: 'see /tmp/very/lo' }, { text: 'ng/name.png ok', wrapped: true }], 16)
    registerPathLinks(term, opts())
    for (const y of [1, 2]) {
      const links = provide(y)!
      expect(links).toHaveLength(1)
      expect(links[0].text).toBe('/tmp/very/long/name.png')
      expect(links[0].range).toEqual({ start: { x: 5, y: 1 }, end: { x: 11, y: 2 } })
    }
  })

  it('keeps the cell range right after a wide character', () => {
    const { term, provide } = fakeTerm([{ text: '日本 docs/a.png' }])
    registerPathLinks(term, opts())
    const [link] = provide(1)!
    expect(link.text).toBe('docs/a.png')
    // 日本 takes cells 1-4, a space is cell 5, so the path starts at x=6
    expect(link.range).toEqual({ start: { x: 6, y: 1 }, end: { x: 15, y: 1 } })
  })

  it('covers a path made of wide characters through the second cell of the last one', () => {
    const { term, provide } = fakeTerm([{ text: 'a/b.画' }])
    registerPathLinks(term, opts())
    expect(provide(1)![0].range).toEqual({ start: { x: 1, y: 1 }, end: { x: 6, y: 1 } })
  })

  it('does not offer a link for a row the match does not touch', () => {
    const { term, provide } = fakeTerm([{ text: '/tmp/a.png' }, { text: 'plain' }])
    registerPathLinks(term, opts())
    expect(provide(2)).toBeUndefined()
  })

  it('caps a 2049-character logical line without throwing', () => {
    const rows: Row[] = Array.from({ length: 103 }, (_, i) => ({ text: 'a'.repeat(20), wrapped: i > 0 }))
    const { term, provide } = fakeTerm(rows)
    registerPathLinks(term, opts())
    expect(() => provide(1)).not.toThrow()
    expect(() => provide(103)).not.toThrow()
  })

  it('offers nothing while disabled', () => {
    const { term, provide } = fakeTerm([{ text: '/tmp/a.png' }])
    registerPathLinks(term, opts({ enabled: () => false }))
    expect(provide(1)).toBeUndefined()
  })
})

describe('underline follows the modifier (PATH-02)', () => {
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')
  beforeEach(() => { Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true }) })
  afterEach(() => { if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform) })

  const setup = (modifier: ModifierMode = 'platform') => {
    const f = fakeTerm([{ text: '/tmp/a.png' }])
    const onClick = vi.fn()
    const reg = registerPathLinks(f.term, { enabled: () => true, modifier: () => modifier, onClick })
    const link = f.provide(1)![0]
    // xterm replaces decorations with a tracked object once the link is hovered
    link.hover?.(new MouseEvent('mousemove'), link.text)
    return { ...f, link, reg, onClick }
  }
  const key = (type: 'keydown' | 'keyup', init: KeyboardEventInit) => window.dispatchEvent(new KeyboardEvent(type, init))
  const state = (l: ILink) => [l.decorations!.underline, l.decorations!.pointerCursor]

  it('starts plain, shows both while the modifier is held, drops both on release', () => {
    const { link, reg } = setup()
    expect(state(link)).toEqual([false, false])
    key('keydown', { key: 'Meta', metaKey: true })
    expect(state(link)).toEqual([true, true])
    key('keyup', { key: 'Meta', metaKey: false })
    expect(state(link)).toEqual([false, false])
    reg.dispose()
  })

  it('drops both on window blur and on a hidden document', () => {
    const { link, reg } = setup()
    key('keydown', { key: 'Meta', metaKey: true })
    window.dispatchEvent(new Event('blur'))
    expect(state(link)).toEqual([false, false])
    key('keydown', { key: 'Meta', metaKey: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(state(link)).toEqual([false, false])
    reg.dispose()
  })

  it('picks the modifier up from a mouse move too (a keyup lost to another window)', () => {
    const { link, element, reg } = setup()
    element.dispatchEvent(new MouseEvent('mousemove', { metaKey: true }))
    expect(state(link)).toEqual([true, true])
    element.dispatchEvent(new MouseEvent('mousemove', { metaKey: false }))
    expect(state(link)).toEqual([false, false])
    reg.dispose()
  })

  it('with modifier none, links start underlined', () => {
    const { link, reg } = setup('none')
    expect(state(link)).toEqual([true, true])
    reg.dispose()
  })

  it('dispose removes the provider and the window listeners', () => {
    const { link, reg, providerDispose } = setup()
    reg.dispose()
    expect(providerDispose).toHaveBeenCalledTimes(1)
    key('keydown', { key: 'Meta', metaKey: true })
    expect(state(link)).toEqual([false, false])
  })
})

describe('activate is modifier-gated (PATH-02)', () => {
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')
  beforeEach(() => { Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true }) })
  afterEach(() => { if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform) })

  const activate = (modifier: ModifierMode, init: Partial<MouseEvent>) => {
    const f = fakeTerm([{ text: '/tmp/a.png:7' }])
    const onClick = vi.fn()
    const reg = registerPathLinks(f.term, { enabled: () => true, modifier: () => modifier, onClick })
    const link = f.provide(1)![0]
    link.activate({ metaKey: false, ctrlKey: false, ...init } as MouseEvent, link.text)
    reg.dispose()
    return onClick
  }

  it('metaKey on a Mac sends the request without :line', () => {
    const onClick = activate('platform', { metaKey: true })
    expect(onClick).toHaveBeenCalledWith('/tmp/a.png')
  })
  it('ctrlKey on a Mac does not', () => {
    expect(activate('platform', { ctrlKey: true })).not.toHaveBeenCalled()
  })
  it('no modifier does not', () => {
    expect(activate('platform', {})).not.toHaveBeenCalled()
  })
  it('with modifier ctrl, ctrlKey does', () => {
    expect(activate('ctrl', { ctrlKey: true })).toHaveBeenCalledWith('/tmp/a.png')
  })
})
