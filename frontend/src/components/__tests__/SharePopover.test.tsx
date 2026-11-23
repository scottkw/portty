/**
 * Portty 04-05 (SHAR-01, 04-UI-SPEC "Share popover"): the one-switch share
 * popover. States, copy, errors, Tailscale off, focus and close.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import raw from '../SharePopover.tsx?raw'

vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  ClipboardSetText: vi.fn(),
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  GetSessionShare: vi.fn(),
  SetSessionShared: vi.fn(),
  HasCTDisclosure: vi.fn(),
}))

import { GetSessionShare, SetSessionShared, HasCTDisclosure } from '../../wailsjs/go/main/App'
import { ClipboardSetText } from '../../wailsjs/wailsjs/runtime/runtime'
import { SharePopover, type SharePopoverProps } from '../SharePopover'
import { TAILSCALE_OFF_REASON } from '../../lib/tailscale'
import { menuShareError, menuStopError, shareStopError } from '../../lib/shareCopy'

const URL = 'https://office-air.tail1234.ts.net/sessions/s1?cap=TOKEN'
const off = { shared: false, url: '', firstShare: false, errorCode: '', errorDetail: '' }
const on = { shared: true, url: URL, firstShare: false, errorCode: '', errorDetail: '' }

let container: HTMLDivElement
let root: Root
let anchor: HTMLButtonElement

function q<T extends Element = HTMLElement>(sel: string): T | null {
  return document.body.querySelector<T>(sel)
}
const sw = () => q<HTMLButtonElement>('[role="switch"]')!
const state = () => q('.share-popover__state')!
const link = () => q<HTMLInputElement>('input[aria-label="Share link"]')
const copyBtn = () => q<HTMLButtonElement>('.share-popover__copy')

async function render(props: Partial<SharePopoverProps> = {}) {
  const all: SharePopoverProps = {
    sessionId: 's1',
    sessionName: 'build',
    shared: false,
    anchor,
    popoverId: 'share-popover',
    tailscaleOff: false,
    tailscaleDomain: 'office-air.tail1234.ts.net',
    onClose: vi.fn(),
    onSharedChange: vi.fn(),
    ...props,
  }
  await act(async () => root.render(<SharePopover {...all} />))
  return all
}

async function click(el: HTMLElement) {
  await act(async () => el.click())
}

beforeEach(() => {
  vi.mocked(GetSessionShare).mockResolvedValue(off)
  vi.mocked(SetSessionShared).mockResolvedValue(on)
  vi.mocked(HasCTDisclosure).mockResolvedValue(false)
  vi.mocked(ClipboardSetText).mockResolvedValue(true)
  anchor = document.createElement('button')
  anchor.textContent = 'Share'
  document.body.appendChild(anchor)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
  anchor.remove()
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('SharePopover: element and off state', () => {
  it('is a non-modal role="dialog" named Share “<name>” with one switch and no tabs, title or close button', async () => {
    await render()
    const d = q('[role="dialog"]')!
    expect(d.getAttribute('aria-label')).toBe('Share “build”')
    expect(d.hasAttribute('aria-modal')).toBe(false)
    expect(d.id).toBe('share-popover')
    expect(d.querySelectorAll('[role="switch"]')).toHaveLength(1)
    expect(d.querySelector('[role="tablist"], h1, h2, h3, [aria-label="Close"]')).toBeNull()
    expect(q(`label[for="${sw().id}"]`)!.textContent).toBe('Share this session')
  })

  it('off: switch off, the description and the CT line', async () => {
    await render()
    expect(sw().getAttribute('aria-checked')).toBe('false')
    expect(sw().hasAttribute('aria-disabled')).toBe(false)
    expect(state().textContent).toContain(
      'Macs signed in to your Tailscale account see it in their sidebar, and you get a link that opens in a browser on any device signed in to your account.',
    )
    expect(state().textContent).toContain(
      'The first share gets an HTTPS certificate for office-air.tail1234.ts.net. That name is published in public Certificate Transparency logs.',
    )
  })

  it('hides the CT line once the disclosure is acknowledged', async () => {
    vi.mocked(HasCTDisclosure).mockResolvedValue(true)
    await render()
    expect(state().textContent).not.toContain('Certificate Transparency')
  })

  it('the state region is aria-live="polite"', async () => {
    await render()
    expect(state().getAttribute('aria-live')).toBe('polite')
  })

  it('focus starts on the switch', async () => {
    await render()
    expect(document.activeElement).toBe(sw())
  })
})

describe('SharePopover: turning Share on', () => {
  it('shows Getting a link… with the switch aria-disabled, then the link, Copy and the warning', async () => {
    let resolve!: (v: typeof on) => void
    vi.mocked(SetSessionShared).mockReturnValue(new Promise((r) => { resolve = r }))
    const props = await render()
    await click(sw())
    expect(SetSessionShared).toHaveBeenCalledWith('s1', true)
    expect(sw().getAttribute('aria-checked')).toBe('true')
    expect(sw().getAttribute('aria-disabled')).toBe('true')
    expect(state().textContent).toBe('Getting a link…')
    await act(async () => resolve(on))
    expect(sw().hasAttribute('aria-disabled')).toBe(false)
    expect(link()!.value).toBe(URL)
    expect(link()!.readOnly).toBe(true)
    expect(link()!.getAttribute('spellcheck')).toBe('false')
    expect(copyBtn()!.textContent).toBe('Copy')
    expect(state().textContent).toContain('Treat this link like a password. It only opens on devices signed in to your Tailscale account.')
    expect(props.onSharedChange).toHaveBeenCalledWith('s1', true)
  })

  it('opening a shared session shows its link straight away', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    await render({ shared: true })
    expect(sw().getAttribute('aria-checked')).toBe('true')
    expect(link()!.value).toBe(URL)
    // The link state takes the 12px gap under the switch row; text states keep 8px.
    expect(state().classList.contains('share-popover__state--link')).toBe(true)
  })

  it('a shared session whose link is still loading shows Getting a link…, never an empty body', async () => {
    vi.mocked(GetSessionShare).mockReturnValue(new Promise(() => {}))
    await render({ shared: true })
    expect(sw().getAttribute('aria-checked')).toBe('true')
    expect(link()).toBeNull()
    expect(state().textContent).toBe('Getting a link…')
  })

  it('a shared session whose link fails to load shows the error, not a blank body', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue({ ...off, errorCode: 'unavailable', errorDetail: 'daemon not connected' })
    await render({ shared: true })
    expect(sw().getAttribute('aria-checked')).toBe('true')
    expect(q('.share-popover__error')!.textContent).toBe(
      "Couldn't get the link: daemon not connected. Check that Tailscale is connected, then try again.",
    )
  })

  it('a rejected link lookup is shown too, not swallowed', async () => {
    vi.mocked(GetSessionShare).mockRejectedValue('daemon not connected')
    await render({ shared: true })
    expect(q('.share-popover__error')!.textContent).toBe(
      "Couldn't get the link: daemon not connected. Check that Tailscale is connected, then try again.",
    )
  })

  it('the link field opens scrolled to its end, where links differ', async () => {
    let left = 0
    const proto = HTMLInputElement.prototype
    Object.defineProperty(proto, 'scrollWidth', { configurable: true, get: () => 500 })
    Object.defineProperty(proto, 'scrollLeft', { configurable: true, get: () => left, set: (v: number) => { left = v } })
    try {
      vi.mocked(GetSessionShare).mockResolvedValue(on)
      await render({ shared: true })
      expect(link()!.scrollLeft).toBe(500)
    } finally {
      delete (proto as unknown as Record<string, unknown>).scrollWidth
      delete (proto as unknown as Record<string, unknown>).scrollLeft
    }
  })

  it('a 300-character link sits whole in one input', async () => {
    const long = 'https://office-air.tail1234.ts.net/sessions/s1?cap=' + 'x'.repeat(300 - 51)
    expect(long).toHaveLength(300)
    vi.mocked(GetSessionShare).mockResolvedValue({ ...on, url: long })
    await render({ shared: true })
    expect(document.body.querySelectorAll('.share-popover input')).toHaveLength(1)
    expect(link()!.value).toBe(long)
  })

  it('https_off: the HTTPS sentence, switch back off', async () => {
    vi.mocked(SetSessionShared).mockResolvedValue({ ...off, errorCode: 'https_off', errorDetail: 'HTTPS certificates are off for your tailnet' })
    const props = await render()
    await click(sw())
    expect(sw().getAttribute('aria-checked')).toBe('false')
    const err = q('.share-popover__error')!
    expect(err.textContent).toBe(
      "Couldn't start sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again.",
    )
    // The error sits above the description.
    expect(state().firstElementChild).toBe(err)
    expect(state().textContent).toContain('Macs signed in to your Tailscale account')
    expect(props.onSharedChange).not.toHaveBeenCalled()
  })

  it('any other code: the general sentence with the detail', async () => {
    vi.mocked(SetSessionShared).mockResolvedValue({ ...off, errorCode: 'start_failed', errorDetail: 'port 8443 is in use' })
    await render()
    await click(sw())
    expect(q('.share-popover__error')!.textContent).toBe(
      "Couldn't start sharing: port 8443 is in use. Check that Tailscale is connected, then try again.",
    )
  })
})

describe('SharePopover: error detail', () => {
  it('a thrown Error loses its "Error: " prefix and its own trailing period', async () => {
    vi.mocked(SetSessionShared).mockRejectedValue(new Error('dial tcp 100.64.0.1:443: connect: connection refused.'))
    await render()
    await click(sw())
    expect(q('.share-popover__error')!.textContent).toBe(
      "Couldn't start sharing: dial tcp 100.64.0.1:443: connect: connection refused. Check that Tailscale is connected, then try again.",
    )
  })

  it('the stop error and both menu banners clean the detail the same way', () => {
    expect(shareStopError('Error: daemon not connected.')).toBe("Couldn't stop sharing: daemon not connected. Try again.")
    expect(menuShareError('build', 'Error: port 8443 is in use.')).toBe(
      "Couldn't share “build”: port 8443 is in use. Check that Tailscale is connected, then try again.",
    )
    expect(menuStopError('build', 'daemon not connected..')).toBe("Couldn't stop sharing “build”: daemon not connected. Try again.")
  })
})

describe('SharePopover: turning Share off', () => {
  it('stopping removes the link at once; success reports the change', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    let resolve!: (v: typeof off) => void
    vi.mocked(SetSessionShared).mockReturnValue(new Promise((r) => { resolve = r }))
    const props = await render({ shared: true })
    await click(sw())
    expect(sw().getAttribute('aria-checked')).toBe('false')
    expect(sw().getAttribute('aria-disabled')).toBe('true')
    expect(link()).toBeNull()
    expect(state().textContent).toBe('')
    await act(async () => resolve(off))
    expect(props.onSharedChange).toHaveBeenCalledWith('s1', false)
    expect(state().textContent).toContain('Macs signed in to your Tailscale account')
  })

  it('stop failure keeps the link and shows the error under the warning', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    vi.mocked(SetSessionShared).mockResolvedValue({ ...off, errorCode: 'stop_failed', errorDetail: 'daemon not connected' })
    await render({ shared: true })
    await click(sw())
    expect(sw().getAttribute('aria-checked')).toBe('true')
    expect(link()!.value).toBe(URL)
    expect(state().lastElementChild!.textContent).toBe("Couldn't stop sharing: daemon not connected. Try again.")
  })
})

describe('SharePopover: Copy', () => {
  it('Copy writes the full link and reads Copied! for 2 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    await render({ shared: true })
    await click(copyBtn()!)
    expect(ClipboardSetText).toHaveBeenCalledWith(URL)
    expect(copyBtn()!.textContent).toBe('Copied!')
    await act(async () => { vi.advanceTimersByTime(1999) })
    expect(copyBtn()!.textContent).toBe('Copied!')
    await act(async () => { vi.advanceTimersByTime(1) })
    expect(copyBtn()!.textContent).toBe('Copy')
  })

  it('a failed clipboard write shows the copy error and selects the field', async () => {
    vi.mocked(ClipboardSetText).mockResolvedValue(false)
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    await render({ shared: true })
    await click(copyBtn()!)
    expect(copyBtn()!.textContent).toBe('Copy')
    expect(state().lastElementChild!.textContent).toBe("Couldn't copy the link. Select it and press ⌘C.")
    expect(link()!.selectionStart).toBe(0)
    expect(link()!.selectionEnd).toBe(URL.length)
  })
})

describe('SharePopover: Tailscale off', () => {
  it('not shared: switch aria-disabled with aria-describedby to the visible reason; clicks do nothing', async () => {
    await render({ tailscaleOff: true })
    expect(sw().getAttribute('aria-disabled')).toBe('true')
    const reason = document.getElementById(sw().getAttribute('aria-describedby')!)!
    expect(reason.textContent).toBe(TAILSCALE_OFF_REASON)
    expect(state().contains(reason)).toBe(true)
    await click(sw())
    expect(SetSessionShared).not.toHaveBeenCalled()
  })

  it('a start error from before Tailscale dropped is cleared, so only the reason shows', async () => {
    vi.mocked(SetSessionShared).mockResolvedValue({ ...off, errorCode: 'start_failed', errorDetail: 'port 8443 is in use' })
    await render()
    await click(sw())
    expect(q('.share-popover__error')).not.toBeNull()
    await render({ tailscaleOff: true })
    expect(q('.share-popover__error')).toBeNull()
    expect(state().textContent).toBe(TAILSCALE_OFF_REASON)
  })

  it('shared: the switch stays operable and the reason replaces the link and warning', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    vi.mocked(SetSessionShared).mockResolvedValue(off)
    await render({ shared: true, tailscaleOff: true })
    expect(sw().hasAttribute('aria-disabled')).toBe(false)
    expect(link()).toBeNull()
    expect(state().textContent).toBe(TAILSCALE_OFF_REASON)
    await click(sw())
    expect(SetSessionShared).toHaveBeenCalledWith('s1', false)
  })
})

describe('SharePopover: close and focus', () => {
  it('Esc closes and returns focus to the Share button', async () => {
    const props = await render()
    await act(async () => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(props.onClose).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(anchor)
  })

  it('a press outside closes; a press inside or on the Share button does not', async () => {
    const props = await render()
    await act(async () => { sw().dispatchEvent(new Event('pointerdown', { bubbles: true })) })
    await act(async () => { anchor.dispatchEvent(new Event('pointerdown', { bubbles: true })) })
    expect(props.onClose).not.toHaveBeenCalled()
    await act(async () => { container.dispatchEvent(new Event('pointerdown', { bubbles: true })) })
    expect(props.onClose).toHaveBeenCalledTimes(1)
  })

  it('the window losing key status closes it', async () => {
    const props = await render()
    await act(async () => { window.dispatchEvent(new Event('blur')) })
    expect(props.onClose).toHaveBeenCalledTimes(1)
  })

  it('Tab wraps among the switch, the link field and Copy', async () => {
    vi.mocked(GetSessionShare).mockResolvedValue(on)
    await render({ shared: true })
    const tab = async (shiftKey = false) => {
      await act(async () => {
        document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey, bubbles: true }))
      })
    }
    expect(document.activeElement).toBe(sw())
    await tab()
    expect(document.activeElement).toBe(link())
    await tab()
    expect(document.activeElement).toBe(copyBtn())
    await tab()
    expect(document.activeElement).toBe(sw())
    await tab(true)
    expect(document.activeElement).toBe(copyBtn())
  })
})

describe('SharePopover: position and source', () => {
  it('hangs 8px under the button, right edges aligned, clamped 12px from both window edges', async () => {
    anchor.getBoundingClientRect = () => ({ left: 900, right: 980, top: 12, bottom: 40, width: 80, height: 28, x: 900, y: 12, toJSON: () => ({}) })
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1000 })
    await render()
    const d = q<HTMLElement>('.share-popover')!
    expect(d.style.top).toBe('48px')
    expect(d.style.left).toBe(`${980 - 320}px`)
    expect(d.style.width).toBe('320px')
    // The arrow is centred on the button: 940 - 660 - 8.
    expect(q<SVGElement>('.share-popover__arrow')!.style.left).toBe('272px')
    await act(async () => root.unmount())
    root = createRoot(container)
    anchor.getBoundingClientRect = () => ({ left: 20, right: 100, top: 12, bottom: 40, width: 80, height: 28, x: 20, y: 12, toJSON: () => ({}) })
    await render()
    expect(q<HTMLElement>('.share-popover')!.style.left).toBe('12px')
  })

  it('uses no hex colours and no aria-modal', () => {
    expect(raw).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
    expect(raw).not.toContain('aria-modal')
  })
})
