import { useState, type ReactNode } from 'react'

import { requestIdFrom } from '../api/client'
import { strings } from '../i18n/strings'

export function LoadingState({ label = strings.common.loading }: { label?: string }) {
  return (
    <p role="status" aria-live="polite">
      {label}
    </p>
  )
}

export function EmptyState({ title, body }: { title: string; body?: ReactNode }) {
  return (
    <div role="status">
      <h2>{title}</h2>
      {body}
    </div>
  )
}

export function ForbiddenState() {
  return (
    <div role="alert">
      <h2>{strings.forbidden.title}</h2>
      <p>{strings.forbidden.body}</p>
    </div>
  )
}

export function NotFoundState() {
  return (
    <div role="alert">
      <h2>{strings.notFound.title}</h2>
      <p>{strings.notFound.body}</p>
    </div>
  )
}

// ErrorState shows a copyable X-Request-Id instead of any internal detail
// (see docs/admin-web-implementation-prompt.md: errors must never leak
// internal details, only a request id support can look up).
export function ErrorState({
  title = strings.serverError.title,
  body = strings.serverError.body,
  response,
  onRetry,
}: {
  title?: string
  body?: ReactNode
  response?: Response
  onRetry?: () => void
}) {
  const requestId = requestIdFrom(response)
  const [copied, setCopied] = useState(false)

  return (
    <div role="alert">
      <h2>{title}</h2>
      <p>{body}</p>
      {requestId && (
        <p>
          <code>{requestId}</code>{' '}
          <button
            type="button"
            onClick={() => {
              void navigator.clipboard.writeText(requestId).then(() => setCopied(true))
            }}
          >
            {copied ? strings.common.copied : strings.common.copyRequestId}
          </button>
        </p>
      )}
      {onRetry && (
        <button type="button" onClick={onRetry}>
          {strings.common.retry}
        </button>
      )}
    </div>
  )
}
