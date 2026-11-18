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

describe('App — update banner rules (06-03)', () => {
  const wait = (ms: number) => act(async () => { await new Promise((r) => setTimeout(r, ms)) })
  const dismiss = async (c: HTMLElement) => {
    await act(async () => { (banner(c)!.querySelector('.banner__dismiss') as HTMLButtonElement).click() })
    await wait(250) // the 200 ms exit animation
    await flush()
  }

  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.pushState({}, '', '/')
  })

  it('a checking event leaves the available banner unchanged; current hides it', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await fire({ state: 'checking', latest: '' })
    expect(c.textContent).toContain('Portty 9.9.9 is available.')
    expect(button(c, 'Download')).toBeDefined()
    await fire({ state: 'current', latest: '' })
    expect(c.textContent).not.toContain('Portty 9.9.9')
  })

  it('idle hides the banner', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await fire({ state: 'idle', latest: '' })
    expect(c.textContent).not.toContain('Portty 9.9.9')
  })

  it('a dismissal holds for that version until a newer version or a download', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await dismiss(c)
    expect(c.textContent).not.toContain('Portty 9.9.9')

    await fire({ state: 'available' })
    expect(c.textContent).not.toContain('Portty 9.9.9')

    await fire({ state: 'available', latest: '9.9.10' })
    expect(c.textContent).toContain('Portty 9.9.10 is available.')

    await dismiss(c)
    expect(c.textContent).not.toContain('Portty 9.9.10')

    await fire({ state: 'downloading', latest: '9.9.10', done: 1, total: 4 })
    expect(c.textContent).toContain('Downloading Portty 9.9.10… 25%')
    await fire({ state: 'verifying', latest: '9.9.10' })
    expect(c.textContent).toContain('Verifying Portty 9.9.10…')
  })

  it('Try Again in the error state calls DownloadUpdate once more', async () => {
    const download = vi.fn(() => available)
    const c = await mountApp({ GetUpdateStatus: () => available, DownloadUpdate: download })
    await act(async () => { button(c, 'Download')!.click() })
    const error = "The download didn't pass the signature check, so Portty deleted it."
    await fire({ state: 'available', error })
    expect(c.textContent).toContain(error)
    expect(button(c, 'Download')).toBeUndefined()
    await act(async () => { button(c, 'Try Again')!.click() })
    await flush()
    expect(download).toHaveBeenCalledTimes(2)
  })

  it('web mode never reads the update status or subscribes to update:status', async () => {
    window.history.pushState({}, '', '/app/?session=s1&cap=c1')
    vi.stubGlobal('fetch', vi.fn(async () => new Response('forbidden', { status: 403 })))
    const getStatus = vi.fn(() => available)
    await mountApp({ GetUpdateStatus: getStatus })
    expect(getStatus).not.toHaveBeenCalled()
    expect(wailsEvents.has('update:status')).toBe(false)
  })
})
