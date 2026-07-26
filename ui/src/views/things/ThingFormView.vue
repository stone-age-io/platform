<!-- ui/src/views/things/ThingFormView.vue -->
<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Thing, ThingType, Location, NatsUser, NatsAccount, NatsRole, NebulaHost, NebulaNetwork } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import NatsUserFormView from '@/views/nats/NatsUserFormView.vue'
import NebulaHostFormView from '@/views/nebula/NebulaHostFormView.vue'
import LocationFormView from '@/views/locations/LocationFormView.vue'

// Define a local interface for the dropdown options
interface LocationOption extends Location {
  displayName: string
  disabled?: boolean
}

type ProvisionMode = 'auto' | 'link' | 'none'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const thingId = route.params.id as string | undefined
const isEdit = computed(() => !!thingId)

// Form data
const formData = ref({
  // Basic
  name: '',
  description: '',
  code: '',
  type: '',
  location: '',

  // Auth (edit mode only)
  email: '',
  password: '',
  passwordConfirm: '',

  // Infrastructure links (for link mode / edit mode)
  nats_user: '',
  nebula_host: '',

  // Meta
  metadata: '',
})

// Whether this caller may attach or mint NATS/Nebula identities. Members hold
// inventory rights but not identity rights (things.createRule freezes
// nats_user/nebula_host for them), and they cannot read nats_users or
// nebula_hosts at all — so for them the identity UI would offer options the API
// won't return and writes the API will refuse.
const canManageIdentities = computed(() => authStore.can.manageInfrastructure)

// Only owner/admin may set a Thing's password (things.manageRule).
const canSetThingPassword = computed(() => authStore.can.decommissionInventory)

// Provisioning modes (create mode only). The default follows the capability: a
// member auto-provisioning would fail on the nats_users create, which is what
// made Thing creation impossible for them in the UI even though the API allows it.
const natsMode = ref<ProvisionMode>(canManageIdentities.value ? 'auto' : 'none')
const nebulaMode = ref<ProvisionMode>('none')

// Auto-provision config
const autoNatsRoleId = ref('')
const autoNebulaNetworkId = ref('')
const autoNebulaOverlayIp = ref('')

// Description of the selected thing type, shown under the select. Falls back to
// the subject prefix so the hint is still useful for a type with no description.
const selectedTypeHint = computed(() => {
  const t = thingTypes.value.find(x => x.id === formData.value.type)
  if (!t) return ''
  if (t.description) return t.description
  return t.subject_prefix ? `Subject prefix: ${t.subject_prefix}` : ''
})

// Auto-slug tracking
const codeManuallyEdited = ref(false)

// Relation options
const thingTypes = ref<ThingType[]>([])
const locations = ref<LocationOption[]>([])
const natsUsers = ref<NatsUser[]>([])
const nebulaHosts = ref<NebulaHost[]>([])

// Auto-provision options
const natsRoles = ref<NatsRole[]>([])
const nebulaNetworks = ref<NebulaNetwork[]>([])
const orgNatsAccount = ref<NatsAccount | null>(null)

// State
const loading = ref(false)
const loadingOptions = ref(true)

// Success modal
const showSuccessModal = ref(false)
const successCredentials = ref({ email: '', password: '' })

// Quick Add modal state
const showNatsModal = ref(false)
const showNebulaModal = ref(false)
const showLocationModal = ref(false)

// Slugify helper (shared logic with auto-slug watcher)
function slugify(text: string): string {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
}

// Preview of the login the server will derive for the Thing. The route builds the
// authoritative value the same way (hooks/thing_routes.go, orgSlugFor) — this is
// display only, so the operator sees the shape before submitting.
const orgSlug = computed(() => slugify(authStore.currentOrg?.name || ''))
const thingEmail = computed(() => {
  if (!formData.value.code) return ''
  return `${formData.value.code}@${orgSlug.value}.thing.local`
})

// Auto-slug: name → code (only in create mode, until manually edited)
watch(() => formData.value.name, (newName) => {
  if (!codeManuallyEdited.value && !isEdit.value) {
    formData.value.code = slugify(newName)
  }
})

