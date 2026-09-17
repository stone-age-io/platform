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

/**
 * Days ahead of expiry at which a credential starts being called out.
 *
 * This is the window for things that can be RENEWED: NATS user JWTs and Nebula
 * host certificates. pb-nebula re-issues a host certificate automatically once
 * it has burned through its lifetime, so the remedy is short and 30 days is
 * ample. Mirrors `certExpiryWindow` in hooks/cert_expiry.go.
 */
export const EXPIRY_WARNING_DAYS = 30

/**
 * The window for a Nebula CA, which is three times longer and must stay that
 * way.
 *
 * A CA cannot be renewed. The only remedy is rotation, and rotation is a
 * three-step procedure with a wait in the middle that nothing can compress: it
 * has to outlast every host fetching a config nobody told it to fetch. Nebula's
 * own guide asks you to begin two to three months out. Thirty days' notice on a
 * CA is notice that the remedy no longer fits.
 *
 * WHY THIS HAS TO LIVE IN THE CONSOLE, and not only in hooks/cert_expiry.go's
 * `caExpiryWindow`. A Nebula CA belongs to a TENANT organization, and rotating
 * one is a tenant action -- POST /api/org/nebula-ca/rotate is owner/admin of
 * the CA's own org, deliberately, because the wait in the middle belongs to
 * whoever operates the devices. The server-side 90-day warning surfaces on
 * /api/ready and /metrics, which are the platform OPERATOR's surfaces, and the
 * operator cannot rotate a tenant's CA. So the party that can act sees only
 * what the console shows them.
 *
 * This constant existed server-side for a while and not here, which meant the
 * 90-day warning was delivered exclusively to the one party unable to act on
 * it, while the party who could got 30 -- less than the procedure needs. Keep
 * the two in step; expiry.spec.ts asserts they are different numbers so a
 * future tidy-up cannot quietly collapse them.
 */
export const CA_EXPIRY_WARNING_DAYS = 90

export type ExpiryState = 'none' | 'ok' | 'expiring' | 'expired'

/**
 * Classify an expiry timestamp. Absent or unparseable input is 'none' (no
 * expiry / nothing to say), never a false alarm.
 *
 * `windowDays` is the second parameter rather than the third because the caller
 * that varies it is ordinary code, while `now` is only ever passed by a test.
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
