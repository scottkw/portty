import React, { useEffect, useRef, useState } from 'react'
import { CursorArrowRaysIcon, EyeIcon, UserIcon } from '@heroicons/react/24/outline'
import { DisconnectDesktopViewer, SetDesktopViewerRights, type DesktopShareActionResult } from '../wailsjs/go/main/App'
import { sanitizeName, type ShareViewer } from '../lib/desktopShare'
import * as copy from '../lib/shareDesktopCopy'
import { Switch } from './Switch'

interface ViewerListProps {
  viewers: ShareViewer[]
}

// The grant caption is one sentence in the copy file with the name inside it; split it
// around a placeholder so the name can sit in its own <bdi>.
const NAME_SLOT = '\u0000'
const [CAPTION_BEFORE, CAPTION_AFTER] = copy.grantCaption(NAME_SLOT).split(NAME_SLOT)

interface Person {
  v: ShareViewer
  own: boolean
  /** The bold line: the display name, or the login name when that is empty or the same. */
  name: string
  /** The separate login line; null when the bold line already is the login name, and for own devices. */
  login: string | null
  device: string
}

function person(v: ShareViewer): Person {
  const own = v.mode === 'own'
  const display = sanitizeName(v.displayName)
  const login = sanitizeName(v.loginName)
  const device = sanitizeName(v.nodeName)
  // An own-account device is named by its device; the person is the sharer.
  const name = own ? device || display || login : display && display !== login ? display : login || device
  return { v, own, name: name || copy.UNNAMED_VIEWER, login: own || !login || login === name ? null : login, device }
}

// Oldest connection first, then id, so a row never jumps when someone joins.
const byAge = (a: ShareViewer, b: ShareViewer): number =>
  (Date.parse(a.since) || 0) - (Date.parse(b.since) || 0) || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)

/**
 * The Watching list (22-UI-SPEC Surface 2): who is watching, with a grant switch pair and
 * Disconnect per guest. Every action acts at once with no confirm; the daemon's next state
 * line is the truth, so a switch only holds a requested value while the daemon still reports
 * the one it had when the call was made. Names are text in <bdi>, never markup.
 */
