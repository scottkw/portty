import { describe, it, expect, afterEach, vi } from 'vitest'
import { act } from 'react'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { TabBar } from '../TabBar'
import raw from '../TabBar.tsx?raw'

// IN-03: derive the override type from the real component prop surface instead of
// a hand-maintained subset, so it can't drift from TabBar's actual props.
type TabBarOverrides = Partial<React.ComponentProps<typeof TabBar>>

function renderTabBar() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(
      React.createElement(TabBar, {
        tabs: [],
        activeId: null,
        onSelect: () => {},
        onClose: () => {},
        onRename: () => {},
      })
    )
  })
  return { container, root }
}

function renderTabBarWithTabs(overrides: TabBarOverrides = {}) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const props = {
    tabs: [
      { id: 'tab1', name: 'shell 1', sessionId: 'sess1' },
      { id: 'tab2', name: 'shell 2', sessionId: 'sess2' },
    ],
    activeId: 'tab1',
    onSelect: () => {},
    onClose: () => {},
    onRename: () => {},
    ...overrides,
  }
  flushSync(() => {
    root.render(React.createElement(TabBar, props))
  })
  return { container, root }
}

describe('TabBar: Show Sidebar (Portty 02-04)', () => {
  it('with sidebarHidden, renders a Show Sidebar button that calls onShowSidebar, and takes the inset class', () => {
    const onShowSidebar = vi.fn()
    const { container, root } = renderTabBarWithTabs({ sidebarHidden: true, onShowSidebar })
    const btn = container.querySelector<HTMLButtonElement>('button[aria-label="Show Sidebar"]')
    expect(btn).not.toBeNull()
    expect(container.querySelector('.tab-bar')?.classList.contains('tab-bar--sidebar-hidden')).toBe(true)
    flushSync(() => btn!.click())
    expect(onShowSidebar).toHaveBeenCalledTimes(1)
    flushSync(() => root.unmount())
    container.remove()
  })

  it('without sidebarHidden, no Show Sidebar button and no inset class', () => {
    const { container, root } = renderTabBarWithTabs()
    expect(container.querySelector('button[aria-label="Show Sidebar"]')).toBeNull()
    expect(container.querySelector('.tab-bar--sidebar-hidden')).toBeNull()
    flushSync(() => root.unmount())
    container.remove()
  })
})

describe('TabBar', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('renders tab-bar root element', () => {
    ;({ container, root } = renderTabBar())
    expect(container.querySelector('.tab-bar')).not.toBeNull()
  })

  it('renders tab-list container', () => {
    ;({ container, root } = renderTabBar())
    expect(container.querySelector('.tab-list')).not.toBeNull()
  })
})


// STRP-01: tabs carry no agent identity — no badge, no agent tooltip suffix.
describe('TabBar shows no agent identity (STRP-01)', () => {
  it('renders no .tab__agent-badge and the pill tooltip is just the name', () => {
    const { container, root } = renderTabBarWithTabs()
    expect(container.querySelector('.tab__agent-badge')).toBeNull()
    expect(container.querySelector('.tab')?.getAttribute('title')).toBe('shell 1')
    root.unmount()
    container.remove()
  })
})

// Phase 98 PRG-03 — Wave 0 RED scaffold.
// These two test cases are RED until Wave 3 (Plan 04) adds the .tab__progress
// underline element to TabBar.tsx. Tagged `progress-underline` and
// `progress-transform` for VALIDATION.md verify targeting.
// Using source-inspection (?raw) pattern consistent with the rest of this file.
describe('Phase 98 PRG-03: TabBar per-tab progress underline — Wave 0 RED scaffold (Plan 98-01 Wave 0)', () => {
  it('progress-underline: TabBarProps interface declares tabProgress?: Record<string, number>', () => {
    // RED until Wave 3 (Plan 04) extends the TabBarProps interface.
    expect(raw).toMatch(/tabProgress\?:\s*Record<string,\s*number>/)
  })

  it('progress-underline: renders .tab__progress element with data-testid keyed by tab.id', () => {
    // RED until Wave 3 adds the .tab__progress <div> inside the tab render.
    expect(raw).toContain('tab__progress')
    expect(raw).toMatch(/data-testid[^}]*tab-progress-/)
  })

  it('progress-transform: uses transform scaleX for the underline animation (not width)', () => {
    // RED until Wave 3. The production element must set transform: scaleX(...)
    // and must NOT set inline width (CSS width stays 100%; only transform changes).
    expect(raw).toMatch(/scaleX\(/)
  })
})

