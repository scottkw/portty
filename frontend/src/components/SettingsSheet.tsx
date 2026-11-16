import React, { useState, useEffect, useRef } from 'react'
import {
  StartWebServer,
  StopWebServer,
  GetWebServerURL,
  IsWebServerRunning,
  HasCTDisclosure,
  AcknowledgeCTDisclosure,
  GetLocalNetworkPassword,
  GetWebServerQRCode,
  GetStartMinimized,
  SetStartMinimized,
  GetAppearance,
  SetAppearance,
  RegenerateSigningKey,
  // Phase 150 SET-01 — shell web-share warning enabled master switch
  GetShellWebShareWarningEnabled,
  SetShellWebShareWarningEnabled,
} from '../wailsjs/go/main/App'
import { BrowserOpenURL, ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import { Cog6ToothIcon, LockClosedIcon, SignalIcon } from '@heroicons/react/24/outline'
import { RegenerateKeyModal } from './RegenerateKeyModal'
import { PluginsSection, type PluginToggleKind } from './PluginsSection'
import { Switch } from './Switch'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../lib/tailscale'
import { shortHost } from '../lib/sidebarPeers'
import { useDialogFocus } from '../lib/useDialogFocus'

interface SettingsSheetProps {
  isOpen: boolean
  onClose: () => void
  tailscaleHealth: TailscaleHealth | null
  webServerMode?: 'tailscale' | 'local' | null
  onWebServerStateChange: () => Promise<void>
  // Phase 99 PUI-02: forwarded to PluginsSection for post-save banner triggers.
  onPluginToggleSideEffect?: (kinds: PluginToggleKind[]) => void
  // Phase 150 SET-01: notifies App.tsx after warning-enabled toggle changes (D-03 re-arm sync).
  onShellWarnEnabledChange?: (enabled: boolean) => void
  // Portty 02-13 THEM-01: notifies App.tsx after the Appearance override is saved.
  onAppearanceChange?: (appearance: Appearance) => void
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

function SettingsSheetContent({ onClose, tailscaleHealth, webServerMode, onWebServerStateChange, onPluginToggleSideEffect, onShellWarnEnabledChange, onAppearanceChange }: SettingsSheetProps): React.ReactElement {
  const [pane, setPane] = useState<Pane>('general')
  const [appearance, setAppearanceState] = useState<Appearance>('system')
  const [appearanceLoaded, setAppearanceLoaded] = useState(false)
  const [appearanceError, setAppearanceError] = useState<string | null>(null)
  const appearanceRefs = useRef<Partial<Record<Appearance, HTMLButtonElement | null>>>({})
  // One SetAppearance at a time: binding calls run on separate goroutines, so
  // overlapping saves could persist and apply out of order (review WR-04).
  const appearanceSavingRef = useRef(false)
  const dialogRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef<Partial<Record<Pane, HTMLButtonElement | null>>>({})
  useDialogFocus(dialogRef, { onCancel: onClose, initialFocus: 'primary' })

  // Web serving state
  const [selectedPort, setSelectedPort] = useState<number>(7443)
  const [isServerRunning, setIsServerRunning] = useState(false)
  const [serverURL, setServerURL] = useState<string>('')
  const [serverError, setServerError] = useState<string | null>(null)
  const [serverLoading, setServerLoading] = useState(false)
  const [ctDisclosed, setCTDisclosed] = useState(false)
  const [ctError, setCTError] = useState<string | null>(null)
  // Local network password display
  const [localPassword, setLocalPassword] = useState('')
  const [copied, setCopied] = useState(false)

  // URL action row state (WEB-01/02/03)
  const [urlCopied, setUrlCopied] = useState(false)
  const [showDashQR, setShowDashQR] = useState(false)
  const [dashQRb64, setDashQRb64] = useState<string | null>(null)
  const [qrError, setQrError] = useState<string | null>(null)

  // Start-minimized toggle state (TRAY-01)
  const [startMinimized, setStartMinimized] = useState(false)
  const [toggleLoaded, setToggleLoaded] = useState(false)
  const [toggleSaving, setToggleSaving] = useState(false)
  const [toggleError, setToggleError] = useState<string | null>(null)

  // Phase 150 SET-01 — shell web-share warning enabled master switch (D-08 default ON).
  const [shellWarnEnabled, setShellWarnEnabled] = useState(true) // default ON (D-08)
  const [shellWarnLoaded, setShellWarnLoaded] = useState(false)
  const [shellWarnSaving, setShellWarnSaving] = useState(false)
  const [shellWarnError, setShellWarnError] = useState<string | null>(null)
  const [showDisableWarnConfirm, setShowDisableWarnConfirm] = useState(false)

  // Security section state (Phase 87 D-16) — panic button to rotate the
  // capability signing key, invalidating every outstanding shared link.
  const [showRegenModal, setShowRegenModal] = useState(false)
  const [regenError, setRegenError] = useState<string | null>(null)

  // Load web serving state on mount.
  useEffect(() => {
    async function loadWebState() {
      try {
        const [running, ctAck] = await Promise.all([
          IsWebServerRunning(),
          HasCTDisclosure(),
        ])
        setIsServerRunning(running)
        setCTDisclosed(ctAck)

        if (running) {
          const url = await GetWebServerURL()
          setServerURL(url)
        }
      } catch (err) {
        console.error('[SettingsSheet] loadWebState:', err)
      }
    }
    void loadWebState()
  }, [])

  // Fetch LAN password when in local mode and server is running.
  useEffect(() => {
    if (webServerMode === 'local' && isServerRunning) {
      GetLocalNetworkPassword().then(setLocalPassword).catch(() => setLocalPassword(''))
    } else {
      setLocalPassword('')
    }
  }, [webServerMode, isServerRunning])

  // Reset QR state when server stops.
  useEffect(() => {
    if (!isServerRunning) {
      setShowDashQR(false)
      setDashQRb64(null)
      setQrError(null)
    }
  }, [isServerRunning])

  // Load start-minimized preference on mount (TRAY-01/TRAY-03).
  useEffect(() => {
    GetStartMinimized().then(val => {
      setStartMinimized(val)
      setToggleLoaded(true)
    }).catch(() => setToggleLoaded(true))
  }, [])

  // Load the Appearance override on mount (Portty 02-13 THEM-01).
  useEffect(() => {
    GetAppearance().then(val => {
      if (isAppearance(val)) setAppearanceState(val)
      setAppearanceLoaded(true)
    }).catch(() => setAppearanceLoaded(true))
  }, [])

  // Load shell web-share warning enabled preference on mount (Phase 150 SET-01).
  useEffect(() => {
    GetShellWebShareWarningEnabled().then(val => {
      setShellWarnEnabled(val)
      setShellWarnLoaded(true)
    }).catch(() => setShellWarnLoaded(true))
  }, [])

  function handleTabKeyDown(e: React.KeyboardEvent): void {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const i = PANES.findIndex((p) => p.id === pane)
    const next = PANES[(i + (e.key === 'ArrowRight' ? 1 : PANES.length - 1)) % PANES.length].id
    setPane(next)
    tabRefs.current[next]?.focus()
  }

  async function handleCopyPassword() {
    if (!localPassword) return
    try {
      await ClipboardSetText(localPassword)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // ClipboardSetText failed — no user-visible action needed
    }
  }

  async function handleCopyURL() {
    if (!serverURL) return
    await ClipboardSetText(serverURL)
    setUrlCopied(true)
    setTimeout(() => setUrlCopied(false), 1500)
  }

  async function handleToggleDashQR() {
    if (showDashQR) {
      setShowDashQR(false)
      return
    }
    setQrError(null)
    if (!dashQRb64) {
      try {
        const b64 = await GetWebServerQRCode()
        setDashQRb64(b64)
      } catch {
        setQrError('QR unavailable — tap to retry')
        return
      }
    }
    setShowDashQR(true)
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

  // Phase 150 SET-01 — D-07: turning OFF requires confirmation; turning ON is instant.
  async function handleToggleShellWarnEnabled() {
    const next = !shellWarnEnabled
    if (!next) {
      // Turning OFF — show confirm dialog before persisting (D-07).
      setShowDisableWarnConfirm(true)
      return
    }
    // Turning ON — instant, no confirmation.
    setShellWarnSaving(true)
    setShellWarnError(null)
    try {
      await SetShellWebShareWarningEnabled(true)
      setShellWarnEnabled(true)
      // D-03 re-arm: notify App.tsx to re-sync shellWebShareWarned state.
      onShellWarnEnabledChange?.(true)
    } catch (err) {
      setShellWarnError('Could not save preference — ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setShellWarnSaving(false)
    }
  }

  async function handleConfirmDisableShellWarn() {
    setShowDisableWarnConfirm(false)
    setShellWarnSaving(true)
    setShellWarnError(null)
    try {
      await SetShellWebShareWarningEnabled(false)
      setShellWarnEnabled(false)
      onShellWarnEnabledChange?.(false)
    } catch (err) {
      setShellWarnError('Could not save preference — ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setShellWarnSaving(false)
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

  // NET-01 / Amendment 3: Start is gated like Share; Stop stays available.
  const tailscaleOff = !!tailscaleHealth && !tailscaleHealth.connected
  // T-02-34: aria-disabled (not `disabled`) keeps Start focusable so its reason is announced;
  // the click returns early, and the daemon keeps its own CT/mode checks.
  const startBlocked = !isServerRunning && (tailscaleOff || !ctDisclosed)
  // A blocked reason outranks a stale start error, which the blocked click can no longer clear.
  const webServerReason: React.ReactNode = serverError && !startBlocked
    ? <span className="settings-row__error" role="alert">{serverError}</span>
    : isServerRunning
      ? (serverURL ? `Running on ${serverURL}` : 'Running')
      : tailscaleOff
        ? TAILSCALE_OFF_REASON
        : !ctDisclosed
          ? 'Acknowledge Certificate Transparency in Security first.'
          : 'Stopped'

  async function handleToggleServer() {
    if (startBlocked) return
    setServerError(null)
    setServerLoading(true)
    try {
      if (isServerRunning) {
        await StopWebServer()
        setIsServerRunning(false)
        setServerURL('')
      } else {
        await StartWebServer(selectedPort)
        const url = await GetWebServerURL()
        setServerURL(url)
        setIsServerRunning(true)
      }
      await onWebServerStateChange()
    } catch (err) {
      const detail = (err instanceof Error ? err.message : String(err)).replace(/\.$/, '')
      setServerError(isServerRunning
        ? `Couldn't stop the web server: ${detail}.`
        : `Couldn't start the web server: ${detail}. Check that port ${selectedPort} isn't in use, then try again.`)
    } finally {
      setServerLoading(false)
    }
  }

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
                {shellWarnLoaded && (
                  <Row
                    label="Ask before sharing a session"
                    htmlFor="shellWebShareWarningEnabled"
                    description="Shows a one-time security reminder the first time you share a session."
                  >
                    <Switch
                      id="shellWebShareWarningEnabled"
                      checked={shellWarnEnabled}
                      saving={shellWarnSaving}
                      onChange={() => void handleToggleShellWarnEnabled()}
                    />
                  </Row>
                )}
              </div>
              {shellWarnError && <p className="settings-sheet__error" role="alert">{shellWarnError}</p>}

              <div className="settings-group">
                {/* UI-REVIEW 01 #2/#3: the reason is on screen, never tooltip-only. */}
                <Row label="Web server" descriptionId="web-server-reason" description={webServerReason}>
                  <button
                    type="button"
                    className="button button--secondary"
                    onClick={() => void handleToggleServer()}
                    disabled={serverLoading}
                    aria-disabled={startBlocked || undefined}
                    aria-describedby={startBlocked ? 'web-server-reason' : undefined}
                  >
                    {serverLoading
                      ? (isServerRunning ? 'Stopping…' : 'Starting…')
                      : (isServerRunning ? 'Stop Server' : 'Start Server')}
                  </button>
                </Row>
                <Row label="Port" htmlFor="web-server-port">
                  <input
                    id="web-server-port"
                    className="settings-input settings-input--number"
                    type="number"
                    value={selectedPort}
                    onChange={(e) => setSelectedPort(Number(e.target.value))}
                    disabled={isServerRunning}
                    min={1}
                    max={65535}
                  />
                </Row>
                {isServerRunning && serverURL && (
                  <div className="settings-row">
                    <span className="settings-row__text settings-row__mono">{serverURL}</span>
                    <div className="settings-row__control">
                      <button
                        type="button"
                        className="button button--secondary"
                        onClick={() => BrowserOpenURL(serverURL)}
                        aria-label="Open dashboard in browser"
                      >
                        Open
                      </button>
                      <button
                        type="button"
                        className="button button--secondary"
                        onClick={() => void handleCopyURL()}
                        aria-label="Copy dashboard URL to clipboard"
                      >
                        {urlCopied ? 'Copied!' : 'Copy'}
                      </button>
                      <button
                        type="button"
                        className="button button--secondary"
                        onClick={() => void handleToggleDashQR()}
                        aria-label={showDashQR ? 'Hide QR code' : 'Show QR code'}
                      >
                        {showDashQR ? 'Hide QR' : 'QR'}
                      </button>
                    </div>
                  </div>
                )}
                {isServerRunning && (qrError || (showDashQR && dashQRb64)) && (
                  <div className="settings-row settings-row--qr">
                    {qrError && <p className="settings-row__description settings-row__error" role="alert">{qrError}</p>}
                    {showDashQR && dashQRb64 && (
                      <img
                        src={`data:image/png;base64,${dashQRb64}`}
                        width={200}
                        height={200}
                        alt="QR code for dashboard URL"
                        className="settings-row__qr"
                      />
                    )}
                  </div>
                )}
                {webServerMode === 'local' && isServerRunning && (
                  <>
                    <Row label="Mode" description="Local network (self-signed TLS)" />
                    <Row label="Username" description="Leave blank or enter anything" />
                    <Row label="Password" description={<span className="settings-row__mono">{localPassword || 'Loading…'}</span>}>
                      <button
                        type="button"
                        className="button button--secondary"
                        onClick={() => void handleCopyPassword()}
                        aria-label="Copy LAN password"
                      >
                        {copied ? 'Copied!' : 'Copy'}
                      </button>
                    </Row>
                  </>
                )}
              </div>
              <p className="settings-sheet__footnote">{"Share a session with the Share button in the main window's toolbar."}</p>
            </div>

            <div role="tabpanel" id="settings-pane-security" aria-labelledby="settings-tab-security" hidden={pane !== 'security'}>
              <div className="settings-group">
                <Row
                  label="Certificate Transparency"
                  description={ctDisclosed
                    ? "Your tailnet certificate's name is published in public CT logs."
                    : "Starting the web server gets a Let's Encrypt certificate for your Mac's tailnet name (for example hostname.ts.net). That name becomes permanently visible in public Certificate Transparency logs."}
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
        isOpen={showDisableWarnConfirm}
        onConfirm={handleConfirmDisableShellWarn}
        onCancel={() => setShowDisableWarnConfirm(false)}
        titleId="disable-shell-warn-title"
        title="Stop asking before sharing?"
        body="You won't see the one-time security reminder before sharing a session. You can turn it back on in Settings ▸ Sharing."
        confirmLabel="Stop Asking"
        actingLabel="Saving…"
        cancelLabel="Keep Asking"
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
