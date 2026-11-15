import React from 'react'
import { CommandLineIcon } from '@heroicons/react/24/outline'

/** UI-SPEC "Empty state": shown in the terminal area while no tab is open. */
export function EmptyState({ onNewSession }: { onNewSession: () => void }): React.ReactElement {
  return (
    <div className="empty-state">
      <div className="empty-state__content">
        <CommandLineIcon className="empty-state__icon" aria-hidden="true" />
        <h2 className="empty-state__heading">No Open Sessions</h2>
        <p className="empty-state__body">Start a new shell, or open a session from the sidebar.</p>
        <div className="empty-state__actions">
          <button type="button" className="empty-state__button" onClick={onNewSession}>New Session</button>
          <span className="empty-state__hint">⌘T</span>
        </div>
      </div>
    </div>
  )
}