function onCodeInput() {
  codeManuallyEdited.value = true
}

/**
 * Helper: Sort locations into a hierarchy and return a flat list with indentation
 */
function sortLocationsHierarchically(items: Location[]): LocationOption[] {
  const result: LocationOption[] = []
  const childrenMap = new Map<string, Location[]>()
  const roots: Location[] = []

  items.forEach(item => {
    if (!item.parent) {
      roots.push(item)
    } else {
      const list = childrenMap.get(item.parent) || []
      list.push(item)
      childrenMap.set(item.parent, list)
    }
  })

  roots.sort((a, b) => (a.name || '').localeCompare(b.name || ''))

  function traverse(node: Location, depth: number) {
    const prefix = depth > 0 ? '\u00A0\u00A0\u00A0'.repeat(depth) + '\u2514 ' : ''
    result.push({
      ...node,
      displayName: prefix + (node.name || 'Unnamed'),
      disabled: false
    })
    const children = childrenMap.get(node.id) || []
    children.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
    children.forEach(child => traverse(child, depth + 1))
  }

  roots.forEach(root => traverse(root, 0))

  const processedIds = new Set(result.map(r => r.id))
  const orphans = items.filter(i => !processedIds.has(i.id))
  orphans.forEach(orphan => {
    result.push({
      ...orphan,
      displayName: `[Orphan] ${orphan.name || 'Unnamed'}`,
      disabled: false
    })
  })

  return result
}

/**
 * Load form options
 */