// ============================================================================
// Phase 139 Plan 01 — TabBar chevron + floor + rename + title tests (TAB-01..03)
//
// These tests are RED until Plan 02 adds:
//   - canScrollLeft / canScrollRight state + ResizeObserver in TabBar.tsx
//   - button[aria-label="Scroll tabs left"] and button[aria-label="Scroll tabs right"]
//   - title attribute on the OUTER .tab div (not just .tab__name)
// ============================================================================

describe('Phase 139 TAB-02: TabBar chevron overflow — RED until Plan 02', () => {
  // All tests in this describe are source-level `raw` string assertions and do not
  // render, so no container/root/afterEach teardown is needed.

  it('right chevron appears when tab list overflows scrollWidth > clientWidth', () => {
    // TabBar source must declare canScrollRight state driven by scrollWidth comparison.
    // This source-level check is RED until Plan 02 adds the canScrollRight state.
    expect(raw).toContain('canScrollRight')
    expect(raw).toContain('aria-label="Scroll tabs right"')
  })

  it('left chevron appears when scrollLeft > 0', () => {
    // TabBar source must declare canScrollLeft state.
    expect(raw).toContain('canScrollLeft')
    expect(raw).toContain('aria-label="Scroll tabs left"')
  })

  it('left chevron is absent at scrollLeft=0 (source guards on canScrollLeft)', () => {
    // The left chevron must be guarded by {canScrollLeft && ...} in JSX.
    // Plan 02 sets canScrollLeft = scrollLeft > 0; at initial render (scrollLeft=0)
    // the chevron must not appear. Source-level check confirms the guard exists.
    expect(raw).toMatch(/canScrollLeft\s*&&/)
  })

  it('right chevron is guarded on canScrollRight', () => {
    expect(raw).toMatch(/canScrollRight\s*&&/)
  })

  it('chevron buttons use ResizeObserver to detect overflow (source check)', () => {
    // Plan 02 must wire a ResizeObserver to the .tab-list element.
    expect(raw).toContain('ResizeObserver')
  })

  it('chevron onClick scrolls the tab list by a fixed step (source check)', () => {
    // Plan 02 must call scrollBy on listRef.current.
    expect(raw).toContain('scrollBy')
  })
})

describe('Phase 139 TAB-03: TabBar title on outer .tab div — RED until Plan 02', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('outer .tab div carries a title attribute equal to the full tab name tooltip', () => {
    // Currently (before Plan 02) the title is only on .tab__name (inner span).
    // Plan 02 must ALSO put title on the outer .tab div so it is discoverable
    // at icon-only floor when .tab__name is hidden by CSS.
    // This test is RED until Plan 02 moves/copies title to the outer .tab div.
    ;({ container, root } = renderTabBarWithTabs())

    const outerTab = container.querySelector('.tab') as HTMLElement
    expect(outerTab).not.toBeNull()
    const outerTitle = outerTab.getAttribute('title')
    expect(outerTitle).not.toBeNull()
    expect(typeof outerTitle).toBe('string')
    expect((outerTitle as string).length).toBeGreaterThan(0)
  })

  it('outer .tab div title contains the full tab name (not truncated)', () => {
    // The title on the outer .tab div must include the full tab name "shell 1".
    ;({ container, root } = renderTabBarWithTabs())

    const outerTab = container.querySelector('.tab') as HTMLElement
    const outerTitle = outerTab.getAttribute('title') || ''
    // Must contain the full tab name (not just "rename hint" text without the name).
    // Note: The existing title on .tab__name contains the rename hint, not the name.
    // Plan 02 must put the full name in the outer div's title attribute.
    expect(outerTitle).toContain('shell 1')
  })
})

// Portty 04-05 (SHAR-05): the tab exposure icons for public sharing are gone.
describe('TabBar has no public-exposure tab icons (Portty 04-05)', () => {
  // The UI-polish globe marks a remote (peer) tab, not internet exposure; it is the one GlobeAlt use.
  it('the source renders no exposure icon', () => {
    expect(raw).not.toMatch(/LockOpen|tab__internet-icon|tab__fullaccess-icon/)
    expect(raw.match(/<GlobeAltIcon[^>]*>/g)).toEqual(['<GlobeAltIcon className="tab__remote-icon" aria-hidden="true" />'])
  })
})

