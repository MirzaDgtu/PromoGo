import { renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { OrgStoreProvider, useOrgStore } from './OrgStoreContext'

const me = { memberships: [] as unknown[] }

vi.mock('../api/queries', () => ({
  useStaffMe: () => ({ data: me }),
  useOrganizations: () => ({ data: [{ id: 1, name: 'Home' }, { id: 2, name: 'Other' }], isLoading: false }),
  useStores: () => ({ data: [], isLoading: false }),
}))

function wrapper({ children }: { children: ReactNode }) {
  return <OrgStoreProvider enabled>{children}</OrgStoreProvider>
}

function permissionsInOrg2(permission: string): boolean {
  sessionStorage.setItem('promogo.admin.selectedOrgID', '2')
  const { result } = renderHook(() => useOrgStore(), { wrapper })
  return result.current.hasPermission(permission)
}

beforeEach(() => sessionStorage.clear())

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
