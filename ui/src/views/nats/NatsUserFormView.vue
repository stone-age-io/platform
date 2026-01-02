<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsUser, NatsAccount, NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: NatsUser): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const userId = route.params.id as string | undefined
const isEdit = computed(() => !!userId && !props.embedded)

// Form data
const formData = ref({
  // Identity
  nats_username: '',
  description: '',
  
  // Auth
  email: '',
  password: '',
  passwordConfirm: '',
  
  // Permissions
  account_id: '',
  role_id: '',
  
  // Options
  bearer_token: false,
  active: true,
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
    
    // Auto-select defaults
    if (!isEdit.value) {
      const defaultRole = roles.value.find(r => r.is_default)
      if (defaultRole) formData.value.role_id = defaultRole.id
      if (accounts.value.length === 1) formData.value.account_id = accounts.value[0].id
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
  if (!userId || props.embedded) return
  
  loading.value = true
  
  try {
    const user = await pb.collection('nats_users').getOne<NatsUser>(userId)
    
    formData.value = {
      nats_username: user.nats_username,
      description: user.description || '',
      email: user.email,
      password: '',
      passwordConfirm: '',
      account_id: user.account_id,
      role_id: user.role_id,
      bearer_token: user.bearer_token || false,
      active: user.active || true,
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
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new users')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  loading.value = true
  
  try {
    const data: any = {
      nats_username: formData.value.nats_username,
      description: formData.value.description || null,
      email: formData.value.email,
      emailVisibility: true,
      account_id: formData.value.account_id,
      role_id: formData.value.role_id,
      bearer_token: formData.value.bearer_token,
      active: formData.value.active,
    }

    // Only send password if entered
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    let record: NatsUser

    if (isEdit.value) {
      record = await pb.collection('nats_users').update<NatsUser>(userId!, data)
      toast.success('NATS user updated')
    } else {
      data.organization = authStore.currentOrgId
      record = await pb.collection('nats_users').create<NatsUser>(data)
      toast.success('NATS user created')
    }
    
    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/nats/users')
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to save NATS user')
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('cancel')
  } else {
    router.back()
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
    <!-- Header: Hidden if Embedded -->
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/users">NATS Users</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit NATS User' : 'Provision NATS User' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Identity">
            <div class="space-y-4">
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

          <BaseCard title="Authentication">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Email (Identity) *</span>
                </label>
                <input 
                  v-model="formData.email"
                  type="email" 
                  placeholder="device-uuid@nats.local"
                  class="input input-bordered"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Unique email used for PocketBase authentication record.</span>
                </label>
              </div>
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.password"
                    type="password" 
                    class="input input-bordered"
                    :required="!isEdit"
                    minlength="8"
                  />
                </div>
                
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Confirm Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.passwordConfirm"
                    type="password" 
                    class="input input-bordered"
                    :required="!!formData.password"
                  />
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Account & Permissions">
            <div class="space-y-4">
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
              </div>
              
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
              </div>
            </div>
          </BaseCard>
          
          <BaseCard title="Security Settings">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.active"
                    type="checkbox" 
                    class="toggle toggle-success"
                  />
                  <span class="label-text">
                    <span class="font-medium">Active Status</span>
                    <span class="block text-sm text-base-content/70">
                      Allow this user to authenticate and connect
                    </span>
                  </span>
                </label>
              </div>

              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.bearer_token"
                    type="checkbox" 
                    class="checkbox"
                  />
                  <span class="label-text">
                    <span class="font-medium">Enable Bearer Token</span>
                    <span class="block text-sm text-base-content/70">
                      Generate a long-lived bearer token for simplified auth
                    </span>
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="handleCancel" 
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} User</span>
        </button>
      </div>
    </form>
  </div>
</template>
