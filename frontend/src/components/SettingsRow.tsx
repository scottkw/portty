import React from 'react'

/** One grouped-form row: label and optional description on the left, control on the right. */
export function Row({ label, htmlFor, description, descriptionId, children }: {
  label: string
  htmlFor?: string
  description?: React.ReactNode
  descriptionId?: string
  children?: React.ReactNode
}): React.ReactElement {
  return (
    <div className="settings-row">
      <div className="settings-row__text">
        {htmlFor
          ? <label className="settings-row__label" htmlFor={htmlFor}>{label}</label>
          : <span className="settings-row__label">{label}</span>}
        {description != null && <p className="settings-row__description" id={descriptionId}>{description}</p>}
      </div>
      {children != null && <div className="settings-row__control">{children}</div>}
    </div>
  )
}
