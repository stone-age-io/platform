<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Thing, ThingType, Location, NatsUser, NatsAccount, NatsRole, NebulaHost, NebulaNetwork } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataEditor from '@/components/common/MetadataEditor.vue'
import ImageUploadField from '@/components/common/ImageUploadField.vue'
import StickyFormActions from '@/components/common/StickyFormActions.vue'
import NatsUserFormView from '@/views/nats/NatsUserFormView.vue'
import NebulaHostFormView from '@/views/nebula/NebulaHostFormView.vue'
import LocationFormView from '@/views/locations/LocationFormView.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import { flattenLocationTree, type LocationNode } from '@/utils/locations'
import type { PickerOption } from '@/types/picker'
import { useEscapeKey } from '@/composables/useEscapeKey'

type ProvisionMode = 'auto' | 'link' | 'none'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const thingId = route.params.id as string | undefined
const isEdit = computed(() => !!thingId)

const formData = ref({
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

  // An object (or null), not a JSON string: MetadataEditor owns the text and
  // never hands back anything unparseable.
  metadata: null as Record<string, any> | null,
})

const metadataEditor = ref<InstanceType<typeof MetadataEditor> | null>(null)

// Photo state, edit mode only. The record is kept whole because ImageUploadField
// resolves its own (protected, file-token) URL from it.
const loadedThing = ref<Thing | null>(null)
const photoFile = ref<File | null>(null)
const photoRemoved = ref(false)

// Null means free-form key/value rows. Changing the type never discards values;
// keys the new schema lacks are kept.
const selectedTypeMetadataSchema = computed(() => {
  const t = thingTypes.value.find(x => x.id === formData.value.type)
  return t?.metadata_schema || null
})

// Members cannot read nats_users/nebula_hosts, and things.createRule freezes
// nats_user/nebula_host for them, so the identity UI is owner/admin only.
const canManageIdentities = computed(() => authStore.can.manageInfrastructure)

// Only owner/admin may set a Thing's password (things.manageRule).
const canSetThingPassword = computed(() => authStore.can.decommissionInventory)

// Checks both capabilities: they are separate entries in the capability map.
const hasIdentityColumn = computed(
  () => canManageIdentities.value || (isEdit.value && canSetThingPassword.value),
)

// Create mode only. The default follows the capability: a member's auto-provision
// would fail on the nats_users create.
const natsMode = ref<ProvisionMode>(canManageIdentities.value ? 'auto' : 'none')
const nebulaMode = ref<ProvisionMode>('none')

const autoNatsRoleId = ref('')
const autoNebulaNetworkId = ref('')
const autoNebulaOverlayIp = ref('')

const selectedTypeHint = computed(() => {
  const t = thingTypes.value.find(x => x.id === formData.value.type)
  if (!t) return ''
  if (t.description) return t.description
  return t.subject_prefix ? `Subject prefix: ${t.subject_prefix}` : ''
})

// A type, once set, is frozen by things.updateRule (ADR 0003): the code prefix
// and the default subject are both derived from it.
const typeLocked = computed(() => isEdit.value && !!loadedThing.value?.type)

// What a generated code looks like for the selected type, for the help text.
const exampleCode = computed(() => {
  const prefix = thingTypes.value.find(t => t.id === formData.value.type)?.prefix
  return prefix ? `${prefix}-9KD-4PX` : '9KD-4PX'
})

const thingTypes = ref<ThingType[]>([])
const locations = ref<LocationNode[]>([])
const natsUsers = ref<NatsUser[]>([])
const nebulaHosts = ref<NebulaHost[]>([])

const natsRoles = ref<NatsRole[]>([])
const nebulaNetworks = ref<NebulaNetwork[]>([])
const orgNatsAccount = ref<NatsAccount | null>(null)

const typeOptions = computed<PickerOption[]>(() =>
  thingTypes.value.map(t => ({ id: t.id, label: t.name || 'Unnamed' }))
)

const locationOptions = computed<PickerOption[]>(() =>
  locations.value.map(n => ({
    id: n.id,
    label: n.orphan ? `${n.name} (orphaned)` : n.name,
    sublabel: n.path || undefined,
    depth: n.depth,
  }))
)

const natsUserOptions = computed<PickerOption[]>(() =>
  natsUsers.value.map(u => ({ id: u.id, label: u.nats_username }))
)

const natsRoleOptions = computed<PickerOption[]>(() =>
  natsRoles.value.map(r => ({
    id: r.id,
    label: r.name,
    sublabel: r.is_default ? 'Default' : undefined,
  }))
)

