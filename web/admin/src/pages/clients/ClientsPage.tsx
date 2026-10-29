import { useState, type FormEvent } from 'react'

import {
  responseOf,
  useClientLookup,
  useClientTransactions,
  useStoreClients,
  type AdminClient,
} from '../../api/queries'
import { NeedStoreState } from '../../components/NeedStoreState'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { formatMoney, formatPhone, formatPointsDelta, isMaskedPhone } from './format'

const t = strings.clients
const dateFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium' })
const dateTimeFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' })
const points = (n: number) => strings.loyalty.points(BigInt(n))

// ClientsPage is the support view of the current store's loyalty clients
// (docs/admin-web-plan.md 6.2): phone search, the paginated client list,
// and a client card with the operation history. Read-only — balance
// corrections aren't in the backend yet. support_viewer gets masked
// phones from the server; the UI only explains why.
export function ClientsPage() {
  const { selectedOrgID, selectedStoreID, stores } = useOrgStore()

  if (selectedOrgID == null || selectedStoreID == null) {
    return (
      <>
        <h1>{t.title}</h1>
        <NeedStoreState body={t.noStoreBody} />
      </>
    )
  }

  const store = stores.find((s) => s.id === selectedStoreID)
  // Remount per store so an open client card never carries over.
  return (
    <StoreClients
      key={`${selectedOrgID}:${selectedStoreID}`}
      orgID={selectedOrgID}
      storeID={selectedStoreID}
      storeName={store?.name ?? ''}
    />
  )
}

function StoreClients({ orgID, storeID, storeName }: { orgID: number; storeID: number; storeName: string }) {
  const { can } = useOrgStore()
  const canRead = can('clients.read', orgID, storeID)
  const canReadHistory = can('transactions.read', orgID, storeID)

  const [selected, setSelected] = useState<AdminClient | null>(null)
  const [searched, setSearched] = useState<string | null>(null)
  const lookup = useClientLookup(orgID, storeID, canRead ? searched : null)

  const heading = <h1>{storeName ? t.titleForStore(storeName) : t.title}</h1>

  if (!canRead) {
    return (
      <>
        {heading}
        <ForbiddenState />
      </>
    )
  }

  // A successful phone search opens the card straight away.
  const open = selected ?? (searched != null ? lookup.data : null) ?? null
  if (open) {
    return (
      <>
        {heading}
        <ClientCard
          orgID={orgID}
          storeID={storeID}
          client={open}
          canReadHistory={canReadHistory}
          onBack={() => {
            setSelected(null)
            setSearched(null)
          }}
        />
      </>
    )
  }

  return (
    <>
      {heading}
      <p className="muted page-intro">{t.intro}</p>

      <PhoneSearch pending={lookup.isFetching} searched={searched} onSearch={setSearched} onReset={() => setSearched(null)} />
      {searched != null && lookup.isError && (
        <ErrorState response={responseOf(lookup.error)} onRetry={() => void lookup.refetch()} />
      )}
      {searched != null && lookup.data === null && (
        <div role="alert" className="callout clients-not-found">
          <p>{t.search.notFound(searched)}</p>
          <p className="muted">{t.search.notFoundHint}</p>
        </div>
      )}

      <ClientList orgID={orgID} storeID={storeID} onOpen={setSelected} />
    </>
  )
}

function PhoneSearch({
  pending,
  searched,
  onSearch,
  onReset,
}: {
  pending: boolean
  searched: string | null
  onSearch: (phone: string) => void
  onReset: () => void
}) {
  const [value, setValue] = useState(searched ?? '')
  const [error, setError] = useState<string | null>(null)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const phone = value.trim()
    if (!/\d/.test(phone)) {
      setError(t.search.required)
      return
    }
    setError(null)
    onSearch(phone)
  }

  return (
    <form className="clients-search" role="search" onSubmit={submit} noValidate>
      <label htmlFor="client-phone">{t.search.label}</label>
      <div className="name-form__row">
        <input
          id="client-phone"
          type="text"
          inputMode="tel"
          autoComplete="off"
          placeholder={t.search.placeholder}
          value={value}
          aria-invalid={error != null}
          aria-describedby={error ? 'client-phone-error' : 'client-phone-hint'}
          onChange={(e) => setValue(e.target.value)}
        />
        <button type="submit" className="button-primary" disabled={pending}>
          {t.search.submit}
        </button>
        {searched != null && (
          <button
            type="button"
            onClick={() => {
              setValue('')
              setError(null)
              onReset()
            }}
          >
            {t.search.reset}
          </button>
        )}
      </div>
      {error ? (
        <p id="client-phone-error" className="field-error">
          {error}
        </p>
      ) : (
        <p id="client-phone-hint" className="field-hint">
          {t.search.hint}
        </p>
      )}
    </form>
  )
}

