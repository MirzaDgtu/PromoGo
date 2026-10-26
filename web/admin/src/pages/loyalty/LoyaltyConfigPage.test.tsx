import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { LoyaltyConfig, LoyaltyConfigVersion } from '../../api/queries'
import { LoyaltyConfigPage } from './LoyaltyConfigPage'

const orgStore = {
  selectedOrgID: 1 as number | null,
  selectedStoreID: 3 as number | null,
  stores: [{ id: 3, organization_id: 1, name: 'Океан', address: '', external_code: null }],
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const configured: LoyaltyConfig = {
  store_id: 3, mechanic: 'points', accrual_percent: '5', min_purchase_amount: '0',
  min_balance_to_redeem: 0, max_redeem_percent: '50', points_exchange_rate: '1', version: 2,
}
const history: LoyaltyConfigVersion[] = [
  { ...configured, version: 2, changed_by_staff_user_id: 3, created_at: '2026-10-26T10:00:00Z' },
  { ...configured, version: 1, accrual_percent: '3', changed_by_staff_user_id: 3, created_at: '2026-10-20T10:00:00Z' },
]

const state = { config: configured as LoyaltyConfig | null }
const refetch = vi.fn()
const mutation = () => ({ isPending: false, error: null as unknown, reset: vi.fn(), mutateAsync: vi.fn() })
let save = mutation()
let rollback = mutation()

vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useLoyaltyConfig: () => ({ isPending: false, isError: false, data: state.config, refetch }),
  useLoyaltyConfigHistory: () => ({ isPending: false, isError: false, data: history }),
  useSaveLoyaltyConfig: () => save,
  useRollbackLoyaltyConfig: () => rollback,
}))

const renderPage = () =>
  render(
    <MemoryRouter>
      <LoyaltyConfigPage />
    </MemoryRouter>,
  )
const field = (label: RegExp) => screen.getByLabelText(label)
const httpError = (status: number) => Object.assign(new Error('x'), { response: new Response(null, { status }) })

beforeEach(() => {
  vi.clearAllMocks()
  state.config = configured
  save = mutation()
  rollback = mutation()
  orgStore.selectedStoreID = 3
  orgStore.can.mockReturnValue(true)
})

describe('LoyaltyConfigPage', () => {
  it('explains an unconfigured store and saves with expected_version 0, accepting a comma', async () => {
    state.config = null
    save.mutateAsync.mockResolvedValue({ ...configured, version: 1 })
    renderPage()

    expect(screen.getByText('Программа лояльности ещё не настроена')).toBeInTheDocument()
    const accrual = field(/Начислять/)
    await userEvent.clear(accrual)
    await userEvent.type(accrual, '2,5')
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить и включить' }))

    expect(save.mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ accrual_percent: '2.5', mechanic: 'points', expected_version: 0 }),
    )
    expect(await screen.findByText('Сохранено. Действует версия 1.')).toBeInTheDocument()
  })

  it('warns that saves apply immediately and enables Save only after a change', async () => {
    renderPage()
    expect(screen.getByText(/Изменения применяются сразу/)).toBeInTheDocument()
    const saveButton = screen.getByRole('button', { name: 'Сохранить' })
    expect(saveButton).toBeDisabled()

    await userEvent.clear(field(/Начислять/))
    await userEvent.type(field(/Начислять/), '7')
    expect(saveButton).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Отменить изменения' }))
    expect(field(/Начислять/)).toHaveValue('5')
  })

  it('validates ranges and precision before sending', async () => {
    renderPage()
    await userEvent.clear(field(/Начислять/))
    await userEvent.type(field(/Начислять/), '101')
    expect(await screen.findByText('От 0 до 100')).toBeInTheDocument()

    await userEvent.clear(field(/Стоимость одного балла/))
    await userEvent.type(field(/Стоимость одного балла/), '0')
    expect(await screen.findByText('Должно быть больше нуля')).toBeInTheDocument()
    expect(screen.getByText('Исправьте значения в форме, чтобы увидеть пример.')).toBeInTheDocument()
  })

  it('previews accrual and the redemption cap, explaining that 0 % is uncapped', async () => {
    renderPage()
    expect(screen.getByText('Покупатель получит 50 баллов')).toBeInTheDocument()
    expect(screen.getByText('Баллами можно оплатить до 500 ₽ (500 баллов)')).toBeInTheDocument()

    await userEvent.clear(field(/Оплатить баллами/))
    await userEvent.type(field(/Оплатить баллами/), '0')
    expect(screen.getByText('Баллами можно оплатить всю покупку')).toBeInTheDocument()
  })

  it('on 409 offers to reload instead of overwriting', async () => {
    save.error = httpError(409)
    renderPage()
    expect(screen.getByRole('alert')).toHaveTextContent('настройки изменил кто-то другой')
    await userEvent.click(screen.getByRole('button', { name: 'Загрузить актуальную версию' }))
    expect(refetch).toHaveBeenCalled()
  })

  it('is read-only without loyalty_config.write', () => {
    orgStore.can.mockImplementation((p) => p === 'loyalty_config.read')
    renderPage()
    expect(field(/Начислять/)).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Сохранить' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Вернуть версию/ })).not.toBeInTheDocument()
    // The preview stays usable for read-only callers.
    expect(screen.getByLabelText('Сумма покупки, ₽')).toBeEnabled()
  })

  it('restores an older version after confirmation, guarded by the current version', async () => {
    rollback.mutateAsync.mockResolvedValue({ ...configured, version: 3 })
    renderPage()
    const rows = screen.getAllByRole('row')
    expect(within(rows[1]!).getByText('Действует')).toBeInTheDocument()
    expect(within(rows[1]!).queryByRole('button')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Вернуть версию 1' }))
    const confirm = screen.getByRole('alertdialog', { name: 'Вернуть версию 1' })
    await userEvent.click(within(confirm).getByRole('button', { name: 'Вернуть версию' }))

    expect(rollback.mutateAsync).toHaveBeenCalledWith({ version: 1, expected_version: 2 })
    expect(await screen.findByText('Значения версии 1 возвращены как версия 3.')).toBeInTheDocument()
  })
})
