import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// Every failure here is silent: a missing cache is a burst of requests, a
// missing reset hands one user's credential to the next, a missing skew breaks
// an image only in a token's last seconds.

const getToken = vi.fn<() => Promise<string>>()
const authStore = { isValid: true }

vi.mock('./pb', () => ({
  pb: {
    get authStore() {
      return authStore
    },
    files: {
      getToken: () => getToken(),
      // Enough of the real getURL to make the token visible in the result.
      getURL: (r: any, f: string, o: any) =>
        `/api/files/${r.collectionName}/${r.id}/${f}?token=${o.token}`,
    },
  },
}))

// An unsigned JWT carrying `exp`, which is all isTokenExpired reads. The `jti`
// nonce keeps two tokens with the same expiry from being byte-identical, so
// "the token changed" assertions mean something. Call counts are the primary
// evidence.
let mintCounter = 0
function tokenExpiringIn(seconds: number): string {
  const payload = { exp: Math.floor(Date.now() / 1000) + seconds, jti: ++mintCounter }
  const b64 = Buffer.from(JSON.stringify(payload))
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
  return `header.${b64}.signature`
}

let fileUrl: typeof import('./fileToken').fileUrl
let resetFileToken: typeof import('./fileToken').resetFileToken

// Tested through fileUrl(), the public surface, so the tests survive the
// caching moving and still fail if the behaviour changes.
const RECORD = { id: 'rec1', collectionName: 'users' }
async function url(): Promise<string | null> {
  return fileUrl({ record: RECORD, filename: 'a.png' })
}

beforeEach(async () => {
  // Fresh module per test: the cache is module state by design (fileToken.ts).
  vi.resetModules()
  getToken.mockReset()
  authStore.isValid = true
  const mod = await import('./fileToken')
  fileUrl = mod.fileUrl
  resetFileToken = mod.resetFileToken
})

afterEach(() => {
  vi.useRealTimers()
})

describe('the file token cache, via fileUrl', () => {
  it('mints once and reuses the token', async () => {
    getToken.mockResolvedValue(tokenExpiringIn(180))

    const first = await url()
    const second = await url()

    expect(first).toBe(second)
    expect(getToken).toHaveBeenCalledTimes(1)
  })

  it('collapses concurrent callers onto one request', async () => {
    // e.g. thirty avatars mounting in the same tick.
    let release!: (t: string) => void
    getToken.mockReturnValue(new Promise<string>((r) => { release = r }))

    const all = Promise.all(Array.from({ length: 30 }, () => url()))
    release(tokenExpiringIn(180))
    const results = await all

    expect(getToken).toHaveBeenCalledTimes(1)
    expect(new Set(results).size).toBe(1)
  })

  it('re-mints once the cached token is inside the refresh skew', async () => {
    // 20s left is inside the 30s skew, so it must not be reused.
    getToken.mockResolvedValueOnce(tokenExpiringIn(20))
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))

    const stale = await url()
    const fresh = await url()

    expect(getToken).toHaveBeenCalledTimes(2)
    expect(fresh).not.toBe(stale)
  })

  it('re-mints after the cached token ages into the skew', async () => {
    vi.useFakeTimers()
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))

    const first = await url()
    expect(await url()).toBe(first)

    // Past the point where fewer than 30 seconds remain.
    vi.advanceTimersByTime(155_000)
    const second = await url()

    expect(getToken).toHaveBeenCalledTimes(2)
    expect(second).not.toBe(first)
  })

  it('does not ask the server when there is no session', async () => {
    authStore.isValid = false

    expect(await url()).toBeNull()
    expect(getToken).not.toHaveBeenCalled()
  })

  it('returns empty rather than throwing when the mint fails', async () => {
    // Callers rendering an image do not try/catch.
    getToken.mockRejectedValue(new Error('offline'))
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

    await expect(url()).resolves.toBeNull()

    warn.mockRestore()
  })

  it('recovers on the next call after a failure', async () => {
    getToken.mockRejectedValueOnce(new Error('offline'))
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

    expect(await url()).toBeNull()
    expect(await url()).not.toBeNull()

    warn.mockRestore()
  })
})

describe('resetFileToken', () => {
  it('forces the next caller to mint a new token', async () => {
    // The logout path: the next user on this tab must not inherit the token.
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))

    const before = await url()
    resetFileToken()
    const after = await url()

    expect(getToken).toHaveBeenCalledTimes(2)
    expect(after).not.toBe(before)
  })
})
