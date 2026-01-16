<template>
  <Teleport to="body">
    <div v-if="modelValue" class="debug-overlay" @click.self="close">
      <div class="debug-panel">
        <div class="panel-header">
          <h3>🔧 Debug Panel</h3>
          <button class="close-btn" @click="close">✕</button>
        </div>
        
        <div class="panel-body">
          <!-- Connection Section -->
          <section class="debug-section">
            <h4>Connection</h4>
            <div class="stat-grid">
              <div class="stat-item">
                <span class="stat-label">Status</span>
                <span class="stat-value" :class="statusClass">{{ natsStore.status }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">RTT</span>
                <span class="stat-value">{{ natsStore.rtt ? `${natsStore.rtt}ms` : '—' }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Reconnects</span>
                <span class="stat-value" :class="{ 'text-warning': natsStore.reconnectCount > 0 }">
                  {{ natsStore.reconnectCount }}
                </span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Server</span>
                <span class="stat-value mono">{{ natsStore.serverUrls[0] || '—' }}</span>
              </div>
            </div>
            <div v-if="natsStore.lastError" class="error-box">{{ natsStore.lastError }}</div>
          </section>
          
          <!-- Subscription Manager Section -->
          <section class="debug-section">
            <h4>Subscription Manager</h4>
            <div class="stat-grid">
              <div class="stat-item">
                <span class="stat-label">Active Subs</span>
                <span class="stat-value">{{ subStats.subscriptionCount }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Queue Size</span>
                <span class="stat-value" :class="queueStatusClass">
                  {{ subStats.queueSize }} / {{ subStats.maxQueueSize }}
                </span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Msgs Received</span>
                <span class="stat-value">{{ formatNumber(subStats.messagesReceived) }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Msgs Dropped</span>
                <span class="stat-value" :class="{ 'text-error': subStats.messagesDropped > 0 }">
                  {{ formatNumber(subStats.messagesDropped) }}
                </span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Sub Errors</span>
                <span class="stat-value" :class="{ 'text-error': subStats.subscriptionErrors > 0 }">
                  {{ subStats.subscriptionErrors }}
                </span>
              </div>
              <div v-if="subStats.lastDropTime" class="stat-item">
                <span class="stat-label">Last Drop</span>
                <span class="stat-value text-warning">{{ formatTime(subStats.lastDropTime) }}</span>
              </div>
            </div>
            
            <!-- Subscription List -->
            <div v-if="subStats.subscriptions.length > 0" class="sub-list">
              <div class="sub-list-header">Active Subscriptions</div>
              <div 
                v-for="sub in subStats.subscriptions" 
                :key="sub.key"
                class="sub-item"
                :class="{ 'is-error': sub.error, 'is-inactive': !sub.isActive }"
              >
                <div class="sub-info">
                  <span class="sub-type" :class="sub.type.toLowerCase()">
                    {{ sub.type === 'JetStream' ? 'JS' : 'Core' }}
                  </span>
                  <span class="sub-subject">{{ sub.subject }}</span>
                  <span class="sub-listeners">{{ sub.listenerCount }} listener(s)</span>
                </div>
                <div v-if="sub.error" class="sub-error">{{ sub.error }}</div>
              </div>
            </div>
          </section>
          
          <!-- Buffer Stats Section -->
          <section class="debug-section">
            <h4>Data Buffers</h4>
            <div class="stat-grid">
              <div class="stat-item">
                <span class="stat-label">Active Buffers</span>
                <span class="stat-value">{{ dataStore.activeBufferCount }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Total Messages</span>
                <span class="stat-value">{{ formatNumber(dataStore.totalBufferedCount) }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Memory Est.</span>
                <span class="stat-value" :class="memoryStatusClass">{{ formatBytes(dataStore.memoryEstimate) }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Cumulative</span>
                <span class="stat-value">{{ formatNumber(dataStore.cumulativeCount) }}</span>
              </div>
            </div>
            
            <!-- Memory Usage Bar -->
            <div class="memory-bar-container">
              <div class="memory-bar-label">Memory Usage: {{ dataStore.memoryUsagePercent.toFixed(1) }}%</div>
              <div class="memory-bar">
                <div 
                  class="memory-bar-fill" 
                  :style="{ width: `${dataStore.memoryUsagePercent}%` }"
                  :class="memoryBarClass"
                ></div>
              </div>
            </div>
            
            <div v-if="dataStore.hasMemoryWarning" class="warning-box">{{ dataStore.memoryWarning }}</div>
          </section>
          
          <!-- Dashboard Stats -->
          <section class="debug-section">
            <h4>Dashboard</h4>
            <div class="stat-grid">
              <div class="stat-item">
                <span class="stat-label">Widgets</span>
                <span class="stat-value">{{ dashboardStore.activeWidgets.length }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Storage</span>
                <span class="stat-value">{{ storageSize.sizeKB.toFixed(1) }} KB</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Locked</span>
                <span class="stat-value">{{ dashboardStore.isLocked ? 'Yes' : 'No' }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">Dirty</span>
                <span class="stat-value" :class="{ 'text-warning': dashboardStore.isDirty }">
                  {{ dashboardStore.isDirty ? 'Yes' : 'No' }}
                </span>
              </div>
            </div>
          </section>
          
          <!-- Actions -->
          <section class="debug-section">
            <h4>Actions</h4>
            <div class="action-buttons">
              <button class="btn-action" @click="refreshStats">🔄 Refresh Stats</button>
              <button class="btn-action warning" @click="clearBuffers">🗑️ Clear Buffers</button>
              <button class="btn-action" @click="copyDebugInfo">📋 Copy Debug Info</button>
            </div>
          </section>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()
const dataStore = useWidgetDataStore()
const subManager = getSubscriptionManager()

const subStats = ref(subManager.getStats())
const storageSize = ref(dashboardStore.getStorageSize())

// Refresh stats when panel opens
watch(() => props.modelValue, (open) => {
  if (open) refreshStats()
})

function refreshStats() {
  subStats.value = subManager.getStats()
  storageSize.value = dashboardStore.getStorageSize()
}

function close() {
  emit('update:modelValue', false)
}

function clearBuffers() {
  if (confirm('Clear all message buffers? This cannot be undone.')) {
    dataStore.clearAllBuffers()
    refreshStats()
  }
}

function copyDebugInfo() {
  const info = {
    timestamp: new Date().toISOString(),
    connection: {
      status: natsStore.status,
      rtt: natsStore.rtt,
      reconnectCount: natsStore.reconnectCount,
      server: natsStore.serverUrls[0],
      lastError: natsStore.lastError
    },
    subscriptions: subStats.value,
    buffers: {
      count: dataStore.activeBufferCount,
      totalMessages: dataStore.totalBufferedCount,
      memoryEstimate: dataStore.memoryEstimate,
      memoryUsagePercent: dataStore.memoryUsagePercent,
      warning: dataStore.memoryWarning
    },
    dashboard: {
      widgets: dashboardStore.activeWidgets.length,
      locked: dashboardStore.isLocked,
      dirty: dashboardStore.isDirty,
      storage: storageSize.value
    }
  }
  navigator.clipboard.writeText(JSON.stringify(info, null, 2))
    .then(() => alert('Debug info copied!'))
    .catch(() => alert('Failed to copy'))
}

const statusClass = computed(() => ({
  'text-success': natsStore.status === 'connected',
  'text-warning': natsStore.status === 'reconnecting',
  'text-error': natsStore.status === 'disconnected'
}))

const queueStatusClass = computed(() => {
  const pct = (subStats.value.queueSize / subStats.value.maxQueueSize) * 100
  return {
    'text-success': pct < 50,
    'text-warning': pct >= 50 && pct < 80,
    'text-error': pct >= 80
  }
})

const memoryStatusClass = computed(() => {
  const pct = dataStore.memoryUsagePercent
  return {
    'text-success': pct < 50,
    'text-warning': pct >= 50 && pct < 80,
    'text-error': pct >= 80
  }
})

const memoryBarClass = computed(() => {
  const pct = dataStore.memoryUsagePercent
  if (pct >= 80) return 'bar-error'
  if (pct >= 50) return 'bar-warning'
  return 'bar-success'
})

function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

function formatTime(ts: number): string {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}
</script>

<style scoped>
.debug-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.debug-panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.panel-header h3 {
  margin: 0;
  font-size: 18px;
}

.close-btn {
  background: none;
  border: none;
  color: var(--muted);
  font-size: 20px;
  cursor: pointer;
}

.close-btn:hover { color: var(--text); }

.panel-body {
  padding: 16px 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.debug-section h4 {
  margin: 0 0 12px 0;
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  font-size: 11px;
  color: var(--muted);
  text-transform: uppercase;
}

.stat-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.stat-value.mono { font-family: var(--mono); font-size: 12px; }
.text-success { color: var(--color-success); }
.text-warning { color: var(--color-warning); }
.text-error { color: var(--color-error); }

.error-box, .warning-box {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 1.4;
}

.error-box {
  background: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  color: var(--color-error);
}

.warning-box {
  background: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  color: var(--color-warning);
}

/* Subscription List */
.sub-list {
  margin-top: 12px;
  border: 1px solid var(--border);
  border-radius: 4px;
  max-height: 200px;
  overflow-y: auto;
}

.sub-list-header {
  padding: 8px 12px;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  background: rgba(0, 0, 0, 0.1);
  border-bottom: 1px solid var(--border);
}

.sub-item {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
}

.sub-item:last-child { border-bottom: none; }
.sub-item.is-error { background: var(--color-error-bg); }
.sub-item.is-inactive { opacity: 0.5; }

.sub-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sub-type {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 3px;
  text-transform: uppercase;
}

.sub-type.core {
  background: var(--color-info-bg);
  color: var(--color-info);
}

.sub-type.jetstream {
  background: var(--color-success-bg);
  color: var(--color-success);
}

.sub-subject {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--text);
  flex: 1;
}

.sub-listeners {
  font-size: 11px;
  color: var(--muted);
}

.sub-error {
  margin-top: 4px;
  font-size: 11px;
  color: var(--color-error);
}

/* Memory Bar */
.memory-bar-container { margin-top: 12px; }

.memory-bar-label {
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 4px;
}

.memory-bar {
  height: 8px;
  background: var(--border);
  border-radius: 4px;
  overflow: hidden;
}

.memory-bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

.bar-success { background: var(--color-success); }
.bar-warning { background: var(--color-warning); }
.bar-error { background: var(--color-error); }

/* Actions */
.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-action {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-action:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--color-accent);
}

.btn-action.warning:hover {
  border-color: var(--color-warning);
  color: var(--color-warning);
}
</style>
