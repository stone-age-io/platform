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
  
  const canManageUsers = computed(() => {
    return ['owner', 'admin'].includes(userRole.value)
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
      authProviders.value = result.authProviders
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
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    try {
      await pb.collection(collection).update(user.value.id, { current_organization: orgId })
    } catch (e) { console.warn('Organization persistence failed:', e) }
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
    authProviders, // Exported
    
    // Computed
    isAuthenticated,
    currentMembership,
    currentOrg,
    currentNatsUser,
    userRole,
    canManageUsers,
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
