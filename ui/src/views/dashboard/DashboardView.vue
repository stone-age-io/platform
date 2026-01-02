<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { formatRelativeTime } from '@/utils/format'
import type { AuditLog } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
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
      pb.collection('audit_logs').getList<AuditLog>(1, 5, { sort: '-created', expand: 'user' }),
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

function getEventBadgeClass(eventType: string) {
  if (eventType.includes('create')) return 'badge-success'
  if (eventType.includes('update')) return 'badge-warning'
  if (eventType.includes('delete')) return 'badge-error'
  return 'badge-ghost'
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
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Things -->
        <div class="stats shadow border border-base-200">
          <div class="stat">
            <div class="stat-figure text-primary">
              <span class="text-3xl">📦</span>
            </div>
            <div class="stat-title">Things</div>
            <div class="stat-value text-primary">{{ stats.things }}</div>
            <div class="stat-actions">
              <router-link to="/things/new" class="btn btn-xs btn-primary btn-outline">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Edges -->
        <div class="stats shadow border border-base-200">
          <div class="stat">
            <div class="stat-figure text-secondary">
              <span class="text-3xl">🔌</span>
            </div>
            <div class="stat-title">Edges</div>
            <div class="stat-value text-secondary">{{ stats.edges }}</div>
            <div class="stat-actions">
              <router-link to="/edges/new" class="btn btn-xs btn-secondary btn-outline">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Locations -->
        <div class="stats shadow border border-base-200">
          <div class="stat">
            <div class="stat-figure text-accent">
              <span class="text-3xl">📍</span>
            </div>
            <div class="stat-title">Locations</div>
            <div class="stat-value text-accent">{{ stats.locations }}</div>
            <div class="stat-actions">
              <router-link to="/locations/new" class="btn btn-xs btn-accent btn-outline">+ Add</router-link>
            </div>
          </div>
        </div>
        
        <!-- Members -->
        <div class="stats shadow border border-base-200">
          <div class="stat">
            <div class="stat-figure">
              <span class="text-3xl">👥</span>
            </div>
            <div class="stat-title">Team</div>
            <div class="stat-value">{{ stats.members }}</div>
            <div class="stat-actions">
              <router-link to="/organization/invitations" class="btn btn-xs btn-ghost btn-outline">Invite</router-link>
            </div>
          </div>
        </div>
      </div>

      <!-- 2. Operational Views -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        <!-- Left: Live Stream (Takes up 2/3) -->
        <div class="lg:col-span-2">
          <LiveMessageStream />
        </div>

        <!-- Right: Recent Activity (Takes up 1/3) -->
        <div class="lg:col-span-1">
          <BaseCard title="Recent Activity" :no-padding="true" class="h-[500px]">
            <div v-if="recentActivity.length === 0" class="text-center py-8 opacity-50">No recent activity</div>
            <div v-else class="overflow-y-auto h-full pb-16"> <!-- Padding bottom for footer -->
              <table class="table table-sm">
                <thead>
                  <tr>
                    <th>User</th>
                    <th>Action</th>
                    <th class="text-right">Time</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="log in recentActivity" :key="log.id">
                    <td>
                      <div class="font-medium text-xs">
                        {{ log.expand?.user?.name || log.expand?.user?.email || 'System' }}
                      </div>
                      <div class="text-[10px] opacity-60">{{ log.collection_name }}</div>
                    </td>
                    <td>
                      <span class="badge badge-xs" :class="getEventBadgeClass(log.event_type)">
                        {{ log.event_type.replace('_request', '') }}
                      </span>
                    </td>
                    <td class="text-right text-[10px] opacity-50 whitespace-nowrap">
                      {{ formatRelativeTime(log.created) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="absolute bottom-0 left-0 right-0 p-3 text-center border-t border-base-200 bg-base-100 rounded-b-xl">
              <router-link to="/audit" class="btn btn-xs btn-ghost w-full">View All Logs</router-link>
            </div>
          </BaseCard>
        </div>

      </div>
    </div>
  </div>
</template>