describe('TabBar remote tab label (UI polish)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('a peer tab shows a decorative globe, "session · host" with the host dimmed, and the Mac colour dot', () => {
    ;({ container, root } = renderTabBarWithTabs({
      tabs: [
        { id: 'tab1', name: 'shell 1', sessionId: 'sess1' },
        { id: '__websession__p', name: 'kens-macbook-air-1574 › shell 1', sessionId: 'p', type: 'web-session', peerHost: 'kens-macbook-air-1574.tail1.ts.net' },
      ],
    }))
    const [local, remote] = Array.from(container.querySelectorAll('[role="tab"]'))
    expect(remote.textContent).toBe('shell 1 · kens-macbook-air-1574')
    expect(remote.querySelector('.tab__host')?.textContent).toBe(' · kens-macbook-air-1574')
    expect(remote.querySelector('svg.tab__remote-icon')?.getAttribute('aria-hidden')).toBe('true')
    const dot = remote.querySelector('.peer-dot') as HTMLElement
    expect(dot.getAttribute('aria-hidden')).toBe('true')
    expect(dot.style.background).toMatch(/^var\(--color-peer-[0-4]\)$/)
    expect(remote.closest('.tab')?.getAttribute('title')).toBe('shell 1 · kens-macbook-air-1574')
    expect(remote.closest('.tab')?.querySelector('.tab__close')?.getAttribute('aria-label')).toBe('Close shell 1 · kens-macbook-air-1574')
    // Local tabs and the web-guest tab (no peerHost) stay plain.
    expect(local.textContent).toBe('shell 1')
    expect(local.querySelector('svg, .peer-dot')).toBeNull()
  })

  it('the web-guest bootstrap tab (web-session, no peerHost) gets no globe or host', () => {
    ;({ container, root } = renderTabBarWithTabs({
      webMode: true,
      tabs: [{ id: '__websession__s', name: 'Session', sessionId: 's', type: 'web-session' }],
      activeId: '__websession__s',
    }))
    const tab = container.querySelector('[role="tab"]')!
    expect(tab.textContent).toBe('Session')
    expect(tab.querySelector('svg, .peer-dot')).toBeNull()
  })
})

// TERM-05 / CONTEXT Amendment 2: tabs show no session status; the OSC 9;4
// progress underline stays.
describe('TabBar shows no session status', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    root.unmount()
    container.remove()
  })

  it('renders no status dot or status label inside a tab, while the progress underline still renders', () => {
    ;({ container, root } = renderTabBarWithTabs({ tabProgress: { sess1: 50 } }))
    const tab = container.querySelector('.tab') as HTMLElement
    expect(tab.querySelector('[role="img"]')).toBeNull()
    expect(tab.textContent).not.toMatch(/Running|Exited|Error/)
    expect(tab.querySelector('[aria-label="Running"], [aria-label="Exited"], [aria-label="Error"]')).toBeNull()
    const bar = container.querySelector('[data-testid="tab-progress-tab1"]') as HTMLElement
    expect(bar).not.toBeNull()
    expect(bar.style.transform).toBe('scaleX(0.5)')
  })
})

