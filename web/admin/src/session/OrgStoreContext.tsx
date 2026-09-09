import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'

import { useOrganizations, useStaffMe, useStores } from '../api/queries'
import type { components } from '../api/schema.gen'

type Organization = components['schemas']['Organization']
type Store = components['schemas']['Store']
type StaffMeMembership = components['schemas']['StaffMeMembership']

interface OrgStoreContextValue {
  organizations: Organization[]
  organizationsLoading: boolean
  selectedOrgID: number | null
  setSelectedOrgID: (id: number | null) => void
  stores: Store[]
  storesLoading: boolean
  selectedStoreID: number | null
  setSelectedStoreID: (id: number | null) => void
  /** The caller's active memberships, for permission-aware nav. */
  memberships: StaffMeMembership[]
  hasPermission: (permission: string) => boolean
}

const OrgStoreContext = createContext<OrgStoreContextValue | null>(null)

export function useOrgStore(): OrgStoreContextValue {
  const ctx = useContext(OrgStoreContext)
  if (!ctx) throw new Error('useOrgStore must be used within OrgStoreProvider')
  return ctx
}

// Selected org/store is a per-viewer UX convenience (which tenant you were
// last looking at), not anything security-sensitive — every actual
// permission check happens server-side against the caller's real
// memberships, so persisting it to sessionStorage (not the access token
// storage restriction, which only applies to credentials) is fine.
const ORG_STORAGE_KEY = 'promogo.admin.selectedOrgID'
const STORE_STORAGE_KEY = 'promogo.admin.selectedStoreID'

function readStoredID(key: string): number | null {
  const raw = sessionStorage.getItem(key)
  const parsed = raw ? Number(raw) : NaN
  return Number.isFinite(parsed) ? parsed : null
}

export function OrgStoreProvider({ children, enabled }: { children: ReactNode; enabled: boolean }) {
  const staffMe = useStaffMe(enabled)
  const memberships = useMemo(() => staffMe.data?.memberships ?? [], [staffMe.data])

  const organizationsQuery = useOrganizations(enabled)
  const organizations = useMemo(() => organizationsQuery.data ?? [], [organizationsQuery.data])

  const [selectedOrgID, setSelectedOrgIDState] = useState<number | null>(() => readStoredID(ORG_STORAGE_KEY))
  const [selectedStoreID, setSelectedStoreIDState] = useState<number | null>(() => readStoredID(STORE_STORAGE_KEY))

  const setSelectedOrgID = useCallback((id: number | null) => {
    setSelectedOrgIDState(id)
    setSelectedStoreIDState(null)
    if (id == null) sessionStorage.removeItem(ORG_STORAGE_KEY)
    else sessionStorage.setItem(ORG_STORAGE_KEY, String(id))
    sessionStorage.removeItem(STORE_STORAGE_KEY)
  }, [])

  const setSelectedStoreID = useCallback((id: number | null) => {
    setSelectedStoreIDState(id)
    if (id == null) sessionStorage.removeItem(STORE_STORAGE_KEY)
    else sessionStorage.setItem(STORE_STORAGE_KEY, String(id))
  }, [])

  // If nothing is selected yet (first login) or the previously selected org
  // is no longer visible to this caller, default to the first organization
  // they can see rather than an empty selector.
  useEffect(() => {
    if (organizations.length === 0) return
    if (selectedOrgID != null && organizations.some((o) => o.id === selectedOrgID)) return
    setSelectedOrgID(organizations[0]?.id ?? null)
    // Only re-run when the visible organization list changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [organizations])

  const storesQuery = useStores(selectedOrgID)
  const stores = useMemo(() => storesQuery.data ?? [], [storesQuery.data])

  const hasPermission = useCallback(
    (permission: string) => {
      if (selectedOrgID == null) return false
      return memberships.some((m) => {
        if (m.organization_id !== selectedOrgID) return false
        if (selectedStoreID != null && m.store_id != null && m.store_id !== selectedStoreID) return false
        return (m.permissions ?? []).includes(permission)
      })
    },
    [memberships, selectedOrgID, selectedStoreID],
  )

  const value = useMemo<OrgStoreContextValue>(
    () => ({
      organizations,
      organizationsLoading: organizationsQuery.isLoading,
      selectedOrgID,
      setSelectedOrgID,
      stores,
      storesLoading: storesQuery.isLoading,
      selectedStoreID,
      setSelectedStoreID,
      memberships,
      hasPermission,
    }),
    [
      organizations,
      organizationsQuery.isLoading,
      selectedOrgID,
      setSelectedOrgID,
      stores,
      storesQuery.isLoading,
      selectedStoreID,
      setSelectedStoreID,
      memberships,
      hasPermission,
    ],
  )

  return <OrgStoreContext.Provider value={value}>{children}</OrgStoreContext.Provider>
}
