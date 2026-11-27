import React, { useState, useEffect, useRef } from 'react'
import {
  HasCTDisclosure,
  AcknowledgeCTDisclosure,
  GetStartMinimized,
  SetStartMinimized,
  GetAppearance,
  SetAppearance,
  GetTerminalFont,
  SetTerminalFont,
  RegenerateSigningKey,
  GetDesktopStatus,
  SetDesktopAlwaysAvailable,
  GetDesktopLink,
  GetDesktopPermissions,
  GetDaemonRepair,
  OpenLoginItems,
  type DesktopStatusResult,
} from '../wailsjs/go/main/App'
import { ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import { Cog6ToothIcon, LockClosedIcon, SignalIcon } from '@heroicons/react/24/outline'
import { RegenerateKeyModal } from './RegenerateKeyModal'
import { DesktopSetupSheet } from './DesktopSetupSheet'
import { PluginsSection, type PluginToggleKind } from './PluginsSection'
import { Switch } from './Switch'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../lib/tailscale'
import { shortHost } from '../lib/sidebarPeers'
import { useDialogFocus } from '../lib/useDialogFocus'
import type { TerminalFont } from '../lib/terminalFont'

interface SettingsSheetProps {
  isOpen: boolean
  onClose: () => void
  tailscaleHealth: TailscaleHealth | null
  // Phase 99 PUI-02: forwarded to PluginsSection for post-save banner triggers.
  onPluginToggleSideEffect?: (kinds: PluginToggleKind[]) => void
  // Phase 150 SET-01: notifies App.tsx after warning-enabled toggle changes (D-03 re-arm sync).
  // Portty 02-13 THEM-01: notifies App.tsx after the Appearance override is saved.
  onAppearanceChange?: (appearance: Appearance) => void
  // Quick 261116-ku9 R1: notifies App.tsx after the Terminal Font is saved.
  onTerminalFontChange?: (font: TerminalFont) => void
}

export type Appearance = 'system' | 'light' | 'dark'

const APPEARANCES: ReadonlyArray<{ id: Appearance; label: string }> = [
  { id: 'system', label: 'Automatic' },
  { id: 'light', label: 'Light' },
  { id: 'dark', label: 'Dark' },
]

function isAppearance(v: string): v is Appearance {
  return APPEARANCES.some((a) => a.id === v)
}

type Pane = 'general' | 'sharing' | 'security'

const PANES: ReadonlyArray<{ id: Pane; label: string; Icon: typeof Cog6ToothIcon }> = [
  { id: 'general', label: 'General', Icon: Cog6ToothIcon },
  { id: 'sharing', label: 'Sharing', Icon: SignalIcon },
  { id: 'security', label: 'Security', Icon: LockClosedIcon },
]

/** One grouped-form row: label and optional description on the left, control on the right. */
function Row({ label, htmlFor, description, descriptionId, children }: {
  label: string
  htmlFor?: string
  description?: React.ReactNode
  descriptionId?: string
  children?: React.ReactNode
}): React.ReactElement {
  return (
    <div className="settings-row">
      <div className="settings-row__text">
        {htmlFor
          ? <label className="settings-row__label" htmlFor={htmlFor}>{label}</label>
          : <span className="settings-row__label">{label}</span>}
        {description != null && <p className="settings-row__description" id={descriptionId}>{description}</p>}
      </div>
      {children != null && <div className="settings-row__control">{children}</div>}
    </div>
  )
}

/** How often the Desktop group asks the daemon for its state while the Sharing pane is open. */
export const DESKTOP_POLL_MS = 2000
/** How long Copy Link reads "Copied!". */
export const COPIED_MS = 2000

const USERSPACE_SENTENCE =
  "Tailscale is running in userspace-networking mode, so Portty can't open a desktop connection. Switch Tailscale to its standard mode, then try again."
const DESKTOP_OFF_DESCRIPTION =
  "View and control this Mac's screen from your other devices any time, even when Portty is quit. Only devices signed in to your Tailscale account can open it."
const DESKTOP_FOOTNOTE =
  'While it is plugged in, Portty keeps this Mac from sleeping on its own, though the display may still sleep. Closing the lid or choosing Sleep still puts it to sleep. On battery, including a low battery, it sleeps as usual. Nothing can wake it from here. macOS asks you to approve Screen Recording again about once a month, at the Mac. The first time Chrome opens the link, it may ask to find devices nearby. Choose Allow.'
const COPY_FAILED = "Couldn't copy the link. Try again."

// The Background service row's description by GetDaemonRepair kind (19-UI-SPEC Row 5).
const REPAIR_TEXT: Record<string, string> = {
  login_items: "Portty's background service is turned off in Login Items, so this Mac's desktop is only reachable while Portty is open.",
  move_to_applications: "Portty is running from a disk image, so this Mac's desktop is only reachable while Portty is open. Move Portty to Applications, then open it again.",
  open_app: "Portty's background service isn't installed, so this Mac's desktop is only reachable while Portty is open. Open Portty.app from Applications once to install it.",
  rehome_pending: "Portty is still moving its background service under macOS's control. It keeps trying.",
}
const REPAIR_DEFAULT = "Portty's background service isn't running under macOS's control. Quit and reopen Portty to try again."

const viewersText = (n: number): string => `${n} viewer${n === 1 ? '' : 's'}`

// A thrown Error stringifies as "Error: …" and the detail often ends in its own
// period; strip both so the fixed sentence reads "…: <detail>. Check…".
const cleanDetail = (d: string): string => d.trim().replace(/^Error:\s*/, '').replace(/\.+$/, '')

type DotKind = 'on' | 'warn' | 'off'
const DOT_CLASS: Record<DotKind, string> = { on: 'ok', warn: 'warn', off: 'error' }

/**
 * The one wording table for the desktop's state (19-UI-SPEC Surface 1 Row 2):
 * `text` and `dot` are the Settings Status row, `tray` is the tray line (the Go
 * mirror in tray_common.go reads the same column). `tray` is null when there is
 * nothing to show. `tailscaleOff` is GUI-derived and wins over every state; a
 * reply with an errorCode means the daemon did not answer.
 */
export function desktopStatusText(
  s: DesktopStatusResult,
  tailscaleOff: boolean,
): { text: string; dot: DotKind | null; tray: string | null } {
  if (tailscaleOff) return { text: 'Waiting for Tailscale', dot: 'warn', tray: 'Desktop: Waiting for Tailscale' }
  if (s.errorCode) return { text: 'Checking…', dot: null, tray: null }
  const row = (text: string, dot: DotKind, tray = text) => ({ text, dot, tray: `Desktop: ${tray}` })
  switch (s.state) {
    case 'needs_permission': return row('Needs permission', 'warn')
    case 'awaiting_approval': return row('Waiting for approval at the Mac', 'warn')
    case 'display_asleep': return row('Display asleep', 'off')
    case 'locked':
      return s.viewers > 0
        ? row(`Locked, streaming to ${viewersText(s.viewers)}`, 'off')
        : row('Locked', 'off')
    case 'idle': return row('Ready. Nobody is watching.', 'on', 'Ready')
    case 'streaming': return row(`Streaming to ${viewersText(s.viewers)}`, 'on')
    case 'capture_paused':
      return row('Paused after repeated errors. Run Test Capture to resume.', 'warn', 'Paused after repeated errors')
    default: return { text: 'Checking…', dot: null, tray: null }
  }
}

/** The line under the Status row for the states that need the user to act at the Mac. */
function desktopStatusNote(s: DesktopStatusResult, waiting: boolean): string | undefined {
  if (waiting || s.errorCode) return undefined
  if (s.state === 'needs_permission') return 'Choose Set Up Permissions below to turn on Screen Recording.'
  if (s.state === 'awaiting_approval') {
    return "macOS is asking for approval on this Mac's screen. Someone at the Mac needs to click Allow, or approve Portty in System Settings."
  }
  if (s.approachingLapse && (s.state === 'idle' || s.state === 'streaming' || s.state === 'locked')) {
    return 'macOS will ask for approval again soon. Someone needs to approve it at the Mac.'
  }
  return undefined
}

// Stands in for the daemon's status before its first answer (and after a turn-on
// whose status read failed): off, or on with a state nobody knows yet.
const NO_STATUS: DesktopStatusResult = {
  alwaysAvailable: false, state: '', viewers: 0, locked: false, approachingLapse: false,
  tailscale: '', launchd: false, reason: '', surfaceKind: '', surfaceName: '', control: '', errorCode: '',
}

/** The 8px dot; no dot at all while the answer is unknown (the words carry the state). */
function StatusDot({ kind }: { kind: DotKind | null }): React.ReactElement | null {
  return kind ? <span className={`ts-status__dot ts-status__dot--${DOT_CLASS[kind]}`} aria-hidden="true" /> : null
}

interface DesktopGroupProps {
  /** The Sharing pane is showing: poll the daemon only then. */
  active: boolean
  tailscaleHealth: TailscaleHealth | null
  ctDisclosed: boolean
  ctLoaded: boolean
  /** A turn-on just recorded the CT acknowledgement in the daemon: read it again. */
  onTurnedOn: () => void
  /** Open the setup sheet; focus goes back to returnTo when it closes. */
  onOpenSetup: (returnTo: HTMLElement | null) => void
  /** Bumped when the setup sheet closes, so the grants are read again. */
  setupClosedCount: number
}

/** The Desktop group in Settings ▸ Sharing (19-UI-SPEC Surface 1). */
function DesktopGroup({ active, tailscaleHealth, ctDisclosed, ctLoaded, onTurnedOn, onOpenSetup, setupClosedCount }: DesktopGroupProps): React.ReactElement {
  const [status, setStatus] = useState<DesktopStatusResult | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [busy, setBusy] = useState<'starting' | 'stopping' | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  // null: not read yet, or the read failed. A failed read is never "missing".
  const [perms, setPerms] = useState<{ sr: boolean; ax: boolean } | null>(null)
  const [repair, setRepair] = useState('')
  const busyRef = useRef(false)
  const askSeq = useRef(0)
  const alive = useRef(true)
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      if (copiedTimer.current) clearTimeout(copiedTimer.current)
    }
  }, [])

  // One read of the daemon's state. The latest answer wins; a failed read keeps
  // the last good state but marks it unanswered, so the row says Checking….
  async function readStatus(): Promise<void> {
    const seq = ++askSeq.current
    const r = await GetDesktopStatus().catch(() => null)
    if (!alive.current || seq !== askSeq.current) return
    setLoaded(true)
    if (!r || r.errorCode) {
      setStatus((prev) => (prev ? { ...prev, errorCode: 'unavailable' } : null))
      return
    }
    setStatus(r)
    if (r.alwaysAvailable && !r.launchd) {
      const kind = await GetDaemonRepair().catch(() => '')
      if (alive.current) setRepair(kind ?? '')
    }
  }

  async function readPerms(): Promise<{ sr: boolean; ax: boolean } | null> {
    const r = await GetDesktopPermissions().catch(() => null)
    if (!alive.current || !r || r.errorCode) return null
    const p = { sr: r.sr, ax: r.ax }
    setPerms(p)
    return p
  }

  useEffect(() => {
    if (!active) return
    const tick = (): void => { if (!busyRef.current) void readStatus() }
    tick()
    const timer = setInterval(tick, DESKTOP_POLL_MS)
    return () => clearInterval(timer)
  }, [active])

  useEffect(() => {
    if (active) void readPerms()
  }, [active, setupClosedCount])

  async function handleToggle(): Promise<void> {
    if (busyRef.current) return
    const turnOn = !(status?.alwaysAvailable ?? false)
    busyRef.current = true
    setError(null)
    setBusy(turnOn ? 'starting' : 'stopping')
    let missing = false
    try {
      const r = await SetDesktopAlwaysAvailable(turnOn).catch((err) => ({
        alwaysAvailable: !turnOn, firstShare: false, errorCode: 'unavailable', errorDetail: String(err),
      }))
      if (r?.errorCode) {
        if (r.errorCode === 'tailscale_userspace') {
          // Not an error line: the switch is blocked with the userspace sentence.
          setStatus((prev) => ({ ...(prev ?? NO_STATUS), tailscale: 'userspace' }))
        } else if (!turnOn) {
          setError(`Couldn't turn off desktop sharing: ${cleanDetail(r.errorDetail)}. Try again.`)
        } else if (r.errorCode === 'https_off') {
          setError("Couldn't turn on desktop sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again.")
        } else {
          setError(`Couldn't turn on desktop sharing: ${cleanDetail(r.errorDetail)}. Check that Tailscale is connected, then try again.`)
        }
      } else if (turnOn) {
        setStatus((prev) => ({ ...(prev ?? NO_STATUS), alwaysAvailable: true }))
        onTurnedOn()
        const p = await readPerms()
        missing = !!p && !(p.sr && p.ax)
      }
      await readStatus()
    } finally {
      busyRef.current = false
      if (alive.current) setBusy(null)
    }
    if (missing && alive.current) onOpenSetup(document.getElementById('desktopAlways'))
  }

  async function handleCopy(): Promise<void> {
    setError(null)
    try {
      // The link is read only now, goes straight to the clipboard and is never kept or shown.
      const r = await GetDesktopLink()
      if (!r?.url || (await ClipboardSetText(r.url)) === false) throw new Error('copy failed')
    } catch {
      if (alive.current) setError(COPY_FAILED)
      return
    }
    if (!alive.current) return
    setCopied(true)
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
    copiedTimer.current = setTimeout(() => setCopied(false), COPIED_MS)
  }

  const tailscaleOff = !!tailscaleHealth && !tailscaleHealth.connected
  const userspace = status?.tailscale === 'userspace'
  const reallyOn = status?.alwaysAvailable ?? false
  const on = busy === 'starting' ? true : busy === 'stopping' ? false : reallyOn
  const waiting = tailscaleOff || userspace
  const shown = status ?? NO_STATUS
  const row2 = busy === 'starting'
    ? { text: 'Getting a link…', dot: null }
    : desktopStatusText(shown, waiting)
  const note = busy === 'starting' ? undefined : desktopStatusNote(shown, waiting)
  const copyBlocked = busy === 'starting' || row2.text === 'Waiting for Tailscale'

  // Turning on is refused while Tailscale cannot carry it; turning off never is.
  const blockedReason = !on && busy === null ? (tailscaleOff ? TAILSCALE_OFF_REASON : userspace ? USERSPACE_SENTENCE : null) : null
  const showCT = ctLoaded && !ctDisclosed && !!tailscaleHealth?.domain
  const description = blockedReason
    ?? (reallyOn && busy === null
      ? 'Turning this off stops every saved desktop link.'
      : (
        <>
          {DESKTOP_OFF_DESCRIPTION}
          {showCT && (
            <span className="settings-row__note">
              The first time, this gets an HTTPS certificate for {tailscaleHealth!.domain}. That name is published in public Certificate Transparency logs.
            </span>
          )}
        </>
      ))

  // The permission preflights are cached per process, so the daemon's measured
  // "control needs Accessibility" overrides a Granted read (20-UI-SPEC Surface E).
  const controlNeedsAx = !!status && !status.errorCode && status.control === 'needs_accessibility'
  const permState = controlNeedsAx ? false : perms === null ? null : perms.sr && perms.ax
  const showing = on && busy !== 'starting' && !!status && !status.errorCode && !waiting && status.viewers > 0 && !!status.surfaceName
    ? (status.surfaceKind === 'window' ? `Window: ${status.surfaceName}` : status.surfaceName)
    : null
  const showRepair = reallyOn && !!status && !status.launchd

  return (
    <>
      <div className="settings-section">Desktop</div>
      <div className="settings-group">
        <Row
          label="Always-available desktop"
          htmlFor="desktopAlways"
          description={description}
          descriptionId={blockedReason ? 'desktop-reason' : undefined}
        >
          <Switch
            id="desktopAlways"
            checked={on}
            saving={busy !== null || !loaded}
            blocked={!!blockedReason}
            describedBy={blockedReason ? 'desktop-reason' : undefined}
            onChange={() => void handleToggle()}
          />
        </Row>
        {on && (
          <Row label="Status" description={note}>
            <span className="ts-status">
              <StatusDot kind={row2.dot} />
              <span className="ts-status__text" aria-live="polite">{row2.text}</span>
            </span>
          </Row>
        )}
        {showing && (
          <Row label="Showing">
            <span className="settings-row__description" aria-live="polite">{showing}</span>
          </Row>
        )}
        {on && (
          <Row label="Link" description="Open it in Chrome or Safari on a device signed in to your Tailscale account. It can control this Mac, so treat it like a password.">
            <button
              type="button"
              className="button button--secondary button--copy"
              aria-disabled={copyBlocked}
              onClick={() => { if (!copyBlocked) void handleCopy() }}
            >
              {copied ? 'Copied!' : 'Copy Link'}
            </button>
          </Row>
        )}
        <Row label="Permissions" description={controlNeedsAx ? 'Control needs Accessibility. Choose Set Up Permissions…' : 'Screen Recording and Accessibility.'}>
          <span className="ts-status">
            <StatusDot kind={permState === null ? null : permState ? 'on' : 'warn'} />
            <span className={`ts-status__text${permState ? ' ts-status__text--granted' : ''}`} aria-live="polite">
              {permState === null ? 'Checking…' : permState ? 'Granted' : 'Needs permission'}
            </span>
          </span>
          <button
            type="button"
            className="button button--secondary"
            onClick={(e) => onOpenSetup(e.currentTarget)}
          >
            {permState ? 'Review Permissions…' : 'Set Up Permissions…'}
          </button>
        </Row>
        {showRepair && (
          <Row label="Background service" description={REPAIR_TEXT[repair] ?? REPAIR_DEFAULT}>
            <span className="ts-status">
              <StatusDot kind="warn" />
              <span className="ts-status__text">Needs repair</span>
            </span>
            {repair === 'login_items' && (
              <button type="button" className="button button--secondary" onClick={() => void OpenLoginItems().catch(() => {})}>
                Open Login Items
              </button>
            )}
          </Row>
        )}
      </div>
      <p className="settings-sheet__footnote">{DESKTOP_FOOTNOTE}</p>
      {error && <p className="settings-sheet__error" role="alert">{error}</p>}
    </>
  )
}

