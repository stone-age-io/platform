<template>
  <div class="sidebar-container">
    <aside class="sidebar">
      <!-- Header -->
      <div class="sidebar-header">
        <h2 class="sidebar-title">Dashboards</h2>
        <!-- CHANGED: Removed v-if="isMobile", always show close button in drawer mode -->
        <button 
          class="close-btn"
          @click="closeSidebar"
          title="Close sidebar"
        >
          ✕
        </button>
      </div>
      
      <!-- Storage Tabs -->
      <div v-if="dashboardStore.enableSharedDashboards" class="storage-tabs">
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'local' }"
          @click="activeTab = 'local'"
        >
          Local
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'shared' }"
          @click="activeTab = 'shared'"
        >
          Shared
        </button>
      </div>
      
      <!-- Dashboard list -->
      <div class="sidebar-body">
        <!-- LOCAL TAB -->
        <template v-if="activeTab === 'local'">
          <div v-if="dashboards.length === 0" class="empty-state">
            <div class="empty-icon">📊</div>
            <div class="empty-text">No local dashboards</div>
          </div>
          
          <div v-else class="dashboard-list">
            <DashboardListItem
              v-for="dashboard in dashboards"
              :key="dashboard.id"
              :dashboard="dashboard"
              :is-active="dashboard.id === activeDashboardId"
              @select="selectDashboard(dashboard)"
              @rename="startRename(dashboard.id)"
              @duplicate="startDuplicateLocal(dashboard.id)"
              @export="exportDashboard(dashboard.id)"
              @delete="confirmDelete(dashboard.id, 'local')"
              @share="handleShare(dashboard.id)"
            />
          </div>
        </template>

        <!-- SHARED TAB -->
        <template v-if="activeTab === 'shared'">
          <div v-if="!natsStore.isConnected" class="empty-state">
            <div class="empty-icon">🔌</div>
            <div class="empty-text">Connect to NATS to view shared dashboards</div>
          </div>
          
          <div v-else-if="remoteKeys.length === 0" class="empty-state">
            <div class="empty-icon">☁️</div>
            <div class="empty-text">No shared dashboards found</div>
            <div class="empty-subtext">Bucket: {{ dashboardStore.kvBucketName }}</div>
          </div>
          
          <div v-else class="dashboard-tree-container">
            <DashboardTree 
              :structure="remoteTree"
              :active-id="activeDashboardId"
              @select="selectRemoteDashboard"
              @delete="confirmDelete($event, 'remote')"
              @duplicate="handleRemoteDuplicate"
              @export="handleRemoteExport"
            />
          </div>
        </template>
      </div>
      
      <!-- Footer with actions -->
      <div class="sidebar-footer">
        <!-- Storage indicator (Local only) -->
        <div v-if="activeTab === 'local'" class="storage-indicator" :class="storageClass">
          <div class="storage-bar-bg">
            <div 
              class="storage-bar-fill"
              :style="{ width: storagePercent + '%' }"
            />
          </div>
          <div class="storage-text">
            {{ storageKB }} KB / 5 MB
          </div>
        </div>
        
        <!-- Action buttons -->
        <div class="action-buttons">
          <button 
            class="btn-action btn-new"
            @click="createNewDashboard"
          >
            <span class="btn-icon">+</span>
            <span class="btn-text">New {{ activeTab === 'local' ? 'Local' : 'Shared' }}</span>
          </button>
          
          <template v-if="activeTab === 'local'">
            <button 
              class="btn-action"
              @click="showImport = true"
            >
              <span class="btn-icon">📥</span>
              <span class="btn-text">Import</span>
            </button>
            
            <button 
              class="btn-action"
              :disabled="dashboards.length === 0"
              @click="exportAllDashboards"
            >
              <span class="btn-icon">💾</span>
              <span class="btn-text">Export All</span>
            </button>
          </template>
          
          <template v-if="activeTab === 'shared'">
             <button 
              class="btn-action"
              @click="dashboardStore.refreshRemoteKeys()"
              :disabled="!natsStore.isConnected"
            >
              <span class="btn-icon">🔄</span>
              <span class="btn-text">Refresh</span>
            </button>
          </template>
        </div>
      </div>
    </aside>
    
    <!-- Modals (Teleported) -->
    <!-- Rename Modal -->
    <Teleport to="body">
      <div v-if="showRename" class="modal-overlay" @click.self="showRename = false">
        <div class="nd-modal">
          <div class="modal-header">
            <h3>Rename Dashboard</h3>
            <button class="close-btn" @click="showRename = false">✕</button>
          </div>
          <div class="modal-body">
            <input 
              ref="renameInput"
              v-model="renameName"
              type="text"
              class="form-input"
              placeholder="Dashboard name"
              @keyup.enter="saveRename"
              @keyup.escape="showRename = false"
            />
          </div>
          <div class="modal-actions">
            <button class="btn-secondary" @click="showRename = false">Cancel</button>
            <button class="btn-primary" @click="saveRename">Rename</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Duplicate Local Modal -->
    <Teleport to="body">
      <div v-if="showDuplicateLocal" class="modal-overlay" @click.self="showDuplicateLocal = false">
        <div class="nd-modal">
          <div class="modal-header">
            <h3>Duplicate Dashboard</h3>
            <button class="close-btn" @click="showDuplicateLocal = false">✕</button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label>New Name</label>
              <input 
                ref="duplicateLocalInput"
                v-model="duplicateLocalName"
                type="text"
                class="form-input"
                placeholder="Dashboard Name"
                @keyup.enter="saveDuplicateLocal"
                @keyup.escape="showDuplicateLocal = false"
              />
            </div>
          </div>
          <div class="modal-actions">
            <button class="btn-secondary" @click="showDuplicateLocal = false">Cancel</button>
            <button class="btn-primary" @click="saveDuplicateLocal">Duplicate</button>
          </div>
        </div>
      </div>
    </Teleport>
    
    <!-- Share / Create / Duplicate Remote Modal -->
    <Teleport to="body">
      <div v-if="showShareModal" class="modal-overlay" @click.self="showShareModal = false">
        <div class="nd-modal">
          <div class="modal-header">
            <h3>
              {{ 
                modalMode === 'create' ? 'Create Shared Dashboard' : 
                modalMode === 'duplicate' ? 'Duplicate Shared Dashboard' : 'Share to KV' 
              }}
            </h3>
            <button class="close-btn" @click="showShareModal = false">✕</button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label>Dashboard Name</label>
              <input 
                v-model="shareForm.name"
                type="text"
                class="form-input"
                placeholder="My Dashboard"
                @keyup.enter="handleModalSubmit"
              />
            </div>
            
            <div class="form-group">
              <label>Folder</label>
              <div class="folder-selection-mode">
                <label class="radio-label">
                  <input type="radio" v-model="shareForm.mode" value="existing" :disabled="knownFolders.length === 0">
                  Existing
                </label>
                <label class="radio-label">
                  <input type="radio" v-model="shareForm.mode" value="new">
                  New
                </label>
              </div>
              
              <select 
                v-if="shareForm.mode === 'existing'" 
                v-model="shareForm.folder" 
                class="form-input"
              >
                <option value="">(Root)</option>
                <option v-for="folder in knownFolders" :key="folder" :value="folder">
                  {{ folder }}
                </option>
              </select>
              
              <input 
                v-else
                v-model="shareForm.folder"
                type="text"
                class="form-input"
                placeholder="e.g. ops.prod"
                @keyup.enter="handleModalSubmit"
              />
              <div class="help-text">
                Use dots for subfolders (e.g. <code>ops.prod</code>)
              </div>
            </div>
          </div>
          <div class="modal-actions">
            <button class="btn-secondary" @click="showShareModal = false">Cancel</button>
            <button class="btn-primary" @click="handleModalSubmit">
              {{ 
                modalMode === 'create' ? 'Create' : 
                modalMode === 'duplicate' ? 'Duplicate' : 'Share' 
              }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
    
    <!-- Import Modal -->
    <Teleport to="body">
      <div v-if="showImport" class="modal-overlay" @click.self="showImport = false">
        <div class="nd-modal">
          <div class="modal-header">
            <h3>Import Dashboards</h3>
            <button class="close-btn" @click="showImport = false">✕</button>
          </div>
          <div class="modal-body">
            <input 
              type="file"
              accept=".json"
              @change="handleImportFile"
              class="file-input"
            />
            <div class="help-text">
              Select a JSON file exported from NATS Dashboard
            </div>
            
            <!-- Import results -->
            <div v-if="importResults" class="import-results">
              <div v-if="importResults.success > 0" class="result-success">
                ✓ Imported {{ importResults.success }} dashboard{{ importResults.success !== 1 ? 's' : '' }}
              </div>
              <div v-if="importResults.skipped > 0" class="result-warning">
                ⚠️ Skipped {{ importResults.skipped }} dashboard{{ importResults.skipped !== 1 ? 's' : '' }}
              </div>
              <div v-if="importResults.errors.length > 0" class="result-errors">
                <div class="error-title">Errors:</div>
                <div v-for="(error, i) in importResults.errors" :key="i" class="error-item">
                  {{ error }}
                </div>
              </div>
            </div>
          </div>
          <div class="modal-actions">
            <button class="btn-primary" @click="showImport = false">Close</button>
          </div>
        </div>
      </div>
    </Teleport>
    
    <!-- Confirm Delete Dialog -->
    <ConfirmDialog
      v-model="showConfirmDelete"
      :title="deleteConfirmTitle"
      :message="deleteConfirmMessage"
      details="This action cannot be undone."
      confirm-text="Delete"
      variant="danger"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import { useNatsStore } from '@/stores/nats'
import DashboardListItem from './DashboardListItem.vue'
import DashboardTree from './DashboardTree.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import type { Dashboard } from '@/types/dashboard'

// CHANGED: Define emits to notify parent
const emit = defineEmits(['close'])

const dashboardStore = useDashboardStore()
const natsStore = useNatsStore()

// Sidebar state
const activeTab = ref<'local' | 'shared'>('local')

// Rename state
const showRename = ref(false)
const renameId = ref<string | null>(null)
const renameName = ref('')
const renameInput = ref<HTMLInputElement | null>(null)

// Duplicate Local state
const showDuplicateLocal = ref(false)
const duplicateLocalId = ref<string | null>(null)
const duplicateLocalName = ref('')
const duplicateLocalInput = ref<HTMLInputElement | null>(null)

// Share/Create/Duplicate Remote state
const showShareModal = ref(false)
const modalMode = ref<'share' | 'create' | 'duplicate'>('share')
const shareForm = ref({
  id: '',
  name: '',
  folder: '',
  mode: 'existing' as 'existing' | 'new'
})

// Import state
const showImport = ref(false)
const importResults = ref<{ success: number; skipped: number; errors: string[] } | null>(null)

// Delete confirmation state
const showConfirmDelete = ref(false)
const deleteId = ref<string | null>(null)
const deleteType = ref<'local' | 'remote'>('local')

// Computed
const dashboards = computed(() => dashboardStore.localDashboards)
const remoteKeys = computed(() => dashboardStore.remoteKeys)
const knownFolders = computed(() => dashboardStore.knownFolders)

const activeDashboardId = computed(() => {
  if (activeTab.value === 'local' && dashboardStore.activeDashboard?.storage === 'local') {
    return dashboardStore.activeDashboardId
  }
  if (activeTab.value === 'shared' && dashboardStore.activeDashboard?.storage === 'kv') {
    return dashboardStore.activeDashboard.kvKey || null
  }
  return null
})

const remoteTree = computed(() => {
  const tree: any = {}
  for (const key of remoteKeys.value) {
    const parts = key.split('.')
    let current = tree
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      if (i === parts.length - 1) {
        current[part] = key
      } else {
        if (!current[part]) current[part] = {}
        current = current[part]
      }
    }
  }
  return tree
})

