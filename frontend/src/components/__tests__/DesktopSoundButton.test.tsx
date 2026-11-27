// Phase 23-06 — the Sound item on its own (23-UI-SPEC Surface 1, Accessibility contract): one component for every bar.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { DesktopSoundButton, type DesktopSoundButtonProps } from '../DesktopSoundButton'

let container: HTMLDivElement
let root: Root
let video: HTMLVideoElement
let play: ReturnType<typeof vi.spyOn>

const btn = () => container.querySelector('button') as HTMLButtonElement
const live = () => container.querySelector('[role="status"]') as HTMLElement

function render(over: Partial<DesktopSoundButtonProps> = {}) {
  const props: DesktopSoundButtonProps = { video, failed: false, phone: false, compact: false, onNotice: vi.fn(), onChange: vi.fn(), ...over }
  act(() => root.render(<DesktopSoundButton {...props} />))
  return props
}
const click = async () => {
  await act(async () => {
    btn().dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  video = document.createElement('video')
  video.muted = true
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.restoreAllMocks()
})

describe('DesktopSoundButton', () => {
  it('starts as Unmute with its name, label and tooltip, and reports off at mount', () => {
    const p = render()
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
    expect(btn().textContent).toContain('Unmute')
    expect(btn().title).toBe("Unmute: hear this Mac's sound")
    expect(btn().getAttribute('aria-pressed')).toBeNull()
    expect(p.onChange).toHaveBeenCalledWith(false)
  })

  it('fires on click only: a pointerdown does nothing', async () => {
    render()
    await act(async () => {
      btn().dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }))
      btn().dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    })
    expect(video.muted).toBe(true)
    expect(play).not.toHaveBeenCalled()
    await click()
    expect(video.muted).toBe(false)
    expect(play).toHaveBeenCalledTimes(1)
  })

  it('stays Unmute until play() resolves, ignores a second click meanwhile, then reads Mute', async () => {
    let resolve!: () => void
    play.mockReturnValue(new Promise<void>((r) => (resolve = r)))
    const p = render()
    await click()
    expect(video.muted).toBe(false)
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
    await click()
    expect(play).toHaveBeenCalledTimes(1)
    await act(async () => resolve())
    expect(btn().getAttribute('aria-label')).toBe('Mute')
    expect(btn().title).toBe('Mute')
    expect(p.onChange).toHaveBeenLastCalledWith(true)
  })

  it('a rejected play() mutes again, stays Unmute and reports play-rejected', async () => {
    play.mockRejectedValue(new DOMException('no gesture', 'NotAllowedError'))
    const p = render()
    await click()
    expect(video.muted).toBe(true)
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
    expect(p.onNotice).toHaveBeenCalledWith('play-rejected')
    expect(live().textContent).toBe('')
  })

  it('Mute sets muted and announces Sound off; the status line says Sound on after Unmute', async () => {
    render()
    expect(live().textContent).toBe('')
    await click()
    expect(live().textContent).toBe('Sound on')
    await click()
    expect(video.muted).toBe(true)
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
    expect(live().textContent).toBe('Sound off')
    expect(live().className).toBe('visually-hidden')
  })

  it('keeps one width: a hidden copy of the wider word sits in the label', async () => {
    render()
    const ghost = () => container.querySelector('.desktop-sound__ghost')
    expect(ghost()?.textContent).toBe('Unmute')
    expect(ghost()?.getAttribute('aria-hidden')).toBe('true')
    await click()
    expect(btn().textContent).toBe('MuteUnmute')
    expect(ghost()?.textContent).toBe('Unmute')
  })

  it('compact is icon only and keeps its accessible name and tooltip', () => {
    render({ compact: true })
    expect(btn().textContent).toBe('')
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
    expect(btn().title).toBe("Unmute: hear this Mac's sound")
    expect(btn().querySelector('svg')).not.toBeNull()
  })

  it('the icon changes shape with the state, not only the word', async () => {
    render()
    const before = btn().querySelector('svg')!.innerHTML
    await click()
    expect(btn().querySelector('svg')!.innerHTML).not.toBe(before)
  })

  it('failed greys it with aria-disabled, names the reason, and a click reports unavailable without playing', async () => {
    const p = render({ failed: true })
    expect(btn().getAttribute('aria-disabled')).toBe('true')
    expect(btn().hasAttribute('disabled')).toBe(false)
    expect(btn().title).toBe("Sound isn't available from this Mac right now.")
    const reason = document.getElementById(btn().getAttribute('aria-describedby')!)
    expect(reason?.textContent).toBe("Sound isn't available from this Mac right now.")
    await click()
    expect(p.onNotice).toHaveBeenCalledWith('unavailable')
    expect(play).not.toHaveBeenCalled()
    expect(video.muted).toBe(true)
  })

  it('the phone form carries the 44px class', () => {
    render({ phone: true })
    expect(btn().className).toContain('desktop-sound--phone')
  })

  it('follows the element: an unmuted element reads Mute, and something else muting it reads Unmute', async () => {
    video.muted = false
    const p = render()
    expect(btn().getAttribute('aria-label')).toBe('Mute')
    expect(p.onChange).toHaveBeenCalledWith(true)
    await act(async () => {
      video.muted = true
      video.dispatchEvent(new Event('volumechange'))
    })
    expect(btn().getAttribute('aria-label')).toBe('Unmute')
  })

  it('remembers nothing outside the element', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const cookie = document.cookie
    render()
    await click()
    await click()
    expect(setItem).not.toHaveBeenCalled()
    expect(document.cookie).toBe(cookie)
  })
})
