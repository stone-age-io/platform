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
const loading = ref(false)

/**
 * Handle login form submission
 */
async function handleLogin() {
  loading.value = true
  
  try {
    await authStore.login(email.value, password.value)
    
    // Check if user has organizations
    if (authStore.memberships.length === 0) {
      // No organizations - send to accept invite screen
      router.push('/accept-invite')
    } else {
      // Has organizations - send to dashboard
      router.push('/')
    }
  } catch (err: any) {
    // Show error toast
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
        <!-- Logo/Title -->
        <div class="flex flex-col items-center mb-4">
          <div class="flex items-center gap-4 mb-2">
            <svg 
              xmlns="http://www.w3.org/2000/svg" 
              width="40" 
              height="40" 
              viewBox="0 0 256 256" 
              aria-label="Stone-Age.io Logo"
            >
              <path 
                d="M 128,0 C 57.343,0 0,57.343 0,128 C 0,198.657 57.343,256 128,256 C 198.657,256 256,198.657 256,128 C 256,57.343 198.657,0 128,0 z M 128,28 C 181.423,28 224.757,71.334 224.757,124.757 C 224.757,139.486 221.04,153.32 214.356,165.42 C 198.756,148.231 178.567,138.124 162.876,124.331 C 155.723,124.214 128.543,124.043 113.254,124.043 C 113.254,147.334 113.254,172.064 113.254,190.513 C 100.456,179.347 94.543,156.243 94.543,156.243 C 83.432,147.065 31.243,124.757 31.243,124.757 C 31.243,71.334 74.577,28 128,28 z" 
                fill="currentColor"
              />
            </svg>
            <h2 class="text-3xl font-semibold">Stone-Age.io</h2>
          </div>
          <p class="text-center text-base-content/70">
            Sign in to your account
          </p>
        </div>
        
        <!-- Login Form -->
        <form @submit.prevent="handleLogin" class="space-y-4">
          <!-- Email Input -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Email</span>
            </label>
            <input 
              v-model="email"
              type="email" 
              placeholder="email@example.com" 
              class="input input-bordered" 
              required
              autocomplete="email"
            />
          </div>
          
          <!-- Password Input -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Password</span>
            </label>
            <input 
              v-model="password"
              type="password" 
              placeholder="••••••••" 
              class="input input-bordered" 
              required
              autocomplete="current-password"
            />
          </div>
          
          <!-- Submit Button -->
          <div class="form-control mt-6">
            <button 
              type="submit" 
              class="btn btn-primary"
              :disabled="loading"
            >
              <span v-if="loading" class="loading loading-spinner"></span>
              <span v-else>Sign In</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
