/**
 * MentionPopover — @alias autocomplete popover over the live session participants.
 *
 *   - Participants are filtered by `filter` (case-insensitive substring on alias).
 *   - The first matching participant is activeIndex 0.
 *   - When no participant matches, nothing renders.
 *
 * Keyboard navigation (Escape global listener is owned here; Arrow/Enter index tracking
 * is owned by the ChatPanel composer — see plan 154-06):
 *   - Escape fires onClose (global keydown listener, SessionCard pattern).
 *
 * Positioning: bottom-anchored to the composer wrapper. The parent is expected to
 * render this inside a `position:relative` wrapper; the popover uses
 * `position:absolute; bottom:100%` (see .mention-popover in style.css).
 * A `style`/`className` passthrough prop is accepted for parent-controlled positioning.
 *
 * Security (T-154-08):
 *   - Aliases render as React text nodes (auto-escaped, never dangerouslySetInnerHTML) (T-154-08).
 */
import { useEffect } from 'react'
import type { PresenceEntry } from '../../lib/relayClient'

/** Deterministic avatar hue from tailnetID — same algorithm as ChatMessage.tsx */
function tailnetIdToHue(tailnetID: string): number {
  let hash = 0
  for (let i = 0; i < tailnetID.length; i++) {
    hash = (hash * 31 + tailnetID.charCodeAt(i)) >>> 0
  }
  return hash % 360
}

export interface MentionPopoverProps {
  /** Live session participants. */
  participants: PresenceEntry[]
  /** Text typed after "@". Used to filter participants. */
  filter: string
  /**
   * Currently active index into the filtered participants (0 = first match).
   * Managed by the parent (ChatPanel composer, plan 154-06).
   */
  activeIndex: number
  /** Called with "@{alias}" when an item is selected. */
  onSelect: (alias: string) => void
  /** Called when Escape is pressed. */
  onClose: () => void
  /** Optional style passthrough for parent-controlled positioning. */
  style?: React.CSSProperties
  /** Optional className passthrough. */
  className?: string
}

export function MentionPopover({
  participants,
  filter,
  activeIndex,
  onSelect,
  onClose,
  style,
  className,
}: MentionPopoverProps) {
  // Filter participants by prefix/substring on alias (case-insensitive).
  const filtered = filter
    ? participants.filter((p) =>
        p.alias.toLowerCase().includes(filter.toLowerCase()),
      )
    : participants

  // Global Escape listener — closes the popover even when focus is in the textarea.
  // Pattern from SessionCard keyboard-dismissable dropdown (PATTERNS.md lines 303–313).
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  if (filtered.length === 0) return null

  return (
    <div
      role="listbox"
      className={`mention-popover${className ? ` ${className}` : ''}`}
      style={style}
    >
      {filtered.map((p, i) => {
        const isActive = activeIndex === i
        const hue = tailnetIdToHue(p.tailnetID)
        return (
          <div
            key={p.personKey}
            role="option"
            className={`mention-popover__item mention-popover__item--participant${isActive ? ' mention-popover__item--active' : ''}`}
            aria-selected={isActive}
            onClick={() => onSelect(`@${p.alias}`)}
          >
            {/* Avatar: 20×20px circle with deterministic hue; alias initial */}
            <span
              className="mention-popover__avatar"
              style={{ background: `hsl(${hue}, 55%, 45%)` }}
              aria-hidden="true"
            >
              {p.alias[0]?.toUpperCase()}
            </span>
            <span className="mention-popover__alias">{p.alias}</span>
            <span className="mention-popover__tailnet-id">{p.tailnetID}</span>
          </div>
        )
      })}
    </div>
  )
}
