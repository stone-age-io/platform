import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { User, Membership, Organization, SuperUser, NatsUser } from '@/types/pocketbase'
import type { AuthProviderInfo } from 'pocketbase' // Import SDK type

// Extended interface to include the expansion we need
export interface ExtendedMembership extends Membership {
  expand?: {
    organization: Organization
    nats_user?: NatsUser
  }
}

export const useAuthStore = defineStore('auth', () => {
  // ============================================================================
  // STATE
  // ============================================================================
  
  const user = ref<User | SuperUser | null>(null)
  const memberships = ref<ExtendedMembership[]>([])
  const currentOrgId = ref<string | null>(null)
  const isSuperAdmin = ref(false)
  
  // OAuth State
  const authProviders = ref<AuthProviderInfo[]>([])
  
  // ============================================================================
  // COMPUTED
  // ============================================================================
  
  const isAuthenticated = computed(() => !!user.value)
  
  // ... (Keep existing computed properties: currentMembership, currentOrg, etc.) ...
  const currentMembership = computed(() => {
    if (!currentOrgId.value) return null
    return memberships.value.find(m => m.organization === currentOrgId.value) || null
  })
  
  const currentOrg = computed((): Organization | null => {
    return currentMembership.value?.expand?.organization || null
  })

  const currentNatsUser = computed((): NatsUser | null => {
    return currentMembership.value?.expand?.nats_user || null
  })
  
  const userRole = computed(() => {
    if (isSuperAdmin.value) return 'owner'
    return currentMembership.value?.role || 'member'
  })
  
  // What each role may do, in one place. The router, the sidebar, and individual
  // views all read this, so a role change is a one-line edit here rather than a
  // hunt through inline ['owner','admin'] arrays.
  //
  // This mirrors the API rules in schema.json and is a UI convenience only —
  // the server enforces the same boundaries in its own rules. Keep the two in
  // step: if you gate a collection there, add or reuse a capability here.
  // scripts/test-authz.sh is what proves the server half.
  const can = computed(() => {
    const role = userRole.value
    const isAdmin = role === 'owner' || role === 'admin'
    return {
      // Members and invitations.
      manageMembers: isAdmin,
      // NATS identities/roles/exports/imports, Nebula CA/networks/hosts. These
      // records mint signed credentials, so they are owner/admin only.
      manageInfrastructure: isAdmin,
      // Thing types, thing operations, message schemas, location types.
      manageDefinitions: isAdmin,
      // Edge nodes.
      manageLeafNodes: isAdmin,
      // JetStream streams and KV buckets.
      manageMessaging: isAdmin,
      // Reading things and locations. `viewer` is a tenant's read-only staff:
      // it sees the inventory screens and writes nothing.
      //
      // This is NOT a read boundary and must not be mistaken for one. The read
      // rules on things, locations, thing_types, location_types,
      // message_schemas and leaf_nodes are org-scoped with no role check, so
      // every role in the org — `dashboard` included — can already read all of
      // it over the API. What this capability decides is which screens the
      // console navigates to. If a read ever needs to be a boundary, it has to
      // be a branch in schema.json; adding it here would be theatre.
      viewInventory: isAdmin || role === 'member' || role === 'viewer',
      // Things and locations — the day-to-day inventory work, open to members.
      // Deliberately excludes `viewer`: this is the capability every create /
      // edit control is gated on.
      manageInventory: isAdmin || role === 'member',
      // Taking inventory out of service: deleting a thing/location, and flipping
      // a thing or leaf node's `active` flag. Separate from manageInventory
      // because members create and edit but do not decommission — deactivating
      // revokes the device's NATS identity, and deleting orphans it and
      // propagates into every edge KV mirror.
      decommissionInventory: isAdmin,
    }
  })

  // Dashboard user: the Visualizer and their own settings, nothing else. This is
  // the least privileged role -- it holds no capability in the `can` map above,
  // which is why scripts/test-authz.sh uses it to prove the rules are allowlists.
  //
  // Distinct from `viewer`, which is the read-only HUMAN: a viewer browses the
  // whole console read surface and writes nothing. `dashboard` is an appliance
  // login for an unattended screen, so it gets one screen. Do not collapse the
  // two -- a kiosk in a hallway and an auditor at a desk want different things,
  // and merging them would also cost the test suite its zero-authority probe.
  const isDashboardUser = computed(() => {
    if (isSuperAdmin.value) return false
    return currentMembership.value?.role === 'dashboard'
  })

  // Operator: can manage all organizations (create/edit/delete orgs, invite to any org)
  const isOperator = computed(() => {
    if (isSuperAdmin.value) return true
    return (user.value as User)?.is_operator === true
  })

  // Can access /organizations routes for org management
  const canManageOrganizations = computed(() => isOperator.value)
  
  // ============================================================================
  // ACTIONS
  // ============================================================================
  
  async function login(email: string, password: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    const authData = await pb.collection(collection).authWithPassword(email, password)
    user.value = authData.record as unknown as User
    isSuperAdmin.value = asSuperAdmin
    await loadContext()
  }

  // NEW: Fetch configured OAuth2 providers from backend
  async function loadAuthMethods() {
    try {
      const result = await pb.collection('users').listAuthMethods()
      authProviders.value = result.oauth2?.providers || []
    } catch (e) {
      console.warn('Failed to load auth methods', e)
    }
  }

  // NEW: Handle OAuth2 Login Flow
  async function loginWithOAuth2(provider: string) {
    // This triggers the popup window
    const authData = await pb.collection('users').authWithOAuth2({ provider })
    
    user.value = authData.record as unknown as User
    isSuperAdmin.value = false // OAuth is strictly for standard users
    
    // Update profile avatar if provided by OAuth and not set locally
    if (authData.meta?.avatarUrl && !user.value.avatar) {
      try {
        const formData = new FormData()
        // Fetch the image and convert to blob to upload to PocketBase
        const response = await fetch(authData.meta.avatarUrl)
        if (response.ok) {
          const blob = await response.blob()
          formData.append('avatar', blob)
          const updated = await pb.collection('users').update(user.value.id, formData)
          user.value = updated as unknown as User
        }
      } catch (e) {
        console.warn('Failed to sync OAuth avatar', e)
      }
    }

    await loadContext()
  }

  async function requestPasswordReset(email: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    return await pb.collection(collection).requestPasswordReset(email)
  }
  
  async function logout() {
    pb.authStore.clear()
    user.value = null
    memberships.value = []
    currentOrgId.value = null
    isSuperAdmin.value = false
  }
  
  // ... (Keep existing loadContext, switchOrganization, etc.) ...
  async function loadContext() {
    if (!user.value) return
    
    if (isSuperAdmin.value) {
      const allOrgs = await pb.collection('organizations').getFullList<Organization>({ sort: 'name' })
      memberships.value = allOrgs.map(org => ({
        id: `super_${org.id}`,
        created: org.created,
        updated: org.updated,
        user: user.value!.id,
        organization: org.id,
        role: 'owner' as const,
        expand: { organization: org, nats_user: undefined }
      }))
    } else {
      memberships.value = await pb.collection('memberships').getFullList<ExtendedMembership>({
        filter: `user = "${user.value.id}"`,
        expand: 'organization,nats_user',
      })
    }
    
    if (user.value.current_organization) {
      const exists = memberships.value.find(m => m.organization === user.value?.current_organization)
      if (exists) {
        currentOrgId.value = user.value.current_organization
        return
      }
    }
    
    if (memberships.value.length > 0) {
      const firstOrgId = memberships.value[0].organization
      switchOrganization(firstOrgId)
    } else {
      currentOrgId.value = null
    }
  }
  
  async function switchOrganization(orgId: string) {
    if (!user.value || currentOrgId.value === orgId) return
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    // Persist to the backend BEFORE flipping local state. PocketBase API rules
    // (e.g. nats_users.viewRule) check the server-side `current_organization`,
    // so any reactive consumer that fires off a request the instant currentOrgId
    // changes must see the backend already in the new context. Otherwise the
    // request gets denied by the rule for the previous org.
    try {
      await pb.collection(collection).update(user.value.id, { current_organization: orgId })
    } catch (e) {
      console.warn('Organization persistence failed:', e)
      return
    }
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    window.dispatchEvent(new CustomEvent('organization-changed', { detail: { orgId } }))
  }

  async function updateCurrentMembership(data: Partial<Membership>) {
    if (!currentMembership.value || isSuperAdmin.value) return
    const memId = currentMembership.value.id
    const idx = memberships.value.findIndex(m => m.id === memId)
    if (idx !== -1) memberships.value[idx] = { ...memberships.value[idx], ...data } as ExtendedMembership
    try {
      const updated = await pb.collection('memberships').update<ExtendedMembership>(memId, data, { expand: 'organization,nats_user' })
      if (idx !== -1) memberships.value[idx] = updated
    } catch (e) {
      await loadContext()
      throw e
    }
  }

  async function leaveOrganization(orgId: string) {
    if (isSuperAdmin.value) return 
    const membership = memberships.value.find(m => m.organization === orgId)
    if (!membership) return
    if (membership.role === 'owner') throw new Error("Owners cannot leave their own organization.")
    await pb.collection('memberships').delete(membership.id)
    await loadContext()
  }
  
  async function initializeFromAuth() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as unknown as User
      isSuperAdmin.value = pb.authStore.model.collectionName === '_superusers'
      try {
        // Refresh token first to verify it's still valid server-side
        const collection = isSuperAdmin.value ? '_superusers' : 'users'
        const authData = await pb.collection(collection).authRefresh()
        user.value = authData.record as unknown as User
        await loadContext()
      } catch (error) {
        console.warn('Session invalid, logging out...')
        await logout()
      }
    }
  }
  
  return {
    // State
    user,
    memberships,
    currentOrgId,
    isSuperAdmin,
    authProviders, // Exported
    
    // Computed
    isAuthenticated,
    currentMembership,
    currentOrg,
    currentNatsUser,
    userRole,
    can,
    isDashboardUser,
    isOperator,
    canManageOrganizations,
    
    // Actions
    login,
    loginWithOAuth2, // Exported
    loadAuthMethods, // Exported
    logout,
    requestPasswordReset,
    loadMemberships: loadContext,
    switchOrganization,
    updateCurrentMembership,
    leaveOrganization,
    initializeFromAuth,
  }
})
