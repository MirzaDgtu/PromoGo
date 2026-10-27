import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

import { responseOf, useStoreOverview, useStoreOverviews, type StoreOverview } from '../../api/queries'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { formatMoney } from '../clients/format'
import { registerCheck, registerState, relativeTime, type CheckState } from './status'

const t = strings.overview

// OverviewPage is the Admin Web landing screen (docs/admin-web-plan.md
// 6.2, DEC-031). With a store selected it shows that store's onboarding
// checklist — loyalty program, cash-register key, sales — and its recent
// traffic; with "all stores" it lists every store's status, so the one
// that needs attention stands out.
export function OverviewPage() {
  const { selectedOrgID, selectedStoreID, organizations, stores, storesLoading } = useOrgStore()

  if (selectedOrgID == null) {
    return (
      <>
        <h1>{t.title}</h1>
        <EmptyState title={strings.orgStore.selectOrganization} body={<p>{t.noOrgBody}</p>} />
      </>
    )
  }

  if (selectedStoreID != null) {
    const store = stores.find((s) => s.id === selectedStoreID)
    return (
      <StoreOverviewView
        key={`${selectedOrgID}:${selectedStoreID}`}
        orgID={selectedOrgID}
        storeID={selectedStoreID}
        storeName={store?.name ?? ''}
      />
    )
  }

  const org = organizations.find((o) => o.id === selectedOrgID)
  return (
    <>
      <h1>{org?.name ? t.titleForOrg(org.name) : t.title}</h1>
      {storesLoading ? (
        <LoadingState />
      ) : stores.length === 0 ? (
        <EmptyState
          title={t.noStores}
          body={
            <p>
              {t.noStoresBody} <Link to="/organizations">{t.toStores}</Link>
            </p>
          }
        />
      ) : (
        <OrganizationOverview orgID={selectedOrgID} />
      )}
    </>
  )
}

function Check({ state, title, children }: { state: CheckState; title: string; children: ReactNode }) {
  const icon = state === 'ok' ? '✓' : state === 'warn' ? '!' : 'i'
  return (
    <li className={`check check--${state}`}>
      <span className="check__icon" aria-hidden="true">
        {icon}
      </span>
      <div>
        <h3>
          {title} <span className="visually-hidden">— {t.status[state]}</span>
        </h3>
        {children}
      </div>
    </li>
  )
}

function StoreOverviewView({ orgID, storeID, storeName }: { orgID: number; storeID: number; storeName: string }) {
  const { can } = useOrgStore()
  const query = useStoreOverview(orgID, storeID)
  const heading = <h1>{storeName ? t.titleForStore(storeName) : t.title}</h1>

  if (query.isPending) {
    return (
      <>
        {heading}
        <LoadingState />
      </>
    )
  }
  if (query.isError) {
    return (
      <>
        {heading}
        {responseOf(query.error)?.status === 403 ? (
          <ForbiddenState />
        ) : (
          <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
        )}
      </>
    )
  }

  const o = query.data
  const register = registerState(o)
  const money = (amount: string) => formatMoney(amount, 'RUB')

  return (
    <>
      {heading}

      <section aria-labelledby="overview-checklist">
        <h2 id="overview-checklist">{t.checklist}</h2>
        <ul className="checklist">
          <Check state={o.loyalty_config.configured ? 'ok' : 'warn'} title={t.loyalty.title}>
            <p>{o.loyalty_config.configured ? t.loyalty.ok(o.loyalty_config.version ?? 0) : t.loyalty.missing}</p>
            {!o.loyalty_config.configured && can('loyalty_config.write', orgID, storeID) && (
              <Link to="/loyalty-config">{t.loyalty.action}</Link>
            )}
          </Check>

          {register.kind !== 'hidden' && (
            <Check state={registerCheck(register)} title={t.register.title}>
              <p>
                {register.kind === 'connected'
                  ? t.register.ok(relativeTime(register.lastUsedAt))
                  : register.kind === 'stale'
                    ? t.register.stale(relativeTime(register.lastUsedAt))
                    : register.kind === 'never_used'
                      ? t.register.neverUsed
                      : t.register.noKey}
              </p>
              {register.kind !== 'connected' && <Link to="/api-keys">{t.register.action}</Link>}
            </Check>
          )}

          <Check state={o.last_transaction_at ? 'ok' : 'info'} title={t.sales.title}>
            <p>{o.last_transaction_at ? t.sales.ok(relativeTime(o.last_transaction_at)) : t.sales.none}</p>
          </Check>
        </ul>
      </section>

      <section className="page-section" aria-labelledby="overview-activity">
        <h2 id="overview-activity">{t.activity}</h2>
        <p>
          {t.clients(o.clients)}
          {o.clients > 0 && can('clients.read', orgID, storeID) && (
            <>
              {' '}
              <Link to="/clients">{t.toClients}</Link>
            </>
          )}
        </p>
        <table className="data-table activity-table">
          <thead>
            <tr>
              <th scope="col">{t.metrics.metric}</th>
              <th scope="col" className="cell-number">
                {t.metrics.last24h}
              </th>
              <th scope="col" className="cell-number">
                {t.metrics.last7d}
              </th>
            </tr>
          </thead>
          <tbody>
            <MetricRow label={t.metrics.operations} a={o.last_24h.operations} b={o.last_7d.operations} />
            <MetricRow label={t.metrics.salesAmount} a={money(o.last_24h.sales_amount)} b={money(o.last_7d.sales_amount)} />
            <MetricRow label={t.metrics.pointsAccrued} a={o.last_24h.points_accrued} b={o.last_7d.points_accrued} />
            <MetricRow label={t.metrics.pointsRedeemed} a={o.last_24h.points_redeemed} b={o.last_7d.points_redeemed} />
          </tbody>
        </table>
      </section>
    </>
  )
}

