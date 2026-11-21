import { describe, it, expect, vi, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import { Sidebar } from '../Sidebar'
import type { Tab } from '../TabBar'
import type { SessionInfo } from '../../wailsjs/go/main/App'
import type { RemotePeerSessions } from '../../lib/remoteSession'
import { peerView } from '../../lib/sidebarPeers'

// Portty 02-04 (NAV-01..03): the sidebar is a macOS source list.

function session(id: string, name: string, extra: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id,
    name,
    state: 'running',
    status: 'running',
    createdAt: '2026-01-01T00:00:00Z',
    hostname: '',
    webEnabled: false,
    viewerCount: 0,
    workDir: '/',
    ...extra,
  }
}

type Props = Parameters<typeof Sidebar>[0]

let container: HTMLDivElement | null = null
let root: Root | null = null

function render(overrides: Partial<Props> = {}) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  const props: Props = {
    sessions: [],
    activeTab: null,
    onOpenLocal: vi.fn(),
    peerView: { kind: 'none' },
    onOpenPeer: vi.fn(),
    onHide: vi.fn(),
    onRename: vi.fn(),
    onEndSession: vi.fn(),
    onShareToggle: vi.fn(),
    onNewSession: vi.fn(),
    tailscaleHealth: { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'office-air.tail1.ts.net' },
    ...overrides,
  }
  act(() => root!.render(<Sidebar {...props} />))
  return { c: container, props }
}

afterEach(() => {
  if (root) act(() => root!.unmount())
  container?.remove()
  container = null
  root = null
})

const localTab = (s: SessionInfo): Tab => ({ id: s.id, name: s.name, sessionId: s.id })
const rows = (c: HTMLElement, groupLabel: string) => {
  const header = Array.from(c.querySelectorAll('h2')).find((h) => h.textContent === groupLabel)
  const section = header?.closest('section')
  return Array.from(section?.querySelectorAll<HTMLButtonElement>('button.sidebar__row') ?? [])
}

