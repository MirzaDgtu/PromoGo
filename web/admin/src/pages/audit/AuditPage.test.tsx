import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AuditCategory, AuditEvent } from '../../api/queries'
import { AuditPage } from './AuditPage'
import { describeEvent } from './describe'

const orgStore = {
  selectedOrgID: 2 as number | null,
  organizations: [{ id: 2, name: 'ИП Агаев Я.А.' }],
  stores: [{ id: 3, organization_id: 2, name: 'Океан', address: '', external_code: null }],
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const at = '2026-10-27T10:00:00Z'
const roleChange: AuditEvent = {
  id: 3, occurred_at: at, actor_type: 'staff', actor_id: 1, actor_name: 'pmp@go-clean.ru', action: 'staff.role_changed',
  target_type: 'staff_membership', target_id: 9, target_name: 'manager@test', store_id: 3,
  metadata: { from: 'store_manager', to: 'support_viewer' }, ip: '172.18.0.1',
}
const keyCreated: AuditEvent = {
  id: 2, occurred_at: at, actor_type: 'staff', actor_id: 7, action: 'api_key.created', store_id: 3, metadata: { name: 'Касса 1' },
}
const legacy: AuditEvent = { id: 1, occurred_at: at, actor_type: 'staff', actor_id: 1, actor_name: 'pmp@go-clean.ru', action: 'loyalty_config.changed' }

const audit = {
  isPending: false, isError: false, error: null as unknown, refetch: vi.fn(),
  data: { pages: [{ events: [roleChange, keyCreated, legacy], next_cursor: 'c' as string | undefined }] },
  hasNextPage: true, isFetchingNextPage: false, fetchNextPage: vi.fn(),
}
const categories: (AuditCategory | null)[] = []
vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useAuditEvents: (_org: number, category: AuditCategory | null) => {
    categories.push(category)
    return audit
  },
}))

const renderPage = () =>
  render(
    <MemoryRouter>
      <AuditPage />
    </MemoryRouter>,
  )

beforeEach(() => {
  vi.clearAllMocks()
  categories.length = 0
  orgStore.can.mockReturnValue(true)
})

describe('AuditPage', () => {
  it('renders each event as a readable sentence with who, store and IP', async () => {
    renderPage()
    const rows = screen.getAllByRole('row').slice(1)
    expect(within(rows[0]!).getByText('pmp@go-clean.ru')).toBeInTheDocument()
    expect(
      within(rows[0]!).getByText('Изменена роль: manager@test — Управляющий магазином → Поддержка'),
    ).toBeInTheDocument()
    expect(within(rows[0]!).getByText('Магазин «Океан»')).toBeInTheDocument()
    expect(within(rows[0]!).getByText('172.18.0.1')).toBeInTheDocument()

    // An unresolvable staff actor falls back to their id.
    expect(within(rows[1]!).getByText('Сотрудник #7')).toBeInTheDocument()
    expect(within(rows[1]!).getByText('Выдан API-ключ «Касса 1»')).toBeInTheDocument()
    // Events from before metadata was recorded still read sensibly.
    expect(within(rows[2]!).getByText('Изменена программа лояльности')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Показать ещё' }))
    expect(audit.fetchNextPage).toHaveBeenCalled()
  })

  it('filters by event type', async () => {
    renderPage()
    await userEvent.selectOptions(screen.getByLabelText('Показать'), 'Сотрудники и доступ')
    expect(categories.at(-1)).toBe('staff')
    await userEvent.selectOptions(screen.getByLabelText('Показать'), 'Все события')
    expect(categories.at(-1)).toBeNull()
  })

  it('is forbidden without audit.read', () => {
    orgStore.can.mockReturnValue(false)
    renderPage()
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
  })
})

describe('describeEvent', () => {
  const store = (id: number) => `Магазин #${id}`
  it('covers membership, status and rollback events', () => {
    const base = { id: 1, occurred_at: at, actor_type: 'staff' as const, target_name: 'Анна' }
    expect(describeEvent({ ...base, action: 'staff.membership_added', metadata: { role: 'store_manager', store_id: 3 } }, store)).toBe(
      'Выдан доступ: Анна — Управляющий магазином, Магазин #3',
    )
    expect(describeEvent({ ...base, action: 'staff.status_changed', metadata: { from: 'active', to: 'disabled' } }, store)).toBe(
      'Отключён доступ: Анна',
    )
    expect(describeEvent({ ...base, action: 'loyalty_config.rolled_back', metadata: { version: 4, restored_version: 2 } }, store)).toBe(
      'Программа лояльности возвращена к версии 2 (стала версией 4)',
    )
    expect(describeEvent({ ...base, action: 'qr.resolved' }, store)).toBe('Событие qr.resolved')
  })
})
