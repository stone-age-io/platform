<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import type { NatsAccountImport } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const importRecord = ref<NatsAccountImport | null>(null)
const loading = ref(true)
const deleting = ref(false)
const importId = route.params.id as string
const showToken = ref(false)

async function loadImport() {
  loading.value = true
  try {
    importRecord.value = await pb.collection('nats_account_imports').getOne<NatsAccountImport>(importId)
  } catch (err: any) {
    toast.error('Failed to load import')
    router.push('/nats/imports')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!importRecord.value) return
  const confirmed = await confirm({
    title: 'Delete Import',
    message: `Are you sure you want to delete "${importRecord.value.name}"?`,
    details: 'This account will lose access to the imported subject.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('nats_account_imports').delete(importRecord.value.id)
    toast.success('Import deleted')
    router.push('/nats/imports')
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
  }
}

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch {
    toast.error('Failed to copy')
  }
}

onMounted(loadImport)
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <template v-else-if="importRecord">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/imports">NATS Imports</router-link></li>
            <li class="truncate max-w-[200px]">{{ importRecord.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ importRecord.name }}</h1>
            <span
              class="badge"
              :class="importRecord.type === 'stream' ? 'badge-info' : 'badge-success'"
            >
              {{ importRecord.type }}
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/imports/${importRecord.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial" :disabled="deleting">Delete</button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Subject</dt>
                <dd class="mt-1"><code class="text-sm">{{ importRecord.subject }}</code></dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ importRecord.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(importRecord.created) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
                <dd class="mt-1 text-sm">{{ formatDate(importRecord.updated) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Routing">
            <dl class="space-y-3">
              <div>
                <dt class="text-xs text-base-content/70">Local Subject</dt>
                <dd class="mt-1">
                  <code v-if="importRecord.local_subject" class="text-sm">{{ importRecord.local_subject }}</code>
                  <span v-else class="text-sm text-base-content/40">Same as subject</span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Share</dt>
                <dd class="mt-1">
                  <span class="badge badge-sm" :class="importRecord.share ? 'badge-primary' : 'badge-ghost'">
                    {{ importRecord.share ? 'Yes' : 'No' }}
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Allow Trace</dt>
                <dd class="mt-1">
                  <span class="badge badge-sm" :class="importRecord.allow_trace ? 'badge-primary' : 'badge-ghost'">
                    {{ importRecord.allow_trace ? 'Yes' : 'No' }}
                  </span>
                </dd>
              </div>
            </dl>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Source Account">
            <div class="space-y-4">
              <div>
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Account Public Key</div>
                  <button @click="copyToClipboard(importRecord.account, 'Public Key')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all border border-base-300">
                  {{ importRecord.account }}
                </div>
              </div>

              <div>
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Activation Token</div>
                  <div v-if="importRecord.token" class="flex gap-1">
                    <button @click="showToken = !showToken" class="btn btn-ghost btn-xs">
                      {{ showToken ? 'Hide' : 'Show' }}
                    </button>
                    <button @click="copyToClipboard(importRecord.token!, 'Token')" class="btn btn-ghost btn-xs">Copy</button>
                  </div>
                </div>
                <div v-if="importRecord.token && showToken" class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all max-h-32 overflow-y-auto border border-base-300">
                  {{ importRecord.token }}
                </div>
                <div v-else-if="importRecord.token" class="text-sm text-base-content/40 italic">
                  Token set (hidden)
                </div>
                <div v-else class="text-sm text-base-content/40 italic">
                  None
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
