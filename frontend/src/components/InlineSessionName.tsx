import React, { useState, useRef, useEffect } from 'react'

export interface InlineSessionNameProps {
  id: string
  name: string
  onRenamed?: (name: string) => void
  /** Controlled edit mode, so a menu item or a parent double-click can start it. */
  editing?: boolean
  onEditingChange?: (editing: boolean) => void
}

/**
 * InlineSessionName — inline-editable session name (sidebar and tab rename reuse it).
 *
 * CR-02 fix: this component does NOT call RenameSession directly.
 * The RPC is owned by App.handleRenameTab, which the parent wires in as the
 * onRenamed callback.
 * This component fires onRenamed(trimmed) and lets the parent chain handle
 * the actual RPC + state update. This prevents the double-RPC that occurred
 * when InlineSessionName called RenameSession AND the parent's onRenamed
 * callback also propagated up to App.handleRenameTab which called it again.
 *
 * - Click the name span to enter edit mode (uncontrolled), or set `editing`
 *   from the parent (TabBar and Sidebar: double-click or the Rename menu item)
 * - Enter commits the rename — fires onRenamed, exits edit mode
 * - Escape cancels and restores the original name
 * - Blur with unchanged or empty-trimmed value does NOT fire onRenamed
 */
export function InlineSessionName({
  id: _id,
  name,
  onRenamed,
  editing: editingProp,
  onEditingChange,
}: InlineSessionNameProps): React.ReactElement {
  const [localEditing, setLocalEditing] = useState(false)
  const editing = editingProp ?? localEditing
  const [editValue, setEditValue] = useState(name)
  const inputRef = useRef<HTMLInputElement>(null)
  // One commit or cancel per edit: unmounting the focused input can also blur it.
  const doneRef = useRef(false)

  function setEditing(next: boolean): void {
    if (editingProp === undefined) setLocalEditing(next)
    onEditingChange?.(next)
  }

  // When entering edit mode, focus and select all text in the input
  useEffect(() => {
    if (editing) {
      doneRef.current = false
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [editing])

  // Keep editValue in sync if name prop changes externally while not editing
  useEffect(() => {
    if (!editing) {
      setEditValue(name)
    }
  }, [name, editing])

  function commitEdit(): void {
    if (doneRef.current) return
    doneRef.current = true
    const trimmed = editValue.trim()
    if (trimmed.length > 0 && trimmed !== name) {
      // CR-02: fire callback only — App.handleRenameTab owns the RenameSession RPC
      onRenamed?.(trimmed)
    } else {
      setEditValue(name)
    }
    setEditing(false)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>): void {
    if (e.key === 'Enter') {
      commitEdit()
    }
    if (e.key === 'Escape') {
      if (doneRef.current) return
      doneRef.current = true
      setEditValue(name)
      setEditing(false)
    }
  }

  if (editing) {
    return (
      <input
        ref={inputRef}
        className="tab__rename-input"
        value={editValue}
        aria-label="Session name"
        onChange={(e) => setEditValue(e.target.value)}
        onBlur={commitEdit}
        onKeyDown={handleKeyDown}
        onClick={(e) => e.stopPropagation()}
      />
    )
  }

  return (
    <span
      className="inline-session-name"
      onClick={() => {
        setEditValue(name)
        setEditing(true)
      }}
    >
      {name}
    </span>
  )
}
