import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { StaffMembership } from '../../api/queries'
import { NoMembershipPage } from '../NoMembershipPage'
import { StaffPage } from './StaffPage'

const orgStore = {
  selectedOrgID: 2 as number | null,
  organizations: [{ id: 2, name: 'ИП Агаев Я.А.' }],
  stores: [{ id: 3, organization_id: 2, name: 'Океан', address: '', external_code: null }],
  memberships: [{ organization_id: 2, role: 'retailer_admin', permissions: ['staff.manage'] }] as {
    organization_id: number
    role: string
    permissions: string[]
  }[],
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const auth = {
  staffUserID: 1,
  identity: null as { subject: string; email: string; name: string } | null,
  logout: vi.fn(),
}
vi.mock('../../auth/AuthProvider', () => ({ useAuth: () => auth }))

const base = { organization_id: 2, status: 'active', created_at: '2026-10-20T10:00:00Z', updated_at: '2026-10-20T10:00:00Z' } as const
const me: StaffMembership = { ...base, id: 1, staff_user_id: 1, role: 'retailer_admin', external_subject: 'me@x', email: 'me@x.ru', display_name: 'Я Админ' }
const manager: StaffMembership = { ...base, id: 2, staff_user_id: 2, role: 'store_manager', store_id: 3, external_subject: 'anna@x', email: 'anna@x.ru', display_name: 'Анна' }
const pending: StaffMembership = { ...base, id: 3, staff_user_id: 3, role: 'support_viewer', external_subject: 'a3f1-c9e2' }
const platform: StaffMembership = { ...base, id: 4, staff_user_id: 4, role: 'platform_admin', external_subject: 'ops@promogo', email: 'ops@promogo.ru' }
const off: StaffMembership = { ...base, id: 5, staff_user_id: 5, role: 'store_manager', status: 'disabled', external_subject: 'boris@x', display_name: 'Борис' }

const list = { isPending: false, isError: false, error: null as unknown, refetch: vi.fn(), data: [] as StaffMembership[] }
const mutation = () => ({ isPending: false, error: null as unknown, reset: vi.fn(), mutateAsync: vi.fn() })
let create = mutation()
let update = mutation()

vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useStaffMemberships: () => list,
  useCreateStaffMembership: () => create,
  useUpdateStaffMembership: () => update,
}))

const renderPage = () =>
  render(
    <MemoryRouter>
      <StaffPage />
    </MemoryRouter>,
  )
const rowOf = (text: string) => screen.getByText(text).closest('tr')!

beforeEach(() => {
  vi.clearAllMocks()
  create = mutation()
  update = mutation()
  list.data = [off, manager, me, pending, platform]
  orgStore.can.mockReturnValue(true)
  orgStore.memberships = [{ organization_id: 2, role: 'retailer_admin', permissions: ['staff.manage'] }]
})

