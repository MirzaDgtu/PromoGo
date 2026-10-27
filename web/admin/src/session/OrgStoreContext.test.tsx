import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { OrgStoreProvider, useOrgStore } from './OrgStoreContext'

const me = { memberships: [] as unknown[] }
const storeList = { data: [] as { id: number; name: string }[] }

vi.mock('../api/queries', () => ({
  useStaffMe: () => ({ data: me }),
  useOrganizations: () => ({ data: [{ id: 1, name: 'Home' }, { id: 2, name: 'Other' }], isLoading: false }),
  useStores: () => ({ data: storeList.data, isLoading: false, isSuccess: true }),
}))

function wrapper({ children }: { children: ReactNode }) {
  return <OrgStoreProvider enabled>{children}</OrgStoreProvider>
}

function permissionsInOrg2(permission: string): boolean {
  sessionStorage.setItem('promogo.admin.selectedOrgID', '2')
  const { result } = renderHook(() => useOrgStore(), { wrapper })
  return result.current.hasPermission(permission)
}

beforeEach(() => {
  sessionStorage.clear()
  storeList.data = []
})

describe('OrgStoreContext.hasPermission', () => {
  it('treats platform_admin as network-wide', () => {
    me.memberships = [
      { organization_id: 1, store_id: null, role: 'platform_admin', permissions: ['stores.manage'] },
    ]
    expect(permissionsInOrg2('stores.manage')).toBe(true)
  })

  it('can() checks a given organization and store, independent of the selection', () => {
    me.memberships = [{ organization_id: 1, store_id: 5, role: 'store_manager', permissions: ['loyalty_config.write'] }]
    const { result } = renderHook(() => useOrgStore(), { wrapper })
    expect(result.current.can('loyalty_config.write', 1, 5)).toBe(true)
    expect(result.current.can('loyalty_config.write', 1, 6)).toBe(false)
    // A store-scoped membership never grants an organization-level permission.
    expect(result.current.can('loyalty_config.write', 1)).toBe(false)
    expect(result.current.can('loyalty_config.write', 2, 5)).toBe(false)
  })

  it('keeps every other role inside its own organization', () => {
    me.memberships = [
      { organization_id: 1, store_id: null, role: 'retailer_admin', permissions: ['stores.manage'] },
    ]
    expect(permissionsInOrg2('stores.manage')).toBe(false)
  })
})

describe('OrgStoreContext store selection', () => {
  it('selects the only visible store, e.g. for a store-scoped manager', async () => {
    storeList.data = [{ id: 3, name: 'Океан' }]
    const { result } = renderHook(() => useOrgStore(), { wrapper })
    await waitFor(() => expect(result.current.selectedStoreID).toBe(3))
  })

  it('drops a remembered store that is no longer visible, and leaves several stores unselected', async () => {
    sessionStorage.setItem('promogo.admin.selectedOrgID', '1')
    sessionStorage.setItem('promogo.admin.selectedStoreID', '99')
    storeList.data = [{ id: 3, name: 'Океан' }, { id: 4, name: 'Море' }]
    const { result } = renderHook(() => useOrgStore(), { wrapper })
    await waitFor(() => expect(result.current.selectedStoreID).toBeNull())
  })
})
