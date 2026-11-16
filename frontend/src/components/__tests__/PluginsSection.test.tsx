import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { act } from 'react'
import raw from '../PluginsSection.tsx?raw'
import { PluginsSection } from '../PluginsSection'
import { SetPluginSettings, SetSearchConfig } from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetPluginSettings: vi.fn(async () => ({
    webgl: true, unicode11: false, search: true, webLinks: true, image: false,
    serialize: false, clipboard: false, progress: false,
    searchConfig: { regex: false, caseSensitive: false, wholeWord: false },
    webLinksConfig: { modifier: 'platform', confirmOSC8: true, confirmIDN: true, confirmTyposquat: true },
    imageConfig: { storageLimit: 16 },
  })),
  SetPluginSettings: vi.fn(async () => undefined),
  SetSearchConfig: vi.fn(async () => undefined),
  SetWebLinksConfig: vi.fn(async () => undefined),
  SetImageConfig: vi.fn(async () => undefined),
}))
// Wails-generated daemon.* constructors fail under jsdom.
vi.mock('../../wailsjs/go/models', () => {
  class Stub { constructor(o: Record<string, unknown> = {}) { Object.assign(this, o) } }
  return { daemon: { PluginSettings: Stub, SearchConfig: Stub, WebLinksConfig: Stub, ImageConfig: Stub } }
})

describe('PUI-01: 8 toggle rows in UI-SPEC order', () => {
  it('contains all 8 plugin labels', () => {
    expect(raw).toContain('WebGL renderer')
    expect(raw).toContain('Unicode 11 widths')
    expect(raw).toContain('Find in scrollback')
    expect(raw).toContain('Clickable web links')
    expect(raw).toContain('Inline images')
    expect(raw).toContain('Save terminal as text')
    expect(raw).toContain('Clipboard (OSC 52)')
    expect(raw).toContain('Progress (OSC 9;4)')
  })

  it('contains all 8 UI-SPEC one-sentence descriptions', () => {
    expect(raw).toContain('GPU-accelerated terminal rendering')
    expect(raw).toContain('Correct cell widths for emoji')
    expect(raw).toContain('Open a find bar with Cmd-F')
    expect(raw).toContain('Detect URLs in terminal output')
    expect(raw).toContain('Render images sent via sixel')
    expect(raw).toContain('Right-click a tab to export')
    expect(raw).toContain('OSC 52 escape sequence')
    expect(raw).toContain('OSC 9;4 progress updates')
  })

  it('renders rows in UI-SPEC order (Pitfall #5 guard)', () => {
    const order = ['webgl', 'unicode11', 'search', 'webLinks', 'image',
                   'serialize', 'clipboard', 'progress']
    for (let i = 0; i < order.length - 1; i++) {
      const a = raw.indexOf(`renderRow('${order[i]}'`)
      const b = raw.indexOf(`renderRow('${order[i + 1]}'`)
      expect(a).toBeGreaterThan(-1)
      expect(b).toBeGreaterThan(-1)
      expect(a).toBeLessThan(b)
    }
  })
})

// Quick 261116-ku9 R5: switches save as they change (macOS); no explicit save button.
describe('PUI-01: plugin switches save on change', () => {
  it('has no explicit save button, save states or save row', () => {
    expect(raw).not.toContain('Save Plugins')
    expect(raw).not.toContain('Saved!')
    expect(raw).not.toMatch(/handleSavePlugins|setSaved|settings-sheet__save-row|<button/)
  })
  it('saves the flipped snapshot from the switch handler, one save in flight', () => {
    const body = raw.slice(raw.indexOf('const toggle = (key: PluginBooleanKey) => async () => {'), raw.indexOf('function renderRow('))
    expect(body).toContain('if (!prev || savingRef.current) return')
    expect(body).toContain('await SetPluginSettings(next)')
  })
})

