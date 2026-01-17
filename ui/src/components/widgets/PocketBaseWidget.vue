<!-- ui/src/components/widgets/PocketBaseWidget.vue -->
<template>
  <div class="pb-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Loading -->
    <div v-if="loading && items.length === 0" class="loading-overlay">
      <span class="loading loading-spinner"></span>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="error-state">
      <span class="text-error">⚠️ {{ error }}</span>
      <button class="btn btn-xs btn-ghost mt-2" @click="fetchData">Retry</button>
    </div>

    <!-- Data -->
    <div v-else class="data-container">
      <ResponsiveList 
        :items="items" 
        :columns="columns" 
        :clickable="false"
      >
        <!-- Generic Cell Renderer -->
        <template v-for="col in columns" :key="col.key" #[`cell-${col.key}`]="{ value }">
          <span class="text-xs truncate block max-w-[200px]" :title="String(value)">
            {{ formatValue(value) }}
          </span>
        </template>

        <!-- Generic Card Renderer -->
        <template v-for="col in columns" :key="col.key" #[`card-${col.key}`]="{ value }">
          <div class="flex flex-col">
            <span class="text-[10px] opacity-50 uppercase font-bold">{{ col.label }}</span>
            <span class="text-sm truncate">{{ formatValue(value) }}</span>
          </div>
        </template>

        <template #empty>
          <div class="text-center py-8 opacity-50 text-xs italic">
            No records found
          </div>
        </template>
      </ResponsiveList>
    </div>
    
    <!-- Footer Info -->
    <div v-if="layoutMode !== 'card'" class="widget-footer">
      <span class="text-[10px] opacity-50">
        {{ items.length }} record{{ items.length !== 1 ? 's' : '' }}
        <span v-if="lastUpdated">• Updated {{ lastUpdated }}</span>
      </span>
      <button @click="fetchData" class="btn btn-ghost btn-xs" title="Refresh">
        🔄
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { pb } from '@/utils/pb'
import { useDashboardStore } from '@/stores/dashboard'
import { resolveTemplate } from '@/utils/variables'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dashboardStore = useDashboardStore()

const items = ref<any[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const lastUpdated = ref('')
let refreshTimer: number | null = null

const cfg = computed(() => props.config.pocketbaseConfig || {
  collection: '',
  limit: 10
})

// Dynamic Columns based on first item or fields config
const columns = computed<Column<any>[]>(() => {
  if (items.value.length === 0) return []
  
  // If fields are specified, use them
  if (cfg.value.fields && cfg.value.fields.trim() !== '*' && cfg.value.fields.trim() !== '') {
    return cfg.value.fields.split(',').map(f => {
      const key = f.trim()
      return { key, label: key, mobileLabel: key }
    })
  }

  // Otherwise guess from first item (exclude system fields if too many)
  const keys = Object.keys(items.value[0]).filter(k => 
    !['collectionId', 'collectionName', 'expand'].includes(k)
  )
  
  // Limit to first 4 columns for sanity
  return keys.slice(0, 4).map(k => ({
    key: k,
    label: k,
    mobileLabel: k
  }))
})

async function fetchData() {
  if (!cfg.value.collection) return

  loading.value = true
  error.value = null
  
  try {
    const filter = resolveTemplate(cfg.value.filter, dashboardStore.currentVariableValues)
    
    // Grug say: Construct options carefully. Don't send undefined.
    const options: any = {
      sort: cfg.value.sort || '-created',
      requestKey: null // Disable auto-cancellation for widget
    }

    // Only add filter if it exists and is not empty
    if (filter && filter.trim() !== '') {
      options.filter = filter
    }

    // Only add fields if specified
    if (cfg.value.fields && cfg.value.fields.trim() !== '') {
      options.fields = cfg.value.fields
    }

    const result = await pb.collection(cfg.value.collection).getList(1, cfg.value.limit || 10, options)
    
    items.value = result.items
    lastUpdated.value = new Date().toLocaleTimeString()
  } catch (err: any) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function formatValue(val: any): string {
  if (val === null || val === undefined) return '-'
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

function setupTimer() {
  if (refreshTimer) clearInterval(refreshTimer)
  if (cfg.value.refreshInterval && cfg.value.refreshInterval > 0) {
    refreshTimer = window.setInterval(fetchData, cfg.value.refreshInterval * 1000)
  }
}

onMounted(() => {
  fetchData()
  setupTimer()
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})

// Watch for config changes
watch(() => props.config.pocketbaseConfig, () => {
  fetchData()
  setupTimer()
}, { deep: true })

// Watch for variable changes (re-fetch if filter uses variables)
watch(() => dashboardStore.currentVariableValues, () => {
  if (cfg.value.filter?.includes('{{')) {
    fetchData()
  }
}, { deep: true })
</script>

<style scoped>
.pb-widget {
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
}

.pb-widget.card-layout {
  padding: 8px;
}

.loading-overlay {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.error-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 16px;
}

.data-container {
  flex: 1;
  overflow: auto;
}

.widget-footer {
  flex-shrink: 0;
  padding: 4px 8px;
  border-top: 1px solid oklch(var(--b3));
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: oklch(var(--b2) / 0.5);
}
</style>
