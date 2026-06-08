<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import type { LeafNode } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const nodeId = route.params.id as string
const node = ref<LeafNode | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

async function loadNode() {
  loading.value = true
  error.value = null
  try {
    node.value = await pb.collection('leaf_nodes').getOne<LeafNode>(nodeId, {
      expand: 'location,nats_user',
    })
  } catch (err: any) {
    error.value = err.message || 'Failed to load leaf node'
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!node.value) return
  const confirmed = await confirm({
    title: 'Delete Leaf Node',
    message: `Are you sure you want to delete "${node.value.name || node.value.code}"?`,
    details: 'This removes the edge node record. Its NATS user is not deleted automatically.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await pb.collection('leaf_nodes').delete(nodeId)
    toast.success('Leaf node deleted')
    router.push('/leaf-nodes')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete leaf node')
  }
}

onMounted(loadNode)
</script>

<template>
  <div class="space-y-6">
    <div class="breadcrumbs text-sm">
      <ul>
        <li><router-link to="/leaf-nodes">Leaf Nodes</router-link></li>
        <li>{{ node?.name || node?.code || 'Detail' }}</li>
      </ul>
    </div>

    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load leaf node</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadNode" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <template v-else-if="node">
      <!-- Header -->
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 class="text-3xl font-bold">{{ node.name || 'Unnamed' }}</h1>
          <p v-if="node.description" class="text-base-content/70 mt-1">{{ node.description }}</p>
        </div>
        <div class="flex gap-2">
          <router-link :to="`/leaf-nodes/${node.id}/edit`" class="btn btn-primary">Edit</router-link>
          <button @click="handleDelete" class="btn btn-outline btn-error">Delete</button>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <BaseCard title="Identity">
          <dl class="space-y-3">
            <div>
              <dt class="text-xs uppercase text-base-content/50">Code</dt>
              <dd><code class="text-sm">{{ node.code || '-' }}</code></dd>
            </div>
            <div>
              <dt class="text-xs uppercase text-base-content/50">JetStream Domain</dt>
              <dd><code class="text-sm">{{ node.domain || '-' }}</code></dd>
            </div>
            <div>
              <dt class="text-xs uppercase text-base-content/50">Location</dt>
              <dd>{{ node.expand?.location?.name || '-' }}</dd>
            </div>
            <div>
              <dt class="text-xs uppercase text-base-content/50">NATS User</dt>
              <dd>
                <router-link
                  v-if="node.nats_user"
                  :to="`/nats/users/${node.nats_user}`"
                  class="link link-primary text-sm font-mono"
                >
                  {{ node.expand?.nats_user?.nats_username || node.nats_user }}
                </router-link>
                <span v-else class="text-warning text-sm">Not provisioned yet</span>
              </dd>
            </div>
          </dl>
        </BaseCard>

        <BaseCard title="Synced Collections">
          <div v-if="node.synced_collections?.length" class="flex flex-wrap gap-2">
            <span v-for="col in node.synced_collections" :key="col" class="badge badge-ghost">
              <code>{{ col }}</code>
            </span>
          </div>
          <p v-else class="text-base-content/60 text-sm">No collections selected.</p>

          <div class="divider"></div>
          <div class="text-sm text-base-content/70 space-y-1">
            <p class="font-semibold">Bootstrap on the edge:</p>
            <pre class="bg-base-200 rounded p-2 text-xs overflow-x-auto">leaf-sync config   # writes nats-leaf.conf + creds
leaf-sync run      # mirror config → local KV</pre>
          </div>
        </BaseCard>
      </div>

      <BaseCard title="Metadata" v-if="node.metadata && Object.keys(node.metadata).length">
        <pre class="bg-base-200 rounded p-3 text-xs overflow-x-auto"><code>{{ JSON.stringify(node.metadata, null, 2) }}</code></pre>
      </BaseCard>

      <p class="text-xs text-base-content/50">
        Created {{ formatDate(node.created, 'PPpp') }} · Updated {{ formatDate(node.updated, 'PPpp') }}
      </p>
    </template>
  </div>
</template>