describe('Quick 261116-ku9 R5: save-on-change behaviour (rendered)', () => {
  let container: HTMLDivElement
  let root: Root
  const sw = (key: string) => container.querySelector<HTMLButtonElement>(`#plugin-${key}`)!
  const flush = () => act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })

  async function mount(props: React.ComponentProps<typeof PluginsSection> = {}) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    await act(async () => root.render(<PluginsSection {...props} />))
    await flush()
  }

  beforeEach(() => {
    vi.mocked(SetPluginSettings).mockReset()
    vi.mocked(SetPluginSettings).mockResolvedValue(undefined)
    vi.mocked(SetSearchConfig).mockClear()
  })
  afterEach(async () => {
    await act(async () => root.unmount())
    container.remove()
  })

  it('a switch click saves once with that key flipped', async () => {
    await mount()
    await act(async () => sw('clipboard').click())
    await flush()
    expect(SetPluginSettings).toHaveBeenCalledTimes(1)
    expect(vi.mocked(SetPluginSettings).mock.calls[0][0]).toMatchObject({ clipboard: true, webgl: true })
    expect(sw('clipboard').getAttribute('aria-checked')).toBe('true')
  })

  it('ignores another switch while a save is pending', async () => {
    let resolve!: () => void
    vi.mocked(SetPluginSettings).mockImplementationOnce(() => new Promise<void>((r) => { resolve = r }))
    await mount()
    await act(async () => sw('clipboard').click())
    await act(async () => sw('progress').click())
    expect(SetPluginSettings).toHaveBeenCalledTimes(1)
    await act(async () => { resolve() })
    await flush()
    expect(sw('progress').getAttribute('aria-checked')).toBe('false')
  })

  it('a rejected save puts the switch back and shows the error', async () => {
    vi.mocked(SetPluginSettings).mockRejectedValueOnce(new Error('daemon not connected'))
    await mount()
    await act(async () => sw('clipboard').click())
    await flush()
    expect(sw('clipboard').getAttribute('aria-checked')).toBe('false')
    expect(container.querySelector('[role="alert"]')!.textContent).toBe('Could not save plugin settings — daemon not connected')
  })

  it.each([['unicode11'], ['image']])('flipping %s fires onPluginToggleSideEffect', async (key) => {
    const onPluginToggleSideEffect = vi.fn()
    await mount({ onPluginToggleSideEffect })
    await act(async () => sw(key).click())
    await flush()
    expect(onPluginToggleSideEffect).toHaveBeenCalledWith([key])
  })

  it('a disclosure sub-key change rides along in the next switch save snapshot', async () => {
    await mount()
    const regex = container.querySelector<HTMLInputElement>('#search-default-regex')!
    await act(async () => regex.click())
    expect(SetSearchConfig).toHaveBeenCalledTimes(1)
    expect(regex.checked).toBe(true)
    await act(async () => sw('clipboard').click())
    await flush()
    expect(vi.mocked(SetPluginSettings).mock.calls[0][0]).toMatchObject({ searchConfig: { regex: true } })
  })
})

describe('PLUG-02: pluginsLoaded flicker guard (Pitfall #3)', () => {
  it('declares pluginsLoaded state', () => {
    expect(raw).toMatch(/pluginsLoaded|setPluginsLoaded/)
  })
  it('gates toggle rows behind pluginsLoaded', () => {
    // The guard should appear in the JSX as a conditional wrap.
    expect(raw).toContain('pluginsLoaded')
  })
})

describe('PLUG-01/PLUG-03: Wails RPC binding usage', () => {
  it('imports GetPluginSettings + SetPluginSettings from Wails bindings', () => {
    expect(raw).toContain('GetPluginSettings')
    expect(raw).toContain('SetPluginSettings')
    expect(raw).toMatch(/wailsjs\/go\/main\/App/)
  })
})

describe('Phase 93 U11-01: italic caption under Unicode 11 row', () => {
  it('renders the verbatim caption "Applies to new sessions you create."', () => {
    expect(raw).toMatch(/Applies to new sessions you create\./)
  })

  it('uses the new settings-panel__description--italic modifier class', () => {
    expect(raw).toContain('settings-panel__description--italic')
  })

  it('caption appears AFTER the Unicode 11 row description (UI-SPEC layout)', () => {
    const captionIdx = raw.indexOf('Applies to new sessions you create.')
    const u11DescIdx = raw.indexOf('Correct cell widths for emoji')
    expect(captionIdx).toBeGreaterThan(-1)
    expect(u11DescIdx).toBeGreaterThan(-1)
    expect(captionIdx).toBeGreaterThan(u11DescIdx)
  })
})

describe('Phase 97 SER-02: italic secrets-warning caption under Serialize row — Plan 97-05 implementation', () => {
  it('Serialize renderRow carries the verbatim secrets warning as its 4th argument', () => {
    // Verbatim per REQUIREMENTS SER-02:
    const VERBATIM = 'Saved files include any secrets, tokens, or sensitive data printed in the session.'
    expect(raw).toContain(VERBATIM)

    // Positional check — the verbatim string must appear within ~250 chars
    // AFTER the 'Save terminal as text' label (the 2nd argument to the
    // serialize renderRow call). This proves it's the 4th argument to
    // serialize renderRow, not somewhere else (e.g. inside a comment or
    // a different row).
    const labelIdx = raw.indexOf('Save terminal as text')
    expect(labelIdx).toBeGreaterThan(-1)
    const window = raw.slice(labelIdx, labelIdx + 400)
    expect(window).toContain(VERBATIM)
  })
})

