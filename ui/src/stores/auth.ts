import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pb } from '@/utils/pb'
import type { User, Membership, Organization, SuperUser } from '@/types/pocketbase'

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | SuperUser | null>(null)
  const memberships = ref<Membership[]>([])
  const currentOrgId = ref<string | null>(null)
  const isSuperAdmin = ref(false) // Track login type
  
  // Computed
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
    // Superadmins are effectively owners of everything
    if (isSuperAdmin.value) return 'owner'
    return currentMembership.value?.role || 'member'
  })
  
  const canManageUsers = computed(() => {
    return ['owner', 'admin'].includes(userRole.value)
  })
  
  // Actions
  async function login(email: string, password: string, asSuperAdmin = false) {
    const collection = asSuperAdmin ? '_superusers' : 'users'
    
    // Auth against specific collection
    const authData = await pb.collection(collection).authWithPassword(email, password)
    
    user.value = authData.record as unknown as User
    isSuperAdmin.value = asSuperAdmin
    
    await loadContext()
  }
  
  async function logout() {
    pb.authStore.clear()
    user.value = null
    memberships.value = []
    currentOrgId.value = null
    isSuperAdmin.value = false
  }
  
  /**
   * Smart Context Loader
   * - Normal User: Loads Memberships
   * - Super Admin: Loads ALL Organizations and maps them to fake memberships
   */
  async function loadContext() {
    if (!user.value) return
    
    if (isSuperAdmin.value) {
      // GOD MODE: Fetch all organizations
      const allOrgs = await pb.collection('organizations').getFullList<Organization>({
        sort: 'name'
      })
      
      // Map to "Fake" Memberships so the UI thinks we belong to them
      memberships.value = allOrgs.map(org => ({
        id: `super_${org.id}`,
        created: new Date().toISOString(),
        updated: new Date().toISOString(),
        user: user.value!.id,
        organization: org.id,
        role: 'owner', // Superadmin is owner of all
        expand: { organization: org }
      }))
    } else {
      // NORMAL MODE: Fetch specific memberships
      memberships.value = await pb.collection('memberships').getFullList<Membership>({
        filter: `user = "${user.value.id}"`,
        expand: 'organization',
      })
    }
    
    // Set initial org context
    // 1. Try saved current_organization from record
    if (user.value.current_organization) {
      // Verify it exists in our list (permissions check)
      const exists = memberships.value.find(m => m.organization === user.value?.current_organization)
      if (exists) {
        currentOrgId.value = user.value.current_organization
        return
      }
    }
    
    // 2. Fallback to first available org
    if (memberships.value.length > 0) {
      const firstOrgId = memberships.value[0].organization
      // We don't await this update to speed up UI load
      switchOrganization(firstOrgId)
    }
  }
  
  async function switchOrganization(orgId: string) {
    if (!user.value) return
    
    currentOrgId.value = orgId
    user.value.current_organization = orgId
    
    // Persist to backend
    const collection = isSuperAdmin.value ? '_superusers' : 'users'
    try {
      await pb.collection(collection).update(user.value.id, {
        current_organization: orgId,
      })
    } catch (e) {
      // If superuser record is locked/immutable, we just ignore the save error
      // The local state is already updated for this session
      console.warn('Could not persist org switch:', e)
    }
    
    window.dispatchEvent(new CustomEvent('organization-changed', { 
      detail: { orgId } 
    }))
  }
  
  function initializeFromAuth() {
    if (pb.authStore.isValid && pb.authStore.model) {
      user.value = pb.authStore.model as unknown as User
      
      // Determine if we are superadmin based on collection name
      isSuperAdmin.value = pb.authStore.model.collectionName === '_superusers'
      
      loadContext()
    }
  }
  
  return {
    user,
    memberships,
    currentOrgId,
    isSuperAdmin, // Exported for UI to show badges if needed
    isAuthenticated,
    currentMembership,
    currentOrg,
    userRole,
    canManageUsers,
    login,
    logout,
    loadMemberships: loadContext, // Renamed internally for cleaner API
    switchOrganization,
    initializeFromAuth,
  }
})
