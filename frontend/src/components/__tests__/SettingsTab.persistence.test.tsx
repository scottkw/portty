import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import raw from '../../components/SettingsTab.tsx?raw'
import pluginsRaw from '../../components/PluginsSection.tsx?raw'

const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

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

    it('uses --saved CSS modifier', () => {
        expect(pluginsRaw).toContain('settings-panel__btn--saved')
    })
})

describe('SET-03: Saved button CSS', () => {
    it('has settings-panel__btn--saved class', () => {
        expect(cssRaw).toContain('.settings-panel__btn--saved')
    })

    it('uses green background for saved state', () => {
        expect(cssRaw).toContain('#9ece6a')
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
