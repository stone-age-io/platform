<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import type { User, NatsUser } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const toast = useToast()
const { confirm } = useConfirm()

// --- Profile State ---
const profileForm = ref({ name: '' })
const avatarFile = ref<File | null>(null)
const avatarPreview = ref<string | null>(null)
const profileLoading = ref(false)

// --- Password State ---
const passwordForm = ref({ oldPassword: '', password: '', passwordConfirm: '' })
const passwordLoading = ref(false)

// --- Connectivity State ---
const newUrl = ref('')
const availableIdentities = ref<NatsUser[]>([])
const loadingIdentities = ref(false)

// Helper to check if we are in "God Mode" (Fake membership)
const isFakeMembership = computed(() => {
  return authStore.currentMembership?.id.startsWith('super_')
})

// ============================================================================
// PROFILE ACTIONS
// ============================================================================

function initProfile() {
  if (authStore.user) {
    profileForm.value.name = authStore.user.name || ''
    if (authStore.user.avatar) {
      avatarPreview.value = pb.files.getUrl(authStore.user, authStore.user.avatar, { token: pb.authStore.token })
    }
  }
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    const file = target.files[0]
    avatarFile.value = file
    const reader = new FileReader()
    reader.onload = (e) => { avatarPreview.value = e.target?.result as string }
    reader.readAsDataURL(file)
  }
}

async function updateProfile() {
  if (!authStore.user) return
  profileLoading.value = true
  try {
    const formData = new FormData()
    formData.append('name', profileForm.value.name)
    if (avatarFile.value) formData.append('avatar', avatarFile.value)
    
    const collectionName = authStore.user.collectionName || 'users'
    const updatedUser = await pb.collection(collectionName).update(authStore.user.id, formData)
    
    authStore.user = updatedUser as unknown as User
    if (updatedUser.avatar) {
      avatarPreview.value = pb.files.getUrl(updatedUser, updatedUser.avatar, { token: pb.authStore.token })
    }
    toast.success('Profile updated')
    avatarFile.value = null
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    profileLoading.value = false
  }
}

async function updatePassword() {
  if (!authStore.user) return
  if (passwordForm.value.password !== passwordForm.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }
  passwordLoading.value = true
  try {
    const collectionName = authStore.user.collectionName || 'users'
    await pb.collection(collectionName).update(authStore.user.id, {
      oldPassword: passwordForm.value.oldPassword,
      password: passwordForm.value.password,
      passwordConfirm: passwordForm.value.passwordConfirm,
    })
    toast.success('Password changed')
    passwordForm.value = { oldPassword: '', password: '', passwordConfirm: '' }
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    passwordLoading.value = false
  }
}

// ============================================================================
// MEMBERSHIP / CONTEXT ACTIONS
// ============================================================================

async function loadIdentities() {
  if (!authStore.currentOrgId) return
  loadingIdentities.value = true
  try {
    // API rules ensure we only see NATS users for this org
    availableIdentities.value = await pb.collection('nats_users').getFullList<NatsUser>({
      sort: 'nats_username',
      filter: 'active = true' 
    })
  } catch (e) {
    console.warn('Could not load NATS identities', e)
  } finally {
    loadingIdentities.value = false
  }
}

async function updateContextIdentity(event: Event) {
  const select = event.target as HTMLSelectElement
  const natsUserId = select.value || null
  
  if (!authStore.currentMembership) return
  
  try {
    // Update the MEMBERSHIP record, not the user record
    await authStore.updateCurrentMembership({ nats_user: natsUserId || '' })
    
    // Force reconnection to pick up new creds
    if (natsStore.isConnected) {
      await natsStore.disconnect()
      // Wait for Vue reactivity to update computed currentNatsUser before reconnecting?
      // Actually updateCurrentMembership awaits the backend response, so state is fresh.
      natsStore.tryAutoConnect() 
      toast.info('Identity updated. Reconnecting...')
    } else {
      toast.success('Identity preference saved')
    }
  } catch (e: any) {
    toast.error(e.message)
  }
}

