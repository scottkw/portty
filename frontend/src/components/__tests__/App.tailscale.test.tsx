/**
 * NET-01 — App.tsx Tailscale-off contract (CONTEXT Amendment 1).
 *
 * While Tailscale is not connected: turning Share on is blocked with the NET-01
 * reason, a shared session's Shared button still opens the popover so Share can
 * be turned off (04-09, user decision 2026-11-17), and the remote-peer poll is
 * skipped with remotePeers cleared. Focus and visibility changes recheck the
 * connection. Mostly source inspection (`App.tsx?raw`); the Share button cases
 * mount the real App (appHarness).
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import raw from '../../App.tsx?raw'
import { TAILSCALE_OFF_REASON } from '../../lib/tailscale'
import { mountApp, unmountApp, flush } from './appHarness'

// jsdom has no xterm.
vi.mock('../TerminalPanel', () => ({
  TerminalPanel: () => React.createElement('div', { 'data-testid': 'terminal-panel' }),
}))

/** Slice from `marker` to the first `end` after it. */
function slice(marker: string, end: string): string {
  const start = raw.indexOf(marker)
  if (start === -1) throw new Error(`${marker} not found in App.tsx`)
  const stop = raw.indexOf(end, start + marker.length)
  if (stop === -1) throw new Error(`${end} not found after ${marker}`)
  return raw.slice(start, stop)
}

const recheckEffect = () => slice("window.addEventListener('focus'", '}, [mode])')
const recheckEffectFull = () => slice('const recheck = () => {', '}, [mode])')
const remotePollStart = () => {
  const end = raw.indexOf('const peers = await GetRemoteSessionsWithMeta()')
  return raw.slice(raw.lastIndexOf('useEffect(() => {', end), end)
}

describe('App.tsx NET-01: focus and visibility recheck', () => {
  it('registers focus and visibilitychange listeners that call GetTailscaleStatus', () => {
    const body = recheckEffectFull()
    expect(body).toContain('GetTailscaleStatus()')
    expect(body).toContain('.catch(')
    // CR-01: the Go side emits tailscale:health; applying the reply here could
    // overwrite a newer event with a stale value.
    expect(body).not.toContain('setTailscaleHealth')
    expect(body).toContain("window.addEventListener('focus', recheck)")
    expect(body).toContain("document.addEventListener('visibilitychange', onVisibility)")
    expect(body).toContain("document.visibilityState === 'visible'")
  })

  it('removes both listeners in cleanup', () => {
    const body = recheckEffect()
    expect(body).toContain("window.removeEventListener('focus', recheck)")
    expect(body).toContain("document.removeEventListener('visibilitychange', onVisibility)")
  })

  it('is skipped in web mode', () => {
    const start = raw.lastIndexOf('useEffect(() => {', raw.indexOf('const recheck = () => {'))
    expect(raw.slice(start, raw.indexOf('const recheck = () => {'))).toContain("if (mode === 'web') return")
  })
})

describe('App.tsx NET-01: remote/peer controls while disconnected', () => {
  it('the remote poll clears remotePeers and returns before polling while tailscaleOff', () => {
    expect(remotePollStart()).toMatch(/if \(tailscaleOff\) \{\s*setRemotePeers\(\[\]\)\s*setRemoteLoaded\(false\)\s*return\s*\}/)
  })

  it('the remote poll has no tab gate (Portty 02-04 NAV-02)', () => {
    expect(remotePollStart()).not.toContain('activeId')
    expect(remotePollStart()).not.toContain('HUB_TAB')
    expect(slice('const peers = await GetRemoteSessionsWithMeta()', '}, [tailscaleOff])')).toContain('30_000')
  })

  it('the remote poll re-runs when tailscaleOff changes (resumes on reconnect)', () => {
    const body = slice('const peers = await GetRemoteSessionsWithMeta()', 'clearInterval(interval)')
    expect(body).toContain('setRemotePeers(peers ?? [])')
    expect(body).toContain('setRemoteLoaded(true)')
    expect(raw).toContain('}, [tailscaleOff])')
    expect(raw).not.toContain('}, [activeId, tailscaleOff])')
  })
})

// Iteration-2 WR-01: a tailscale:health event that lands while init/retryInit's
// Promise.all is pending is newer than the reply, so the reply must not win.
describe.each([
  ['init', () => slice('async function init()', 'void init()')],
  ['retryInit', () => slice('const retryInit = useCallback(', '}, [])')],
])('App.tsx NET-01: %s does not overwrite a newer health event', (_name, body) => {
  it('captures the event sequence before Promise.all and applies the reply only if unchanged', () => {
    const b = body()
    const seq = b.indexOf('const healthSeq = healthEventSeqRef.current')
    expect(seq).toBeGreaterThan(-1)
    expect(seq).toBeLessThan(b.indexOf('await Promise.all('))
    expect(b).toContain('if (healthEventSeqRef.current === healthSeq) setTailscaleHealth(health)')
    expect(b).not.toMatch(/^\s*setTailscaleHealth\(health\)/m)
  })
})

describe('App.tsx NET-01: tailscale:health handler', () => {
  it('bumps the event sequence before applying the event', () => {
    const h = slice("EventsOn('tailscale:health'", 'setTailscaleHealth(h)')
    expect(h).toContain('healthEventSeqRef.current++')
  })
})

