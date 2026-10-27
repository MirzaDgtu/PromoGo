import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { StoreOverview } from '../../api/queries'
import { OverviewPage } from './OverviewPage'
import { registerState, relativeTime } from './status'

const orgStore = {
  selectedOrgID: 2 as number | null,
  selectedStoreID: 3 as number | null,
  organizations: [{ id: 2, name: 'ИП Агаев Я.А.' }],
  stores: [
    { id: 3, organization_id: 2, name: 'Океан', address: '', external_code: null },
    { id: 4, organization_id: 2, name: 'Море', address: '', external_code: null },
  ],
  storesLoading: false,
  setSelectedStoreID: vi.fn(),
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const minutesAgo = (n: number) => new Date(Date.now() - n * 60_000).toISOString()
const window0 = { operations: 0, points_accrued: 0, points_redeemed: 0, sales_amount: '0' }
const healthy: StoreOverview = {
  store_id: 3,
  loyalty_config: { configured: true, version: 2 },
  api_keys: { active: 1, last_used_at: minutesAgo(5) },
  clients: 12,
  last_transaction_at: minutesAgo(5),
  last_24h: { operations: 4, points_accrued: 150, points_redeemed: 20, sales_amount: '3000' },
  last_7d: { operations: 30, points_accrued: 1200, points_redeemed: 300, sales_amount: '24000.5' },
}
const fresh: StoreOverview = {
  store_id: 4, loyalty_config: { configured: false }, api_keys: { active: 0 }, clients: 0, last_24h: window0, last_7d: window0,
}

const byStore = new Map<number, StoreOverview>()
vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useStoreOverview: (_o: number, storeID: number) => ({ isPending: false, isError: false, data: byStore.get(storeID), refetch: vi.fn() }),
  useStoreOverviews: (_o: number, ids: number[]) => ids.map((id) => ({ isError: false, data: byStore.get(id) })),
}))

const renderPage = () =>
  render(
    <MemoryRouter>
      <OverviewPage />
    </MemoryRouter>,
  )

beforeEach(() => {
  vi.clearAllMocks()
  byStore.set(3, healthy)
  byStore.set(4, fresh)
  orgStore.selectedStoreID = 3
  orgStore.can.mockReturnValue(true)
})

describe('OverviewPage', () => {
  it('shows a healthy store as ready, with its activity', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'Обзор — Океан' })).toBeInTheDocument()
    expect(screen.getByText('Настроена, действует версия 2')).toBeInTheDocument()
    expect(screen.getByText('Подключена, последний запрос 5 минут назад')).toBeInTheDocument()
    expect(screen.getByText('Клиентов в программе: 12')).toBeInTheDocument()

    const row = screen.getByRole('row', { name: /Начислено баллов/ })
    expect(within(row).getByText('150')).toBeInTheDocument()
    expect(within(row).getByText('1 200')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Настроить программу' })).not.toBeInTheDocument()
  })

  it('points a fresh store at what is missing', () => {
    orgStore.selectedStoreID = 4
    renderPage()
    expect(screen.getByText(/Не настроена: продажи записываются с 0 баллов/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Настроить программу' })).toHaveAttribute('href', '/loyalty-config')
    expect(screen.getByText(/Нет действующего API-ключа/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Открыть API-ключи' })).toBeInTheDocument()
    expect(screen.getByText('Операций ещё не было.')).toBeInTheDocument()
  })

  it('hides the register check from callers without api_keys.read', () => {
    byStore.set(3, { ...healthy, api_keys: undefined } as unknown as StoreOverview)
    renderPage()
    expect(screen.queryByText('Касса или 1С')).not.toBeInTheDocument()
  })

  it('lists every store with all stores selected, and opens one', async () => {
    orgStore.selectedStoreID = null
    renderPage()
    expect(screen.getByRole('heading', { name: 'Обзор — ИП Агаев Я.А.' })).toBeInTheDocument()
    const ocean = screen.getByRole('button', { name: 'Открыть обзор магазина «Океан»' }).closest('tr')!
    expect(within(ocean).getByText('Настроена')).toBeInTheDocument()
    expect(within(ocean).getByText('Подключена, 5 минут назад')).toBeInTheDocument()
    const sea = screen.getByRole('button', { name: 'Открыть обзор магазина «Море»' }).closest('tr')!
    expect(within(sea).getByText('Не настроена')).toBeInTheDocument()
    expect(within(sea).getByText('Нет ключа')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Открыть обзор магазина «Море»' }))
    expect(orgStore.setSelectedStoreID).toHaveBeenCalledWith(4)
  })
})

describe('overview status helpers', () => {
  const now = new Date('2026-10-27T12:00:00Z')
  it('renders relative times in Russian', () => {
    expect(relativeTime('2026-10-27T11:59:30Z', now)).toBe('сейчас')
    expect(relativeTime('2026-10-27T11:55:00Z', now)).toBe('5 минут назад')
    expect(relativeTime('2026-10-26T12:00:00Z', now)).toBe('вчера')
  })

  it('flags a register silent for over a day', () => {
    expect(registerState({ ...healthy, api_keys: { active: 1, last_used_at: '2026-10-25T12:00:00Z' } }, now).kind).toBe('stale')
    expect(registerState({ ...healthy, api_keys: { active: 1 } }, now).kind).toBe('never_used')
    expect(registerState({ ...healthy, api_keys: { active: 0, last_used_at: '2026-10-27T11:00:00Z' } }, now).kind).toBe('no_key')
  })
})
