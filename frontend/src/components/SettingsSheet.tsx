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
  RegenerateSigningKey,
  // Phase 150 SET-01 — shell web-share warning enabled master switch
  GetShellWebShareWarningEnabled,
  SetShellWebShareWarningEnabled,
} from '../wailsjs/go/main/App'
import { BrowserOpenURL, ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import {
  ArrowTopRightOnSquareIcon,
  ClipboardDocumentIcon,
  Cog6ToothIcon,
  LockClosedIcon,
  QrCodeIcon,
  SignalIcon,
} from '@heroicons/react/24/outline'
import { RegenerateKeyModal } from './RegenerateKeyModal'
import { PluginsSection, type PluginToggleKind } from './PluginsSection'
import { Switch } from './Switch'
import { TAILSCALE_OFF_REASON, type TailscaleHealth } from '../lib/tailscale'
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

function SettingsSheetContent({ onClose, tailscaleHealth, webServerMode, onWebServerStateChange, onPluginToggleSideEffect, onShellWarnEnabledChange }: SettingsSheetProps): React.ReactElement {
  const [pane, setPane] = useState<Pane>('general')
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

  function tailscaleStatusClass(h: TailscaleHealth | null): string {
    if (!h) return ''
    return h.connected ? 'ok' : 'error'
  }

  function tailscaleStatusText(h: TailscaleHealth | null): string {
    if (!h) return 'Checking…'
    return h.connected ? 'Connected' : 'Not connected'
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

  async function handleToggleServer() {
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
      setServerError(err instanceof Error ? err.message : String(err))
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
                {/* THEM-01: follows the system. Interim static row; plan 02-13 adds the override. */}
                <Row label="Appearance" description="Follows your Mac's Light or Dark setting in System Settings.">
                  <span className="settings-row__value">Automatic</span>
                </Row>
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
              {toggleError && <p className="settings-sheet__error" role="alert">{toggleError}</p>}

              {/* Plugins (Phase 92 PUI-01, Phase 99 PUI-02) */}
              <PluginsSection onPluginToggleSideEffect={onPluginToggleSideEffect} />
            </div>

            <div role="tabpanel" id="settings-pane-sharing" aria-labelledby="settings-tab-sharing" hidden={pane !== 'sharing'}>
              <div className="settings-panel__body">
                <h3 id="settings-session-behavior">Session Behavior</h3>
                <div className="settings-panel__field-group">
                  {shellWarnLoaded && (
                    <label
                      className={`settings-panel__toggle-row${shellWarnEnabled ? ' settings-panel__toggle-row--checked' : ''}`}
                      htmlFor="shellWebShareWarningEnabled"
                      style={shellWarnSaving ? { pointerEvents: 'none', opacity: 0.6 } : undefined}
                    >
                      <span className="settings-panel__toggle-track">
                        <span className="settings-panel__toggle-thumb" />
                      </span>
                      <span className="settings-panel__toggle-label">Warn before web-sharing a shell session.</span>
                    </label>
                  )}
                  <input
                    type="checkbox"
                    id="shellWebShareWarningEnabled"
                    className="settings-panel__toggle-input"
                    checked={shellWarnEnabled}
                    onChange={() => void handleToggleShellWarnEnabled()}
                  />
                  <p className="settings-panel__description">
                    Show a one-time security reminder before web-sharing a shell session. Disabling suppresses the reminder.
                  </p>
                  {shellWarnError && <p className="settings-panel__error">{shellWarnError}</p>}
                </div>

                <h3 id="settings-web-server">Web Server</h3>
                <div className="settings-panel__field-group">
                  <label className="settings-panel__label">Tailscale Status</label>
                  <div className="ts-status">
                    {tailscaleHealth && (
                      <span className={`ts-status__dot ts-status__dot--${tailscaleStatusClass(tailscaleHealth)}`} />
                    )}
                    <span className="ts-status__text">{tailscaleStatusText(tailscaleHealth)}</span>
                  </div>
                  <p className="settings-panel__description">
                    {tailscaleHealth
                      ? (tailscaleHealth.connected
                          ? `Connected via ${tailscaleHealth.domain || tailscaleHealth.ip}`
                          : TAILSCALE_OFF_REASON)
                      : 'Checking…'}
                  </p>
                </div>

                <div className="settings-panel__field-group">
                  <label className="settings-panel__label">Port</label>
                  <input
                    className="settings-panel__path-input settings-panel__port-input"
                    type="number"
                    value={selectedPort}
                    onChange={(e) => setSelectedPort(Number(e.target.value))}
                    disabled={isServerRunning}
                    min={1}
                    max={65535}
                  />
                </div>

                <div className="settings-panel__field-group">
                  <button
                    className={`settings-panel__btn ${isServerRunning ? 'settings-panel__btn--cancel' : 'settings-panel__btn--save'}`}
                    onClick={handleToggleServer}
                    disabled={serverLoading || (!isServerRunning && (!ctDisclosed || tailscaleOff))}
                    title={
                      isServerRunning
                        ? undefined
                        : tailscaleOff
                          ? TAILSCALE_OFF_REASON
                          : !ctDisclosed
                            ? 'Acknowledge the Certificate Transparency disclosure first'
                            : undefined
                    }
                  >
                    {serverLoading
                      ? (isServerRunning ? 'Stopping…' : 'Starting…')
                      : (isServerRunning ? 'Stop Web Server' : 'Start Web Server')}
                  </button>
                  {serverError && <p className="settings-panel__error">{serverError}</p>}
                  {isServerRunning && serverURL && (
                    <>
                      <div className="settings-web-server__url-row">
                        <span className="settings-web-server__url-text">{serverURL}</span>
                        <button
                          className="settings-web-server__action-btn"
                          onClick={() => BrowserOpenURL(serverURL)}
                          aria-label="Open dashboard in browser"
                        >
                          <ArrowTopRightOnSquareIcon style={{ width: 14, height: 14 }} />
                          Open
                        </button>
                        <button
                          className={`settings-web-server__action-btn${urlCopied ? ' settings-web-server__action-btn--copy-done' : ''}`}
                          onClick={handleCopyURL}
                          aria-label="Copy dashboard URL to clipboard"
                        >
                          <ClipboardDocumentIcon style={{ width: 14, height: 14 }} />
                          {urlCopied ? 'Copied!' : 'Copy'}
                        </button>
                        <button
                          className={`settings-web-server__action-btn${showDashQR ? ' settings-web-server__action-btn--active' : ''}`}
                          onClick={handleToggleDashQR}
                          aria-label={showDashQR ? 'Hide QR code' : 'Show QR code'}
                        >
                          <QrCodeIcon style={{ width: 14, height: 14 }} />
                          {showDashQR ? 'Hide QR' : 'QR'}
                        </button>
                      </div>
                      {qrError && <p className="settings-panel__error">{qrError}</p>}
                      {showDashQR && dashQRb64 && (
                        <img
                          src={`data:image/png;base64,${dashQRb64}`}
                          width={200}
                          height={200}
                          alt="QR code for dashboard URL"
                          className="settings-web-server__qr"
                        />
                      )}
                    </>
                  )}
                </div>

                {webServerMode === 'local' && isServerRunning && (
                  <>
                    <div className="settings-web-server__mode-indicator">
                      Web server mode: Local network (self-signed TLS)
                    </div>
                    <div className="settings-web-server__password-label">
                      LAN Access Credentials
                    </div>
                    <div className="settings-web-server__credential-hint">
                      Username: <span className="settings-web-server__credential-value">leave blank or enter anything</span>
                    </div>
                    <div className="settings-web-server__password-label">
                      Password
                    </div>
                    <div
                      className="settings-web-server__password-field"
                      onClick={() => void handleCopyPassword()}
                      title="Click to copy password"
                    >
                      <span>{localPassword || 'Loading…'}</span>
                      <span className={`settings-web-server__copy-hint${copied ? ' settings-web-server__copy-hint--copied' : ''}`}>
                        {copied ? 'Copied!' : '(click to copy)'}
                      </span>
                    </div>
                  </>
                )}
              </div>
            </div>

            <div role="tabpanel" id="settings-pane-security" aria-labelledby="settings-tab-security" hidden={pane !== 'security'}>
              <div className="settings-panel__body">
                <div className={`ct-disclosure ${ctDisclosed ? 'ct-disclosure--acknowledged' : ''}`}>
                  <label className="settings-panel__label">Certificate Transparency</label>
                  {ctDisclosed ? (
                    <p className="ct-disclosure__text">
                      <span style={{ color: '#9ece6a' }}>&#10003;</span> Certificate Transparency acknowledged
                    </p>
                  ) : (
                    <>
                      <p className="ct-disclosure__text">
                        When you start the web server, Tailscale will provision a Let&apos;s Encrypt TLS certificate
                        for your device&apos;s hostname (e.g., <code className="settings-panel__code">hostname.ts.net</code>).
                        This hostname will be permanently visible in public Certificate Transparency logs.
                        This is normal and expected for any Let&apos;s Encrypt certificate.
                      </p>
                      <button
                        className="ct-disclosure__btn settings-panel__btn settings-panel__btn--save"
                        onClick={handleAcknowledgeCT}
                      >
                        I Understand
                      </button>
                      {ctError && <p className="settings-panel__error">{ctError}</p>}
                    </>
                  )}
                </div>

                <h3 id="settings-security">Security</h3>
                <p className="settings-panel__description">
                  Rotating the signing key immediately invalidates all shared links across all sessions. Use this if you suspect a link has been leaked.
                </p>
                <div className="settings-panel__field-group">
                  <button
                    className="settings-panel__btn settings-panel__btn--destructive"
                    onClick={() => setShowRegenModal(true)}
                  >
                    Regenerate Signing Key
                  </button>
                  {regenError && <p className="settings-panel__error">{regenError}</p>}
                </div>
              </div>
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
        title="Disable shell web-share warning?"
        body="AgentHub won't show the one-time security reminder before you web-share a shell session. You can re-enable it anytime in Settings → Session Behavior."
        confirmLabel="Disable warning"
        actingLabel="Disabling…"
        cancelLabel="Keep warning"
      />
      <RegenerateKeyModal
        isOpen={showRegenModal}
        onConfirm={handleRegenerateSigningKey}
        onCancel={() => setShowRegenModal(false)}
      />
    </>
  )
}