/**
 * Settings as one sheet over the main window (UI-SPEC "Settings sheet"):
 * General / Sharing / Security panes in macOS grouped-form style, closed by
 * Done or Esc. Mounts its content only while open, so values load fresh on
 * every open.
 */
export function SettingsSheet(props: SettingsSheetProps): React.ReactElement | null {
  if (!props.isOpen) return null
  return <SettingsSheetContent {...props} />
}

function SettingsSheetContent({ onClose, tailscaleHealth, onPluginToggleSideEffect, onAppearanceChange, onTerminalFontChange }: SettingsSheetProps): React.ReactElement {
  const [pane, setPane] = useState<Pane>('general')
  const [appearance, setAppearanceState] = useState<Appearance>('system')
  const [appearanceLoaded, setAppearanceLoaded] = useState(false)
  const [appearanceError, setAppearanceError] = useState<string | null>(null)
  const appearanceRefs = useRef<Partial<Record<Appearance, HTMLButtonElement | null>>>({})
  // One SetAppearance at a time: binding calls run on separate goroutines, so
  // overlapping saves could persist and apply out of order (review WR-04).
  const appearanceSavingRef = useRef(false)
  // Terminal Font (quick 261116-ku9 R1): inputs commit on blur or Enter, like a macOS text field.
  const [fontFamily, setFontFamily] = useState('')
  const [fontSize, setFontSize] = useState('13')
  const [fontLoaded, setFontLoaded] = useState(false)
  const [fontError, setFontError] = useState<string | null>(null)
  const fontSavedRef = useRef<TerminalFont>({ family: '', size: 13 })
  const fontRequestedRef = useRef<TerminalFont>({ family: '', size: 13 })
  // Saves run in commit order through one promise chain.
  const fontChainRef = useRef<Promise<void>>(Promise.resolve())
  const dialogRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef<Partial<Record<Pane, HTMLButtonElement | null>>>({})
  useDialogFocus(dialogRef, { onCancel: onClose, initialFocus: 'primary' })

  // Certificate Transparency acknowledgement (Security pane)
  const [ctDisclosed, setCTDisclosed] = useState(false)
  const [ctLoaded, setCTLoaded] = useState(false)
  const [ctError, setCTError] = useState<string | null>(null)

  // Set Up Desktop Sharing sheet (19-11), opened from the Desktop group.
  const [setup, setSetup] = useState<{ returnTo: HTMLElement | null } | null>(null)
  const [setupClosedCount, setSetupClosedCount] = useState(0)

  // Start-minimized toggle state (TRAY-01)
  const [startMinimized, setStartMinimized] = useState(false)
  const [toggleLoaded, setToggleLoaded] = useState(false)
  const [toggleSaving, setToggleSaving] = useState(false)
  const [toggleError, setToggleError] = useState<string | null>(null)

  // Security section state (Phase 87 D-16) — panic button to rotate the
  // capability signing key, invalidating every outstanding shared link.
  const [showRegenModal, setShowRegenModal] = useState(false)
  const [regenError, setRegenError] = useState<string | null>(null)

  // Load the CT acknowledgement on mount.
  function readCTDisclosure(): void {
    HasCTDisclosure()
      .then((v) => { setCTDisclosed(v); setCTLoaded(true) })
      .catch((err) => console.error('[SettingsSheet] HasCTDisclosure:', err))
  }
  useEffect(readCTDisclosure, [])

  // Load start-minimized preference on mount (TRAY-01/TRAY-03).
  useEffect(() => {
    GetStartMinimized().then(val => {
      setStartMinimized(val)
      setToggleLoaded(true)
    }).catch(() => setToggleLoaded(true))
  }, [])

  // Load the Terminal Font on mount (quick 261116-ku9 R1).
  useEffect(() => {
    GetTerminalFont().then(f => {
      fontSavedRef.current = f
      fontRequestedRef.current = f
      setFontFamily(f.family)
      setFontSize(String(f.size))
      setFontLoaded(true)
    }).catch(() => setFontLoaded(true))
  }, [])

  // Load the Appearance override on mount (Portty 02-13 THEM-01).
  useEffect(() => {
    GetAppearance().then(val => {
      if (isAppearance(val)) setAppearanceState(val)
      setAppearanceLoaded(true)
    }).catch(() => setAppearanceLoaded(true))
  }, [])

  function handleTabKeyDown(e: React.KeyboardEvent): void {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const i = PANES.findIndex((p) => p.id === pane)
    const next = PANES[(i + (e.key === 'ArrowRight' ? 1 : PANES.length - 1)) % PANES.length].id
    setPane(next)
    tabRefs.current[next]?.focus()
  }

  function tailscaleStatusText(h: TailscaleHealth | null): string {
    if (!h) return 'Checking…'
    return h.connected ? `Connected as ${shortHost(h.domain) || h.ip}` : 'Not connected'
  }

  async function handleAcknowledgeCT() {
    setCTError(null)
    try {
      await AcknowledgeCTDisclosure()
      setCTDisclosed(true)
    } catch (err) {
      setCTError(err instanceof Error ? err.message : String(err))
    }
  }

  async function selectAppearance(next: Appearance) {
    if (next === appearance || appearanceSavingRef.current) return
    appearanceSavingRef.current = true
    const prev = appearance
    setAppearanceState(next)
    setAppearanceError(null)
    try {
      await SetAppearance(next)
      onAppearanceChange?.(next)
    } catch (err) {
      setAppearanceState(prev)
      setAppearanceError('Could not save preference — ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      appearanceSavingRef.current = false
    }
  }

  function commitTerminalFont(): void {
    // The daemon is the validation point; a bad size comes back as an error.
    const next: TerminalFont = { family: fontFamily.trim(), size: Number(fontSize) }
    const req = fontRequestedRef.current
    if (next.family === req.family && next.size === req.size) return
    fontRequestedRef.current = next
    setFontError(null)
    fontChainRef.current = fontChainRef.current.then(async () => {
      try {
        await SetTerminalFont(next.family, next.size)
        fontSavedRef.current = next
        onTerminalFontChange?.(next)
      } catch (err) {
        const saved = fontSavedRef.current
        fontRequestedRef.current = saved
        setFontFamily(saved.family)
        setFontSize(String(saved.size))
        setFontError(`Couldn't save the terminal font: ${err instanceof Error ? err.message : String(err)}.`)
      }
    })
  }

  function handleFontKeyDown(e: React.KeyboardEvent): void {
    if (e.key === 'Enter') commitTerminalFont()
  }

  function handleAppearanceKeyDown(e: React.KeyboardEvent): void {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    if (appearanceSavingRef.current) return // keep focus on the checked segment
    const i = APPEARANCES.findIndex((a) => a.id === appearance)
    const next = APPEARANCES[(i + (e.key === 'ArrowRight' ? 1 : -1) + APPEARANCES.length) % APPEARANCES.length].id
    appearanceRefs.current[next]?.focus()
    void selectAppearance(next)
  }

  async function handleToggleMinimized() {
    const next = !startMinimized
    setToggleSaving(true)
    setToggleError(null)
    try {
      await SetStartMinimized(next)
      setStartMinimized(next)
    } catch (err) {
      setToggleError('Could not save preference — ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setToggleSaving(false)
    }
  }

  async function handleRegenerateSigningKey(): Promise<void> {
    setRegenError(null)
    try {
      await RegenerateSigningKey()
      setShowRegenModal(false)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setRegenError(msg)
      // Re-throw so RegenerateKeyModal's inline error path surfaces the
      // failure in the modal (and keeps the modal open for retry).
      throw err
    }
  }

  const tailscaleOff = !!tailscaleHealth && !tailscaleHealth.connected

  const paneLabel = PANES.find((p) => p.id === pane)!.label

  return (
    <>
      <div className="dialog-scrim">
        <div
          ref={dialogRef}
          className="dialog dialog--settings"
          role="dialog"
          aria-modal="true"
          aria-labelledby="settings-sheet-title"
        >
          <div className="settings-sheet__header">
            <h2 className="settings-sheet__title" id="settings-sheet-title">{paneLabel}</h2>
            <div className="settings-sheet__tabs" role="tablist" aria-label="Settings sections" onKeyDown={handleTabKeyDown}>
              {PANES.map(({ id, label, Icon }) => (
                <button
                  key={id}
                  ref={(el) => { tabRefs.current[id] = el }}
                  type="button"
                  role="tab"
                  id={`settings-tab-${id}`}
                  aria-controls={`settings-pane-${id}`}
                  aria-selected={pane === id}
                  tabIndex={pane === id ? 0 : -1}
                  className="settings-sheet__tab"
                  onClick={() => setPane(id)}
                >
                  <Icon className="settings-sheet__tab-icon" aria-hidden="true" />
                  <span>{label}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="settings-sheet__content">
            <div role="tabpanel" id="settings-pane-general" aria-labelledby="settings-tab-general" hidden={pane !== 'general'}>
              <div className="settings-group">
                {/* THEM-01 (amended 2026-11-15): Automatic follows macOS; Light/Dark override it natively. */}
                {appearanceLoaded && (
                  <Row label="Appearance" description="Automatic follows your Mac's Light or Dark setting in System Settings.">
                    {/* Styled like the Share dialog segmented control (NSSegmentedControl). */}
                    <div className="share-segbar settings-appearance" role="radiogroup" aria-label="Appearance" onKeyDown={handleAppearanceKeyDown}>
                      {APPEARANCES.map(({ id, label }) => (
                        <button
                          key={id}
                          ref={(el) => { appearanceRefs.current[id] = el }}
                          type="button"
                          role="radio"
                          aria-checked={appearance === id}
                          tabIndex={appearance === id ? 0 : -1}
                          className={appearance === id ? 'share-seg is-active' : 'share-seg'}
                          onClick={() => void selectAppearance(id)}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  </Row>
                )}
                {fontLoaded && (
                  <Row
                    label="Terminal Font"
                    htmlFor="terminal-font-family"
                    description="Leave the name empty to use SF Mono. Nerd Font icons keep working."
                  >
                    <input
                      id="terminal-font-family"
                      className="settings-input settings-input--font"
                      type="text"
                      aria-label="Font name"
                      placeholder="SF Mono"
                      maxLength={64}
                      spellCheck={false}
                      autoComplete="off"
                      autoCapitalize="off"
                      value={fontFamily}
                      onChange={(e) => setFontFamily(e.target.value)}
                      onBlur={commitTerminalFont}
                      onKeyDown={handleFontKeyDown}
                    />
                    <input
                      id="terminal-font-size"
                      className="settings-input settings-input--number"
                      type="number"
                      aria-label="Font size"
                      min={6}
                      max={32}
                      step={1}
                      value={fontSize}
                      onChange={(e) => setFontSize(e.target.value)}
                      onBlur={commitTerminalFont}
                      onKeyDown={handleFontKeyDown}
                    />
                  </Row>
                )}
                {toggleLoaded && (
                  <Row
                    label="Start hidden in the menu bar"
                    htmlFor="startMinimized"
                    description="Opens with no window. Click the menu bar icon to show it."
                  >
                    <Switch
                      id="startMinimized"
                      checked={startMinimized}
                      saving={toggleSaving}
                      onChange={() => void handleToggleMinimized()}
                    />
                  </Row>
                )}
              </div>
              {appearanceError && <p className="settings-sheet__error" role="alert">{appearanceError}</p>}
              {fontError && <p className="settings-sheet__error" role="alert">{fontError}</p>}
              {toggleError && <p className="settings-sheet__error" role="alert">{toggleError}</p>}

              {/* Plugins (Phase 92 PUI-01, Phase 99 PUI-02) */}
              <PluginsSection onPluginToggleSideEffect={onPluginToggleSideEffect} />
            </div>

            <div role="tabpanel" id="settings-pane-sharing" aria-labelledby="settings-tab-sharing" hidden={pane !== 'sharing'}>
              <div className="settings-group">
                <Row label="Tailscale" description={tailscaleOff ? TAILSCALE_OFF_REASON : undefined}>
                  <span className="ts-status">
                    {tailscaleHealth && (
                      <span className={`ts-status__dot ts-status__dot--${tailscaleHealth.connected ? 'ok' : 'error'}`} aria-hidden="true" />
                    )}
                    <span className="ts-status__text">{tailscaleStatusText(tailscaleHealth)}</span>
                  </span>
                </Row>
              </div>
              <p className="settings-sheet__footnote">
                Share a session from the toolbar, or right-click it in the sidebar. Macs signed in to your Tailscale account see shared sessions in their sidebar, and each shared session gets a link that opens in a browser on any device signed in to your account. Turning Share off disconnects everyone.
              </p>
              <DesktopGroup
                active={pane === 'sharing'}
                tailscaleHealth={tailscaleHealth}
                ctDisclosed={ctDisclosed}
                ctLoaded={ctLoaded}
                onTurnedOn={readCTDisclosure}
                onOpenSetup={(returnTo) => setSetup({ returnTo })}
                setupClosedCount={setupClosedCount}
              />
            </div>

            <div role="tabpanel" id="settings-pane-security" aria-labelledby="settings-tab-security" hidden={pane !== 'security'}>
              <div className="settings-group">
                <Row
                  label="Certificate Transparency"
                  description={ctDisclosed
                    ? "Your tailnet certificate's name is published in public CT logs."
                    : "Sharing a session gets a Let's Encrypt certificate for your Mac's tailnet name (for example hostname.ts.net). That name becomes permanently visible in public Certificate Transparency logs."}
                >
                  {ctDisclosed
                    ? <span className="settings-row__success">Acknowledged</span>
                    : (
                      <button type="button" className="button button--secondary" onClick={() => void handleAcknowledgeCT()}>
                        I Understand
                      </button>
                    )}
                </Row>
                <Row label="Session keys" description="Regenerating signs out every browser and Mac attached to your sessions, and stops every saved desktop link.">
                  <button type="button" className="button button--secondary" onClick={() => setShowRegenModal(true)}>
                    Regenerate…
                  </button>
                </Row>
              </div>
              {ctError && <p className="settings-sheet__error" role="alert">{ctError}</p>}
              {regenError && <p className="settings-sheet__error" role="alert">{regenError}</p>}
            </div>
          </div>

          <div className="settings-sheet__footer">
            <button type="button" className="button button--primary" data-dialog-primary="" onClick={onClose}>
              Done
            </button>
          </div>
        </div>
      </div>

      {/* Confirms and the setup sheet sit after the sheet in the DOM, so they are the topmost modal for Esc and Tab. */}
      <DesktopSetupSheet
        open={setup !== null}
        onClose={() => { setSetup(null); setSetupClosedCount((c) => c + 1) }}
        returnFocusTo={setup?.returnTo}
      />
      <RegenerateKeyModal
        isOpen={showRegenModal}
        onConfirm={handleRegenerateSigningKey}
        onCancel={() => setShowRegenModal(false)}
        destructive
      />
    </>
  )
}