async function loadOptions() {
  loadingOptions.value = true

  try {
    // Inventory reference data every role can read.
    const [typesRes, locsRes] = await Promise.all([
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
    ])
    thingTypes.value = typesRes
    locations.value = sortLocationsHierarchically(locsRes)

    // Identity options are owner/admin-only reads. Fetching them as a member
    // returns their own personal NATS identity as the sole option (nats_users is
    // row-scoped to the caller) and empty Nebula lists, so don't ask.
    if (!canManageIdentities.value) return

    const [natsRes, nebulaRes, rolesRes, networksRes] = await Promise.all([
      pb.collection('nats_users').getFullList<NatsUser>({ sort: 'nats_username' }),
      pb.collection('nebula_hosts').getFullList<NebulaHost>({ sort: 'hostname' }),
      pb.collection('nats_roles').getFullList<NatsRole>({ sort: 'name' }),
      pb.collection('nebula_networks').getFullList<NebulaNetwork>({ sort: 'name', filter: 'active = true' }),
    ])

    natsUsers.value = natsRes
    nebulaHosts.value = nebulaRes
    natsRoles.value = rolesRes
    nebulaNetworks.value = networksRes

    // Auto-select defaults for roles
    const defaultRole = natsRoles.value.find(r => r.is_default)
    if (defaultRole) autoNatsRoleId.value = defaultRole.id
    if (nebulaNetworks.value.length === 1) autoNebulaNetworkId.value = nebulaNetworks.value[0].id

    // Fetch org's NATS account
    if (authStore.currentOrgId) {
      try {
        orgNatsAccount.value = await pb.collection('nats_accounts').getFirstListItem<NatsAccount>(
          `organization = "${authStore.currentOrgId}" && active = true`
        )
      } catch {
        // No NATS account for this org — auto-provision won't work
        orgNatsAccount.value = null
      }
    }
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing thing for editing
 */
async function loadThing() {
  if (!thingId) return

  loading.value = true

  try {
    const thing = await pb.collection('things').getOne<Thing>(thingId)

    formData.value = {
      name: thing.name || '',
      description: thing.description || '',
      code: thing.code || '',
      type: thing.type || '',
      location: thing.location || '',
      email: thing.email,
      password: '',
      passwordConfirm: '',
      nats_user: thing.nats_user || '',
      nebula_host: thing.nebula_host || '',
      metadata: thing.metadata ? JSON.stringify(thing.metadata, null, 2) : '',
    }
    // Don't auto-slug in edit mode
    codeManuallyEdited.value = true
  } catch (err: any) {
    toast.error('Failed to load thing')
    router.push('/things')
  } finally {
    loading.value = false
  }
}

/**
 * Validate metadata JSON
 */
function validateMetadata(): boolean {
  if (!formData.value.metadata.trim()) return true
  try {
    JSON.parse(formData.value.metadata)
    return true
  } catch {
    toast.error('Invalid JSON in metadata field')
    return false
  }
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  if (!validateMetadata()) return

  if (isEdit.value) {
    await handleUpdate()
  } else {
    await handleCreate()
  }
}

/**
 * Handle create.
 *
 * One call to POST /api/org/things (hooks/thing_routes.go), which creates the
 * Thing and mints any requested identities inside a single transaction. This used
 * to be three separate client calls with no rollback, so a failure on the last one
 * orphaned a signed NATS credential — and it never set `active`, so the Thing it
 * produced could not authenticate. The server also owns the account lookup, the
 * default role, the email shape, and the Thing's password.
 */
async function handleCreate() {
  if (!formData.value.code) {
    toast.error('Code is required')
    return
  }

  // Client-side checks for the auto paths, so the operator sees the problem
  // before a round trip. The route re-checks all of these.
  if (natsMode.value === 'auto') {
    if (!orgNatsAccount.value) {
      toast.error('No NATS account found for this organization. Cannot auto-provision.')
      return
    }
    if (!autoNatsRoleId.value) {
      toast.error('Please select a NATS role for auto-provisioning.')
      return
    }
  }

  if (nebulaMode.value === 'auto') {
    if (!autoNebulaNetworkId.value) {
      toast.error('Please select a Nebula network for auto-provisioning.')
      return
    }
    if (!autoNebulaOverlayIp.value) {
      toast.error('Please enter an overlay IP for auto-provisioning.')
      return
    }
  }

  loading.value = true

  try {
    const created = await pb.send('/api/org/things', {
      method: 'POST',
      body: {
        name: formData.value.name,
        description: formData.value.description || '',
        code: formData.value.code,
        type: formData.value.type || '',
        location: formData.value.location || '',
        metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
        nats: {
          mode: natsMode.value,
          user_id: natsMode.value === 'link' ? formData.value.nats_user : '',
          role_id: natsMode.value === 'auto' ? autoNatsRoleId.value : '',
        },
        nebula: {
          mode: nebulaMode.value,
          host_id: nebulaMode.value === 'link' ? formData.value.nebula_host : '',
          network_id: nebulaMode.value === 'auto' ? autoNebulaNetworkId.value : '',
          overlay_ip: nebulaMode.value === 'auto' ? autoNebulaOverlayIp.value : '',
        },
      },
    })

    // The route returns the generated password exactly once.
    successCredentials.value = {
      email: created.email,
      password: created.password,
    }
    showSuccessModal.value = true
  } catch (err: any) {
    toast.error(err.response?.message || err.message || 'Provisioning failed')
  } finally {
    loading.value = false
  }
}

/**
 * Handle update (edit mode)
 */
async function handleUpdate() {
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  loading.value = true

  try {
    const data: any = {
      name: formData.value.name,
      description: formData.value.description || null,
      code: formData.value.code || null,
      type: formData.value.type || null,
      location: formData.value.location || null,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    }

    // Send the identity relations only if this caller may change them. The member
    // branch of things.updateRule requires `nats_user:changed = false`, and a
    // field absent from the body counts as unchanged — sending it is what turns a
    // legitimate inventory edit into a 404.
    if (canManageIdentities.value) {
      data.nats_user = formData.value.nats_user || null
      data.nebula_host = formData.value.nebula_host || null
    }

    if (canSetThingPassword.value && formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }

    await pb.collection('things').update(thingId!, data)
    toast.success('Thing updated')
    router.push('/things')
  } catch (err: any) {
    toast.error(err.message || 'Failed to update thing')
  } finally {
    loading.value = false
  }
}

// Success modal actions
function copyCredentials() {
  const text = `Email: ${successCredentials.value.email}\nPassword: ${successCredentials.value.password}`
  navigator.clipboard.writeText(text)
  toast.success('Credentials copied to clipboard')
}

function closeSuccessModal() {
  showSuccessModal.value = false
  router.push('/things')
}

// Handlers for Quick Add Success (link mode)
function onNatsCreated(record: NatsUser) {
  natsUsers.value.push(record)
  natsUsers.value.sort((a, b) => a.nats_username.localeCompare(b.nats_username))
  formData.value.nats_user = record.id
  showNatsModal.value = false
}

function onNebulaCreated(record: NebulaHost) {
  nebulaHosts.value.push(record)
  nebulaHosts.value.sort((a, b) => a.hostname.localeCompare(b.hostname))
  formData.value.nebula_host = record.id
  showNebulaModal.value = false
}

async function onLocationCreated(record: Location) {
  await loadOptions()
  formData.value.location = record.id
  showLocationModal.value = false
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadThing()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things">Things</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Thing' : 'Provision Thing' }}
      </h1>
    </div>

    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">

        <!-- Left Column: Identity & Info -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Name *</span>
                </label>
                <input
                  v-model="formData.name"
                  type="text"
                  placeholder="e.g. Warehouse HVAC"
                  class="input input-bordered"
                  required
                />
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">Description</span>
                </label>
                <textarea
                  v-model="formData.description"
                  class="textarea textarea-bordered"
                  rows="2"
                  placeholder="Optional description"
                ></textarea>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Code *</span>
                  </label>
                  <input
                    v-model="formData.code"
                    @input="onCodeInput"
                    type="text"
                    placeholder="auto-generated-from-name"
                    class="input input-bordered font-mono"
                    required
                  />
                  <label class="label">
                    <span class="label-text-alt">
                      {{ !isEdit ? 'Auto-generated from name. Used as NATS username, Nebula hostname, and email prefix.' : 'Unique identifier' }}
                    </span>
                  </label>
                </div>

                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Type</span>
                  </label>
                  <select v-model="formData.type" class="select select-bordered">
                    <option value="">Select Type...</option>
                    <option v-for="t in thingTypes" :key="t.id" :value="t.id">
                      {{ t.name }}
                    </option>
                  </select>
                  <!-- Type names are often terse codes. Anyone who can pick a type
                       can also read its description, so show it rather than making
                       them go and look the definition up. -->
                  <label v-if="selectedTypeHint" class="label">
                    <span class="label-text-alt text-base-content/60">{{ selectedTypeHint }}</span>
                  </label>
                </div>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">Location</span>
                </label>
                <div class="flex gap-2">
                  <select v-model="formData.location" class="select select-bordered flex-1 min-w-0 font-mono text-sm">
                    <option value="">Select Location...</option>
                    <option
                      v-for="loc in locations"
                      :key="loc.id"
                      :value="loc.id"
                      :disabled="loc.disabled"
                    >
                      {{ loc.displayName }}
                    </option>
                  </select>
                  <button
                    type="button"
                    class="btn btn-square btn-outline"
                    @click="showLocationModal = true"
                    title="Quick Add Location"
                  >
                    +
                  </button>
                </div>
              </div>

              <!-- Email preview (create mode only) -->
              <div v-if="!isEdit && formData.code" class="bg-base-200 rounded-lg p-3">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Generated Identity</span>
                <span class="font-mono text-sm select-all break-all">{{ thingEmail }}</span>
              </div>
            </div>
          </BaseCard>

          <!-- Authentication: Edit mode only -->
          <!-- Setting a Thing password is owner/admin only (things.manageRule). -->
          <BaseCard v-if="isEdit && canSetThingPassword" title="Authentication">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Email</span>
                </label>
                <input
                  :value="formData.email"
                  type="email"
                  class="input input-bordered"
                  disabled
                />
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">New Password (Optional)</span>
                  </label>
                  <input
                    v-model="formData.password"
                    type="password"
                    class="input input-bordered"
                    minlength="8"
                  />
                </div>

                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Confirm Password</span>
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

        <!-- Right Column: Connectivity & Metadata -->
        <div class="space-y-6">

          <!-- NATS Connectivity -->
          <BaseCard v-if="canManageIdentities" title="NATS Connectivity">
            <!-- Mode tabs (create mode only) -->
            <div v-if="!isEdit" class="tabs tabs-boxed mb-4">
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': natsMode === 'auto' }" @click="natsMode = 'auto'">
                Auto-Provision
              </a>
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': natsMode === 'link' }" @click="natsMode = 'link'">
                Link Existing
              </a>
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': natsMode === 'none' }" @click="natsMode = 'none'">
                None
              </a>
            </div>

            <!-- Auto mode -->
            <div v-if="!isEdit && natsMode === 'auto'" class="space-y-4">
              <div v-if="!orgNatsAccount" class="alert alert-warning">
                <span>No NATS account found for this organization. Auto-provisioning unavailable.</span>
              </div>
              <template v-else>
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Role *</span>
                  </label>
                  <select v-model="autoNatsRoleId" class="select select-bordered" required>
                    <option value="">Select a role...</option>
                    <option v-for="role in natsRoles" :key="role.id" :value="role.id">
                      {{ role.name }}{{ role.is_default ? ' (Default)' : '' }}
                    </option>
                  </select>
                </div>
                <div v-if="formData.code" class="bg-base-200 rounded-lg p-3 space-y-1">
                  <span class="text-xs text-base-content/50 uppercase block">Will Create</span>
                  <div class="text-sm">
                    <span class="text-base-content/50">Username:</span>
                    <span class="font-mono ml-1">{{ formData.code }}</span>
                  </div>
                  <div class="text-sm">
                    <span class="text-base-content/50">Account:</span>
                    <span class="font-mono ml-1">{{ orgNatsAccount.name }}</span>
                  </div>
                </div>
              </template>
            </div>

            <!-- Link mode (create) or dropdown (edit) -->
            <div v-if="(isEdit) || (!isEdit && natsMode === 'link')" class="space-y-4">
              <div class="form-control">
                <label v-if="isEdit" class="label">
                  <span class="label-text">NATS User</span>
                </label>
                <div class="flex gap-2">
                  <select v-model="formData.nats_user" class="select select-bordered font-mono flex-1 min-w-0">
                    <option value="">None</option>
                    <option v-for="user in natsUsers" :key="user.id" :value="user.id">
                      {{ user.nats_username }}
                    </option>
                  </select>
                  <button
                    type="button"
                    class="btn btn-square btn-outline"
                    @click="showNatsModal = true"
                    title="Quick Add NATS User"
                  >
                    +
                  </button>
                </div>
                <label class="label">
                  <span class="label-text-alt">
                    Links this device to a specific NATS identity.
                  </span>
                </label>
              </div>
            </div>

            <!-- None mode -->
            <div v-if="!isEdit && natsMode === 'none'" class="text-sm text-base-content/50 py-2">
              No NATS connectivity. This Thing will be an asset/inventory record only.
            </div>
          </BaseCard>

          <!-- Nebula Connectivity -->
          <BaseCard v-if="canManageIdentities" title="Nebula Connectivity">
            <!-- Mode tabs (create mode only) -->
            <div v-if="!isEdit" class="tabs tabs-boxed mb-4">
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': nebulaMode === 'auto' }" @click="nebulaMode = 'auto'">
                Auto-Provision
              </a>
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': nebulaMode === 'link' }" @click="nebulaMode = 'link'">
                Link Existing
              </a>
              <a class="tab tab-sm sm:tab-md flex-1 min-w-0 truncate" :class="{ 'tab-active': nebulaMode === 'none' }" @click="nebulaMode = 'none'">
                None
              </a>
            </div>

            <!-- Auto mode -->
            <div v-if="!isEdit && nebulaMode === 'auto'" class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Network *</span>
                </label>
                <select v-model="autoNebulaNetworkId" class="select select-bordered" required>
                  <option value="">Select a network...</option>
                  <option v-for="net in nebulaNetworks" :key="net.id" :value="net.id">
                    {{ net.name }} ({{ net.cidr_range }})
                  </option>
                </select>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">Overlay IP *</span>
                </label>
                <input
                  v-model="autoNebulaOverlayIp"
                  type="text"
                  placeholder="e.g. 10.100.0.45"
                  class="input input-bordered font-mono"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Must be unique within the selected network.</span>
                </label>
              </div>

              <div v-if="formData.code" class="bg-base-200 rounded-lg p-3 space-y-1">
                <span class="text-xs text-base-content/50 uppercase block">Will Create</span>
                <div class="text-sm">
                  <span class="text-base-content/50">Hostname:</span>
                  <span class="font-mono ml-1">{{ formData.code }}</span>
                </div>
              </div>
            </div>

            <!-- Link mode (create) or dropdown (edit) -->
            <div v-if="(isEdit) || (!isEdit && nebulaMode === 'link')" class="space-y-4">
              <div class="form-control">
                <label v-if="isEdit" class="label">
                  <span class="label-text">Nebula Host</span>
                </label>
                <div class="flex gap-2">
                  <select v-model="formData.nebula_host" class="select select-bordered font-mono flex-1 min-w-0">
                    <option value="">None</option>
                    <option v-for="host in nebulaHosts" :key="host.id" :value="host.id">
                      {{ host.hostname }} ({{ host.overlay_ip }})
                    </option>
                  </select>
                  <button
                    type="button"
                    class="btn btn-square btn-outline"
                    @click="showNebulaModal = true"
                    title="Quick Add Nebula Host"
                  >
                    +
                  </button>
                </div>
                <label class="label">
                  <span class="label-text-alt">
                    Links this device to a Nebula VPN node.
                  </span>
                </label>
              </div>
            </div>

            <!-- None mode -->
            <div v-if="!isEdit && nebulaMode === 'none'" class="text-sm text-base-content/50 py-2">
              No Nebula VPN connectivity.
            </div>
          </BaseCard>

          <BaseCard title="Metadata (JSON)">
            <div class="form-control">
              <textarea
                v-model="formData.metadata"
                class="textarea textarea-bordered font-mono"
                rows="8"
                placeholder='{"key": "value"}'
              ></textarea>
            </div>
          </BaseCard>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button
          type="button"
          @click="router.back()"
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Thing</span>
        </button>
      </div>
    </form>

    <!-- Success Modal (create only) -->
    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showSuccessModal }">
        <div class="modal-box">
          <h3 class="font-bold text-lg text-success">Thing Provisioned Successfully</h3>
          <p class="py-4 text-sm text-base-content/70">
            Use these credentials to bootstrap the edge agent.
            <strong>Save this password now, it cannot be recovered later.</strong>
          </p>

          <div class="bg-base-200 p-4 rounded-lg space-y-3 font-mono text-sm">
            <div>
              <span class="text-base-content/50 text-xs uppercase block">Email</span>
              <span class="select-all">{{ successCredentials.email }}</span>
            </div>
            <div>
              <span class="text-base-content/50 text-xs uppercase block">Password</span>
              <span class="select-all text-primary font-bold">{{ successCredentials.password }}</span>
            </div>
          </div>

          <div class="modal-action">
            <button class="btn btn-ghost" @click="closeSuccessModal">Close</button>
            <button class="btn btn-primary" @click="copyCredentials">Copy & Close</button>
          </div>
        </div>
        <div class="modal-backdrop" @click="closeSuccessModal"></div>
      </dialog>
    </Teleport>

    <!-- Quick Add Modals (for Link mode) -->
    <dialog class="modal" :class="{ 'modal-open': showNatsModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add NATS User</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showNatsModal = false">&#x2715;</button>
        </div>
        <div v-if="showNatsModal">
          <NatsUserFormView :embedded="true" @success="onNatsCreated" @cancel="showNatsModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showNatsModal = false"><button>close</button></form>
    </dialog>

    <dialog class="modal" :class="{ 'modal-open': showNebulaModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add Nebula Host</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showNebulaModal = false">&#x2715;</button>
        </div>
        <div v-if="showNebulaModal">
          <NebulaHostFormView :embedded="true" @success="onNebulaCreated" @cancel="showNebulaModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showNebulaModal = false"><button>close</button></form>
    </dialog>

    <dialog class="modal" :class="{ 'modal-open': showLocationModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add Location</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showLocationModal = false">&#x2715;</button>
        </div>
        <div v-if="showLocationModal">
          <LocationFormView :embedded="true" @success="onLocationCreated" @cancel="showLocationModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showLocationModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
