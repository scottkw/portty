// Portty 06-03 (UPD-01, UPD-03): the real App follows Go's update status. It reads
// GetUpdateStatus on mount, then update:status events, and the banner's Download
// calls DownloadUpdate. Go reports progress, verifying and "shown in Finder" back
// through events; the UI never acts on the binding's result.
import { describe, it, expect, vi, afterEach } from 'vitest'
import { act } from 'react'

import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'

const available = { state: 'available', current: '0.9.0', latest: '9.9.9', done: 0, total: 0, error: '' }

const banner = (c: HTMLElement) => [...c.querySelectorAll('.banner')].find((b) => b.textContent?.includes('Portty 9.9') || b.querySelector('.banner__progress')) ?? null
const button = (c: HTMLElement, label: string) =>
  [...c.querySelectorAll('button')].find((b) => b.textContent === label) as HTMLButtonElement | undefined

async function fire(status: Record<string, unknown>) {
  await act(async () => { wailsEvents.get('update:status')!({ ...available, ...status }) })
  await flush()
}

afterEach(async () => {
  await unmountApp()
})

describe('App — update banner follows Go (06-03)', () => {
  it('an available status from GetUpdateStatus shows the banner with Download', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    expect(c.textContent).toContain('Portty 9.9.9 is available.')
    expect(button(c, 'Download')).toBeDefined()
  })

  it('Download calls DownloadUpdate once', async () => {
    const download = vi.fn(() => available)
    const c = await mountApp({ GetUpdateStatus: () => available, DownloadUpdate: download })
    await act(async () => { button(c, 'Download')!.click() })
    await flush()
    expect(download).toHaveBeenCalledTimes(1)
  })

  it('downloading, verifying and revealed events render progress, Verifying and shown in Finder', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await fire({ state: 'downloading', done: 50, total: 200 })
    expect(c.textContent).toContain('Downloading Portty 9.9.9… 25%')
    const progress = c.querySelector('progress.banner__progress') as HTMLProgressElement
    expect(progress).not.toBeNull()
    expect(progress.value).toBe(50)
    expect(progress.max).toBe(200)

    await fire({ state: 'verifying', done: 200, total: 200 })
    expect(c.textContent).toContain('Verifying Portty 9.9.9…')
    expect(c.querySelector('progress')).toBeNull()

    await fire({ state: 'revealed' })
    expect(c.textContent).toContain('Portty 9.9.9 is verified and shown in Finder.')
    expect(button(c, 'Download')).toBeUndefined()
  })

  it('no status and no events render no update banner', async () => {
    const c = await mountApp({})
    expect(banner(c)).toBeNull()
    expect(c.textContent).not.toContain('is available')
  })
})
