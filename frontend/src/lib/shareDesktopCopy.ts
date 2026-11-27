// Phase 22: every UI string of 22-UI-SPEC's Copywriting Contract and Surface 6, word for word,
// sharer side and guest side. No component spells one of these anywhere else. Names, logins and
// devices are interpolated by the caller as text; a counted form is a function (Phase 19 singular
// and plural rule).

// ---- Entry points (Surface 1) ----
export const SHARE_DESKTOP_ACTION = 'Share Desktop…'
export const MANAGE_SHARING_ACTION = 'Manage Sharing…'
export const SETTINGS_ROW_TITLE = 'Share Desktop'
export const SETTINGS_ROW_DESCRIPTION =
  'Show your screen and sound to someone on your tailnet. They wait for you to let them in, and can only watch until you allow more.'
export const QUIT_WARNING = 'Quitting also stops sharing your desktop and disconnects everyone watching.'
export const SECURITY_KEYS_DESCRIPTION =
  'Regenerating signs out every browser and Mac attached to your sessions, stops every saved desktop link, and ends any desktop sharing.'

// ---- Share Desktop sheet (Surface 2) ----
export const SHARE_SHEET_TITLE = 'Share Desktop'
export const SHARE_NOT_SHARING_DESCRIPTION =
  'Show your screen and sound to someone on your tailnet. They wait at a prompt on this Mac until you let them in, and they can only watch until you allow more.'
export const SHARE_CT_LINE = (host: string): string =>
  `The first time, this gets an HTTPS certificate for ${host}.ts.net. That name is published in public Certificate Transparency logs.`
export const SHARE_STARTING = 'Getting a link…'
export const SHARE_LINK_LABEL = 'Link'
export const SHARE_LINK_DESCRIPTION =
  'Anyone on your tailnet who opens it waits for you to let them in. It stops working when you stop sharing.'
export const COPY_LINK = 'Copy Link'
export const COPIED = 'Copied!'
export const COPY_LINK_FAILED = "Couldn't copy the link. Try again."
export const WATCHING_HEADER = 'Watching'
export const WATCHING_EMPTY = 'Nobody is watching yet. Send the link to someone on your tailnet.'
export const BADGE_VIEW_ONLY = 'View only'
export const BADGE_CONTROLLING = 'Controlling'
export const BADGE_OWN_ACCOUNT = 'Own account'
export const ALLOW_CONTROL = 'Allow control'
export const ALLOW_CLIPBOARD = 'Allow clipboard'
export const DISCONNECT = 'Disconnect'
export const OWN_DEVICE_LINE = 'Your own device'
export const UNNAMED_VIEWER = 'Unknown viewer'
export const grantCaption = (name: string): string =>
  `Control lets ${name} use this Mac as you, so they can do anything you can. Clipboard lets them copy text to and from this Mac.`
export const SHARE_FOOTNOTE =
  "Denying or disconnecting someone is final for this share. To let them back in, stop sharing and share again. Anyone watching sees everything on this screen, including this window, and hears this Mac's sound, even before they unmute. If someone can't open the link, your tailnet's access rules may not let them reach this Mac."
export const STOP_SHARING = 'Stop Sharing'
export const DONE = 'Done'
export const NOT_NOW = 'Not Now'
export const START_SHARING = 'Start Sharing'
export const SHARE_STOPPED_NOTICE = 'Sharing stopped. Choose Start Sharing to get a new link.'

export const shareDesktopStartError = (detail: string): string =>
  `Couldn't share the desktop: ${detail}. Check that Tailscale is connected, then try again.`
export const SHARE_HTTPS_OFF_ERROR =
  "Couldn't share the desktop: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again."
export const shareDesktopStopError = (detail: string): string => `Couldn't stop sharing: ${detail}. Try again.`
export const GRANT_CHANGE_ERROR = "Couldn't change what this person can do. Try again."
export const DISCONNECT_ERROR = "Couldn't disconnect this person. Try again, or choose Stop Sharing."
export const ADMIT_ANSWER_ERROR = "Couldn't send your answer. Try again."

// Status row (Phase 19 words, verbatim).
export const STATUS_NEEDS_PERMISSION = 'Needs permission'
export const STATUS_AWAITING_APPROVAL = 'Waiting for approval at the Mac'
export const STATUS_DISPLAY_ASLEEP = 'Display asleep'
export const STATUS_CAPTURE_PAUSED = 'Paused after repeated errors. Run Test Capture to resume.'
export const STATUS_WAITING_FOR_TAILSCALE = 'Waiting for Tailscale'
export const SET_UP_PERMISSIONS = 'Set Up Permissions…'
export const SHARE_STATUS_LABEL = 'Status'

// ---- Admit sheet (Surface 3) ----
export const ADMIT_TITLE = 'Someone Wants to Watch Your Desktop'
export const ADMIT_LOGIN_LABEL = 'Login name'
export const ADMIT_DEVICE_LABEL = 'Device'
export const ADMIT_PARAGRAPH_1 =
  "They will see your whole screen and hear this Mac's sound, and can only watch until you allow control. Deny is final for this share."
