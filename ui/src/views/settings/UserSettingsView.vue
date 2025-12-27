<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import type { User } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const authStore = useAuthStore()
const uiStore = useUIStore()
const toast = useToast()

// Profile State
const profileForm = ref({
  name: '',
})
const avatarFile = ref<File | null>(null)
const avatarPreview = ref<string | null>(null)
const profileLoading = ref(false)

// Password State
const passwordForm = ref({
  oldPassword: '',
  password: '',
  passwordConfirm: '',
})
const passwordLoading = ref(false)

/**
 * Initialize form data from current user
 */
function initProfile() {
  if (authStore.user) {
    profileForm.value.name = authStore.user.name || ''
    
    // Set initial avatar preview if exists
    if (authStore.user.avatar) {
      avatarPreview.value = pb.files.getUrl(authStore.user, authStore.user.avatar)
    }
  }
}

/**
 * Handle Avatar File Selection
 */
function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    const file = target.files[0]
    avatarFile.value = file
    
    // Create local preview
    const reader = new FileReader()
    reader.onload = (e) => {
      avatarPreview.value = e.target?.result as string
    }
    reader.readAsDataURL(file)
  }
}

/**
 * Update Profile (Name & Avatar)
 */
async function updateProfile() {
  if (!authStore.user) return
  
  profileLoading.value = true
  
  try {
    const formData = new FormData()
    formData.append('name', profileForm.value.name)
    
    if (avatarFile.value) {
      formData.append('avatar', avatarFile.value)
    }
    
    // Update in PocketBase
    const updatedUser = await pb.collection('users').update(authStore.user.id, formData)
    
    // Update local store with type assertion
    authStore.user = updatedUser as unknown as User
    
    // Refresh preview from server URL to ensure consistency
    if (updatedUser.avatar) {
      // We can use the updatedUser record directly for the file URL
      avatarPreview.value = pb.files.getUrl(updatedUser, updatedUser.avatar)
    }
    
    toast.success('Profile updated successfully')
    
    // Clear file input
    avatarFile.value = null
  } catch (err: any) {
    toast.error(err.message || 'Failed to update profile')
  } finally {
    profileLoading.value = false
  }
}

/**
 * Update Password
 */
async function updatePassword() {
  if (!authStore.user) return
  
  if (passwordForm.value.password !== passwordForm.value.passwordConfirm) {
    toast.error('New passwords do not match')
    return
  }
  
  passwordLoading.value = true
  
  try {
    await pb.collection('users').update(authStore.user.id, {
      oldPassword: passwordForm.value.oldPassword,
      password: passwordForm.value.password,
      passwordConfirm: passwordForm.value.passwordConfirm,
    })
    
    toast.success('Password changed successfully')
    
    // Reset form
    passwordForm.value = {
      oldPassword: '',
      password: '',
      passwordConfirm: '',
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to update password')
  } finally {
    passwordLoading.value = false
  }
}

/**
 * Trigger hidden file input
 */
function triggerFileInput() {
  document.getElementById('avatar-upload')?.click()
}

onMounted(() => {
  initProfile()
})
</script>

<template>
  <div class="space-y-6 max-w-4xl mx-auto">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold">Account Settings</h1>
      <p class="text-base-content/70 mt-1">
        Manage your profile and security preferences
      </p>
    </div>

    <!-- Main Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      
      <!-- Profile Section -->
      <BaseCard title="Public Profile">
        <form @submit.prevent="updateProfile" class="space-y-6">
          
          <!-- Avatar Upload -->
          <div class="flex flex-col items-center gap-4">
            <div class="avatar placeholder">
              <div class="w-24 rounded-full ring ring-primary ring-offset-base-100 ring-offset-2 overflow-hidden">
                <img v-if="avatarPreview" :src="avatarPreview" alt="Avatar" />
                <span v-else class="text-3xl">{{ profileForm.name?.[0] || 'U' }}</span>
              </div>
            </div>
            
            <input 
              id="avatar-upload"
              type="file" 
              accept="image/*" 
              class="hidden" 
              @change="handleFileChange"
            />
            
            <button 
              type="button" 
              @click="triggerFileInput"
              class="btn btn-sm btn-outline"
            >
              Change Avatar
            </button>
          </div>

          <!-- Name Input -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Display Name</span>
            </label>
            <input 
              v-model="profileForm.name"
              type="text" 
              class="input input-bordered" 
              placeholder="Your Name"
            />
          </div>

          <!-- Email (Read Only) -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Email</span>
            </label>
            <input 
              :value="authStore.user?.email"
              type="email" 
              class="input input-bordered" 
              disabled
            />
            <label class="label">
              <span class="label-text-alt text-base-content/60">
                Email cannot be changed here.
              </span>
            </label>
          </div>

          <!-- Submit Profile -->
          <div class="flex justify-end">
            <button 
              type="submit" 
              class="btn btn-primary"
              :disabled="profileLoading"
            >
              <span v-if="profileLoading" class="loading loading-spinner"></span>
              Save Profile
            </button>
          </div>
        </form>
      </BaseCard>

      <!-- Security Section -->
      <div class="space-y-6">
        
        <!-- Password Change -->
        <BaseCard title="Change Password">
          <form @submit.prevent="updatePassword" class="space-y-4">
            <div class="form-control">
              <label class="label">
                <span class="label-text">Current Password</span>
              </label>
              <input 
                v-model="passwordForm.oldPassword"
                type="password" 
                class="input input-bordered" 
                required
              />
            </div>

            <div class="form-control">
              <label class="label">
                <span class="label-text">New Password</span>
              </label>
              <input 
                v-model="passwordForm.password"
                type="password" 
                class="input input-bordered" 
                minlength="8"
                required
              />
            </div>

            <div class="form-control">
              <label class="label">
                <span class="label-text">Confirm New Password</span>
              </label>
              <input 
                v-model="passwordForm.passwordConfirm"
                type="password" 
                class="input input-bordered" 
                required
              />
            </div>

            <div class="flex justify-end mt-4">
              <button 
                type="submit" 
                class="btn btn-secondary"
                :disabled="passwordLoading"
              >
                <span v-if="passwordLoading" class="loading loading-spinner"></span>
                Update Password
              </button>
            </div>
          </form>
        </BaseCard>

        <!-- Preferences -->
        <BaseCard title="Preferences">
          <div class="form-control">
            <label class="label cursor-pointer">
              <span class="label-text">Dark Mode</span> 
              <input 
                type="checkbox" 
                class="toggle toggle-primary" 
                :checked="uiStore.theme === 'dark'"
                @change="uiStore.toggleTheme"
              />
            </label>
          </div>
        </BaseCard>
      </div>
    </div>
  </div>
</template>
