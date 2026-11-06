<script setup lang="ts">
import { Boxes, RadioTower, RefreshCw, ShieldOff, TriangleAlert } from '@lucide/vue'
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useNow } from '@vueuse/core'
import { useNatsStore } from '@/stores/nats'
import { useEscapeKey } from '@/composables/useEscapeKey'
import { discoverService, groupServices, metadataKeys, shortVersion, DiscoveryDeniedError } from '@/utils/natsServices'
import { formatAge, formatDate } from '@/utils/format'
import { sortBy } from '@/utils/clientSort'
import type { ServiceEndpoint, ServiceInstance } from '@/types/services'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import SubjectChip from '@/components/common/SubjectChip.vue'

// Keyed on the service name, which is stable. An instance id is random per
// start, so an instance gets a modal rather than a URL.

const route = useRoute()
const natsStore = useNatsStore()
const name = computed(() => String(route.params.name))

const instances = ref<ServiceInstance[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const denied = ref(false)
const askedAt = ref<number | null>(null)
const sort = ref('')
const selected = ref<ServiceInstance | null>(null)

useEscapeKey(() => !!selected.value, () => { selected.value = null })

const now = useNow({ interval: 1000 })
const askedAge = computed(() => askedAt.value === null ? '' : formatAge(now.value.getTime() - askedAt.value))

const group = computed(() => groupServices(instances.value)[0] ?? null)
const keys = computed(() => metadataKeys(instances.value))
const mixedVersions = computed(() => (group.value?.versions.length ?? 0) > 1)

// Instances of one service share a metadata shape, so each key becomes a
// column. Flattened under positional names: a key may contain a dot, which
// ResponsiveList would read as a path.
type Row = ServiceInstance & Record<string, unknown>
const rows = computed<Row[]>(() => instances.value.map(i => {
  const row: Row = { ...i }
  keys.value.forEach((k, n) => { row[`meta${n}`] = i.metadata[k] ?? '' })
  return row
}))
const sortedRows = computed(() => sortBy(rows.value, sort.value || (keys.value.length ? 'meta0' : 'id')))

// Ascending `started` is longest-running first.
const columns = computed<Column<Row>[]>(() => [
  ...(keys.value.length
    ? keys.value.map((k, n) => ({ key: `meta${n}`, sortable: `meta${n}`, label: k, mobileLabel: k }))
    : [{ key: 'id', sortable: 'id', label: 'Instance', mobileLabel: 'Instance' }]),
  { key: 'version', sortable: 'version', width: '11rem', label: 'Version', mobileLabel: 'Version' },
  { key: 'started', sortable: 'started', width: '6rem', label: 'Up', mobileLabel: 'Up' },
  { key: 'requests', sortable: '-requests', width: '7rem', label: 'Requests', mobileLabel: 'Reqs', format: (v: number | null) => v === null ? '—' : v.toLocaleString() },
  { key: 'errors', sortable: '-errors', width: '6rem', label: 'Errors', mobileLabel: 'Errors' },
])

function uptime(started: string | null): string {
  if (!started || askedAt.value === null) return '—'
  return formatAge(askedAt.value - new Date(started).getTime()).replace(/ ago$/, '')
}

const num = (v: number | null) => v === null ? '—' : v.toLocaleString()

// An endpoint nobody has called has no average, not one of 0 ms.
function average(ep: ServiceEndpoint): string {
  if (ep.averageMs === null || !ep.requests) return '—'
  if (ep.averageMs < 0.01) return '<0.01 ms'
  return `${ep.averageMs.toFixed(ep.averageMs < 10 ? 2 : 0)} ms`
}

async function loadData() {
  if (!natsStore.isConnected || !natsStore.nc) {
    loading.value = false
    return
  }
  loading.value = true
  error.value = null
  denied.value = false
  try {
    instances.value = await discoverService(natsStore.nc, name.value)
    askedAt.value = Date.now()
    // Keep an open modal on the fresh numbers, or close it if that instance
    // stopped answering.
    if (selected.value) selected.value = instances.value.find(i => i.id === selected.value!.id) ?? null
  } catch (e: any) {
    if (e instanceof DiscoveryDeniedError) denied.value = true
    else error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(() => { loadData() })

watch(() => natsStore.isConnected, (connected) => {
  if (connected) loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col gap-4">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/services">Services</router-link></li>
          <li class="truncate max-w-[200px] font-mono">{{ name }}</li>
        </ul>
      </div>
      <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
        <div class="min-w-0">
          <h1 class="text-3xl font-bold font-mono break-words">{{ name }}</h1>
          <p v-if="group?.description" class="text-base-content/70 mt-1">{{ group.description }}</p>
        </div>
        <button v-if="natsStore.isConnected" class="btn w-full sm:w-auto" :disabled="loading" @click="loadData">
          <RefreshCw :size="16" :class="{ 'animate-spin': loading }" />
          <span>Refresh</span>
        </button>
      </div>
    </div>

    <!-- Not Connected -->
    <BaseCard v-if="!natsStore.isConnected">
      <div class="text-center py-12">
        <RadioTower :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">Not connected to NATS</h3>
        <p class="text-base-content/70 mt-2">Connect to NATS to discover services.</p>
      </div>
    </BaseCard>

    <template v-else>
      <div v-if="loading && instances.length === 0" class="flex justify-center p-12">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <BaseCard v-else-if="denied">
        <div class="text-center py-12">
          <ShieldOff :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Your NATS user cannot ask</h3>
          <p class="text-base-content/70 mt-2 max-w-lg mx-auto">
            Give the role on your NATS user publish on <code class="font-mono">$SRV.&gt;</code>.
          </p>
        </div>
      </BaseCard>

      <BaseCard v-else-if="error && instances.length === 0">
        <div class="text-center py-12">
          <TriangleAlert :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">Discovery failed</h3>
          <p class="text-base-content/70 mt-2">{{ error }}</p>
          <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
        </div>
      </BaseCard>

      <BaseCard v-else-if="instances.length === 0">
        <div class="text-center py-12">
          <Boxes :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
          <h3 class="text-xl font-bold mt-4">No instance answered</h3>
          <p class="text-base-content/70 mt-2 max-w-lg mx-auto">
            Nothing named <code class="font-mono">{{ name }}</code> is answering in this account right now.
            Behind a leaf node, the site's uplink credential needs <code class="font-mono">$SRV.&gt;</code>,
            so silence is not proof that it is down.
          </p>
          <router-link to="/nats/services" class="btn btn-ghost mt-4">All services</router-link>
        </div>
      </BaseCard>

      <template v-else-if="group">
        <!-- Summary -->
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="bg-base-200 rounded-lg p-3 border border-base-300">
            <span class="text-xs text-base-content/50 uppercase block mb-1">Instances</span>
            <span class="font-medium text-sm">{{ instances.length }}</span>
          </div>
          <div class="bg-base-200 rounded-lg p-3 border border-base-300 min-w-0">
            <span class="text-xs text-base-content/50 uppercase block mb-1">{{ mixedVersions ? 'Versions' : 'Version' }}</span>
            <span v-if="mixedVersions" class="badge badge-sm badge-warning" :title="group.versions.join('\n')">{{ group.versions.length }} running</span>
            <span v-else class="font-mono text-sm block truncate" :title="group.versions[0]">{{ shortVersion(group.versions[0]) || '—' }}</span>
          </div>
          <div class="bg-base-200 rounded-lg p-3 border border-base-300">
            <span class="text-xs text-base-content/50 uppercase block mb-1">Requests</span>
            <span class="font-medium text-sm">{{ num(group.requests) }}</span>
          </div>
          <div class="bg-base-200 rounded-lg p-3 border border-base-300">
            <span class="text-xs text-base-content/50 uppercase block mb-1">Errors</span>
            <span class="font-medium text-sm" :class="{ 'text-error': group.errors }">{{ num(group.errors) }}</span>
          </div>
        </div>

        <p class="text-sm text-base-content/60 -mb-2">Asked {{ askedAge }}. Select an instance for its endpoints.</p>

        <!-- Instances -->
        <BaseCard :no-padding="true">
          <ResponsiveList
            :items="sortedRows"
            :columns="columns"
            :loading="loading"
            :sort="sort"
            @update:sort="sort = $event"
            @row-click="selected = $event"
          >
            <template #cell-id="{ item }"><span class="font-mono text-xs">{{ item.id }}</span></template>

            <!-- Off the most-run version: the instance a rollout missed. -->
            <template #cell-version="{ item }">
              <span
                class="font-mono text-xs whitespace-nowrap"
                :class="{ 'badge badge-sm badge-warning badge-outline': mixedVersions && item.version !== group.versions[0] }"
                :title="item.version"
              >{{ shortVersion(item.version) || '—' }}</span>
            </template>
            <template #card-version="{ item }">
              <span class="font-mono text-xs" :class="{ 'text-warning': mixedVersions && item.version !== group.versions[0] }">{{ shortVersion(item.version) || '—' }}</span>
            </template>

            <template #cell-started="{ item }">{{ uptime(item.started) }}</template>
            <template #card-started="{ item }">{{ uptime(item.started) }}</template>

            <template #cell-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }" :title="item.lastError || undefined">{{ num(item.errors) }}</span>
            </template>
            <template #card-errors="{ item }">
              <span :class="{ 'text-error font-medium': item.errors }">{{ num(item.errors) }}</span>
            </template>
          </ResponsiveList>
        </BaseCard>
      </template>
    </template>

    <!-- Instance Modal -->
    <dialog class="modal" :class="{ 'modal-open': !!selected }">
      <div class="modal-box w-11/12 max-w-3xl" v-if="selected">
        <div class="flex justify-between items-start mb-4">
          <div class="min-w-0">
            <h3 class="font-bold text-lg font-mono">{{ selected.name }}</h3>
            <p class="text-xs text-base-content/50 font-mono break-all">{{ selected.id }}</p>
          </div>
          <button class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="selected = null">✕</button>
        </div>

        <dl class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
          <div>
            <dt class="text-base-content/60">Version</dt>
            <dd class="font-mono break-all">{{ selected.version || '—' }}</dd>
          </div>
          <div>
            <dt class="text-base-content/60">Started</dt>
            <dd>{{ selected.started ? `${formatDate(selected.started, 'PPpp')} (${uptime(selected.started)})` : '—' }}</dd>
          </div>
          <div v-for="(v, k) in selected.metadata" :key="k">
            <dt class="text-base-content/60 font-mono">{{ k }}</dt>
            <dd class="font-mono break-all">{{ v }}</dd>
          </div>
        </dl>

        <h4 class="font-semibold mt-6 mb-2">Endpoints</h4>
        <p v-if="selected.endpoints.length === 0" class="text-sm text-base-content/60">This instance lists no endpoints.</p>
        <div v-else class="space-y-3">
          <div v-for="ep in selected.endpoints" :key="ep.name" class="bg-base-200 rounded-lg p-3 border border-base-300">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium font-mono text-sm">{{ ep.name }}</span>
              <SubjectChip :subject="ep.subject" />
              <span v-if="ep.queueGroup" class="text-xs text-base-content/50">queue {{ ep.queueGroup }}</span>
            </div>
            <div class="flex flex-wrap gap-x-6 gap-y-1 mt-2 text-sm">
              <span><span class="text-base-content/60">Requests</span> {{ num(ep.requests) }}</span>
              <span :class="{ 'text-error': ep.errors }"><span class="text-base-content/60">Errors</span> {{ num(ep.errors) }}</span>
              <span><span class="text-base-content/60">Average</span> {{ average(ep) }}</span>
            </div>
            <p v-if="ep.lastError" class="text-xs text-error font-mono mt-2 break-all">Last error: {{ ep.lastError }}</p>
          </div>
        </div>

        <div class="modal-action">
          <button class="btn" @click="selected = null">Close</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="selected = null"><button>close</button></form>
    </dialog>
  </div>
</template>
