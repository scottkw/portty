import React from 'react'

interface SwitchProps {
  /** Pair with a `<label htmlFor={id}>` (or wrap it in a <label>) for the accessible name. */
  id?: string
  checked: boolean
  onChange: () => void
  disabled?: boolean
  /** A save is in flight: dimmed and click-through blocked, but still focusable. */
  saving?: boolean
  'aria-label'?: string
}

/**
 * The one toggle control (UI-SPEC "Switch"): a native-metric NSSwitch look,
 * 38×22px with an 18px knob, on = --color-selection-fill.
 */
export function Switch({ id, checked, onChange, disabled, saving, 'aria-label': ariaLabel }: SwitchProps): React.ReactElement {
  return (
    <button
      type="button"
      role="switch"
      id={id}
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      className={`switch${saving ? ' switch--saving' : ''}`}
      onClick={() => {
        if (!saving) onChange()
      }}
    >
      <span className="switch__knob" aria-hidden="true" />
    </button>
  )
}
