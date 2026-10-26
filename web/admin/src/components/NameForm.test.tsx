import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { NameForm } from './NameForm'

function renderForm(props: Partial<Parameters<typeof NameForm>[0]> = {}) {
  const onSubmit = props.onSubmit ?? vi.fn().mockResolvedValue(undefined)
  render(<NameForm label="Новый магазин" submitLabel="Создать" pending={false} error={null} {...props} onSubmit={onSubmit} />)
  return { onSubmit, input: screen.getByLabelText('Новый магазин') }
}

describe('NameForm', () => {
  it('rejects a whitespace-only name without calling onSubmit', async () => {
    const { onSubmit, input } = renderForm()
    await userEvent.type(input, '   ')
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('Укажите название')).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('counts characters, not bytes: 255 Cyrillic letters pass, 256 do not', async () => {
    const { onSubmit, input } = renderForm()
    await userEvent.click(input)
    await userEvent.paste('я'.repeat(256))
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('Не длиннее 255 символов')).toBeInTheDocument()

    await userEvent.type(input, '{Backspace}')
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(onSubmit).toHaveBeenCalledWith('я'.repeat(255))
  })

  it('submits the trimmed name and clears the input on success', async () => {
    const { onSubmit, input } = renderForm()
    await userEvent.type(input, '  Магазин на Ленина  ')
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(onSubmit).toHaveBeenCalledWith('Магазин на Ленина')
    expect(input).toHaveValue('')
  })

  it('keeps the typed name when the submit fails', async () => {
    const { input } = renderForm({ onSubmit: vi.fn().mockRejectedValue(new Error('boom')) })
    await userEvent.type(input, 'Ромашка')
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(input).toHaveValue('Ромашка')
  })

  it('shows the forbidden state for a 403 and the request id for a 5xx', () => {
    const forbidden = Object.assign(new Error('x'), { response: new Response(null, { status: 403 }) })
    const { unmount } = render(
      <NameForm label="L" submitLabel="S" pending={false} error={forbidden} onSubmit={vi.fn()} />,
    )
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
    unmount()

    const serverError = Object.assign(new Error('x'), {
      response: new Response(null, { status: 500, headers: { 'X-Request-Id': 'req-500' } }),
    })
    render(<NameForm label="L" submitLabel="S" pending={false} error={serverError} onSubmit={vi.fn()} />)
    expect(screen.getByText('req-500')).toBeInTheDocument()
  })
})
