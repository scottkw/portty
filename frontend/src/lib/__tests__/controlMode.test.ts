// Portty 21-09 (DVWR-04) — the control-mode state machine: every event from on and from off.

import { describe, it, expect } from 'vitest'
import { controlReducer, isPasteChord, pasteDecision, PASTE_NOTICES, type ControlEvent } from '../controlMode'

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

describe('isPasteChord', () => {
  const key = (init: Partial<KeyboardEventInit>) => new KeyboardEvent('keydown', { code: 'KeyV', ...init })
  it('is Cmd+V on the physical key and nothing else', () => {
    expect(isPasteChord(key({ metaKey: true }))).toBe(true)
    expect(isPasteChord(key({ metaKey: true, shiftKey: true }))).toBe(false)
    expect(isPasteChord(key({ metaKey: true, ctrlKey: true }))).toBe(false)
    expect(isPasteChord(key({ metaKey: true, altKey: true }))).toBe(false)
    expect(isPasteChord(key({}))).toBe(false)
    expect(isPasteChord(key({ metaKey: true, code: 'KeyB' }))).toBe(false)
  })
})

describe('pasteDecision (UI-SPEC Cmd+V table)', () => {
  it('plain text different from the last send is sent, then the key', () => {
    expect(pasteDecision({ outcome: 'ok', text: 'new' }, null)).toEqual({ send: 'new', key: true, notice: null })
    expect(pasteDecision({ outcome: 'ok', text: 'new' }, 'old')).toEqual({ send: 'new', key: true, notice: null })
  })

  it('the same text as the last send, or no text, sends only the key', () => {
    expect(pasteDecision({ outcome: 'ok', text: 'same' }, 'same')).toEqual({ send: null, key: true, notice: null })
    expect(pasteDecision({ outcome: 'empty' }, 'old')).toEqual({ send: null, key: true, notice: null })
    expect(pasteDecision({ outcome: 'ok', text: '' }, null)).toEqual({ send: null, key: true, notice: null })
  })

  it('a concealed item, text over 1 MB and a failed read send nothing and no key, with the matching notice', () => {
    expect(pasteDecision({ outcome: 'concealed' }, null)).toEqual({
      send: null, key: false, notice: 'Your clipboard holds a hidden item (password manager); not sent',
    })
    expect(pasteDecision({ outcome: 'too_large' }, null)).toEqual({
      send: null, key: false, notice: 'Your clipboard text is too large to send (limit 1 MB)',
    })
    expect(pasteDecision({ outcome: 'failed' }, null)).toEqual({
      send: null, key: false, notice: "Couldn't send to the Mac. Try again.",
    })
  })

  it('a concealed item never carries text through, even if the binding sent some', () => {
    expect(pasteDecision({ outcome: 'concealed', text: 'secret' }, null).send).toBeNull()
  })

  it('an outcome it does not know fails closed', () => {
    expect(pasteDecision({ outcome: 'surprise' as never, text: 'x' }, null)).toEqual({
      send: null, key: false, notice: PASTE_NOTICES.failed,
    })
  })
})