describe('IMG-01: italic next-session-only caption under Image row (Plan 96-04)', () => {
  it("Image renderRow carries 'Applies to new sessions you create.' as its 4th argument", () => {
    // The exact string must appear at least TWICE — once for unicode11
    // (existing — Phase 93 U11-01), once for image (this plan adds it).
    // UX consistency is intentional: both addons are next-session-only;
    // they share the same caption verbatim.
    const matches = raw.match(/Applies to new sessions you create\./g) || []
    expect(matches.length).toBeGreaterThanOrEqual(2)

    // Tighter assertion: the caption must appear within ~400 chars AFTER
    // the 'Inline images' label, proving it is the 4th argument of the
    // image renderRow call (not just floating elsewhere).
    const inlineImagesIdx = raw.indexOf("'Inline images'")
    expect(inlineImagesIdx).toBeGreaterThan(-1)
    const captionAfterImage = raw.indexOf('Applies to new sessions you create.', inlineImagesIdx)
    expect(captionAfterImage).toBeGreaterThan(-1)
    expect(captionAfterImage - inlineImagesIdx).toBeLessThan(400)
  })
})

// Quick 261116-ku9 R3 (BRND-04): Portty copy — no milestone caption, no agent-era wording, macOS-only click.
describe('Phase 98 PRG-01: Progress row copy', () => {
  it('the progress row has no caption (the milestone caption is gone)', () => {
    expect(raw).not.toMatch(/v3\.2|v3\.3/)
    const row = raw.slice(raw.indexOf("renderRow('progress'"), raw.indexOf('</div>', raw.indexOf("renderRow('progress'")))
    expect(row).toContain("'Show a per-tab progress underline when a program in the terminal emits OSC 9;4 progress updates.')}")
  })

  it('clipboard says "a program in the terminal" and the link row is macOS-only', () => {
    expect(raw).toContain("'Allow a program in the terminal to place text on the system clipboard via the OSC 52 escape sequence.'")
    expect(raw).toContain("'Detect URLs in terminal output and make them clickable with ⌘-click.'")
    expect(raw).not.toMatch(/running CLI|Ctrl-click/)
  })

  it('Progress renderRow uses settings-panel__description--italic modifier class', () => {
    // The renderRow helper already routes caption into the italic class.
    // Verify the class exists in the source (set up by prior phases).
    expect(raw).toContain('settings-panel__description--italic')
  })

  it('keeps the Progress toggle default-OFF (progress key is false by default in daemon PluginSettings)', () => {
    // Source-level assertion: the progress row must NOT hardcode checked={true}.
    // Render-time behavior is daemon-side (Phase 92); this asserts the source
    // does not override the daemon default at the component level.
    const progressRowStart = raw.indexOf("'progress'")
    expect(progressRowStart).toBeGreaterThan(-1)
    const progressRowWindow = raw.slice(progressRowStart, progressRowStart + 300)
    expect(progressRowWindow).not.toContain('checked={true}')
  })
})

describe('Phase 99 PUI-02: onPluginToggleSideEffect side-effect callback', () => {
  it('exports PluginToggleKind type and PluginsSectionProps interface', () => {
    expect(raw).toContain('PluginToggleKind')
    expect(raw).toContain('PluginsSectionProps')
    expect(raw).toContain('onPluginToggleSideEffect')
  })

  it('holds a lastSavedRef for diff detection', () => {
    expect(raw).toContain('lastSavedRef')
  })

  it('compares prior.unicode11 vs the saved snapshot', () => {
    expect(raw).toContain('prior.unicode11 !== next.unicode11')
  })

  it('compares prior.image vs the saved snapshot', () => {
    expect(raw).toContain('prior.image !== next.image')
  })

  it('updates lastSavedRef.current after a successful switch save', () => {
    const saveIdx = raw.indexOf('await SetPluginSettings(next)')
    expect(saveIdx).toBeGreaterThan(-1)
    expect(raw.indexOf('lastSavedRef.current = next', saveIdx)).toBeGreaterThan(-1)
  })

  it('calls onPluginToggleSideEffect when kinds array is non-empty', () => {
    expect(raw).toContain('onPluginToggleSideEffect')
    expect(raw).toMatch(/kinds\.length > 0/)
  })
})