const storageInfo = computed(() => dashboardStore.getStorageSize())
const storageKB = computed(() => (storageInfo.value.sizeKB / 1024).toFixed(2))
const storagePercent = computed(() => Math.min(storageInfo.value.sizePercent, 100))
const storageClass = computed(() => {
  const percent = storagePercent.value
  if (percent >= 90) return 'storage-critical'
  if (percent >= 70) return 'storage-warning'
  return 'storage-ok'
})

const deleteConfirmTitle = computed(() => {
  return deleteType.value === 'local' ? 'Delete Local Dashboard?' : 'Delete Shared Dashboard?'
})

const deleteConfirmMessage = computed(() => {
  return `This will permanently delete the dashboard.`
})

// --- Actions ---

// CHANGED: Emit close event to parent
function closeSidebar() {
  emit('close')
}

function selectDashboard(dashboard: Dashboard) {
  dashboardStore.setActiveDashboard(dashboard)
  // CHANGED: Close sidebar on selection for better UX on mobile/drawer
  emit('close')
}

function selectRemoteDashboard(key: string) {
  dashboardStore.loadRemoteDashboard(key)
  // CHANGED: Close sidebar on selection
  emit('close')
}

function createNewDashboard() {
  if (activeTab.value === 'local') {
    const name = prompt('Dashboard name:', 'New Dashboard')
    if (name) dashboardStore.createDashboard(name)
  } else {
    shareForm.value = {
      id: '',
      name: 'New Dashboard',
      folder: '',
      mode: knownFolders.value.length > 0 ? 'existing' : 'new'
    }
    modalMode.value = 'create'
    showShareModal.value = true
  }
}

