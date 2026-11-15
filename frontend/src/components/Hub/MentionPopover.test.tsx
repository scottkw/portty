/**
 * MentionPopover component tests — Phase 154-04, @session removed in Portty Phase 1 (CHAT-01)
 *
 * Covers:
 *   1. No "Agent" section label, no @session row, no divider; participants render
 *   2. With filter "al" — only alice renders
 *   3. activeIndex=0 highlights the first participant
 *   4. Nothing renders when no participant matches the filter
 *   5. Clicking alice calls onSelect("@alice")
 *   6. Escape keydown calls onClose
 *
 * Test infrastructure follows the ChatMessage.test.tsx pattern (createRoot + act).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { MentionPopover } from './MentionPopover'
import type { PresenceEntry } from '../../lib/relayClient'

// ── helpers ────────────────────────────────────────────────────────────────

function makeParticipant(alias: string, tailnetID?: string): PresenceEntry {
  return {
    personKey: `${tailnetID ?? `node:${alias}`}:web`,
    tailnetID: tailnetID ?? `node:${alias}`,
    origin: 'web',
    alias,
    connCount: 1,
  }
}

function renderPopover(props: {
  participants: PresenceEntry[]
  filter: string
  activeIndex: number
  onSelect: (alias: string) => void
  onClose: () => void
}): { container: HTMLElement; root: ReturnType<typeof createRoot> } {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => {
    root.render(<MentionPopover {...props} />)
  })
  return { container, root }
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('MentionPopover', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  afterEach(() => {
    act(() => { root?.unmount() })
    container?.remove()
  })

  it('renders only participants: no Agent label, no @session row, no divider', () => {
    ;({ container, root } = renderPopover({
      participants: [makeParticipant('alice'), makeParticipant('bob')],
      filter: '',
      activeIndex: 0,
      onSelect: vi.fn(),
      onClose: vi.fn(),
    }))

    const listbox = container.querySelector('[role="listbox"]')
    expect(listbox).not.toBeNull()
    expect(container.querySelector('.mention-popover__section-label')).toBeNull()
    expect(container.querySelector('.mention-popover__divider')).toBeNull()
    expect(listbox?.textContent).not.toContain('Agent')
    expect(listbox?.textContent).not.toContain('@session')

    const options = container.querySelectorAll('[role="option"]')
    expect(options.length).toBe(2)
    expect(options[0].textContent).toContain('alice')
    expect(options[1].textContent).toContain('bob')
  })

  it('filters participants by alias when filter="al"', () => {
    ;({ container, root } = renderPopover({
      participants: [makeParticipant('alice'), makeParticipant('bob')],
      filter: 'al',
      activeIndex: 0,
      onSelect: vi.fn(),
      onClose: vi.fn(),
    }))

    const options = container.querySelectorAll('[role="option"]')
    expect(options.length).toBe(1)
    expect(options[0].textContent).toContain('alice')
  })

  it('activeIndex=0 highlights the first participant', () => {
    ;({ container, root } = renderPopover({
      participants: [makeParticipant('alice'), makeParticipant('bob')],
      filter: '',
      activeIndex: 0,
      onSelect: vi.fn(),
      onClose: vi.fn(),
    }))

    const options = container.querySelectorAll('[role="option"]')
    expect(options[0].classList.contains('mention-popover__item--active')).toBe(true)
    expect(options[0].getAttribute('aria-selected')).toBe('true')
    expect(options[1].classList.contains('mention-popover__item--active')).toBe(false)
  })

  it('renders nothing when no participant matches the filter', () => {
    ;({ container, root } = renderPopover({
      participants: [makeParticipant('alice')],
      filter: 'zzz',
      activeIndex: 0,
      onSelect: vi.fn(),
      onClose: vi.fn(),
    }))

    expect(container.querySelector('.mention-popover')).toBeNull()
    expect(container.innerHTML).toBe('')
  })

  it('calls onSelect("@alice") when alice row is clicked', () => {
    const onSelect = vi.fn()
    ;({ container, root } = renderPopover({
      participants: [makeParticipant('alice')],
      filter: '',
      activeIndex: 0,
      onSelect,
      onClose: vi.fn(),
    }))

    const aliceOption = container.querySelector('[role="option"]')
    act(() => {
      aliceOption?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(onSelect).toHaveBeenCalledWith('@alice')
  })

  it('calls onClose when Escape is pressed (global keydown listener)', () => {
    const onSelect = vi.fn()
    const onClose = vi.fn()
    ;({ container, root } = renderPopover({
      participants: [],
      filter: '',
      activeIndex: 0,
      onSelect,
      onClose,
    }))

    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(onClose).toHaveBeenCalled()
  })
})
