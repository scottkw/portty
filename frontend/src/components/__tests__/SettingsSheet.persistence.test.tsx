import { describe, it, expect } from 'vitest'
import raw from '../../components/SettingsSheet.tsx?raw'
import pluginsRaw from '../../components/PluginsSection.tsx?raw'


// Quick 261116-ku9 R5: Settings has no Save button left. Plugin switches save
// as they change (the Save Paths flow went with Settings → Paths in 01-11), so
// the persistence contract is "each switch saves itself; failures show".
describe('SET-03: Plugin settings persist on change', () => {
    it('has no explicit save state or "Saved!" confirmation', () => {
        expect(pluginsRaw).not.toContain('setSaved')
        expect(pluginsRaw).not.toContain("'Saved!'")
    })

    it('each switch change awaits SetPluginSettings with the flipped snapshot', () => {
        expect(pluginsRaw).toContain('await SetPluginSettings(next)')
        expect(pluginsRaw).toContain('onChange={() => void toggle(key)()}')
    })

    it('a failed save restores the switch and shows the error', () => {
        expect(pluginsRaw).toContain('[key]: prev[key]')
        expect(pluginsRaw).toContain('Could not save plugin settings — {error}')
    })

    it('has no push button in the plugin section (Portty 02-11: no green fill)', () => {
        expect(pluginsRaw).not.toContain('<button')
        expect(pluginsRaw).not.toContain('settings-panel__btn--saved')
    })
})

describe('Portty 01-11: Settings → Paths removed', () => {
    it('has no file picker, CLI path or shell path plumbing', () => {
        expect(raw).not.toMatch(/OpenFileDialog|GetCLIPaths|UpdateCLIPath|GetShellPath|SetShellPath/)
    })

    it('has no Paths state, handlers or anchor', () => {
        expect(raw).not.toMatch(/setCustomPaths|handleBrowse|handleShellBrowse|handleSaveCLIPaths|settings-paths/)
    })
})
