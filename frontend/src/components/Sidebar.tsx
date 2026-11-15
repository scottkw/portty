import React from 'react'
import { CommandLineIcon, SignalIcon } from '@heroicons/react/24/outline'
import type { Tab } from './TabBar'
import type { SessionInfo } from '../wailsjs/go/main/App'

// Portty 02-04 (NAV-01): macOS source-list sidebar. Desktop only: App does not
// render it in web mode (Phase 159 WEBCHAT-03 guest scope).
interface SidebarProps {
  /** Every daemon session, in the order the daemon returns them. */
  sessions: SessionInfo[]
  activeTab: Tab | null
  onOpenLocal: (sessionId: string, name: string) => void
}

export function Sidebar({ sessions, activeTab, onOpenLocal }: SidebarProps): React.ReactElement {
  // Local terminal tabs carry no type (or 'terminal'); other tab kinds never select a local row.
  const activeLocalId =
    activeTab && (!activeTab.type || activeTab.type === 'terminal') ? activeTab.sessionId : null

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
      </div>
    </nav>
  )
}
