import React, { useState, useRef, useEffect } from 'react'
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  GlobeAltIcon,
  LockOpenIcon,
  PlusIcon,
  SignalIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline'
import { SidebarToggleIcon } from './Sidebar'
import { ContextMenu, isMenuKey, type ContextMenuItem } from './ContextMenu'
import { InlineSessionName } from './InlineSessionName'

export interface Tab {
  id: string
  name: string
  sessionId: string
  type?: 'terminal' | 'settings' | 'web-session'
  /** Phase 168-03 (FIX-03) — per-tab remote peer origin for 'web-session' tabs
   *  opened via handleOpenRemoteSession. undefined for the app's own web-share
   *  bootstrap tab (which falls back to the mount-stable webParams). */
  baseURL?: string
  /** Phase 168-03 (FIX-03) — per-tab capability token, paired with baseURL. */
  capToken?: string
}
interface TabBarProps {
  tabs: Tab[]
  activeId: string | null
  onSelect: (id: string) => void
  onClose: (id: string) => void
  onRename: (id: string, name: string) => void
  /** Portty 02-08 (TERM-03): End Session… on a local tab; App confirms, then kills. */
  onEndSession?: (id: string) => void
  /**
   * Phase 97 SER-01: invoked when user picks the save menu item from
   * the right-click context menu. The handler in App.tsx looks up the
   * tab's session, runs the saver closure, strips ANSI, sanitizes the
   * filename, and calls the SaveTerminalSession Wails RPC. Portty 02-08:
   * App passes it only while the serialize plugin is on, and the menu item
   * shows only when it is set.
   */
  onRequestSave?: (tabId: string) => void
  /**
   * Phase 98 PRG-02 — per-tab progress value (0-100) keyed by sessionId.
   * Populated by App.tsx handleProgressChange; consumed in Wave 3 (Plan 04)
   * to render the progress underline on each tab. Declared optional here so
   * the prop can be wired at Wave 2 and consumed at Wave 3 without a
   * TypeScript error between waves.
   */
  tabProgress?: Record<string, number>
  /**
   * Phase 159-04 (WEBCHAT-05) — web-share guest mode. The one session shows as
   * a static pill: no rename, no context menu (its items are Wails RPCs with no
   * browser bridge), no close button and no "+" (UI-SPEC "Web share page").
   */
  webMode?: boolean
  /**
   * Phase 166 / FUI-03 — per-session internet exposure state, keyed by sessionId.
   * When funnelActiveSessions[tab.sessionId] is true, the tab renders a globe icon
   * with aria-label/title "Internet exposed". Derived from sessions in App.tsx
   * (rides the existing 3s poll — no new interval added).
   */
  funnelActiveSessions?: Record<string, boolean>
  /**
   * Phase 171 / FNL-09 — per-session public write (FULL ACCESS) exposure
   * state, keyed by sessionId. When funnelWriteActiveSessions[tab.sessionId]
   * is true, the tab renders a LockOpenIcon after the internet icon (D-10
   * read-then-write order) with aria-label/title "Full access exposed —
   * command execution". Mirrors funnelActiveSessions exactly — derived from
   * the same sessions 3s poll in App.tsx, no new interval.
   */
  funnelWriteActiveSessions?: Record<string, boolean>
  /** Portty 02-04: the sidebar is hidden, so the bar takes the 80px traffic-light inset and a Show Sidebar button. */
  sidebarHidden?: boolean
  onShowSidebar?: () => void
  /** Portty 02-08: the toolbar "+" button (desktop only). */
  onNewSession?: () => void
  /** Portty 02-08 (ROADMAP SC5): opens the Share dialog for the active local session. */
  onShare?: () => void
  /** The active local session is web-shared: the button reads "Shared". */
  shared?: boolean
  /** NET-01: Tailscale is off. Share stays focusable with aria-disabled and the banner's reason. */
  shareDisabled?: boolean
}

/** A local terminal tab (no type, or 'terminal'): the only kind with Rename and End Session…. */
function isLocal(tab: Tab): boolean {
  return !tab.type || tab.type === 'terminal'
}

/**
 * Portty 02-08: the macOS unified toolbar. Pill tabs in a tablist, then the
 * New Session button. Right-click or Shift+F10 opens the tab menu;
 * double-clicking a local tab's name renames it in place.
 */