describe('App.tsx NET-01: Share controls', () => {
  // Portty 02-08 / 04-09: the toolbar Share button is aria-disabled while Tailscale
  // is off and the session is not shared (TabBar.test.tsx covers aria-describedby).
  // WR-05: an open popover keeps the button working, so it can close the popover.
  it('TabBar receives shareDisabled from tailscaleOff && !activeShared && !sharePopoverOpen', () => {
    const idx = raw.indexOf('<TabBar')
    const props = raw.slice(idx, raw.indexOf('/>', idx))
    expect(props).toContain('shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}')
    expect(props).toContain('shared={activeShared}')
  })

  // Portty 04-05 (UI-SPEC): the popover stays open when Tailscale drops and shows
  // the reason itself (turning Share off must still work), so it gets the flag.
  it('passes tailscaleOff to the Share popover instead of closing it', () => {
    const idx = raw.indexOf('<SharePopover')
    expect(raw.slice(idx, raw.indexOf('/>', idx))).toContain('tailscaleOff={tailscaleOff}')
  })

  it('does not restate the NET-01 reason (TailscaleBanner renders the shared constant)', () => {
    expect(raw).not.toContain("Tailscale isn't connected")
    expect(raw).toContain('{tailscaleOff && <TailscaleBanner />}')
  })
})

describe('App.tsx NET-01: no Tailscale install flow', () => {
  it('does not import or call AutoInstallTailscale', () => {
    expect(raw).not.toContain('AutoInstallTailscale')
    expect(raw).not.toContain('tailscale:install')
  })
})

// 04-09 (UAT gap 3, user decision 2026-11-17: Button opens when shared).
describe('App — Tailscale off: the toolbar Share button (mounted)', () => {
  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  // The daemon's share state follows SetSessionShared, so a refetch after Share off reads false.
  function impl(webEnabled: boolean, setShared: (...a: unknown[]) => unknown) {
    let shared = webEnabled
    return {
      GetRelayPort: () => 0,
      GetTailscaleStatus: () => ({ connected: false, hasCerts: false, ip: '', domain: '' }),
      ListSessions: () => [{
        id: 'sess-1', name: 'shell 1', state: 'running', status: 'idle', createdAt: '', hostname: '',
        webEnabled: shared, viewerCount: 0, workDir: '/',
      }],
      GetSessionShare: () => ({ shared, url: shared ? 'https://office-air.tail1234.ts.net:7444/sessions/sess-1?cap=CAP' : '', errorCode: '', errorDetail: '' }),
      SetSessionShared: (...a: unknown[]) => {
        const r = setShared(...a) as { shared: boolean } | undefined
        if (r) shared = r.shared
        return r
      },
      HasCTDisclosure: () => true,
    }
  }

  const button = (c: HTMLElement) => c.querySelector<HTMLButtonElement>('.toolbar-share')!
  const dialog = () => document.querySelector<HTMLElement>('[role="dialog"][aria-label="Share “shell 1”"]')

  async function click(el: HTMLElement) {
    await act(async () => { el.click() })
    await flush()
  }

  it('a shared session: the Shared button opens the popover, which shows the reason and turns Share off', async () => {
    const setShared = vi.fn(() => ({ shared: false, url: '', errorCode: '', errorDetail: '' }))
    const c = await mountApp(impl(true, setShared))
    const btn = button(c)
    expect(btn.textContent).toBe('Shared')
    expect(btn.hasAttribute('aria-disabled')).toBe(false)
    expect(btn.hasAttribute('aria-describedby')).toBe(false)

    await click(btn)
    const d = dialog()
    expect(d, 'Share popover opens').not.toBeNull()
    expect(d!.textContent).toContain(TAILSCALE_OFF_REASON)
    expect(d!.querySelector('[aria-label="Share link"]')).toBeNull()
    const sw = d!.querySelector<HTMLButtonElement>('[role="switch"]')!
    expect(sw.getAttribute('aria-checked')).toBe('true')
    expect(sw.hasAttribute('aria-disabled')).toBe(false)

    await click(sw)
    expect(setShared).toHaveBeenCalledTimes(1)
    expect(setShared).toHaveBeenCalledWith('sess-1', false)

    // WR-05: now unshared, the open popover's button still closes it, then goes aria-disabled.
    expect(btn.textContent).toBe('Share')
    expect(btn.hasAttribute('aria-disabled')).toBe(false)
    expect(sw.getAttribute('aria-checked')).toBe('false')
    expect(sw.getAttribute('aria-disabled')).toBe('true')
    await click(btn)
    expect(dialog(), 'button closes the popover').toBeNull()
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    await click(btn)
    expect(dialog(), 'button cannot reopen it').toBeNull()
    expect(setShared).toHaveBeenCalledTimes(1)
  })

  it('an unshared session: the Share button is aria-disabled and opens nothing', async () => {
    const setShared = vi.fn()
    const c = await mountApp(impl(false, setShared))
    const btn = button(c)
    expect(btn.textContent).toBe('Share')
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    expect(btn.getAttribute('aria-describedby')).toBe('ts-off-reason')

    await click(btn)
    expect(dialog()).toBeNull()
    expect(setShared).not.toHaveBeenCalled()
  })
})
