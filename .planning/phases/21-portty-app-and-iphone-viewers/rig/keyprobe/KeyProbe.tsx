// THROWAWAY Phase 21-02 key-routing probe (scratch copy only, never the shipped tree).
// A full-window layer holding a focused, visually hidden textarea and a plain focusable div.
// Every key, clipboard, selection, visibility, blur and menu event is logged through the
// ProbeLog binding (~/p21/keyprobe.log in the VM). The textarea and the div handlers call
// preventDefault and stopPropagation, the same focus target the Phase 20 stage uses.
import { useEffect, useRef } from 'react'
import { EventsOn } from './wailsjs/wailsjs/runtime/runtime'
import { ProbeLog, ProbeMode, ProbeBlankMenuKeys, ProbeRebuildMenu } from './wailsjs/go/main/App'

const MENU_EVENTS = ['menu:close-tab', 'menu:new-session', 'menu:settings', 'menu:toggle-sidebar', 'app:quit-requested']

function flags(e: KeyboardEvent): string {
  return `meta=${+e.metaKey} ctrl=${+e.ctrlKey} alt=${+e.altKey} shift=${+e.shiftKey}`
}

export function KeyProbe() {
  const ta = useRef<HTMLTextAreaElement>(null)
  const div = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const log = (s: string) => { void ProbeLog(s) }
    const tgt = (e: Event) => {
      const t = e.target
      return t === ta.current ? 'ta' : t === div.current ? 'div' : t === document.body ? 'body' : 'other'
    }
    const key = (down: boolean) => (e: KeyboardEvent) =>
      log(`${down ? 'KEYDOWN' : 'KEYUP'} ${e.code} key=${JSON.stringify(e.key)} ${flags(e)} rep=${+e.repeat} tgt=${tgt(e)}`)
    const onDown = key(true)
    const onUp = key(false)
    const dom = (name: string) => (e: Event) => log(`${name} tgt=${tgt(e)}`)
    const onCopy = dom('COPY'), onCut = dom('CUT'), onPaste = dom('PASTE'), onSel = dom('SELECTSTART')
    const onBefore = (e: Event) => log(`BEFOREINPUT ${(e as InputEvent).inputType} tgt=${tgt(e)}`)
    const onVis = () => log(`VIS ${document.visibilityState}`)
    const onBlur = () => log('BLUR')
    const onFocus = () => log('FOCUS')
    document.addEventListener('keydown', onDown, true)
    document.addEventListener('keyup', onUp, true)
    document.addEventListener('copy', onCopy, true)
    document.addEventListener('cut', onCut, true)
    document.addEventListener('paste', onPaste, true)
    document.addEventListener('selectstart', onSel, true)
    document.addEventListener('beforeinput', onBefore, true)
    document.addEventListener('visibilitychange', onVis)
    window.addEventListener('blur', onBlur)
    window.addEventListener('focus', onFocus)
    const offs = MENU_EVENTS.map((n) => EventsOn(n, () => log(`MENU ${n}`)))

    let mode = ''
    let epoch = ''
    const applyFocus = (target: string) => {
      if (target === 'div') div.current?.focus()
      else if (target === 'none') (document.activeElement as HTMLElement | null)?.blur()
      else ta.current?.focus()
    }
    const poll = async () => {
      const [m, f, ep] = (await ProbeMode()).split(' ')
      if (m !== mode) {
        const prev = mode
        mode = m
        // The first poll in page mode touches nothing: the real menu is untouched.
        if (m === 'blank') await ProbeBlankMenuKeys(true)
        else if (m === 'rebuild') { await ProbeBlankMenuKeys(false); await ProbeRebuildMenu(true) }
        else if (prev === 'blank') await ProbeBlankMenuKeys(false)
        else if (prev === 'rebuild') await ProbeRebuildMenu(false)
        log(`PROBE mode=${m}`)
      }
      if (ep !== epoch) {
        epoch = ep
        applyFocus(f)
        log(`PROBE focus=${f} active=${document.activeElement === ta.current ? 'ta' : document.activeElement === div.current ? 'div' : 'other'}`)
      }
    }
    // Restore the real menu before the page goes away.
    const onUnload = () => { void ProbeBlankMenuKeys(false); void ProbeRebuildMenu(false) }
    window.addEventListener('beforeunload', onUnload)
    const timer = window.setInterval(() => { void poll() }, 300)
    log(`PROBE ready vis=${document.visibilityState}`)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('beforeunload', onUnload)
      document.removeEventListener('keydown', onDown, true)
      document.removeEventListener('keyup', onUp, true)
      document.removeEventListener('copy', onCopy, true)
      document.removeEventListener('cut', onCut, true)
      document.removeEventListener('paste', onPaste, true)
      document.removeEventListener('selectstart', onSel, true)
      document.removeEventListener('beforeinput', onBefore, true)
      document.removeEventListener('visibilitychange', onVis)
      window.removeEventListener('blur', onBlur)
      window.removeEventListener('focus', onFocus)
      offs.forEach((off) => off())
    }
  }, [])

  const swallow = (e: React.KeyboardEvent) => { e.preventDefault(); e.stopPropagation() }
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 2147483000, pointerEvents: 'none' }}>
      <textarea
        ref={ta}
        aria-label="keyprobe stage"
        autoFocus
        defaultValue="probe-text"
        onKeyDown={swallow}
        onKeyUp={swallow}
        style={{ position: 'absolute', width: 1, height: 1, margin: -1, padding: 0, border: 0, overflow: 'hidden', clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', pointerEvents: 'auto' }}
      />
      <div
        ref={div}
        tabIndex={0}
        onKeyDown={swallow}
        onKeyUp={swallow}
        style={{ position: 'absolute', right: 8, bottom: 8, padding: '2px 6px', font: '11px monospace', background: '#222', color: '#fff', pointerEvents: 'auto' }}
      >
        keyprobe
      </div>
    </div>
  )
}
