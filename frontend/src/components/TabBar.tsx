import React, { useState, useRef, useEffect } from 'react'
import { GlobeAltIcon, LockOpenIcon } from '@heroicons/react/24/outline'

export interface Tab {
  id: string
  name: string
  sessionId: string
  type?: 'terminal' | 'welcome' | 'settings' | 'hub' | 'help' | 'web-session'
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
  /**
   * Phase 97 SER-01: invoked when user picks the save menu item from
   * the right-click context menu. The handler in App.tsx looks up the
   * tab's session, runs the saver closure, strips ANSI, sanitizes the
   * filename, and calls the SaveTerminalSession Wails RPC.
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
   * Phase 159-04 (WEBCHAT-05) — web-share guest mode. When true, the desktop-only
   * tab affordances are suppressed: no double-click/right-click rename and no
   * Rename / Save Terminal As menu items (both are Wails RPCs with no bridge in a
   * browser — they fail silently and only relabel the local tab). The × close
   * button stays. Guests get no session-menu chevron.
   */
  webMode?: boolean
  /**
   * Phase 166 / FUI-03 — per-session internet exposure state, keyed by sessionId.
   * When funnelActiveSessions[tab.sessionId] is true, the tab renders a globe icon
   * with aria-label/title "Internet exposed". Derived from hubSessions in App.tsx
   * (rides the existing 3s poll — no new interval added).
   */
  funnelActiveSessions?: Record<string, boolean>
  /**
   * Phase 171 / FNL-09 — per-session public write (FULL ACCESS) exposure
   * state, keyed by sessionId. When funnelWriteActiveSessions[tab.sessionId]
   * is true, the tab renders a LockOpenIcon after the internet icon (D-10
   * read-then-write order) with aria-label/title "Full access exposed —
   * command execution". Mirrors funnelActiveSessions exactly — derived from
   * the same hubSessions 3s poll in App.tsx, no new interval.
   */
  funnelWriteActiveSessions?: Record<string, boolean>
}

/**
 * Horizontal tab bar with add/rename/close controls.
 * Double-clicking or right-clicking a tab name enables inline editing.
 */
export function TabBar({
  tabs,
  activeId,
  onSelect,
  onClose,
  onRename,
  onRequestSave,
  tabProgress,
  webMode = false,
  funnelActiveSessions,
  funnelWriteActiveSessions,
}: TabBarProps): React.ReactElement {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const [contextMenu, setContextMenu] = useState<{ tabId: string; x: number; y: number } | null>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  // Focus the rename input as soon as it appears.
  useEffect(() => {
    if (editingId !== null) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [editingId])

  // Dismiss context menu on outside click or Escape.
  useEffect(() => {
    if (contextMenu === null) return
    function handleOutsideClick(_e: MouseEvent) {
      setContextMenu(null)
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setContextMenu(null)
    }
    document.addEventListener('mousedown', handleOutsideClick)
    document.addEventListener('keydown', handleEscape)
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [contextMenu])

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

  function startEdit(tab: Tab, e: React.MouseEvent) {
    e.stopPropagation()
    setEditingId(tab.id)
    setEditValue(tab.name)
  }

  function startEditById(tabId: string) {
    const tab = tabs.find(t => t.id === tabId)
    if (tab) {
      setEditingId(tab.id)
      setEditValue(tab.name)
    }
  }

  function commitEdit() {
    if (editingId !== null) {
      const trimmed = editValue.trim()
      if (trimmed.length > 0) {
        onRename(editingId, trimmed)
      }
      setEditingId(null)
    }
  }

  function cancelEdit() {
    setEditingId(null)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') commitEdit()
    if (e.key === 'Escape') cancelEdit()
  }

  return (
    <div className="tab-bar">
      {canScrollLeft && (
        <button
          className="tab-bar__chevron tab-bar__chevron--left"
          onClick={() => { listRef.current?.scrollBy({ left: -160, behavior: 'smooth' }) }}
          aria-label="Scroll tabs left"
          tabIndex={0}
        >&#8249;</button>
      )}
      <div className="tab-list" ref={listRef}>
        {tabs.map((tab) => {
          return (
          <div
            key={tab.id}
            className={`tab${tab.id === activeId ? ' tab--active' : ''}`}
            onClick={() => onSelect(tab.id)}
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
              <input
                ref={inputRef}
                className="tab__rename-input"
                value={editValue}
                onChange={(e) => setEditValue(e.target.value)}
                onBlur={commitEdit}
                onKeyDown={handleKeyDown}
                onClick={(e) => e.stopPropagation()}
              />
            ) : (
              <span
                className="tab__name"
                onDoubleClick={webMode ? undefined : (e) => startEdit(tab, e)}
                onContextMenu={webMode ? undefined : (e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  setContextMenu({ tabId: tab.id, x: e.clientX, y: e.clientY })
                }}
                title="Double-click or right-click to rename"
              >
                {tab.name}
              </span>
            )}
            {tab.sessionId && !webMode && (
              <button
                className="tab__chevron"
                data-testid="tab-chevron"
                title="Session menu"
                aria-label="Session menu"
                aria-haspopup="menu"
                aria-expanded={contextMenu?.tabId === tab.id}
                // WR-02 a11y: advertise the popup + reflect open state.
                // IN-01: stop mousedown so the document outside-click handler
                // doesn't pre-close the menu before this click can toggle it.
                onMouseDown={(e) => e.stopPropagation()}
                onClick={(e) => {
                  e.stopPropagation()
                  // Toggle: a click on the already-open chevron dismisses the menu.
                  if (contextMenu?.tabId === tab.id) {
                    setContextMenu(null)
                    return
                  }
                  const rect = (e.currentTarget as HTMLButtonElement).getBoundingClientRect()
                  setContextMenu({ tabId: tab.id, x: rect.left, y: rect.bottom })
                }}
              >
                ▾
              </button>
            )}
            <button
              className="tab__close"
              onClick={(e) => {
                e.stopPropagation()
                onClose(tab.id)
              }}
              title="Close tab"
              aria-label={`Close ${tab.name}`}
            >
              ×
            </button>
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
          className="tab-bar__chevron tab-bar__chevron--right"
          onClick={() => { listRef.current?.scrollBy({ left: 160, behavior: 'smooth' }) }}
          aria-label="Scroll tabs right"
          tabIndex={0}
        >&#8250;</button>
      )}

      {contextMenu && tabs.some(t => t.id === contextMenu.tabId) && (
        <div
          className="tab__context-menu"
          role="menu"
          style={{ position: 'fixed', top: contextMenu.y, left: contextMenu.x }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          {/* Rename + Save Terminal As are Wails RPCs with no browser bridge —
              hidden for web-share guests (WEBCHAT-05). */}
          {!webMode && (
          <button
            role="menuitem"
            className="tab__context-menu__item"
            onClick={() => {
              startEditById(contextMenu.tabId)
              setContextMenu(null)
            }}
          >
            Rename
          </button>
          )}
          {!webMode && (
          <button
            role="menuitem"
            className="tab__context-menu__item"
            onClick={() => {
              onRequestSave?.(contextMenu.tabId)
              setContextMenu(null)
            }}
          >
            Save Terminal As…
          </button>
          )}
        </div>
      )}
    </div>
  )
}
