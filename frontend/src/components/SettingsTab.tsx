import React, { useState, useEffect } from 'react'
import { ALLOWED_THEMES } from '../themes'
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
  // Phase 167 NTF-04 — awaiting-input native notification toggle
  GetNotifyOnWaiting,
  SetNotifyOnWaiting,
  // Phase 168 UX-01 — stay-on-hub-after-create toggle
  GetStayOnHubAfterCreate,
  SetStayOnHubAfterCreate,
  RegenerateSigningKey,
  // Phase 150 SET-01 — shell web-share warning enabled master switch
  GetShellWebShareWarningEnabled,
  SetShellWebShareWarningEnabled,
} from '../wailsjs/go/main/App'
import { BrowserOpenURL, ClipboardSetText, EventsOn } from '../wailsjs/wailsjs/runtime/runtime'
import {
  ArrowTopRightOnSquareIcon,
  ClipboardDocumentIcon,
  QrCodeIcon,
  SunIcon,
  MoonIcon,
} from '@heroicons/react/24/outline'
import { RegenerateKeyModal } from './RegenerateKeyModal'
import { PluginsSection, type PluginToggleKind } from './PluginsSection'
import { SettingsJumpBar } from './SettingsJumpBar'
import { SettingsSearch } from './SettingsSearch'

const THEME_NAMES = ALLOWED_THEMES

interface SettingsTabProps {
  tailscaleHealth: {
    installed: boolean
    connected: boolean
    hasCerts: boolean
    ip: string
    domain: string
    binaryFound: boolean
    daemonUp: boolean
    platformHint: string
    // Phase 169-02 (FIX-05): honest permission-aware macsys detection (169-01) —
    // daemon confirmed alive but this account can't read its status.
    permissionLimited?: boolean
  } | null
  webServerMode?: 'tailscale' | 'local' | null
  onWebServerStateChange: () => Promise<void>
  selectedTheme: string
  onThemeChange: (name: string) => void
  // Phase 141-08: whole-app UI theme (light/dark), distinct from terminal color theme.
  uiTheme: 'dark' | 'light'
  onUiThemeChange: (t: 'dark' | 'light') => void
  // Phase 99 PUI-02: forwarded to PluginsSection for post-save banner triggers.
  onPluginToggleSideEffect?: (kinds: PluginToggleKind[]) => void
  // Phase 150 SET-01: notifies App.tsx after warning-enabled toggle changes (D-03 re-arm sync).
  onShellWarnEnabledChange?: (enabled: boolean) => void
}

/**
 * Inline settings tab: shell and tailscale path overrides, web serving and
 * the other app preferences.
 * Renders as a sidebar tab — no modal shell. Single scrollable page with section headers.
 */
