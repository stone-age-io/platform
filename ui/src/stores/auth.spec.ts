import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from './auth'

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
