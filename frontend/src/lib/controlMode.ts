// Portty 21-09 (DVWR-04): control mode in the Desktop tab. Control mode is Phase 20's "keyboard captured"
// state; this file decides what each way in and out of it does, so every exit lets go of the Mac first.

export type ControlEvent = 'capture-on' | 'capture-off' | 'tab-inactive' | 'window-blur' | 'not-connected' | 'tab-close'
export type ControlAction = 'releaseAll' | 'releaseKeyboard' | 'menuOn' | 'menuOff'

/** The user's keyboard going away without a click on the tab: it must be taken off the stage as well. */
const TAKES_KEYBOARD: ReadonlySet<ControlEvent> = new Set(['tab-inactive', 'window-blur', 'not-connected'])

/**
 * One event in, the next state and the actions to carry out in order. Release-all is always first, so no key or
 * button stays held on the unattended Mac. `native` is true only when a native menu switch ships (21-02's
 * KEYROUTE blank or rebuild); with page-level handling it is false and no menu action is ever returned.
 * Only tab-close acts while control mode is already off: closing must release whatever a stale state missed.
 */
export function controlReducer(
  controlling: boolean,
  ev: ControlEvent,
  native: boolean,
): { controlling: boolean; actions: ControlAction[] } {
  if (ev === 'capture-on') return { controlling: true, actions: native ? ['menuOn'] : [] }
  if (!controlling && ev !== 'tab-close') return { controlling: false, actions: [] }
  const actions: ControlAction[] = ['releaseAll']
  if (TAKES_KEYBOARD.has(ev)) actions.push('releaseKeyboard')
  if (native) actions.push('menuOff')
  return { controlling: false, actions }
}

/** What the Mac's own clipboard held when a Cmd+V was typed (the ReadHomeClipboard binding's answer). */
export type HomeClipboard = { outcome: 'ok' | 'concealed' | 'empty' | 'too_large' | 'failed'; text?: string }

/** The words for a Cmd+V that sends nothing (UI-SPEC Copywriting Contract; the last two are Phase 20's strings). */
export const PASTE_NOTICES = {
  concealed: 'Your clipboard holds a hidden item (password manager); not sent',
  too_large: 'Your clipboard text is too large to send (limit 1 MB)',
  failed: "Couldn't send to the Mac. Try again.",
} as const

/** Cmd+V on the physical key with no other modifier; Cmd+Shift+V and the like stay plain keys. */
export function isPasteChord(e: Pick<KeyboardEvent, 'code' | 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey'>): boolean {
  return e.code === 'KeyV' && e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey
}

/**
 * What a captured Cmd+V does, from the home clipboard and the text last sent in this tab: the text to send
 * (null for none), whether the key goes out, and the notice. Text the user already pasted is not sent again,
 * so copying on the Mac and pasting there keeps working. A concealed item, oversize text, a failed read or an
 * outcome this does not know sends nothing and no key (fail closed).
 */
export function pasteDecision(
  read: HomeClipboard,
  lastSent: string | null,
): { send: string | null; key: boolean; notice: string | null } {
  switch (read.outcome) {
    case 'ok':
      return { send: read.text && read.text !== lastSent ? read.text : null, key: true, notice: null }
    case 'empty':
      return { send: null, key: true, notice: null }
    case 'concealed':
      return { send: null, key: false, notice: PASTE_NOTICES.concealed }
    case 'too_large':
      return { send: null, key: false, notice: PASTE_NOTICES.too_large }
    default:
      return { send: null, key: false, notice: PASTE_NOTICES.failed }
  }
}