// Portty 02-08 (TERM-03, LOOK-01): unified toolbar, pill tabs, detach-only close.
describe('TabBar toolbar (Portty 02-08)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    flushSync(() => root.unmount())
    container.remove()
  })

  it('renders a tablist labelled "Open sessions" with one role=tab per pill; the active one is aria-selected', () => {
    ;({ container, root } = renderTabBarWithTabs())
    const list = container.querySelector('[role="tablist"]') as HTMLElement
    expect(list.getAttribute('aria-label')).toBe('Open sessions')
    const tabs = Array.from(list.querySelectorAll('[role="tab"]'))
    expect(tabs).toHaveLength(2)
    expect(tabs.map((t) => t.getAttribute('aria-selected'))).toEqual(['true', 'false'])
    expect(tabs[0].getAttribute('aria-controls')).toBe('tabpanel-tab1')
    expect(tabs.map((t) => t.getAttribute('tabindex'))).toEqual(['0', '-1'])
    expect(container.querySelectorAll('.tab.tab--active')).toHaveLength(1)
  })

  it('clicking a tab calls onSelect(id)', () => {
    const onSelect = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onSelect }))
    flushSync(() => (container.querySelectorAll('[role="tab"]')[1] as HTMLElement).click())
    expect(onSelect).toHaveBeenCalledWith('tab2')
  })

  it('ArrowRight / ArrowLeft move focus between tabs, wrapping', () => {
    ;({ container, root } = renderTabBarWithTabs())
    const tabs = Array.from(container.querySelectorAll<HTMLElement>('[role="tab"]'))
    tabs[0].focus()
    flushSync(() => tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })))
    expect(document.activeElement).toBe(tabs[1])
    flushSync(() => tabs[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })))
    expect(document.activeElement).toBe(tabs[0])
    flushSync(() => tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true })))
    expect(document.activeElement).toBe(tabs[1])
  })

  it('the close button is labelled "Close <name>" and calls onClose(id) only (detach, no select)', () => {
    const onClose = vi.fn()
    const onSelect = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onClose, onSelect }))
    const close = container.querySelector('button[aria-label="Close shell 2"]') as HTMLButtonElement
    expect(close).not.toBeNull()
    flushSync(() => close.click())
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onClose).toHaveBeenCalledWith('tab2')
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('the New Session button is labelled, titled with ⌘T, and calls onNewSession', () => {
    const onNewSession = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onNewSession }))
    const btn = container.querySelector('button[aria-label="New Session"]') as HTMLButtonElement
    expect(btn.getAttribute('title')).toBe('New Session (⌘T)')
    flushSync(() => btn.click())
    expect(onNewSession).toHaveBeenCalledTimes(1)
  })

  it('with no tabs the tablist is empty and "+" stays', () => {
    ;({ container, root } = renderTabBar())
    expect(container.querySelectorAll('[role="tab"]')).toHaveLength(0)
    expect(container.querySelector('button[aria-label="New Session"]')).not.toBeNull()
  })

  it('web mode: one static pill with no close button and no "+"', () => {
    ;({ container, root } = renderTabBarWithTabs({
      webMode: true,
      tabs: [{ id: '__websession__s', name: 'Session', sessionId: 's', type: 'web-session' }],
      activeId: '__websession__s',
    }))
    expect(container.querySelectorAll('[role="tab"]')).toHaveLength(1)
    expect(container.querySelector('.tab__close')).toBeNull()
    expect(container.querySelector('button[aria-label="New Session"]')).toBeNull()
  })
})

