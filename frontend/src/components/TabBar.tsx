import React, { useState, useRef, useEffect } from 'react'
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  DocumentIcon,
  DocumentTextIcon,
  GlobeAltIcon,
  PhotoIcon,
  PlusIcon,
  SignalIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline'
import { PeerDot, SidebarToggleIcon } from './Sidebar'
import { peerTabParts } from '../lib/tabs'
import { ContextMenu, isMenuKey, type ContextMenuItem } from './ContextMenu'
import { InlineSessionName } from './InlineSessionName'
import type { PreviewKind } from '../lib/previewAssembler'

export interface Tab {
  id: string
  name: string
  sessionId: string
  type?: 'terminal' | 'web-session' | 'preview'
  /** Phase 168-03 (FIX-03) — per-tab remote peer origin for 'web-session' tabs
   *  opened via handleOpenRemoteSession. undefined for the app's own web-share
   *  bootstrap tab (which falls back to the mount-stable webParams). */
  baseURL?: string
  /** Phase 168-03 (FIX-03) — per-tab capability token, paired with baseURL. */
  capToken?: string
  /** Portty 04-06: the peer's hostname for a peer tab (sidebar group key and banner host). */
  peerHost?: string
  /** Portty 05-05: the host ConnectPeerSession was called with (MagicDNS name or hostname); reused before each redial. */
  connectHost?: string
  /** Portty 04-06: ConnectPeerSession failed; the tab shows the connect-failed banner. */
  connectFailed?: boolean
  /** The host stopped sharing this peer tab's session; a row click reconnects (WR-06). */
  shareStopped?: boolean
  /** Phase 11: a preview tab's session tab id (its owner). A preview's sessionId is its own tab id. */
  ownerTabId?: string
  /** Phase 11: a preview tab's file type (its icon). */
  previewKind?: PreviewKind
}

/** UI-SPEC "Anatomy": the preview tab's leading type icon. */
function PreviewIcon({ kind }: { kind?: PreviewKind }): React.ReactElement {
  const Icon = kind === 'pdf' ? DocumentIcon : kind === 'markdown' ? DocumentTextIcon : PhotoIcon
  return <Icon className="tab__remote-icon" aria-hidden="true" />
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
  /** Portty 02-04: the sidebar is hidden, so the bar takes the 80px traffic-light inset and a Show Sidebar button. */
  sidebarHidden?: boolean
  onShowSidebar?: () => void
  /** Portty 02-08: the toolbar "+" button (desktop only). */
  onNewSession?: () => void
  /** Portty 04-05: toggles the Share popover for the active local session. */
  onShare?: () => void
  /** The Share popover is open (aria-expanded). */
  shareOpen?: boolean
  /** The popover's id (aria-controls). */
  sharePopoverId?: string
  /** The Share button, the popover's anchor. */
  shareButtonRef?: React.Ref<HTMLButtonElement>
  /** The active local session is web-shared: the button reads "Shared". */
  shared?: boolean
  /**
   * NET-01: App sets it while Tailscale is off and the session is not shared, so
   * turning Share on is blocked; the button stays focusable with aria-disabled and
   * the banner's reason. A shared session's button stays enabled so its popover
   * can turn Share off (04-09, user decision 2026-11-17).
   */
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
  sidebarHidden = false,
  onShowSidebar,
  onNewSession,
  onShare,
  shared = false,
  shareDisabled = false,
  shareOpen = false,
  sharePopoverId,
  shareButtonRef,
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
          const peer = peerTabParts(tab)
          const label = peer ? `${peer.session} · ${peer.host}` : tab.name
          return (
          <div
            key={tab.id}
            className={`tab${active ? ' tab--active' : ''}`}
            title={label}
          >
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
                id={`tab-${tab.id}`}
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
                {peer && <GlobeAltIcon className="tab__remote-icon" aria-hidden="true" />}
                {tab.type === 'preview' && <PreviewIcon kind={tab.previewKind} />}
                <span
                  className="tab__name"
                  onDoubleClick={webMode || !isLocal(tab) ? undefined : () => setEditingId(tab.id)}
                >
                  {peer ? <>{peer.session}<span className="tab__host"> · {peer.host}</span></> : tab.name}
                </span>
                {peer && tab.peerHost && <PeerDot hostname={tab.peerHost} />}
              </button>
            )}
            {/* Web share page: one static pill, no close (UI-SPEC "Web share page"). */}
            {!webMode && (
              <button
                type="button"
                className="tab__close"
                onClick={() => onClose(tab.id)}
                title="Close Tab"
                aria-label={`Close ${label}`}
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
          It toggles the Share popover (Portty 04-05). shareDisabled (Tailscale off and
          not shared): aria-disabled (not disabled) keeps it focusable with its name, the
          reason is the visible banner sentence, and it does nothing. A shared session's
          button stays enabled with Tailscale off, so its popover can turn Share off, and
          App keeps it enabled while the popover is open so it can close it (04 WR-05). */}
      {!webMode && activeTab && isLocal(activeTab) && onShare && (
        <button
          ref={shareButtonRef}
          type="button"
          className={`toolbar-share${shared ? ' toolbar-share--shared' : ''}`}
          aria-haspopup="dialog"
          aria-expanded={shareOpen}
          aria-controls={shareOpen ? sharePopoverId : undefined}
          {...(shareDisabled
            ? { 'aria-disabled': 'true' as const, 'aria-describedby': 'ts-off-reason' }
            : { onClick: onShare })}
        >
          {shareIcon}{shared ? 'Shared' : 'Share'}
        </button>
      )}
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
