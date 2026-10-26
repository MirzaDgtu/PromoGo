import { useState } from 'react'

import {
  responseOf,
  useLoyaltyConfig,
  useLoyaltyConfigHistory,
  useRollbackLoyaltyConfig,
  useSaveLoyaltyConfig,
  type LoyaltyConfigVersion,
} from '../../api/queries'
import { NeedStoreState } from '../../components/NeedStoreState'
import { SubmitError } from '../../components/SubmitError'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { LoyaltyConfigForm, toConfigInput } from './LoyaltyConfigForm'

const t = strings.loyalty
const dateTimeFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' })

// LoyaltyConfigPage edits the current store's loyalty rules
// (docs/admin-web-plan.md 6.2): the live form, an immediate-effect warning
// (Q-P0-068 is still open, so saves apply at once), and the version
// history with forward-only rollback. Every write sends expected_version,
// so a save never silently overwrites someone else's newer change.
export function LoyaltyConfigPage() {
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
  return (
    <StoreLoyalty
      key={`${selectedOrgID}:${selectedStoreID}`}
      orgID={selectedOrgID}
      storeID={selectedStoreID}
      storeName={store?.name ?? ''}
    />
  )
}

function StoreLoyalty({ orgID, storeID, storeName }: { orgID: number; storeID: number; storeName: string }) {
  const { can } = useOrgStore()
  const canRead = can('loyalty_config.read', orgID, storeID)
  const canWrite = can('loyalty_config.write', orgID, storeID)
  const config = useLoyaltyConfig(orgID, storeID, canRead)
  const save = useSaveLoyaltyConfig(orgID, storeID)
  const [notice, setNotice] = useState<string | null>(null)

  const heading = <h1>{storeName ? t.titleForStore(storeName) : t.title}</h1>

  if (!canRead) {
    return (
      <>
        {heading}
        <ForbiddenState />
      </>
    )
  }
  if (config.isPending) {
    return (
      <>
        {heading}
        <LoadingState />
      </>
    )
  }
  if (config.isError) {
    return (
      <>
        {heading}
        {responseOf(config.error)?.status === 403 ? (
          <ForbiddenState />
        ) : (
          <ErrorState response={responseOf(config.error)} onRetry={() => void config.refetch()} />
        )}
      </>
    )
  }

  const current = config.data
  const currentVersion = current?.version ?? 0

  return (
    <>
      <div className="page-header">
        {heading}
        {current && <span className="badge">{t.version(currentVersion)}</span>}
      </div>

      {current == null ? (
        <div role="status" className="alert">
          <strong>{t.notConfigured}</strong>
          <p>{t.notConfiguredBody}</p>
        </div>
      ) : (
        canWrite && <p className="callout">{t.immediate}</p>
      )}

      {notice && (
        <p role="status" className="notice">
          {notice}
        </p>
      )}

      <LoyaltyConfigForm
        // A new version (after save, rollback or reload) remounts the form
        // with the server's values as its clean state.
        key={currentVersion}
        config={current}
        canWrite={canWrite}
        pending={save.isPending}
        error={save.error}
        onReload={() => {
          save.reset()
          setNotice(null)
          void config.refetch()
        }}
        onSubmit={async (values) => {
          setNotice(null)
          const saved = await save.mutateAsync(toConfigInput(values, currentVersion))
          save.reset()
          setNotice(t.saved(saved.version ?? currentVersion + 1))
        }}
      />

      {current != null && (
        <History
          orgID={orgID}
          storeID={storeID}
          currentVersion={currentVersion}
          canWrite={canWrite}
          onRestored={(from, to) => setNotice(t.history.restored(from, to))}
        />
      )}
    </>
  )
}

function History({
  orgID,
  storeID,
  currentVersion,
  canWrite,
  onRestored,
}: {
  orgID: number
  storeID: number
  currentVersion: number
  canWrite: boolean
  onRestored: (from: number, to: number) => void
}) {
  const history = useLoyaltyConfigHistory(orgID, storeID, true)
  const rollback = useRollbackLoyaltyConfig(orgID, storeID)
  const [confirmVersion, setConfirmVersion] = useState<number | null>(null)
  const h = t.history

  return (
    <section className="page-section" aria-labelledby="loyalty-history">
      <h2 id="loyalty-history">{h.title}</h2>
      {history.isPending ? (
        <LoadingState />
      ) : history.isError ? (
        <ErrorState response={responseOf(history.error)} onRetry={() => void history.refetch()} />
      ) : history.data.length === 0 ? (
        <EmptyState title={h.empty} />
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th scope="col">{h.version}</th>
              <th scope="col">{h.date}</th>
              <th scope="col">{h.author}</th>
              <th scope="col">{h.accrual}</th>
              <th scope="col">{h.redemption}</th>
              <th scope="col">
                <span className="visually-hidden">{strings.stores.actions}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {history.data.map((v) =>
              confirmVersion === v.version ? (
                <tr key={v.version} className="form-row">
                  <td colSpan={6}>
                    <div role="alertdialog" aria-label={h.restoreLabel(v.version)} className="confirm-row">
                      <span>{h.restoreConfirm(v.version)}</span>
                      <button
                        type="button"
                        className="button-primary"
                        autoFocus
                        disabled={rollback.isPending}
                        onClick={() =>
                          void rollback.mutateAsync({ version: v.version, expected_version: currentVersion }).then(
                            (cfg) => {
                              setConfirmVersion(null)
                              onRestored(v.version, cfg.version ?? currentVersion + 1)
                            },
                            () => {
                              // Rendered from rollback.error below.
                            },
                          )
                        }
                      >
                        {rollback.isPending ? strings.form.saving : h.restoreSubmit}
                      </button>
                      <button type="button" onClick={() => setConfirmVersion(null)}>
                        {strings.common.cancel}
                      </button>
                    </div>
                    {rollback.error != null &&
                      (responseOf(rollback.error)?.status === 409 ? (
                        <p role="alert" className="field-error">
                          {t.errors.conflict}
                        </p>
                      ) : (
                        <SubmitError error={rollback.error} />
                      ))}
                  </td>
                </tr>
              ) : (
                <HistoryRow
                  key={v.version}
                  v={v}
                  isCurrent={v.version === currentVersion}
                  canRestore={canWrite}
                  onRestore={() => {
                    rollback.reset()
                    setConfirmVersion(v.version)
                  }}
                />
              ),
            )}
          </tbody>
        </table>
      )}
    </section>
  )
}

function HistoryRow({
  v,
  isCurrent,
  canRestore,
  onRestore,
}: {
  v: LoyaltyConfigVersion
  isCurrent: boolean
  canRestore: boolean
  onRestore: () => void
}) {
  const h = t.history
  return (
    <tr aria-current={isCurrent ? 'true' : undefined}>
      <td>
        {v.version} {isCurrent && <span className="badge">{h.current}</span>}
      </td>
      <td>{dateTimeFormat.format(new Date(v.created_at))}</td>
      <td>{v.changed_by_staff_user_id != null ? h.authorStaff(v.changed_by_staff_user_id) : h.authorSystem}</td>
      <td>{h.accrualSummary(v.accrual_percent, v.min_purchase_amount)}</td>
      <td>{h.redemptionSummary(v.max_redeem_percent, v.points_exchange_rate, v.min_balance_to_redeem)}</td>
      <td className="cell-actions">
        {canRestore && !isCurrent && (
          <button type="button" aria-label={h.restoreLabel(v.version)} onClick={onRestore}>
            {h.restore}
          </button>
        )}
      </td>
    </tr>
  )
}