function startRename(id: string) {
  const dashboard = dashboards.value.find(d => d.id === id)
  if (!dashboard) return
  renameId.value = id
  renameName.value = dashboard.name
  showRename.value = true
  nextTick(() => {
    renameInput.value?.focus()
    renameInput.value?.select()
  })
}

function saveRename() {
  if (!renameId.value || !renameName.value.trim()) return
  dashboardStore.renameDashboard(renameId.value, renameName.value)
  showRename.value = false
  renameId.value = null
  renameName.value = ''
}

function startDuplicateLocal(id: string) {
  const dashboard = dashboards.value.find(d => d.id === id)
  if (!dashboard) return
  duplicateLocalId.value = id
  duplicateLocalName.value = `${dashboard.name} (Copy)`
  showDuplicateLocal.value = true
  nextTick(() => {
    duplicateLocalInput.value?.focus()
    duplicateLocalInput.value?.select()
  })
}

function saveDuplicateLocal() {
  if (!duplicateLocalId.value || !duplicateLocalName.value.trim()) return
  dashboardStore.duplicateDashboard(duplicateLocalId.value, duplicateLocalName.value.trim())
  showDuplicateLocal.value = false
  duplicateLocalId.value = null
  duplicateLocalName.value = ''
}

function exportDashboard(id: string) {
  const json = dashboardStore.exportDashboard(id)
  if (!json) return
  const dashboard = dashboards.value.find(d => d.id === id)
  if (!dashboard) return
  downloadJSON(json, `${dashboard.name}.json`)
}

