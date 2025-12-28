import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { User, Membership, Organization, SuperUser } from '@/types/pocketbase'

/**
 * Auth Store
 * 
 * Manages authentication state and organization context.
 * Supports dual-collection authentication (users and _superusers).
 * Implements "God Mode" for Super Admins by mapping all organizations to 
 * a membership-like structure.
 */
export const useAuthStore = defineStore('auth', () => {
  // ============================================================================
  // STATE
  // ============================================================================
  
  const user = ref<User | SuperUser | null>(null)
  const memberships = ref<Membership[]>([])
  const currentOrgId = ref<string | null>(null)
  const isSuperAdmin = ref(false) // Tracks if authenticated against _superusers
  
  // ============================================================================
  // COMPUTED
  // ============================================================================
  
  const isAuthenticated = computed(() => !!user.value)
  
  /**
   * Returns the membership record associated with the current active organization.
   */
  const currentMembership = computed(() => {
    if (!currentOrgId.value) return null
    return memberships.value.find(m => m.organization === currentOrgId.value)
  })
  
  /**
   * Returns the full organization record for the active context.
   */
  const currentOrg = computed((): Organization | null => {
    if (!currentMembership.value?.expand?.organization) return null
    return currentMembership.value.expand.organization as Organization
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
  
  /**
   * Login with email and password.
   * @param asSuperAdmin - If true, authenticates against the _superusers collection.
   */
  async function login(email: string, password: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    
    const authData = await pb.collection(collection).authWithPassword(email, password)
    
    user.value = authData.record as unknown as User
    isSuperAdmin.value = asSuperAdmin
    
    await loadContext()
  }

  /**
   * Triggers the PocketBase password reset email flow.
   */
  async function requestPasswordReset(email: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    return await pb.collection(collection).requestPasswordReset(email)
  }
  
  /**
   * Logout and clear all local and persistent auth state.
   */
  async function logout() {
    pb.authStore.clear()
    user.value = null
    memberships.value = []
    currentOrgId.value = null
    isSuperAdmin.value = false
  }
  
  /**
   * Internal Context Loader
   * - Normal User: Loads specific memberships via the memberships collection.
   * - Super Admin: Loads ALL organizations and maps them to a fake membership structure
   *   so the rest of the UI logic (Sidebar/Header) remains identical.
   */
  async function loadContext() {
    if (!user.value) return
    
    if (isSuperAdmin.value) {
      // GOD MODE: Fetch every organization in the system
      const allOrgs = await pb.collection('organizations').getFullList<Organization>({
        sort: 'name'
      })
      
      // Map to "Fake" Memberships
      memberships.value = allOrgs.map(org => ({
        id: `super_${org.id}`,
        created: org.created,
        updated: org.updated,
        user: user.value!.id,
        organization: org.id,
        role: 'owner' as const,
        expand: { organization: org }
      }))
    } else {
      // NORMAL MODE: Fetch user's actual assigned memberships
      memberships.value = await pb.collection('memberships').getFullList<Membership>({
        filter: `user = "${user.value.id}"`,
        expand: 'organization',
      })
    }
    
    // Determine the initial organization context
    // 1. Try saved current_organization from the user record
    if (user.value.current_organization) {
      const exists = memberships.value.find(m => m.organization === user.value?.current_organization)
      if (exists) {
        currentOrgId.value = user.value.current_organization
        return
      }
    }
    
    // 2. Fallback to the first available organization
    if (memberships.value.length > 0) {
      const firstOrgId = memberships.value[0].organization
      // Don't await the persist call here to allow faster UI loading
      switchOrganization(firstOrgId)
    }
  }
  
  /**
   * Switch to a different organization.
   * Updates local state and persists the choice to the user record on the backend.
   */
  async function switchOrganization(orgId: string) {
    if (!user.value || currentOrgId.value === orgId) return
    
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    
    // Persist choice to the backend (background)
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    try {
      await pb.collection(collection).update(user.value.id, {
        current_organization: orgId,
      })
    } catch (e) {
      // We log but don't throw, as the local session is already updated correctly
      console.warn('Organization persistence failed:', e)
    }
    
    // Dispatch global event so views can reactively reload their data
    window.dispatchEvent(new CustomEvent('organization-changed', { 
      detail: { orgId } 
    }))
  }
  
  /**
   * Restore auth state from the PocketBase SDK Store.
   * Called once during application mount.
   */
  function initializeFromAuth() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as unknown as User
      
      // Determine if we are superadmin based on the collection name of the stored model
      isSuperAdmin.value = pb.authStore.model.collectionName === '_superusers'
      
      loadContext()
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
    userRole,
    canManageUsers,
    
    // Actions
    login,
    logout,
    requestPasswordReset,
    loadMemberships: loadContext,
    switchOrganization,
    initializeFromAuth,
  }
})
