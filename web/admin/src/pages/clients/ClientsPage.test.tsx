import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminClient, AdminTransaction } from '../../api/queries'
import { ClientsPage } from './ClientsPage'
import { formatPhone, formatPointsDelta } from './format'

const orgStore = {
  selectedOrgID: 2 as number | null,
  selectedStoreID: 3 as number | null,
  stores: [{ id: 3, organization_id: 2, name: 'Океан', address: '', external_code: null }],
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const anna: AdminClient = {
  client_id: 7, phone: '+79261234567', balance: 150, app_connected: true, created_at: '2026-10-20T10:00:00Z',
}
const boris: AdminClient = {
  client_id: 8, phone: '+79035550011', balance: 0, app_connected: false, created_at: '2026-10-21T10:00:00Z',
}
const txs: AdminTransaction[] = [
  {
    id: 2, external_tx_id: 'R-2', type: 'redeem', amount: '500', currency: 'RUB',
    points_delta: -50, balance_after: 150, created_at: '2026-10-22T10:00:00Z',
  },
  {
    id: 1, external_tx_id: 'A-1', type: 'accrual', amount: '4000', currency: 'RUB',
    points_delta: 200, balance_after: 200, created_at: '2026-10-21T10:00:00Z',
  },
]

const list = {
  isPending: false, isError: false, error: null as unknown, refetch: vi.fn(),
  data: { pages: [{ clients: [anna, boris], next_cursor: 'c1' as string | undefined }] },
  hasNextPage: true, isFetchingNextPage: false, fetchNextPage: vi.fn(),
}
const lookupResults = new Map<string, AdminClient | null>()
const lookupCalls: (string | null)[] = []

vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useStoreClients: () => list,
  useClientLookup: (_o: number, _s: number, phone: string | null) => {
    lookupCalls.push(phone)
    return {
      isFetching: false, isError: false, error: null, refetch: vi.fn(),
      data: phone == null ? undefined : lookupResults.get(phone),
    }
  },
  useClientTransactions: () => ({ isPending: false, isError: false, data: txs, refetch: vi.fn() }),
}))

const renderPage = () =>
  render(
    <MemoryRouter>
      <ClientsPage />
    </MemoryRouter>,
  )

beforeEach(() => {
  vi.clearAllMocks()
  lookupResults.clear()
  lookupCalls.length = 0
  orgStore.selectedStoreID = 3
  orgStore.can.mockReturnValue(true)
  list.data.pages[0]!.clients = [anna, boris]
  list.hasNextPage = true
})

describe('ClientsPage', () => {
  it('lists clients with formatted phone, balance and app status, and loads more', async () => {
    renderPage()
    const rows = screen.getAllByRole('row')
    expect(within(rows[1]!).getByText('+7 926 123-45-67')).toBeInTheDocument()
    expect(within(rows[1]!).getByText('Подключено')).toBeInTheDocument()
    expect(within(rows[2]!).getByText('Не установлено')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Показать ещё' }))
    expect(list.fetchNextPage).toHaveBeenCalled()
  })

  it('opens a client card with the operation history', async () => {
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Открыть клиента +7 926 123-45-67' }))

    expect(screen.getByRole('heading', { name: '+7 926 123-45-67' })).toBeInTheDocument()
    expect(screen.getByText('150 баллов')).toBeInTheDocument()
    const rows = screen.getAllByRole('row')
    expect(within(rows[1]!).getByText('Списание')).toBeInTheDocument()
    expect(within(rows[1]!).getByText('−50')).toBeInTheDocument()
    expect(within(rows[2]!).getByText('+200')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: '← Все клиенты' }))
    expect(screen.getByRole('search')).toBeInTheDocument()
  })

  it('searches by phone: a hit opens the card, a miss explains why', async () => {
    lookupResults.set('8 903 555 00 11', boris)
    lookupResults.set('8 900 000 00 00', null)
    renderPage()

    await userEvent.click(screen.getByRole('button', { name: 'Найти' }))
    expect(screen.getByText('Введите номер телефона')).toBeInTheDocument()

    await userEvent.type(screen.getByLabelText('Найти по телефону'), '8 900 000 00 00')
    await userEvent.click(screen.getByRole('button', { name: 'Найти' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Клиент с номером 8 900 000 00 00 в этом магазине не найден.')

    await userEvent.clear(screen.getByLabelText('Найти по телефону'))
    await userEvent.type(screen.getByLabelText('Найти по телефону'), '8 903 555 00 11')
    await userEvent.click(screen.getByRole('button', { name: 'Найти' }))
    expect(screen.getByRole('heading', { name: '+7 903 555-00-11' })).toBeInTheDocument()
    expect(screen.getByText(/ещё не входил в приложение/)).toBeInTheDocument()
  })

  it('hides history without transactions.read and explains masked phones', async () => {
    orgStore.can.mockImplementation((p) => p === 'clients.read')
    list.data.pages[0]!.clients = [{ ...anna, phone: '********4567' }]
    renderPage()
    expect(screen.getByText(/Номера телефонов скрыты/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Открыть клиента ********4567' }))
    expect(screen.getByText('У вашей роли нет доступа к истории операций.')).toBeInTheDocument()
  })

  it('is forbidden without clients.read and asks for a store when none is selected', () => {
    orgStore.can.mockReturnValue(false)
    const { unmount } = renderPage()
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
    expect(lookupCalls.every((p) => p == null)).toBe(true)
    unmount()

    orgStore.selectedStoreID = null
    renderPage()
    expect(screen.getByText('Выберите магазин')).toBeInTheDocument()
  })
})

describe('client formatting', () => {
  it('formats Russian numbers and keeps anything else as sent', () => {
    expect(formatPhone('+79261234567')).toBe('+7 926 123-45-67')
    expect(formatPhone('+4915112345678')).toBe('+4915112345678')
    expect(formatPhone('********4567')).toBe('********4567')
  })

  it('signs point deltas explicitly', () => {
    expect(formatPointsDelta(1200)).toBe('+1 200')
    expect(formatPointsDelta(-5)).toBe('−5')
    expect(formatPointsDelta(0)).toBe('0')
  })
})
