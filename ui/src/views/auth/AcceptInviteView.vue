<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

// URL Params
const token = ref('')
const emailParam = ref('')

// UI State
const loading = ref(false)
const mode = ref<'register' | 'login'>('register')

// Form Data
const form = ref({
  email: '',
  password: '',
  passwordConfirm: '',
  name: ''
})

onMounted(() => {
  // 1. Extract Token & Email from URL
  const t = route.query.token as string
  if (t) token.value = t
  
  const e = route.query.email as string
  if (e) {
    emailParam.value = e
    form.value.email = e
  }

  // 2. Decide Default Mode
  // If user is already logged in, we don't need forms, just the Action Button (handled in template)
  // If not logged in, 'register' is usually the best default for invites
})

async function executeAccept() {
  // Call the custom tenancy endpoint
  await pb.send('/api/tenancy/accept-invite', {
    method: 'POST',
    body: { token: token.value },
  })

  // Refresh context to see new org
  await authStore.loadMemberships()
  
  // Switch to the new org (usually the last one added)
  if (authStore.memberships.length > 0) {
    const newMembership = authStore.memberships[authStore.memberships.length - 1]
    await authStore.switchOrganization(newMembership.organization)
  }
}

async function handleAction() {
  if (!token.value) {
    toast.error('Missing invitation token')
    return
  }

  loading.value = true
  
  try {
    // SCENARIO 1: User is already authenticated
    if (authStore.isAuthenticated) {
      await executeAccept()
      toast.success('Invitation accepted!')
      router.push('/')
      return
    }

    // SCENARIO 2: New User Registration
    if (mode.value === 'register') {
      if (form.value.password !== form.value.passwordConfirm) {
        throw new Error('Passwords do not match')
      }

      // 1. Create User
      await pb.collection('users').create({
        email: form.value.email,
        password: form.value.password,
        passwordConfirm: form.value.passwordConfirm,
        name: form.value.name,
        emailVisibility: true, // Useful for team visibility
      })

      // 2. Auto Login
      await authStore.login(form.value.email, form.value.password)
      
      // 3. Accept Invite
      await executeAccept()
      
      toast.success('Account created and joined!')
      router.push('/')
    } 
    
    // SCENARIO 3: Existing User Login
    else {
      // 1. Login
      await authStore.login(form.value.email, form.value.password)
      
      // 2. Accept Invite
      await executeAccept()
      
      toast.success('Welcome back! Invitation accepted.')
      router.push('/')
    }

  } catch (err: any) {
    console.error(err)
    toast.error(err.data?.message || err.message || 'Action failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-base-200 p-4">
    <div class="card w-full max-w-md bg-base-100 shadow-xl border border-base-300">
      <div class="card-body">
        
        <!-- Header -->
        <div class="text-center mb-6">
          <h2 class="text-2xl font-bold">You've been invited!</h2>
          <p class="text-sm opacity-60 mt-1">
            Accept the invitation to join the organization.
          </p>
        </div>

        <!-- Token Input (Hidden if present in URL, visible if manual entry needed) -->
        <div v-if="!route.query.token" class="form-control mb-4">
          <label class="label"><span class="label-text">Invitation Token</span></label>
          <input v-model="token" type="text" class="input input-bordered font-mono" placeholder="Paste token here..." />
        </div>

        <!-- STATE 1: ALREADY LOGGED IN -->
        <div v-if="authStore.isAuthenticated" class="text-center">
          <div class="alert alert-info shadow-sm mb-6 text-left text-sm">
            <span class="mr-2">👤</span>
            <div>
              Logged in as <strong>{{ authStore.user?.email }}</strong>
            </div>
          </div>
          <button 
            @click="handleAction" 
            class="btn btn-primary w-full" 
            :disabled="loading || !token"
          >
            <span v-if="loading" class="loading loading-spinner"></span>
            Join Organization
          </button>
          <div class="divider text-xs opacity-50">OR</div>
          <button @click="authStore.logout()" class="btn btn-ghost btn-sm">
            Log out to use a different account
          </button>
        </div>

        <!-- STATE 2: NOT LOGGED IN (Auth Forms) -->
        <div v-else>
          <!-- Tabs -->
          <div role="tablist" class="tabs tabs-boxed mb-6">
            <a role="tab" class="tab" :class="{ 'tab-active': mode === 'register' }" @click="mode = 'register'">Create Account</a>
            <a role="tab" class="tab" :class="{ 'tab-active': mode === 'login' }" @click="mode = 'login'">Log In</a>
          </div>

          <form @submit.prevent="handleAction" class="space-y-4">
            
            <div class="form-control">
              <label class="label"><span class="label-text">Email</span></label>
              <input 
                v-model="form.email" 
                type="email" 
                class="input input-bordered" 
                required 
                :readonly="!!emailParam && mode === 'register'" 
              />
            </div>

            <div v-if="mode === 'register'" class="form-control">
              <label class="label"><span class="label-text">Full Name</span></label>
              <input v-model="form.name" type="text" class="input input-bordered" required placeholder="John Doe" />
            </div>

            <div class="form-control">
              <label class="label"><span class="label-text">Password</span></label>
              <input v-model="form.password" type="password" class="input input-bordered" required minlength="8" />
            </div>

            <div v-if="mode === 'register'" class="form-control">
              <label class="label"><span class="label-text">Confirm Password</span></label>
              <input v-model="form.passwordConfirm" type="password" class="input input-bordered" required />
            </div>

            <div class="form-control mt-6">
              <button type="submit" class="btn btn-primary w-full" :disabled="loading || !token">
                <span v-if="loading" class="loading loading-spinner"></span>
                {{ mode === 'register' ? 'Register & Accept' : 'Log In & Accept' }}
              </button>
            </div>
          </form>
        </div>

      </div>
    </div>
  </div>
</template>
