import { describe, expect, it } from 'vitest'
import { importerKeyError, isAccountPublicKey, revokedImporters } from './exportGrants'

// A real account key shape: A + 55 base32 characters.
const KEY = 'ACP2FVRLBV4MYHBITV7B23F2B3MF5J5TLCELFUQVM26FQVMZH7CQCHJS'

describe('isAccountPublicKey', () => {
  it('accepts an account key', () => {
    expect(isAccountPublicKey(KEY)).toBe(true)
  })

  it('refuses the other key kinds and near misses', () => {
    // A user key (U), an operator key (O), one character short, lower case, and
    // a base32-invalid digit -- each would be stored and match no account.
    for (const key of ['U' + KEY.slice(1), 'O' + KEY.slice(1), KEY.slice(0, -1), KEY.toLowerCase(), KEY.slice(0, -1) + '1']) {
      expect(isAccountPublicKey(key)).toBe(false)
    }
  })
})

describe('importerKeyError', () => {
  it('admits a new valid key', () => {
    expect(importerKeyError(KEY, [])).toBeNull()
  })

  it('refuses an empty, malformed or duplicate key', () => {
    expect(importerKeyError('', [])).not.toBeNull()
    expect(importerKeyError('not-a-key', [])).not.toBeNull()
    expect(importerKeyError(KEY, [KEY])).toBe('Already listed')
  })
})

describe('revokedImporters', () => {
  it('lists revocations newest first, as dates', () => {
    const list = revokedImporters({ AOLD: 1_700_000_000, ANEW: 1_800_000_000 })
    expect(list.map(r => r.key)).toEqual(['ANEW', 'AOLD'])
    expect(list[0].revokedAt.getTime()).toBe(1_800_000_000 * 1000)
  })

  it('treats a missing map as no revocations', () => {
    expect(revokedImporters(null)).toEqual([])
    expect(revokedImporters(undefined)).toEqual([])
  })
})
