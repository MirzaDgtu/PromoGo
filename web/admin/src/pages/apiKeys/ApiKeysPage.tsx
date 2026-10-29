import { useState } from 'react'

import {
  responseOf,
  useCreateStoreAPIKey,
  useRevokeStoreAPIKey,
  useStoreAPIKeys,
  type StoreAPIKey,
} from '../../api/queries'
import { NeedStoreState } from '../../components/NeedStoreState'
import { OneTimeSecretDialog } from '../../components/OneTimeSecretDialog'
import { SubmitError } from '../../components/SubmitError'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { ApiKeyForm } from './ApiKeyForm'

const t = strings.apiKeys
const dateFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium' })
const dateTimeFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' })

type KeyStatus = 'active' | 'expired' | 'revoked'

export function keyStatus(key: StoreAPIKey, now = new Date()): KeyStatus {
  if (key.revoked_at) return 'revoked'
  if (key.expires_at && new Date(key.expires_at) <= now) return 'expired'
  return 'active'
}

// ApiKeysPage manages the current store's API keys (docs/admin-web-plan.md
// 6.2): list, issue with a one-time plaintext display, and revoke. The
// plaintext exists only in this page's state between creation and the
// dialog closing — it's never stored, logged, or refetchable.
export function ApiKeysPage() {
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
  // Remount per store so an open form or revoke confirmation never carries
  // over to a different store.
  return (
    <StoreApiKeys
      key={`${selectedOrgID}:${selectedStoreID}`}
      orgID={selectedOrgID}
      storeID={selectedStoreID}
      storeName={store?.name ?? ''}
    />
  )
}

function StoreApiKeys({ orgID, storeID, storeName }: { orgID: number; storeID: number; storeName: string }) {
  const { can } = useOrgStore()
  const canRead = can('api_keys.read', orgID, storeID)
  const canManage = can('api_keys.rotate', orgID, storeID)
  const query = useStoreAPIKeys(orgID, storeID, canRead)
  const create = useCreateStoreAPIKey(orgID, storeID)
  const revoke = useRevokeStoreAPIKey(orgID, storeID)

  const [issuing, setIssuing] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)
  const [confirmRevokeID, setConfirmRevokeID] = useState<number | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  return (
    <>
      <div className="page-header">
        <h1>{storeName ? t.titleForStore(storeName) : t.title}</h1>
        {canManage && !issuing && (
          <button
            type="button"
            onClick={() => {
              setNotice(null)
              create.reset()
              setIssuing(true)
            }}
          >
            {t.issue}
          </button>
        )}
      </div>
      <p className="muted page-intro">{t.intro}</p>

      {issuing && (
        <ApiKeyForm
          pending={create.isPending}
          error={create.error}
          onCancel={() => setIssuing(false)}
          onSubmit={async (input) => {
            const created = await create.mutateAsync(input)
            setIssuing(false)
            setSecret(created.plaintext_key)
          }}
        />
      )}

      {secret != null && (
        <OneTimeSecretDialog
          secret={secret}
          onClose={() => {
            setSecret(null)
            // Drops the response (and the plaintext in it) from the mutation cache.
            create.reset()
          }}
        />
      )}

      {notice && (
        <p role="status" className="notice">
          {notice}
        </p>
      )}

      {!canRead ? (
        <ForbiddenState />
      ) : query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        responseOf(query.error)?.status === 403 ? (
          <ForbiddenState />
        ) : (
          <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
        )
      ) : query.data.length === 0 ? (
        <EmptyState title={t.empty} body={<p>{t.emptyBody}</p>} />
      ) : (
        <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th scope="col">{t.columns.name}</th>
              <th scope="col">{t.columns.keyID}</th>
              <th scope="col">{t.columns.scopes}</th>
              <th scope="col">{t.columns.createdAt}</th>
              <th scope="col">{t.columns.lastUsedAt}</th>
              <th scope="col">{t.columns.expiresAt}</th>
              <th scope="col">{t.columns.status}</th>
              <th scope="col">
                <span className="visually-hidden">{t.columns.actions}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {query.data.map((key) => {
              const status = keyStatus(key)
              if (confirmRevokeID === key.id) {
                return (
                  <tr key={key.id} className="form-row">
                    <td colSpan={8}>
                      <div role="alertdialog" aria-label={t.revokeLabel(key.name)} className="confirm-row">
                        <span>{t.revokeConfirm(key.name)}</span>
                        <button
                          type="button"
                          className="button-danger"
                          disabled={revoke.isPending}
                          autoFocus
                          onClick={() =>
                            void revoke.mutateAsync(key.id).then(
                              () => {
                                setConfirmRevokeID(null)
                                setNotice(t.revoked(key.name))
                              },
                              () => {
                                // Rendered from revoke.error below.
                              },
                            )
                          }
                        >
                          {revoke.isPending ? strings.form.saving : t.revokeSubmit}
                        </button>
                        <button type="button" onClick={() => setConfirmRevokeID(null)}>
                          {strings.common.cancel}
                        </button>
                      </div>
                      {revoke.error != null && <SubmitError error={revoke.error} />}
                    </td>
                  </tr>
                )
              }
              return (
                <tr key={key.id} className={status === 'active' ? undefined : 'row-inactive'}>
                  <td>{key.name}</td>
                  <td>
                    <code>{key.key_id}</code>
                  </td>
                  <td>
                    <ul className="scope-list">
                      {key.scopes.map((scope) => (
                        <li key={scope} title={t.scopes[scope].description}>
                          {t.scopes[scope].label}
                        </li>
                      ))}
                    </ul>
                  </td>
                  <td className="cell-nowrap">{dateFormat.format(new Date(key.created_at))}</td>
                  <td className="cell-nowrap">{key.last_used_at ? dateTimeFormat.format(new Date(key.last_used_at)) : <span className="muted">{t.never}</span>}</td>
                  <td className="cell-nowrap">{key.expires_at ? dateFormat.format(new Date(key.expires_at)) : <span className="muted">{t.noExpiry}</span>}</td>
                  <td>
                    <span className={`status status--${status}`}>{t.status[status]}</span>
                  </td>
                  <td className="cell-actions">
                    {canManage && status !== 'revoked' && (
                      <button
                        type="button"
                        aria-label={t.revokeLabel(key.name)}
                        onClick={() => {
                          setNotice(null)
                          revoke.reset()
                          setConfirmRevokeID(key.id)
                        }}
                      >
                        {t.revoke}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
        </div>
      )}
    </>
  )
}
