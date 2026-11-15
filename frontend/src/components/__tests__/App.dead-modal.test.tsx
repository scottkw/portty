import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'

// Source-inspection guard: the retired SettingsPanel modal must not come back.
// Portty 02-11 intentionally reintroduces a `showSettings` state for the
// Settings *sheet* (CONTEXT ▸ Settings: "Settings opens as a sheet over the
// main window"), so this file guards the old component name instead.

describe('UI-02 Gap 6: the old SettingsPanel modal stays gone', () => {
  it('does NOT reference SettingsPanel (old modal component)', () => {
    expect(raw).not.toContain('SettingsPanel')
  })

  it('does NOT contain handleSettingsClose (old modal handler)', () => {
    expect(raw).not.toContain('handleSettingsClose')
  })

  it('imports the SettingsSheet component (Portty 02-11)', () => {
    expect(raw).toContain("from './components/SettingsSheet'")
  })
})
