<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const roleId = route.params.id as string | undefined
const isEdit = computed(() => !!roleId)

// Form data
const formData = ref({
  name: '',
  description: '',
  is_default: false,
  publish_permissions: '',
  subscribe_permissions: '',
  max_subscriptions: '-1',
  max_data: '-1',
  max_payload: '-1',
})

// State
const loading = ref(false)

/**
 * Load existing role for editing
 */
async function loadRole() {
  if (!roleId) return
  
  loading.value = true
  
  try {
    const role = await pb.collection('nats_roles').getOne<NatsRole>(roleId)
    
    formData.value = {
      name: role.name,
      description: role.description || '',
      is_default: role.is_default || false,
      publish_permissions: role.publish_permissions || '',
      subscribe_permissions: role.subscribe_permissions || '',
      max_subscriptions: role.max_subscriptions?.toString() || '-1',
      max_data: role.max_data?.toString() || '-1',
      max_payload: role.max_payload?.toString() || '-1',
    }
  } catch (err: any) {
    toast.error('Failed to load role')
    router.push('/nats/roles')
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
      name: formData.value.name,
      description: formData.value.description || null,
      is_default: formData.value.is_default,
      publish_permissions: formData.value.publish_permissions || null,
      subscribe_permissions: formData.value.subscribe_permissions || null,
      max_subscriptions: parseInt(formData.value.max_subscriptions),
      max_data: parseInt(formData.value.max_data),
      max_payload: parseInt(formData.value.max_payload),
    }
    
    if (isEdit.value) {
      await pb.collection('nats_roles').update(roleId!, data)
      toast.success('Role updated')
    } else {
      data.organization = authStore.currentOrgId
      await pb.collection('nats_roles').create(data)
      toast.success('Role created')
    }
    
    router.push('/nats/roles')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save role')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (isEdit.value) {
    loadRole()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/roles">NATS Roles</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit NATS Role' : 'Create NATS Role' }}
      </h1>
    </div>
    
    <!-- Form -->
    <form @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Basic Info & Limits -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <!-- Name -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Name *</span>
                </label>
                <input 
                  v-model="formData.name"
                  type="text" 
                  placeholder="Enter role name"
                  class="input input-bordered"
                  required
                />
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
              
              <!-- Is Default -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.is_default"
                    type="checkbox" 
                    class="checkbox"
                  />
                  <span class="label-text">
                    <span class="font-medium">Default Role</span>
                    <span class="block text-sm text-base-content/70 mt-1">
                      Automatically assign this role to new NATS users
                    </span>
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Resource Limits">
            <div class="grid grid-cols-1 gap-4">
              <!-- Max Subscriptions -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Max Subscriptions</span>
                </label>
                <input 
                  v-model="formData.max_subscriptions"
                  type="number"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">-1 = Unlimited</span>
                </label>
              </div>
              
              <!-- Max Data -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Max Data (bytes)</span>
                </label>
                <input 
                  v-model="formData.max_data"
                  type="number"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">-1 = Unlimited</span>
                </label>
              </div>
              
              <!-- Max Payload -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Max Payload (bytes)</span>
                </label>
                <input 
                  v-model="formData.max_payload"
                  type="number"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">-1 = Unlimited</span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Permissions -->
        <div class="space-y-6">
          <BaseCard title="Permissions">
            <div class="space-y-4">
              <!-- Publish Permissions -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Publish Permissions</span>
                </label>
                <textarea 
                  v-model="formData.publish_permissions"
                  class="textarea textarea-bordered font-mono"
                  rows="8"
                  placeholder="subjects.>"
                ></textarea>
                <label class="label">
                  <span class="label-text-alt">
                    Comma-separated list of subjects this role can publish to. Use '>' for wildcards.
                  </span>
                </label>
              </div>
              
              <!-- Subscribe Permissions -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Subscribe Permissions</span>
                </label>
                <textarea 
                  v-model="formData.subscribe_permissions"
                  class="textarea textarea-bordered font-mono"
                  rows="8"
                  placeholder="subjects.>"
                ></textarea>
                <label class="label">
                  <span class="label-text-alt">
                    Comma-separated list of subjects this role can subscribe to. Use '>' for wildcards.
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
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Role</span>
        </button>
      </div>
    </form>
  </div>
</template>
