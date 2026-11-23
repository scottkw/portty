/**
 * Phase 175-06 (BUG-02 / #125) — SessionEndedBanner render tests.
 *
 * Fixed copy: "Session ended — the owner stopped this session."
 * Accessibility: role="status" + aria-live="polite" (mirrors WebGLRecoveryBanner).
 * Security (T-175-06-02): the raw CloseEvent.reason must NEVER be rendered
 * into the DOM, even when it is a hostile string (script tags, HTML, etc.).
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { SessionEndedBanner } from '../SessionEndedBanner'

function render(onDismiss: () => void, reason?: string) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(SessionEndedBanner, { onDismiss, reason }))
  })
  return { container, root }
}

describe('SessionEndedBanner', () => {
  let container: HTMLElement | undefined
  let root: Root | undefined

  afterEach(() => {
    if (root) {
      flushSync(() => root!.unmount())
      root = undefined
    }
    if (container) {
      container.remove()
      container = undefined
    }
    vi.clearAllMocks()
  })

  it('renders the fixed generic copy', () => {
    ;({ container, root } = render(vi.fn()))
    expect(container.textContent).toContain('Session ended — the owner stopped this session.')
  })

  it('has role="status" and aria-live="polite" (accessibility contract)', () => {
    ;({ container, root } = render(vi.fn()))
    const statusEl = container.querySelector('[role="status"]')
    expect(statusEl).not.toBeNull()
    expect(statusEl?.getAttribute('aria-live')).toBe('polite')
  })

  it('dismiss button has aria-label="Dismiss" and fires onDismiss when clicked', () => {
    const onDismiss = vi.fn()
    ;({ container, root } = render(onDismiss))
    const btn = container.querySelector('[aria-label="Dismiss"]') as HTMLButtonElement | null
    expect(btn).not.toBeNull()
    flushSync(() => { btn!.click() })
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('renders no raw reason text even when a benign reason is passed', () => {
    ;({ container, root } = render(vi.fn(), 'session ended'))
    // The passed reason string must never surface verbatim in the rendered
    // text alongside the fixed copy — only the fixed message is visible.
    expect(container.textContent).toBe('Session ended — the owner stopped this session.')
  })

  it('renders no raw reason text and injects no HTML when a hostile reason string is passed (T-175-06-02)', () => {
    const hostile = '<img src=x onerror=alert(1)><script>alert(document.cookie)</script>'
    ;({ container, root } = render(vi.fn(), hostile))

    // The hostile string must not appear anywhere in the rendered text.
    expect(container.textContent).not.toContain(hostile)
    expect(container.textContent).not.toContain('<script>')
    expect(container.textContent).not.toContain('onerror')
    // No injected <script> or <img> elements — dangerouslySetInnerHTML was
    // never used to render the reason.
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
    // The fixed copy is still the only visible text.
    expect(container.textContent).toBe('Session ended — the owner stopped this session.')
  })

  it('does not throw when reason is omitted', () => {
    expect(() => {
      ;({ container, root } = render(vi.fn()))
    }).not.toThrow()
  })

  it('is the info variant of the shared banner with an svg icon and an XMarkIcon Dismiss (Portty 02-10)', () => {
    ;({ container, root } = render(vi.fn()))
    const banner = container.querySelector('.banner')!
    expect(banner.classList.contains('banner--info')).toBe(true)
    expect(banner.querySelector('svg.banner__icon')?.getAttribute('aria-hidden')).toBe('true')
    const dismiss = banner.querySelector('.banner__dismiss')!
    expect(dismiss.getAttribute('aria-label')).toBe('Dismiss')
    expect(dismiss.querySelector('svg')).not.toBeNull()
    expect(dismiss.textContent).toBe('')
    expect(container.textContent).not.toContain('×')
  })

  // Portty 04-06 (SHAR-04): the stopped-sharing kind.
  function renderKind(props: { host?: string; reason?: string }) {
    const c = document.createElement('div')
    document.body.appendChild(c)
    const r = createRoot(c)
    flushSync(() => {
      r.render(React.createElement(SessionEndedBanner, { onDismiss: vi.fn(), kind: 'share-stopped', ...props }))
    })
    return { container: c, root: r }
  }

  it("kind 'share-stopped' with a host renders '<host> stopped sharing this session.' and never the reason", () => {
    ;({ container, root } = renderKind({ host: 'office-air', reason: 'share stopped <b>x</b>' }))
    expect(container.textContent).toBe('office-air stopped sharing this session.')
    expect(container.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite')
    expect(container.querySelector('[aria-label="Dismiss"]')).not.toBeNull()
  })

  it("kind 'share-stopped' without a host renders the fallback copy", () => {
    ;({ container, root } = renderKind({ reason: 'share stopped' }))
    expect(container.textContent).toBe('This session is no longer shared.')
  })

  // Phase 5 (SHAR-07): shown while a dropped guest socket is being replaced.
  it("kind 'reconnecting' renders the fixed copy with role status and no Dismiss button", () => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() => {
      root!.render(React.createElement(SessionEndedBanner, { kind: 'reconnecting', onDismiss: vi.fn(), reason: '<b>x</b>' }))
    })
    expect(container.textContent).toBe('Connection lost — reconnecting…')
    expect(container.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite')
    expect(container.querySelector('[aria-label="Dismiss"]')).toBeNull()
  })

  // B5: the browser's probes have failed for minutes; still retrying, never final.
  it.each([
    ['office-air', "Can't reach office-air. The session may have ended or the Mac may be asleep. Still retrying…"],
    [undefined, "Can't reach the host Mac. The session may have ended or the Mac may be asleep. Still retrying…"],
  ])("kind 'unreachable' with host %s renders the long-outage copy and no Dismiss button", (host, text) => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() => {
      root!.render(React.createElement(SessionEndedBanner, { kind: 'unreachable', host, onDismiss: vi.fn() }))
    })
    expect(container.textContent).toBe(text)
    expect(container.querySelector('[aria-label="Dismiss"]')).toBeNull()
  })

  // Phase 12 (PATH-02): the two notices a refused path click shows in the clicking pane.
  describe.each([
    ['path-unsupported', "Can't preview this file type. Portty previews images, PDFs and Markdown files."],
    ['path-too-large', 'Too large to preview. The limit is 20 MB.'],
  ] as const)('kind %s (PATH-02)', (kind, copy) => {
    function renderKind(extra: object = {}) {
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      flushSync(() => { root!.render(React.createElement(SessionEndedBanner, { kind, onDismiss: vi.fn(), ...extra })) })
      return container
    }

    it('renders exactly the fixed copy', () => {
      expect(renderKind().querySelector('.banner__message')?.textContent).toBe(copy)
    })

    it('is a polite status with the info icon and a Dismiss button', () => {
      const c = renderKind()
      const status = c.querySelector('[role="status"]')
      expect(status?.getAttribute('aria-live')).toBe('polite')
      expect(status?.classList.contains('banner--info')).toBe(true)
      expect(c.querySelector('.banner__icon')).not.toBeNull()
      expect(c.querySelector('[aria-label="Dismiss"]')).not.toBeNull()
    })

    it('never renders a reason', () => {
      expect(renderKind({ reason: '<b>hostile</b> wire text' }).textContent).not.toContain('hostile')
    })
  })

  // Phase 13 (URL-01): why a clicked remote localhost URL did not open.
  describe.each([
    ['port-host-outdated', 'office-air', 8000, 'office-air needs a Portty update to open its local pages.', 'The host Mac needs a Portty update to open its local pages.'],
    ['port-daemon-outdated', 'office-air', 8000, "Portty's background service on this Mac is too old to open remote pages. Relaunch Portty to update it.", "Portty's background service on this Mac is too old to open remote pages. Relaunch Portty to update it."],
    ['port-in-use', 'office-air', 8000, 'Port 8000 is in use on this Mac.', 'Port 8000 is in use on this Mac.'],
    ['port-refused', 'office-air', 8000, 'office-air refused access to port 8000.', 'The host Mac refused access to port 8000.'],
    ['port-unreachable', 'office-air', 8000, "Couldn't reach office-air.", "Couldn't reach the host Mac."],
  ] as const)('kind %s (URL-01)', (kind, host, port, copy, fallbackCopy) => {
    function renderKind(extra: object = {}) {
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      flushSync(() => { root!.render(React.createElement(SessionEndedBanner, { kind, host, port, onDismiss: vi.fn(), ...extra })) })
      return container
    }

    it('renders exactly the fixed copy', () => {
      expect(renderKind().querySelector('.banner__message')?.textContent).toBe(copy)
    })

    it('falls back to the generic host text when the host label is absent', () => {
      expect(renderKind({ host: undefined }).querySelector('.banner__message')?.textContent).toBe(fallbackCopy)
    })

    it('is a polite status with a Dismiss button', () => {
      const c = renderKind()
      expect(c.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite')
      expect(c.querySelector('[aria-label="Dismiss"]')).not.toBeNull()
    })
  })

  // Phase 14 (MOB-03): the Reload action.
  function renderReload(kind: string, onReload?: () => void) {
    const c = document.createElement('div')
    document.body.appendChild(c)
    const r = createRoot(c)
    flushSync(() => {
      r.render(React.createElement(SessionEndedBanner, { onDismiss: vi.fn(), onReload, kind } as never))
    })
    return { container: c, root: r }
  }
  const reloadBtn = (c: HTMLElement) =>
    Array.from(c.querySelectorAll('button')).find((b) => b.textContent === 'Reload') ?? null

  it.each(['share-stopped', 'ended', 'unreachable', 'reconnecting'])(
    "kind '%s' with onReload renders Reload before Dismiss and it fires onReload", (kind) => {
      const onReload = vi.fn()
      ;({ container, root } = renderReload(kind, onReload))
      const btn = reloadBtn(container)!
      expect(btn).not.toBeNull()
      expect(btn.className).toBe('banner__action')
      expect(btn.type).toBe('button')
      const dismiss = container.querySelector('.banner__dismiss')
      if (dismiss) expect(btn.compareDocumentPosition(dismiss) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      flushSync(() => { btn.click() })
      expect(onReload).toHaveBeenCalledTimes(1)
    })

  it('without onReload no kind renders Reload', () => {
    ;({ container, root } = renderReload('share-stopped'))
    expect(reloadBtn(container)).toBeNull()
  })

  it.each([
    'paste-interrupted', 'path-unsupported', 'path-too-large',
    'port-host-outdated', 'port-daemon-outdated', 'port-in-use', 'port-refused', 'port-unreachable',
  ])("kind '%s' never renders Reload even when onReload is passed", (kind) => {
    ;({ container, root } = renderReload(kind, vi.fn()))
    expect(reloadBtn(container)).toBeNull()
  })
})
