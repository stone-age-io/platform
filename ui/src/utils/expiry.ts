/**
 * Credential expiry badges for list views: NATS user JWTs
 * (`nats_users.jwt_expires_at`) and Nebula host certificates
 * (`nebula_hosts.expires_at`). Expired device credentials fail silently.
 */

/**
 * Warning window for credentials that can be renewed (NATS user JWTs, Nebula
 * host certificates). Mirrors `certExpiryWindow` in hooks/cert_expiry.go.
 */
export const EXPIRY_WARNING_DAYS = 30

/**
 * Warning window for a Nebula CA; must stay longer than the one above. A CA
 * cannot be renewed, only rotated, and rotation includes a wait long enough
 * for every host to fetch a new config. Nebula advises starting 2-3 months out.
 *
 * It must be shown in the console: rotation is an owner/admin action of the
 * CA's own org, and the server-side warning (/api/ready, /metrics) reaches only
 * the platform operator. Keep in step with `caExpiryWindow` in
 * hooks/cert_expiry.go.
 */
export const CA_EXPIRY_WARNING_DAYS = 90

export type ExpiryState = 'none' | 'ok' | 'expiring' | 'expired'

/**
 * Classify an expiry timestamp. Absent or unparseable input is 'none', never a
 * false alarm. `now` is for tests.
 */
export function expiryState(
  value: string | null | undefined,
  windowDays: number = EXPIRY_WARNING_DAYS,
  now: number = Date.now(),
): ExpiryState {
  if (!value) return 'none'

  // PocketBase stores "2026-12-31 23:59:59.000Z"; Safari needs the T.
  const ms = new Date(String(value).replace(' ', 'T')).getTime()
  if (isNaN(ms)) return 'none'

  if (ms <= now) return 'expired'
  if (ms - now <= windowDays * 86400000) return 'expiring'
  return 'ok'
}

/** Whole days until expiry; negative once past. */
export function daysUntil(
  value: string | null | undefined,
  now: number = Date.now(),
): number | null {
  if (!value) return null
  const ms = new Date(String(value).replace(' ', 'T')).getTime()
  if (isNaN(ms)) return null
  return Math.ceil((ms - now) / 86400000)
}

/** Short label for a badge, e.g. "Expires in 12d" / "Expired". */
export function expiryLabel(
  value: string | null | undefined,
  windowDays: number = EXPIRY_WARNING_DAYS,
  now: number = Date.now(),
): string {
  const state = expiryState(value, windowDays, now)
  if (state === 'expired') return 'Expired'
  if (state === 'expiring') {
    const d = daysUntil(value, now)
    return d !== null && d <= 1 ? 'Expires today' : `Expires in ${d}d`
  }
  return ''
}
