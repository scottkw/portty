import { describe, it, expect, vi, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import { Sidebar } from '../Sidebar'
import type { Tab } from '../TabBar'
import type { SessionInfo } from '../../wailsjs/go/main/App'

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
    funnelActive: false,
    workDir: '/',
    funnelWriteActive: false,
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
      activeTab: { id: '__settings__', name: 'Settings', sessionId: '', type: 'settings' },
    })
    expect(c.querySelectorAll('[aria-current="page"]').length).toBe(0)
  })

  it('shows a non-interactive "No sessions" line with an empty array', () => {
    const { c } = render({ sessions: [] })
    const muted = c.querySelector('li.sidebar__muted')
    expect(muted?.textContent).toBe('No sessions')
    expect(muted?.querySelector('button')).toBeNull()
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

// Source gates carried over from the old Sidebar test file; they guard other
// components and stay until those components change (02-05 deletes the Hub ones).
const terminalPanelRaw = readFileSync(resolve(__dirname, '../TerminalPanel.tsx'), 'utf-8')
const hubFilterBarRaw = readFileSync(resolve(__dirname, '../Hub/HubFilterBar.tsx'), 'utf-8')
const hubEmptyStateRaw = readFileSync(resolve(__dirname, '../Hub/HubEmptyState.tsx'), 'utf-8')

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

describe('Source-gate: POL-03 PlusIcon in HubFilterBar.tsx and HubEmptyState.tsx', () => {
  it('HubFilterBar.tsx contains PlusIcon', () => {
    expect(hubFilterBarRaw).toContain('PlusIcon')
  })

  it('HubEmptyState.tsx contains PlusIcon', () => {
    expect(hubEmptyStateRaw).toContain('PlusIcon')
  })
})
