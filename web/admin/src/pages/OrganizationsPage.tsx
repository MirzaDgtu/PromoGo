import { useState, type ReactNode } from 'react'

import {
  responseOf,
  useCreateOrganization,
  useCreateStore,
  useOrganizations,
  useStoresByOrganization,
  useUpdateStore,
} from '../api/queries'
import type { components } from '../api/schema.gen'
import { ChevronIcon } from '../components/icons'
import { NameForm } from '../components/NameForm'
import { StoreForm, toStoreInput } from '../components/StoreForm'
import { EmptyState, ErrorState, LoadingState } from '../components/states'
import { strings } from '../i18n/strings'
import { useOrgStore } from '../session/OrgStoreContext'

type Organization = components['schemas']['Organization']
type Store = components['schemas']['Store']

// At most one store form is open at a time, either adding to an
// organization or editing one store.
type Editor = { kind: 'create'; orgID: number } | { kind: 'edit'; orgID: number; storeID: number } | null

const COLUMNS = 5

// OrganizationsPage is one table of organizations, each a collapsible group
// of its stores. Picking an organization or store here is the same
// selection as the sidebar switchers. Add/edit controls appear only with
// the matching permission; the backend enforces the same check regardless
// (docs/admin-web-plan.md 4: hiding a button is not access control).
export function OrganizationsPage() {
  const { hasGlobalPermission, setSelectedOrgID } = useOrgStore()
  const organizations = useOrganizations(true)
  const createOrg = useCreateOrganization()
  const [addingOrg, setAddingOrg] = useState(false)
  const [editor, setEditor] = useState<Editor>(null)
  const [notice, setNotice] = useState<string | null>(null)

  return (
    <>
      <div className="page-header">
        <h1>{strings.organizations.title}</h1>
        {hasGlobalPermission('organizations.manage') && !addingOrg && (
          <button
            type="button"
            onClick={() => {
              setNotice(null)
              createOrg.reset()
              setAddingOrg(true)
            }}
          >
            {strings.organizations.addOrganization}
          </button>
        )}
      </div>

      {addingOrg && (
        <NameForm
          label={strings.organizations.createLabel}
          submitLabel={strings.organizations.createSubmit}
          pending={createOrg.isPending}
          error={createOrg.error}
          onCancel={() => setAddingOrg(false)}
          onSubmit={async (name) => {
            const org = await createOrg.mutateAsync(name)
            setAddingOrg(false)
            if (org.id != null) {
              setSelectedOrgID(org.id)
              // A new organization's next onboarding step is its first store.
              setEditor({ kind: 'create', orgID: org.id })
            }
            setNotice(strings.organizations.created(org.name ?? name))
          }}
        />
      )}
      {notice && (
        <p role="status" className="notice">
          {notice}
        </p>
      )}

      {organizations.isPending ? (
        <LoadingState />
      ) : organizations.isError ? (
        <ErrorState response={responseOf(organizations.error)} onRetry={() => void organizations.refetch()} />
      ) : organizations.data.length === 0 ? (
        <EmptyState title={strings.organizations.empty} />
      ) : (
        <OrganizationsTable
          organizations={organizations.data}
          editor={editor}
          setEditor={(next) => {
            setNotice(null)
            setEditor(next)
          }}
          onSaved={(message) => {
            setEditor(null)
            setNotice(message)
          }}
        />
      )}
    </>
  )
}

function OrganizationsTable({
  organizations,
  editor,
  setEditor,
  onSaved,
}: {
  organizations: Organization[]
  editor: Editor
  setEditor: (editor: Editor) => void
  onSaved: (message: string) => void
}) {
  const orgIDs = organizations.flatMap((o) => (o.id != null ? [o.id] : []))
  const storeQueries = useStoresByOrganization(orgIDs)
  const [collapsed, setCollapsed] = useState<ReadonlySet<number>>(new Set())

  return (
    <div className="table-wrap">
    <table className="org-table">
      <thead>
        <tr>
          <th scope="col">{strings.stores.name}</th>
          <th scope="col">{strings.stores.address}</th>
          <th scope="col">{strings.stores.externalCode}</th>
          <th scope="col">{strings.stores.id}</th>
          <th scope="col">
            <span className="visually-hidden">{strings.stores.actions}</span>
          </th>
        </tr>
      </thead>
      {organizations.map((org, i) => {
        if (org.id == null) return null
        const orgID = org.id
        const isCollapsed = collapsed.has(orgID)
        return (
          <OrganizationGroup
            key={orgID}
            org={{ ...org, id: orgID }}
            query={storeQueries[i]!}
            collapsed={isCollapsed}
            onToggle={() =>
              setCollapsed((prev) => {
                const next = new Set(prev)
                if (isCollapsed) next.delete(orgID)
                else next.add(orgID)
                return next
              })
            }
            editor={editor?.orgID === orgID ? editor : null}
            setEditor={setEditor}
            onSaved={onSaved}
          />
        )
      })}
    </table>
    </div>
  )
}