function exportAllDashboards() {
  const json = dashboardStore.exportAllDashboards()
  const timestamp = new Date().toISOString().split('T')[0]
  downloadJSON(json, `nats-dashboards-${timestamp}.json`)
}

function downloadJSON(json: string, filename: string) {
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

async function handleImportFile(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const results = dashboardStore.importDashboards(text, 'merge')
    importResults.value = results
    target.value = ''
  } catch (err) {
    console.error('[Sidebar] Import error:', err)
    importResults.value = { success: 0, skipped: 0, errors: ['Failed to read file'] }
  }
}

function confirmDelete(idOrKey: string, type: 'local' | 'remote') {
  deleteId.value = idOrKey
  deleteType.value = type
  showConfirmDelete.value = true
}

function handleDelete() {
  if (!deleteId.value) return
  if (deleteType.value === 'local') {
    dashboardStore.deleteDashboard(deleteId.value)
  } else {
    dashboardStore.deleteRemoteDashboard(deleteId.value)
  }
  deleteId.value = null
}

function handleShare(id: string) {
  const dashboard = dashboards.value.find(d => d.id === id)
  if (!dashboard) return
  shareForm.value = {
    id: id,
    name: dashboard.name,
    folder: '',
    mode: knownFolders.value.length > 0 ? 'existing' : 'new'
  }
  modalMode.value = 'share'
  showShareModal.value = true
}

function handleModalSubmit() {
  if (!shareForm.value.name.trim()) return
  
  if (modalMode.value === 'share') {
    dashboardStore.uploadLocalToRemote(
      shareForm.value.id, 
      shareForm.value.folder.trim(),
      shareForm.value.name.trim()
    )
  } else if (modalMode.value === 'create') {
    dashboardStore.createRemoteDashboard(
      shareForm.value.name.trim(),
      shareForm.value.folder.trim()
    )
  } else if (modalMode.value === 'duplicate') {
    dashboardStore.duplicateRemoteDashboard(
      shareForm.value.id, // sourceKey
      shareForm.value.name.trim(),
      shareForm.value.folder.trim()
    )
  }
  
  showShareModal.value = false
  // CHANGED: Emit close event
  emit('close')
}

async function handleRemoteDuplicate(key: string) {
  const json = await dashboardStore.fetchRemoteDashboardJson(key)
  if (!json) return

  let currentName = 'Dashboard'
  try {
    const parsed = JSON.parse(json)
    currentName = parsed.name
  } catch {}

  const lastDot = key.lastIndexOf('.')
  let currentFolder = ''
  if (lastDot > -1) {
    currentFolder = key.substring(0, lastDot)
  }

  shareForm.value = {
    id: key,
    name: `${currentName} (Copy)`,
    folder: currentFolder,
    mode: knownFolders.value.length > 0 ? 'existing' : 'new'
  }
  
  modalMode.value = 'duplicate'
  showShareModal.value = true
}

