// Portty 04-06 (SHAR-03): the in-app dead-link block shows only its fixed copy.
import { describe, it, expect, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { LinkNoLongerWorks } from '../LinkNoLongerWorks'

const HEADING = 'Link No Longer Works'
const BODY = 'This link was turned off or is incomplete. Copy a fresh link from the Share button on the Mac that runs the session.'

let root: Root | undefined
let container: HTMLDivElement | undefined

afterEach(() => {
  if (root) act(() => root!.unmount())
  container?.remove()
  root = undefined
  container = undefined
})

describe('LinkNoLongerWorks', () => {
  it('renders the heading and body copy and nothing else', () => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    act(() => root!.render(React.createElement(LinkNoLongerWorks)))
    expect(container.querySelector('h1')?.textContent).toBe(HEADING)
    expect(container.querySelector('p')?.textContent).toBe(BODY)
    expect(container.textContent).toBe(HEADING + BODY)
  })
})
