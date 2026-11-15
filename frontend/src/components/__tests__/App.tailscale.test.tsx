/**
 * NET-01 — App.tsx Tailscale-off contract (CONTEXT Amendment 1).
 *
 * While Tailscale is not connected: Share controls are disabled with the
 * NET-01 reason, an open Share dialog closes, and the remote-peer poll is
 * skipped with remotePeers cleared. Focus and visibility changes recheck
 * the connection. Source inspection (`App.tsx?raw`), like the other App tests.
 */
import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'

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
  it('StatusBar receives shareDisabledReason from tailscaleOff', () => {
    const idx = raw.indexOf('<StatusBar')
    expect(raw.slice(idx, raw.indexOf('/>', idx))).toContain('shareDisabledReason={tailscaleOff ? TAILSCALE_OFF_REASON : undefined}')
  })

  it('HubPanel receives shareDisabledReason from tailscaleOff', () => {
    const idx = raw.indexOf('<HubPanel')
    expect(raw.slice(idx, raw.indexOf('/>', idx))).toContain('shareDisabledReason={tailscaleOff ? TAILSCALE_OFF_REASON : undefined}')
  })

  it('closes an open Share dialog when tailscaleOff becomes true', () => {
    expect(raw).toMatch(/useEffect\(\(\) => \{\s*if \(tailscaleOff\) setShareModalSession\(null\)\s*\}, \[tailscaleOff\]\)/)
  })

  it('imports the shared NET-01 reason rather than restating it', () => {
    expect(raw).toContain("import { TAILSCALE_OFF_REASON, TailscaleHealth } from './lib/tailscale'")
    expect(raw).not.toContain("Tailscale isn't connected")
  })
})

describe('App.tsx NET-01: no Tailscale install flow', () => {
  it('does not import or call AutoInstallTailscale', () => {
    expect(raw).not.toContain('AutoInstallTailscale')
    expect(raw).not.toContain('tailscale:install')
  })
})
