<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { formatRelativeTime } from '@/utils/format'
import type { AuditLog } from '@/types/pocketbase'

const authStore = useAuthStore()

// Stats state
const stats = ref({
  things: 0,
  edges: 0,
  locations: 0,
  members: 0,
})

// Recent activity
const recentActivity = ref<AuditLog[]>([])
const loading = ref(true)

/**
 * Load dashboard data
 * 
 * Note: No organization filtering needed!
 * The backend API rules automatically filter all data
 * based on the user's current_organization.
 */
async function loadDashboard() {
  if (!authStore.currentOrgId) return
  
  loading.value = true
  
  try {
    // Backend handles organization filtering via API rules
    // We just fetch the data - no manual filtering needed
    const [thingsResult, edgesResult, locationsResult, membersResult, activityResult] = await Promise.all([
      pb.collection('things').getList(1, 1),
      pb.collection('edges').getList(1, 1),
      pb.collection('locations').getList(1, 1),
      pb.collection('memberships').getList(1, 1),
      pb.collection('audit_logs').getList<AuditLog>(1, 5, {
        sort: '-created',
        expand: 'user',
      }),
    ])
    
    stats.value = {
      things: thingsResult.totalItems,
      edges: edgesResult.totalItems,
      locations: locationsResult.totalItems,
      members: membersResult.totalItems,
    }
    
    recentActivity.value = activityResult.items
  } catch (err) {
    console.error('Failed to load dashboard:', err)
  } finally {
    loading.value = false
  }
}

/**
 * Handle organization change event
 */
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

/**
 * Get badge class for event type
 */
function getEventBadgeClass(eventType: string) {
  switch (eventType) {
    case 'create':
    case 'create_request':
      return 'badge-success'
    case 'update':
    case 'update_request':
      return 'badge-warning'
    case 'delete':
    case 'delete_request':
      return 'badge-error'
    case 'auth':
      return 'badge-info'
    default:
      return ''
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold">Dashboard</h1>
      <p class="text-base-content/70 mt-1">
        Overview of {{ authStore.currentOrg?.name }}
      </p>
    </div>
    
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Stats Grid -->
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <!-- Things -->
      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-primary">
            <span class="text-3xl sm:text-4xl">📦</span>
          </div>
          <div class="stat-title">Things</div>
          <div class="stat-value text-primary text-2xl sm:text-3xl">{{ stats.things }}</div>
          <div class="stat-desc">Connected devices</div>
        </div>
      </div>
      
      <!-- Edges -->
      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-secondary">
            <span class="text-3xl sm:text-4xl">🔌</span>
          </div>
          <div class="stat-title">Edges</div>
          <div class="stat-value text-secondary text-2xl sm:text-3xl">{{ stats.edges }}</div>
          <div class="stat-desc">Edge gateways</div>
        </div>
      </div>
      
      <!-- Locations -->
      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure text-accent">
            <span class="text-3xl sm:text-4xl">📍</span>
          </div>
          <div class="stat-title">Locations</div>
          <div class="stat-value text-accent text-2xl sm:text-3xl">{{ stats.locations }}</div>
          <div class="stat-desc">Physical sites</div>
        </div>
      </div>
      
      <!-- Team Members -->
      <div class="stats shadow">
        <div class="stat">
          <div class="stat-figure">
            <span class="text-3xl sm:text-4xl">👥</span>
          </div>
          <div class="stat-title">Team Members</div>
          <div class="stat-value text-2xl sm:text-3xl">{{ stats.members }}</div>
          <div class="stat-desc">Active users</div>
        </div>
      </div>
    </div>
    
    <!-- Recent Activity -->
    <div class="card bg-base-100 shadow-xl">
      <div class="card-body">
        <h2 class="card-title">Recent Activity</h2>
        
        <div v-if="recentActivity.length === 0" class="text-center py-8 text-base-content/50">
          No recent activity
        </div>
        
        <template v-else>
          <!-- Desktop Table -->
          <div class="hidden sm:block overflow-x-auto">
            <table class="table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>User</th>
                  <th>Action</th>
                  <th>Collection</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="log in recentActivity" :key="log.id">
                  <td class="whitespace-nowrap">
                    {{ formatRelativeTime(log.created) }}
                  </td>
                  <td>
                    {{ log.expand?.user?.name || log.expand?.user?.email || 'System' }}
                  </td>
                  <td>
                    <span 
                      class="badge badge-sm"
                      :class="getEventBadgeClass(log.event_type)"
                    >
                      {{ log.event_type }}
                    </span>
                  </td>
                  <td>
                    <code class="text-xs">{{ log.collection_name }}</code>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          
          <!-- Mobile Card View -->
          <div class="sm:hidden space-y-3">
            <div 
              v-for="log in recentActivity" 
              :key="log.id"
              class="card bg-base-200 p-3"
            >
              <div class="flex justify-between items-start mb-2">
                <span class="text-xs text-base-content/70">
                  {{ formatRelativeTime(log.created) }}
                </span>
                <span 
                  class="badge badge-sm"
                  :class="getEventBadgeClass(log.event_type)"
                >
                  {{ log.event_type }}
                </span>
              </div>
              <div class="text-sm font-medium">
                {{ log.expand?.user?.name || log.expand?.user?.email || 'System' }}
              </div>
              <div class="text-xs text-base-content/60 mt-1">
                <code>{{ log.collection_name }}</code>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
