<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { User } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id)
const loading = ref(false)

// Form state
const form = ref({
  name: '',
  description: '',
  active: true,
  owner: '',
})

// Owner selection state
const ownerMode = ref<'existing' | 'new'>('existing')
const users = ref<User[]>([])
const loadingUsers = ref(false)

// New user form
const newUserForm = ref({
  email: '',
  name: '',
  password: '',
  generatePassword: true,
})
const generatedPassword = ref('')
const showGeneratedPassword = ref(false)

async function loadUsers() {
  loadingUsers.value = true
  try {
    users.value = await pb.collection('users').getFullList({ sort: 'email' })
    // Default owner to current user for new orgs
    if (!isEdit.value && authStore.user && !form.value.owner) {
      form.value.owner = authStore.user.id
    }
  } catch (err) {
    console.error('Failed to load users:', err)
  } finally {
    loadingUsers.value = false
  }
}

async function loadData() {
  if (!id) return
  loading.value = true
  try {
    const record = await pb.collection('organizations').getOne(id)
    form.value = {
      name: record.name,
      description: record.description || '',
      active: record.active,
      owner: record.owner,
    }
  } catch (err: any) {
    toast.error('Failed to load organization')
    router.push('/organizations')
  } finally {
    loading.value = false
  }
}

function generateRandomPassword(length = 16) {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*'
  return Array.from({ length }, () => chars[Math.floor(Math.random() * chars.length)]).join('')
}

async function createNewUser(): Promise<string | null> {
  const password = newUserForm.value.generatePassword
    ? generateRandomPassword()
    : newUserForm.value.password

  try {
    const user = await pb.collection('users').create({
      email: newUserForm.value.email,
      name: newUserForm.value.name,
      password: password,
      passwordConfirm: password,
      emailVisibility: true,
    })

    if (newUserForm.value.generatePassword) {
      generatedPassword.value = password
    }

    toast.success(`User "${newUserForm.value.email}" created`)
    return user.id
  } catch (err: any) {
    toast.error(`Failed to create user: ${err.message}`)
    return null
  }
}

