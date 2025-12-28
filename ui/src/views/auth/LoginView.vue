<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

// View States
const mode = ref<'login' | 'forgot'>('login')
const loading = ref(false)
const showAdminToggle = ref(false) // Triggered by URL param

// Form Data
const email = ref('')
const password = ref('')
const isSuperAdmin = ref(false)

/**
 * Check for the subtle "admin" trigger in the URL
 */
onMounted(() => {
  if (route.query.admin === '1' || route.query.admin === 'true') {
    showAdminToggle.value = true
    isSuperAdmin.value = true
  }
})

/**
 * Authentication Handler
 */
async function handleLogin() {
  loading.value = true
  try {
    await authStore.login(email.value, password.value, isSuperAdmin.value)
    
    if (authStore.memberships.length === 0 && !authStore.isSuperAdmin) {
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

/**
 * Password Reset Handler
 */
async function handleResetRequest() {
  if (!email.value) {
    toast.error('Please enter your email address')
    return
  }
  
  loading.value = true
  try {
    await authStore.requestPasswordReset(email.value, isSuperAdmin.value)
    toast.success('Password reset email sent! Please check your inbox.')
    mode.value = 'login'
  } catch (err: any) {
    // We use error toast, but standard practice is to not reveal if email exists
    toast.error(err.message || 'Failed to request reset')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200 p-4">
    <div class="card w-full max-w-sm bg-base-100 shadow-xl">
      <div class="card-body">
        
        <!-- App Logo & Title -->
        <div class="flex flex-col items-center mb-6">
          <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 256 256" class="text-primary mb-2">
            <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" fill="currentColor"/>
          </svg>
          <h2 class="text-2xl font-bold tracking-tight">Stone-Age.io</h2>
          <p class="text-sm opacity-60">
            {{ mode === 'login' ? 'Sign in to your account' : 'Reset your password' }}
          </p>
        </div>

        <!-- LOGIN MODE -->
        <form v-if="mode === 'login'" @submit.prevent="handleLogin" class="space-y-4">
          <div class="form-control">
            <label class="label"><span class="label-text">Email</span></label>
            <input v-model="email" type="email" placeholder="name@company.com" class="input input-bordered" required />
          </div>
          
          <div class="form-control">
            <div class="flex justify-between items-center pr-1">
              <label class="label"><span class="label-text">Password</span></label>
              <button type="button" @click="mode = 'forgot'" class="text-xs link link-primary no-underline hover:underline">
                Forgot password?
              </button>
            </div>
            <input v-model="password" type="password" placeholder="••••••••" class="input input-bordered" required />
          </div>

          <!-- Subtle Admin Toggle (Hidden unless ?admin=1 is in URL) -->
          <div v-if="showAdminToggle" class="form-control bg-base-200 p-3 rounded-lg border border-base-300">
            <label class="label cursor-pointer justify-between">
              <span class="label-text font-bold text-xs uppercase tracking-widest opacity-70">Super User</span>
              <input type="checkbox" v-model="isSuperAdmin" class="toggle toggle-primary toggle-sm" />
            </label>
          </div>
          
          <div class="form-control mt-6">
            <button type="submit" class="btn btn-primary w-full" :disabled="loading">
              <span v-if="loading" class="loading loading-spinner"></span>
              <span v-else>{{ isSuperAdmin ? 'Super User Login' : 'Sign In' }}</span>
            </button>
          </div>
        </form>

        <!-- FORGOT PASSWORD MODE -->
        <form v-else @submit.prevent="handleResetRequest" class="space-y-4">
          <div class="form-control">
            <label class="label"><span class="label-text">Account Email</span></label>
            <input v-model="email" type="email" placeholder="name@company.com" class="input input-bordered" required />
            <label class="label">
              <span class="label-text-alt opacity-70">
                We'll send a password reset link if an account exists for this email.
              </span>
            </label>
          </div>

          <div class="form-control mt-6 space-y-2">
            <button type="submit" class="btn btn-primary w-full" :disabled="loading">
              <span v-if="loading" class="loading loading-spinner"></span>
              <span v-else>Send Reset Link</span>
            </button>
            <button type="button" @click="mode = 'login'" class="btn btn-ghost w-full btn-sm">
              Back to Sign In
            </button>
          </div>
        </form>

      </div>
    </div>
  </div>
</template>
