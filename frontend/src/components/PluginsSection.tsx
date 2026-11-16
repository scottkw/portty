import React, { useState, useEffect, useRef } from 'react'
import {
  GetPluginSettings,
  SetPluginSettings,
  SetSearchConfig,
  SetWebLinksConfig,
  SetImageConfig,
} from '../wailsjs/go/main/App'
import { daemon } from '../wailsjs/go/models'
import { Switch } from './Switch'

type PluginSettings = daemon.PluginSettings
const PluginSettings = daemon.PluginSettings

/**
 * PluginsSection — Phase 92 (PUI-01), Phase 99 (PUI-02)
 *
 * Renders the 8-plugin enable/disable section in the Settings sheet (General pane).
 * Each switch saves as it changes, the macOS way (quick 261116-ku9 R5): there is
 * no explicit save button, so closing the sheet can never drop a change. The GUI
 * layer (app.go) emits 'settings:plugins' on a successful save, which propagates
 * the new pluginConfig into every open TerminalPanel via App.tsx prop drilling.
 *
 * Phase 99 PUI-02: onPluginToggleSideEffect callback fires after a successful
 * save when unicode11 or image booleans changed vs the last-saved snapshot.
 * See PluginToggleKind + PluginsSectionProps exports below the function.
 */
export function PluginsSection({
  onPluginToggleSideEffect,
}: PluginsSectionProps = {}): React.ReactElement {
  // Local edited state — null until GetPluginSettings resolves
  const [pluginConfig, setPluginConfig] = useState<PluginSettings | null>(null)
  const [pluginsLoaded, setPluginsLoaded] = useState(false)

  // Phase 99 PUI-02: snapshot of the last-saved state for diff detection.
  // Initialized when GetPluginSettings resolves; updated after each successful save.
  const lastSavedRef = useRef<PluginSettings | null>(null)

  // Phase 99 PUI-03: 500ms debounce for ImageConfig.storageLimit number input
  // — avoids spamming SetImageConfig RPC while user types digits (T-99-05).
  const imageStorageDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    return () => {
      if (imageStorageDebounceRef.current) {
        clearTimeout(imageStorageDebounceRef.current)
      }
    }
  }, [])

  // One switch save in flight at a time: the ref guards synchronously, the state
  // drives the Switch's saving look (which also ignores clicks).
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const [error, setError] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    GetPluginSettings()
      .then((s) => {
        setPluginConfig(s)
        setPluginsLoaded(true)
        lastSavedRef.current = s
      })
      .catch((err) => {
        setLoadError(err instanceof Error ? err.message : String(err))
        setPluginsLoaded(true)
      })
  }, [])

  // Narrow the row keys to only boolean fields so renderRow stays type-safe
  // and `checked={...}` accepts the value (PluginSettings has a nested object
  // field added in Phase 94 — see daemon models).
  type PluginBooleanKey = {
    [K in keyof PluginSettings]: PluginSettings[K] extends boolean ? K : never
  }[keyof PluginSettings]

  // Save the full snapshot with this key flipped. Disclosure sub-keys are kept
  // in local state, so this snapshot never reverts them.
  const toggle = (key: PluginBooleanKey) => async () => {
    const prev = pluginConfig
    if (!prev || savingRef.current) return
    // Fresh PluginSettings instance so class identity and nested fields are preserved.
    const next = new PluginSettings({ ...prev, [key]: !prev[key] })
    savingRef.current = true
    setPluginConfig(next)
    setSaving(true)
    setError(null)
    try {
      await SetPluginSettings(next)
      // Phase 99 PUI-02: only unicode11 and image trigger the side-effect
      // (cannot hot-swap — new sessions only).
      const prior = lastSavedRef.current
      const kinds: PluginToggleKind[] = []
      if (prior) {
        if (prior.unicode11 !== next.unicode11) kinds.push('unicode11')
        if (prior.image !== next.image) kinds.push('image')
      }
      lastSavedRef.current = next
      if (kinds.length > 0 && onPluginToggleSideEffect) {
        onPluginToggleSideEffect(kinds)
      }
    } catch (err) {
      // Put only this switch back; a sub-key changed meanwhile stays.
      setPluginConfig((cur) => (cur ? new PluginSettings({ ...cur, [key]: prev[key] }) : cur))
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }

  // One grouped-form row per plugin (UI-SPEC "General pane"): label,
  // description and optional caption/disclosure on the left, Switch on the right.
  function renderRow(
    key: PluginBooleanKey,
    label: string,
    description: string,
    caption?: string,
    disclosure?: React.ReactNode,
  ): React.ReactElement {
    return (
      <div className="settings-row" key={key as string}>
        <div className="settings-row__text">
          <label className="settings-row__label" htmlFor={`plugin-${key as string}`}>{label}</label>
          <p className="settings-row__description">{description}</p>
          {caption && (
            <p className="settings-row__description settings-panel__description--italic">
              {caption}
            </p>
          )}
          {disclosure}
        </div>
        <div className="settings-row__control">
          <Switch
            id={`plugin-${key as string}`}
            checked={pluginConfig?.[key] ?? false}
            saving={saving}
            onChange={() => void toggle(key)()}
          />
        </div>
      </div>
    )
  }

  // Phase 99 PUI-03: inline <details> disclosures for plugins with meaningful
  // runtime config. Each disclosure dispatches a SUB-KEY RPC (SetSearchConfig /
  // SetWebLinksConfig / SetImageConfig) — never the full-snapshot save RPC —
  // to honor the PUI-04 anti-race contract from Phase 94-07 WR-03. Each also
  // writes its sub-key into local state, so a later switch save (full
  // snapshot) carries it instead of reverting it (quick 261116-ku9 R5).
  const renderSearchDisclosure = (): React.ReactNode => {
    if (!pluginsLoaded || !pluginConfig) return null
    const sc = pluginConfig.searchConfig
    const dispatch = (next: daemon.SearchConfig) => {
      setPluginConfig((prev) => (prev ? new PluginSettings({ ...prev, searchConfig: next }) : prev))
      SetSearchConfig(next).catch(() => { /* silent — sub-key RPC */ })
    }
    return (
      <details className="settings-panel__details">
        <summary>Search defaults</summary>
        <label className="settings-check" htmlFor="search-default-regex">
          <input
            type="checkbox"
            id="search-default-regex"
            checked={sc.regex}
            onChange={(e) => dispatch(new daemon.SearchConfig({ ...sc, regex: e.target.checked }))}
          />
          <span>Regex</span>
        </label>
        <label className="settings-check" htmlFor="search-default-case">
          <input
            type="checkbox"
            id="search-default-case"
            checked={sc.caseSensitive}
            onChange={(e) => dispatch(new daemon.SearchConfig({ ...sc, caseSensitive: e.target.checked }))}
          />
          <span>Case sensitive</span>
        </label>
        <label className="settings-check" htmlFor="search-default-word">
          <input
            type="checkbox"
            id="search-default-word"
            checked={sc.wholeWord}
            onChange={(e) => dispatch(new daemon.SearchConfig({ ...sc, wholeWord: e.target.checked }))}
          />
          <span>Whole word</span>
        </label>
      </details>
    )
  }

  const renderWebLinksDisclosure = (): React.ReactNode => {
    if (!pluginsLoaded || !pluginConfig) return null
    const wc = pluginConfig.webLinksConfig
    const dispatch = (next: daemon.WebLinksConfig) => {
      setPluginConfig((prev) => (prev ? new PluginSettings({ ...prev, webLinksConfig: next }) : prev))
      SetWebLinksConfig(next).catch(() => { /* silent — sub-key RPC */ })
    }
    return (
      <details className="settings-panel__details">
        <summary>Link click behavior</summary>
        <label className="settings-field">
          <span className="settings-row__description">Modifier</span>
          <select
            value={wc.modifier}
            onChange={(e) => dispatch(new daemon.WebLinksConfig({ ...wc, modifier: e.target.value }))}
          >
            <option value="platform">Platform default (Cmd on macOS, Ctrl elsewhere)</option>
            <option value="cmd">Cmd</option>
            <option value="ctrl">Ctrl</option>
            <option value="none">No modifier (plain click)</option>
          </select>
        </label>
        <label className="settings-check">
          <input
            type="checkbox"
            checked={wc.confirmOSC8}
            onChange={(e) => dispatch(new daemon.WebLinksConfig({ ...wc, confirmOSC8: e.target.checked }))}
          />
          <span>Confirm OSC 8 hyperlinks (mismatched display vs href)</span>
        </label>
        <label className="settings-check">
          <input
            type="checkbox"
            checked={wc.confirmIDN}
            onChange={(e) => dispatch(new daemon.WebLinksConfig({ ...wc, confirmIDN: e.target.checked }))}
          />
          <span>Confirm IDN / Punycode URLs</span>
        </label>
        <label className="settings-check">
          <input
            type="checkbox"
            checked={wc.confirmTyposquat}
            onChange={(e) => dispatch(new daemon.WebLinksConfig({ ...wc, confirmTyposquat: e.target.checked }))}
          />
          <span>Confirm typosquat patterns</span>
        </label>
      </details>
    )
  }

  const renderImageDisclosure = (): React.ReactNode => {
    if (!pluginsLoaded || !pluginConfig) return null
    const ic = pluginConfig.imageConfig
    return (
      <details className="settings-panel__details">
        <summary>Storage limit</summary>
        <label className="settings-field">
          <span className="settings-row__description">Per-terminal decoded image storage cap (default 16 MB; max 1000 MB).</span>
          <input
            className="settings-input settings-input--number"
            type="number"
            min={1}
            max={1000}
            step={1}
            value={ic.storageLimit}
            onChange={(e) => {
              const raw = Number(e.target.value)
              const v = Math.max(1, Math.min(1000, Number.isFinite(raw) ? raw : 16))
              setPluginConfig((prev) => {
                if (!prev) return prev
                return new PluginSettings({
                  ...prev,
                  imageConfig: new daemon.ImageConfig({ storageLimit: v }),
                })
              })
              if (imageStorageDebounceRef.current) clearTimeout(imageStorageDebounceRef.current)
              imageStorageDebounceRef.current = setTimeout(() => {
                SetImageConfig(new daemon.ImageConfig({ storageLimit: v })).catch(() => { /* silent */ })
              }, 500)
            }}
          />
          {' MB'}
        </label>
      </details>
    )
  }

  return (
    <>
      <h3 className="settings-section">Terminal Plugins</h3>
      {loadError && (
        <p className="settings-sheet__error" role="alert">
          Could not load plugin settings — {loadError}
        </p>
      )}
      {/* Rows render only after the values load (Pitfall #3 flicker guard).
          Order is non-negotiable per UI-SPEC: webgl → unicode11 → search →
          webLinks → image → serialize → clipboard → progress (Pitfall #5). */}
      {pluginsLoaded && pluginConfig && (
      <div className="settings-group">
      {renderRow('webgl', 'WebGL renderer',
        'GPU-accelerated terminal rendering with automatic DOM fallback if the GPU context is lost.')}
      {renderRow('unicode11', 'Unicode 11 widths',
        'Correct cell widths for emoji and CJK characters using the Unicode 11 width tables.',
        'Applies to new sessions you create.')}
      {renderRow('search', 'Find in scrollback',
        'Open a find bar with Cmd-F to search the terminal scrollback buffer.',
        undefined,
        renderSearchDisclosure())}
      {renderRow('webLinks', 'Clickable web links',
        'Detect URLs in terminal output and make them clickable with ⌘-click.',
        undefined,
        renderWebLinksDisclosure())}
      {renderRow('image', 'Inline images',
        'Render images sent via sixel or the iTerm2 inline image protocol directly inside the terminal.',
        'Applies to new sessions you create.',
        renderImageDisclosure())}
      {renderRow('serialize', 'Save terminal as text',
        'Right-click a tab to export the visible scrollback as a text file.',
        'Saved files include any secrets, tokens, or sensitive data printed in the session.')}
      {renderRow('clipboard', 'Clipboard (OSC 52)',
        'Allow a program in the terminal to place text on the system clipboard via the OSC 52 escape sequence.')}
      {renderRow('progress', 'Progress (OSC 9;4)',
        'Show a per-tab progress underline when a program in the terminal emits OSC 9;4 progress updates.')}
      </div>
      )}

      {error && (
        <p className="settings-sheet__error" role="alert">
          Could not save plugin settings — {error}
        </p>
      )}
    </>
  )
}

/**
 * Phase 99 PUI-02 type exports.
 * Declared after PluginsSection body so the plugin key literals ('webgl', 'unicode11', etc.)
 * in the renderRow call sites retain their UI-SPEC positional order for source-inspection tests.
 * TypeScript module-level type declarations are accessible throughout the module regardless
 * of declaration position (they are not runtime values).
 */
export type PluginToggleKind = 'unicode11' | 'image'

export interface PluginsSectionProps {
  onPluginToggleSideEffect?: (kinds: PluginToggleKind[]) => void
}