describe('Sidebar: This Mac (NAV-01)', () => {
  it('is a nav landmark named "Sessions"', () => {
    const { c } = render()
    const nav = c.querySelector('nav')
    expect(nav?.getAttribute('aria-label')).toBe('Sessions')
    expect(c.querySelectorAll('nav[aria-label="Sessions"]').length).toBe(1)
  })

  it('renders one row per session in the array order (no client sort)', () => {
    const sessions = [session('b', 'zeta'), session('a', 'alpha'), session('c', 'mid')]
    const { c } = render({ sessions })
    expect(rows(c, 'This Mac').map((b) => b.textContent)).toEqual(['zeta', 'alpha', 'mid'])
  })

  it('section is labelled by its "This Mac" header', () => {
    const { c } = render({ sessions: [session('a', 'x')] })
    const section = c.querySelector('section')!
    const header = c.querySelector(`#${section.getAttribute('aria-labelledby')}`)
    expect(header?.textContent).toBe('This Mac')
  })

  it('clicking a row calls onOpenLocal(id, name)', () => {
    const onOpenLocal = vi.fn()
    const { c } = render({ sessions: [session('a', 'alpha'), session('b', 'beta')], onOpenLocal })
    act(() => rows(c, 'This Mac')[1].click())
    expect(onOpenLocal).toHaveBeenCalledWith('b', 'beta')
  })

  it('two sessions with the same name render two rows, each opening its own id', () => {
    const onOpenLocal = vi.fn()
    const { c } = render({ sessions: [session('s1', 'shell'), session('s2', 'shell')], onOpenLocal })
    const r = rows(c, 'This Mac')
    expect(r.length).toBe(2)
    act(() => r[0].click())
    act(() => r[1].click())
    expect(onOpenLocal.mock.calls).toEqual([['s1', 'shell'], ['s2', 'shell']])
  })

  it('marks only the active local tab row with aria-current="page"', () => {
    const sessions = [session('a', 'alpha'), session('b', 'beta')]
    const { c } = render({ sessions, activeTab: localTab(sessions[1]) })
    const current = c.querySelectorAll('[aria-current="page"]')
    expect(current.length).toBe(1)
    expect(current[0].textContent).toBe('beta')
  })

  it('selects no local row when the active tab is not a local terminal tab', () => {
    const sessions = [session('a', 'alpha')]
    const { c } = render({
      sessions,
      // A peer tab that happens to share the local session's id still selects no local row.
      activeTab: { id: '__websession__a', name: 'peer › alpha', sessionId: 'a', type: 'web-session' },
    })
    expect(c.querySelectorAll('[aria-current="page"]').length).toBe(0)
  })

  // Quick 261116-c9f (R3): the empty group offers New Session instead of a dead line.
  it('with no sessions, This Mac shows one "New Session ⌘T" row that calls onNewSession', () => {
    const onNewSession = vi.fn()
    const { c } = render({ sessions: [], onNewSession })
    const section = c.querySelector('section[aria-labelledby="sidebar-group-local"]')!
    expect(section.querySelector('li.sidebar__muted')).toBeNull()
    const r = rows(c, 'This Mac')
    expect(r).toHaveLength(1)
    expect(r[0].getAttribute('aria-label')).toBe('New Session')
    expect(r[0].getAttribute('aria-keyshortcuts')).toBe('Meta+T')
    expect(r[0].textContent).toBe('New Session⌘T')
    expect(r[0].querySelector('svg')).not.toBeNull()
    act(() => r[0].click())
    expect(onNewSession).toHaveBeenCalledTimes(1)
  })

  // Quick 261116-ku9 (R4): while the main-area empty state shows, the empty
  // state's button is the only labelled New Session CTA; the header "+" stays.
  it('hideNewSessionRow drops the empty-group row but keeps the empty class and the header "+"', () => {
    const { c } = render({ sessions: [], hideNewSessionRow: true })
    const section = c.querySelector('section[aria-labelledby="sidebar-group-local"]')!
    expect(section.querySelector('.sidebar__row--action')).toBeNull()
    expect(section.classList.contains('sidebar__group--empty')).toBe(true)
    expect(section.querySelector('ul.sidebar__rows')).not.toBeNull()
    expect(c.querySelector('button.sidebar__group-add')!.getAttribute('aria-label')).toBe('New Session')
  })

  // Quick 261116-c9f (R2): a header "+" beside "This Mac", outside the h2.
  it.each([
    ['without sessions', [] as SessionInfo[]],
    ['with sessions', [session('a', 'alpha')]],
  ])('the This Mac "+" is present %s and calls onNewSession', (_label, sessions) => {
    const onNewSession = vi.fn()
    const { c } = render({ sessions, onNewSession })
    const btn = c.querySelector<HTMLButtonElement>('button.sidebar__group-add')!
    expect(btn).not.toBeNull()
    expect(btn.getAttribute('aria-label')).toBe('New Session')
    expect(btn.getAttribute('title')).toBe('New Session (⌘T)')
    expect(btn.getAttribute('aria-keyshortcuts')).toBe('Meta+T')
    const h2 = c.querySelector('#sidebar-group-local')!
    expect(h2.textContent).toBe('This Mac')
    expect(h2.querySelector('button')).toBeNull()
    act(() => btn.click())
    expect(onNewSession).toHaveBeenCalledTimes(1)
  })

  it('marks This Mac sidebar__group--empty only when there are no sessions', () => {
    const section = (c: HTMLElement) => c.querySelector('section[aria-labelledby="sidebar-group-local"]')!
    const { c } = render({ sessions: [] })
    expect(section(c).classList.contains('sidebar__group--empty')).toBe(true)
    act(() => root!.unmount())
    container!.remove()
    root = null
    const { c: c2 } = render({ sessions: [session('a', 'alpha')] })
    expect(section(c2).classList.contains('sidebar__group--empty')).toBe(false)
  })

  it('the This Mac and Other Macs headers each carry an aria-hidden glyph (R1)', () => {
    const { c } = render({ peerView: { kind: 'off' } })
    for (const label of ['This Mac', 'Other Macs']) {
      const h2 = Array.from(c.querySelectorAll('h2')).find((h) => h.textContent === label)
      expect(h2?.querySelector('svg[aria-hidden="true"]')).not.toBeNull()
    }
  })

  it('shows a Shared img only for web-enabled sessions', () => {
    const { c } = render({ sessions: [session('a', 'alpha', { webEnabled: true }), session('b', 'beta')] })
    const [a, b] = rows(c, 'This Mac')
    expect(a.querySelector('[role="img"][aria-label="Shared"]')).not.toBeNull()
    expect(b.querySelector('[role="img"]')).toBeNull()
  })

  it('exposes the full name in title for truncation', () => {
    const long = 'a-very-long-session-name-that-will-not-fit-in-the-sidebar'
    const { c } = render({ sessions: [session('a', long)] })
    expect(c.querySelector('.sidebar__row-name')?.getAttribute('title')).toBe(long)
  })

  it('has no Home, Hub, Help or Settings item (NAV-03)', () => {
    const { c } = render({ sessions: [session('a', 'alpha')] })
    const names = Array.from(c.querySelectorAll('button')).map(
      (b) => b.getAttribute('aria-label') ?? b.textContent,
    )
    for (const n of ['Home', 'Hub', 'Help', 'Settings']) expect(names).not.toContain(n)
  })
})

