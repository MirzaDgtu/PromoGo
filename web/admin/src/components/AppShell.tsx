import { NavLink, Outlet } from 'react-router-dom'

import { useAuth } from '../auth/AuthProvider'
import { strings } from '../i18n/strings'
import { useOrgStore } from '../session/OrgStoreContext'

// One nav entry per pilot admin module (docs/admin-web-plan.md 6.2). Each
// entry's requiredPermission gates visibility — hiding a link the caller
// can't use isn't the actual access control (the backend enforces that
// unconditionally), it's just not showing dead ends. loyalty_config.read is
// granted to every role including support_viewer, so "Программа лояльности"
// is effectively always visible to anyone with any membership in scope.
const NAV_ITEMS = [
  { to: '/', label: strings.nav.dashboard, permission: null },
  { to: '/organizations', label: strings.nav.organizations, permission: 'stores.read' },
  { to: '/staff', label: strings.nav.staff, permission: 'staff.manage' },
  { to: '/loyalty-config', label: strings.nav.loyaltyConfig, permission: 'loyalty_config.read' },
  { to: '/clients', label: strings.nav.clients, permission: 'clients.read' },
  { to: '/api-keys', label: strings.nav.apiKeys, permission: 'api_keys.read' },
  { to: '/audit', label: strings.nav.audit, permission: 'audit.read' },
] as const

export function AppShell() {
  const { logout } = useAuth()
  const { organizations, selectedOrgID, setSelectedOrgID, stores, selectedStoreID, setSelectedStoreID, hasPermission } =
    useOrgStore()

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '240px 1fr', minHeight: '100vh' }}>
      <nav aria-label="Основная навигация" style={{ borderRight: '1px solid var(--color-border)', padding: 'var(--space-4)' }}>
        <div style={{ marginBottom: 'var(--space-4)' }}>
          <label htmlFor="org-select">{strings.orgStore.organization}</label>
          <select
            id="org-select"
            value={selectedOrgID ?? ''}
            onChange={(e) => setSelectedOrgID(e.target.value ? Number(e.target.value) : null)}
          >
            {organizations.length === 0 && <option value="">{strings.orgStore.noOrganizations}</option>}
            {organizations.map((org) => (
              <option key={org.id} value={org.id}>
                {org.name}
              </option>
            ))}
          </select>

          <label htmlFor="store-select">{strings.orgStore.store}</label>
          <select
            id="store-select"
            value={selectedStoreID ?? ''}
            onChange={(e) => setSelectedStoreID(e.target.value ? Number(e.target.value) : null)}
          >
            <option value="">{strings.orgStore.allStores}</option>
            {stores.map((store) => (
              <option key={store.id} value={store.id}>
                {store.name}
              </option>
            ))}
          </select>
        </div>

        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          {NAV_ITEMS.filter((item) => item.permission === null || hasPermission(item.permission)).map((item) => (
            <li key={item.to}>
              <NavLink to={item.to} end={item.to === '/'}>
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>

        <button type="button" onClick={() => void logout()}>
          {strings.nav.logout}
        </button>
      </nav>

      <main style={{ padding: 'var(--space-6)' }}>
        <Outlet />
      </main>
    </div>
  )
}
