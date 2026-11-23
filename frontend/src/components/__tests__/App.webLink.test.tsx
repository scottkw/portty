// Portty 04-06 (SHAR-03, SHAR-04): the web share page (/app/) checks the link's
// cap with GET /api/sessions/{id}/info before opening its tab. 401/403 or an
// incomplete link shows Link No Longer Works with no tab and nothing from the URL;
// 200 opens the terminal tab and hands it the host from the session info.
import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { mountApp, unmountApp } from './appHarness'

vi.mock('../WebShareSessionView', () => ({
  WebShareSessionView: (p: { sessionId: string; hostLabel?: string; onOtherAccount?: () => void }) =>
    React.createElement('div', { 'data-testid': 'web-session-view', 'data-session': p.sessionId, 'data-host': p.hostLabel ?? '' },
      React.createElement('button', { 'data-testid': 'refuse', onClick: p.onOtherAccount })),
}))

const SID = 'sess-secret-42'

function atLink(query: string) {
  window.history.pushState({}, '', `/app/${query}`)
}

function stubInfo(status: number, body: unknown = {}, headers?: Record<string, string>) {
  const fetchMock = vi.fn(async () => new Response(status === 200 ? JSON.stringify(body) : 'forbidden', { status, headers }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(async () => {
  await unmountApp()
  vi.unstubAllGlobals()
  window.history.pushState({}, '', '/')
})

describe('App web mode — link states', () => {
  it('403 from the info check shows Link No Longer Works, no tab, and never the session id', async () => {
    atLink(`?session=${SID}&cap=revoked`)
    const fetchMock = stubInfo(403)
    const c = await mountApp({})
    expect(fetchMock).toHaveBeenCalledWith(`/api/sessions/${SID}/info?cap=revoked`)
    expect(c.textContent).toContain('Link No Longer Works')
    expect(c.querySelectorAll('[role="tab"]').length).toBe(0)
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    expect(c.textContent).not.toContain(SID)
  })

  it('403 with X-Portty shows the own-devices screen, no tab, never the session id', async () => {
    atLink(`?session=${SID}&cap=good`)
    stubInfo(403, {}, { 'X-Portty': '1' })
    const c = await mountApp({})
    expect(c.textContent).toContain('This link only works on your own devices')
    expect(c.textContent).not.toContain('Link No Longer Works')
    expect(c.querySelectorAll('[role="tab"]').length).toBe(0)
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    expect(c.textContent).not.toContain(SID)
  })

  it('a refusal from the reconnect probe replaces the open view with the own-devices screen (MOB-05)', async () => {
    atLink(`?session=${SID}&cap=good`)
    stubInfo(200, { id: SID, hostname: 'office-air' })
    const c = await mountApp({})
    const refuse = c.querySelector<HTMLButtonElement>('[data-testid="refuse"]')!
    await act(async () => { refuse.click() })
    expect(c.textContent).toContain('This link only works on your own devices')
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
  })

  it('503 opens the tab', async () => {
    atLink(`?session=${SID}&cap=good`)
    stubInfo(503)
    const c = await mountApp({})
    expect(c.textContent).not.toContain('Link No Longer Works')
    expect(c.querySelector('[data-testid="web-session-view"]')).not.toBeNull()
  })

  it('401 is a dead link too', async () => {
    atLink(`?session=${SID}&cap=bad`)
    stubInfo(401)
    const c = await mountApp({})
    expect(c.textContent).toContain('Link No Longer Works')
  })

  it('a missing cap shows the block without fetching', async () => {
    atLink(`?session=${SID}`)
    const fetchMock = stubInfo(200)
    const c = await mountApp({})
    expect(fetchMock).not.toHaveBeenCalled()
    expect(c.textContent).toContain('Link No Longer Works')
    expect(c.textContent).not.toContain(SID)
    expect(c.querySelectorAll('[role="tab"]').length).toBe(0)
  })

  it('200 opens the terminal tab with the host from the session info', async () => {
    atLink(`?session=${SID}&cap=good`)
    stubInfo(200, { id: SID, name: 'build', status: 'running', hostname: 'office-air', perms: ['read', 'write'] })
    const c = await mountApp({})
    expect(c.textContent).not.toContain('Link No Longer Works')
    expect(c.querySelectorAll('[role="tab"]').length).toBe(1)
    const view = c.querySelector('[data-testid="web-session-view"]')!
    expect(view.getAttribute('data-session')).toBe(SID)
    expect(view.getAttribute('data-host')).toBe('office-air')
  })

  it('a network error still opens the tab (the WebSocket reports the real state)', async () => {
    atLink(`?session=${SID}&cap=good`)
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('network') }))
    const c = await mountApp({})
    expect(c.querySelector('[data-testid="web-session-view"]')).not.toBeNull()
  })
})
