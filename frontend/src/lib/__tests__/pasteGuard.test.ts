/**
 * 08-REVIEW WR-01: a paste must not stay in xterm's helper textarea. Real
 * xterm 6.0.0 in jsdom; jsdom runs no default action, so the browser's
 * insertFromPaste is modelled: the text lands in the textarea unless the
 * event's default was prevented.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { Terminal } from '@xterm/xterm'
import { guardPasteInsert } from '../pasteGuard'
import panelSrc from '../../components/TerminalPanel.tsx?raw'

const big = 'x'.repeat(5 * 1024 * 1024) + '\r' // 5 MB, as in V-05

let term: Terminal | null = null
afterEach(() => { term?.dispose(); term = null })

function openTerm(): Terminal {
  const el = document.createElement('div')
  document.body.appendChild(el)
  term = new Terminal()
  term.open(el)
  return term
}

// Dispatch a paste as WebKit does, then apply its default insertion.
function paste(ta: HTMLTextAreaElement, text: string): ClipboardEvent {
  const ev = new Event('paste', { bubbles: true, cancelable: true }) as ClipboardEvent
  Object.defineProperty(ev, 'clipboardData', { value: { getData: (t: string) => (t === 'text/plain' ? text : '') } })
  ta.dispatchEvent(ev)
  if (!ev.defaultPrevented) ta.value += text
  return ev
}

describe('WR-01 guardPasteInsert', () => {
  it('without the guard the paste stays in the textarea (the bug)', () => {
    const t = openTerm()
    paste(t.textarea!, big)
    expect(t.textarea!.value.length).toBe(big.length)
  })

  it('leaves the textarea empty after a large paste; xterm still sends it once', () => {
    const t = openTerm()
    const sent: string[] = []
    t.onData((d) => sent.push(d))
    const unguard = guardPasteInsert(t.textarea)
    const ev = paste(t.textarea!, big)
    expect(ev.defaultPrevented).toBe(true)
    expect(t.textarea!.value).toBe('')
    expect(sent).toEqual([big])
    unguard()
    paste(t.textarea!, 'y')
    expect(t.textarea!.value).toBe('y') // removed
  })

  it('keeps bracketed paste', async () => {
    const t = openTerm()
    await new Promise<void>((r) => t.write('\x1b[?2004h', r))
    const sent: string[] = []
    t.onData((d) => sent.push(d))
    guardPasteInsert(t.textarea)
    paste(t.textarea!, 'echo hi\r')
    expect(sent).toEqual(['\x1b[200~echo hi\r\x1b[201~'])
    expect(t.textarea!.value).toBe('')
  })

  it('TerminalPanel guards right after term.open and unguards in cleanup', () => {
    expect(panelSrc).toMatch(/term\.open\(containerRef\.current\)\s*\n\s*const unguardPaste = guardPasteInsert\(term\.textarea\)/)
    expect(panelSrc).toMatch(/return \(\) => \{\s*\n\s*cancelled = true[^\n]*\n\s*unguardPaste\(\)/)
  })
})
