import { describe, it, expect } from 'vitest'
import raw from '../../components/SettingsSheet.tsx?raw'
import pluginsRaw from '../../components/PluginsSection.tsx?raw'


// Portty 01-11: Save Plugins is the only Save button left in Settings (the
// Save Paths flow went with Settings → Paths), so the save-confirmation
// contract is checked on PluginsSection.
describe('SET-03: Save confirmation feedback', () => {
    it('has saved state variable', () => {
        expect(pluginsRaw).toContain('setSaved')
    })

    it('sets saved true after successful save', () => {
        expect(pluginsRaw).toContain('setSaved(true)')
    })

    it('resets saved after timeout', () => {
        expect(pluginsRaw).toContain('setSaved(false)')
    })

    it('displays Saved! text', () => {
        expect(pluginsRaw).toContain("'Saved!'")
    })

    it('is the shared secondary push button (Portty 02-11: no green fill)', () => {
        expect(pluginsRaw).toContain('className="button button--secondary"')
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