export function TabBar({
  tabs,
  activeId,
  onSelect,
  onClose,
  onRename,
  onEndSession,
  onRequestSave,
  tabProgress,
  webMode = false,
  funnelActiveSessions,
  funnelWriteActiveSessions,
  sidebarHidden = false,
  onShowSidebar,
  onNewSession,
  onShare,
  shared = false,
  shareDisabled = false,
}: TabBarProps): React.ReactElement {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [menu, setMenu] = useState<{ tab: Tab; x: number; y: number; invoker: HTMLElement } | null>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  // D-09: scroll-position-aware chevron state — ResizeObserver + scroll listener
  function checkScroll() {
    const el = listRef.current
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 0)
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1)
  }

  useEffect(() => {
    const el = listRef.current
    if (!el) return
    el.addEventListener('scroll', checkScroll, { passive: true })
    const ro = new ResizeObserver(checkScroll)
    ro.observe(el)
    checkScroll()
    return () => {
      el.removeEventListener('scroll', checkScroll)
      ro.disconnect()
    }
  }, [])
  // Adding or removing a tab changes scrollWidth, not the list's box, so the observer misses it.
  useEffect(checkScroll, [tabs.length])

  // ARIA tabs pattern: Left/Right move focus between tabs.
  function handleTabKeyDown(e: React.KeyboardEvent<HTMLButtonElement>) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    const all = Array.from(listRef.current?.querySelectorAll<HTMLElement>('[role="tab"]') ?? [])
    const i = all.indexOf(e.currentTarget)
    const next = all[(i + (e.key === 'ArrowRight' ? 1 : all.length - 1)) % all.length]
    e.preventDefault()
    next?.focus()
  }

  const activeTab = tabs.find((t) => t.id === activeId) ?? null
  const shareIcon = <SignalIcon className="toolbar-share__icon" aria-hidden="true" />

  // UI-SPEC "Context menus": a local tab gets the full menu, any other tab only Close Tab.
  function menuItems(tab: Tab): ContextMenuItem[] {
    const close = { label: 'Close Tab', onSelect: () => onClose(tab.id) }
    if (!isLocal(tab)) return [close]
    return [
      { label: 'Rename', onSelect: () => setEditingId(tab.id) },
      ...(onRequestSave ? [{ label: 'Save Terminal As…', onSelect: () => onRequestSave(tab.id) }] : []),
      close,
      ...(onEndSession ? [{ label: 'End Session…', onSelect: () => onEndSession(tab.id), separatorBefore: true }] : []),
    ]
  }

  return (
    <div className={`tab-bar${sidebarHidden ? ' tab-bar--sidebar-hidden' : ''}`}>
      {sidebarHidden && (
        <button
          type="button"
          className="tab-bar__show-sidebar"
          aria-label="Show Sidebar"
          title="Show Sidebar"
          onClick={onShowSidebar}
        >
          <SidebarToggleIcon />
        </button>
      )}
      {canScrollLeft && (
        <button
          type="button"
          className="tab-bar__icon-button"
          onClick={() => { listRef.current?.scrollBy({ left: -160, behavior: 'smooth' }) }}
          aria-label="Scroll tabs left"
        >
          <ChevronLeftIcon aria-hidden="true" />
        </button>
      )}
      <div className="tab-list" role="tablist" aria-label="Open sessions" ref={listRef}>
        {tabs.map((tab) => {
          const active = tab.id === activeId
          return (
          <div
            key={tab.id}
            className={`tab${active ? ' tab--active' : ''}`}
            title={tab.name}
          >
            {/* Phase 166 / FUI-03 — tab internet exposure indicator.
                COLORBLIND-SAFE: GlobeAltIcon shape carries state; color is reinforcement only.
                Text label is in aria-label + title — not rendered visually (D-09). */}
            {funnelActiveSessions?.[tab.sessionId] && (
              <span
                className="tab__internet-icon"
                aria-label="Internet exposed"
                title="Internet exposed"
              >
                <GlobeAltIcon aria-hidden="true" />
              </span>
            )}
            {/* Phase 171 / FNL-09 — tab full-access (public write) indicator.
                COLORBLIND-SAFE: LockOpenIcon shape carries state; color is
                reinforcement only. Rendered AFTER the internet icon
                (read-then-write order, D-10). */}
            {funnelWriteActiveSessions?.[tab.sessionId] && (
              <span
                className="tab__fullaccess-icon"
                aria-label="Full access exposed — command execution"
                title="Full access exposed — command execution"
              >
                <LockOpenIcon aria-hidden="true" />
              </span>
            )}
            {editingId === tab.id ? (
              <InlineSessionName
                id={tab.id}
                name={tab.name}
                editing
                onEditingChange={(on) => { if (!on) setEditingId(null) }}
                onRenamed={(n) => onRename(tab.id, n)}
              />
            ) : (
              <button
                type="button"
                role="tab"
                className="tab__label"
                aria-selected={active}
                aria-controls={`tabpanel-${tab.id}`}
                tabIndex={active ? 0 : -1}
                onClick={() => onSelect(tab.id)}
                onKeyDown={(e) => {
                  if (!webMode && isMenuKey(e)) {
                    e.preventDefault()
                    const r = e.currentTarget.getBoundingClientRect()
                    setMenu({ tab, x: r.left, y: r.bottom, invoker: e.currentTarget })
                    return
                  }
                  handleTabKeyDown(e)
                }}
                onContextMenu={webMode ? undefined : (e) => {
                  e.preventDefault()
                  setMenu({ tab, x: e.clientX, y: e.clientY, invoker: e.currentTarget })
                }}
              >
                <span
                  className="tab__name"
                  onDoubleClick={webMode || !isLocal(tab) ? undefined : () => setEditingId(tab.id)}
                >
                  {tab.name}
                </span>
              </button>
            )}
            {/* Web share page: one static pill, no close (UI-SPEC "Web share page"). */}
            {!webMode && (
              <button
                type="button"
                className="tab__close"
                onClick={() => onClose(tab.id)}
                title="Close Tab"
                aria-label={`Close ${tab.name}`}
              >
                <XMarkIcon aria-hidden="true" />
              </button>
            )}
            {/* Phase 98 PRG-02 — per-tab progress underline. transform (not width) for GPU
                compositor smoothness (Pitfall #4); transform-origin: left grows L→R; transition
                in style.css handles the 200ms ease-out animation on value changes. */}
            <div
              className="tab__progress"
              style={{
                transform: `scaleX(${(tabProgress?.[tab.sessionId] ?? 0) / 100})`,
              }}
              data-testid={`tab-progress-${tab.id}`}
            />
          </div>
          )
        })}
      </div>
      {canScrollRight && (
        <button
          type="button"
          className="tab-bar__icon-button"
          onClick={() => { listRef.current?.scrollBy({ left: 160, behavior: 'smooth' }) }}
          aria-label="Scroll tabs right"
        >
          <ChevronRightIcon aria-hidden="true" />
        </button>
      )}
      {!webMode && (
        <button
          type="button"
          className="tab-bar__icon-button"
          aria-label="New Session"
          title="New Session (⌘T)"
          onClick={onNewSession}
        >
          <PlusIcon aria-hidden="true" />
        </button>
      )}
      {/* Share for a local session, "On <host>" for a peer tab, nothing otherwise.
          Tailscale off: aria-disabled (not disabled) keeps it focusable with a stable
          name, the reason is the visible banner sentence, and there is no handler. */}
      {!webMode && activeTab && isLocal(activeTab) && onShare && (shareDisabled ? (
        <button type="button" className="toolbar-share" aria-disabled="true" aria-describedby="ts-off-reason">
          {shareIcon}Share
        </button>
      ) : (
        <button type="button" className={`toolbar-share${shared ? ' toolbar-share--shared' : ''}`} onClick={onShare}>
          {shareIcon}{shared ? 'Shared' : 'Share'}
        </button>
      ))}
      {!webMode && activeTab?.type === 'web-session' && (
        <span className="toolbar-chip">On {activeTab.name.split(' › ')[0]}</span>
      )}

      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          invoker={menu.invoker}
          onClose={() => setMenu(null)}
          items={menuItems(menu.tab)}
        />
      )}
    </div>
  )
}
