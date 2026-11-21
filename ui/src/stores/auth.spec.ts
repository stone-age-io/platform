import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ClientResponseError } from 'pocketbase'

import { pb } from '@/utils/pb'
import { isSessionRejected, useAuthStore } from './auth'

// The `can` map mirrors the API rules in schema.json by hand; scripts/test-authz.sh
// covers the server half. The matrix below is the role table in the root
// CLAUDE.md: if a cell fails, either the table or the change is wrong.
type Role = 'owner' | 'admin' | 'member' | 'viewer' | 'dashboard'
type Capability =
  | 'manageMembers'
  | 'manageInfrastructure'
  | 'manageDefinitions'
  | 'manageMessaging'
  | 'viewInventory'
  | 'manageInventory'
  | 'decommissionInventory'

const ORG = 'org_1'

function storeFor(role: Role | null) {
  setActivePinia(createPinia())
  const auth = useAuthStore()
  auth.user = { id: 'u1' } as never
  if (role) {
    auth.memberships = [{ id: 'm1', organization: ORG, user: 'u1', role }] as never
    auth.currentOrgId = ORG
  }
  return auth
}

// true = the role holds the capability.
const MATRIX: Record<Role, Record<Capability, boolean>> = {
  owner: {
    manageMembers: true,
    manageInfrastructure: true,
    manageDefinitions: true,
    manageMessaging: true,
    viewInventory: true,
    manageInventory: true,
    decommissionInventory: true,
  },
  admin: {
    manageMembers: true,
    manageInfrastructure: true,
    manageDefinitions: true,
    manageMessaging: true,
    viewInventory: true,
    manageInventory: true,
    decommissionInventory: true,
  },
  member: {
    manageMembers: false,
    manageInfrastructure: false,
    manageDefinitions: false,
    manageMessaging: false,
    viewInventory: true,
    manageInventory: true,
    decommissionInventory: false,
  },
  // Read-only staff: every inventory screen, no write control anywhere.
  viewer: {
    manageMembers: false,
    manageInfrastructure: false,
    manageDefinitions: false,
    manageMessaging: false,
    viewInventory: true,
    manageInventory: false,
    decommissionInventory: false,
  },
  // The zero-authority probe: a deny-list posing as an allowlist lets it
  // through, where a role with some authority would pass for the wrong reason.
  dashboard: {
    manageMembers: false,
    manageInfrastructure: false,
    manageDefinitions: false,
    manageMessaging: false,
    viewInventory: false,
    manageInventory: false,
    decommissionInventory: false,
  },
}

const ROLES = Object.keys(MATRIX) as Role[]
const CAPABILITIES = Object.keys(MATRIX.owner) as Capability[]

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('auth capability map', () => {
  for (const role of ROLES) {
    describe(role, () => {
      for (const capability of CAPABILITIES) {
        const expected = MATRIX[role][capability]
        it(`${expected ? 'holds' : 'does not hold'} ${capability}`, () => {
          const auth = storeFor(role)
          expect(auth.can[capability]).toBe(expected)
        })
      }
    })
  }

  it('holds nothing at all for the dashboard role', () => {
    const auth = storeFor('dashboard')
    expect(Object.values(auth.can).some(Boolean)).toBe(false)
  })

  // The only owner/admin difference is outside `can`: an owner cannot leave.
  it('gives owner and admin identical capabilities', () => {
    expect(storeFor('owner').can).toEqual(storeFor('admin').can)
  })

  // Reads are not role-scoped in the API rules, so only writes differ.
  it('differs between member and viewer only on write capabilities', () => {
    const member = storeFor('member').can
    const viewer = storeFor('viewer').can
    expect(viewer.viewInventory).toBe(member.viewInventory)
    expect(viewer.manageInventory).toBe(false)
    expect(member.manageInventory).toBe(true)
  })
})

describe('userRole', () => {
  // A fallback role here would fail open while memberships load.
  it('is null with no membership in the active organization, not a role', () => {
    const auth = storeFor(null)
    expect(auth.userRole).toBeNull()
    expect(Object.values(auth.can).some(Boolean)).toBe(false)
  })

  it('is null when the membership belongs to a different organization', () => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.user = { id: 'u1' } as never
    auth.memberships = [{ id: 'm1', organization: 'other_org', user: 'u1', role: 'owner' }] as never
    auth.currentOrgId = ORG

    expect(auth.userRole).toBeNull()
    expect(auth.can.manageMembers).toBe(false)
  })

  it('treats a superuser as an owner', () => {
    setActivePinia(createPinia())
    const auth = useAuthStore()
    auth.user = { id: 'su' } as never
    auth.isSuperAdmin = true

    expect(auth.userRole).toBe('owner')
    expect(auth.can.manageMembers).toBe(true)
  })
})

// Startup checks the stored session with the server. Only the server saying no
// may sign the user out: an unattended dashboard screen that lost its session
// to a server restart stays signed out until someone types the password.
describe('isSessionRejected', () => {
  it.each([
    [400, true], [401, true], [403, true], [404, true],
    [0, false], [408, false], [429, false], [500, false], [502, false], [503, false],
  ])('status %i -> %s', (status, rejected) => {
    expect(isSessionRejected(new ClientResponseError({ status }))).toBe(rejected)
  })

  it('is false for an error with no status', () => {
    expect(isSessionRejected(new Error('boom'))).toBe(false)
    expect(isSessionRejected(null)).toBe(false)
  })
})

describe('hydrating a stored session', () => {
  function storedSession() {
    const b64 = (o: object) => btoa(JSON.stringify(o)).replace(/=+$/, '')
    const token = `${b64({ alg: 'HS256' })}.${b64({ exp: Math.floor(Date.now() / 1000) + 3600 })}.sig`
    pb.authStore.save(token, { id: 'u1', collectionName: 'users' } as never)
    return token
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    storedSession()
    vi.spyOn(pb.collection('memberships'), 'getFullList').mockResolvedValue([] as never)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    pb.authStore.clear()
  })

  it('keeps the session and retries while the server cannot be reached', async () => {
    const refresh = vi.spyOn(pb.collection('users'), 'authRefresh')
      .mockRejectedValueOnce(new ClientResponseError({ status: 0 }))
      .mockRejectedValueOnce(new ClientResponseError({ status: 502 }))
      .mockResolvedValueOnce({ token: 't', record: { id: 'u1' } } as never)
    const auth = useAuthStore()

    const done = auth.initializeFromAuth()
    await vi.advanceTimersByTimeAsync(0)
    expect(auth.hydrationRetries).toBe(1)
    expect(auth.isAuthenticated).toBe(true)

    await vi.advanceTimersByTimeAsync(3000)
    await done
    expect(refresh).toHaveBeenCalledTimes(3)
    expect(auth.isAuthenticated).toBe(true)
    expect(auth.hydrationRetries).toBe(0)
    expect(pb.authStore.isValid).toBe(true)
  })

  it('signs out when the server rejects the token', async () => {
    vi.spyOn(pb.collection('users'), 'authRefresh')
      .mockRejectedValueOnce(new ClientResponseError({ status: 401 }))
    const auth = useAuthStore()

    await auth.initializeFromAuth()
    expect(auth.isAuthenticated).toBe(false)
    expect(pb.authStore.isValid).toBe(false)
  })
})
