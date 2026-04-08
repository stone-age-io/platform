<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsAccountExport } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const exportId = route.params.id as string | undefined
const isEdit = computed(() => !!exportId)
const loading = ref(false)
const accountId = ref('')

const formData = ref({
  name: '',
  subject: '',
  type: 'stream' as 'stream' | 'service',
  description: '',
  token_req: false,
  response_type: '',
  response_threshold: '',
  account_token_position: '',
  advertise: false,
  allow_trace: false,
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

async function loadExport() {
  if (!exportId) return
  loading.value = true
  try {
    const rec = await pb.collection('nats_account_exports').getOne<NatsAccountExport>(exportId)
    accountId.value = rec.account_id
    formData.value = {
      name: rec.name,
      subject: rec.subject,
      type: rec.type,
      description: rec.description || '',
      token_req: rec.token_req || false,
      response_type: rec.response_type || '',
      response_threshold: rec.response_threshold?.toString() || '',
      account_token_position: rec.account_token_position?.toString() || '',
      advertise: rec.advertise || false,
      allow_trace: rec.allow_trace || false,
    }
  } catch (err: any) {
    toast.error('Failed to load export')
    router.push('/nats/exports')
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
      type: formData.value.type,
      description: formData.value.description || null,
      token_req: formData.value.token_req,
      advertise: formData.value.advertise,
      allow_trace: formData.value.allow_trace,
      account_token_position: formData.value.account_token_position ? parseInt(formData.value.account_token_position) : null,
    }

    if (formData.value.type === 'service') {
      data.response_type = formData.value.response_type || null
      data.response_threshold = formData.value.response_threshold ? parseInt(formData.value.response_threshold) : null
    } else {
      data.response_type = null
      data.response_threshold = null
    }

    if (isEdit.value) {
      await pb.collection('nats_account_exports').update(exportId!, data)
      toast.success('Export updated')
    } else {
      data.organization = authStore.currentOrgId
      await pb.collection('nats_account_exports').create(data)
      toast.success('Export created')
    }
    router.push('/nats/exports')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save export')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  if (isEdit.value) {
    await loadExport()
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
          <li><router-link to="/nats/exports">NATS Exports</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit Export' : 'Create Export' }}</h1>
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

          <BaseCard title="Options">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.token_req" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Require Activation Token</span>
                    <span class="block text-xs text-base-content/60 mt-1">Importers must provide a token to activate this export.</span>
                  </span>
                </label>
              </div>
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.advertise" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Advertise</span>
                    <span class="block text-xs text-base-content/60 mt-1">Make this export discoverable by other accounts.</span>
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
              <div class="form-control">
                <label class="label"><span class="label-text">Account Token Position</span></label>
                <input v-model="formData.account_token_position" type="number" class="input input-bordered font-mono" min="0" placeholder="0" />
                <label class="label"><span class="label-text-alt">Position of account token in wildcard subject</span></label>
              </div>
            </div>
          </BaseCard>
        </div>

        <div class="space-y-6">
          <BaseCard v-if="formData.type === 'service'" title="Service Response">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Response Type</span></label>
                <select v-model="formData.response_type" class="select select-bordered">
                  <option value="">Not set</option>
                  <option value="Singleton">Singleton</option>
                  <option value="Stream">Stream</option>
                  <option value="Chunked">Chunked</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Response Threshold (ms)</span></label>
                <input v-model="formData.response_threshold" type="number" class="input input-bordered font-mono" min="0" placeholder="5000" />
              </div>
            </div>
          </BaseCard>
        </div>
      </div>

      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button type="button" @click="router.back()" class="btn btn-ghost order-2 sm:order-1" :disabled="loading">Cancel</button>
        <button type="submit" class="btn btn-primary order-1 sm:order-2" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Export</span>
        </button>
      </div>
    </form>
  </div>
</template>
