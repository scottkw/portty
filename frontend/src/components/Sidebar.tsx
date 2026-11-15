import React from 'react'
import { CommandLineIcon, ComputerDesktopIcon, SignalIcon } from '@heroicons/react/24/outline'
import type { Tab } from './TabBar'
import type { SessionInfo } from '../wailsjs/go/main/App'
import type { RemotePeerSessions, RemoteSession } from '../lib/remoteSession'
import type { TailscaleHealth } from '../lib/tailscale'
import { shortHost, type PeerView } from '../lib/sidebarPeers'

const PEER_PLACEHOLDER: Record<'off' | 'loading' | 'none', string> = {
  off: 'Peers appear when Tailscale is connected.',
  loading: 'Looking for other Macs…',
  none: 'No shared sessions on other Macs.',
}
const PEER_UNREACHABLE = "Can't reach this Mac. Check that it's awake and on your tailnet."

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
}

export function Sidebar({
  sessions,
  activeTab,
  onOpenLocal,
  peerView,
  onOpenPeer,
  tailscaleHealth,
}: SidebarProps): React.ReactElement {
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
      <div className="sidebar__list">
        <section className="sidebar__group" aria-labelledby="sidebar-group-local">
          <h2 id="sidebar-group-local" className="sidebar__group-header">This Mac</h2>
          <ul className="sidebar__rows">
            {sessions.length === 0 && <li className="sidebar__muted">No sessions</li>}
            {sessions.map((s) => {
              const name = s.name || 'shell'
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    className="sidebar__row"
                    aria-current={s.id === activeLocalId ? 'page' : undefined}
                    onClick={() => onOpenLocal(s.id, s.name)}
                  >
                    <CommandLineIcon className="sidebar__row-icon" aria-hidden="true" />
                    <span className="sidebar__row-name" title={name}>{name}</span>
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
