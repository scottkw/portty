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
