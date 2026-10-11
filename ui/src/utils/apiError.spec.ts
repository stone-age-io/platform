import { describe, it, expect } from 'vitest'
import { apiErrorMessage } from './apiError'

// The shape the PocketBase SDK's ClientResponseError carries: the top-level
// message, and the response body with per-field errors under `data`.
function responseError(message: string, data: unknown) {
  return { message, response: { message, data } }
}

describe('apiErrorMessage', () => {
  it('shows the field errors rather than the generic top-level message', () => {
    const err = responseError('Failed to create record.', {
      max_subscriptions: { code: 'validation_nats_limit_zero', message: 'NATS reads 0 as zero subscriptions.' },
    })
    expect(apiErrorMessage(err, 'Failed to save role')).toBe('max_subscriptions: NATS reads 0 as zero subscriptions.')
  })

  it('joins several field errors', () => {
    const err = responseError('Failed.', {
      max_subscriptions: { message: 'A.' },
      max_payload: { message: 'B.' },
    })
    expect(apiErrorMessage(err, 'x')).toBe('max_subscriptions: A. max_payload: B.')
  })

  it('falls back to the top-level message when no field explains itself', () => {
    expect(apiErrorMessage(responseError('Only superusers can do this.', {}), 'x')).toBe('Only superusers can do this.')
    expect(apiErrorMessage(responseError('Bad.', { name: { code: 'x' } }), 'x')).toBe('Bad.')
  })

  it.each([undefined, null, {}, { message: '' }, 'boom'])('falls back to the caller for %s', (err) => {
    expect(apiErrorMessage(err, 'Failed to save role')).toBe('Failed to save role')
  })

  it('reads a plain Error', () => {
    expect(apiErrorMessage(new Error('network down'), 'x')).toBe('network down')
  })
})
