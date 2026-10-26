import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { StoreAPIKey } from '../../api/queries'
import { ApiKeysPage, keyStatus } from './ApiKeysPage'

const orgStore = {
  selectedOrgID: 1 as number | null,
  selectedStoreID: 3 as number | null,
  stores: [{ id: 3, organization_id: 1, name: 'Океан', address: '', external_code: null }],
  can: vi.fn<(permission: string, orgID: number, storeID?: number | null) => boolean>(),
}
vi.mock('../../session/OrgStoreContext', () => ({ useOrgStore: () => orgStore }))

const keys: StoreAPIKey[] = [
  {
    id: 1, key_id: 'pk_live_ab12', name: 'Касса 1', scopes: ['transactions.write', 'clients.lookup'],
    created_at: '2026-10-01T10:00:00Z', last_used_at: '2026-10-26T09:30:00Z',
  },
  { id: 2, key_id: 'pk_live_cd34', name: 'Старый 1С', scopes: ['balances.read'], created_at: '2026-09-01T10:00:00Z', revoked_at: '2026-10-02T10:00:00Z' },
]
const mutation = () => ({ isPending: false, error: null as unknown, reset: vi.fn(), mutateAsync: vi.fn() })
const create = mutation()
const revoke = mutation()

vi.mock('../../api/queries', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/queries')>()),
  useStoreAPIKeys: () => ({ isPending: false, isError: false, data: keys }),
  useCreateStoreAPIKey: () => create,
  useRevokeStoreAPIKey: () => revoke,
}))

function renderPage() {
  return render(
    <MemoryRouter>
      <div id="root">
        <ApiKeysPage />
      </div>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  orgStore.selectedOrgID = 1
  orgStore.selectedStoreID = 3
  orgStore.can.mockReturnValue(true)
  create.error = null
})

describe('keyStatus', () => {
  const base = { id: 1, key_id: 'k', name: 'k', scopes: [], created_at: '2026-01-01T00:00:00Z' } satisfies StoreAPIKey
  it('treats revoked as winning over expiry, and a past expiry as expired', () => {
    const now = new Date('2026-10-26T12:00:00Z')
    expect(keyStatus({ ...base, revoked_at: '2026-10-01T00:00:00Z', expires_at: '2026-01-02T00:00:00Z' }, now)).toBe('revoked')
    expect(keyStatus({ ...base, expires_at: '2026-10-26T11:59:59Z' }, now)).toBe('expired')
    expect(keyStatus({ ...base, expires_at: '2027-01-01T00:00:00Z' }, now)).toBe('active')
    expect(keyStatus(base, now)).toBe('active')
  })
})

describe('ApiKeysPage', () => {
  it('asks to pick a store when none is selected', () => {
    orgStore.selectedStoreID = null
    renderPage()
    expect(screen.getByRole('heading', { name: 'Выберите магазин' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Перейти к магазинам' })).toHaveAttribute('href', '/organizations')
  })

  it('lists keys with readable scopes and statuses, never offering to revoke a revoked key', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'API-ключи — Океан' })).toBeInTheDocument()
    const [, active, revoked] = screen.getAllByRole('row')
    expect(within(active!).getByText('Операции')).toBeInTheDocument()
    expect(within(active!).getByText('Активен')).toBeInTheDocument()
    expect(within(active!).getByRole('button', { name: 'Отозвать ключ «Касса 1»' })).toBeInTheDocument()
    expect(within(revoked!).getByText('Отозван')).toBeInTheDocument()
    expect(within(revoked!).queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows forbidden instead of the list without api_keys.read', () => {
    orgStore.can.mockReturnValue(false)
    renderPage()
    expect(screen.getByText('Недостаточно прав')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '+ Выдать ключ' })).not.toBeInTheDocument()
  })

  it('hides issue and revoke for a read-only caller', () => {
    orgStore.can.mockImplementation((p) => p === 'api_keys.read')
    renderPage()
    expect(screen.queryByRole('button', { name: '+ Выдать ключ' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Отозвать/ })).not.toBeInTheDocument()
  })

  it('requires at least one scope before issuing a key', async () => {
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: '+ Выдать ключ' }))
    await userEvent.type(screen.getByLabelText('Название'), 'Касса 2')
    await userEvent.click(screen.getByRole('button', { name: 'Выдать ключ' }))
    expect(await screen.findByText('Выберите хотя бы одно право')).toBeInTheDocument()
    expect(create.mutateAsync).not.toHaveBeenCalled()
  })

  it('shows the plaintext once, closes only after confirmation, then forgets it', async () => {
    create.mutateAsync.mockResolvedValue({ ...keys[0], id: 3, name: 'Касса 2', plaintext_key: 'pk_live_ef56.s3cr3t' })
    renderPage()

    await userEvent.click(screen.getByRole('button', { name: '+ Выдать ключ' }))
    await userEvent.type(screen.getByLabelText('Название'), ' Касса 2 ')
    await userEvent.click(screen.getByRole('checkbox', { name: /Операции/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Выдать ключ' }))
    expect(create.mutateAsync).toHaveBeenCalledWith({ name: 'Касса 2', scopes: ['transactions.write'] })

    const dialog = screen.getByRole('dialog', { name: 'Ключ выдан' })
    expect(within(dialog).getByLabelText('API-ключ')).toHaveValue('pk_live_ef56.s3cr3t')
    expect(document.getElementById('root')).toHaveAttribute('inert')

    const close = within(dialog).getByRole('button', { name: 'Закрыть' })
    expect(close).toBeDisabled()
    await userEvent.keyboard('{Escape}')
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    await userEvent.click(within(dialog).getByRole('checkbox', { name: 'Я сохранил ключ в надёжном месте' }))
    await userEvent.click(close)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByDisplayValue('pk_live_ef56.s3cr3t')).not.toBeInTheDocument()
    expect(document.getElementById('root')).not.toHaveAttribute('inert')
    // Clears the response holding the plaintext from the mutation cache.
    expect(create.reset).toHaveBeenCalled()
  })

  it('revokes only after an inline confirmation', async () => {
    revoke.mutateAsync.mockResolvedValue(undefined)
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Отозвать ключ «Касса 1»' }))
    expect(revoke.mutateAsync).not.toHaveBeenCalled()

    const confirm = screen.getByRole('alertdialog', { name: 'Отозвать ключ «Касса 1»' })
    await userEvent.click(within(confirm).getByRole('button', { name: 'Отозвать ключ' }))
    expect(revoke.mutateAsync).toHaveBeenCalledWith(1)
    expect(await screen.findByRole('status')).toHaveTextContent('Ключ «Касса 1» отозван')
  })
})
