import { useState } from 'react'

import {
  responseOf,
  useCreateStaffMembership,
  useStaffMemberships,
  useUpdateStaffMembership,
  type StaffMembership,
  type StaffRole,
} from '../../api/queries'
import { useAuth } from '../../auth/AuthProvider'
import { SubmitError } from '../../components/SubmitError'
import { EmptyState, ErrorState, ForbiddenState, LoadingState } from '../../components/states'
import { strings } from '../../i18n/strings'
import { useOrgStore } from '../../session/OrgStoreContext'
import { grantableRoles, StaffForm } from './StaffForm'

const t = strings.staff

// who names a membership's person for labels and notices: their profile
// once they've logged in, otherwise the access code they were granted by.
function who(m: StaffMembership): string {
  return m.display_name || m.email || m.external_subject
}

// StaffPage manages the selected organization's staff memberships
// (docs/admin-web-plan.md 6.2): grant by access code (DEC-029), change the
// role, disable/enable. The server enforces the same guards the UI hides
// behind — no self-modification, platform_admin only by a platform admin.
export function StaffPage() {
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
  return <OrgStaff key={selectedOrgID} orgID={selectedOrgID} orgName={org?.name ?? ''} />
}

function OrgStaff({ orgID, orgName }: { orgID: number; orgName: string }) {
  const { can, memberships: myMemberships, stores } = useOrgStore()
  const { staffUserID } = useAuth()
  const canManage = can('staff.manage', orgID)
  const callerIsPlatformAdmin = myMemberships.some((m) => m.role === 'platform_admin')

  const query = useStaffMemberships(orgID, canManage)
  const create = useCreateStaffMembership(orgID)
  const update = useUpdateStaffMembership(orgID)

  const [adding, setAdding] = useState(false)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [editRole, setEditRole] = useState<StaffRole>('store_manager')
  const [confirmDisableID, setConfirmDisableID] = useState<number | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const heading = <h1>{orgName ? t.titleForOrg(orgName) : t.title}</h1>

  if (!canManage) {
    return (
      <>
        {heading}
        <ForbiddenState />
      </>
    )
  }

  const storeName = (id: number) => stores.find((s) => s.id === id)?.name
  const scopeLabel = (m: StaffMembership) =>
    m.store_id == null ? t.allStores : storeName(m.store_id) ? t.storeScope(storeName(m.store_id)!) : t.unknownStore(m.store_id)

  const startAction = () => {
    setNotice(null)
    update.reset()
  }

  const run = (input: Parameters<typeof update.mutateAsync>[0], message: string) =>
    void update.mutateAsync(input).then(
      () => {
        setEditingID(null)
        setConfirmDisableID(null)
        setNotice(message)
      },
      () => {
        // Rendered from update.error below.
      },
    )

  // Active first, then by name, so disabled people sink to the bottom.
  const rows = [...(query.data ?? [])].sort(
    (a, b) => Number(a.status === 'disabled') - Number(b.status === 'disabled') || who(a).localeCompare(who(b), 'ru'),
  )

  return (
    <>
      <div className="page-header">
        {heading}
        {!adding && (
          <button
            type="button"
            onClick={() => {
              setNotice(null)
              create.reset()
              setAdding(true)
            }}
          >
            {t.add}
          </button>
        )}
      </div>
      <p className="muted page-intro">{t.intro}</p>

      {adding && (
        <StaffForm
          stores={stores}
          callerIsPlatformAdmin={callerIsPlatformAdmin}
          pending={create.isPending}
          error={create.error}
          onCancel={() => setAdding(false)}
          onSubmit={async (input) => {
            const created = await create.mutateAsync(input)
            setAdding(false)
            setNotice(t.added(who(created)))
          }}
        />
      )}

      {notice && (
        <p role="status" className="notice">
          {notice}
        </p>
      )}
      {update.error != null && editingID == null && confirmDisableID == null && <SubmitError error={update.error} />}

      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        responseOf(query.error)?.status === 403 ? (
          <ForbiddenState />
        ) : (
          <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
        )
      ) : rows.length === 0 ? (
        <EmptyState title={t.empty} />
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th scope="col">{t.columns.person}</th>
              <th scope="col">{t.columns.role}</th>
              <th scope="col">{t.columns.scope}</th>
              <th scope="col">{t.columns.status}</th>
              <th scope="col">
                <span className="visually-hidden">{t.columns.actions}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((m) => {
              const name = who(m)
              const isSelf = m.staff_user_id === staffUserID
              const locked = m.role === 'platform_admin' && !callerIsPlatformAdmin
              const canAct = !isSelf && !locked

              if (confirmDisableID === m.id) {
                return (
                  <tr key={m.id} className="form-row">
                    <td colSpan={5}>
                      <div role="alertdialog" aria-label={t.disableLabel(name)} className="confirm-row">
                        <span>{t.disableConfirm(name)}</span>
                        <button
                          type="button"
                          className="button-danger"
                          autoFocus
                          disabled={update.isPending}
                          onClick={() => run({ id: m.id, status: 'disabled' }, t.disabled(name))}
                        >
                          {update.isPending ? strings.form.saving : t.disable}
                        </button>
                        <button type="button" onClick={() => setConfirmDisableID(null)}>
                          {strings.common.cancel}
                        </button>
                      </div>
                      {update.error != null && <SubmitError error={update.error} />}
                    </td>
                  </tr>
                )
              }

              return (
                <tr key={m.id} className={m.status === 'disabled' ? 'row-inactive' : undefined}>
                  <td>
                    {m.display_name || m.email ? (
                      <>
                        <div>{m.display_name || m.email}</div>
                        {m.display_name && m.email && <div className="muted staff-sub">{m.email}</div>}
                      </>
                    ) : (
                      <>
                        <div className="muted">{t.neverLoggedIn}</div>
                        <div className="muted staff-sub">
                          {t.codePrefix} <code>{m.external_subject}</code>
                        </div>
                      </>
                    )}
                    {isSelf && <span className="badge">{t.you}</span>}
                  </td>
                  <td>
                    {editingID === m.id ? (
                      <div className="confirm-row">
                        <select
                          aria-label={t.editLabel(name)}
                          value={editRole}
                          autoFocus
                          onChange={(e) => setEditRole(e.target.value as StaffRole)}
                        >
                          {grantableRoles(callerIsPlatformAdmin).map((r) => (
                            <option key={r} value={r}>
                              {t.roles[r].label}
                            </option>
                          ))}
                        </select>
                        <button
                          type="button"
                          className="button-primary"
                          disabled={update.isPending || editRole === m.role}
                          onClick={() => run({ id: m.id, role: editRole }, t.updated(name))}
                        >
                          {update.isPending ? strings.form.saving : t.save}
                        </button>
                        <button type="button" onClick={() => setEditingID(null)}>
                          {strings.common.cancel}
                        </button>
                        {update.error != null && <SubmitError error={update.error} />}
                      </div>
                    ) : (
                      <span title={t.roles[m.role].description}>{t.roles[m.role].label}</span>
                    )}
                  </td>
                  <td>{scopeLabel(m)}</td>
                  <td>
                    <span className={`status status--${m.status === 'active' ? 'active' : 'revoked'}`}>
                      {t.status[m.status]}
                    </span>
                  </td>
                  <td className="cell-actions">
                    {isSelf ? (
                      <span className="muted staff-sub">{t.selfHint}</span>
                    ) : (
                      canAct &&
                      editingID !== m.id && (
                        <div className="row-actions">
                          {m.status === 'active' && (
                            <button
                              type="button"
                              aria-label={t.editLabel(name)}
                              onClick={() => {
                                startAction()
                                setEditRole(m.role)
                                setEditingID(m.id)
                              }}
                            >
                              {t.edit}
                            </button>
                          )}
                          {m.status === 'active' ? (
                            <button
                              type="button"
                              aria-label={t.disableLabel(name)}
                              onClick={() => {
                                startAction()
                                setEditingID(null)
                                setConfirmDisableID(m.id)
                              }}
                            >
                              {t.disable}
                            </button>
                          ) : (
                            <button
                              type="button"
                              aria-label={t.enableLabel(name)}
                              disabled={update.isPending}
                              onClick={() => {
                                startAction()
                                run({ id: m.id, status: 'active' }, t.enabled(name))
                              }}
                            >
                              {t.enable}
                            </button>
                          )}
                        </div>
                      )
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </>
  )
}
