import React from 'react'
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline'

/**
 * Portty 05-09 (SHAR-07): contains a render or effect error to one pane, so the
 * sidebar and other tabs stay usable (the user may have no other way into the
 * remote Mac). The fallback is fixed copy and the log names only the error
 * type: a WebSocket error message carries the URL, and a peer URL has ?cap=.
 */
interface Props {
  label: string
  /** A change clears the error and renders the children again. */
  resetKey?: unknown
  fallback?: React.ReactNode
  children?: React.ReactNode
}

export class ErrorBoundary extends React.Component<Props, { hasError: boolean }> {
  state = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch(error: unknown) {
    console.error('[ErrorBoundary]', this.props.label, error instanceof Error ? error.name : typeof error)
  }

  componentDidUpdate(prev: Props) {
    if (this.state.hasError && prev.resetKey !== this.props.resetKey) this.setState({ hasError: false })
  }

  render() {
    if (!this.state.hasError) return this.props.children
    return this.props.fallback ?? (
      <div className="banner banner--warning" role="status">
        <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
        <span className="banner__message">This tab hit an error. Close it and open the session again.</span>
      </div>
    )
  }
}
