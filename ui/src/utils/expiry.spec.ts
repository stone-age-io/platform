import { describe, it, expect } from 'vitest'
import {
  expiryState,
  expiryLabel,
  daysUntil,
  EXPIRY_WARNING_DAYS,
  CA_EXPIRY_WARNING_DAYS,
} from './expiry'

/**
 * The window a caller passes is the whole point of these tests.
 *
 * `hooks/cert_expiry.go` deliberately warns at 30 days for a host certificate
 * and 90 for a CA: a host certificate renews itself, a CA can only be ROTATED,
 * and rotation is a three-step procedure with a wait in the middle long enough
 * to outlast every host fetching a config nobody told it to fetch.
 *
 * That split existed server-side while this module had one constant, so the
 * console badged a CA at 30 -- and since a Nebula CA belongs to a TENANT
 * organization and only its owners and admins may rotate one, the console is
 * the surface reaching the party who can act. /api/ready and /metrics reach the
 * platform operator, who cannot. The 90-day warning was therefore delivered
 * exclusively to whoever could do nothing about it.
 */

const DAY = 86400000
const NOW = Date.parse('2026-06-01T00:00:00Z')

/** PocketBase's stored shape, which is not what `new Date()` wants. */
function pbDate(msFromNow: number): string {
  return new Date(NOW + msFromNow).toISOString().replace('T', ' ').replace('Z', 'Z')
}

describe('the two windows are different numbers', () => {
  // The guard against a future "tidy-up" collapsing them back into one
  // constant. Both halves of the reason live in expiry.ts; this is the part a
  // test can hold.
  it('warns three times earlier for a CA than for a renewable credential', () => {
    expect(CA_EXPIRY_WARNING_DAYS).toBe(90)
    expect(EXPIRY_WARNING_DAYS).toBe(30)
    expect(CA_EXPIRY_WARNING_DAYS).toBeGreaterThan(EXPIRY_WARNING_DAYS)
  })
})

describe('expiryState', () => {
  it('says nothing when there is no expiry to report', () => {
    expect(expiryState(null)).toBe('none')
    expect(expiryState(undefined)).toBe('none')
    expect(expiryState('')).toBe('none')
  })

  // Never a false alarm: garbage in is silence, not a warning.
  it('treats an unparseable date as nothing to say', () => {
    expect(expiryState('not a date', EXPIRY_WARNING_DAYS, NOW)).toBe('none')
  })

  it('parses the space-separated form PocketBase stores', () => {
    expect(expiryState(pbDate(200 * DAY), EXPIRY_WARNING_DAYS, NOW)).toBe('ok')
  })

  it('reports an elapsed date as expired', () => {
    expect(expiryState(pbDate(-DAY), EXPIRY_WARNING_DAYS, NOW)).toBe('expired')
  })

  it('defaults to the 30-day window', () => {
    expect(expiryState(pbDate(20 * DAY), undefined, NOW)).toBe('expiring')
    expect(expiryState(pbDate(40 * DAY), undefined, NOW)).toBe('ok')
  })

  // The regression this file exists for: 60 days out is comfortable for a host
  // certificate and already late for a CA.
  it('classifies the same date differently under each window', () => {
    const in60Days = pbDate(60 * DAY)
    expect(expiryState(in60Days, EXPIRY_WARNING_DAYS, NOW)).toBe('ok')
    expect(expiryState(in60Days, CA_EXPIRY_WARNING_DAYS, NOW)).toBe('expiring')
  })

  it('leaves a CA alone while it is beyond even the long window', () => {
    expect(expiryState(pbDate(120 * DAY), CA_EXPIRY_WARNING_DAYS, NOW)).toBe('ok')
  })
})

describe('expiryLabel', () => {
  // An empty label is what hides the badge, so 'ok' must produce one.
  it('is empty while there is nothing to warn about', () => {
    expect(expiryLabel(pbDate(200 * DAY), EXPIRY_WARNING_DAYS, NOW)).toBe('')
    expect(expiryLabel(null)).toBe('')
  })

  it('names the day count inside the window', () => {
    expect(expiryLabel(pbDate(12 * DAY), EXPIRY_WARNING_DAYS, NOW)).toBe('Expires in 12d')
  })

  it('carries the window through, so a CA gets a label a host would not', () => {
    const in60Days = pbDate(60 * DAY)
    expect(expiryLabel(in60Days, EXPIRY_WARNING_DAYS, NOW)).toBe('')
    expect(expiryLabel(in60Days, CA_EXPIRY_WARNING_DAYS, NOW)).toBe('Expires in 60d')
  })

  it('collapses the last day rather than counting down to zero', () => {
    expect(expiryLabel(pbDate(DAY / 2), EXPIRY_WARNING_DAYS, NOW)).toBe('Expires today')
  })

  it('says Expired without a count once past', () => {
    expect(expiryLabel(pbDate(-DAY), EXPIRY_WARNING_DAYS, NOW)).toBe('Expired')
  })
})

describe('daysUntil', () => {
  it('is null when there is no date', () => {
    expect(daysUntil(null)).toBeNull()
    expect(daysUntil('nonsense')).toBeNull()
  })

  it('goes negative once past', () => {
    expect(daysUntil(pbDate(-3 * DAY), NOW)).toBe(-3)
  })
})
