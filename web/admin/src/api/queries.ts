import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from './client'
import type { components } from './schema.gen'

export type StoreProfileInput = components['schemas']['StoreProfileInput']

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

function storesQuery(orgID: number) {
  return {
    queryKey: ['stores', orgID],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores', {
        params: { path: { orgID } },
      })
      if (error) throw Object.assign(new Error('load stores'), { response })
      return data?.stores ?? []
    },
  }
}

export function useStores(orgID: number | null, enabled: boolean) {
  return useQuery({
    ...storesQuery(orgID ?? 0),
    enabled: enabled && orgID != null,
  })
}

// useStoresByOrganization loads every listed organization's stores, one
// request per organization, sharing useStores' cache entries. Fine for the
// pilot's handful of organizations; a cross-organization list endpoint
// would replace it if that number grows.
export function useStoresByOrganization(orgIDs: number[]) {
  return useQueries({ queries: orgIDs.map((id) => storesQuery(id)) })
}

// Mutations await their list invalidation in onSuccess, so by the time
// mutateAsync resolves the caller's list already includes the new row.

export function useCreateOrganization() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (name: string) => {
      const { data, error, response } = await api.POST('/api/v1/admin/organizations', { body: { name } })
      if (error || !data) throw Object.assign(new Error('create organization'), { response })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['organizations'] }),
  })
}

export function useCreateStore(orgID: number) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: StoreProfileInput & { name: string }) => {
      const { data, error, response } = await api.POST('/api/v1/admin/organizations/{orgID}/stores', {
        params: { path: { orgID } },
        body: input,
      })
      if (error || !data) throw Object.assign(new Error('create store'), { response })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['stores', orgID] }),
  })
}

export function useUpdateStore(orgID: number, storeID: number) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: StoreProfileInput) => {
      const { data, error, response } = await api.PATCH('/api/v1/admin/organizations/{orgID}/stores/{storeID}', {
        params: { path: { orgID, storeID } },
        body: input,
      })
      if (error || !data) throw Object.assign(new Error('update store'), { response })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['stores', orgID] }),
  })
}

export type StoreAPIKey = components['schemas']['StoreAPIKey']
export type APIKeyScope = components['schemas']['APIKeyScope']

export function useStoreAPIKeys(orgID: number | null, storeID: number | null, enabled: boolean) {
  return useQuery({
    queryKey: ['apiKeys', orgID, storeID],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys', {
        params: { path: { orgID: orgID!, storeID: storeID! } },
      })
      if (error) throw Object.assign(new Error('load api keys'), { response })
      return data?.api_keys ?? []
    },
    enabled: enabled && orgID != null && storeID != null,
  })
}

// The create response carries the one-time plaintext key. gcTime: 0 drops
// it from the mutation cache as soon as the caller resets the mutation, so
// the secret doesn't linger in memory after the one-time dialog closes.
export function useCreateStoreAPIKey(orgID: number, storeID: number) {
  const queryClient = useQueryClient()
  return useMutation({
    gcTime: 0,
    mutationFn: async (input: { name: string; scopes: APIKeyScope[]; expires_at?: string }) => {
      const { data, error, response } = await api.POST('/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys', {
        params: { path: { orgID, storeID } },
        body: input,
      })
      if (error || !data) throw Object.assign(new Error('create api key'), { response })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['apiKeys', orgID, storeID] }),
  })
}

export function useRevokeStoreAPIKey(orgID: number, storeID: number) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (keyID: number) => {
      const { error, response } = await api.POST(
        '/api/v1/admin/organizations/{orgID}/stores/{storeID}/api-keys/{keyID}/revoke',
        { params: { path: { orgID, storeID, keyID } } },
      )
      if (error) throw Object.assign(new Error('revoke api key'), { response })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['apiKeys', orgID, storeID] }),
  })
}

export type LoyaltyConfig = components['schemas']['LoyaltyConfig']
export type LoyaltyConfigVersion = components['schemas']['LoyaltyConfigVersion']

