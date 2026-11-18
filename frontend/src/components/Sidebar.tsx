import React, { useState } from 'react'
import { CommandLineIcon, ComputerDesktopIcon, GlobeAltIcon, PlusIcon, SignalIcon } from '@heroicons/react/24/outline'
import type { Tab } from './TabBar'
import type { SessionInfo } from '../wailsjs/go/main/App'
import type { RemotePeerSessions, RemoteSession } from '../lib/remoteSession'
import type { TailscaleHealth } from '../lib/tailscale'
import { peerColorIndex, shortHost, type PeerView } from '../lib/sidebarPeers'
import { ContextMenu, isMenuKey } from './ContextMenu'
import { InlineSessionName } from './InlineSessionName'

const PEER_PLACEHOLDER: Record<'off' | 'loading' | 'none', string> = {
  off: 'Peers appear when Tailscale is connected.',
  loading: 'Looking for other Macs…',
  none: 'No shared sessions on other Macs.',
}
const PEER_UNREACHABLE = "Can't reach this Mac. Check that it's awake and on your tailnet."
const PEER_NOTHING_SHARED = 'Nothing shared'

/** Decorative per-Mac colour dot (sidebar peer rows and peer tabs); the host name carries the identity. */
export function PeerDot({ hostname }: { hostname: string }): React.ReactElement {
  return <span className="peer-dot" style={{ background: `var(--color-peer-${peerColorIndex(hostname)})` }} aria-hidden="true" />
}

/** UI-SPEC Iconography: the one non-Heroicons glyph (Hide/Show Sidebar). */
export function SidebarToggleIcon(): React.ReactElement {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth={1.6} aria-hidden="true">
      <rect x="3" y="4.5" width="18" height="15" rx="3" />
      <path d="M9 4.5v15" />
    </svg>
  )
}

// Portty 02-04 (NAV-01): macOS source-list sidebar. Desktop only: App does not
// render it in web mode (Phase 159 WEBCHAT-03 guest scope).
interface SidebarProps {
  /** Every daemon session, in the order the daemon returns them. */
  sessions: SessionInfo[]
  activeTab: Tab | null
  onOpenLocal: (sessionId: string, name: string) => void
  /** Peer area state from peerView(); see lib/sidebarPeers.ts. */
  peerView: PeerView
  onOpenPeer: (peer: RemotePeerSessions, session: RemoteSession) => void
  /** null until the first Tailscale check answers. */
  tailscaleHealth: TailscaleHealth | null
  onHide: () => void
  /** Portty 02-08 (TERM-03): local rows only. App owns the RPCs. */
  onRename: (sessionId: string, name: string) => void
  onEndSession: (sessionId: string) => void
  /** Portty 04-05: the row menu's Share / Stop Sharing; App flips the daemon state. */
  onShareToggle: (sessionId: string, shared: boolean) => void
  /** Opens the New Session dialog (same as ⌘T and the toolbar "+"), quick 261116-c9f. */
  onNewSession: () => void
  /** Quick 261116-ku9 R4: hide the empty-group New Session row while the main-area empty state shows. */
  hideNewSessionRow?: boolean
  /** Full hostname of the peer whose open tab is reconnecting: its dot turns amber. */
  reconnectingHost?: string | null
}

