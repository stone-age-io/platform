<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { format } from 'date-fns'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NatsUser, NatsAccount, NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import type { PickerOption } from '@/types/picker'

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: NatsUser): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const userId = route.params.id as string | undefined
const isEdit = computed(() => !!userId && !props.embedded)

// Form data
const formData = ref({
  // Identity
  nats_username: '',
  description: '',

  // Auth
  email: '',
  password: '',
  passwordConfirm: '',

  // Permissions
  account_id: '',
  role_id: '',

  // Per-user permission overrides
  publish_permissions: '',
  subscribe_permissions: '',
  publish_deny_permissions: '',
  subscribe_deny_permissions: '',

  // Options
  bearer_token: false,

  // Credential lifetime (optional; empty = never expires)
  jwt_expires_at: '',
})

/**
 * Grug helper: UI needs string for textarea, API wants Array.
 */
function formatForInput(val: any): string {
  if (!val) return ''
  if (Array.isArray(val)) return val.join(', ')
  return String(val)
}

/**
 * Grug helper: API wants Array, UI gives string.
 */
function formatForApi(val: string): string[] {
  if (!val) return []
  return val.split(',')
    .map(s => s.trim())
    .filter(s => s !== '')
}

/**
 * Grug helper: PocketBase date string -> <input type="datetime-local"> value.
 * PB stores UTC ("2026-12-31 23:59:59.000Z"); the input wants local "yyyy-MM-ddTHH:mm".
 */
function pbDateToInput(val?: string): string {
  if (!val) return ''
  const d = new Date(val.replace(' ', 'T'))
  return isNaN(d.getTime()) ? '' : format(d, "yyyy-MM-dd'T'HH:mm")
}

/**
 * The account is not a choice. An organization has exactly one, provisioned with
 * it -- `nats_accounts.createRule` is null, so nothing can create a second one
 * through the API -- and which organization you are working in is already
 * answered by the sidebar switcher, the same source `data.organization` uses on
 * create below.
 *
 * As a dropdown it implied a decision that does not exist, and for a platform
 * operator it was worse than useless: `nats_accounts.listRule`'s first branch is
 * an unqualified `is_operator = true`, so the list held every tenant's account
 * with nothing but a name to tell them apart. Filtering by the active org here
 * fixes the display for both; the server-side half of that is tracked separately.
 */
const orgAccount = ref<NatsAccount | null>(null)
const roles = ref<NatsRole[]>([])

const roleOptions = computed<PickerOption[]>(() =>
  roles.value.map(r => ({
    id: r.id,
    label: r.name,
    sublabel: r.is_default ? 'Default' : undefined,
  }))
)

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load form options (accounts and roles)
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [accountsResult, rolesResult] = await Promise.all([
      pb.collection('nats_accounts').getFullList<NatsAccount>({
        sort: 'name',
        filter: pb.filter('active = true && organization = {:org}', {
          org: authStore.currentOrgId,
        }),
      }),
      pb.collection('nats_roles').getFullList<NatsRole>({
        sort: 'name'
      }),
    ])

    orgAccount.value = accountsResult[0] || null
    roles.value = rolesResult

    // Auto-select defaults
    if (!isEdit.value) {
      const defaultRole = roles.value.find(r => r.is_default)
      if (defaultRole) formData.value.role_id = defaultRole.id
      if (orgAccount.value) formData.value.account_id = orgAccount.value.id
    }
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing user for editing
 */
