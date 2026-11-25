// Portty 06-03 (UPD-01, UPD-03): the real App follows Go's update status. It reads
// GetUpdateStatus on mount, then update:status events, and the banner's Update
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
  it('an available status from GetUpdateStatus shows the banner with Update', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    expect(c.textContent).toContain('Portty 9.9.9 is available.')
    expect(button(c, 'Update')).toBeDefined()
  })

  it('Download calls DownloadUpdate once', async () => {
    const download = vi.fn(() => available)
    const c = await mountApp({ GetUpdateStatus: () => available, DownloadUpdate: download })
    await act(async () => { button(c, 'Update')!.click() })
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
    expect(button(c, 'Update')).toBeUndefined()
  })

  it('installing and relaunching events render Installing and Restarting with no action button', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await fire({ state: 'installing' })
    expect(c.textContent).toContain('Installing Portty 9.9.9…')
    expect(button(c, 'Update')).toBeUndefined()
    await fire({ state: 'relaunching' })
    expect(c.textContent).toContain('Restarting Portty 9.9.9…')
    expect(button(c, 'Update')).toBeUndefined()
  })

  it("a revealed event with an error renders the warning with Go's text", async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    const error = "Portty couldn't install the update, so the download is shown in Finder."
    await fire({ state: 'revealed', error })
    expect(c.textContent).toContain(error)
    expect(c.querySelector('.banner--warning')).not.toBeNull()
    expect(button(c, 'Update')).toBeUndefined()
    expect(button(c, 'Try Again')).toBeUndefined()
  })

  it('a GetUpdateStatus reply that lands after a newer event does not roll the banner back', async () => {
    let reply: (s: unknown) => void = () => {}
    const c = await mountApp({ GetUpdateStatus: () => new Promise((r) => { reply = r }) })
    await fire({ state: 'downloading', done: 50, total: 200 })
    await act(async () => { reply(available) })
    await flush()
    expect(c.textContent).toContain('Downloading Portty 9.9.9… 25%')
    expect(button(c, 'Update')).toBeUndefined()
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
    expect(button(c, 'Update')).toBeDefined()
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

  it('an installing event brings back a dismissed banner, and relaunching keeps it', async () => {
    const c = await mountApp({ GetUpdateStatus: () => available })
    await dismiss(c)
    expect(c.textContent).not.toContain('Portty 9.9.9')
    await fire({ state: 'installing' })
    expect(c.textContent).toContain('Installing Portty 9.9.9…')
    await fire({ state: 'relaunching' })
    expect(c.textContent).toContain('Restarting Portty 9.9.9…')
  })

  it('Try Again in the error state calls DownloadUpdate once more', async () => {
    const download = vi.fn(() => available)
    const c = await mountApp({ GetUpdateStatus: () => available, DownloadUpdate: download })
    await act(async () => { button(c, 'Update')!.click() })
    const error = "The download didn't pass the signature check, so Portty deleted it."
    await fire({ state: 'available', error })
    expect(c.textContent).toContain(error)
    expect(button(c, 'Update')).toBeUndefined()
    await act(async () => { button(c, 'Try Again')!.click() })
    await flush()
    expect(download).toHaveBeenCalledTimes(2)
  })

  it('update:show after a dismissal shows the available banner again and downloads nothing', async () => {
    const download = vi.fn(() => available)
    const c = await mountApp({ GetUpdateStatus: () => available, DownloadUpdate: download })
    await dismiss(c)
    expect(c.textContent).not.toContain('Portty 9.9.9')

    await act(async () => { wailsEvents.get('update:show')!() })
    await flush()
    expect(c.textContent).toContain('Portty 9.9.9 is available.')
    expect(button(c, 'Update')).toBeDefined()
    expect(download).not.toHaveBeenCalled()
  })

  it('update:show with an idle status shows no banner', async () => {
    const c = await mountApp({ GetUpdateStatus: () => ({ ...available, state: 'idle', latest: '' }) })
    await act(async () => { wailsEvents.get('update:show')!() })
    await flush()
    expect(c.textContent).not.toContain('is available')
    expect(banner(c)).toBeNull()
  })

  it('web mode never reads the update status or subscribes to update:status', async () => {
    window.history.pushState({}, '', '/app/?session=s1&cap=c1')
    vi.stubGlobal('fetch', vi.fn(async () => new Response('forbidden', { status: 403 })))
    const getStatus = vi.fn(() => available)
    await mountApp({ GetUpdateStatus: getStatus })
    expect(getStatus).not.toHaveBeenCalled()
    expect(wailsEvents.has('update:status')).toBe(false)
    expect(wailsEvents.has('update:show')).toBe(false)
  })
})