const nebulaHostOptions = computed<PickerOption[]>(() =>
  nebulaHosts.value.map(h => ({ id: h.id, label: h.hostname, sublabel: h.overlay_ip }))
)

const nebulaNetworkOptions = computed<PickerOption[]>(() =>
  nebulaNetworks.value.map(n => ({ id: n.id, label: n.name, sublabel: n.cidr_range }))
)

const loading = ref(false)
const loadingOptions = ref(true)

const showSuccessModal = ref(false)
const successCredentials = ref({ email: '', password: '' })

const showNatsModal = ref(false)
const showNebulaModal = ref(false)
const showLocationModal = ref(false)

// Display only. The server derives the real login from the org code
// (hooks/thing_routes.go, orgSlugFor).
const thingEmail = computed(() => {
  if (!formData.value.code) return ''
  return `${formData.value.code}@${authStore.currentOrg?.code || ''}.thing.local`
})

// No client-side code generator, deliberately. The server generates a blank code
// under the type's prefix (hooks/codes.go); a code is frozen and printed, so a
// second implementation that drifted would be wrong permanently.

async function loadOptions() {
  loadingOptions.value = true

  try {
    // Inventory reference data every role can read.
    const [typesRes, locsRes] = await Promise.all([
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
    ])
    thingTypes.value = typesRes
    locations.value = flattenLocationTree(locsRes)

    // Owner/admin-only reads. A member would get only their own NATS identity
    // and empty Nebula lists.
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

    const defaultRole = natsRoles.value.find(r => r.is_default)
    if (defaultRole) autoNatsRoleId.value = defaultRole.id
    if (nebulaNetworks.value.length === 1) autoNebulaNetworkId.value = nebulaNetworks.value[0].id

    if (authStore.currentOrgId) {
      try {
        orgNatsAccount.value = await pb.collection('nats_accounts').getFirstListItem<NatsAccount>(
          `organization = "${authStore.currentOrgId}" && active = true`
        )
      } catch {
        // No NATS account: auto-provision is unavailable
        orgNatsAccount.value = null
      }
    }
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

async function loadThing() {
  if (!thingId) return

  loading.value = true

  try {
    const thing = await pb.collection('things').getOne<Thing>(thingId)
    loadedThing.value = thing

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
      metadata: thing.metadata && Object.keys(thing.metadata).length ? thing.metadata : null,
    }
  } catch (err: any) {
    toast.error('Failed to load thing')
    router.push('/things')
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  // Commit a JSON tab left mid-edit. Returns false (and toasts) if it doesn't
  // parse, so a bad blob blocks the save instead of being dropped from it.
  if (metadataEditor.value && !metadataEditor.value.commit()) return

  if (isEdit.value) {
    await handleUpdate()
  } else {
    await handleCreate()
  }
}

/**
 * One call to POST /api/org/things (hooks/thing_routes.go): the Thing and any
 * requested identities in a single transaction. Keep it one call; separate client
 * calls have no rollback. The server owns the account lookup, default role, email
 * shape and the Thing's password.
 */
async function handleCreate() {
  // Early feedback only; the route re-checks all of these.
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
        metadata: formData.value.metadata,
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

    // The route returns the generated password exactly once, and only to an
    // owner or admin: a member never learns a Thing's password.
    if (!created.password) {
      toast.success('Thing created. An owner or admin sets its password before the device can sign in.')
      router.push('/things')
      return
    }
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
      metadata: formData.value.metadata,
    }

    // Only send the identity relations if this caller may change them. The member
    // branch of things.updateRule requires `nats_user:changed = false`, and only
    // an ABSENT field counts as unchanged; sending it 404s a member's edit.
    if (canManageIdentities.value) {
      data.nats_user = formData.value.nats_user || null
      data.nebula_host = formData.value.nebula_host || null
    }

    if (canSetThingPassword.value && formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }

    await pb.collection('things').update(thingId!, data)

    // The photo is a separate request and must stay one. The body above relies on
    // null-vs-absent; FormData can neither omit a field nor send null (an empty
    // string clears it), so a FormData body would 404 a member's edit. Sent
    // second, so a failure costs the photo rather than the edit.
    if (photoFile.value || photoRemoved.value) {
      const photoData = new FormData()
      photoData.append('photo', photoFile.value ?? '')
      try {
        await pb.collection('things').update(thingId!, photoData)
      } catch (err: any) {
        toast.warning('Saved, but the photo could not be uploaded')
        console.warn('photo upload failed', err)
      }
    }

    toast.success('Thing updated')
    router.push('/things')
  } catch (err: any) {
    toast.error(err.message || 'Failed to update thing')
  } finally {
    loading.value = false
  }
}