const peer = (hostname: string, reachable: boolean, sessions: [string, string][] = []): RemotePeerSessions => ({
  hostname,
  reachable,
  sessions: sessions.map(([id, name]) => ({ id, name, status: 'running', url: `https://${hostname}:8443/sessions/${id}` })),
})
const hostRow = (c: HTMLElement, host: string) =>
  Array.from(c.querySelectorAll<HTMLElement>('.sidebar__host')).find(
    (h) => h.querySelector('.sidebar__row-name')?.textContent === host,
  )
// Quick 261116-c9f: peer sessions nest under their host row inside "Other Macs".
const peerRows = (c: HTMLElement, host: string) =>
  Array.from(hostRow(c, host)?.closest('li')?.querySelectorAll<HTMLButtonElement>('button.sidebar__row') ?? [])
const mutedText = (c: HTMLElement) => Array.from(c.querySelectorAll('li.sidebar__muted')).map((l) => l.textContent)

describe('Sidebar: peer groups (NAV-02)', () => {
  it.each([
    ['off', 'Peers appear when Tailscale is connected.'],
    ['loading', 'Looking for other Macs…'],
    ['none', 'No shared sessions on other Macs.'],
  ] as const)('%s: one "Other Macs" section with the muted line', (kind, line) => {
    const { c } = render({ peerView: { kind } })
    expect(Array.from(c.querySelectorAll('h2')).map((h) => h.textContent)).toEqual(['This Mac', 'Other Macs'])
    expect(mutedText(c)).toContain(line)
  })

  it('a reachable peer is a host row inside one "Other Macs" section, its sessions nested beneath', () => {
    const p = peer('home-air.tail1.ts.net', true, [['p1', 'shell'], ['p2', 'logs']])
    const { c } = render({ peerView: peerView(false, true, [p]) })
    expect(Array.from(c.querySelectorAll('h2')).map((h) => h.textContent)).toEqual(['This Mac', 'Other Macs'])
    const host = hostRow(c, 'home-air')!
    expect(host.getAttribute('title')).toBe('home-air.tail1.ts.net')
    expect(host.closest('section')?.getAttribute('aria-labelledby')).toBe('sidebar-group-peers')
    const r = peerRows(c, 'home-air')
    expect(r.map((b) => b.textContent)).toEqual(['shell', 'logs'])
    expect(r[0].querySelector('[role="img"]')).toBeNull()
  })

  it('the host row dot is filled and "Reachable" when reachable, hollow and "Not reachable" when not', () => {
    const v = peerView(false, true, [peer('up.tail1.ts.net', true, [['u1', 'one']]), peer('down.tail1.ts.net', false)])
    const { c } = render({ peerView: v })
    const up = hostRow(c, 'up')!.querySelector('.sidebar__dot')!
    const down = hostRow(c, 'down')!.querySelector('.sidebar__dot')!
    expect(up.getAttribute('role')).toBe('img')
    expect(up.classList.contains('sidebar__dot--on')).toBe(true)
    expect(up.getAttribute('aria-label')).toBe('Reachable')
    expect(down.getAttribute('role')).toBe('img')
    expect(down.classList.contains('sidebar__dot--on')).toBe(false)
    expect(down.getAttribute('aria-label')).toBe('Not reachable')
  })

  it('UI polish: the dot is amber and "Reconnecting" for the reconnecting host only, even if the poll reads it unreachable', () => {
    const v = peerView(false, true, [peer('up.tail1.ts.net', true, [['u1', 'one']]), peer('down.tail1.ts.net', false)])
    const { c } = render({ peerView: v, reconnectingHost: 'down.tail1.ts.net' })
    const up = hostRow(c, 'up')!.querySelector('.sidebar__dot')!
    const down = hostRow(c, 'down')!.querySelector('.sidebar__dot')!
    expect(down.classList.contains('sidebar__dot--warn')).toBe(true)
    expect(down.getAttribute('aria-label')).toBe('Reconnecting')
    expect(up.classList.contains('sidebar__dot--warn')).toBe(false)
    expect(up.getAttribute('aria-label')).toBe('Reachable')
  })

  it("UI polish: each peer session row ends in that Mac's decorative colour dot", () => {
    const { c } = render({ peerView: peerView(false, true, [peer('home-air.tail1.ts.net', true, [['p1', 'shell']])]) })
    const dot = peerRows(c, 'home-air')[0].querySelector('.peer-dot') as HTMLElement
    expect(dot.getAttribute('aria-hidden')).toBe('true')
    expect(dot.style.background).toMatch(/^var\(--color-peer-[0-4]\)$/)
  })

  it('a reachable peer with nothing shared shows "Nothing shared" under its host, no rows', () => {
    const { c } = render({ peerView: peerView(false, true, [peer('idle.tail1.ts.net', true)]) })
    expect(peerRows(c, 'idle')).toHaveLength(0)
    const li = hostRow(c, 'idle')!.closest('li')!
    expect(Array.from(li.querySelectorAll('li.sidebar__muted')).map((l) => l.textContent)).toEqual(['Nothing shared'])
  })

  it('clicking a peer row calls onOpenPeer(peer, session)', () => {
    const onOpenPeer = vi.fn()
    const p = peer('home-air.tail1.ts.net', true, [['p1', 'shell'], ['p2', 'logs']])
    const { c } = render({ peerView: peerView(false, true, [p]), onOpenPeer })
    act(() => peerRows(c, 'home-air')[1].click())
    expect(onOpenPeer).toHaveBeenCalledWith(p, p.sessions[1])
  })

  it('an unreachable peer shows its host row and the can\'t-reach line, no rows', () => {
    const { c } = render({ peerView: peerView(false, true, [peer('down.tail1.ts.net', false)]) })
    expect(hostRow(c, 'down')).toBeDefined()
    expect(peerRows(c, 'down')).toHaveLength(0)
    const li = hostRow(c, 'down')!.closest('li')!
    expect(li.querySelector('li.sidebar__muted')?.textContent).toBe("Can't reach this Mac. Check that it's awake and on your tailnet.")
  })

  it('two peers sharing a first label render as two host rows', () => {
    const v = peerView(false, true, [
      peer('air.tail-a.ts.net', true, [['a1', 'one']]),
      peer('air.tail-b.ts.net', true, [['b1', 'two']]),
    ])
    const { c } = render({ peerView: v })
    const names = Array.from(c.querySelectorAll('.sidebar__host .sidebar__row-name')).map((n) => n.textContent)
    expect(names).toEqual(['air', 'air'])
  })

  it('aria-current marks the peer row of an active web-session tab, and only that row', () => {
    const p = peer('home-air.tail1.ts.net', true, [['p1', 'shell'], ['p2', 'logs']])
    const sessions = [session('l1', 'local')]
    const { c } = render({
      sessions,
      peerView: peerView(false, true, [p]),
      activeTab: { id: '__websession__home-air.tail1.ts.net/p2', name: 'home-air › logs', sessionId: 'p2', type: 'web-session', baseURL: 'https://h', capToken: 'x', peerHost: 'home-air.tail1.ts.net' },
    })
    const current = c.querySelectorAll('[aria-current="page"]')
    expect(current.length).toBe(1)
    expect(current[0].textContent).toBe('logs')
  })

  // INPT-04: two Macs can hand out the same session id; only the active tab's host row is current.
  it('with two hosts listing the same session id, aria-current marks only the active host\'s row', () => {
    const v = peerView(false, true, [
      peer('office-air.tail1.ts.net', true, [['s1', 'office shell']]),
      peer('studio-mac.tail1.ts.net', true, [['s1', 'studio shell']]),
    ])
    const { c } = render({
      peerView: v,
      activeTab: { id: '__websession__studio-mac.tail1.ts.net/s1', name: 'studio-mac › studio shell', sessionId: 's1', type: 'web-session', peerHost: 'studio-mac.tail1.ts.net' },
    })
    const current = c.querySelectorAll('[aria-current="page"]')
    expect(current.length).toBe(1)
    expect(current[0].textContent).toBe('studio shell')
  })
})

