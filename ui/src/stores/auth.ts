import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import { resetFileToken } from '@/utils/fileToken'
import { useToast } from '@/composables/useToast'
import type { User, Membership, Organization, SuperUser, NatsUser } from '@/types/pocketbase'
import type { AuthProviderInfo } from 'pocketbase'

export interface ExtendedMembership extends Membership {
  expand?: {
    organization: Organization
    nats_user?: NatsUser
  }
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | SuperUser | null>(null)
  const memberships = ref<ExtendedMembership[]>([])
  const currentOrgId = ref<string | null>(null)
  const isSuperAdmin = ref(false)

  const authProviders = ref<AuthProviderInfo[]>([])

  const isAuthenticated = computed(() => !!user.value)
  
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
  
  // null (not a fallback role) when there is no membership in the active org,
  // e.g. while memberships load. Every consumer compares against specific role
  // strings, so null denies everything. Do not default to 'member': that fails open.
  const userRole = computed<Membership['role'] | null>(() => {
    if (isSuperAdmin.value) return 'owner'
    return currentMembership.value?.role ?? null
  })
  
  // What each role may do; the router, sidebar and views all read this. It
  // mirrors the API rules in schema.json and the role table in the root
  // CLAUDE.md, and is navigation only: the server enforces the rules
  // (scripts/test-authz.sh proves it). Keep all three in step.
  const can = computed(() => {
    const role = userRole.value
    const isAdmin = role === 'owner' || role === 'admin'
    return {
      manageMembers: isAdmin,
      // NATS and Nebula records mint signed credentials.
      manageInfrastructure: isAdmin,
      // Thing types, thing operations, location types.
      manageDefinitions: isAdmin,
      // JetStream streams and KV buckets.
      manageMessaging: isAdmin,
      // Which screens the console shows, NOT a read boundary: inventory read
      // rules are org-scoped with no role check, so every role (dashboard too)
      // can read it over the API. A real boundary belongs in schema.json.
      viewInventory: isAdmin || role === 'member' || role === 'viewer',
      // Same roles as viewInventory, named separately so routes say what they
      // mean. Also not a boundary: the feed is org-scoped with no role branch.
      viewActivity: isAdmin || role === 'member' || role === 'viewer',
      // Every create / edit control. Excludes `viewer`.
      manageInventory: isAdmin || role === 'member',
      // Delete a thing/location, or flip a thing's `active` flag. Not for
      // members: deactivating revokes NATS and Nebula identities.
      decommissionInventory: isAdmin,
    }
  })

  // Appliance login for an unattended screen: the Visualizer and its own
  // settings only. Holds no capability, which makes it the zero-authority probe
  // in scripts/test-authz.sh. Do not merge it with `viewer` (a read-only human).
  const isDashboardUser = computed(() => {
    if (isSuperAdmin.value) return false
    return currentMembership.value?.role === 'dashboard'
  })

  // Operators manage every organization.
  const isOperator = computed(() => {
    if (isSuperAdmin.value) return true
    return (user.value as User)?.is_operator === true
  })

  // Gates the /organizations routes.
  const canManageOrganizations = computed(() => isOperator.value)

  async function login(email: string, password: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    const authData = await pb.collection(collection).authWithPassword(email, password)
    user.value = authData.record as unknown as User
    isSuperAdmin.value = asSuperAdmin
    await loadContext()
  }

  // The OAuth2 providers this deployment has configured, for the login screen.
  async function loadAuthMethods() {
    try {
      const result = await pb.collection('users').listAuthMethods()
      authProviders.value = result.oauth2?.providers || []
    } catch (e) {
      console.warn('Failed to load auth methods', e)
    }
  }

  async function loginWithOAuth2(provider: string) {
    const authData = await pb.collection('users').authWithOAuth2({ provider })
    
    user.value = authData.record as unknown as User
    isSuperAdmin.value = false // OAuth is strictly for standard users
    
    // Copy the provider's avatar into PocketBase if the user has none.
    if (authData.meta?.avatarUrl && !user.value.avatar) {
      try {
        const formData = new FormData()
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
    // authStore.clear() does not clear the cached file token, a bearer
    // credential valid up to 180s; the next login on this tab would inherit it.
    resetFileToken()
    user.value = null
    memberships.value = []
    currentOrgId.value = null
    isSuperAdmin.value = false
  }
  
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
      await switchOrganization(firstOrgId)
    } else {
      currentOrgId.value = null
    }
  }
  
  async function switchOrganization(orgId: string) {
    if (!user.value || currentOrgId.value === orgId) return
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    // Persist before flipping local state: API rules read the server-side
    // `current_organization`, and consumers fire requests as soon as
    // currentOrgId changes.
    try {
      await pb.collection(collection).update(user.value.id, { current_organization: orgId })
    } catch (e) {
      console.warn('Organization persistence failed:', e)
      useToast().error('Could not switch organization. Please try again.')
      return
    }
    currentOrgId.value = orgId
    user.value.current_organization = orgId
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
  
  // Memoized so the router guard and main.ts share one in-flight promise. The
  // router starts the initial navigation before main.ts calls this, and until
  // memberships load every capability reads false, so the guard must await it.
  // Memoized rather than "call it earlier in main.ts": an ordering convention
  // between two files is what broke before.
  // No reset on logout: the cleared authStore makes a later await a no-op.
  let hydration: Promise<void> | null = null

  function initializeFromAuth(): Promise<void> {
    hydration ??= hydrate()
    return hydration
  }

  async function hydrate() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as unknown as User
      isSuperAdmin.value = pb.authStore.model.collectionName === '_superusers'
      try {
        // Verify the stored token server-side before trusting it.
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
    user,
    memberships,
    currentOrgId,
    isSuperAdmin,
    authProviders,

    isAuthenticated,
    currentMembership,
    currentOrg,
    currentNatsUser,
    userRole,
    can,
    isDashboardUser,
    isOperator,
    canManageOrganizations,

    login,
    loginWithOAuth2,
    loadAuthMethods,
    logout,
    requestPasswordReset,
    loadMemberships: loadContext,
    switchOrganization,
    updateCurrentMembership,
    leaveOrganization,
    initializeFromAuth,
  }
})
