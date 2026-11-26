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
  type DesktopStatusResult,
} from '../wailsjs/go/main/App'
import { ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import { Cog6ToothIcon, LockClosedIcon, SignalIcon } from '@heroicons/react/24/outline'
import { RegenerateKeyModal } from './RegenerateKeyModal'
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

const viewersText = (n: number): string => `${n} viewer${n === 1 ? '' : 's'}`

/**
 * The one wording table for the desktop's state (19-UI-SPEC Surface 1 Row 2):
 * `text` and `dot` are the Settings Status row, `tray` is the tray line (the Go
 * mirror in tray_common.go reads the same column). `tray` is null when there is
 * nothing to show. A reply with an errorCode means the daemon did not answer.
 */
export function desktopStatusText(
  s: DesktopStatusResult,
  _tailscaleOff: boolean,
): { text: string; dot: 'on' | 'warn' | 'off' | null; tray: string | null } {
  if (s.errorCode) return { text: 'Checking…', dot: null, tray: null }
  switch (s.state) {
    case 'idle':
      return { text: 'Ready. Nobody is watching.', dot: 'on', tray: 'Desktop: Ready' }
    case 'streaming':
      return {
        text: `Streaming to ${viewersText(s.viewers)}`,
        dot: 'on',
        tray: `Desktop: Streaming to ${viewersText(s.viewers)}`,
      }
    default:
      return { text: 'Checking…', dot: null, tray: null }
  }
}

/** The Desktop group in Settings ▸ Sharing: switch, live status, Copy Link. */
function DesktopGroup({ active }: { active: boolean }): React.ReactElement {
  const [status, setStatus] = useState<DesktopStatusResult | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [busy, setBusy] = useState<'starting' | 'stopping' | null>(null)
  const [copied, setCopied] = useState(false)
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
    if (r && !r.errorCode) setStatus(r)
    else setStatus((prev) => (prev ? { ...prev, errorCode: 'unavailable' } : null))
  }

  useEffect(() => {
    if (!active) return
    const tick = (): void => { if (!busyRef.current) void readStatus() }
    tick()
    const timer = setInterval(tick, DESKTOP_POLL_MS)
    return () => clearInterval(timer)
  }, [active])

  async function handleToggle(): Promise<void> {
    if (busyRef.current) return
    const turnOn = !(status?.alwaysAvailable ?? false)
    busyRef.current = true
    setBusy(turnOn ? 'starting' : 'stopping')
    try {
      await SetDesktopAlwaysAvailable(turnOn)
      await readStatus()
    } finally {
      busyRef.current = false
      if (alive.current) setBusy(null)
    }
  }

  async function handleCopy(): Promise<void> {
    // The link is read only now, goes straight to the clipboard and is never kept or shown.
    const r = await GetDesktopLink()
    if (!r?.url) return
    await ClipboardSetText(r.url)
    setCopied(true)
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
    copiedTimer.current = setTimeout(() => setCopied(false), COPIED_MS)
  }

  const on = busy === 'starting' ? true : busy === 'stopping' ? false : (status?.alwaysAvailable ?? false)
  const shown = status ?? ({ alwaysAvailable: false, state: '', viewers: 0, errorCode: 'unavailable' } as DesktopStatusResult)
  const row2 = busy === 'starting'
    ? { text: 'Getting a link…', dot: null }
    : desktopStatusText(shown, false)
  const copyBlocked = busy === 'starting'

  return (
    <>
      <div className="settings-section">Desktop</div>
      <div className="settings-group">
        <Row
          label="Always-available desktop"
          htmlFor="desktopAlways"
          description={on
            ? 'Turning this off stops every saved desktop link.'
            : "Open this Mac's screen from your other devices any time, even when Portty is quit. Only devices signed in to your Tailscale account can open it."}
        >
          <Switch
            id="desktopAlways"
            checked={on}
            saving={busy !== null || !loaded}
            onChange={() => void handleToggle()}
          />
        </Row>
        {on && (
          <Row label="Status">
            <span className="ts-status">
              {row2.dot && <span className={`ts-status__dot ts-status__dot--${row2.dot === 'on' ? 'ok' : row2.dot === 'warn' ? 'warn' : 'error'}`} aria-hidden="true" />}
              <span className="ts-status__text" aria-live="polite">{row2.text}</span>
            </span>
          </Row>
        )}
        {on && (
          <Row label="Link" description="Open it in Chrome or Safari on a device signed in to your Tailscale account. Treat it like a password.">
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
      </div>
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
  const [ctError, setCTError] = useState<string | null>(null)

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
  useEffect(() => {
    HasCTDisclosure().then(setCTDisclosed).catch((err) => console.error('[SettingsSheet] HasCTDisclosure:', err))
  }, [])

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
              <DesktopGroup active={pane === 'sharing'} />
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
                <Row label="Session keys" description="Regenerating signs out every browser and Mac attached to your sessions.">
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

      {/* Confirms sit after the sheet in the DOM, so they are the topmost modal for Esc and Tab. */}
      <RegenerateKeyModal
        isOpen={showRegenModal}
        onConfirm={handleRegenerateSigningKey}
        onCancel={() => setShowRegenModal(false)}
        destructive
      />
    </>
  )
}
