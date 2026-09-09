import { useQuery } from '@tanstack/react-query'

import { api } from './client'

// enabled defaults to true in every hook below except where noted — callers
// pass enabled: false while auth/org/store context isn't ready yet, so a
// query never fires with an id that doesn't exist yet.

export function useStaffMe(enabled: boolean) {
  return useQuery({
    queryKey: ['staffMe'],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/staff/me')
      if (error) throw Object.assign(new Error('load staff profile'), { response })
      return data
    },
    enabled,
  })
}

export function useOrganizations(enabled: boolean) {
  return useQuery({
    queryKey: ['organizations'],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations')
      if (error) throw Object.assign(new Error('load organizations'), { response })
      return data?.organizations ?? []
    },
    enabled,
  })
}

export function useStores(orgID: number | null) {
  return useQuery({
    queryKey: ['stores', orgID],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores', {
        params: { path: { orgID: orgID! } },
      })
      if (error) throw Object.assign(new Error('load stores'), { response })
      return data?.stores ?? []
    },
    enabled: orgID != null,
  })
}