async function handleLeaveOrg() {
  const orgName = authStore.currentOrg?.name
  const confirmed = await confirm({
    title: 'Leave Organization',
    message: `Are you sure you want to leave ${orgName}?`,
    details: 'You will lose access to all resources in this organization immediately.',
    confirmText: 'Leave Organization',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    if (authStore.currentOrgId) {
      await authStore.leaveOrganization(authStore.currentOrgId)
      toast.success(`Left ${orgName}`)
      // Router will handle redirect if current route becomes invalid,
      // but usually authStore.loadContext() switches to another org automatically.
    }
  } catch (e: any) {
    toast.error(e.message)
  }
}

function handleAddUrl() {
  if (newUrl.value) {
    const url = newUrl.value.trim()
    if (!url.startsWith('ws://') && !url.startsWith('wss://')) {
      toast.error('URL must start with ws:// or wss://')
      return
    }
    natsStore.addUrl(url)
    newUrl.value = ''
  }
}

onMounted(() => {
  initProfile()
  natsStore.loadSettings()
  loadIdentities()
})

function triggerAvatarUpload() {
  document.getElementById('avatar-upload')?.click()
}

// Reload identities if user switches org while on Settings page
watch(() => authStore.currentOrgId, loadIdentities)
</script>

<template>
  <div class="space-y-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold">Account Settings</h1>
      <p class="text-base-content/70 mt-1">
        Manage your profile, security, and organization context
      </p>
    </div>

    <!-- 1. Global Profile -->
    <BaseCard>
      <template #header>
        <div class="flex justify-between items-center mb-2">
          <h3 class="card-title">Public Profile</h3>
          <div class="flex items-center gap-2">
            <span class="text-xs font-medium opacity-60">Dark Mode</span>
            <input 
              type="checkbox" 
              class="toggle toggle-sm toggle-primary" 
              :checked="uiStore.theme === 'dark'"
              @change="uiStore.toggleTheme"
            />
          </div>
        </div>
      </template>

      <form @submit.prevent="updateProfile" class="flex flex-col md:flex-row gap-8 items-start">
        <div class="flex flex-col items-center gap-3 shrink-0 mx-auto md:mx-0">
          <div class="avatar placeholder">
            <div class="w-24 md:w-32 rounded-full ring ring-primary ring-offset-base-100 ring-offset-2 overflow-hidden bg-neutral text-neutral-content">
              <img v-if="avatarPreview" :src="avatarPreview" alt="Avatar" class="object-cover w-full h-full" />
              <span v-else class="text-4xl font-bold">{{ profileForm.name?.[0]?.toUpperCase() || 'U' }}</span>
            </div>
          </div>
          
          <input id="avatar-upload" type="file" accept="image/*" class="hidden" @change="handleFileChange" />
          <button type="button" @click="triggerAvatarUpload" class="btn btn-xs btn-outline">Change Avatar</button>
        </div>

        <div class="flex-1 w-full grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="form-control">
            <label class="label"><span class="label-text">Display Name</span></label>
            <input v-model="profileForm.name" type="text" class="input input-bordered" placeholder="Your Name" />
          </div>
          <div class="form-control">
            <label class="label"><span class="label-text">Email</span></label>
            <input :value="authStore.user?.email" type="email" class="input input-bordered bg-base-200 text-base-content/70" disabled />
          </div>
          <div class="md:col-span-2 flex justify-end mt-2">
            <button type="submit" class="btn btn-primary px-8" :disabled="profileLoading">
              <span v-if="profileLoading" class="loading loading-spinner"></span> Save Profile
            </button>
          </div>
        </div>
      </form>
    </BaseCard>

    <!-- 2. Organization Context Settings -->
    <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
      
      <!-- Left: Context & Connectivity -->
      <BaseCard class="h-full">
        <template #header>
          <div class="flex justify-between items-center mb-2">
            <div>
              <h3 class="card-title">Context: {{ authStore.currentOrg?.name }}</h3>
              <p class="text-xs opacity-60">Settings specific to this organization</p>
            </div>
            <div v-if="!isFakeMembership && authStore.userRole !== 'owner'">
              <button @click="handleLeaveOrg" class="btn btn-xs btn-error btn-outline">Leave Org</button>
            </div>
          </div>
        </template>

        <div class="space-y-6">
          
          <!-- Identity Selector (Membership Bound) -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Operational Identity</span>
              <div class="tooltip tooltip-left" data-tip="Select the NATS credentials to use for this specific organization context">
                <span class="label-text-alt text-info cursor-help">ⓘ</span>
              </div>
            </label>
            
            <div v-if="isFakeMembership" class="alert alert-warning text-xs py-2">
              SuperUser God Mode active. Cannot bind identity. Join organization as member to configure.
            </div>
            
            <select 
              v-else
              class="select select-bordered font-mono text-sm" 
              :value="authStore.currentMembership?.nats_user || ''"
              @change="updateContextIdentity"
              :disabled="loadingIdentities"
            >
              <option value="">-- No Identity Linked --</option>
              <option v-for="u in availableIdentities" :key="u.id" :value="u.id">
                {{ u.nats_username }}
              </option>
            </select>
          </div>

          <div class="divider text-xs opacity-50 font-bold">NATS Connection</div>

          <!-- Connection URLs -->
          <div class="form-control">
            <label class="label"><span class="label-text">Server URLs</span></label>
            <div class="space-y-2 mb-2">
              <div v-for="url in natsStore.serverUrls" :key="url" class="flex gap-2 items-center bg-base-200 p-2 rounded border border-base-300">
                <span class="font-mono text-xs flex-1 truncate">{{ url }}</span>
                <button @click="natsStore.removeUrl(url)" class="btn btn-xs btn-ghost text-error">✕</button>
              </div>
              <div v-if="natsStore.serverUrls.length === 0" class="text-xs text-base-content/50 italic p-3 text-center bg-base-200 rounded border border-dashed border-base-300">
                No URLs configured.
              </div>
            </div>
            <div class="join w-full">
              <input v-model="newUrl" type="text" placeholder="wss://nats.example.com" class="input input-bordered input-sm join-item flex-1 font-mono" @keyup.enter="handleAddUrl" />
              <button @click="handleAddUrl" class="btn btn-sm btn-primary join-item">Add</button>
            </div>
          </div>

          <!-- Connection Controls -->
          <div class="bg-base-200/50 p-4 rounded-lg mt-4 border border-base-300 space-y-4">
            <div class="flex justify-between items-center">
              <div class="flex items-center gap-3">
                <div class="w-3 h-3 rounded-full transition-colors shadow-sm ring-2 ring-offset-2 ring-offset-base-200"
                  :class="{'bg-success ring-success/30': natsStore.status === 'connected', 'bg-warning ring-warning/30': natsStore.status === 'connecting' || natsStore.status === 'reconnecting', 'bg-error ring-error/30': natsStore.status === 'disconnected'}">
                </div>
                <div class="flex flex-col">
                  <span class="text-xs font-bold uppercase opacity-70">{{ natsStore.status }}</span>
                  <span v-if="natsStore.rtt" class="text-[10px] font-mono opacity-50">RTT: {{ natsStore.rtt }}ms</span>
                </div>
              </div>
              <button v-if="natsStore.isConnected" @click="natsStore.disconnect" class="btn btn-sm btn-error btn-outline">Disconnect</button>
              <button v-else @click="() => natsStore.connect()" class="btn btn-sm btn-success" :disabled="natsStore.status === 'connecting' || !natsStore.serverUrls.length">Connect</button>
            </div>
            <div class="form-control">
              <label class="label cursor-pointer gap-2 p-0 justify-start">
                <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" v-model="natsStore.autoConnect" @change="natsStore.saveSettings" />
                <span class="label-text text-xs font-medium">Auto-connect on login</span>
              </label>
            </div>
          </div>
        </div>
      </BaseCard>

      <!-- Right: Security -->
      <BaseCard title="Global Security" class="h-full">
        <form @submit.prevent="updatePassword" class="space-y-4">
          <div class="form-control">
            <label class="label"><span class="label-text">Current Password</span></label>
            <input v-model="passwordForm.oldPassword" type="password" class="input input-bordered" required />
          </div>
          <div class="form-control">
            <label class="label"><span class="label-text">New Password</span></label>
            <input v-model="passwordForm.password" type="password" class="input input-bordered" minlength="8" required />
          </div>
          <div class="form-control">
            <label class="label"><span class="label-text">Confirm New Password</span></label>
            <input v-model="passwordForm.passwordConfirm" type="password" class="input input-bordered" required />
          </div>
          <div class="flex justify-end mt-6">
            <button type="submit" class="btn btn-secondary w-full sm:w-auto" :disabled="passwordLoading">
              <span v-if="passwordLoading" class="loading loading-spinner"></span> Update Password
            </button>
          </div>
        </form>
      </BaseCard>
    </div>
  </div>
</template>