describe('Sidebar: Hide Sidebar button', () => {
  it('calls onHide', () => {
    const onHide = vi.fn()
    const { c } = render({ onHide })
    const btn = c.querySelector<HTMLButtonElement>('button[aria-label="Hide Sidebar"]')
    expect(btn?.querySelector('svg')).not.toBeNull()
    act(() => btn!.click())
    expect(onHide).toHaveBeenCalledTimes(1)
  })
})

describe('Sidebar: Tailscale footer', () => {
  const footer = (c: HTMLElement) => c.querySelector('[role="status"]')

  it('connected: dot plus "Tailscale connected · <short host>"', () => {
    const { c } = render()
    expect(footer(c)?.textContent).toBe('Tailscale connected · office-air')
    expect(footer(c)?.querySelector('.sidebar__dot--on')).not.toBeNull()
  })

  it.each([
    ['connected', { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'office-air.tail1.ts.net' }],
    ['not connected', { connected: false, hasCerts: false, ip: '', domain: '' }],
  ] as const)('%s: the footer text carries its full copy in title (quick 261116-c9f)', (_label, tailscaleHealth) => {
    const { c } = render({ tailscaleHealth })
    const text = c.querySelector('.sidebar__footer-text')!
    expect(text.getAttribute('title')).toBe(text.textContent)
  })

  it('connected with no domain falls back to the IP', () => {
    const { c } = render({ tailscaleHealth: { connected: true, hasCerts: false, ip: '100.64.0.9', domain: '' } })
    expect(footer(c)?.textContent).toBe('Tailscale connected · 100.64.0.9')
  })

  it('not connected', () => {
    const { c } = render({ tailscaleHealth: { connected: false, hasCerts: false, ip: '', domain: '' } })
    expect(footer(c)?.textContent).toBe('Tailscale not connected')
    expect(footer(c)?.querySelector('.sidebar__dot')).not.toBeNull()
    expect(footer(c)?.querySelector('.sidebar__dot--on')).toBeNull()
  })

  it('health unknown', () => {
    const { c } = render({ tailscaleHealth: null })
    expect(footer(c)?.textContent).toBe('Checking Tailscale…')
  })
})