export function ViewerList({ viewers }: ViewerListProps): React.ReactElement {
  const people = [...viewers].sort(byAge).map(person)
  const [busy, setBusy] = useState<Record<string, boolean>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [requested, setRequested] = useState<Record<string, { from: [boolean, boolean]; to: [boolean, boolean] }>>({})
  const [announcement, setAnnouncement] = useState('')
  const alive = useRef(true)
  const seen = useRef<Map<string, string> | null>(null)

  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  // The hidden status line says who came and went, so a screen reader hears it with banners silenced.
  // The first render announces nothing: the list as it was opened is not news.
  useEffect(() => {
    const now = new Map(people.map((p) => [p.v.id, p.name]))
    const before = seen.current
    seen.current = now
    if (!before) return
    const said = [
      ...[...now].filter(([id]) => !before.has(id)).map(([, n]) => copy.liveJoined(n)),
      ...[...before].filter(([id]) => !now.has(id)).map(([, n]) => copy.liveLeft(n)),
    ]
    if (said.length > 0) setAnnouncement(said.join('. '))
    // Rows that left take their errors with them.
    setErrors((e) => (Object.keys(e).every((id) => now.has(id)) ? e : Object.fromEntries(Object.entries(e).filter(([id]) => now.has(id)))))
    // eslint-disable-next-line react-hooks/exhaustive-deps -- keyed on the viewers prop only
  }, [viewers])

  async function run(id: string, call: () => Promise<DesktopShareActionResult>, failure: string, onDone?: () => void): Promise<void> {
    if (busy[id]) return
    setBusy((b) => ({ ...b, [id]: true }))
    setErrors(({ [id]: _gone, ...rest }) => rest)
    const r = await call().catch((): DesktopShareActionResult => ({ errorCode: 'unavailable', detail: '' }))
    if (!alive.current) return
    setBusy(({ [id]: _done, ...rest }) => rest)
    // no_viewer: they already left, and the next state line removes the row.
    if (r?.errorCode && r.errorCode !== 'no_viewer') setErrors((e) => ({ ...e, [id]: failure }))
    else onDone?.()
  }

  function setGrant(p: Person, control: boolean, clipboard: boolean): void {
    const from: [boolean, boolean] = [p.v.control, p.v.clipboard]
    void run(p.v.id, () => SetDesktopViewerRights(p.v.id, control, clipboard), copy.GRANT_CHANGE_ERROR,
      () => setRequested((r) => ({ ...r, [p.v.id]: { from, to: [control, clipboard] } })))
  }

  return (
    <div className="share-desktop__watching">
      <div className="share-desktop__watching-header">{copy.WATCHING_HEADER}</div>
      <div className="settings-group" aria-live="polite">
        {people.length === 0 && (
          <div className="settings-row"><p className="settings-row__description">{copy.WATCHING_EMPTY}</p></div>
        )}
        {people.map((p) => {
          const { v } = p
          const req = requested[v.id]
          const held = req && req.from[0] === v.control && req.from[1] === v.clipboard ? req.to : null
          const control = held ? held[0] : v.control
          const clipboard = held ? held[1] : v.clipboard
          const inflight = !!busy[v.id]
          const nameId = `viewer-${v.id}-name`
          const captionId = `viewer-${v.id}-caption`
          const Badge = p.own ? UserIcon : control ? CursorArrowRaysIcon : EyeIcon
          return (
            <div key={v.id} className="settings-row viewer-row" role="group" aria-labelledby={nameId}>
              <div className="viewer-row__head">
                <span className="viewer-row__name" id={nameId}><bdi>{p.name}</bdi></span>
                <span className="viewer-badge">
                  <Badge className="viewer-badge__icon" aria-hidden="true" />
                  {p.own ? copy.BADGE_OWN_ACCOUNT : control ? copy.BADGE_CONTROLLING : copy.BADGE_VIEW_ONLY}
                </span>
              </div>
              <p className="viewer-row__identity">
                {p.own ? copy.OWN_DEVICE_LINE : (
                  <>
                    {p.login && <bdi className="viewer-row__login">{p.login}</bdi>}
                    {p.login && p.device && ' · '}
                    {p.device && <bdi className="viewer-row__device" title={p.device}>{p.device}</bdi>}
                  </>
                )}
              </p>
              <div className="viewer-row__controls">
                {!p.own && (
                  <>
                    <span className="viewer-row__switch">
                      <span id={`viewer-${v.id}-control-label`}>{copy.ALLOW_CONTROL}</span>
                      <Switch
                        checked={control}
                        saving={inflight}
                        aria-labelledby={`viewer-${v.id}-control-label ${nameId}`}
                        describedBy={captionId}
                        onChange={() => setGrant(p, !control, clipboard)}
                      />
                    </span>
                    <span className="viewer-row__switch">
                      <span id={`viewer-${v.id}-clipboard-label`}>{copy.ALLOW_CLIPBOARD}</span>
                      <Switch
                        checked={clipboard}
                        saving={inflight}
                        aria-labelledby={`viewer-${v.id}-clipboard-label ${nameId}`}
                        describedBy={captionId}
                        onChange={() => setGrant(p, control, !clipboard)}
                      />
                    </span>
                  </>
                )}
                <button
                  type="button"
                  className="button button--secondary viewer-row__disconnect"
                  aria-label={`${copy.DISCONNECT} ${p.name}`}
                  aria-disabled={inflight}
                  onClick={() => { if (!inflight) void run(v.id, () => DisconnectDesktopViewer(v.id), copy.DISCONNECT_ERROR) }}
                >
                  {copy.DISCONNECT}
                </button>
              </div>
              {!p.own && (
                <p className="viewer-row__caption" id={captionId}>
                  {CAPTION_BEFORE}<bdi>{p.name}</bdi>{CAPTION_AFTER}
                </p>
              )}
              {errors[v.id] && <p className="viewer-row__error" role="alert">{errors[v.id]}</p>}
            </div>
          )
        })}
      </div>
      <span className="visually-hidden" role="status">{announcement}</span>
    </div>
  )
}
