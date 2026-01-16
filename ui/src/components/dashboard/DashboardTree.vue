<template>
  <div class="dashboard-tree">
    <!-- Folders -->
    <div v-for="(content, folderName) in folders" :key="folderName" class="tree-folder">
      <div 
        class="folder-header" 
        @click="toggleFolder(folderName)"
        :class="{ 'is-open': openFolders[folderName] }"
        :title="folderName"
      >
        <span class="folder-chevron">{{ openFolders[folderName] ? '▼' : '▶' }}</span>
        <span class="folder-icon">📁</span>
        <span class="folder-name">{{ folderName }}</span>
      </div>
      
      <div v-if="openFolders[folderName]" class="folder-content">
        <!-- Recursive call for subfolders -->
        <DashboardTree 
          v-if="hasSubfolders(content)"
          :structure="content"
          :active-id="activeId"
          @select="$emit('select', $event)"
          @delete="$emit('delete', $event)"
          @duplicate="$emit('duplicate', $event)"
          @export="$emit('export', $event)"
        />
      </div>
    </div>

    <!-- Items (Dashboards) in this level -->
    <div v-for="item in items" :key="item.key" class="tree-item">
      <div 
        class="dashboard-item"
        :class="{ 'is-active': item.key === activeId }"
        @click="$emit('select', item.key)"
        :title="item.name"
      >
        <span class="item-icon">
          <span v-if="isStartup(item.key)" title="Startup Dashboard">🏠</span>
          <span v-else>📊</span>
        </span>
        <span class="item-name">{{ item.name }}</span>
        
        <!-- Actions menu -->
        <div class="item-actions" @click.stop>
          <button 
            class="action-btn"
            :class="{ 'is-open': activeMenuId === item.key }"
            @click="toggleMenu(item.key)"
            title="Actions"
          >
            ⋮
          </button>
          
          <div v-if="activeMenuId === item.key" class="action-menu">
            <button class="menu-item" @click="handleSetStartup(item.key)">
              <span class="menu-icon">🏠</span>
              <span class="menu-text">Set as Startup</span>
            </button>
            
            <div class="menu-divider"></div>
            
            <button class="menu-item" @click="handleDuplicate(item.key)">
              <span class="menu-icon">📋</span>
              <span class="menu-text">Duplicate</span>
            </button>
            <button class="menu-item" @click="handleExport(item.key)">
              <span class="menu-icon">💾</span>
              <span class="menu-text">Export</span>
            </button>
            
            <div class="menu-divider"></div>
            
            <button class="menu-item danger" @click="handleDelete(item.key)">
              <span class="menu-icon">🗑️</span>
              <span class="menu-text">Delete</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'

// Define recursive structure
interface TreeStructure {
  [key: string]: TreeStructure | string // string is the leaf (key)
}

const props = defineProps<{
  structure: TreeStructure
  activeId: string | null
}>()

const emit = defineEmits<{
  select: [key: string]
  delete: [key: string]
  duplicate: [key: string]
  export: [key: string]
}>()

const dashboardStore = useDashboardStore()
const openFolders = ref<Record<string, boolean>>({})
const activeMenuId = ref<string | null>(null)

// Separate items (leaves) from folders
const items = computed(() => {
  const result: { name: string; key: string }[] = []
  for (const [key, value] of Object.entries(props.structure)) {
    if (typeof value === 'string') {
      result.push({ name: key, key: value })
    }
  }
  return result.sort((a, b) => a.name.localeCompare(b.name))
})

const folders = computed(() => {
  const result: Record<string, TreeStructure> = {}
  for (const [key, value] of Object.entries(props.structure)) {
    if (typeof value !== 'string') {
      result[key] = value
    }
  }
  return result
})

function toggleFolder(name: string) {
  openFolders.value[name] = !openFolders.value[name]
}

function hasSubfolders(content: any): boolean {
  return typeof content === 'object' && content !== null
}

function isStartup(key: string): boolean {
  return dashboardStore.startupDashboard?.id === key && 
         dashboardStore.startupDashboard?.storage === 'kv'
}

function toggleMenu(key: string) {
  activeMenuId.value = activeMenuId.value === key ? null : key
}

function handleSetStartup(key: string) {
  dashboardStore.setStartupDashboard(key, 'kv')
  activeMenuId.value = null
}

function handleDuplicate(key: string) {
  activeMenuId.value = null
  emit('duplicate', key)
}

function handleExport(key: string) {
  activeMenuId.value = null
  emit('export', key)
}

function handleDelete(key: string) {
  activeMenuId.value = null
  emit('delete', key)
}

function handleClickOutside(event: MouseEvent) {
  if (activeMenuId.value) {
    const target = event.target as HTMLElement
    if (!target.closest('.item-actions')) {
      activeMenuId.value = null
    }
  }
}

// --- Auto-Expand Logic ---

function containsKey(item: TreeStructure | string, targetKey: string): boolean {
  if (typeof item === 'string') {
    return item === targetKey
  }
  return Object.values(item).some(child => containsKey(child, targetKey))
}

watch([() => props.activeId, folders], ([newId, currentFolders]) => {
  if (!newId) return
  
  for (const [folderName, content] of Object.entries(currentFolders)) {
    if (containsKey(content, newId)) {
      openFolders.value[folderName] = true
    }
  }
}, { immediate: true })

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dashboard-tree {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.tree-folder {
  margin-bottom: 1px;
}

.folder-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 4px;
  cursor: pointer;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 500;
  user-select: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: all 0.2s;
}

.folder-header:hover {
  background: rgba(255, 255, 255, 0.05);
  color: var(--text);
}

.folder-chevron {
  font-size: 10px;
  width: 16px;
  text-align: center;
  color: var(--muted);
  transition: transform 0.2s;
  flex-shrink: 0;
}

.folder-icon {
  font-size: 18px;
  flex-shrink: 0;
  opacity: 0.8;
}

.folder-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.folder-content {
  padding-left: 10px;
  border-left: 1px solid var(--border);
  margin-left: 12px;
  margin-top: 1px;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.tree-item {
  margin-bottom: 1px;
}

.dashboard-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text);
  font-size: 15px;
  font-weight: 500;
  border: 1px solid transparent;
}

.dashboard-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.dashboard-item.is-active {
  background: var(--color-info-bg);
  color: var(--color-info);
  font-weight: 600;
  border-color: var(--color-info-border);
}

.item-icon {
  font-size: 18px;
  opacity: 0.7;
  width: 18px;
  text-align: center;
  flex-shrink: 0;
}

.item-name {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-actions {
  position: relative;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.action-btn {
  opacity: 0;
  background: none;
  border: none;
  color: var(--muted);
  cursor: pointer;
  font-size: 16px;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  border-radius: 4px;
}

.dashboard-item:hover .action-btn,
.action-btn.is-open {
  opacity: 1;
}

.action-btn:hover,
.action-btn.is-open {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
}

/* Action Menu Dropdown */
.action-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  min-width: 160px;
  z-index: 100;
  animation: slideDown 0.15s ease-out;
}

@keyframes slideDown {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.menu-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text);
  font-size: 13px;
  transition: all 0.15s;
  text-align: left;
}

.menu-item:hover {
  background: rgba(255, 255, 255, 0.1);
}

.menu-item.danger:hover {
  background: var(--color-error-bg);
  color: var(--color-error);
}

.menu-icon {
  font-size: 14px;
  line-height: 1;
}

.menu-divider {
  height: 1px;
  background: var(--border);
  margin: 4px 0;
}
</style>
