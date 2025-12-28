<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'

const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()

const token = ref('')
const loading = ref(false)

/**
 * Handle invitation acceptance
 * Calls custom endpoint from pb-tenancy library
 */
async function handleAccept() {
  if (!token.value.trim()) {
    toast.error('Please enter an invitation token')
    return
  }
  
  loading.value = true
  
  try {
    // CORRECTED: Endpoint path updated to /api/tenancy/accept-invite
    await pb.send('/api/tenancy/accept-invite', {
      method: 'POST',
      body: { token: token.value },
    })
    
    // Reload memberships to get the new organization
    await authStore.loadMemberships()
    
    if (authStore.memberships.length > 0) {
      // Find the membership for the newly joined org (likely the last one added)
      // or default to the last one in the list
      const newMembership = authStore.memberships[authStore.memberships.length - 1]
      
      // Switch context to that organization
      await authStore.switchOrganization(newMembership.organization)
      
      toast.success('Invitation accepted!')
      router.push('/')
    } else {
      // Edge case: Accept succeeded but membership query came back empty
      toast.warning('Invitation accepted, but could not load organization details')
      router.push('/')
    }
  } catch (err: any) {
    // Handle specific error cases if needed
    console.error('Invite Error:', err)
    toast.error(err.data?.message || err.message || 'Failed to accept invitation')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200">
    <div class="card w-96 bg-base-100 shadow-xl">
      <div class="card-body">
        <!-- Title -->
        <h2 class="card-title text-2xl mb-2">Accept Invitation</h2>
        <p class="text-sm text-base-content/70 mb-6">
          Enter the invitation token you received via email to join an organization.
        </p>
        
        <!-- Form -->
        <form @submit.prevent="handleAccept" class="space-y-4">
          <!-- Token Input -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Invitation Token</span>
            </label>
            <input 
              v-model="token"
              type="text" 
              placeholder="Paste token here..." 
              class="input input-bordered font-mono" 
              required
            />
            <label class="label">
              <span class="label-text-alt">
                Check your email for the invitation token
              </span>
            </label>
          </div>
          
          <!-- Submit Button -->
          <div class="form-control mt-6">
            <button 
              type="submit" 
              class="btn btn-primary"
              :disabled="loading"
            >
              <span v-if="loading" class="loading loading-spinner"></span>
              <span v-else>Accept Invitation</span>
            </button>
          </div>
        </form>
        
        <!-- Back to Login (Logout) -->
        <div class="divider">OR</div>
        <button 
          @click="router.push('/login')" 
          class="btn btn-ghost btn-sm"
        >
          Back to Login
        </button>
      </div>
    </div>
  </div>
</template>
