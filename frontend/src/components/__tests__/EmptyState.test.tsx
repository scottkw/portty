import { describe, it, expect, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { EmptyState } from '../EmptyState'

describe('EmptyState (LOOK-01 empty edge)', () => {
  it('shows the UI-SPEC copy and the ⌘T hint; New Session calls onNewSession', () => {
    const onNewSession = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    flushSync(() => root.render(React.createElement(EmptyState, { onNewSession })))

    expect(container.querySelector('h2')?.textContent).toBe('No Open Sessions')
    expect(container.textContent).toContain('Start a new shell, or open a session from the sidebar.')
    expect(container.textContent).toContain('⌘T')
    const btn = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'New Session')
    expect(btn).toBeDefined()
    flushSync(() => btn!.click())
    expect(onNewSession).toHaveBeenCalledTimes(1)

    flushSync(() => root.unmount())
    container.remove()
  })
})
