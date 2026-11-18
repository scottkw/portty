import React from 'react'

// RED placeholder: renders children, catches nothing.
export class ErrorBoundary extends React.Component<{ label: string; resetKey?: unknown; fallback?: React.ReactNode; children?: React.ReactNode }> {
  render() { return this.props.children }
}
