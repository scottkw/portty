import { describe, it, expect } from 'vitest'
import raw from '../../components/SettingsSheet.tsx?raw'

describe('TRAY-01: Start-minimized toggle — Wails bindings', () => {
    it('imports GetStartMinimized from Wails bindings', () => {
        expect(raw).toContain('GetStartMinimized')
    })

    it('imports SetStartMinimized from Wails bindings', () => {
        expect(raw).toContain('SetStartMinimized')
    })
})

describe('TRAY-01: Start-minimized toggle — state variables', () => {
    it('has startMinimized state variable', () => {
        expect(raw).toContain('startMinimized')
        expect(raw).toContain('setStartMinimized')
    })

    it('has toggleLoaded state variable for flash prevention', () => {
        expect(raw).toContain('toggleLoaded')
        expect(raw).toContain('setToggleLoaded')
    })

    it('has toggleSaving state variable', () => {
        expect(raw).toContain('toggleSaving')
        expect(raw).toContain('setToggleSaving')
    })

    it('has toggleError state variable', () => {
        expect(raw).toContain('toggleError')
        expect(raw).toContain('setToggleError')
    })
})

describe('TRAY-01: Start-minimized toggle — mount behavior', () => {
    it('calls GetStartMinimized() in useEffect on mount', () => {
        expect(raw).toContain('GetStartMinimized()')
    })

    it('sets toggleLoaded true after GetStartMinimized resolves', () => {
        expect(raw).toContain('setToggleLoaded(true)')
    })
})

describe('TRAY-01: Start-minimized toggle — handler', () => {
    it('has handleToggleMinimized async function', () => {
        expect(raw).toContain('async function handleToggleMinimized')
    })

    it('calls await SetStartMinimized before setStartMinimized (non-optimistic)', () => {
        // Find the positions of both calls to verify ordering.
        const awaitPos = raw.indexOf('await SetStartMinimized(next)')
        const setPos = raw.indexOf('setStartMinimized(next)')
        expect(awaitPos).toBeGreaterThan(-1)
        expect(setPos).toBeGreaterThan(-1)
        expect(awaitPos).toBeLessThan(setPos)
    })
})

describe('TRAY-01: Start-minimized switch — JSX structure (Portty 02-11)', () => {
    it('labels the switch "Start hidden in the menu bar"', () => {
        expect(raw).toContain('label="Start hidden in the menu bar"')
        expect(raw).toContain('htmlFor="startMinimized"')
    })

    it('renders it as a Switch bound to startMinimized, shown only after load', () => {
        const start = raw.indexOf('{toggleLoaded && (')
        expect(start).toBeGreaterThan(-1)
        const block = raw.slice(start, raw.indexOf('</Row>', start))
        expect(block).toContain('<Switch')
        expect(block).toContain('id="startMinimized"')
        expect(block).toContain('checked={startMinimized}')
        expect(block).toContain('saving={toggleSaving}')
    })

    it('has the UI-SPEC description', () => {
        expect(raw).toContain('Opens with no window. Click the menu bar icon to show it.')
    })
})
