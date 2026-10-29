import { useId, useState } from 'react'

import { responseOf, useAuditEvents, type AuditCategory } from '../../api/queries'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { describeActor, describeEvent } from './describe'

const t = strings.audit
const dateTimeFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' })
const CATEGORIES: AuditCategory[] = ['organization', 'store', 'staff', 'loyalty_config', 'api_key']

// AuditPage is the read-only audit trail of the selected organization
// (docs/admin-web-plan.md 6.2): newest first, filterable by event type,
// paged with "show more". Each event is rendered as a sentence from its
// action and metadata (see describe.ts).
export function AuditPage() {
  const { selectedOrgID, organizations } = useOrgStore()

  if (selectedOrgID == null) {
    return (
      <>
        <h1>{t.title}</h1>
        <EmptyState title={strings.orgStore.selectOrganization} body={<p>{t.noOrgBody}</p>} />
      </>
    )
  }

  const org = organizations.find((o) => o.id === selectedOrgID)
  return <OrgAudit key={selectedOrgID} orgID={selectedOrgID} orgName={org?.name ?? ''} />
}

function OrgAudit({ orgID, orgName }: { orgID: number; orgName: string }) {
  const { can, stores } = useOrgStore()
  const canRead = can('audit.read', orgID)
  const [category, setCategory] = useState<AuditCategory | null>(null)
  const query = useAuditEvents(orgID, category, canRead)
  const filterID = useId()

  const heading = <h1>{orgName ? t.titleForOrg(orgName) : t.title}</h1>
  if (!canRead) {
    return (
      <>
        {heading}
        <ForbiddenState />
      </>
    )
  }

  const storeName = (id: number) => {
    const name = stores.find((s) => s.id === id)?.name
    return name ? strings.staff.storeScope(name) : t.unknownStore(id)
  }
  const events = query.data?.pages.flatMap((p) => p.events) ?? []

  return (
    <>
      {heading}
      <p className="muted page-intro">{t.intro}</p>

      <div className="audit-filter">
        <label htmlFor={filterID}>{t.filter}</label>
        <select
          id={filterID}
          value={category ?? ''}
          onChange={(e) => setCategory((e.target.value || null) as AuditCategory | null)}
        >
          <option value="">{t.categories.all}</option>
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {t.categories[c]}
            </option>
          ))}
        </select>
      </div>

      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        responseOf(query.error)?.status === 403 ? (
          <ForbiddenState />
        ) : (
          <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
        )
      ) : events.length === 0 ? (
        <EmptyState title={category ? t.emptyFiltered : t.empty} />
      ) : (
        <>
          <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">{t.columns.when}</th>
                <th scope="col">{t.columns.who}</th>
                <th scope="col">{t.columns.what}</th>
                <th scope="col">{t.columns.store}</th>
                <th scope="col">{t.columns.ip}</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id}>
                  <td className="cell-nowrap">{dateTimeFormat.format(new Date(e.occurred_at))}</td>
                  <td className="cell-nowrap">{describeActor(e)}</td>
                  <td>{describeEvent(e, storeName)}</td>
                  <td>{e.store_id != null ? storeName(e.store_id) : <span className="muted">—</span>}</td>
                  <td>{e.ip ? <code>{e.ip}</code> : <span className="muted">—</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
          {query.hasNextPage && (
            <p className="load-more">
              <button type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
                {query.isFetchingNextPage ? t.loadingMore : t.loadMore}
              </button>
            </p>
          )}
        </>
      )}
    </>
  )
}
