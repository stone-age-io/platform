import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from './auth'

// The `can` map is the console's entire authorization surface, and it mirrors
// the API rules in schema.json BY HAND. Nothing checked the mirror: the server
// half is proven by scripts/test-authz.sh, and this half had no guard at all,
// so a role gaining or losing a capability here was a silent change.
//
// The matrix below is the table in CLAUDE.md, transcribed. If a change makes a
// cell here fail, either the table is out of date or the change is wrong --
// and either way it should be a decision rather than a surprise.
type Role = 'owner' | 'admin' | 'member' | 'viewer' | 'dashboard'
type Capability =
  | 'manageMembers'
  | 'manageInfrastructure'
  | 'manageDefinitions'
  | 'manageLeafNodes'
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
    manageLeafNodes: true,
    manageMessaging: true,
    viewInventory: true,
    manageInventory: true,
    decommissionInventory: true,
  },
  admin: {
    manageMembers: true,
    manageInfrastructure: true,
    manageDefinitions: true,
    manageLeafNodes: true,
    manageMessaging: true,
    viewInventory: true,
    manageInventory: true,
    decommissionInventory: true,
  },
  member: {
    manageMembers: false,
    manageInfrastructure: false,
    manageDefinitions: false,
    manageLeafNodes: false,
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
    manageLeafNodes: false,
    manageMessaging: false,
    viewInventory: true,
    manageInventory: false,
    decommissionInventory: false,
  },
  // The zero-authority role, and the reason it exists. It is the probe: an
  // allowlist that has quietly become a deny-list lets `dashboard` through, and
  // a role holding SOME authority cannot prove that, because it passes for the
  // wrong reason.
  dashboard: {
    manageMembers: false,
    manageInfrastructure: false,
    manageDefinitions: false,
    manageLeafNodes: false,
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

  // owner and admin are deliberately identical in the rules; the only
  // difference lives elsewhere (an owner cannot leave their own organization).
  it('gives owner and admin identical capabilities', () => {
    expect(storeFor('owner').can).toEqual(storeFor('admin').can)
  })

  // viewer reads exactly what member reads. The difference between them is
  // writes -- not reads, which the API rules do not role-scope at all.
  it('differs between member and viewer only on write capabilities', () => {
    const member = storeFor('member').can
    const viewer = storeFor('viewer').can
    expect(viewer.viewInventory).toBe(member.viewInventory)
    expect(viewer.manageInventory).toBe(false)
    expect(member.manageInventory).toBe(true)
  })
})

describe('userRole', () => {
  // This used to fall back to 'member', which is a fail-OPEN default: a user
  // whose memberships had not loaded, or whose membership was just deleted, was
  // handed member capabilities and offered controls the API then refused.
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