export const ADMIT_PARAGRAPH_2 = "Check the login name. Anyone can set a display name that looks like someone else's."
/** `more` is how many others are waiting besides the one shown; the line is for 1 or more. */
export const morePeopleWaiting = (more: number): string =>
  more === 1 ? '1 more person is waiting.' : `${more} more people are waiting.`
export const DENY = 'Deny'
export const ADMIT = 'Admit'

// ---- Tray (Surface 4) ----
export const sharedWithPeople = (n: number): string =>
  n === 0 ? 'Shared with nobody yet' : n === 1 ? 'Shared with 1 person' : `Shared with ${n} people`
export const traySharedLine = (n: number): string =>
  n === 0 ? 'Desktop shared with nobody yet' : n === 1 ? 'Desktop shared with 1 person' : `Desktop shared with ${n} people`

// ---- Notifications and banners (Surface 5) ----
export const NOTIFICATION_TITLE = 'Portty'
export const notifyAdmitRequest = (login: string): string => `${login} wants to see your desktop.`
export const notifyJoined = (who: string): string => `${who} joined your desktop.`
export const notifyLeft = (who: string): string => `${who} left your desktop.`
export const viewersJoined = (n: number): string => `${n} viewers joined your desktop.`
export const viewersLeft = (n: number): string => `${n} viewers left your desktop.`
export const liveJoined = (name: string): string => `${name} joined`
export const liveLeft = (name: string): string => `${name} left`
export const BANNER_SECURE_INPUT =
  "Typing at this Mac isn't detected right now, because a password field or Secure Keyboard Entry is on. Someone you've allowed to control may type at the same time as you."
export const BANNER_SHARE_STOPPED = 'Desktop sharing stopped because Portty restarted. Share Desktop again to get a new link.'

// ---- Guest page (Surface 6) ----
export const guestWaitingHeading = (mac: string): string => `Waiting for ${mac} to let you in`
export const GUEST_CARD_WAITING_BODY =
  'The person at that Mac sees your name and chooses whether to let you in. This page keeps waiting for up to 5 minutes.'
export const GUEST_CARD_NOT_ADMITTED = 'Not admitted'
export const GUEST_CARD_DENIED_BODY = "The person at that Mac didn't let you in. You can't ask again while this share is on."
export const GUEST_CARD_TIMEOUT_BODY = 'Nobody let you in within 5 minutes. Reload this page to ask again.'
export const GUEST_CARD_REVOKED = 'You were disconnected'
export const GUEST_CARD_REVOKED_BODY = "The person at that Mac ended your connection. You can't rejoin while this share is on."
export const GUEST_CARD_ENDED = 'Sharing has ended'
export const GUEST_CARD_ENDED_BODY =
  'The share has ended, so this link no longer works. Ask the person at that Mac for a new link.'
export const GUEST_CARD_LINK = 'This link no longer works'
export const GUEST_CARD_LINK_BODY = 'The share has ended, or the link is incomplete. Ask the person who shared it for a new link.'
export const GUEST_CARD_CONNECTING = 'Connecting…'
export const guestConnectingBody = (mac: string): string =>
  `Waiting for the first picture from ${mac}. If your browser asks to find devices nearby, choose Allow.`
export const GUEST_CARD_UNREACHABLE = "Can't reach this Mac"
export const GUEST_CARD_UNREACHABLE_BODY =
  "Check that it's awake and on your tailnet. Your tailnet's access rules may also be blocking the connection. This page keeps trying."
export const RELOAD_PAGE = 'Reload Page'

export const GUEST_CHIP_VIEW_ONLY = 'View only. Ask the person at the Mac to allow control.'
export const GUEST_CHIP_CONTROL = 'You have control'
/** `name` is the lease holder's sanitized display name, the only name a guest page renders. */
export const controllingChip = (name: string): string => `${name} is controlling`
export const GUEST_CHIP_SOMEONE = 'Someone is using this Mac'
// 23-UI-SPEC Surface 3: a guest cannot unlock, so the chip names who can. DesktopView renders the same words.
export const GUEST_CHIP_LOCKED = 'Locked. Ask the person at the Mac to unlock it.'
export const GUEST_CHIP_RECONNECTING = 'Reconnecting…'

export const GUEST_NOTICE_CLIPBOARD_ON = 'You can now copy text to and from this Mac'
export const GUEST_NOTICE_CLIPBOARD_OFF = 'Clipboard was turned off'

// ---- Static pages (Surface 7) ----
export const GUEST_PAGE_LINK_HEADING = 'This link no longer works'
export const GUEST_PAGE_REFUSED_HEADING = "This device can't open this link"
export const GUEST_PAGE_REFUSED_BODY =
  "Open it on a device you sign in to Tailscale with, on the same tailnet as the Mac that shared it. Tagged devices and devices shared in from another tailnet can't use it."
