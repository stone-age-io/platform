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
        <h2 class="card-title text-3xl mb-4 justify-center">
          Stone Age 🪨
        </h2>
        <p class="text-center text-base-content/70 mb-6">
          Sign in to your account
        </p>
        
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
