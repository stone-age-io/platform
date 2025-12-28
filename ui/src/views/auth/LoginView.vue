<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'

const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()

const email = ref('')
const password = ref('')
const isSuperAdmin = ref(false) // State for checkbox
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  
  try {
    // Pass isSuperAdmin to login action
    await authStore.login(email.value, password.value, isSuperAdmin.value)
    
    if (authStore.memberships.length === 0) {
      router.push('/accept-invite')
    } else {
      router.push('/')
    }
  } catch (err: any) {
    toast.error(err.message || 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200">
    <div class="card w-96 bg-base-100 shadow-xl">
      <div class="card-body">
        <!-- Logo Header -->
        <div class="flex flex-col items-center mb-4">
          <div class="flex items-center gap-4 mb-2">
            <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 256 256" class="text-primary">
              <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" fill="currentColor"/>
            </svg>
            <h2 class="text-3xl font-semibold">Stone-Age.io</h2>
          </div>
          <p class="text-center text-base-content/70">
            Sign in to your account
          </p>
        </div>
        
        <form @submit.prevent="handleLogin" class="space-y-4">
          <div class="form-control">
            <label class="label"><span class="label-text">Email</span></label>
            <input v-model="email" type="email" placeholder="email@example.com" class="input input-bordered" required />
          </div>
          
          <div class="form-control">
            <label class="label"><span class="label-text">Password</span></label>
            <input v-model="password" type="password" placeholder="••••••••" class="input input-bordered" required />
          </div>

          <!-- Super Admin Toggle -->
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input type="checkbox" v-model="isSuperAdmin" class="checkbox checkbox-primary checkbox-sm" />
              <span class="label-text font-medium">Log in as Super User</span>
            </label>
          </div>
          
          <div class="form-control mt-6">
            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span v-if="loading" class="loading loading-spinner"></span>
              <span v-else>{{ isSuperAdmin ? 'Super User Login' : 'Sign In' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
