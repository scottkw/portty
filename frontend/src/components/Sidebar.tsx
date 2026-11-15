import React, { useState } from 'react'
import { CommandLineIcon, ComputerDesktopIcon, SignalIcon } from '@heroicons/react/24/outline'
import type { Tab } from './TabBar'
import type { SessionInfo } from '../wailsjs/go/main/App'
import type { RemotePeerSessions, RemoteSession } from '../lib/remoteSession'
import type { TailscaleHealth } from '../lib/tailscale'
import { shortHost, type PeerView } from '../lib/sidebarPeers'
import { ContextMenu, isMenuKey } from './ContextMenu'
import { InlineSessionName } from './InlineSessionName'

const PEER_PLACEHOLDER: Record<'off' | 'loading' | 'none', string> = {
  off: 'Peers appear when Tailscale is connected.',
  loading: 'Looking for other Macs…',
  none: 'No shared sessions on other Macs.',
}
const PEER_UNREACHABLE = "Can't reach this Mac. Check that it's awake and on your tailnet."

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
        <section className="sidebar__group" aria-labelledby="sidebar-group-local">
          <h2 id="sidebar-group-local" className="sidebar__group-header">This Mac</h2>
          <ul className="sidebar__rows">
            {sessions.length === 0 && <li className="sidebar__muted">No sessions</li>}
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

        {peerView.kind !== 'groups' ? (
          <section className="sidebar__group" aria-labelledby="sidebar-group-peers">
            <h2 id="sidebar-group-peers" className="sidebar__group-header">Other Macs</h2>
            <ul className="sidebar__rows">
              <li className="sidebar__muted">{PEER_PLACEHOLDER[peerView.kind]}</li>
            </ul>
          </section>
        ) : (
          peerView.groups.map((g, i) => (
            <section key={g.key} className="sidebar__group" aria-labelledby={`sidebar-group-peer-${i}`}>
              <h2 id={`sidebar-group-peer-${i}`} className="sidebar__group-header" title={g.key}>{g.host}</h2>
              <ul className="sidebar__rows">
                {!g.reachable && <li className="sidebar__muted">{PEER_UNREACHABLE}</li>}
                {g.reachable && g.peer.sessions.map((s) => (
                  <li key={s.id}>
                    <button
                      type="button"
                      className="sidebar__row"
                      aria-current={s.id === activePeerId ? 'page' : undefined}
                      onClick={() => onOpenPeer(g.peer, s)}
                    >
                      <ComputerDesktopIcon className="sidebar__row-icon" aria-hidden="true" />
                      <span className="sidebar__row-name" title={s.name}>{s.name}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ))
        )}
      </div>

      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          invoker={menu.invoker}
          onClose={() => setMenu(null)}
          items={[
            { label: 'Rename', onSelect: () => setEditingId(menu.id) },
            { label: 'End Session…', onSelect: () => onEndSession(menu.id), separatorBefore: true },
          ]}
        />
      )}

      <div className="sidebar__footer" role="status">
        <span
          className={`sidebar__dot${tailscaleHealth?.connected ? ' sidebar__dot--on' : ''}`}
          aria-hidden="true"
        />
        <span className="sidebar__footer-text">{footerText}</span>
      </div>
    </nav>
  )
}