async function loadUser() {
  if (!userId || props.embedded) return
  
  loading.value = true
  
  try {
    const user = await pb.collection('nats_users').getOne<NatsUser>(userId)
    
    formData.value = {
      nats_username: user.nats_username,
      description: user.description || '',
      email: user.email,
      password: '',
      passwordConfirm: '',
      account_id: user.account_id,
      role_id: user.role_id,
      publish_permissions: formatForInput(user.publish_permissions),
      subscribe_permissions: formatForInput(user.subscribe_permissions),
      publish_deny_permissions: formatForInput(user.publish_deny_permissions),
      subscribe_deny_permissions: formatForInput(user.subscribe_deny_permissions),
      bearer_token: user.bearer_token || false,
      jwt_expires_at: pbDateToInput(user.jwt_expires_at),
    }
  } catch (err: any) {
    toast.error('Failed to load NATS user')
    router.push('/nats/users')
  } finally {
    loading.value = false
  }
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new users')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  // Nothing in this form can fix a missing account, so say so here rather than
  // letting the create fail on a rule the operator cannot see.
  if (!formData.value.account_id) {
    toast.error('This organization has no NATS account yet')
    return
  }

  loading.value = true
  
  try {
    const data: any = {
      nats_username: formData.value.nats_username,
      description: formData.value.description || null,
      email: formData.value.email,
      emailVisibility: true,
      account_id: formData.value.account_id,
      role_id: formData.value.role_id,
      publish_permissions: formatForApi(formData.value.publish_permissions),
      subscribe_permissions: formatForApi(formData.value.subscribe_permissions),
      publish_deny_permissions: formatForApi(formData.value.publish_deny_permissions),
      subscribe_deny_permissions: formatForApi(formData.value.subscribe_deny_permissions),
      bearer_token: formData.value.bearer_token,
      // Empty string clears the date in PocketBase (never expires).
      jwt_expires_at: formData.value.jwt_expires_at
        ? new Date(formData.value.jwt_expires_at).toISOString()
        : '',
    }

    // Only send password if entered
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    let record: NatsUser

    if (isEdit.value) {
      record = await pb.collection('nats_users').update<NatsUser>(userId!, data)
      toast.success('NATS user updated')
    } else {
      data.organization = authStore.currentOrgId
      // Set once, on create. `active` is not editable here on purpose -- see the
      // note on the Security Settings card. Revoke/Re-enable own it.
      data.active = true
      record = await pb.collection('nats_users').create<NatsUser>(data)
      toast.success('NATS user created')
    }
    
    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/nats/users')
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to save NATS user')
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('cancel')
  } else {
    router.back()
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadUser()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header: Hidden if Embedded -->
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nats/users">NATS Users</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit NATS User' : 'Provision NATS User' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Identity">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">NATS Username *</span>
                </label>
                <input 
                  v-model="formData.nats_username"
                  type="text" 
                  placeholder="user.device.sensor1"
                  class="input input-bordered font-mono"
                  required
                  :disabled="isEdit"
                />
                <label class="label">
                  <span class="label-text-alt">
                    {{ isEdit ? 'Username cannot be changed after creation' : 'Use dot notation for hierarchical naming' }}
                  </span>
                </label>
              </div>
              
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Description</span>
                </label>
                <textarea 
                  v-model="formData.description"
                  class="textarea textarea-bordered"
                  rows="3"
                  placeholder="Optional description"
                ></textarea>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Authentication">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Email (Identity) *</span>
                </label>
                <input 
                  v-model="formData.email"
                  type="email" 
                  placeholder="device-uuid@nats.local"
                  class="input input-bordered"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Unique email used for PocketBase authentication record.</span>
                </label>
              </div>
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.password"
                    type="password" 
                    class="input input-bordered"
                    :required="!isEdit"
                    minlength="8"
                  />
                </div>
                
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Confirm Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.passwordConfirm"
                    type="password" 
                    class="input input-bordered"
                    :required="!!formData.password"
                  />
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Account & Permissions">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">NATS Account</span>
                </label>
                <div class="input input-bordered bg-base-200/50 flex items-center overflow-hidden">
                  <span v-if="orgAccount" class="truncate">{{ orgAccount.name }}</span>
                  <span
                    v-else-if="formData.account_id"
                    class="truncate font-mono text-xs text-base-content/60"
                  >{{ formData.account_id }}</span>
                  <span v-else class="text-sm text-base-content/50">
                    No NATS account provisioned for this organization
                  </span>
                </div>
                <label class="label">
                  <span class="label-text-alt text-base-content/60">
                    Every organization has one account. Switch organization to provision a
                    user in a different one.
                  </span>
                </label>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">Role *</span>
                </label>
                <RecordPicker
                  v-model="formData.role_id"
                  :options="roleOptions"
                  title="Role"
                  placeholder="Select a role"
                  empty-text="No roles defined for this organization yet."
                />
              </div>
            </div>
          </BaseCard>
          
          <BaseCard title="Security Settings">
            <div class="space-y-4">
              <!--
                There is deliberately no "Active" toggle here. `active` is a LABEL
                that pb-nats sets; it is not a control. Clearing it changes the
                badge and nothing else -- the signed user JWT stays valid and the
                device keeps connecting. Only revocation cuts NATS off, by adding
                the public key to the account's revocation list. Use Revoke /
                Re-enable on the user's detail page.
              -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input
                    v-model="formData.bearer_token"
                    type="checkbox"
                    class="checkbox"
                  />
                  <span class="label-text">
                    <span class="font-medium">Enable Bearer Token</span>
                    <span class="block text-sm text-base-content/70">
                      Generate a long-lived bearer token for simplified auth
                    </span>
                  </span>
                </label>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text font-medium">JWT Expiry</span>
                </label>
                <input
                  v-model="formData.jwt_expires_at"
                  type="datetime-local"
                  class="input input-bordered"
                />
                <label class="label">
                  <span class="label-text-alt">
                    Optional. Leave blank for a non-expiring credential. Applied to the JWT on save.
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Permission Overrides">
            <p class="text-xs text-base-content/60 mb-4">
              Optional. These merge with role permissions (union). Leave empty to inherit role only.
            </p>
            <div class="space-y-6">
              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📤</span>
                  <h4 class="text-xs font-black uppercase opacity-50 tracking-widest">Publishing</h4>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs">Allow Subjects</span></label>
                  <textarea v-model="formData.publish_permissions" class="textarea textarea-bordered font-mono text-xs" rows="2" placeholder="admin.reports.>"></textarea>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs text-error font-bold">Deny Subjects</span></label>
                  <textarea v-model="formData.publish_deny_permissions" class="textarea textarea-bordered font-mono text-xs border-error/30 focus:border-error" rows="2" placeholder="events.internal.>"></textarea>
                </div>
              </div>

              <div class="divider my-0 opacity-30"></div>

              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📥</span>
                  <h4 class="text-xs font-black uppercase opacity-50 tracking-widest">Subscribing</h4>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs">Allow Subjects</span></label>
                  <textarea v-model="formData.subscribe_permissions" class="textarea textarea-bordered font-mono text-xs" rows="2" placeholder="admin.reports.>"></textarea>
                </div>
                <div class="form-control">
                  <label class="label py-1"><span class="label-text text-xs text-error font-bold">Deny Subjects</span></label>
                  <textarea v-model="formData.subscribe_deny_permissions" class="textarea textarea-bordered font-mono text-xs border-error/30 focus:border-error" rows="2" placeholder="events.internal.>"></textarea>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="handleCancel" 
          class="btn btn-ghost order-2 sm:order-1"
          :disabled="loading"
        >
          Cancel
        </button>
        <button 
          type="submit" 
          class="btn btn-primary order-1 sm:order-2"
          :disabled="loading"
        >
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} User</span>
        </button>
      </div>
    </form>
  </div>
</template>