// useLoyaltyConfig resolves to null when the store has no configuration
// yet (404) — an expected onboarding state, not an error.
export function useLoyaltyConfig(orgID: number, storeID: number, enabled: boolean) {
  return useQuery({
    queryKey: ['loyaltyConfig', orgID, storeID],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config', {
        params: { path: { orgID, storeID } },
      })
      if (response.status === 404) return null
      if (error || !data) throw Object.assign(new Error('load loyalty config'), { response })
      return data
    },
    enabled,
  })
}

export function useLoyaltyConfigHistory(orgID: number, storeID: number, enabled: boolean) {
  return useQuery({
    queryKey: ['loyaltyConfigHistory', orgID, storeID],
    queryFn: async () => {
      const { data, error, response } = await api.GET(
        '/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config/history',
        { params: { path: { orgID, storeID } } },
      )
      if (error) throw Object.assign(new Error('load loyalty config history'), { response })
      return data?.history ?? []
    },
    enabled,
  })
}

function useInvalidateLoyalty(orgID: number, storeID: number) {
  const queryClient = useQueryClient()
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['loyaltyConfig', orgID, storeID] }),
      queryClient.invalidateQueries({ queryKey: ['loyaltyConfigHistory', orgID, storeID] }),
    ])
}

export function useSaveLoyaltyConfig(orgID: number, storeID: number) {
  const invalidate = useInvalidateLoyalty(orgID, storeID)
  return useMutation({
    mutationFn: async (input: LoyaltyConfig) => {
      const { data, error, response } = await api.PUT('/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config', {
        params: { path: { orgID, storeID } },
        body: input,
      })
      if (error || !data) throw Object.assign(new Error('save loyalty config'), { response })
      return data
    },
    onSuccess: invalidate,
  })
}

export function useRollbackLoyaltyConfig(orgID: number, storeID: number) {
  const invalidate = useInvalidateLoyalty(orgID, storeID)
  return useMutation({
    mutationFn: async (input: { version: number; expected_version: number }) => {
      const { data, error, response } = await api.POST(
        '/api/v1/admin/organizations/{orgID}/stores/{storeID}/loyalty-config/rollback',
        { params: { path: { orgID, storeID } }, body: input },
      )
      if (error || !data) throw Object.assign(new Error('rollback loyalty config'), { response })
      return data
    },
    onSuccess: invalidate,
  })
}

export type AdminClient = components['schemas']['AdminClient']
export type AdminTransaction = components['schemas']['AdminTransaction']

// useStoreClients pages through the store's clients with the backend's
// keyset cursor; fetchNextPage appends the next page.
export function useStoreClients(orgID: number, storeID: number, enabled: boolean) {
  return useInfiniteQuery({
    queryKey: ['clients', orgID, storeID],
    queryFn: async ({ pageParam }) => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores/{storeID}/clients', {
        params: { path: { orgID, storeID }, query: pageParam ? { cursor: pageParam } : {} },
      })
      if (error || !data) throw Object.assign(new Error('load clients'), { response })
      return data
    },
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled,
  })
}

// useClientLookup resolves to null when no client has that phone in this
// store (404) — a normal search outcome, not an error. phone is null until
// the user submits a search.
export function useClientLookup(orgID: number, storeID: number, phone: string | null) {
  return useQuery({
    queryKey: ['clientLookup', orgID, storeID, phone],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/lookup', {
        params: { path: { orgID, storeID }, query: { phone: phone! } },
      })
      if (response.status === 404) return null
      if (error || !data) throw Object.assign(new Error('lookup client'), { response })
      return data
    },
    enabled: phone != null,
  })
}

export function useClientTransactions(orgID: number, storeID: number, clientID: number, enabled: boolean) {
  return useQuery({
    queryKey: ['clientTransactions', orgID, storeID, clientID],
    queryFn: async () => {
      const { data, error, response } = await api.GET(
        '/api/v1/admin/organizations/{orgID}/stores/{storeID}/clients/{clientID}/transactions',
        { params: { path: { orgID, storeID, clientID } } },
      )
      if (error) throw Object.assign(new Error('load client transactions'), { response })
      return data?.transactions ?? []
    },
    enabled,
  })
}

// responseOf recovers the Response attached by the hooks above, so error UI
// can branch on status and show the request id.
export function responseOf(error: unknown): Response | undefined {
  if (error && typeof error === 'object' && 'response' in error && error.response instanceof Response) {
    return error.response
  }
  return undefined
}
