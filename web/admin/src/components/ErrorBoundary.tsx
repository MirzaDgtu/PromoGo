import { Component, type ErrorInfo, type ReactNode } from 'react'

import { strings } from '../i18n/strings'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

// A last-resort boundary for render-time errors outside of TanStack Query's
// own error states (which components handle inline via ErrorState). Never
// logs the error object itself to any remote telemetry — only this app's
// own console, and only in dev — since it may contain values from
// component props/state that could include customer data.
export class ErrorBoundary extends Component<Props, State> {
  override state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    if (import.meta.env.DEV) {
      console.error('Unhandled render error', error, info.componentStack)
    }
  }

  override render() {
    if (this.state.error) {
      return (
        <main role="alert" style={{ padding: 32 }}>
          <h1>{strings.serverError.title}</h1>
          <p>{strings.serverError.body}</p>
          <button type="button" onClick={() => this.setState({ error: null })}>
            {strings.common.retry}
          </button>
        </main>
      )
    }
    return this.props.children
  }
}
