import type { StoreOverview } from '../../api/queries'

export type CheckState = 'ok' | 'warn' | 'info'

// A register that hasn't called in this long is flagged: a store sells
// every day, so a silent day usually means a broken 1C exchange.
export const STALE_AFTER_MS = 24 * 60 * 60 * 1000

const relative = new Intl.RelativeTimeFormat('ru-RU', { numeric: 'auto' })

// relativeTime renders "5 минут назад", "вчера", "3 дня назад" — the
// largest whole unit, which is all an overview needs.
export function relativeTime(iso: string, now: Date = new Date()): string {
  const seconds = Math.round((new Date(iso).getTime() - now.getTime()) / 1000)
  const abs = Math.abs(seconds)
  if (abs < 60) return relative.format(0, 'second')
  if (abs < 3600) return relative.format(Math.trunc(seconds / 60), 'minute')
  if (abs < 86400) return relative.format(Math.trunc(seconds / 3600), 'hour')
  return relative.format(Math.trunc(seconds / 86400), 'day')
}

export type RegisterState =
  | { kind: 'hidden' } // caller lacks api_keys.read
  | { kind: 'no_key' }
  | { kind: 'never_used' }
  | { kind: 'connected'; lastUsedAt: string }
  | { kind: 'stale'; lastUsedAt: string }

export function registerState(o: StoreOverview, now: Date = new Date()): RegisterState {
  if (!o.api_keys) return { kind: 'hidden' }
  if (o.api_keys.active === 0) return { kind: 'no_key' }
  const last = o.api_keys.last_used_at
  if (!last) return { kind: 'never_used' }
  return now.getTime() - new Date(last).getTime() > STALE_AFTER_MS
    ? { kind: 'stale', lastUsedAt: last }
    : { kind: 'connected', lastUsedAt: last }
}

export function registerCheck(state: RegisterState): CheckState {
  return state.kind === 'connected' ? 'ok' : state.kind === 'hidden' ? 'info' : 'warn'
}
