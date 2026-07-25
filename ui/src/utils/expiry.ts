/**
 * Credential expiry classification.
 *
 * The platform mints two kinds of expiring credential — NATS user JWTs
 * (`nats_users.jwt_expires_at`, optional) and Nebula host certificates
 * (`nebula_hosts.expires_at`, always set from validity_years). Both were stored
 * and displayed as a bare date on a detail page, which means the only way to
 * discover an expiry was to already suspect it. Expired device credentials fail
 * silently and all at once, so the useful place for this is the list views the
 * operator already scans.
 *
 * Deliberately not a background job, an email, or a notifications table: making
 * the date visible where someone is already looking costs three functions and no
 * moving parts.
 */

/** Days ahead of expiry at which a credential starts being called out. */
export const EXPIRY_WARNING_DAYS = 30

export type ExpiryState = 'none' | 'ok' | 'expiring' | 'expired'

/**
 * Classify an expiry timestamp. Absent or unparseable input is 'none' (no
 * expiry / nothing to say), never a false alarm.
 */
export function expiryState(
  value: string | null | undefined,
  now: number = Date.now(),
): ExpiryState {
  if (!value) return 'none'

  // PocketBase stores "2026-12-31 23:59:59.000Z"; Safari needs the T.
  const ms = new Date(String(value).replace(' ', 'T')).getTime()
  if (isNaN(ms)) return 'none'

  if (ms <= now) return 'expired'
  if (ms - now <= EXPIRY_WARNING_DAYS * 86400000) return 'expiring'
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
  now: number = Date.now(),
): string {
  const state = expiryState(value, now)
  if (state === 'expired') return 'Expired'
  if (state === 'expiring') {
    const d = daysUntil(value, now)
    return d !== null && d <= 1 ? 'Expires today' : `Expires in ${d}d`
  }
  return ''
}