function copyCredentials() {
  const text = `Email: ${successCredentials.value.email}\nPassword: ${successCredentials.value.password}`
  navigator.clipboard.writeText(text)
  toast.success('Credentials copied to clipboard')
}

function closeSuccessModal() {
  showSuccessModal.value = false
  router.push('/things')
}

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

// See useEscapeKey. showSuccessModal is deliberately absent: it shows the
// one-time device password.
useEscapeKey(showNatsModal, () => { showNatsModal.value = false })
useEscapeKey(showNebulaModal, () => { showNebulaModal.value = false })
useEscapeKey(showLocationModal, () => { showLocationModal.value = false })
</script>

<template>
  <div class="space-y-6">
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

    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <form v-else @submit.prevent="handleSubmit" class="space-y-6">

      <!-- Laid out like the detail view: what the Thing is on the left, how it
           connects on the right. On a phone the inventory comes first. -->
      <div
        class="grid grid-cols-1 gap-6 items-start"
        :class="hasIdentityColumn ? 'lg:grid-cols-2' : 'max-w-3xl'"
      >

        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <!--
                Photo beside Name and Description only, as on the detail view;
                the rows below need the card's full width.
                Edit only: create goes through the JSON transaction route
                POST /api/org/things, which cannot carry a multipart upload
                (feature 19 in ui/CLAUDE.md).
              -->
              <div class="flex flex-col sm:flex-row sm:items-start gap-5">
                <div v-if="isEdit" class="shrink-0 sm:order-last">
                  <ImageUploadField
                    v-model:file="photoFile"
                    v-model:removed="photoRemoved"
                    :source="
                      loadedThing?.photo
                        ? { record: loadedThing, filename: loadedThing.photo, thumb: '400x400' }
                        : null
                    "
                    :size="140"
                    add-label="Add photo"
                    empty-label="No photo"
                  />
                </div>

                <div class="flex-1 min-w-0 space-y-4">
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
                </div>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Code</span>
                  </label>
                  <input
                    v-model="formData.code"
                    type="text"
                    :placeholder="isEdit ? '' : 'Blank to generate'"
                    class="input input-bordered font-mono"
                    :readonly="isEdit"
                  />
                  <label class="label">
                    <span class="label-text-alt">
                      {{ !isEdit
                        ? `Blank gets a generated code like ${exampleCode}. Used as NATS username, Nebula hostname and email prefix, and cannot be changed later.`
                        : 'Frozen: it is printed on labels and built into subjects.' }}
                    </span>
                  </label>
                </div>

                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Type</span>
                  </label>
                  <RecordPicker
                    v-model="formData.type"
                    :options="typeOptions"
                    title="Type"
                    placeholder="Select Type..."
                    :clearable="!typeLocked"
                    :disabled="typeLocked"
                    clear-label="No type"
                    empty-text="No thing types defined yet."
                  />
                  <label v-if="typeLocked" class="label">
                    <span class="label-text-alt text-base-content/60">
                      A type cannot be changed once set. To fix a wrong one, delete and recreate the Thing.
                    </span>
                  </label>
                  <!-- Anyone who can pick a type can read its description. -->
                  <label v-if="selectedTypeHint" class="label">
                    <span class="label-text-alt text-base-content/60">{{ selectedTypeHint }}</span>
                  </label>
                </div>
              </div>

              <div class="form-control">
                <label class="label">
                  <span class="label-text">Location</span>
                </label>
                <RecordPicker
                  v-model="formData.location"
                  :options="locationOptions"
                  title="Location"
                  placeholder="Select Location..."
                  clearable
                  clear-label="No location"
                  empty-text="No locations yet."
                >
                  <template #footer="{ close }">
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost w-full justify-start"
                      @click="close(); showLocationModal = true"
                    >
                      + New Location
                    </button>
                  </template>
                </RecordPicker>
              </div>

              <div v-if="!isEdit && formData.code" class="bg-base-200 rounded-lg p-3">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Generated Identity</span>
                <span class="font-mono text-sm select-all break-all">{{ thingEmail }}</span>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Metadata">
            <MetadataEditor
              ref="metadataEditor"
              v-model="formData.metadata"
              :schema="selectedTypeMetadataSchema"
            />
          </BaseCard>
        </div>

        <!-- Identities: every card is owner/admin only, so a member gets one column. -->
        <div v-if="hasIdentityColumn" class="space-y-6">

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

          <BaseCard v-if="canManageIdentities" title="NATS Connectivity">
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

            <div v-if="!isEdit && natsMode === 'auto'" class="space-y-4">
              <div v-if="!orgNatsAccount" class="alert alert-warning">
                <span>No NATS account found for this organization. Auto-provisioning unavailable.</span>
              </div>
              <template v-else>
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Role *</span>
                  </label>
                  <RecordPicker
                    v-model="autoNatsRoleId"
                    :options="natsRoleOptions"
                    title="Role"
                    placeholder="Select a role..."
                    empty-text="No NATS roles defined for this organization yet."
                  />
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

            <div v-if="(isEdit) || (!isEdit && natsMode === 'link')" class="space-y-4">
              <div class="form-control">
                <label v-if="isEdit" class="label">
                  <span class="label-text">NATS User</span>
                </label>
                <RecordPicker
                  v-model="formData.nats_user"
                  :options="natsUserOptions"
                  title="NATS user"
                  placeholder="None"
                  clearable
                  empty-text="No NATS users in this organization yet."
                >
                  <template #footer="{ close }">
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost w-full justify-start"
                      @click="close(); showNatsModal = true"
                    >
                      + New NATS User
                    </button>
                  </template>
                </RecordPicker>
                <label class="label">
                  <span class="label-text-alt">
                    Links this device to a specific NATS identity.
                  </span>
                </label>
              </div>
            </div>

            <div v-if="!isEdit && natsMode === 'none'" class="text-sm text-base-content/50 py-2">
              No NATS connectivity. This Thing will be an asset/inventory record only.
            </div>
          </BaseCard>

          <BaseCard v-if="canManageIdentities" title="Nebula Connectivity">
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

            <div v-if="!isEdit && nebulaMode === 'auto'" class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Network *</span>
                </label>
                <RecordPicker
                  v-model="autoNebulaNetworkId"
                  :options="nebulaNetworkOptions"
                  title="Network"
                  placeholder="Select a network..."
                  empty-text="No Nebula networks defined yet."
                />
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

            <div v-if="(isEdit) || (!isEdit && nebulaMode === 'link')" class="space-y-4">
              <div class="form-control">
                <label v-if="isEdit" class="label">
                  <span class="label-text">Nebula Host</span>
                </label>
                <RecordPicker
                  v-model="formData.nebula_host"
                  :options="nebulaHostOptions"
                  title="Nebula host"
                  placeholder="None"
                  clearable
                  empty-text="No Nebula hosts in this organization yet."
                >
                  <template #footer="{ close }">
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost w-full justify-start"
                      @click="close(); showNebulaModal = true"
                    >
                      + New Nebula Host
                    </button>
                  </template>
                </RecordPicker>
                <label class="label">
                  <span class="label-text-alt">
                    Links this device to a Nebula VPN node.
                  </span>
                </label>
              </div>
            </div>

            <div v-if="!isEdit && nebulaMode === 'none'" class="text-sm text-base-content/50 py-2">
              No Nebula VPN connectivity.
            </div>
          </BaseCard>
        </div>
      </div>

      <StickyFormActions>
        <button
          type="button"
          @click="router.back()"
          class="btn btn-ghost flex-1 sm:flex-none"
          :disabled="loading"
        >
          Cancel
        </button>
        <button
          type="submit"
          class="btn btn-primary flex-1 sm:flex-none"
          :disabled="loading"
        >
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Thing</span>
        </button>
      </StickyFormActions>
    </form>

    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showSuccessModal }">
        <div class="modal-box">
          <h3 class="font-bold text-lg text-success">Thing Provisioned Successfully</h3>
          <p class="py-4 text-sm text-base-content/70">
            The device signs in with these. An edge agent uses them to fetch its
            leaf configuration.
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
        <!-- No click-to-close: this is the only time the password is shown. -->
        <div class="modal-backdrop"></div>
      </dialog>
    </Teleport>

    <dialog class="modal" :class="{ 'modal-open': showNatsModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add NATS User</h3>
          <button class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="showNatsModal = false">&#x2715;</button>
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
          <button class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="showNebulaModal = false">&#x2715;</button>
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
          <button class="btn btn-sm btn-circle btn-ghost" aria-label="Close" @click="showLocationModal = false">&#x2715;</button>
        </div>
        <div v-if="showLocationModal">
          <LocationFormView :embedded="true" @success="onLocationCreated" @cancel="showLocationModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showLocationModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
