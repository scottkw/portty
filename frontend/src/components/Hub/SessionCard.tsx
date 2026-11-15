import React, { useState, useEffect, useRef, useCallback } from 'react'
import type { SessionInfo } from '../../wailsjs/go/main/App'
import type { AdaptedRemoteSessionInfo } from '../../lib/remoteAdapter'
import type { ITheme } from '@xterm/xterm'
import { daemon } from '../../wailsjs/go/models'
import {
  ComputerDesktopIcon,
  GlobeAltIcon,
  EyeIcon,
  Bars3Icon,
  EllipsisHorizontalIcon,
  LockClosedIcon,         // D-13: shape signal for disabled Share button on remote peer cards
  LockOpenIcon,           // Phase 171 / FNL-09: FULL ACCESS badge shape carrier (colorblind-safe, distinct from GlobeAltIcon)
  LinkIcon,               // CARD-03: "Connected" state shape signal (colorblind-safe)
  WindowIcon,             // FIX-03 RC-C: "Open in tab" in-app glyph (D-17 opens an in-app tab, not a browser)
} from '@heroicons/react/24/outline'
import { InlineSessionName } from '../InlineSessionName'
import { memberKey, type HubGroupDef } from '../../lib/hubGroups'
import { MiniPreview } from './MiniPreview'
import { ChatBadge } from './ChatBadge'

// ---- Helpers ----

/**
 * Format seconds into "Xh Ym" uptime string (for running sessions).
 * UI-SPEC Copywriting Contract: "2h 14m"
 */
function formatUptime(createdAt: string): string {
  const startMs = new Date(createdAt).getTime()
  const elapsedSec = Math.max(0, Math.floor((Date.now() - startMs) / 1000))
  return formatHM(elapsedSec)
}

/**
 * Format seconds into "Ran Xh Ym" string (for stopped sessions).
 * UI-SPEC Copywriting Contract: "Ran 2h 14m"
 */
function formatDuration(seconds: number): string {
  return `Ran ${formatHM(seconds)}`
}