// Portty 02-08 (TERM-03): one shared context menu, in-place rename, End Session….
describe('TabBar context menu and rename (Portty 02-08)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    flushSync(() => root.unmount())
    container.remove()
    document.querySelectorAll('.context-menu').forEach((m) => m.remove())
  })

  const items = () => Array.from(document.querySelectorAll('[role="menu"] [role="menuitem"]')).map((b) => b.textContent)
  const clickItem = (label: string) => {
    const btn = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((b) => b.textContent === label)
    expect(btn, label).toBeDefined()
    flushSync(() => btn!.click())
  }
  const rightClick = (el: Element) =>
    flushSync(() => el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })))

  it('renders no session-menu chevron', () => {
    ;({ container, root } = renderTabBarWithTabs())
    expect(container.querySelector('[data-testid="tab-chevron"], .tab__chevron')).toBeNull()
    expect(raw).not.toContain('tab__chevron')
  })

  it('right-click on a local tab: Rename · Save Terminal As… · Close Tab · ─ · End Session…', () => {
    ;({ container, root } = renderTabBarWithTabs({ onEndSession: vi.fn(), onRequestSave: vi.fn() }))
    rightClick(container.querySelector('[role="tab"]')!)
    expect(items()).toEqual(['Rename', 'Save Terminal As…', 'Close Tab', 'End Session…'])
    const sep = document.querySelector('[role="menu"] [role="separator"]')
    expect(sep?.nextElementSibling?.textContent).toBe('End Session…')
  })

  it('Save Terminal As… is absent while the serialize plugin is off (no onRequestSave)', () => {
    ;({ container, root } = renderTabBarWithTabs({ onEndSession: vi.fn() }))
    rightClick(container.querySelector('[role="tab"]')!)
    expect(items()).toEqual(['Rename', 'Close Tab', 'End Session…'])
  })

  it('Shift+F10 on a focused tab opens the same menu, anchored under the tab', () => {
    ;({ container, root } = renderTabBarWithTabs({ onEndSession: vi.fn() }))
    const tab = container.querySelector<HTMLElement>('[role="tab"]')!
    tab.getBoundingClientRect = () => ({ left: 40, bottom: 44, right: 200, top: 16, x: 40, y: 16, width: 160, height: 28, toJSON: () => ({}) }) as DOMRect
    flushSync(() => tab.dispatchEvent(new KeyboardEvent('keydown', { key: 'F10', shiftKey: true, bubbles: true })))
    const menu = document.querySelector<HTMLElement>('[role="menu"]')!
    expect(menu).not.toBeNull()
    expect(menu.style.left).toBe('40px')
    expect(menu.style.top).toBe('44px')
    expect(document.activeElement?.textContent).toBe('Rename')
  })

  it('End Session… calls onEndSession(id) and never onClose', () => {
    const onEndSession = vi.fn()
    const onClose = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onEndSession, onClose }))
    rightClick(container.querySelectorAll('[role="tab"]')[1])
    clickItem('End Session…')
    expect(onEndSession).toHaveBeenCalledWith('tab2')
    expect(onClose).not.toHaveBeenCalled()
    expect(document.querySelector('[role="menu"]')).toBeNull()
  })

  it('Close Tab in the menu calls onClose(id)', () => {
    const onClose = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onClose }))
    rightClick(container.querySelector('[role="tab"]')!)
    clickItem('Close Tab')
    expect(onClose).toHaveBeenCalledWith('tab1')
  })

  it('a peer tab offers only Close Tab, and its name does not rename on double-click', () => {
    ;({ container, root } = renderTabBarWithTabs({
      onEndSession: vi.fn(),
      tabs: [{ id: '__websession__p', name: 'home-air › shell', sessionId: 'p', type: 'web-session' }],
      activeId: '__websession__p',
    }))
    rightClick(container.querySelector('[role="tab"]')!)
    expect(items()).toEqual(['Close Tab'])
    flushSync(() => container.querySelector('.tab__name')!.dispatchEvent(new MouseEvent('dblclick', { bubbles: true })))
    expect(container.querySelector('input[aria-label="Session name"]')).toBeNull()
  })

  it('web mode: no context menu and no double-click rename', () => {
    ;({ container, root } = renderTabBarWithTabs({ webMode: true }))
    rightClick(container.querySelector('[role="tab"]')!)
    expect(document.querySelector('[role="menu"]')).toBeNull()
    flushSync(() => container.querySelector('.tab__name')!.dispatchEvent(new MouseEvent('dblclick', { bubbles: true })))
    expect(container.querySelector('input')).toBeNull()
  })

  it('double-clicking a local tab name renames in place; Enter commits onRename(id, trimmed)', () => {
    const onRename = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onRename }))
    act(() => { container.querySelector('.tab__name')!.dispatchEvent(new MouseEvent('dblclick', { bubbles: true })) })
    const input = container.querySelector<HTMLInputElement>('input[aria-label="Session name"]')!
    expect(input).not.toBeNull()
    expect(input.value).toBe('shell 1')
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '  build  ')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(onRename).toHaveBeenCalledTimes(1)
    expect(onRename).toHaveBeenCalledWith('tab1', 'build')
    expect(container.querySelector('input')).toBeNull()
  })

  it('the Rename menu item starts the same in-place edit', () => {
    ;({ container, root } = renderTabBarWithTabs())
    rightClick(container.querySelector('[role="tab"]')!)
    act(() => clickItem('Rename'))
    expect(container.querySelector('input[aria-label="Session name"]')).not.toBeNull()
  })
})