// Source gate carried over from the old Sidebar test file; it guards another
// component and stays until that component changes.
const terminalPanelRaw = readFileSync(resolve(__dirname, '../TerminalPanel.tsx'), 'utf-8')

describe('Source-gate: POL-04 pendingThemeRef in TerminalPanel.tsx', () => {
  it('TerminalPanel.tsx contains pendingThemeRef', () => {
    expect(terminalPanelRaw).toContain('pendingThemeRef')
  })

  it('fitTerminal appears after clearTextureAtlas in TerminalPanel.tsx', () => {
    const clearIdx = terminalPanelRaw.indexOf('clearTextureAtlas')
    const fitIdx = terminalPanelRaw.indexOf('fitTerminal', clearIdx)
    expect(clearIdx).toBeGreaterThan(-1)
    expect(fitIdx).toBeGreaterThan(clearIdx)
  })
})

// Portty 02-08 (TERM-03): local rows get Rename · ─ · End Session… and in-place rename.
describe('Sidebar: local row menu and rename (Portty 02-08)', () => {
  const items = () => Array.from(document.querySelectorAll('[role="menu"] [role="menuitem"]')).map((b) => b.textContent)
  const clickItem = (label: string) =>
    act(() => Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((b) => b.textContent === label)!.click())

  it('right-click on a local row opens Rename · Share · ─ · End Session…; End Session… calls onEndSession(id)', () => {
    const onEndSession = vi.fn()
    const { c } = render({ sessions: [session('a', 'shell 1')], onEndSession })
    act(() => { rows(c, 'This Mac')[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })) })
    expect(items()).toEqual(['Rename', 'Share', 'End Session…'])
    expect(document.querySelector('[role="menu"] [role="separator"]')).not.toBeNull()
    clickItem('End Session…')
    expect(onEndSession).toHaveBeenCalledWith('a')
  })

  it('Shift+F10 on a focused local row opens the same menu', () => {
    const { c } = render({ sessions: [session('a', 'shell 1')] })
    const row = rows(c, 'This Mac')[0]
    act(() => { row.dispatchEvent(new KeyboardEvent('keydown', { key: 'F10', shiftKey: true, bubbles: true })) })
    expect(items()).toEqual(['Rename', 'Share', 'End Session…'])
  })

  it('double-clicking the name renames in place; Enter commits onRename(id, name); a single click still opens', () => {
    const onRename = vi.fn()
    const onOpenLocal = vi.fn()
    const { c } = render({ sessions: [session('a', 'shell 1')], onRename, onOpenLocal })
    act(() => rows(c, 'This Mac')[0].click())
    expect(onOpenLocal).toHaveBeenCalledWith('a', 'shell 1')
    act(() => { c.querySelector('.sidebar__row-name')!.dispatchEvent(new MouseEvent('dblclick', { bubbles: true })) })
    const input = c.querySelector<HTMLInputElement>('input[aria-label="Session name"]')!
    expect(input).not.toBeNull()
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, 'logs')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(onRename).toHaveBeenCalledWith('a', 'logs')
    expect(c.querySelector('input')).toBeNull()
  })

  // Portty 04-05 (UI-SPEC "Sidebar context menu"): Share / Stop Sharing act at once.
  const openMenu = (c: HTMLElement) =>
    act(() => { rows(c, 'This Mac')[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })) })
  const item = (label: string) =>
    Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((b) => b.textContent === label)!

  it('Share calls onShareToggle(id, true)', () => {
    const onShareToggle = vi.fn()
    const { c } = render({ sessions: [session('a', 'shell 1')], onShareToggle })
    openMenu(c)
    clickItem('Share')
    expect(onShareToggle).toHaveBeenCalledWith('a', true)
  })

  it('a shared row reads Rename · Stop Sharing · ─ · End Session…; Stop Sharing calls onShareToggle(id, false)', () => {
    const onShareToggle = vi.fn()
    const { c } = render({ sessions: [session('a', 'shell 1', { webEnabled: true })], onShareToggle })
    openMenu(c)
    expect(items()).toEqual(['Rename', 'Stop Sharing', 'End Session…'])
    clickItem('Stop Sharing')
    expect(onShareToggle).toHaveBeenCalledWith('a', false)
  })

  it('Tailscale off: Share is aria-disabled with aria-describedby="ts-off-reason" and does nothing', () => {
    const onShareToggle = vi.fn()
    const { c } = render({
      sessions: [session('a', 'shell 1')],
      onShareToggle,
      tailscaleHealth: { connected: false, hasCerts: false, ip: '', domain: '' },
    })
    openMenu(c)
    expect(item('Share').getAttribute('aria-disabled')).toBe('true')
    expect(item('Share').getAttribute('aria-describedby')).toBe('ts-off-reason')
    act(() => item('Share').click())
    expect(onShareToggle).not.toHaveBeenCalled()
  })

  it('Tailscale off: Stop Sharing stays enabled', () => {
    const onShareToggle = vi.fn()
    const { c } = render({
      sessions: [session('a', 'shell 1', { webEnabled: true })],
      onShareToggle,
      tailscaleHealth: { connected: false, hasCerts: false, ip: '', domain: '' },
    })
    openMenu(c)
    expect(item('Stop Sharing').hasAttribute('aria-disabled')).toBe(false)
    clickItem('Stop Sharing')
    expect(onShareToggle).toHaveBeenCalledWith('a', false)
  })

  it('the Rename menu item starts the in-place edit', () => {
    const { c } = render({ sessions: [session('a', 'shell 1')] })
    act(() => { rows(c, 'This Mac')[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })) })
    clickItem('Rename')
    expect(c.querySelector('input[aria-label="Session name"]')).not.toBeNull()
  })

  it('peer rows have no menu and no rename', () => {
    const peer: RemotePeerSessions = {
      hostname: 'home-air.tail1.ts.net',
      reachable: true,
      sessions: [{ id: 'p1', name: 'remote', status: 'running', url: 'https://home-air.tail1.ts.net:8443/sessions/p1' }],
    }
    const { c } = render({ peerView: peerView(false, true, [peer]) })
    const peerRow = peerRows(c, 'home-air')[0]
    act(() => { peerRow.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })) })
    expect(document.querySelector('[role="menu"]')).toBeNull()
    act(() => { peerRow.querySelector('.sidebar__row-name')!.dispatchEvent(new MouseEvent('dblclick', { bubbles: true })) })
    expect(c.querySelector('input')).toBeNull()
  })
})
