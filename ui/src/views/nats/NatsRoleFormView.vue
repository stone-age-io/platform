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
const loading = ref(false)

const formData = ref({
  name: '',
  description: '',
  is_default: false,
  publish_permissions: '',
  publish_deny_permissions: '',
  subscribe_permissions: '',
  subscribe_deny_permissions: '',
  max_subscriptions: '-1',
  max_data: '-1',
  max_payload: '-1',
})

/**
 * Grug helper: UI needs string for textarea, API wants Array.
 * This takes the JSON field (Array) and makes it a comma-separated string.
 */
function formatForInput(val: any): string {
  if (!val) return ''
  if (Array.isArray(val)) return val.join(', ')
  return String(val)
}

/**
 * Grug helper: API wants Array, UI gives string.
 * This cleans up the comma-separated input into a clean Array.
 */
function formatForApi(val: string): string[] {
  if (!val) return []
  return val.split(',')
    .map(s => s.trim())
    .filter(s => s !== '')
}

async function loadRole() {
  if (!roleId) return
  loading.value = true
  try {
    const role = await pb.collection('nats_roles').getOne<NatsRole>(roleId)
    formData.value = {
      name: role.name,
      description: role.description || '',
      is_default: role.is_default || false,
      // Map JSON arrays to strings for the UI
      publish_permissions: formatForInput(role.publish_permissions),
      publish_deny_permissions: formatForInput(role.publish_deny_permissions),
      subscribe_permissions: formatForInput(role.subscribe_permissions),
      subscribe_deny_permissions: formatForInput(role.subscribe_deny_permissions),
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

async function handleSubmit() {
  loading.value = true
  try {
    const data: any = {
      name: formData.value.name,
      description: formData.value.description || null,
      is_default: formData.value.is_default,
      // Send clean arrays to the API
      publish_permissions: formatForApi(formData.value.publish_permissions),
      publish_deny_permissions: formatForApi(formData.value.publish_deny_permissions),
      subscribe_permissions: formatForApi(formData.value.subscribe_permissions),
      subscribe_deny_permissions: formatForApi(formData.value.subscribe_deny_permissions),
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

onMounted(() => { if (isEdit.value) loadRole() })
</script>

<template>
  <!-- Template remains the same as previous version -->
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/roles">NATS Roles</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit NATS Role' : 'Create NATS Role' }}</h1>
    </div>
    
    <form @submit.prevent="handleSubmit" class="space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Name *</span></label>
                <input v-model="formData.name" type="text" class="input input-bordered" required />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Description</span></label>
                <textarea v-model="formData.description" class="textarea textarea-bordered" rows="2"></textarea>
              </div>
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.is_default" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Default Role</span>
                    <span class="block text-xs text-base-content/60 mt-1">Automatically assign to new users.</span>
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Resource Limits">
            <div class="grid grid-cols-1 gap-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Max Subscriptions</span></label>
                <input v-model="formData.max_subscriptions" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Data (bytes)</span></label>
                <input v-model="formData.max_data" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Payload (bytes)</span></label>
                <input v-model="formData.max_payload" type="number" class="input input-bordered font-mono" />
              </div>
              <p class="text-[10px] opacity-50 italic px-1">-1 = Unlimited</p>
            </div>
          </BaseCard>
        </div>

        <div class="space-y-6">
          <BaseCard title="Permissions">
            <div class="space-y-6">
              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📤</span>
                  <h4 class="text-xs font-black uppercase opacity-50 tracking-widest">Publishing</h4>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs">Allow Subjects</span></label>
                  <textarea v-model="formData.publish_permissions" class="textarea textarea-bordered font-mono text-xs" rows="3" placeholder="sensors.data, logs.>"></textarea>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs text-error font-bold">Deny Subjects</span></label>
                  <textarea v-model="formData.publish_deny_permissions" class="textarea textarea-bordered font-mono text-xs border-error/30 focus:border-error" rows="3" placeholder="admin.>"></textarea>
                </div>
              </div>

              <div class="divider my-0 opacity-30"></div>

              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📥</span>
                  <h4 class="text-xs font-black uppercase opacity-50 tracking-widest">Subscribing</h4>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs">Allow Subjects</span></label>
                  <textarea v-model="formData.subscribe_permissions" class="textarea textarea-bordered font-mono text-xs" rows="3" placeholder=">"></textarea>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs text-error font-bold">Deny Subjects</span></label>
                  <textarea v-model="formData.subscribe_deny_permissions" class="textarea textarea-bordered font-mono text-xs border-error/30 focus:border-error" rows="3" placeholder="secrets.>"></textarea>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
      
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button type="button" @click="router.back()" class="btn btn-ghost order-2 sm:order-1" :disabled="loading">Cancel</button>
        <button type="submit" class="btn btn-primary order-1 sm:order-2" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Role</span>
        </button>
      </div>
    </form>
  </div>
</template>
