<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsUser, NatsAccount, NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const userId = route.params.id as string | undefined
const isEdit = computed(() => !!userId)

// Form data
const formData = ref({
  nats_username: '',
  description: '',
  account_id: '',
  role_id: '',
  bearer_token: false,
  active: true,
  regenerate: false,
})

// Relation options
const accounts = ref<NatsAccount[]>([])
const roles = ref<NatsRole[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load form options (accounts and roles)
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [accountsResult, rolesResult] = await Promise.all([
      pb.collection('nats_accounts').getFullList<NatsAccount>({ 
        sort: 'name',
        filter: 'active = true'
      }),
      pb.collection('nats_roles').getFullList<NatsRole>({ 
        sort: 'name'
      }),
    ])
    
    accounts.value = accountsResult
    roles.value = rolesResult
    
    // Auto-select default role if creating new user
    if (!isEdit.value) {
      const defaultRole = roles.value.find(r => r.is_default)
      if (defaultRole) {
        formData.value.role_id = defaultRole.id
      }
      
      // Auto-select first account if only one exists
      if (accounts.value.length === 1) {
        formData.value.account_id = accounts.value[0].id
      }
    }
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing user for editing
 */
async function loadUser() {
  if (!userId) return
  
  loading.value = true
  
  try {
    const user = await pb.collection('nats_users').getOne<NatsUser>(userId)
    
    formData.value = {
      nats_username: user.nats_username,
      description: user.description || '',
      account_id: user.account_id,
      role_id: user.role_id,
      bearer_token: user.bearer_token || false,
      active: user.active || true,
      regenerate: false,
    }
  } catch (err: any) {
    toast.error('Failed to load NATS user')
    router.push('/nats/users')
  } finally {
    loading.value = false
  }
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  loading.value = true
  
  try {
    const data: any = {
      nats_username: formData.value.nats_username,
      description: formData.value.description || null,
      account_id: formData.value.account_id,
      role_id: formData.value.role_id,
      bearer_token: formData.value.bearer_token,
      active: formData.value.active,
    }
    
    if (isEdit.value) {
      // Update existing user
      data.regenerate = formData.value.regenerate
      await pb.collection('nats_users').update(userId!, data)
      toast.success('NATS user updated')
    } else {
      // Create new user
      // IMPORTANT: Frontend must set organization
      data.organization = authStore.currentOrgId
      
      await pb.collection('nats_users').create(data)
      toast.success('NATS user created')
    }
    
    router.push('/nats/users')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save NATS user')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadUser()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/users">NATS Users</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit NATS User' : 'Create NATS User' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      <BaseCard title="Basic Information">
        <div class="space-y-4">
          <!-- Username -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">NATS Username *</span>
            </label>
            <input 
              v-model="formData.nats_username"
              type="text" 
              placeholder="user.device.sensor1"
              class="input input-bordered font-mono"
              required
              :disabled="isEdit"
            />
            <label class="label">
              <span class="label-text-alt">
                {{ isEdit ? 'Username cannot be changed after creation' : 'Use dot notation for hierarchical naming' }}
              </span>
            </label>
          </div>
          
          <!-- Description -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Description</span>
            </label>
            <textarea 
              v-model="formData.description"
              class="textarea textarea-bordered"
              rows="3"
              placeholder="Optional description"
            ></textarea>
          </div>
        </div>
      </BaseCard>
      
      <!-- Account & Role -->
      <BaseCard title="Account & Permissions">
        <div class="space-y-4">
          <!-- Account -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">NATS Account *</span>
            </label>
            <select v-model="formData.account_id" class="select select-bordered" required>
              <option value="">Select an account</option>
              <option v-for="account in accounts" :key="account.id" :value="account.id">
                {{ account.name }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                The NATS account this user belongs to
              </span>
            </label>
          </div>
          
          <!-- Role -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Role *</span>
            </label>
            <select v-model="formData.role_id" class="select select-bordered" required>
              <option value="">Select a role</option>
              <option v-for="role in roles" :key="role.id" :value="role.id">
                {{ role.name }}
                <span v-if="role.is_default"> (Default)</span>
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                Defines permissions for this user
              </span>
            </label>
          </div>
        </div>
      </BaseCard>
      
      <!-- Options -->
      <BaseCard title="Options">
        <div class="space-y-4">
          <!-- Bearer Token -->
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input 
                v-model="formData.bearer_token"
                type="checkbox" 
                class="checkbox"
              />
              <span class="label-text">
                <span class="font-medium">Enable Bearer Token</span>
                <span class="block text-sm text-base-content/70 mt-1">
                  Allow authentication using bearer token
                </span>
              </span>
            </label>
          </div>
          
          <!-- Active -->
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input 
                v-model="formData.active"
                type="checkbox" 
                class="checkbox"
              />
              <span class="label-text">
                <span class="font-medium">Active</span>
                <span class="block text-sm text-base-content/70 mt-1">
                  User can authenticate and use NATS
                </span>
              </span>
            </label>
          </div>
          
          <!-- Regenerate (edit only) -->
          <div v-if="isEdit" class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input 
                v-model="formData.regenerate"
                type="checkbox" 
                class="checkbox checkbox-warning"
              />
              <span class="label-text">
                <span class="font-medium text-warning">Regenerate Credentials</span>
                <span class="block text-sm text-base-content/70 mt-1">
                  Generate new JWT and credentials. This will invalidate existing credentials.
                </span>
              </span>
            </label>
          </div>
        </div>
      </BaseCard>
      
      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="router.back()" 
          class="btn btn-ghost order-2 sm:order-1"
          :disabled="loading"
        >
          Cancel
        </button>
        <button 
          type="submit" 
          class="btn btn-primary order-1 sm:order-2"
          :disabled="loading"
        >
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} User</span>
        </button>
      </div>
    </form>
  </div>
</template>