// Portty 02-08 (ROADMAP SC5, NET-01): toolbar Share button and peer chip.
// Carries the NET-01 intent of the deleted StatusBar.shareSession.test.tsx,
// fixed per UI-REVIEW 01 #2/#3: aria-disabled + aria-describedby, stable name.
describe('TabBar Share button and "On <host>" chip (Portty 02-08)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    flushSync(() => root.unmount())
    container.remove()
  })

  const share = () => Array.from(container.querySelectorAll<HTMLButtonElement>('.toolbar-share'))
  const peerTabs = {
    tabs: [
      { id: 'tab1', name: 'shell 1', sessionId: 'sess1' },
      { id: '__websession__p', name: 'home-air › logs', sessionId: 'p', type: 'web-session' as const },
    ],
  }

  it('a local active tab shows "Share"; clicking opens the dialog via onShare', () => {
    const onShare = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onShare }))
    expect(share()).toHaveLength(1)
    expect(share()[0].textContent).toBe('Share')
    expect(share()[0].querySelector('svg')).not.toBeNull()
    expect(share()[0].hasAttribute('aria-disabled')).toBe(false)
    expect(share()[0].hasAttribute('aria-describedby')).toBe(false)
    flushSync(() => share()[0].click())
    expect(onShare).toHaveBeenCalledTimes(1)
  })

  it('reads "Shared" on the accent fill when the active session is web-shared', () => {
    const onShare = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onShare, shared: true }))
    expect(share()[0].textContent).toBe('Shared')
    expect(share()[0].classList.contains('toolbar-share--shared')).toBe(true)
    flushSync(() => share()[0].click())
    expect(onShare).toHaveBeenCalledTimes(1)
  })

  it('Tailscale off: aria-disabled="true" (not disabled), aria-describedby="ts-off-reason", keeps its Shared look, click and Enter do nothing', () => {
    const onShare = vi.fn()
    ;({ container, root } = renderTabBarWithTabs({ onShare, shareDisabled: true, shared: true }))
    const btn = share()[0]
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    expect(btn.disabled).toBe(false)
    expect(btn.getAttribute('aria-describedby')).toBe('ts-off-reason')
    expect(btn.hasAttribute('aria-label')).toBe(false)
    expect(btn.textContent).toBe('Shared')
    expect(btn.classList.contains('toolbar-share--shared')).toBe(true)
    btn.focus()
    expect(document.activeElement).toBe(btn)
    flushSync(() => btn.click())
    flushSync(() => btn.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })))
    expect(onShare).not.toHaveBeenCalled()
  })

  // Portty 04-05: the button opens the Share popover.
  it('carries aria-haspopup="dialog", aria-expanded and aria-controls while open', () => {
    ;({ container, root } = renderTabBarWithTabs({ onShare: vi.fn() }))
    expect(share()[0].getAttribute('aria-haspopup')).toBe('dialog')
    expect(share()[0].getAttribute('aria-expanded')).toBe('false')
    expect(share()[0].hasAttribute('aria-controls')).toBe(false)
    flushSync(() => root.unmount())
    container.remove()
    ;({ container, root } = renderTabBarWithTabs({ onShare: vi.fn(), shareOpen: true, sharePopoverId: 'share-popover' }))
    expect(share()[0].getAttribute('aria-expanded')).toBe('true')
    expect(share()[0].getAttribute('aria-controls')).toBe('share-popover')
  })

  it('exposes the button through shareButtonRef (the popover anchor)', () => {
    const ref = React.createRef<HTMLButtonElement>()
    ;({ container, root } = renderTabBarWithTabs({ onShare: vi.fn(), shareButtonRef: ref }))
    expect(ref.current).toBe(share()[0])
  })

  it('a peer active tab shows the "On <host>" chip instead of Share', () => {
    ;({ container, root } = renderTabBarWithTabs({ ...peerTabs, activeId: '__websession__p', onShare: vi.fn() }))
    expect(share()).toHaveLength(0)
    expect(container.querySelector('.toolbar-chip')?.textContent).toBe('On home-air')
  })

  it('neither Share nor the chip shows with no tabs or in web mode', () => {
    ;({ container, root } = renderTabBarWithTabs({ tabs: [], activeId: null, onShare: vi.fn() }))
    expect(share()).toHaveLength(0)
    expect(container.querySelector('.toolbar-chip')).toBeNull()
    flushSync(() => root.unmount())
    container.remove()
    ;({ container, root } = renderTabBarWithTabs({
      webMode: true,
      tabs: [{ id: '__websession__s', name: 'Session', sessionId: 's', type: 'web-session' }],
      activeId: '__websession__s',
      onShare: vi.fn(),
    }))
    expect(share()).toHaveLength(0)
    expect(container.querySelector('.toolbar-chip')).toBeNull()
  })

  it('TabBar never toggles sharing itself', () => {
    expect(raw).not.toContain('SetSessionShared')
  })

  it('scroll buttons are chevron icon buttons, not ‹ › entities', () => {
    expect(raw).not.toMatch(/&#8249;|&#8250;/)
    expect(raw).toContain('<ChevronLeftIcon aria-hidden="true" />')
    expect(raw).toContain('<ChevronRightIcon aria-hidden="true" />')
  })
})