describe('StaffPage', () => {
  it('lists people with role, scope and status; disabled last; self and platform admins locked', () => {
    renderPage()
    const rows = screen.getAllByRole('row').slice(1)
    expect(within(rows.at(-1)!).getByText('Борис')).toBeInTheDocument()
    expect(within(rows.at(-1)!).getByText('Отключён')).toBeInTheDocument()

    expect(within(rowOf('Анна')).getByText('Управляющий магазином')).toBeInTheDocument()
    expect(within(rowOf('Анна')).getByText('Магазин «Океан»')).toBeInTheDocument()

    // Never logged in: shown by the access code it was granted with.
    expect(within(rowOf('a3f1-c9e2').closest('tr')!).getByText('Ещё не входил')).toBeInTheDocument()

    expect(within(rowOf('Я Админ')).getByText('Это вы')).toBeInTheDocument()
    expect(within(rowOf('Я Админ')).queryByRole('button')).not.toBeInTheDocument()
    expect(within(rowOf('ops@promogo.ru')).queryByRole('button')).not.toBeInTheDocument()
  })

  it('grants access by pasted code, scoped to a store', async () => {
    create.mutateAsync.mockResolvedValue({ ...pending, id: 9, external_subject: 'new-code' })
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: '+ Добавить сотрудника' }))
    const form = screen.getByRole('form', { name: 'Новый сотрудник' })

    // A retailer admin can't grant platform_admin.
    expect(within(form).queryByRole('option', { name: 'Администратор платформы' })).not.toBeInTheDocument()

    await userEvent.click(within(form).getByRole('button', { name: 'Выдать доступ' }))
    expect(within(form).getByText('Вставьте код доступа сотрудника')).toBeInTheDocument()

    await userEvent.type(within(form).getByLabelText('Код доступа'), '  new-code ')
    await userEvent.selectOptions(within(form).getByLabelText('Доступ'), 'Магазин «Океан»')
    await userEvent.click(within(form).getByRole('button', { name: 'Выдать доступ' }))

    expect(create.mutateAsync).toHaveBeenCalledWith({ external_subject: 'new-code', role: 'store_manager', store_id: 3 })
    expect(await screen.findByText('Доступ выдан: new-code. Сотрудник может войти в админку.')).toBeInTheDocument()
  })

  it('keeps network-wide roles organization-wide and explains a 409', async () => {
    create.error = Object.assign(new Error('x'), { response: new Response(null, { status: 409 }) })
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: '+ Добавить сотрудника' }))
    const form = screen.getByRole('form', { name: 'Новый сотрудник' })

    await userEvent.selectOptions(within(form).getByLabelText('Роль'), 'Администратор сети')
    expect(within(form).getByLabelText('Доступ')).toBeDisabled()
    expect(within(form).getByText('Эта роль всегда действует во всей организации.')).toBeInTheDocument()
    expect(within(form).getByRole('alert')).toHaveTextContent('уже есть такой доступ')
  })

  it('changes a role inline and disables after confirmation', async () => {
    update.mutateAsync.mockResolvedValue(undefined)
    renderPage()

    await userEvent.click(screen.getByRole('button', { name: 'Изменить роль: Анна' }))
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Изменить роль: Анна' }), 'Поддержка')
    await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    expect(update.mutateAsync).toHaveBeenCalledWith({ id: 2, role: 'support_viewer' })
    expect(await screen.findByText('Изменения сохранены: Анна')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Отключить доступ: Анна' }))
    const confirm = screen.getByRole('alertdialog', { name: 'Отключить доступ: Анна' })
    await userEvent.click(within(confirm).getByRole('button', { name: 'Отключить' }))
    expect(update.mutateAsync).toHaveBeenLastCalledWith({ id: 2, status: 'disabled' })

    await userEvent.click(screen.getByRole('button', { name: 'Включить доступ: Борис' }))
    expect(update.mutateAsync).toHaveBeenLastCalledWith({ id: 5, status: 'active' })
  })

  it('lets a platform admin manage and grant platform_admin', async () => {
    orgStore.memberships = [{ organization_id: 1, role: 'platform_admin', permissions: ['staff.manage'] }]
    renderPage()
    expect(within(rowOf('ops@promogo.ru')).getByRole('button', { name: /Отключить/ })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '+ Добавить сотрудника' }))
    expect(screen.getByRole('option', { name: 'Администратор платформы' })).toBeInTheDocument()
  })

  it('is forbidden without staff.manage', () => {
    orgStore.can.mockReturnValue(false)
    renderPage()
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
  })
})

describe('NoMembershipPage', () => {
  it('shows the access code to hand to an administrator', async () => {
    auth.identity = { subject: 'a3f1-c9e2', email: 'new@x.ru', name: '' }
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    render(<NoMembershipPage />)

    expect(screen.getByText('Вы вошли как new@x.ru.')).toBeInTheDocument()
    expect(screen.getByLabelText('Ваш код доступа')).toHaveValue('a3f1-c9e2')
    await userEvent.click(screen.getByRole('button', { name: 'Скопировать код' }))
    expect(writeText).toHaveBeenCalledWith('a3f1-c9e2')
  })
})
