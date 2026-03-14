<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useNatsStore } from '@/stores/nats'
import { useJetStreamManager, nanosToFormValue } from '@/composables/useJetStreamManager'
import { useToast } from '@/composables/useToast'
import type { StreamFormData } from '@/types/jetstream'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const natsStore = useNatsStore()
const toast = useToast()
const { createStream, updateStream, getStreamInfo } = useJetStreamManager()

const streamName = route.params.name as string | undefined
const isEdit = computed(() => !!streamName)
const loading = ref(false)

const formData = ref<StreamFormData>({
  name: '',
  description: '',
  subjects: '',
  retention: 'limits',
  storage: 'file',
  maxMsgs: '-1',
  maxBytes: '-1',
  maxMsgSize: '-1',
  maxAge: '0',
  maxConsumers: '-1',
  maxMsgsPerSubject: '-1',
  numReplicas: '1',
  discard: 'old',
  duplicateWindow: '2m',
  allowDirect: true,
})

async function loadStream() {
  if (!streamName) return
  loading.value = true
  try {
    const info = await getStreamInfo(streamName)
    const c = info.config
    formData.value = {
      name: c.name,
      description: c.description || '',
      subjects: (c.subjects || []).join(', '),
      retention: c.retention as any,
      storage: c.storage as any,
      maxMsgs: String(c.max_msgs),
      maxBytes: String(c.max_bytes),
      maxMsgSize: String(c.max_msg_size),
      maxAge: nanosToFormValue(c.max_age),
      maxConsumers: String(c.max_consumers),
      maxMsgsPerSubject: String(c.max_msgs_per_subject),
      numReplicas: String(c.num_replicas),
      discard: c.discard as any,
      duplicateWindow: nanosToFormValue(c.duplicate_window),
      allowDirect: c.allow_direct,
    }
  } catch {
    router.push('/nats/streams')
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  if (!formData.value.name.trim()) {
    toast.error('Stream name is required')
    return
  }
  if (!formData.value.subjects.trim()) {
    toast.error('At least one subject is required')
    return
  }

  loading.value = true
  try {
    if (isEdit.value) {
      await updateStream(streamName!, formData.value)
    } else {
      await createStream(formData.value)
    }
    router.push('/nats/streams')
  } catch {
    // Error already toasted by composable
  } finally {
    loading.value = false
  }
}

onMounted(() => { if (isEdit.value) loadStream() })
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/streams">Streams</router-link></li>
          <li>{{ isEdit ? streamName : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit Stream' : 'Create Stream' }}</h1>
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
                <label class="label"><span class="label-text font-bold">Name *</span></label>
                <input
                  v-model="formData.name"
                  type="text"
                  class="input input-bordered font-mono"
                  :disabled="isEdit"
                  required
                  placeholder="my-stream"
                />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Description</span></label>
                <textarea v-model="formData.description" class="textarea textarea-bordered" rows="2"></textarea>
              </div>
              <div class="form-control">
                <label class="label">
                  <span class="label-text font-bold">Subjects *</span>
                </label>
                <textarea
                  v-model="formData.subjects"
                  class="textarea textarea-bordered font-mono text-xs"
                  rows="3"
                  placeholder="sensors.>, logs.*, events.created"
                ></textarea>
                <label class="label"><span class="label-text-alt">Comma or newline separated. Supports wildcards: *, ></span></label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Storage">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Retention Policy</span></label>
                <select v-model="formData.retention" class="select select-bordered">
                  <option value="limits">Limits (default)</option>
                  <option value="interest">Interest</option>
                  <option value="workqueue">Work Queue</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Storage Type</span></label>
                <select v-model="formData.storage" class="select select-bordered">
                  <option value="file">File (persistent)</option>
                  <option value="memory">Memory (fast, not persistent)</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Discard Policy</span></label>
                <select v-model="formData.discard" class="select select-bordered">
                  <option value="old">Old (drop oldest)</option>
                  <option value="new">New (reject new)</option>
                </select>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Replicas</span></label>
                <input v-model="formData.numReplicas" type="number" min="1" max="5" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input v-model="formData.allowDirect" type="checkbox" class="checkbox checkbox-primary" />
                  <span class="label-text">
                    <span class="font-medium">Allow Direct Get</span>
                    <span class="block text-xs text-base-content/60 mt-1">Enable direct message access for faster reads</span>
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Limits">
            <div class="grid grid-cols-1 gap-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Max Messages</span></label>
                <input v-model="formData.maxMsgs" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Bytes</span></label>
                <input v-model="formData.maxBytes" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Message Size</span></label>
                <input v-model="formData.maxMsgSize" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Messages Per Subject</span></label>
                <input v-model="formData.maxMsgsPerSubject" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Consumers</span></label>
                <input v-model="formData.maxConsumers" type="number" class="input input-bordered font-mono" />
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Max Age</span></label>
                <input v-model="formData.maxAge" type="text" class="input input-bordered font-mono" placeholder="0 (unlimited)" />
                <label class="label"><span class="label-text-alt">e.g. 30s, 5m, 24h, 7d</span></label>
              </div>
              <div class="form-control">
                <label class="label"><span class="label-text">Duplicate Window</span></label>
                <input v-model="formData.duplicateWindow" type="text" class="input input-bordered font-mono" placeholder="2m" />
                <label class="label"><span class="label-text-alt">e.g. 2m, 5m</span></label>
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
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Stream</span>
        </button>
      </div>
    </form>
  </div>
</template>
