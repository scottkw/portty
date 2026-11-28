---
status: resolved
trigger: "VM row P-01 FAIL: after submitting the Mac password in the desktop viewer's Unlock panel, Chrome shows a 'Save password?' bubble (Never / Google Password Manager). Safari does not. Input is type=password, outside a form, autocomplete=off, which Chrome ignores. Requirement (23-UI-SPEC Surface 2, UNLK-01): the browser must not offer to save the Mac password (Chrome, Safari, iPhone Safari)."
created: 2026-11-27T00:00:00Z
updated: 2026-11-28T00:00:00Z
---

## Current Focus

status_note: fix applied and self-verified by unit tests; awaiting the VM P-01 re-run (Chrome, Safari, iPhone Safari if available)
bug_class: Bohrbug (deterministic; reproduced in Chrome on every submit in the VM row)
reasoning_checkpoint:
  hypothesis: "Chrome offers to save because the field is <input type=password>. Chrome's password manager caches the user-typed value of any password-type field (cleared DOM values do not erase it) and, for formless fields, reads the field disappearing (the panel unmounts when the Mac unlocks) as a successful login. autocomplete=off, no form, no name and data-*-ignore are not opt-outs for that path."
  confirming_evidence:
    - "P-01.chrome.txt: prompt appeared after Return and persisted 5.5 s; the lockstate line shows locked=false at the end, so the panel (and its password field) left the DOM after a submit"
    - "The component already clears el.value in the same synchronous step as the read and has every attribute-level opt-out; the only remaining password signal is type=password"
    - "Safari (which keys its save offer on form submission, and there is no form) shows nothing"
  falsification_test: "If Chrome still shows Save password? on the VM re-run with a type=text field (no password-type input anywhere on the page), the hypothesis is wrong (e.g. Chrome classifying by label text 'password')"
  fix_rationale: "Chrome's password manager only considers password-type fields (or fields that were ever password-type). A type=text field masked by -webkit-text-security is never a password field to it, so there is nothing to provisionally save and no prompt, whatever the submission heuristics fire on."
  blind_spots: "No real Chrome run here (unit tests only; VM re-run needed). Server/ML field predictions could in theory classify a text field labelled 'Mac password'; unlikely on a private origin with no form signature. iPhone Safari not exercised."
  candidate_causes:
    - "code: input type=password (chosen)"
    - "environment: Chrome profile with password saving on (the throwaway profile default) — not controllable by the page, not a fix"
    - "code: value kept in DOM/React state until unmount — eliminated: value is cleared synchronously at submit and never in React state"
  and_gate: "yes: the prompt needs a password-type field with a user-typed value AND a submission signal (field removed when the Mac unlocked). Removing the submission signal is not possible (the panel must go when the Mac unlocks), so the fix removes the first condition."
next_action: VM P-01 re-run on a build containing the fix commit; see the handback for what it checks

## Symptoms

expected: no browser save-password prompt after an Unlock submit (Chrome, Safari, iPhone Safari)
actual: Chrome shows "Save password?" (AXWindow 448x374, Never / Google Password Manager), still there 5.5 s later; Safari shows nothing
errors: none (browser UI, not a page error)
reproduction: Chrome on portty-test-2 (throwaway profile), desktop viewer on a locked Mac, click panel field, type five keys, Return
started: since the Unlock panel shipped (23-07); first observed in 23-10 P-01

## Eliminated

## Evidence

- timestamp: 2026-11-27T00:00:00Z
  checked: .planning/phases/23-audio-adaptive-quality-unlock-and-release/evidence/P-01.chrome.txt and 23-VM-CHECKS.md row P-01
  found: AX field role AXTextField title "Mac password"; after five key events and Return, AXWindow "Save password?" with Never and Google Password Manager; persisted 5.5 s later. Safari file shows no match for any save/keychain string.
  implication: Chrome-specific password-manager heuristic, not a page bug that Safari also sees.

- timestamp: 2026-11-27T00:00:00Z
  checked: frontend/src/components/DesktopUnlockPanel.tsx
  found: input type="password", uncontrolled (ref), no form, no name, autocomplete/autocapitalize/autocorrect off, spellcheck false, data-1p-ignore, data-lpignore, data-bwignore; submit() reads el.value, sets el.value = '' in the same step, sends, zeroes the bytes. Value never in React state.
  implication: every page-level opt-out short of not being a password field is already there, and the value is already cleared synchronously; the one remaining trigger is the password input type itself (Chrome caches typed values of password fields, so clearing the DOM value does not hide it).

- timestamp: 2026-11-27T00:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entry on password manager, autofill or unlock
  implication: no known-pattern candidate

- timestamp: 2026-11-27T00:00:00Z
  checked: new tests in frontend/src/components/__tests__/DesktopUnlockPanel.test.tsx against the current component
  found: 3 failed / 30 passed: an input[type=password] exists; style.css .desktop-unlock__input has no -webkit-text-security; copy is not defaultPrevented
  implication: tests reproduce the defect's precondition (password-type field) and pin the replacement's masking and parity

