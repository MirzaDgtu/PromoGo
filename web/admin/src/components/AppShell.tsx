import { useEffect, useState, type ComponentType, type SVGProps } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'

import { useStoreOverview } from '../api/queries'
import { useAuth } from '../auth/AuthProvider'
import { strings } from '../i18n/strings'
import { registerCheck, registerState, relativeTime } from '../pages/overview/status'
import { useOrgStore } from '../session/OrgStoreContext'
import {
  AuditIcon,
  BrandMark,
  ChevronIcon,
  ClientsIcon,
  KeyIcon,
  LogoutIcon,
  LoyaltyIcon,
  MenuIcon,
  OverviewIcon,
  StaffIcon,
  StoresIcon,
} from './icons'

// One nav entry per pilot admin module (docs/admin-web-plan.md 6.2). Each
// entry's requiredPermission gates visibility — hiding a link the caller
// can't use isn't the actual access control (the backend enforces that
// unconditionally), it's just not showing dead ends. loyalty_config.read is
// granted to every role including support_viewer, so "Программа лояльности"
// is effectively always visible to anyone with any membership in scope.
const NAV_ITEMS: readonly {
  to: string
  label: string
  permission: string | null
  icon: ComponentType<SVGProps<SVGSVGElement>>
}[] = [
  { to: '/', label: strings.nav.dashboard, permission: null, icon: OverviewIcon },
  { to: '/organizations', label: strings.nav.organizations, permission: 'stores.read', icon: StoresIcon },
  { to: '/staff', label: strings.nav.staff, permission: 'staff.manage', icon: StaffIcon },
  { to: '/loyalty-config', label: strings.nav.loyaltyConfig, permission: 'loyalty_config.read', icon: LoyaltyIcon },
  { to: '/clients', label: strings.nav.clients, permission: 'clients.read', icon: ClientsIcon },
  { to: '/api-keys', label: strings.nav.apiKeys, permission: 'api_keys.read', icon: KeyIcon },
  { to: '/audit', label: strings.nav.audit, permission: 'audit.read', icon: AuditIcon },
]

// AppShell is the register's body: a tinted L-frame (sidebar + top bar)
// around the white sheet where each screen works. The top bar carries the
// current scope (organization / store) and, for a selected store, the
// signal light of its cash-register exchange — the one thing both a
// retailer and a platform admin check first.
export function AppShell() {
  const { logout, identity } = useAuth()
  const { organizations, selectedOrgID, setSelectedOrgID, stores, selectedStoreID, setSelectedStoreID, hasPermission } =
    useOrgStore()
  const location = useLocation()
  // The drawer remembers where it was opened, so any navigation closes it:
  // a drawer that outlives its navigation is a trap.
  const [navOpenAt, setNavOpenAt] = useState<string | null>(null)
  const navOpen = navOpenAt === location.pathname
  const setNavOpen = (open: boolean) => setNavOpenAt(open ? location.pathname : null)

  useEffect(() => {
    if (!navOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setNavOpenAt(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navOpen])

  return (
    <div className={navOpen ? 'shell shell--nav-open' : 'shell'}>
      <nav id="app-nav" aria-label="Основная навигация" className="shell__sidebar">
        <Link to="/" className="brand">
          <BrandMark className="brand__mark" />
          <span className="brand__name">PromoGo</span>
          <span className="brand__product">{strings.nav.product}</span>
        </Link>

        <ul className="nav">
          {NAV_ITEMS.filter((item) => item.permission === null || hasPermission(item.permission)).map((item) => (
            <li key={item.to}>
              <NavLink to={item.to} end={item.to === '/'}>
                <item.icon className="nav__icon" />
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>

        <div className="shell__account">
          {identity && (identity.name || identity.email) && (
            <div className="shell__who">
              {identity.name && <span>{identity.name}</span>}
              {identity.email && <span className="muted">{identity.email}</span>}
            </div>
          )}
          <button type="button" onClick={() => void logout()}>
            <LogoutIcon className="nav__icon" />
            {strings.nav.logout}
          </button>
        </div>
      </nav>

      {navOpen && <div className="shell__scrim" aria-hidden="true" onClick={() => setNavOpen(false)} />}

      <header className="topbar">
        <button
          type="button"
          className="icon-button menu-button"
          aria-label={strings.nav.menu}
          aria-expanded={navOpen}
          aria-controls="app-nav"
          onClick={() => setNavOpen(!navOpen)}
        >
          <MenuIcon className="nav__icon" />
        </button>

        <div className="scope">
          <div className="scope__field">
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
          </div>

          <ChevronIcon className="scope__sep" />

          <div className="scope__field">
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
        </div>

        {selectedOrgID != null && selectedStoreID != null && (
          <ExchangeLight key={`${selectedOrgID}:${selectedStoreID}`} orgID={selectedOrgID} storeID={selectedStoreID} />
        )}
      </header>

      <main className="sheet">
        <div key={location.pathname} className="page">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

// ExchangeLight is the store's register status in the top bar: the same
// overview the Overview screen loads (shared cache), reduced to one light
// and one line, linking to the API keys where a broken exchange is fixed.
function ExchangeLight({ orgID, storeID }: { orgID: number; storeID: number }) {
  const query = useStoreOverview(orgID, storeID)
  if (!query.data) return null

  const state = registerState(query.data)
  if (state.kind === 'hidden') return null

  const t = strings.overview
  const detail =
    state.kind === 'no_key'
      ? t.cell.noKey
      : state.kind === 'never_used'
        ? t.cell.neverUsed
        : state.kind === 'stale'
          ? `${t.status.warn}, ${relativeTime(state.lastUsedAt)}`
          : `${t.cell.connected} ${relativeTime(state.lastUsedAt)}`

  return (
    <Link to="/api-keys" className="exchange">
      <span className={`exchange__light exchange__light--${registerCheck(state)}`} aria-hidden="true" />
      <span>
        <strong>{t.register.title}</strong> · {detail}
      </span>
    </Link>
  )
}
