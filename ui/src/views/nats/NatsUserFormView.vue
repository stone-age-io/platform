<script setup lang="ts">
import { ArrowDownToLine, ArrowUpFromLine } from '@lucide/vue'
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

const formData = ref({
  nats_username: '',
  description: '',

  email: '',
  password: '',
  passwordConfirm: '',

  account_id: '',
  role_id: '',

  // Per-user overrides, comma-separated in the textareas
  publish_permissions: '',
  subscribe_permissions: '',
  publish_deny_permissions: '',
  subscribe_deny_permissions: '',

  bearer_token: false,

  // Empty = never expires
  jwt_expires_at: '',
})

function formatForInput(val: any): string {
  if (!val) return ''
  if (Array.isArray(val)) return val.join(', ')
  return String(val)
}

function formatForApi(val: string): string[] {
  if (!val) return []
  return val.split(',')
    .map(s => s.trim())
    .filter(s => s !== '')
}

// PB stores UTC ("2026-12-31 23:59:59.000Z"); datetime-local wants local "yyyy-MM-ddTHH:mm".
function pbDateToInput(val?: string): string {
  if (!val) return ''
  const d = new Date(val.replace(' ', 'T'))
  return isNaN(d.getTime()) ? '' : format(d, "yyyy-MM-dd'T'HH:mm")
}

/**
 * Not a choice: an organization has exactly one account (`nats_accounts.createRule`
 * is null) and the active org decides which. Filtered by org because an
 * operator's listRule branch returns every tenant's account.
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

const loading = ref(false)
const loadingOptions = ref(true)

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

async function handleSubmit() {
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new users')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  // Say so here rather than fail on a rule the user cannot see.
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
      // Set once, on create; see the Security Settings note.
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
              <!-- No "Active" toggle here. `active` false is a suspend: pb-nats revokes
                   the key and issues nothing new. Reactivate is on the detail page. -->
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
                  <ArrowUpFromLine :size="14" class="inline-block shrink-0" />
                  <h4 class="text-xs font-black uppercase text-base-content/50 tracking-widest">Publishing</h4>
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
                  <ArrowDownToLine :size="14" class="inline-block shrink-0" />
                  <h4 class="text-xs font-black uppercase text-base-content/50 tracking-widest">Subscribing</h4>
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
