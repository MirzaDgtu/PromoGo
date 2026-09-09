import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { ErrorState, ForbiddenState, NotFoundState } from './states'

describe('ForbiddenState', () => {
  it('renders as an alert, not silently', () => {
    render(<ForbiddenState />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

describe('NotFoundState', () => {
  it('renders as an alert', () => {
    render(<NotFoundState />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

describe('ErrorState', () => {
  it('shows a copyable request id from the response header, never internal error detail', () => {
    const response = new Response(null, { headers: { 'X-Request-Id': 'req-123' } })
    render(<ErrorState response={response} />)
    expect(screen.getByText('req-123')).toBeInTheDocument()
  })

  it('calls onRetry when the retry button is clicked', async () => {
    const onRetry = vi.fn()
    render(<ErrorState onRetry={onRetry} />)
    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })
})
