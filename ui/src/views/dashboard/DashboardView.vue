<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { formatRelativeTime } from '@/utils/format'
import type { AuditLog } from '@/types/pocketbase'
import LiveMessageStream from '@/components/nats/LiveMessageStream.vue'

const authStore = useAuthStore()

// Stats state
const stats = ref({
  things: 0,
  edges: 0,
  locations: 0,
  members: 0,
})

// Activity
const recentActivity = ref<AuditLog[]>([])
const loading = ref(true)

/**
 * Load dashboard data
 */
async function loadDashboard() {
  if (!authStore.currentOrgId) return
  
  loading.value = true
  
  try {
    const [
      thingsRes, 
      edgesRes, 
      locsRes, 
      membersRes, 
      activityRes
    ] = await Promise.all([
      pb.collection('things').getList(1, 1),
      pb.collection('edges').getList(1, 1),
      pb.collection('locations').getList(1, 1),
      pb.collection('memberships').getList(1, 1, { filter: `organization = "${authStore.currentOrgId}"` }),
      // Fetch a few more logs since the feed is compact
      pb.collection('audit_logs').getList<AuditLog>(1, 15, { sort: '-created', expand: 'user' }),
    ])
    
    stats.value = {
      things: thingsRes.totalItems,
      edges: edgesRes.totalItems,
      locations: locsRes.totalItems,
      members: membersRes.totalItems,
    }
    
    recentActivity.value = activityRes.items

  } catch (err) {
    console.error('Failed to load dashboard:', err)
  } finally {
    loading.value = false
  }
}

function handleOrgChange() {
  loadDashboard()
}

onMounted(() => {
  loadDashboard()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})

function getActionIcon(eventType: string) {
  if (eventType.includes('create')) return '✨'
  if (eventType.includes('update')) return '✏️'
  if (eventType.includes('delete')) return '🗑️'
  if (eventType.includes('auth')) return '🔐'
  return '📝'
}

function getActionColor(eventType: string) {
  if (eventType.includes('create')) return 'text-success'
  if (eventType.includes('update')) return 'text-warning'
  if (eventType.includes('delete')) return 'text-error'
  if (eventType.includes('auth')) return 'text-info'
  return 'text-base-content'
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Dashboard</h1>
        <p class="text-base-content/70 mt-1">
          {{ authStore.currentOrg?.name }}
        </p>
      </div>
    </div>
    
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <div v-else class="space-y-6">
      
      <!-- 1. Stats Grid (Inventory) -->
      <!-- Restored larger padding and icon sizes, removed border to match original style -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Things -->
        <div class="stats shadow bg-base-100">
          <div class="stat">
            <div class="stat-figure text-primary">
              <span class="text-4xl">📦</span>
            </div>
            <div class="stat-title">Things</div>
            <div class="stat-value text-primary">{{ stats.things }}</div>
            <div class="stat-actions">
              <router-link to="/things/new" class="btn btn-sm btn-primary btn-outline w-full">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Edges -->
        <div class="stats shadow bg-base-100">
          <div class="stat">
            <div class="stat-figure text-secondary">
              <span class="text-4xl">🔌</span>
            </div>
            <div class="stat-title">Edges</div>
            <div class="stat-value text-secondary">{{ stats.edges }}</div>
            <div class="stat-actions">
              <router-link to="/edges/new" class="btn btn-sm btn-secondary btn-outline w-full">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Locations -->
        <div class="stats shadow bg-base-100">
          <div class="stat">
            <div class="stat-figure text-accent">
              <span class="text-4xl">📍</span>
            </div>
            <div class="stat-title">Locations</div>
            <div class="stat-value text-accent">{{ stats.locations }}</div>
            <div class="stat-actions">
              <router-link to="/locations/new" class="btn btn-sm btn-accent btn-outline w-full">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Members -->
        <div class="stats shadow bg-base-100">
          <div class="stat">
            <div class="stat-figure">
              <span class="text-4xl">👥</span>
            </div>
            <div class="stat-title">Team</div>
            <div class="stat-value">{{ stats.members }}</div>
            <div class="stat-actions">
              <router-link to="/organization/invitations" class="btn btn-sm btn-ghost btn-outline w-full">Invite</router-link>
            </div>
          </div>
        </div>
      </div>

      <!-- 2. Operational Views -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        <!-- Left: Live Stream -->
        <div class="lg:col-span-2">
          <LiveMessageStream />
        </div>

        <!-- Right: Activity Feed -->
        <div class="lg:col-span-1">
          <!-- Explicit Card implementation to fix layout gap -->
          <div class="card bg-base-100 shadow-xl h-[500px] flex flex-col">
            
            <!-- Header -->
            <div class="card-body flex-none pb-2">
              <h2 class="card-title">Recent Activity</h2>
            </div>

            <!-- Content -->
            <div v-if="recentActivity.length === 0" class="flex-1 flex items-center justify-center text-base-content/50">
              No recent activity
            </div>
            
            <div v-else class="flex-1 overflow-y-auto px-6 py-2 space-y-4">
              <!-- Timeline Item -->
              <div v-for="log in recentActivity" :key="log.id" class="flex gap-3 relative group">
                
                <!-- Connector Line -->
                <div class="absolute left-[15px] top-8 bottom-[-16px] w-px bg-base-300 last:hidden"></div>

                <!-- Avatar/Icon -->
                <div class="flex-none z-10">
                  <div class="w-8 h-8 rounded-full bg-base-200 border border-base-300 flex items-center justify-center text-sm shadow-sm" :title="log.event_type">
                    {{ getActionIcon(log.event_type) }}
                  </div>
                </div>

                <!-- Content -->
                <div class="flex-1 min-w-0 pb-1">
                  <div class="flex justify-between items-start">
                    <p class="text-xs font-bold truncate">
                      {{ log.expand?.user?.name || log.expand?.user?.email || 'System' }}
                    </p>
                    <time class="text-[10px] opacity-50 whitespace-nowrap">{{ formatRelativeTime(log.created) }}</time>
                  </div>
                  
                  <p class="text-xs mt-0.5">
                    <span :class="getActionColor(log.event_type)" class="font-medium">
                      {{ log.event_type.replace('_request', '').toUpperCase() }}
                    </span>
                    <span class="opacity-70 mx-1">in</span>
                    <span class="font-mono bg-base-200 px-1 rounded text-[10px]">{{ log.collection_name }}</span>
                  </p>
                  
                  <!-- ID/Target (Optional) -->
                  <p v-if="log.record_id" class="text-[10px] font-mono opacity-40 truncate mt-1 group-hover:opacity-100 transition-opacity">
                    #{{ log.record_id }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Footer -->
            <div class="p-3 border-t border-base-200 bg-base-100/50 text-center rounded-b-xl flex-none">
              <router-link to="/audit" class="btn btn-xs btn-ghost w-full">View Full Audit Log</router-link>
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>
