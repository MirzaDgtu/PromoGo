import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { StoreForm } from './StoreForm'

function renderForm(props: Partial<Parameters<typeof StoreForm>[0]> = {}) {
  const handlers = { onSubmit: vi.fn().mockResolvedValue(undefined), onCancel: vi.fn() }
  render(<StoreForm title="Новый магазин" submitLabel="Создать" pending={false} error={null} {...handlers} {...props} />)
  return handlers
}

const httpError = (status: number) => Object.assign(new Error('x'), { response: new Response(null, { status }) })

describe('StoreForm', () => {
  it('requires a name and focuses it first', async () => {
    const { onSubmit } = renderForm()
    expect(screen.getByLabelText('Название')).toHaveFocus()
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('Укажите название')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('limits the 1C code to 64 characters', async () => {
    const { onSubmit } = renderForm()
    await userEvent.type(screen.getByLabelText('Название'), 'Океан')
    await userEvent.click(screen.getByLabelText('Код в 1С'))
    await userEvent.paste('1'.repeat(65))
    await userEvent.click(screen.getByRole('button', { name: 'Создать' }))
    expect(await screen.findByText('Не длиннее 64 символов')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('shows a 409 on the 1C code field', () => {
    renderForm({ error: httpError(409) })
    expect(screen.getByLabelText('Код в 1С')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Этот код уже используется другим магазином организации')).toBeInTheDocument()
  })

  it('shows the forbidden state for a 403', () => {
    renderForm({ error: httpError(403) })
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
  })

  it('cancels on Escape', async () => {
    const { onCancel } = renderForm()
    await userEvent.keyboard('{Escape}')
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