function formatHM(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m`
}

// ---- KillConfirmItem ----

/**
 * Two-step destructive confirm — label flips on first click, second click confirms.
 * No modal — inline within the overflow menu (UI-SPEC Claude's Discretion choice).
 * CARD-04 / UI-SPEC §Kill: inline two-step, no dialog.
 * COLORBLIND-SAFE: "Kill session" text is the primary signal; color is reinforcement only.
 */
function KillConfirmItem({ onKill }: { onKill: () => void }) {
  const [confirming, setConfirming] = useState(false)
  return (
    <button
      type="button"
      className="hub-card__menu-item hub-card__menu-item--destructive"
      role="menuitem"
      onClick={(e) => {
        e.stopPropagation()
        if (!confirming) { setConfirming(true); return }
        onKill()
      }}
    >
      {confirming ? (
        <span>
          <span>Confirm kill</span>
          <span className="hub-card__menu-item-sub">This will stop the session</span>
        </span>
      ) : 'Kill session'}
    </button>
  )
}

// ---- Props ----

export interface SessionCardProps {
  session: SessionInfo
  onRename?: (id: string, name: string) => void
  /**
   * Opens (or focuses) this session's terminal tab. Phase 131 UAT follow-up —
   * re-attach to a running session whose terminal tab is not open in this window.
   * Surfaced as an explicit "Open" button (not whole-card click — card-click is
   * reserved for the Phase 134 modal gesture). Only shown for live sessions.
   */
  onOpenSession?: (sessionId: string, name: string) => void
  /** CARD-07/CARD-05: styled tail lines from usePreviewPoller; undefined = loading. Phase 139: StyledSpan[][] */
  previewLines?: daemon.StyledSpan[][]
  /** Phase 139 / CARD-05: active xterm ITheme for color resolution in MiniPreview */
  previewTheme?: ITheme
  /** GROUP-02: group definitions for the "Move to group" overflow menu */
  groupDefs?: HubGroupDef[]
  /** GROUP-02: fires when user assigns this card to a group via menu */
  onAssignGroup?: (memberKey: string, groupId: string) => void
  /**
   * Phase 134 — fires when card body is clicked (not Open/menu/drag-handle/rename input).
   * Receives the session and the card's bounding rect (used for grow animation origin).
   */
  onCardClick?: (session: SessionInfo, rect: DOMRect) => void
  /**
   * Phase 137 / D-12: fires when the Share button is clicked on a local card.
   * Disabled (no-op) on remote peer cards — only the session owner can share.
   * D-13: colorblind-safe disabled state uses LockClosedIcon + tooltip, not color alone.
   */
  onShare?: (session: SessionInfo) => void
  /** NET-01: when set, a local card's Share button is disabled with this reason. */
  shareDisabledReason?: string
  /**
   * Phase 138 / CARD-02: explicit provenance flag — true when session came from
   * remoteSessions prop (not local sessions). Replaces hostname-based isLocal heuristic.
   * Derived in HubPanel from remoteIdSet.has(session.id).
   */
  isRemote?: boolean
  /**
   * Phase 138 / CARD-03: true when remoteCapsCached.has(session.id) — user has
   * exchanged a join code. Never exposes the token itself (T-122-03-01).
   */
  isConnected?: boolean
  /** Phase 138 / CARD-04: Kill session — wired to handleCloseTab in App. */
  onKill?: (sessionId: string) => void
  /** Phase 138 / CARD-04: Open remote session in system browser (Phase 146: receives session object for cap exchange). */
  onOpenInBrowser?: (session: AdaptedRemoteSessionInfo) => void
  /** NOTIF-01 / D-10: unread chat message count for this session.
   *  Rendered as a ChatBadge on the card header. 0 = no badge. */
  unreadCount?: number
  /** D-10: true when any unread message contains an @mention for this user. */
  hasChatMention?: boolean
}

// ---- Component ----

/**
 * SessionCard — presentational card for the Hub session grid.
 *
 * Layout (Phase 172 — consolidated chip row, Sketch 001 Variant B winner):
 *   HEADER: InlineSessionName + ChatBadge
 *   CHIP ROW: agent chip · origin chip · exposure cluster (INTERNET / FULL
 *     ACCESS filled badges forced onto their own right-aligned line, D-01/D-04/D-05)
 *   META LINE: uptime · viewer count · Connected/Available, muted (D-06)
 *   ACTIONS: Open (when live, Phase 131) / Share
 *   PREVIEW: MiniPreview (CARD-07 plain-text snapshot; NO xterm)
 *
 * No session status, dimming or attention styling (CONTEXT Amendment 2).
 */
export function SessionCard({
  session,
  onRename,
  onOpenSession,
  previewLines,
  previewTheme,
  groupDefs,
  onAssignGroup,
  onCardClick,
  onShare,
  shareDisabledReason,
  isRemote,
  isConnected,
  onKill,
  onOpenInBrowser,
  unreadCount,
  hasChatMention,
}: SessionCardProps): React.ReactElement {
  const {
    id,
    name,
    hostname,
    viewerCount,
    duration,
    createdAt,
  } = session

  // Origin marker: provenance-based when isRemote prop is supplied (CARD-02 fix),
  // falls back to hostname-based for callers that do not yet supply the prop.
  // isRemote is derived in HubPanel from remoteIdSet.has(session.id); local sessions
  // carry the machine's os.Hostname(), so hostname-check alone misclassifies them
  // as remote (GAP-134-A / RESEARCH Pattern 1 / anti-pattern).
  const isLocal = isRemote !== undefined ? !isRemote : (!hostname || hostname === '')
  const originText = isLocal ? 'Local' : hostname

  // Time display
  // IN-03: remote sessions use createdAt = new Date().toISOString() (set at poll time),
  // so formatUptime would show "0m" for ~29s then reset on the next 30s remote poll.
  // Omit uptime for remote sessions entirely — there is no reliable createdAt from the wire.
  const timeText =
    !isLocal
      ? '' // remote session — no reliable createdAt; omit rather than show misleading "0m"
      : session.state === 'stopped' && duration !== undefined && duration !== null
      ? formatDuration(duration)
      : formatUptime(createdAt)

  // Card aria-label per Accessibility Contract (no session status, CONTEXT Amendment 2)
  // CARD-03: append ", connected" or ", available" for remote cards
  const cardAriaLabel = `${name}, ${originText}${isRemote ? (isConnected ? ', connected' : ', available') : ''}`

  /* GROUP-04: membership key = "${session.name}:::${session.workDir}" — survives session-id churn */
  const memberKeyForSession = memberKey(name, session.workDir)

  // Drag state
  const [isDragging, setIsDragging] = useState(false)

  // Menu state
  const [menuOpen, setMenuOpen] = useState(false)
  const menuBtnRef = useRef<HTMLButtonElement>(null)

  // Determine if session is currently in any named group
  const isInNamedGroup = (groupDefs ?? []).some((g) =>
    g.memberKeys.includes(memberKeyForSession)
  )

  // Close menu on Escape key (global listener)
  useEffect(() => {
    if (!menuOpen) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setMenuOpen(false)
        menuBtnRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [menuOpen])

  // Close menu on click-outside
  const menuRef = useRef<HTMLDivElement>(null)
  const handleClickOutside = useCallback((e: MouseEvent) => {
    if (
      menuRef.current &&
      !menuRef.current.contains(e.target as Node) &&
      !menuBtnRef.current?.contains(e.target as Node)
    ) {
      setMenuOpen(false)
    }
  }, [])

  useEffect(() => {
    if (!menuOpen) return
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [menuOpen, handleClickOutside])

  function handleAssign(groupId: string) {
    onAssignGroup?.(memberKeyForSession, groupId)
    setMenuOpen(false)
  }

  // Phase 172 / D-06: consolidated meta line — uptime · viewers · Connected/Available,
  // each separated by a muted dot. Built as an array so a separator only renders
  // between two items that are both actually present (no dangling leading/trailing dot).
  const metaItems: React.ReactNode[] = []
  if (timeText) {
    metaItems.push(<span className="hub-card__uptime">{timeText}</span>)
  }
  if (viewerCount > 0) {
    metaItems.push(
      <span className="hub-card__viewers">
        <EyeIcon className="hub-card__viewers-icon" aria-hidden="true" />
        <span>
          {viewerCount} {viewerCount === 1 ? 'viewer' : 'viewers'}
        </span>
      </span>
    )
  }
  if (isRemote) {
    // COLORBLIND-SAFE: LinkIcon (connected) + GlobeAltIcon (available) carry the state;
    // color is reinforcement only. Hex source: --hub-accent, --hub-text-muted.
    metaItems.push(
      <span className={`hub-card__conn${isConnected ? ' hub-card__conn--connected' : ''}`}>
        {isConnected ? (
          <><LinkIcon className="hub-card__conn-icon" aria-hidden="true" /><span>Connected</span></>
        ) : (
          <><GlobeAltIcon className="hub-card__conn-icon" aria-hidden="true" /><span>Available</span></>
        )}
      </span>
    )
  }

  return (
    <article
      className={[
        'hub-card',
        isDragging ? 'hub-card--dragging' : '',
      ].filter(Boolean).join(' ')}
      draggable="true"
      onDragStart={(e) => {
        e.dataTransfer.setData('text/plain', memberKeyForSession)
        e.dataTransfer.effectAllowed = 'move'
        setIsDragging(true)
      }}
      onDragEnd={() => setIsDragging(false)}
      onClick={(e) => {
        // Defense-in-depth: verify click did not originate from controlled children
        const target = e.target as HTMLElement
        if (target.closest('.hub-card__open')) return
        if (target.closest('.hub-card__share')) return  // D-12/Pitfall 6: Share button guard
        if (target.closest('.hub-card__menu-btn')) return
        if (target.closest('.hub-card__menu')) return
        if (target.closest('.InlineSessionName input')) return
        if (isDragging) return
        onCardClick?.(session, (e.currentTarget as HTMLElement).getBoundingClientRect())
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onCardClick?.(session, (e.currentTarget as HTMLElement).getBoundingClientRect())
        }
      }}
      aria-label={cardAriaLabel}
      tabIndex={0}
    >
      {/* Drag handle — visible on hover (CSS opacity transition); aria-hidden for screen readers */}
      <span
        className="hub-card__drag-handle"
        aria-label="Drag to reorder"
        aria-hidden="true"
      >
        <Bars3Icon className="w-4 h-4" />
      </span>

      {/* Overflow menu button — visible on hover; keyboard-reachable via focus-within */}
      <button
        ref={menuBtnRef}
        type="button"
        className="hub-card__menu-btn"
        aria-label={`Card options for ${name}`}
        aria-expanded={menuOpen}
        aria-haspopup="menu"
        onClick={(e) => { e.stopPropagation(); setMenuOpen((p) => !p) }}
      >
        <EllipsisHorizontalIcon className="w-4 h-4" />
      </button>

      {/* Group overflow menu */}
      {menuOpen && (
        <div ref={menuRef} className="hub-card__menu" role="menu">
          <div className="hub-card__menu-item hub-card__menu-item--header" role="presentation">
            Move to group
          </div>
          {(groupDefs ?? []).map((g) => (
            <button
              key={g.id}
              type="button"
              className="hub-card__menu-item hub-card__menu-item--group"
              role="menuitem"
              onClick={() => handleAssign(g.id)}
            >
              {g.name}
            </button>
          ))}
          {/* WR-01: hide "Other (default)" when the session is already ungrouped.
              Showing it to an ungrouped session is a no-op and a wasted saveGroups write.
              When the session IS in a named group this item moves it back to "Other",
              making the separate "Remove from group" section redundant — removed. */}
          {isInNamedGroup && (
            <button
              type="button"
              className="hub-card__menu-item hub-card__menu-item--group"
              role="menuitem"
              onClick={() => handleAssign('__other__')}
            >
              Other (default)
            </button>
          )}

          {/* Remote-only actions — CARD-04 */}
          {isRemote && (
            <>
              <hr className="hub-card__menu-divider" />
              {/* Phase 146 FIX-03 (out-of-band redesign): open unconditionally — modal replaces dead-end 401 (D-03).
                  No broadcast code gate; the modal guides the viewer to obtain a code from the owner. */}
              <button
                type="button"
                className="hub-card__menu-item"
                role="menuitem"
                onClick={(e) => { e.stopPropagation(); onOpenInBrowser?.(session as AdaptedRemoteSessionInfo); setMenuOpen(false) }}
              >
                <WindowIcon className="hub-card__conn-icon" aria-hidden="true" />
                Open in tab
              </button>
            </>
          )}

          {/* Kill session — LOCAL live sessions only (CARD-04 / CR-02). onKill →
              handleCloseTab kills locally-owned daemon tabs; a remote id has no local
              tab, so showing Kill on remote cards is a dead two-step destructive action.
              The old Remote page offered no Kill either, so hiding it preserves parity. */}
          {isLocal && session.state !== 'stopped' && (
            <>
              <hr className="hub-card__menu-divider" />
              <KillConfirmItem onKill={() => { onKill?.(id); setMenuOpen(false) }} />
            </>
          )}
        </div>
      )}

      {/* HEADER: session name centered on the top line, flanked by the absolute
          drag-handle (left) and overflow menu (right).
          NOTIF-01 / D-10: ChatBadge appears right of the session name when unreadCount > 0. */}
      <div className="hub-card__header">
        <InlineSessionName
          id={id}
          name={name}
          onRenamed={(newName) => onRename?.(id, newName)}
        />
        <ChatBadge count={unreadCount ?? 0} hasMention={hasChatMention ?? false} />
      </div>

      {/* Phase 172 / D-01/D-04/D-05 — consolidated chip row: origin · exposure.
          Sketch 001 Variant B (winner): rounded-rect quiet chip for origin,
          the exposure cluster (INTERNET / FULL ACCESS) is the only FILLED chips and is
          forced onto its own right-aligned line by .hub-card__exposure. */}
      <div className="hub-card__chiprow">
        {/* Origin chip — fully muted (D-01/Sketch pin, no green-local/blue-remote color
            coding). COLORBLIND-SAFE: the ComputerDesktopIcon/GlobeAltIcon shape + text
            label carry the Local/Remote meaning; this chip carries no color signal. */}
        <span className="hub-card__chip hub-card__chip--origin">
          {isLocal ? (
            <>
              <ComputerDesktopIcon className="hub-card__chip-icon" aria-hidden="true" />
              <span>Local</span>
            </>
          ) : (
            <>
              <GlobeAltIcon className="hub-card__chip-icon" aria-hidden="true" />
              <span>{hostname}</span>
            </>
          )}
        </span>

        {(session.funnelActive || session.funnelWriteActive) && (
          <span className="hub-card__exposure">
            {/* Phase 166 / FUI-03 — internet exposure badge.
                COLORBLIND-SAFE: GlobeAltIcon shape + "INTERNET" text carry state; color is reinforcement only.
                Dark hex #43ddb2 / light hex #0d7a5c — verify at source, NOT by eye (user is colorblind). */}
            {session.funnelActive && (
              <span className="hub-internet-badge">
                <GlobeAltIcon className="hub-internet-badge__icon" aria-hidden="true" />
                <span className="hub-internet-badge__label">INTERNET</span>
              </span>
            )}

            {/* Phase 171 / FNL-09 — FULL ACCESS (public write) badge.
                COLORBLIND-SAFE: LockOpenIcon shape + "FULL ACCESS" text + notched
                clip-path badge geometry carry the state; color is reinforcement
                only. Dark hex #f7768e / light hex #c0394f — verify at source, NOT
                by eye (user is colorblind). Read-then-write order: rendered AFTER
                .hub-internet-badge so both may coexist (read-many, write-one). It
                clears independently of funnelActive (RW teardown keeps the read
                badge, D-10) — gated solely on session.funnelWriteActive. */}
            {session.funnelWriteActive && (
              <span className="hub-fullaccess-badge">
                <LockOpenIcon className="hub-fullaccess-badge__icon" aria-hidden="true" />
                <span className="hub-fullaccess-badge__label">FULL ACCESS</span>
              </span>
            )}
          </span>
        )}
      </div>

      {/* Phase 172 / D-06: muted meta line — uptime · viewers · Connected/Available.
          WR-02: only render the wrapper when it has content, otherwise .hub-card__meta's
          min-height + margin-bottom reserves an empty ~24px strip (regression in vertical rhythm). */}
      {metaItems.length > 0 && (
        <div className="hub-card__meta">
          {metaItems.map((item, i) => (
            <React.Fragment key={i}>
              {i > 0 && <span className="hub-card__meta-dot" aria-hidden="true">·</span>}
              {item}
            </React.Fragment>
          ))}
        </div>
      )}

      {/* ROW 5: actions — Open (re-attach terminal tab; LOCAL live sessions only, WR-01,
          Phase 131 UAT follow-up) and Share, side by side as real bordered buttons.
          D-12: Share opens the per-card Share modal; D-13 disables it on remote peer cards.
          COLORBLIND-SAFE: text labels + LockClosedIcon (shape) carry state, not color.
          Pitfall 6: e.stopPropagation() prevents the card-click modal opening on button click;
          also guarded in the article onClick via the .hub-card__open / .hub-card__share checks. */}
      <div className="hub-card__row5">
        {isLocal && onOpenSession && session.state !== 'stopped' && (
          <button
            type="button"
            className="hub-card__open"
            onClick={(e) => { e.stopPropagation(); onOpenSession(id, name) }}
            aria-label={`Open ${name}`}
          >
            Open
          </button>
        )}
        <button
          type="button"
          className="hub-card__share"
          onClick={(e) => { e.stopPropagation(); onShare?.(session) }}
          disabled={!isLocal || !!shareDisabledReason}
          aria-label={!isLocal ? 'Only the session owner can share' : shareDisabledReason ?? `Share ${name}`}
          title={!isLocal ? 'Only the session owner can share' : shareDisabledReason ?? 'Share session'}
        >
          {(!isLocal || shareDisabledReason) && <LockClosedIcon aria-hidden="true" className="hub-card__share-lock" />}
          Share
        </button>
      </div>

      {/* ROW 6: MiniPreview — CARD-07/CARD-05: styled cell grid snapshot; NO xterm instance; polling interval 3s shared */}
      <MiniPreview lines={previewLines} theme={previewTheme ?? {} as ITheme} />
    </article>
  )
}