export function SettingsTab({ tailscaleHealth, webServerMode, onWebServerStateChange, selectedTheme, onThemeChange, uiTheme, onUiThemeChange, onPluginToggleSideEffect, onShellWarnEnabledChange }: SettingsTabProps): React.ReactElement {
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

  // Notify-on-waiting toggle state (Phase 167 NTF-04) — default OFF (D-04 locked decision).
  const [notifyOnWaiting, setNotifyOnWaiting] = useState(false)
  const [notifyOnWaitingLoaded, setNotifyOnWaitingLoaded] = useState(false)
  const [notifyOnWaitingSaving, setNotifyOnWaitingSaving] = useState(false)
  const [notifyOnWaitingError, setNotifyOnWaitingError] = useState<string | null>(null)
  // Phase 167-07 (M-41 gap closure) — set true when the backend reports macOS
  // denied notification authorization (notification:permission-denied event,
  // emitted by 167-06's onNotificationAuthResult callback).
  const [notifyPermissionDenied, setNotifyPermissionDenied] = useState(false)

  // Stay-on-hub-after-create toggle state (Phase 168 UX-01) — default OFF (D-09).
  const [stayOnHubAfterCreate, setStayOnHubAfterCreate] = useState(false)
  const [stayOnHubAfterCreateLoaded, setStayOnHubAfterCreateLoaded] = useState(false)
  const [stayOnHubAfterCreateSaving, setStayOnHubAfterCreateSaving] = useState(false)
  const [stayOnHubAfterCreateError, setStayOnHubAfterCreateError] = useState<string | null>(null)

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
        console.error('[SettingsTab] loadWebState:', err)
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

  // Load notify-on-waiting preference on mount (Phase 167 NTF-04).
  useEffect(() => {
    GetNotifyOnWaiting().then(val => {
      setNotifyOnWaiting(val)
      setNotifyOnWaitingLoaded(true)
    }).catch(() => setNotifyOnWaitingLoaded(true))
  }, [])

  // Load stay-on-hub-after-create preference on mount (Phase 168 UX-01).
  useEffect(() => {
    GetStayOnHubAfterCreate().then(val => {
      setStayOnHubAfterCreate(val)
      setStayOnHubAfterCreateLoaded(true)
    }).catch(() => setStayOnHubAfterCreateLoaded(true))
  }, [])

  // Load shell web-share warning enabled preference on mount (Phase 150 SET-01).
  useEffect(() => {
    GetShellWebShareWarningEnabled().then(val => {
      setShellWarnEnabled(val)
      setShellWarnLoaded(true)
    }).catch(() => setShellWarnLoaded(true))
  }, [])

  // Phase 167-07 (M-41 gap closure) — subscribe to the backend's
  // notification:permission-denied event so a denied macOS authorization
  // request surfaces a remediation hint instead of a silent dead end. The
  // paired permission-granted subscription lets the hint self-heal once the
  // user fixes permissions and re-toggles (Phase 167 WR-01) — otherwise the
  // hint stayed stuck for the whole session and its own remediation text
  // ("toggle off and on again") never cleared it.
  useEffect(() => {
    const offDenied = EventsOn('notification:permission-denied', () => setNotifyPermissionDenied(true))
    const offGranted = EventsOn('notification:permission-granted', () => setNotifyPermissionDenied(false))
    return () => { offDenied(); offGranted() }
  }, [])

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
        setQrError('QR unavailable \u2014 tap to retry')
        return
      }
    }
    setShowDashQR(true)
  }

  function tailscaleStatusClass(h: SettingsTabProps['tailscaleHealth']): string {
    if (!h) return ''
    if (h.connected) return 'ok'
    // Phase 169-02 (FIX-05): permission-limited is a distinct caution state \u2014
    // never 'ok' (would falsely imply Connected). Checked before daemonUp/
    // binaryFound so it pre-empts the generic "Not Connected"/"Daemon Stopped"
    // classification once the backend has confirmed the daemon is alive but
    // this account can't read its status.
    if (h.permissionLimited) return 'warn'
    if (h.daemonUp) return 'warn'
    if (h.binaryFound) return 'warn'
    return 'error'
  }

  function tailscaleStatusText(h: SettingsTabProps['tailscaleHealth']): string {
    if (!h) return 'Checking\u2026'
    if (h.connected) return 'Connected'
    // Phase 169-02 (FIX-05): distinct label \u2014 must never contain "Connected".
    if (h.permissionLimited) return 'Permission Limited'
    if (h.daemonUp) return 'Not Connected'
    if (h.binaryFound) return 'Daemon Stopped'
    return 'Not Installed'
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
      setToggleError('Could not save preference \u2014 ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setToggleSaving(false)
    }
  }

  // Phase 167 NTF-04 \u2014 instant toggle (no confirm dialog), mirrors handleToggleMinimized.
  async function handleToggleNotifyOnWaiting() {
    const next = !notifyOnWaiting
    setNotifyOnWaitingSaving(true)
    setNotifyOnWaitingError(null)
    // Clear any stale permission-denied hint before re-requesting authorization
    // (Phase 167 WR-01) — the backend re-runs the auth request on enable and
    // will re-emit denied if it is still denied, or granted to self-heal.
    if (next) setNotifyPermissionDenied(false)
    try {
      await SetNotifyOnWaiting(next)
      setNotifyOnWaiting(next)
    } catch (err) {
      setNotifyOnWaitingError('Could not save preference \u2014 ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setNotifyOnWaitingSaving(false)
    }
  }

  // Phase 168 UX-01 — instant toggle (no confirm dialog), mirrors handleToggleNotifyOnWaiting.
  async function handleToggleStayOnHub() {
    const next = !stayOnHubAfterCreate
    setStayOnHubAfterCreateSaving(true)
    setStayOnHubAfterCreateError(null)
    try {
      await SetStayOnHubAfterCreate(next)
      setStayOnHubAfterCreate(next)
    } catch (err) {
      setStayOnHubAfterCreateError('Could not save preference — ' + (err instanceof Error ? err.message : String(err)))
    } finally {
      setStayOnHubAfterCreateSaving(false)
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

  return (
    <div className="settings-tab">
      <div className="settings-panel__body">
        {/* Phase 104 SETUI-01/02/03 — Hyperlinked index: sticky jump-bar
            + autocomplete search, mounted above the first section. */}
        <SettingsJumpBar />
        <SettingsSearch />

        {/* Behavior section (TRAY-01) */}
        <h3 id="settings-behavior">Behavior</h3>
        <div className="settings-panel__field-group">
          {toggleLoaded && (
            <label
              className={`settings-panel__toggle-row${startMinimized ? ' settings-panel__toggle-row--checked' : ''}`}
              htmlFor="startMinimized"
              style={toggleSaving ? { pointerEvents: 'none', opacity: 0.6 } : undefined}
            >
              <span className="settings-panel__toggle-track">
                <span className="settings-panel__toggle-thumb" />
              </span>
              <span className="settings-panel__toggle-label">Start minimized to system tray</span>
            </label>
          )}
          <input
            type="checkbox"
            id="startMinimized"
            className="settings-panel__toggle-input"
            checked={startMinimized}
            onChange={() => void handleToggleMinimized()}
          />
          <p className="settings-panel__description">
            When enabled, AgentHub launches with the window hidden. Click the tray icon to open it.
          </p>
          {toggleError && <p className="settings-panel__error">{toggleError}</p>}
        </div>

        {/* Phase 167 NTF-04 — notify-on-waiting toggle (Behavior section, per LOCKED user
            correction — NOT Session Behavior; default OFF). */}
        <div className="settings-panel__field-group">
          {notifyOnWaitingLoaded && (
            <label
              className={`settings-panel__toggle-row${notifyOnWaiting ? ' settings-panel__toggle-row--checked' : ''}`}
              htmlFor="notifyOnWaiting"
              style={notifyOnWaitingSaving ? { pointerEvents: 'none', opacity: 0.6 } : undefined}
            >
              <span className="settings-panel__toggle-track">
                <span className="settings-panel__toggle-thumb" />
              </span>
              <span className="settings-panel__toggle-label">Notify me when a session is awaiting input</span>
            </label>
          )}
          <input
            type="checkbox"
            id="notifyOnWaiting"
            className="settings-panel__toggle-input"
            checked={notifyOnWaiting}
            onChange={() => void handleToggleNotifyOnWaiting()}
          />
          <p className="settings-panel__description">
            Shows a native OS notification the moment a session starts waiting for your input, even when AgentHub is hidden in the tray. Off by default.
          </p>
          {notifyOnWaitingError && <p className="settings-panel__error">{notifyOnWaitingError}</p>}
          {notifyPermissionDenied && (
            <p className="settings-panel__error">
              AgentHub is not allowed to send notifications. Enable it in System Settings {'>'} Notifications {'>'} AgentHub, then toggle this setting off and on again.
            </p>
          )}
        </div>

        {/* Session Behavior section */}
        <h3 id="settings-session-behavior">Session Behavior</h3>
        {/* Phase 168 UX-01 — stay-on-hub-after-create toggle (Session Behavior, D-08/D-09) */}
        <div className="settings-panel__field-group">
          {stayOnHubAfterCreateLoaded && (
            <label
              className={`settings-panel__toggle-row${stayOnHubAfterCreate ? ' settings-panel__toggle-row--checked' : ''}`}
              htmlFor="stayOnHubAfterCreate"
              style={stayOnHubAfterCreateSaving ? { pointerEvents: 'none', opacity: 0.6 } : undefined}
            >
              <span className="settings-panel__toggle-track">
                <span className="settings-panel__toggle-thumb" />
              </span>
              <span className="settings-panel__toggle-label">Stay on Hub after creating a session</span>
            </label>
          )}
          <input
            type="checkbox"
            id="stayOnHubAfterCreate"
            className="settings-panel__toggle-input"
            checked={stayOnHubAfterCreate}
            onChange={() => void handleToggleStayOnHub()}
          />
          <p className="settings-panel__description">
            When enabled, creating a new session from the Hub keeps you on the Hub tab instead of switching to it. The session is still created and can be opened from its card or the tab strip anytime. Off by default.
          </p>
          {stayOnHubAfterCreateError && <p className="settings-panel__error">{stayOnHubAfterCreateError}</p>}
        </div>

        {/* Phase 150 SET-01 — shell web-share warning toggle (Session Behavior, D-05/D-06) */}
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

        {/* D-07 confirm-on-disable dialog for shell web-share warning (reuses RegenerateKeyModal .quit-modal* pattern with action-specific copy) */}
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

        {/* Appearance section (SETT-02) — POL-02: single role=switch toggle (D-06 colorblind-safe) */}
        <h3 id="settings-appearance">Appearance</h3>
        <div className="settings-panel__field-group">
          <label className="settings-panel__label">Interface Theme</label>
          <button
            type="button"
            role="switch"
            aria-checked={uiTheme === 'light'}
            aria-label={uiTheme === 'light' ? 'Light mode — click to switch to dark' : 'Dark mode — click to switch to light'}
            className={`settings-panel__theme-toggle${uiTheme === 'light' ? ' settings-panel__theme-toggle--light' : ''}`}
            onClick={() => onUiThemeChange(uiTheme === 'light' ? 'dark' : 'light')}
          >
            <span className="settings-panel__theme-toggle-track">
              <span className="settings-panel__theme-toggle-knob" aria-hidden="true">
                {uiTheme === 'light'
                  ? <><SunIcon className="settings-panel__theme-toggle-icon" /><span>{'Light'}</span></>
                  : <><MoonIcon className="settings-panel__theme-toggle-icon" /><span>{'Dark'}</span></>
                }
              </span>
            </span>
          </button>
          <p className="settings-panel__description" style={{ marginTop: '0.5rem' }}>Switches the whole app between light and dark appearance. Default is dark.</p>
        </div>
        <div className="settings-panel__field-group">
          <label className="settings-panel__label">Terminal Theme</label>
          <select
            className="settings-panel__path-input settings-panel__theme-select"
            value={selectedTheme}
            onChange={(e) => onThemeChange(e.target.value)}
          >
            {THEME_NAMES.map(name => (
              <option key={name} value={name}>{name.replace(/_/g, ' ')}</option>
            ))}
          </select>
          <p className="settings-panel__description" style={{ marginTop: '0.5rem' }}>Themes may not apply correctly to existing sessions. Only sessions created after selecting a new theme will display as intended.</p>
        </div>

        {/* Web Server section (SETT-02) */}
        <h3 id="settings-web-server">Web Server</h3>
        <p className="settings-panel__description">
          Enable HTTPS access to terminal sessions from your Tailscale network.
        </p>

        {/* Tailscale Status Indicator */}
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
                  : tailscaleHealth.permissionLimited
                    ? "Tailscale is running, but AgentHub can't read its status on this macOS account. Grant this account admin access, or install the Homebrew tailscale build (which uses a different socket path)."
                    : tailscaleHealth.daemonUp
                      ? 'Daemon running but not connected to a Tailscale network'
                      : tailscaleHealth.binaryFound
                        ? (tailscaleHealth.platformHint === 'darwin'
                            ? 'Installed but not running \u2014 open Tailscale from Applications or the menu bar'
                            : 'Installed but daemon is not running')
                        : 'Not detected \u2014 install from tailscale.com')
              : 'Checking\u2026'}
          </p>
          {tailscaleHealth && !tailscaleHealth.connected && (
            <details className="settings-panel__details" style={{ marginTop: '0.5rem' }}>
              <summary style={{ cursor: 'pointer', color: '#7aa2f7', fontSize: '0.8rem' }}>Show diagnostics</summary>
              <div style={{ marginTop: '0.5rem', fontSize: '0.8rem' }}>
                {/* Step 1: Binary detected */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                  <span style={{ color: tailscaleHealth.binaryFound ? '#9ece6a' : '#f7768e', fontFamily: 'monospace' }}>
                    {tailscaleHealth.binaryFound ? '\u2713' : '\u2717'}
                  </span>
                  <span style={{ color: tailscaleHealth.binaryFound ? '#c0caf5' : '#f7768e' }}>Binary detected</span>
                </div>
                {/* Step 2: Daemon running */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                  <span style={{ color: !tailscaleHealth.binaryFound ? '#414868' : tailscaleHealth.daemonUp ? '#9ece6a' : '#f7768e', fontFamily: 'monospace' }}>
                    {!tailscaleHealth.binaryFound ? '\u2500' : tailscaleHealth.daemonUp ? '\u2713' : '\u2717'}
                  </span>
                  <span style={{ color: !tailscaleHealth.binaryFound ? '#414868' : tailscaleHealth.daemonUp ? '#c0caf5' : '#f7768e' }}>Daemon running</span>
                </div>
                {/* Step 3: Connected to Tailscale */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                  <span style={{ color: !tailscaleHealth.daemonUp ? '#414868' : tailscaleHealth.connected ? '#9ece6a' : '#f7768e', fontFamily: 'monospace' }}>
                    {!tailscaleHealth.daemonUp ? '\u2500' : tailscaleHealth.connected ? '\u2713' : '\u2717'}
                  </span>
                  <span style={{ color: !tailscaleHealth.daemonUp ? '#414868' : '#c0caf5' }}>Connected to Tailscale</span>
                </div>
                {/* Step 4: TLS certificates ready */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ color: !tailscaleHealth.connected ? '#414868' : tailscaleHealth.hasCerts ? '#9ece6a' : '#f59e0b', fontFamily: 'monospace' }}>
                    {!tailscaleHealth.connected ? '\u2500' : tailscaleHealth.hasCerts ? '\u2713' : '\u2717'}
                  </span>
                  <span style={{ color: !tailscaleHealth.connected ? '#414868' : '#c0caf5' }}>TLS certificates ready</span>
                </div>
              </div>
            </details>
          )}
        </div>

        {/* CT Disclosure Banner */}
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

        {/* Port */}
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

        {/* Start/Stop Server */}
        <div className="settings-panel__field-group">
          <button
            className={`settings-panel__btn ${isServerRunning ? 'settings-panel__btn--cancel' : 'settings-panel__btn--save'}`}
            onClick={handleToggleServer}
            disabled={serverLoading || (!isServerRunning && !ctDisclosed)}
            title={
              !ctDisclosed && !isServerRunning
                ? 'Acknowledge the Certificate Transparency disclosure first'
                : undefined
            }
          >
            {serverLoading
              ? (isServerRunning ? 'Stopping\u2026' : 'Starting\u2026')
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

        {/* LAN Password — only shown in local network mode */}
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
              <span>{localPassword || 'Loading\u2026'}</span>
              <span className={`settings-web-server__copy-hint${copied ? ' settings-web-server__copy-hint--copied' : ''}`}>
                {copied ? 'Copied!' : '(click to copy)'}
              </span>
            </div>
          </>
        )}

        {/* Security section (Phase 87 D-16, UI-SPEC Surface 2) */}
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
        <RegenerateKeyModal
          isOpen={showRegenModal}
          onConfirm={handleRegenerateSigningKey}
          onCancel={() => setShowRegenModal(false)}
        />

        {/* Plugins section (Phase 92 PUI-01, Phase 99 PUI-02) \u2014 last section per UI-SPEC layout */}
        <PluginsSection onPluginToggleSideEffect={onPluginToggleSideEffect} />

      </div>
    </div>
  )
}