async function submit() {
  loading.value = true
  try {
    let ownerId = form.value.owner

    // Create new user if needed
    if (ownerMode.value === 'new') {
      if (!newUserForm.value.email.trim()) {
        toast.error('Email is required for new user')
        loading.value = false
        return
      }
      if (!newUserForm.value.generatePassword && !newUserForm.value.password) {
        toast.error('Password is required')
        loading.value = false
        return
      }

      const newUserId = await createNewUser()
      if (!newUserId) {
        loading.value = false
        return
      }
      ownerId = newUserId
    }

    if (!ownerId) {
      toast.error('Please select or create an owner')
      loading.value = false
      return
    }

    const data = {
      name: form.value.name,
      description: form.value.description,
      active: form.value.active,
      owner: ownerId,
    }

    if (isEdit.value) {
      await pb.collection('organizations').update(id!, data)
      toast.success('Organization updated')
      router.push('/organizations')
    } else {
      await pb.collection('organizations').create(data)
      toast.success('Organization created')

      // Show generated password modal if applicable
      if (ownerMode.value === 'new' && generatedPassword.value) {
        showGeneratedPassword.value = true
      } else {
        router.push('/organizations')
      }
    }
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

function copyPassword() {
  navigator.clipboard.writeText(generatedPassword.value)
  toast.success('Password copied to clipboard')
}

function closePasswordModal() {
  showGeneratedPassword.value = false
  generatedPassword.value = ''
  router.push('/organizations')
}

onMounted(() => {
  loadUsers()
  if (isEdit.value) loadData()
})
</script>

<template>
  <div class="max-w-2xl mx-auto space-y-6">
    <div class="breadcrumbs text-sm">
      <ul>
        <li><router-link to="/organizations">Organizations</router-link></li>
        <li>{{ isEdit ? 'Edit' : 'New' }}</li>
      </ul>
    </div>

    <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Organization</h1>

    <form @submit.prevent="submit">
      <BaseCard>
        <div class="space-y-4">
          <!-- Name -->
          <div class="form-control">
            <label class="label"><span class="label-text">Name *</span></label>
            <input v-model="form.name" type="text" class="input input-bordered" required :disabled="loading" />
          </div>

          <!-- Description -->
          <div class="form-control">
            <label class="label"><span class="label-text">Description</span></label>
            <textarea v-model="form.description" class="textarea textarea-bordered" rows="3" :disabled="loading"></textarea>
          </div>

          <!-- Owner Selection -->
          <div class="form-control">
            <label class="label"><span class="label-text">Owner *</span></label>

            <!-- Mode Toggle -->
            <div class="tabs tabs-boxed mb-3 w-fit">
              <a
                class="tab"
                :class="{ 'tab-active': ownerMode === 'existing' }"
                @click="ownerMode = 'existing'"
              >
                Select Existing User
              </a>
              <a
                class="tab"
                :class="{ 'tab-active': ownerMode === 'new' }"
                @click="ownerMode = 'new'"
              >
                Create New User
              </a>
            </div>

            <!-- Existing User Dropdown -->
            <div v-if="ownerMode === 'existing'">
              <select
                v-model="form.owner"
                class="select select-bordered w-full"
                required
                :disabled="loadingUsers || loading"
              >
                <option value="" disabled>Select owner...</option>
                <option v-for="u in users" :key="u.id" :value="u.id">
                  {{ u.email }}{{ u.name ? ` (${u.name})` : '' }}
                </option>
              </select>
              <label class="label">
                <span class="label-text-alt">The owner has full control over this organization</span>
              </label>
            </div>

            <!-- New User Form -->
            <div v-else class="space-y-3 p-4 bg-base-200 rounded-lg">
              <div class="form-control">
                <label class="label py-1"><span class="label-text">Email *</span></label>
                <input
                  v-model="newUserForm.email"
                  type="email"
                  class="input input-bordered input-sm"
                  placeholder="user@example.com"
                  :disabled="loading"
                />
              </div>

              <div class="form-control">
                <label class="label py-1"><span class="label-text">Name</span></label>
                <input
                  v-model="newUserForm.name"
                  type="text"
                  class="input input-bordered input-sm"
                  placeholder="John Doe"
                  :disabled="loading"
                />
              </div>

              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-3 py-1">
                  <input
                    v-model="newUserForm.generatePassword"
                    type="checkbox"
                    class="checkbox checkbox-sm"
                    :disabled="loading"
                  />
                  <span class="label-text">Auto-generate password</span>
                </label>
              </div>

              <div v-if="!newUserForm.generatePassword" class="form-control">
                <label class="label py-1"><span class="label-text">Password *</span></label>
                <input
                  v-model="newUserForm.password"
                  type="password"
                  class="input input-bordered input-sm"
                  placeholder="Minimum 8 characters"
                  :disabled="loading"
                />
              </div>

              <p class="text-xs opacity-70">
                The user can use "Forgot Password" to set their own password later.
              </p>
            </div>
          </div>

          <!-- Active Toggle -->
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input v-model="form.active" type="checkbox" class="toggle toggle-success" :disabled="loading" />
              <span class="label-text">Active</span>
            </label>
          </div>
        </div>

        <div class="flex justify-end gap-2 mt-6">
          <button type="button" class="btn btn-ghost" @click="router.back()" :disabled="loading">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner"></span>
            Save
          </button>
        </div>
      </BaseCard>
    </form>

    <!-- Generated Password Modal -->
    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showGeneratedPassword }">
        <div class="modal-box">
          <h3 class="font-bold text-lg mb-4">Organization Created</h3>

          <div class="alert alert-info mb-4">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <span>Save this password now. It won't be shown again.</span>
          </div>

          <div class="form-control">
            <label class="label"><span class="label-text">Owner's Password</span></label>
            <div class="join w-full">
              <input
                type="text"
                :value="generatedPassword"
                readonly
                class="input input-bordered join-item flex-1 font-mono"
              />
              <button type="button" class="btn btn-primary join-item" @click="copyPassword">
                Copy
              </button>
            </div>
          </div>

          <p class="text-sm opacity-70 mt-4">
            Share this password securely with the user, or let them use the "Forgot Password" feature to set their own.
          </p>

          <div class="modal-action">
            <button class="btn" @click="closePasswordModal">Done</button>
          </div>
        </div>
        <div class="modal-backdrop" @click="closePasswordModal"></div>
      </dialog>
    </Teleport>
  </div>
</template>
