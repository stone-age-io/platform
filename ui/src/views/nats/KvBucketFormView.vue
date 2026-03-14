<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager } from '@/composables/useJetStreamManager'
import { useToast } from '@/composables/useToast'
import type { KvBucketFormData } from '@/types/jetstream'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const natsStore = useNatsStore()
const toast = useToast()
const { createKvBucket } = useJetStreamManager()

const loading = ref(false)

const formData = ref<KvBucketFormData>({
  name: '',
  description: '',
  storage: 'file',
  history: '1',
  maxBytes: '-1',
  maxValueSize: '-1',
  ttl: '0',
  replicas: '1',
})

async function handleSubmit() {
  if (!formData.value.name.trim()) {
    toast.error('Bucket name is required')
    return
  }

  loading.value = true
  try {
    await createKvBucket(formData.value)
    router.push('/nats/kv')
  } catch {
    // Error already toasted by composable
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/kv">KV Buckets</router-link></li>
          <li>New</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">Create KV Bucket</h1>
    </div>

    <!-- Not Connected -->
    <BaseCard v-if="!natsStore.isConnected">
      <div class="text-center py-12">
        <span class="text-6xl">📡</span>
        <h3 class="text-xl font-bold mt-4">Not connected to NATS</h3>
      </div>
    </BaseCard>

    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text font-bold">Bucket Name *</span></label>
                <input
                  v-model="formData.name"
                  type="text"
                  class="input input-bordered font-mono"
                  required
                  placeholder="my-bucket"
                />
                <label class="label"><span class="label-text-alt">Alphanumeric, hyphens, and underscores only</span></label>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Description</span></label>
                <textarea v-model="formData.description" class="textarea textarea-bordered" rows="2"></textarea>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Storage">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Storage Type</span></label>
                <select v-model="formData.storage" class="select select-bordered">
                  <option value="file">File (persistent)</option>
                  <option value="memory">Memory (fast, not persistent)</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Replicas</span></label>
                <input v-model="formData.replicas" type="number" min="1" max="5" class="input input-bordered font-mono" />
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Configuration">
            <div class="grid grid-cols-1 gap-4">
              <div class="form-control">
                <label class="label"><span class="label-text">History (revisions per key)</span></label>
                <input v-model="formData.history" type="number" min="1" max="64" class="input input-bordered font-mono" />
                <label class="label"><span class="label-text-alt">1 - 64 revisions kept per key</span></label>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Bytes</span></label>
                <input v-model="formData.maxBytes" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Value Size</span></label>
                <input v-model="formData.maxValueSize" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">TTL (Time to Live)</span></label>
                <input v-model="formData.ttl" type="text" class="input input-bordered font-mono" placeholder="0 (no expiry)" />
                <label class="label"><span class="label-text-alt">e.g. 30s, 5m, 24h, 7d. 0 = no expiry</span></label>
              </div>
              <p class="text-[10px] opacity-50 italic px-1">-1 = Unlimited, 0 = Default/None</p>
            </div>
          </BaseCard>
        </div>
      </div>

      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button type="button" @click="router.back()" class="btn btn-ghost order-2 sm:order-1" :disabled="loading">Cancel</button>
        <button type="submit" class="btn btn-primary order-1 sm:order-2" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>Create Bucket</span>
        </button>
      </div>
    </form>
  </div>
</template>
