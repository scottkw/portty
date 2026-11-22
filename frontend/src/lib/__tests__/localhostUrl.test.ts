/**
 * Phase 13 (URL-01): the TypeScript loopback classifier agrees with the Go one case for case through the
 * shared fixture (13-02), and survives hostile input.
 */
import { describe, it, expect } from 'vitest'
import fixture from './fixtures/localhost-url.json'
import { classifyLoopbackUrl, trimLinkTail } from '../localhostUrl'

type Case = { input: string; match: boolean; port?: number; rest?: string; note: string }
const cases = fixture.cases as Case[]

describe('classifyLoopbackUrl (shared fixture localhost-url.json)', () => {
  it('loads the full fixture', () => {
    expect(cases.length).toBeGreaterThanOrEqual(28)
  })

  it.each(cases.map((c) => [c.note, c] as const))('%s', (_note, c) => {
    const got = classifyLoopbackUrl(c.input)
    if (c.match) expect(got).toEqual({ port: c.port, rest: c.rest })
    else expect(got).toBeNull()
  })
})

describe('trimLinkTail', () => {
  it('keeps a balanced (...) and drops an unbalanced trailing )', () => {
    expect(trimLinkTail('http://localhost:1/wiki/Foo_(bar)')).toBe('http://localhost:1/wiki/Foo_(bar)')
    expect(trimLinkTail('http://localhost:1/x)')).toBe('http://localhost:1/x')
    expect(trimLinkTail('http://localhost:1/x).')).toBe('http://localhost:1/x')
  })
})

describe('classifyLoopbackUrl hostile input', () => {
  it('returns null for empty, scheme only and huge input without throwing', () => {
    expect(classifyLoopbackUrl('')).toBeNull()
    expect(classifyLoopbackUrl('http://')).toBeNull()
    expect(classifyLoopbackUrl('http://localhost:8000/' + 'a'.repeat(10000) + ' ')).toBeNull()
    expect(classifyLoopbackUrl('x'.repeat(10000))).toBeNull()
    expect(classifyLoopbackUrl(')'.repeat(10000))).toBeNull()
  })

  it('refuses whitespace and control characters in the rest', () => {
    expect(classifyLoopbackUrl('http://localhost:8000/a b')).toBeNull()
    expect(classifyLoopbackUrl('http://localhost:8000/a\u0000b')).toBeNull()
    expect(classifyLoopbackUrl('http://localhost:8000/a b')).toBeNull()
  })
})
