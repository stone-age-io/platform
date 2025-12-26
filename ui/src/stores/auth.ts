import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { User, Membership, Organization } from '@/types/pocketbase'

/**
 * Auth Store
 * 
 * Manages authentication state and organization context.
 * This is the single source of truth for:
 * - Current user
 * - User's memberships
 * - Current organization context
 * - User's role in current org
 */
export const useAuthStore = defineStore('auth', () => {
  // ============================================================================
  // STATE
  // ============================================================================
  
  const user = ref<User | null>(null)
  const memberships = ref<Membership[]>([])
  const currentOrgId = ref<string | null>(null)
  
  // ============================================================================
  // COMPUTED
  // ============================================================================
  
  const isAuthenticated = computed(() => !!user.value)
  
  const currentMembership = computed(() => {
    if (!currentOrgId.value) return null
    return memberships.value.find(m => m.organization === currentOrgId.value)
  })
  
  const currentOrg = computed((): Organization | null => {
    if (!currentMembership.value?.expand?.organization) return null
    return currentMembership.value.expand.organization as Organization
  })
  
  const userRole = computed(() => {
    return currentMembership.value?.role || 'member'
  })
  
  const canManageUsers = computed(() => {
    return ['owner', 'admin'].includes(userRole.value)
  })
  
  // ============================================================================
  // ACTIONS
  // ============================================================================
  
  /**
   * Login with email and password
   */
  async function login(email: string, password: string) {
    const authData = await pb.collection('users').authWithPassword(email, password)
    user.value = authData.record as User
    await loadMemberships()
    
    // Set initial org context
    if (user.value.current_organization) {
      currentOrgId.value = user.value.current_organization
    } else if (memberships.value.length > 0) {
      // Default to first membership if no current_organization set
      currentOrgId.value = memberships.value[0].organization
    }
  }
  
  /**
   * Logout and clear all state
   */
  async function logout() {
    pb.authStore.clear()
    user.value = null
    memberships.value = []
    currentOrgId.value = null
  }
  
  /**
   * Load user's memberships with expanded organization data
   * 
   * Note: We DO filter by user here because we need to know which
   * organizations this specific user has access to (for the org switcher).
   * This is different from viewing all memberships as an admin,
   * which we'll handle separately in the organization management views.
   */
  async function loadMemberships() {
    if (!user.value) return
    
    memberships.value = await pb.collection('memberships').getFullList<Membership>({
      filter: `user = "${user.value.id}"`,
      expand: 'organization',
    })
  }
  
  /**
   * Switch to a different organization
   * This updates the user's current_organization and emits an event
   * for all views to reactively refresh their data
   */
  async function switchOrganization(orgId: string) {
    if (!user.value) return
    
    // Update user's current_organization in PocketBase
    await pb.collection('users').update(user.value.id, {
      current_organization: orgId,
    })
    
    // Update local state
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    
    // Emit custom event for views to listen to
    // Views can reload their data when this event fires
    window.dispatchEvent(new CustomEvent('organization-changed', { 
      detail: { orgId } 
    }))
  }
  
  /**
   * Initialize auth state from PocketBase authStore
   * Called on app mount to restore session
   */
  function initializeFromAuth() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as User
      loadMemberships()
      currentOrgId.value = user.value.current_organization || null
    }
  }
  
  // ============================================================================
  // RETURN PUBLIC API
  // ============================================================================
  
  return {
    // State
    user,
    memberships,
    currentOrgId,
    
    // Computed
    isAuthenticated,
    currentMembership,
    currentOrg,
    userRole,
    canManageUsers,
    
    // Actions
    login,
    logout,
    loadMemberships,
    switchOrganization,
    initializeFromAuth,
  }
})
