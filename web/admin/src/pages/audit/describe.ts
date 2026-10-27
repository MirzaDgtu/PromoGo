import type { AuditEvent, StaffRole } from '../../api/queries'
import { strings } from '../../i18n/strings'

const t = strings.audit

const text = (v: unknown): string => (typeof v === 'string' ? v : '')
const num = (v: unknown): number | null => (typeof v === 'number' ? v : null)

function roleLabel(v: unknown): string {
  const role = text(v) as StaffRole
  return role in strings.staff.roles ? strings.staff.roles[role].label : text(v)
}

// describeEvent renders one audit event as a sentence. Metadata is only
// present on events recorded since DEC-030, so every branch degrades to a
// shorter sentence when a field is missing rather than showing raw keys.
export function describeEvent(e: AuditEvent, storeName: (id: number) => string): string {
  const m = e.metadata ?? {}
  const person = e.target_name || t.unknownPerson
  switch (e.action) {
    case 'organization.created':
      return t.events.organizationCreated(text(m.name))
    case 'store.created':
      return t.events.storeCreated(text(m.name))
    case 'store.updated':
      return t.events.storeUpdated(text(m.name))
    case 'staff.membership_added': {
      const role = roleLabel(m.role)
      const storeID = num(m.store_id)
      return t.events.membershipAdded(person, storeID ? `${role}, ${storeName(storeID)}` : role)
    }
    case 'staff.role_changed':
      return t.events.roleChanged(person, roleLabel(m.from), roleLabel(m.to))
    case 'staff.status_changed':
      return m.to === 'disabled'
        ? t.events.statusDisabled(person)
        : m.to === 'active'
          ? t.events.statusEnabled(person)
          : t.events.statusChanged(person)
    case 'loyalty_config.changed':
      return t.events.loyaltyChanged(num(m.version))
    case 'loyalty_config.rolled_back':
      return t.events.loyaltyRolledBack(num(m.restored_version), num(m.version))
    case 'api_key.created':
      return t.events.apiKeyCreated(text(m.name))
    case 'api_key.rotated':
      return t.events.apiKeyRotated(text(m.name))
    case 'api_key.revoked':
      return t.events.apiKeyRevoked(text(m.name))
    default:
      return t.events.unknown(e.action)
  }
}

export function describeActor(e: AuditEvent): string {
  switch (e.actor_type) {
    case 'staff':
      return e.actor_name || (e.actor_id != null ? t.actors.staff(e.actor_id) : t.actors.system)
    case 'store_api_key':
      return t.actors.store_api_key
    case 'customer':
      return t.actors.customer
    default:
      return t.actors.system
  }
}