async function handleRemoteExport(key: string) {
  const json = await dashboardStore.fetchRemoteDashboardJson(key)
  if (!json) return
  
  let filename = 'shared-dashboard.json'
  try {
    const parsed = JSON.parse(json)
    if (parsed.name) filename = `${parsed.name}.json`
  } catch {}
  
  const exportData = {
    version: '1.0',
    exportDate: Date.now(),
    appVersion: '0.1.0',
    dashboards: [JSON.parse(json)]
  }
  
  downloadJSON(JSON.stringify(exportData, null, 2), filename)
}

onMounted(() => {
  if (dashboardStore.activeDashboard?.storage === 'kv') {
    activeTab.value = 'shared'
  }
  if (!dashboardStore.enableSharedDashboards) {
    activeTab.value = 'local'
  }
})

watch(() => dashboardStore.enableSharedDashboards, (enabled) => {
  if (!enabled) activeTab.value = 'local'
})

watch(() => dashboardStore.activeDashboard?.storage, (storage) => {
  if (storage === 'kv' && dashboardStore.enableSharedDashboards) {
    activeTab.value = 'shared'
  } else if (storage === 'local') {
    activeTab.value = 'local'
  }
})
</script>

<style scoped>
/* 
   Updated for Drawer Layout
   The parent wrapper handles width and positioning.
   This component just needs to fill the wrapper.
*/

.sidebar-container {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
}

.sidebar {
  width: 100%;
  height: 100%;
  background: oklch(var(--b1));
  border-right: 1px solid oklch(var(--b3));
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Remove resize handle since it's a drawer now */
.resize-handle {
  display: none; 
}

.sidebar-header {
  padding: 20px;
  border-bottom: 1px solid oklch(var(--b3));
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.sidebar-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: oklch(var(--bc));
  white-space: nowrap;
}

/* CHANGED: Ensure close button is visible */
.close-btn {
  background: none;
  border: none;
  color: oklch(var(--bc) / 0.5);
  font-size: 24px;
  cursor: pointer;
  padding: 4px;
  display: block; /* Always visible in drawer mode */
}

/* Tabs */
.storage-tabs {
  display: flex;
  padding: 12px;
  gap: 8px;
  border-bottom: 1px solid oklch(var(--b3));
}

.tab-btn {
  flex: 1;
  padding: 6px;
  background: transparent;
  border: 1px solid oklch(var(--b3));
  border-radius: 4px;
  color: oklch(var(--bc) / 0.5);
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: all 0.2s;
}

.tab-btn:hover {
  color: oklch(var(--bc));
  border-color: oklch(var(--a));
}

.tab-btn.active {
  background: oklch(var(--a));
  border-color: oklch(var(--a));
  color: white;
}

.sidebar-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}

.dashboard-tree-container {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: oklch(var(--bc) / 0.5);
  text-align: center;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  opacity: 0.5;
}

.empty-text {
  font-size: 14px;
  margin-bottom: 4px;
}

.empty-subtext {
  font-size: 11px;
  font-family: var(--mono);
  opacity: 0.7;
}

.dashboard-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* Footer */
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid oklch(var(--b3));
  flex-shrink: 0;
}

.storage-indicator {
  margin-bottom: 12px;
}

.storage-bar-bg {
  height: 4px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
  overflow: hidden;
  margin-bottom: 6px;
}

.storage-bar-fill {
  height: 100%;
  background: oklch(var(--su));
  transition: all 0.3s;
}

.storage-indicator.storage-warning .storage-bar-fill {
  background: oklch(var(--wa));
}

.storage-indicator.storage-critical .storage-bar-fill {
  background: oklch(var(--er));
}

.storage-text {
  font-size: 11px;
  color: oklch(var(--bc) / 0.5);
  text-align: center;
}

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.btn-action {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  color: oklch(var(--bc));
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.btn-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.15);
  border-color: oklch(var(--a));
}

.btn-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-action.btn-new {
  background: oklch(var(--p));
  border-color: oklch(var(--p));
  color: white;
}

.btn-action.btn-new:hover:not(:disabled) {
  background: oklch(var(--pf));
}

.btn-icon {
  font-size: 16px;
  line-height: 1;
}

.btn-text {
  flex: 1;
}
</style>
