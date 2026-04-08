<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsAccountImport } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const importId = route.params.id as string | undefined
const isEdit = computed(() => !!importId)
const loading = ref(false)
const accountId = ref('')

const formData = ref({
  name: '',
  subject: '',
  account: '',
  token: '',
  local_subject: '',
  type: 'stream' as 'stream' | 'service',
  share: false,
  allow_trace: false,
  description: '',
})

async function resolveAccountId() {
  if (!authStore.currentOrgId) return
  try {
    const acc = await pb.collection('nats_accounts').getFirstListItem(
      `organization = "${authStore.currentOrgId}"`
    )
    accountId.value = acc.id
  } catch {
    toast.error('No NATS account found for this organization')
  }
}

async function loadImport() {
  if (!importId) return
  loading.value = true
  try {
    const rec = await pb.collection('nats_account_imports').getOne<NatsAccountImport>(importId)
    accountId.value = rec.account_id
    formData.value = {
      name: rec.name,
      subject: rec.subject,
      account: rec.account,
      token: rec.token || '',
      local_subject: rec.local_subject || '',
      type: rec.type,
      share: rec.share || false,
      allow_trace: rec.allow_trace || false,
      description: rec.description || '',
    }
  } catch (err: any) {
    toast.error('Failed to load import')
    router.push('/nats/imports')
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  if (!accountId.value) {
    toast.error('No NATS account available')
    return
  }

  loading.value = true
  try {
    const data: any = {
      account_id: accountId.value,
      name: formData.value.name,
      subject: formData.value.subject,
      account: formData.value.account,
      token: formData.value.token || null,
      local_subject: formData.value.local_subject || null,
      type: formData.value.type,
      share: formData.value.share,
      allow_trace: formData.value.allow_trace,
      description: formData.value.description || null,
    }

    if (isEdit.value) {
      await pb.collection('nats_account_imports').update(importId!, data)
      toast.success('Import updated')
    } else {
      data.organization = authStore.currentOrgId
      await pb.collection('nats_account_imports').create(data)
      toast.success('Import created')
    }
    router.push('/nats/imports')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save import')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  if (isEdit.value) {
    await loadImport()
  } else {
    await resolveAccountId()
  }
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/imports">NATS Imports</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit Import' : 'Create Import' }}</h1>
    </div>

    <form @submit.prevent="handleSubmit" class="space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">

        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Name *</span></label>
                <input v-model="formData.name" type="text" class="input input-bordered" required maxlength="100" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Subject *</span></label>
                <input v-model="formData.subject" type="text" class="input input-bordered font-mono" required maxlength="500" placeholder="sensors.>" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Type *</span></label>
                <select v-model="formData.type" class="select select-bordered">
                  <option value="stream">Stream</option>
                  <option value="service">Service</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Description</span></label>
                <textarea v-model="formData.description" class="textarea textarea-bordered" rows="2" maxlength="500"></textarea>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Routing">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Local Subject</span></label>
                <input v-model="formData.local_subject" type="text" class="input input-bordered font-mono" maxlength="500" placeholder="Leave empty to use subject as-is" />
                <label class="label"><span class="label-text-alt">Remap the imported subject locally (supports $1, $2 references)</span></label>
              </div>
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.share" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Share</span>
                    <span class="block text-xs text-base-content/60 mt-1">Enable latency tracking for this service import.</span>
                  </span>
                </label>
              </div>
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.allow_trace" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Allow Trace</span>
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>

        <div class="space-y-6">
          <BaseCard title="Source Account">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Account Public Key *</span></label>
                <input v-model="formData.account" type="text" class="input input-bordered font-mono" required maxlength="200" placeholder="AABC...exporting account public key" />
                <label class="label"><span class="label-text-alt">The public key of the account exporting the subject</span></label>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Activation Token</span></label>
                <textarea v-model="formData.token" class="textarea textarea-bordered font-mono text-xs" rows="6" maxlength="10000" placeholder="Paste activation JWT if required by the export"></textarea>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>

      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button type="button" @click="router.back()" class="btn btn-ghost order-2 sm:order-1" :disabled="loading">Cancel</button>
        <button type="submit" class="btn btn-primary order-1 sm:order-2" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Import</span>
        </button>
      </div>
    </form>
  </div>
</template>
