import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { User, Membership, Organization, SuperUser, NatsUser } from '@/types/pocketbase'

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
  
  // ============================================================================
  // COMPUTED
  // ============================================================================
  
  const isAuthenticated = computed(() => !!user.value)
  
  /**
   * Returns the membership record associated with the current active organization.
   */
  const currentMembership = computed(() => {
    if (!currentOrgId.value) return null
    return memberships.value.find(m => m.organization === currentOrgId.value) || null
  })
  
  /**
   * Returns the full organization record for the active context.
   */
  const currentOrg = computed((): Organization | null => {
    return currentMembership.value?.expand?.organization || null
  })

  /**
   * Returns the NATS User credentials associated with the CURRENT context.
   * This is what nats.ts should watch.
   */
  const currentNatsUser = computed((): NatsUser | null => {
    return currentMembership.value?.expand?.nats_user || null
  })
  
  /**
   * Determines the role of the user. Super Admins are hardcoded to 'owner'.
   */
  const userRole = computed(() => {
    if (isSuperAdmin.value) return 'owner'
    return currentMembership.value?.role || 'member'
  })
  
  /**
   * Checks if user has administrative privileges (Owner or Admin).
   */
  const canManageUsers = computed(() => {
    return ['owner', 'admin'].includes(userRole.value)
  })
  
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
  
  /**
   * Internal Context Loader
   * Fetches memberships with expansions for Organization and NATS User.
   */
  async function loadContext() {
    if (!user.value) return
    
    if (isSuperAdmin.value) {
      // GOD MODE: Fetch all organizations, map to fake memberships
      const allOrgs = await pb.collection('organizations').getFullList<Organization>({ sort: 'name' })
      
      memberships.value = allOrgs.map(org => ({
        id: `super_${org.id}`, // Fake ID
        created: org.created,
        updated: org.updated,
        user: user.value!.id,
        organization: org.id,
        role: 'owner' as const,
        expand: { 
          organization: org,
          // SuperUsers don't have auto-linked NATS users in God Mode
          // They must join as a real member to have a persistent identity
          nats_user: undefined 
        }
      }))
    } else {
      // NORMAL MODE: Fetch actual memberships with NATS identity
      // Important: Expand nats_user to get the .creds
      memberships.value = await pb.collection('memberships').getFullList<ExtendedMembership>({
        filter: `user = "${user.value.id}"`,
        expand: 'organization,nats_user',
      })
    }
    
    // Determine initial context
    // 1. Try saved current_organization from user record
    if (user.value.current_organization) {
      const exists = memberships.value.find(m => m.organization === user.value?.current_organization)
      if (exists) {
        currentOrgId.value = user.value.current_organization
        return
      }
    }
    
    // 2. Fallback to first available
    if (memberships.value.length > 0) {
      const firstOrgId = memberships.value[0].organization
      // Don't await persistence to keep UI snappy
      switchOrganization(firstOrgId)
    } else {
      currentOrgId.value = null
    }
  }
  
  /**
   * Switch to a different organization.
   */
  async function switchOrganization(orgId: string) {
    if (!user.value || currentOrgId.value === orgId) return
    
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    
    // Persist choice
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    try {
      await pb.collection(collection).update(user.value.id, {
        current_organization: orgId,
      })
    } catch (e) {
      console.warn('Organization persistence failed:', e)
    }
    
    // Dispatch global event for NATS reconnection and view reloads
    window.dispatchEvent(new CustomEvent('organization-changed', { 
      detail: { orgId } 
    }))
  }

  /**
   * Update the current membership (e.g. assigning a NATS user)
   * This updates the local store immediately, then the backend.
   */
  async function updateCurrentMembership(data: Partial<Membership>) {
    if (!currentMembership.value || isSuperAdmin.value) return

    const memId = currentMembership.value.id
    
    // 1. Optimistic Update (partial)
    const idx = memberships.value.findIndex(m => m.id === memId)
    if (idx !== -1) {
      memberships.value[idx] = { ...memberships.value[idx], ...data }
    }

    // 2. Backend Update
    try {
      const updated = await pb.collection('memberships').update<ExtendedMembership>(memId, data, {
        expand: 'organization,nats_user'
      })
      // 3. Full State Refresh from response
      if (idx !== -1) memberships.value[idx] = updated
    } catch (e) {
      console.error('Failed to update membership', e)
      // Revert on failure (reload context)
      await loadContext()
      throw e
    }
  }

  /**
   * Leave the current organization
   */
  async function leaveOrganization(orgId: string) {
    if (isSuperAdmin.value) return 

    const membership = memberships.value.find(m => m.organization === orgId)
    if (!membership) return

    if (membership.role === 'owner') {
      throw new Error("Owners cannot leave their own organization. Transfer ownership or delete the organization first.")
    }

    await pb.collection('memberships').delete(membership.id)
    
    // Refresh context
    await loadContext()
  }
  
  async function initializeFromAuth() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as unknown as User
      isSuperAdmin.value = pb.authStore.model.collectionName === '_superusers'
      
      await loadContext()

      try {
        const collection = isSuperAdmin.value ? '_superusers' : 'users'
        const authData = await pb.collection(collection).authRefresh()
        user.value = authData.record as unknown as User
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
    
    // Computed
    isAuthenticated,
    currentMembership,
    currentOrg,
    currentNatsUser,
    userRole,
    canManageUsers,
    
    // Actions
    login,
    logout,
    requestPasswordReset,
    loadMemberships: loadContext,
    switchOrganization,
    updateCurrentMembership,
    leaveOrganization,
    initializeFromAuth,
  }
})