- timestamp: 2026-11-27T00:00:00Z
  checked: jsdom CSSStyleDeclaration and csstype for -webkit-text-security
  found: jsdom drops the property from the style attribute; csstype (React.CSSProperties) has no WebkitTextSecurity
  implication: masking lives in style.css next to the input's other rules (house convention) and is pinned by a source-level CSS test

- timestamp: 2026-11-27T00:00:00Z
  checked: support for -webkit-text-security (web search, bug 1826629)
  found: Blink and WebKit support it (Chrome, Safari, iOS Safari, WKWebView); Mozilla bug 1826629 tracks firefox114 fixed
  implication: every target browser masks the field; a browser without it would show the text (not a target)

- timestamp: 2026-11-27T00:00:00Z
  checked: .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/js/unlockpanel.js
  found: the rig reports password_inputs = count of input[type=password]; U-08 evidence uses it (0 for a guest) alongside labelled_mac_password
  implication: after the fix password_inputs is 0 on every page, so a row must use labelled_mac_password to see whether the field exists; on the own-account panel the re-run should read password_inputs 0 and labelled_mac_password 1

## Resolution

root_cause: "The Unlock panel's Mac password field was <input type=password>. Chrome's password manager caches what the user typed into any password-type field (clearing the DOM value does not erase it) and, for a formless field, reads the field going away (the panel unmounts when the Mac unlocks) as a successful login, so it offers Save password?. autocomplete=off, no form, no name and the password-manager data-* opt-outs do not stop that path. (AND-gate: password-type field with typed value AND a submission signal; the fix removes the first.)"
fix: "Field is now type=text, masked by -webkit-text-security: disc on .desktop-unlock__input in style.css (placeholder kept readable with ::placeholder -webkit-text-security: none). Added data-form-type=other, data-gramm=false, data-enable-grammarly=false, and copy/cut/dragstart preventDefault to keep the password-field behaviours a text field lacks (paste still allowed). Value handling unchanged: uncontrolled, never in React state, emptied in the same step as the read at submit, emptied on Esc/hidden/pagehide/unmount."
oracle_type: specified (UNLK-01 / 23-UI-SPEC Surface 2: the browser must not offer to save the Mac password; the unit oracle is the derived precondition: no password-type input exists, the field is masked)
verification:
  target_test: { result: pass, tests: "DesktopUnlockPanel.test.tsx: masked text input + opt-outs; stylesheet mask + readable placeholder; copy/cut/drag blocked, paste allowed; value empty after submit" }
  mutation_check: { result: pass, reason_if_skipped: "Stryker not configured; three manual mutants seeded instead", mutant_killed: "type text->password: killed; css mask line removed: killed; onCopy removed: killed" }
  no_op_deletion: { result: pass, deletion_justified_by_rca: n/a (additive change plus one type swap) }
  adjacent_tests: { result: pass, detail: "full frontend vitest 154 files / 2829 tests green; tsc (noEmit) exit 0; tests/check-brand.sh clean" }
  revert_reconfirm: { result: pass, detail: "new tests ran RED (3 failed) on the unfixed component, GREEN after; each fix site reverted individually re-fails its test, restored files byte-identical" }
  guardrail_verdict: accepted (unit level); real-browser confirmation pending VM P-01 re-run
files_changed: [frontend/src/components/DesktopUnlockPanel.tsx, frontend/src/style.css, frontend/src/components/__tests__/DesktopUnlockPanel.test.tsx]

## Decision (2026-11-28): won't fix, accepted known limit

decision: "Won't fix. The masked text-input fix (dc02ab88 on branch p01-chrome-textfield-option) is NOT merged; the branch is kept for reference. The unlock field stays <input type=password>."
decided_by: "the user's delegate (the main orchestrator), on the user's behalf"
rationale: "type=password brings protections that matter more than Chrome's prompt: macOS Secure Keyboard Entry while the field has focus (other processes cannot read the keystrokes), screen readers announce it as secure and never speak the characters, and the system offers a Latin keyboard / no IME composition or autocorrect for it. Chrome's 'Save password?' bubble is opt-in: it saves nothing unless the user clicks Save, and the user can choose Never for the origin. Safari shows no prompt (P-01 Safari half PASS). Swapping in a CSS-masked text field to silence Chrome would give up those protections for a cosmetic gain."
residual_risk: "A user who clicks Save in Chrome stores the Mac login password in Google Password Manager for the share origin. Mitigated by the panel's copy (Portty never saves it) and by it being an explicit user action."
recorded_in: ".planning/phases/23-audio-adaptive-quality-unlock-and-release/23-VM-CHECKS.md (P-01 accepted known limit note) and the deferred real-device row for iPhone Safari (23-UAT.md / TESTING.md)"
