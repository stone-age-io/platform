import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

/**
 * The file-token cache, which is pure logic over a clock and one HTTP call --
 * exactly the shape vitest.config.ts says is worth covering, and exactly the
 * shape `vue-tsc && vite build` cannot protect.
 *
 * It is worth covering for a second reason. Every failure here is SILENT: a
 * missing cache is a burst of token requests nobody sees, a missing reset hands
 * one user's credential to the next, and a missing skew is an image that breaks
 * only for a caller unlucky enough to ask in the last second of a token's life.
 * None of them throws, and none shows up on screen as itself.
 */

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

// A JWT whose payload carries `exp`, which is all isTokenExpired reads. Signed
// with nothing -- it never reaches a server here.
//
// The `jti` nonce is load-bearing. Two tokens minted with the same expiry are
// otherwise byte-identical, so "the token changed" silently becomes "the token
// is the same" and the assertion passes for the wrong reason -- the same trap
// the NATS credential note in CLAUDE.md describes, in miniature. Every test
// below still asserts the CALL COUNT as its primary evidence; the nonce only
// stops the secondary assertion lying.
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

// The cache is only reachable through fileUrl(), which is the whole public
// surface. Asserting through it rather than through the internal token getter
// means these tests keep passing if the caching moves, and keep failing if the
// behaviour does.
const RECORD = { id: 'rec1', collectionName: 'users' }
async function url(): Promise<string | null> {
  return fileUrl({ record: RECORD, filename: 'a.png' })
}

beforeEach(async () => {
  // Fresh module per test: the cache is module state by design (see the
  // "why it is not reactive" note in fileToken.ts), so tests must not share it.
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
    // The case a members list makes: thirty avatars mount in the same tick.
    let release!: (t: string) => void
    getToken.mockReturnValue(new Promise<string>((r) => { release = r }))

    const all = Promise.all(Array.from({ length: 30 }, () => url()))
    release(tokenExpiringIn(180))
    const results = await all

    expect(getToken).toHaveBeenCalledTimes(1)
    expect(new Set(results).size).toBe(1)
  })

  it('re-mints once the cached token is inside the refresh skew', async () => {
    // 20s left is inside the 30s skew, so this must NOT be handed out: the
    // request it is about to be used in could outlive it.
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
    // A caller rendering an avatar will not try/catch, so a rejection here
    // would surface as an unhandled rejection and a component that never
    // resolves its fallback.
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
    // The logout path. A token outliving its session is a bearer credential for
    // every file that session could read, handed to whoever logs in next on
    // this tab.
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))
    getToken.mockResolvedValueOnce(tokenExpiringIn(180))

    const before = await url()
    resetFileToken()
    const after = await url()

    expect(getToken).toHaveBeenCalledTimes(2)
    expect(after).not.toBe(before)
  })
})