export function Sidebar({
  sessions,
  activeTab,
  onOpenLocal,
  peerView,
  onOpenPeer,
  tailscaleHealth,
  onHide,
  onRename,
  onEndSession,
  onShareToggle,
  onNewSession,
  hideNewSessionRow = false,
  reconnectingHost = null,
}: SidebarProps): React.ReactElement {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [menu, setMenu] = useState<{ id: string; x: number; y: number; invoker: HTMLElement } | null>(null)
  // Local terminal tabs carry no type (or 'terminal'); other tab kinds never select a local row.
  const activeLocalId =
    activeTab && (!activeTab.type || activeTab.type === 'terminal') ? activeTab.sessionId : null
  // Peer tabs are web-session tabs; session ids are random 16-byte hex, unique across machines.
  const activePeerId = activeTab?.type === 'web-session' ? activeTab.sessionId : null

  const footerText =
    tailscaleHealth === null
      ? 'Checking Tailscale…'
      : tailscaleHealth.connected
        ? `Tailscale connected · ${shortHost(tailscaleHealth.domain) || tailscaleHealth.ip}`
        : 'Tailscale not connected'

  return (
    <nav aria-label="Sessions" className="sidebar">
      {/* Sits in the 52px top strip, clear of the 80px traffic-light reserve. */}
      <button type="button" className="sidebar__hide" aria-label="Hide Sidebar" title="Hide Sidebar" onClick={onHide}>
        <SidebarToggleIcon />
      </button>
      <div className="sidebar__list">
        <section
          className={`sidebar__group${sessions.length === 0 ? ' sidebar__group--empty' : ''}`}
          aria-labelledby="sidebar-group-local"
        >
          <div className="sidebar__group-head">
            <h2 id="sidebar-group-local" className="sidebar__group-header">
              <ComputerDesktopIcon className="sidebar__group-icon" aria-hidden="true" />
              <span className="sidebar__group-label">This Mac</span>
            </h2>
            {/* A sibling of the h2, so the heading's name stays "This Mac". */}
            <button
              type="button"
              className="sidebar__group-add"
              aria-label="New Session"
              title="New Session (⌘T)"
              aria-keyshortcuts="Meta+T"
              onClick={onNewSession}
            >
              <PlusIcon aria-hidden="true" />
            </button>
          </div>
          <ul className="sidebar__rows">
            {sessions.length === 0 && !hideNewSessionRow && (
              <li>
                <button
                  type="button"
                  className="sidebar__row sidebar__row--action"
                  aria-label="New Session"
                  aria-keyshortcuts="Meta+T"
                  onClick={onNewSession}
                >
                  <PlusIcon className="sidebar__row-icon" aria-hidden="true" />
                  <span className="sidebar__row-name">New Session</span>
                  <span className="sidebar__row-hint" aria-hidden="true">⌘T</span>
                </button>
              </li>
            )}
            {sessions.map((s) => {
              const name = s.name || 'shell'
              if (editingId === s.id) {
                return (
                  <li key={s.id}>
                    <div className="sidebar__row sidebar__row--editing">
                      <CommandLineIcon className="sidebar__row-icon" aria-hidden="true" />
                      <InlineSessionName
                        id={s.id}
                        name={name}
                        editing
                        onEditingChange={(on) => { if (!on) setEditingId(null) }}
                        onRenamed={(n) => onRename(s.id, n)}
                      />
                    </div>
                  </li>
                )
              }
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    className="sidebar__row"
                    aria-current={s.id === activeLocalId ? 'page' : undefined}
                    onClick={() => onOpenLocal(s.id, s.name)}
                    onContextMenu={(e) => {
                      e.preventDefault()
                      setMenu({ id: s.id, x: e.clientX, y: e.clientY, invoker: e.currentTarget })
                    }}
                    onKeyDown={(e) => {
                      if (!isMenuKey(e)) return
                      e.preventDefault()
                      const r = e.currentTarget.getBoundingClientRect()
                      setMenu({ id: s.id, x: r.left, y: r.bottom, invoker: e.currentTarget })
                    }}
                  >
                    <CommandLineIcon className="sidebar__row-icon" aria-hidden="true" />
                    <span className="sidebar__row-name" title={name} onDoubleClick={() => setEditingId(s.id)}>{name}</span>
                    {s.webEnabled && (
                      <SignalIcon className="sidebar__row-icon" role="img" aria-hidden={false} aria-label="Shared" />
                    )}
                  </button>
                </li>
              )
            })}
          </ul>
        </section>

        <section className="sidebar__group" aria-labelledby="sidebar-group-peers">
          <div className="sidebar__group-head">
            <h2 id="sidebar-group-peers" className="sidebar__group-header">
              <GlobeAltIcon className="sidebar__group-icon" aria-hidden="true" />
              <span className="sidebar__group-label">Other Macs</span>
            </h2>
          </div>
          <ul className="sidebar__rows">
            {peerView.kind !== 'groups' ? (
              <li className="sidebar__muted">{PEER_PLACEHOLDER[peerView.kind]}</li>
            ) : (
              peerView.groups.map((g, i) => {
                // Reconnecting wins over the poll: a blocked link can read unreachable while the tab still retries.
                const reconnecting = g.key === reconnectingHost
                const status = reconnecting ? 'Reconnecting' : g.reachable ? 'Reachable' : 'Not reachable'
                return (
                  <li key={g.key} className="sidebar__host-item">
                    <div className="sidebar__host" title={g.key}>
                      <ComputerDesktopIcon className="sidebar__row-icon" aria-hidden="true" />
                      <span id={`sidebar-peer-${i}`} className="sidebar__row-name">{g.host}</span>
                      <span
                        className={`sidebar__dot${reconnecting ? ' sidebar__dot--warn' : g.reachable ? ' sidebar__dot--on' : ''}`}
                        role="img"
                        aria-label={status}
                        title={status}
                      />
                    </div>
                    <ul className="sidebar__rows sidebar__rows--nested" aria-labelledby={`sidebar-peer-${i}`}>
                      {/* The text line, not the dot colour, carries the state. */}
                      {!g.reachable && <li className="sidebar__muted">{PEER_UNREACHABLE}</li>}
                      {g.reachable && g.peer.sessions.length === 0 && (
                        <li className="sidebar__muted">{PEER_NOTHING_SHARED}</li>
                      )}
                      {g.reachable && g.peer.sessions.map((s) => (
                        <li key={s.id}>
                          <button
                            type="button"
                            className="sidebar__row"
                            aria-current={s.id === activePeerId ? 'page' : undefined}
                            onClick={() => onOpenPeer(g.peer, s)}
                          >
                            <CommandLineIcon className="sidebar__row-icon" aria-hidden="true" />
                            <span className="sidebar__row-name" title={s.name}>{s.name}</span>
                            <PeerDot hostname={g.key} />
                          </button>
                        </li>
                      ))}
                    </ul>
                  </li>
                )
              })
            )}
          </ul>
        </section>
      </div>

      {menu && (() => {
        // UI-SPEC "Sidebar context menu": Share acts at once (no ellipsis) and is
        // blocked while Tailscale is off; Stop Sharing always works.
        const shared = !!sessions.find((s) => s.id === menu.id)?.webEnabled
        const tailscaleOff = tailscaleHealth !== null && !tailscaleHealth.connected
        return (
          <ContextMenu
            x={menu.x}
            y={menu.y}
            invoker={menu.invoker}
            onClose={() => setMenu(null)}
            items={[
              { label: 'Rename', onSelect: () => setEditingId(menu.id) },
              shared
                ? { label: 'Stop Sharing', onSelect: () => onShareToggle(menu.id, false) }
                : { label: 'Share', onSelect: () => onShareToggle(menu.id, true), disabled: tailscaleOff, describedBy: 'ts-off-reason' },
              { label: 'End Session…', onSelect: () => onEndSession(menu.id), separatorBefore: true },
            ]}
          />
        )
      })()}

      <div className="sidebar__footer" role="status">
        <span
          className={`sidebar__dot${tailscaleHealth?.connected ? ' sidebar__dot--on' : ''}`}
          aria-hidden="true"
        />
        <span className="sidebar__footer-text" title={footerText}>{footerText}</span>
      </div>
    </nav>
  )
}
