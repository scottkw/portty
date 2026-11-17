import React from 'react'

interface SwitchProps {
  /** Pair with a `<label htmlFor={id}>` (or wrap it in a <label>) for the accessible name. */
  id?: string
  checked: boolean
  onChange: () => void
  disabled?: boolean
  /** A save is in flight: dimmed and click-through blocked, but still focusable. */
  saving?: boolean
  /** Visible but not operable (Tailscale off): aria-disabled, click blocked, still focusable. */
  blocked?: boolean
  /** id of the visible text that explains the switch (rendered as aria-describedby). */
  describedBy?: string
  'aria-label'?: string
}

/**
 * The one toggle control (UI-SPEC "Switch"): a native-metric NSSwitch look,
 * 38×22px with an 18px knob, on = --color-selection-fill.
 */
export function Switch({ id, checked, onChange, disabled, saving, blocked, describedBy, 'aria-label': ariaLabel }: SwitchProps): React.ReactElement {
  const inert = saving || blocked
  return (
    <button
      type="button"
      role="switch"
      id={id}
      aria-checked={checked}
      aria-label={ariaLabel}
      aria-disabled={inert ? 'true' : undefined}
      aria-describedby={describedBy}
      disabled={disabled}
      className={`switch${inert ? ' switch--saving' : ''}`}
      onClick={() => {
        if (!inert) onChange()
      }}
    >
      <span className="switch__knob" aria-hidden="true" />
    </button>
  )
}
