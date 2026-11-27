// Portty 21-09 (DVWR-04) — the control-mode state machine: every event from on and from off.

import { describe, it, expect } from 'vitest'
import { controlReducer, type ControlEvent } from '../controlMode'

describe('controlReducer', () => {
  it('capture-on turns control mode on and asks for the menu switch only when native', () => {
    expect(controlReducer(false, 'capture-on', false)).toEqual({ controlling: true, actions: [] })
    expect(controlReducer(false, 'capture-on', true)).toEqual({ controlling: true, actions: ['menuOn'] })
    expect(controlReducer(true, 'capture-on', false)).toEqual({ controlling: true, actions: [] })
  })

  it('capture-off from on releases everything and turns off', () => {
    expect(controlReducer(true, 'capture-off', false)).toEqual({ controlling: false, actions: ['releaseAll'] })
    expect(controlReducer(true, 'capture-off', true)).toEqual({ controlling: false, actions: ['releaseAll', 'menuOff'] })
  })

  it.each(['tab-inactive', 'window-blur', 'not-connected'] as ControlEvent[])(
    '%s from on releases everything first, then takes the keyboard off the stage',
    (ev) => {
      expect(controlReducer(true, ev, false)).toEqual({ controlling: false, actions: ['releaseAll', 'releaseKeyboard'] })
      expect(controlReducer(true, ev, true)).toEqual({
        controlling: false,
        actions: ['releaseAll', 'releaseKeyboard', 'menuOff'],
      })
    },
  )

  it('tab-close from on releases everything', () => {
    expect(controlReducer(true, 'tab-close', false)).toEqual({ controlling: false, actions: ['releaseAll'] })
    expect(controlReducer(true, 'tab-close', true)).toEqual({ controlling: false, actions: ['releaseAll', 'menuOff'] })
  })

  it.each(['capture-off', 'tab-inactive', 'window-blur', 'not-connected'] as ControlEvent[])(
    '%s while already off does nothing',
    (ev) => {
      expect(controlReducer(false, ev, false)).toEqual({ controlling: false, actions: [] })
      expect(controlReducer(false, ev, true)).toEqual({ controlling: false, actions: [] })
    },
  )

  it('tab-close while off still releases, so a stale state never strands a key', () => {
    expect(controlReducer(false, 'tab-close', false)).toEqual({ controlling: false, actions: ['releaseAll'] })
    expect(controlReducer(false, 'tab-close', true)).toEqual({ controlling: false, actions: ['releaseAll', 'menuOff'] })
  })
})
