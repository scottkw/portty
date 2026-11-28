// What the viewer page shows about unlock: whether the Unlock This Mac panel is in the DOM, its status line,
// the Locked chip and the blocked-input notice texts, and how many password inputs the page holds at all.
// Reads the page only; the password field's value is never read.
const t = document.body.innerText.replace(/\s+/g, ' ')
const g = [...document.querySelectorAll('[role=group]')].find((e) => /Unlock This Mac/.test(e.innerText || ''))
const st = g ? g.querySelector('.desktop-unlock__status') : null
const btn = g ? [...g.querySelectorAll('button')].find((b) => /^Unlock$/.test(b.innerText.trim())) : null
return JSON.stringify({
  panel: !!g,
  status: st ? st.innerText.replace(/\s+/g, ' ') : null,
  button_aria_disabled: btn ? btn.getAttribute('aria-disabled') : null,
  password_inputs: document.querySelectorAll('input[type=password]').length,
  labelled_mac_password: document.querySelectorAll('[aria-label="Mac password"]').length,
  locked_chip: (t.match(/Locked[^.]*\.?/) || [''])[0].slice(0, 80),
  ask_chip: /Ask the person at the Mac to unlock it/.test(t),
  blocked_notice: (t.match(/Clicks and typing are off[^.]*\./) || [''])[0].slice(0, 120),
  mac_unlocked_notice: (t.match(/Mac unlocked[^.]*\.?/) || [''])[0].slice(0, 80),
  status_page: (window.__desktopStats || {}).status || null,
})