function ClientList({ orgID, storeID, onOpen }: { orgID: number; storeID: number; onOpen: (c: AdminClient) => void }) {
  const query = useStoreClients(orgID, storeID, true)

  if (query.isPending) return <LoadingState />
  if (query.isError) {
    return responseOf(query.error)?.status === 403 ? (
      <ForbiddenState />
    ) : (
      <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
    )
  }

  const clients = query.data.pages.flatMap((p) => p.clients ?? [])
  if (clients.length === 0) return <EmptyState title={t.empty} body={<p>{t.emptyBody}</p>} />

  return (
    <section className="page-section">
      {clients.some((c) => isMaskedPhone(c.phone)) && <p className="muted">{t.maskedNote}</p>}
      <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th scope="col">{t.columns.phone}</th>
            <th scope="col" className="cell-number">
              {t.columns.balance}
            </th>
            <th scope="col">{t.columns.app}</th>
            <th scope="col">{t.columns.createdAt}</th>
          </tr>
        </thead>
        <tbody>
          {clients.map((client) => {
            const phone = formatPhone(client.phone)
            return (
              <tr key={client.client_id}>
                <td>
                  <button type="button" className="link-button" aria-label={t.open(phone)} onClick={() => onOpen(client)}>
                    {phone}
                  </button>
                </td>
                <td className="cell-number">{client.balance.toLocaleString('ru-RU')}</td>
                <td>
                  <AppStatus connected={client.app_connected} />
                </td>
                <td className="cell-nowrap">{dateFormat.format(new Date(client.created_at))}</td>
              </tr>
            )
          })}
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
    </section>
  )
}

function AppStatus({ connected }: { connected: boolean }) {
  return connected ? (
    <span className="status status--active">{t.app.connected}</span>
  ) : (
    <span className="muted">{t.app.notConnected}</span>
  )
}

function ClientCard({
  orgID,
  storeID,
  client,
  canReadHistory,
  onBack,
}: {
  orgID: number
  storeID: number
  client: AdminClient
  canReadHistory: boolean
  onBack: () => void
}) {
  const history = useClientTransactions(orgID, storeID, client.client_id, canReadHistory)
  const d = t.detail

  return (
    <>
      <p className="client-back">
        <button type="button" className="link-button" onClick={onBack}>
          {d.back}
        </button>
      </p>
      <h2>{formatPhone(client.phone)}</h2>
      {isMaskedPhone(client.phone) && <p className="muted">{t.maskedNote}</p>}

      <dl className="client-summary">
        <div>
          <dt>{d.balance}</dt>
          <dd className="client-summary__balance">{points(client.balance)}</dd>
        </div>
        <div>
          <dt>{d.app}</dt>
          <dd>
            <AppStatus connected={client.app_connected} />
            <p className="field-hint">{client.app_connected ? d.appConnectedHint : d.appMissingHint}</p>
          </dd>
        </div>
        <div>
          <dt>{d.createdAt}</dt>
          <dd>{dateFormat.format(new Date(client.created_at))}</dd>
        </div>
      </dl>

      <section className="page-section">
        <h2>{d.history}</h2>
        {!canReadHistory ? (
          <p className="muted">{d.historyForbidden}</p>
        ) : history.isPending ? (
          <LoadingState />
        ) : history.isError ? (
          <ErrorState response={responseOf(history.error)} onRetry={() => void history.refetch()} />
        ) : history.data.length === 0 ? (
          <p className="muted">{d.historyEmpty}</p>
        ) : (
          <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">{d.columns.date}</th>
                <th scope="col">{d.columns.type}</th>
                <th scope="col" className="cell-number">
                  {d.columns.amount}
                </th>
                <th scope="col" className="cell-number">
                  {d.columns.points}
                </th>
                <th scope="col" className="cell-number">
                  {d.columns.balanceAfter}
                </th>
                <th scope="col">{d.columns.receipt}</th>
              </tr>
            </thead>
            <tbody>
              {history.data.map((tx) => (
                <tr key={tx.id}>
                  <td className="cell-nowrap">{dateTimeFormat.format(new Date(tx.created_at))}</td>
                  <td>{d.types[tx.type]}</td>
                  <td className="cell-number cell-nowrap">{formatMoney(tx.amount, tx.currency)}</td>
                  <td className="cell-number">{formatPointsDelta(tx.points_delta)}</td>
                  <td className="cell-number">{tx.balance_after.toLocaleString('ru-RU')}</td>
                  <td>
                    <code>{tx.external_tx_id}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        )}
      </section>
    </>
  )
}