function OrganizationGroup({
  org,
  query,
  collapsed,
  onToggle,
  editor,
  setEditor,
  onSaved,
}: {
  org: Organization & { id: number }
  query: ReturnType<typeof useStoresByOrganization>[number]
  collapsed: boolean
  onToggle: () => void
  editor: Editor
  setEditor: (editor: Editor) => void
  onSaved: (message: string) => void
}) {
  const { selectedOrgID, setSelectedOrgID, selectedStoreID, setSelectedStoreID, can } = useOrgStore()
  const isCurrent = org.id === selectedOrgID
  const name = org.name ?? ''
  const canCreate = can('stores.manage', org.id)

  return (
    <tbody className="org-group">
      <tr className="org-group__header" aria-current={isCurrent ? 'true' : undefined}>
        <th scope="colgroup" colSpan={COLUMNS - 1}>
          <button
            type="button"
            className="icon-button"
            aria-expanded={!collapsed}
            aria-label={collapsed ? strings.organizations.expand(name) : strings.organizations.collapse(name)}
            onClick={onToggle}
          >
            <ChevronIcon className={collapsed ? 'disclosure' : 'disclosure disclosure--open'} />
          </button>
          <button type="button" className="link-button org-group__name" onClick={() => setSelectedOrgID(org.id)}>
            {name}
          </button>
          <span className="org-group__meta">
            {query.data && strings.organizations.storeCount(query.data.length)}
            {isCurrent && <span className="badge">{strings.organizations.current}</span>}
          </span>
        </th>
        <td className="cell-actions">
          {canCreate && (
            <button
              type="button"
              aria-label={strings.stores.addLabel(name)}
              onClick={() => {
                if (collapsed) onToggle()
                setEditor({ kind: 'create', orgID: org.id })
              }}
            >
              {strings.stores.add}
            </button>
          )}
        </td>
      </tr>

      {!collapsed && (
        <>
          {editor?.kind === 'create' && (
            <FormRow>
              <CreateStoreForm org={org} onCancel={() => setEditor(null)} onSaved={onSaved} />
            </FormRow>
          )}

          {query.isPending ? (
            <MessageRow>
              <LoadingState />
            </MessageRow>
          ) : query.isError ? (
            <MessageRow>
              <ErrorState response={responseOf(query.error)} onRetry={() => void query.refetch()} />
            </MessageRow>
          ) : query.data.length === 0 && editor?.kind !== 'create' ? (
            <MessageRow>
              <span className="muted">{strings.stores.empty}</span>
            </MessageRow>
          ) : (
            query.data.map((store) => {
              if (store.id == null) return null
              const storeID = store.id
              if (editor?.kind === 'edit' && editor.storeID === storeID) {
                return (
                  <FormRow key={storeID}>
                    <EditStoreForm
                      orgID={org.id}
                      store={{ ...store, id: storeID }}
                      onCancel={() => setEditor(null)}
                      onSaved={onSaved}
                    />
                  </FormRow>
                )
              }
              const isCurrentStore = isCurrent && storeID === selectedStoreID
              return (
                <tr key={storeID} aria-current={isCurrentStore ? 'true' : undefined}>
                  <td className="cell-store-name">
                    <button
                      type="button"
                      className="link-button"
                      aria-label={strings.stores.select(store.name)}
                      onClick={() => {
                        // Switching organization clears the store, so set it second.
                        setSelectedOrgID(org.id)
                        setSelectedStoreID(storeID)
                      }}
                    >
                      {store.name}
                    </button>
                    {isCurrentStore && <span className="badge">{strings.stores.current}</span>}
                  </td>
                  <td>{store.address || <span className="muted">{strings.stores.notSet}</span>}</td>
                  <td>{store.external_code ?? <span className="muted">{strings.stores.notSet}</span>}</td>
                  <td className="muted">{storeID}</td>
                  <td className="cell-actions">
                    {can('stores.manage', org.id, storeID) && (
                      <button
                        type="button"
                        aria-label={strings.stores.editLabel(store.name)}
                        onClick={() => setEditor({ kind: 'edit', orgID: org.id, storeID })}
                      >
                        {strings.stores.edit}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })
          )}
        </>
      )}
    </tbody>
  )
}

function FormRow({ children }: { children: ReactNode }) {
  return (
    <tr className="form-row">
      <td colSpan={COLUMNS}>{children}</td>
    </tr>
  )
}

function MessageRow({ children }: { children: ReactNode }) {
  return (
    <tr>
      <td colSpan={COLUMNS}>{children}</td>
    </tr>
  )
}

function CreateStoreForm({
  org,
  onCancel,
  onSaved,
}: {
  org: Organization & { id: number }
  onCancel: () => void
  onSaved: (message: string) => void
}) {
  const create = useCreateStore(org.id)
  return (
    <StoreForm
      title={strings.stores.createTitle(org.name ?? '')}
      submitLabel={strings.stores.createSubmit}
      pending={create.isPending}
      error={create.error}
      onCancel={onCancel}
      onSubmit={async (values) => {
        const store = await create.mutateAsync(toStoreInput(values))
        onSaved(strings.stores.created(store.name))
      }}
    />
  )
}

function EditStoreForm({
  orgID,
  store,
  onCancel,
  onSaved,
}: {
  orgID: number
  store: Store & { id: number }
  onCancel: () => void
  onSaved: (message: string) => void
}) {
  const update = useUpdateStore(orgID, store.id)
  return (
    <StoreForm
      title={strings.stores.editTitle(store.name)}
      initial={{
        name: store.name,
        address: store.address,
        externalCode: store.external_code ?? '',
        smsInviteEnabled: store.sms_invite_enabled,
      }}
      submitLabel={strings.stores.editSubmit}
      pending={update.isPending}
      error={update.error}
      onCancel={onCancel}
      onSubmit={async (values) => {
        const saved = await update.mutateAsync(toStoreInput(values))
        onSaved(strings.stores.updated(saved.name))
      }}
    />
  )
}
