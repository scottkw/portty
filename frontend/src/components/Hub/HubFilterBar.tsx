import React from 'react'
import { PlusIcon } from '@heroicons/react/24/outline'

// ---- Props ----

export interface HubFilterBarProps {
  /** The current search text (controlled by parent). */
  searchText: string
  /** Ref forwarded to the search input — parent uses this for the "/" shortcut. */
  searchRef: React.RefObject<HTMLInputElement | null>
  /** Fired when the search text changes (including Escape → ''). */
  onSearchChange: (text: string) => void
  /** Fired when the user clicks "New session". */
  onNewSession: () => void
}

// ---- Component ----

/**
 * HubFilterBar — a search input and the "New session" button.
 *
 * UI-SPEC Interaction Contract — Filter Bar:
 *   - Typing in search → fires onSearchChange with current value
 *   - Escape in search → fires onSearchChange(''), blurs the input
 *   - "New session" click → fires onNewSession
 *   - searchRef is forwarded to the input so the parent's "/" shortcut can focus it
 */
export function HubFilterBar({
  searchText,
  searchRef,
  onSearchChange,
  onNewSession,
}: HubFilterBarProps): React.ReactElement {
  return (
    <div className="hub__filter-bar">
      {/* Search input — aria-label per Accessibility Contract (UI-SPEC §Accessibility) */}
      <input
        ref={searchRef}
        className="hub-filter__search"
        type="text"
        placeholder="Search sessions…"
        aria-label="Search sessions by name or host"
        value={searchText}
        onChange={(e) => onSearchChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            onSearchChange('')
            e.currentTarget.blur()
          }
        }}
      />

      {/* New session button — POL-03: minimal comp affordance with accent PlusIcon */}
      <button
        className="hub-filter__new-session"
        onClick={onNewSession}
        type="button"
      >
        <PlusIcon className="hub-filter__new-session-icon" aria-hidden="true" />
        New session
      </button>
    </div>
  )
}