function MetricRow({ label, a, b }: { label: string; a: number | string; b: number | string }) {
  const fmt = (v: number | string) => (typeof v === 'number' ? v.toLocaleString('ru-RU') : v)
  return (
    <tr>
      <th scope="row">{label}</th>
      <td className="cell-number">{fmt(a)}</td>
      <td className="cell-number">{fmt(b)}</td>
    </tr>
  )
}

function StatusCell({ state, text }: { state: CheckState; text: string }) {
  return <span className={`status status--${state === 'ok' ? 'active' : state === 'warn' ? 'warn' : 'revoked'}`}>{text}</span>
}

function OrganizationOverview({ orgID }: { orgID: number }) {
  const { stores, setSelectedStoreID } = useOrgStore()
  const results = useStoreOverviews(
    orgID,
    stores.map((s) => s.id),
  )

  return (
    <>
      <p className="muted page-intro">{t.orgIntro}</p>
      <table className="data-table">
        <thead>
          <tr>
            <th scope="col">{t.columns.store}</th>
            <th scope="col">{t.columns.loyalty}</th>
            <th scope="col">{t.columns.register}</th>
            <th scope="col">{t.columns.lastOperation}</th>
            <th scope="col" className="cell-number">
              {t.columns.operations7d}
            </th>
          </tr>
        </thead>
        <tbody>
          {stores.map((store, i) => {
            const r = results[i]
            const o: StoreOverview | undefined = r?.data
            return (
              <tr key={store.id}>
                <td>
                  <button
                    type="button"
                    className="link-button"
                    aria-label={t.open(store.name ?? '')}
                    onClick={() => setSelectedStoreID(store.id)}
                  >
                    {store.name}
                  </button>
                </td>
                {!o ? (
                  <td colSpan={4} className="muted">
                    {r?.isError ? strings.serverError.title : strings.common.loading}
                  </td>
                ) : (
                  <>
                    <td>
                      <StatusCell
                        state={o.loyalty_config.configured ? 'ok' : 'warn'}
                        text={o.loyalty_config.configured ? t.cell.configured : t.cell.notConfigured}
                      />
                    </td>
                    <td>
                      <RegisterCell overview={o} />
                    </td>
                    <td>{o.last_transaction_at ? relativeTime(o.last_transaction_at) : <span className="muted">{t.cell.never}</span>}</td>
                    <td className="cell-number">{o.last_7d.operations.toLocaleString('ru-RU')}</td>
                  </>
                )}
              </tr>
            )
          })}
        </tbody>
      </table>
    </>
  )
}

function RegisterCell({ overview }: { overview: StoreOverview }) {
  const state = registerState(overview)
  switch (state.kind) {
    case 'hidden':
      return <span className="muted">{t.cell.hidden}</span>
    case 'no_key':
      return <StatusCell state="warn" text={t.cell.noKey} />
    case 'never_used':
      return <StatusCell state="warn" text={t.cell.neverUsed} />
    default:
      return <StatusCell state={registerCheck(state)} text={`${t.cell.connected}, ${relativeTime(state.lastUsedAt)}`} />
  }
}
