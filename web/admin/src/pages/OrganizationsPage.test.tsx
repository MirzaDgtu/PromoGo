import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { OrganizationsPage } from './OrganizationsPage'

const orgStore = {
  selectedOrgID: 1 as number | null,
  setSelectedOrgID: vi.fn(),
  selectedStoreID: null as number | null,
  setSelectedStoreID: vi.fn(),
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
  hasGlobalPermission: vi.fn<(permission: string) => boolean>(),
}

vi.mock('../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const organizations = [
  { id: 1, name: 'Ромашка', created_at: '2026-09-09T21:34:56Z' },
  { id: 2, name: 'Лютик', created_at: '2026-10-01T10:00:00Z' },
]
const storesByOrg = [
  [{ id: 7, organization_id: 1, name: 'Океан', address: 'г. Махачкала, ул. Ленина, 1', external_code: '000123', sms_invite_enabled: false }],
  [],
]
const mutation = () => ({ isPending: false, error: null, reset: vi.fn(), mutateAsync: vi.fn() })
const createOrg = mutation()
const createStore = mutation()
const updateStore = mutation()

vi.mock('../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api/queries')>()),
  useOrganizations: () => ({ isPending: false, isError: false, data: organizations }),
  useStoresByOrganization: () => storesByOrg.map((data) => ({ isPending: false, isError: false, data })),
  useCreateOrganization: () => createOrg,
  useCreateStore: () => createStore,
  useUpdateStore: () => updateStore,
}))

// Each organization is its own tbody: [thead, Ромашка, Лютик].
const group = (index: number) => screen.getAllByRole('rowgroup')[index + 1]!

beforeEach(() => {
  vi.clearAllMocks()
  orgStore.selectedOrgID = 1
  orgStore.selectedStoreID = null
  orgStore.can.mockReturnValue(false)
  orgStore.hasGlobalPermission.mockReturnValue(false)
})

describe('OrganizationsPage', () => {
  it('groups stores under their organization with address and 1C code', () => {
    render(<OrganizationsPage />)
    const romashka = within(group(0))
    expect(romashka.getByRole('button', { name: 'Ромашка' })).toBeInTheDocument()
    expect(romashka.getByText('1 магазин')).toBeInTheDocument()
    expect(romashka.getByText('г. Махачкала, ул. Ленина, 1')).toBeInTheDocument()
    expect(romashka.getByText('000123')).toBeInTheDocument()
    expect(within(group(1)).getByText(/Магазинов пока нет/)).toBeInTheDocument()
  })

  it('shows no add or edit controls to a read-only caller', () => {
    render(<OrganizationsPage />)
    expect(screen.queryByRole('button', { name: '+ Организация' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Добавить магазин/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Изменить магазин/ })).not.toBeInTheDocument()
  })

  it('checks stores.manage per organization, not only the selected one', () => {
    orgStore.can.mockImplementation((p, orgID) => p === 'stores.manage' && orgID === 2)
    render(<OrganizationsPage />)
    expect(screen.queryByRole('button', { name: 'Добавить магазин в «Ромашка»' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Добавить магазин в «Лютик»' })).toBeInTheDocument()
  })

  it('collapses and expands an organization group', async () => {
    render(<OrganizationsPage />)
    await userEvent.click(screen.getByRole('button', { name: 'Свернуть «Ромашка»' }))
    expect(screen.queryByText('Океан')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Развернуть «Ромашка»' }))
    expect(screen.getByText('Океан')).toBeInTheDocument()
  })

  it('creates a store with trimmed fields inside its organization group', async () => {
    orgStore.can.mockReturnValue(true)
    createStore.mutateAsync.mockResolvedValue({ id: 8, name: 'Волна' })
    render(<OrganizationsPage />)

    await userEvent.click(screen.getByRole('button', { name: 'Добавить магазин в «Лютик»' }))
    const form = screen.getByRole('form', { name: 'Новый магазин в «Лютик»' })
    expect(group(1)).toContainElement(form)
    await userEvent.type(within(form).getByLabelText('Название'), ' Волна ')
    await userEvent.type(within(form).getByLabelText('Адрес'), 'ул. Гагарина, 5')
    await userEvent.click(within(form).getByRole('button', { name: 'Создать' }))

    expect(createStore.mutateAsync).toHaveBeenCalledWith({
      name: 'Волна',
      address: 'ул. Гагарина, 5',
      external_code: '',
      sms_invite_enabled: false,
    })
    expect(screen.queryByRole('form')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Магазин «Волна» создан')
  })

  it('edits a store in place, prefilled with its current values', async () => {
    orgStore.can.mockReturnValue(true)
    updateStore.mutateAsync.mockResolvedValue({ id: 7, name: 'Океан' })
    render(<OrganizationsPage />)

    await userEvent.click(screen.getByRole('button', { name: 'Изменить магазин «Океан»' }))
    const form = screen.getByRole('form', { name: 'Изменение магазина «Океан»' })
    expect(within(form).getByLabelText('Код в 1С')).toHaveValue('000123')
    const address = within(form).getByLabelText('Адрес')
    await userEvent.clear(address)
    await userEvent.type(address, 'ул. Гагарина, 5')
    const invite = within(form).getByRole('checkbox', { name: /SMS-приглашение в приложение/ })
    expect(invite).not.toBeChecked()
    await userEvent.click(invite)
    await userEvent.click(within(form).getByRole('button', { name: 'Сохранить' }))

    expect(updateStore.mutateAsync).toHaveBeenCalledWith({
      name: 'Океан',
      address: 'ул. Гагарина, 5',
      external_code: '000123',
      sms_invite_enabled: true,
    })
  })

  it('selecting a store also selects its organization', async () => {
    render(<OrganizationsPage />)
    await userEvent.click(screen.getByRole('button', { name: 'Сделать «Океан» текущим магазином' }))
    expect(orgStore.setSelectedOrgID).toHaveBeenCalledWith(1)
    expect(orgStore.setSelectedStoreID).toHaveBeenCalledWith(7)
  })

  it('after creating an organization, selects it and opens its first-store form', async () => {
    orgStore.hasGlobalPermission.mockReturnValue(true)
    orgStore.can.mockReturnValue(true)
    createOrg.mutateAsync.mockResolvedValue({ id: 2, name: 'Лютик' })
    render(<OrganizationsPage />)

    await userEvent.click(screen.getByRole('button', { name: '+ Организация' }))
    await userEvent.type(screen.getByLabelText('Название новой организации'), 'Лютик{Enter}')

    expect(orgStore.setSelectedOrgID).toHaveBeenCalledWith(2)
    expect(screen.getByRole('form', { name: 'Новый магазин в «Лютик»' })).toBeInTheDocument()
  })
})
