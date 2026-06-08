<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { generateRandomPassword } from '@/utils/password'
import type { LeafNode, Location, NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import NebulaHostFormView from '@/views/nebula/NebulaHostFormView.vue'

// Hard allowlist — must match the server-side rule grants + leaf-sync allowlist.
const SYNCABLE_COLLECTIONS = [
  'things',
  'locations',
  'thing_types',
  'location_types',
  'thing_type_operations',
  'message_schemas',
]

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const nodeId = route.params.id as string | undefined
const isEdit = computed(() => !!nodeId)

const formData = ref({
  name: '',
  description: '',
  code: '',
  domain: '',
  location: '',
  nebula_host: '',
  synced_collections: [] as string[],
  metadata: '',
})

const codeManuallyEdited = ref(false)
const domainManuallyEdited = ref(false)

const locations = ref<Location[]>([])
const nebulaHosts = ref<NebulaHost[]>([])
const loading = ref(false)
const loadingOptions = ref(true)

// Success modal (create only)
const showSuccessModal = ref(false)
const successCredentials = ref({ email: '', password: '' })

// Quick Add Nebula host modal
const showNebulaModal = ref(false)

function slugify(text: string): string {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
}

const orgSlug = computed(() => slugify(authStore.currentOrg?.name || ''))
const leafEmail = computed(() => {
  if (!formData.value.code) return ''
  return `${formData.value.code}@${orgSlug.value}.leaf.local`
})

// Auto-slug: name → code (create mode, until manually edited)
watch(() => formData.value.name, (newName) => {
  if (!codeManuallyEdited.value && !isEdit.value) {
    formData.value.code = slugify(newName)
  }
})

// Auto-domain: code → edge-<code> (until manually edited)
watch(() => formData.value.code, (newCode) => {
  if (!domainManuallyEdited.value) {
    formData.value.domain = newCode ? `edge-${newCode}` : ''
  }
})

function onCodeInput() {
  codeManuallyEdited.value = true
}
function onDomainInput() {
  domainManuallyEdited.value = true
}

async function loadOptions() {
  loadingOptions.value = true
  try {
    const [locs, hosts] = await Promise.all([
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
      pb.collection('nebula_hosts').getFullList<NebulaHost>({ sort: 'hostname' }),
    ])
    locations.value = locs
    nebulaHosts.value = hosts
  } catch {
    // Non-fatal — both location and nebula host are optional.
  } finally {
    loadingOptions.value = false
  }
}

async function loadNode() {
  if (!nodeId) return
  loading.value = true
  try {
    const node = await pb.collection('leaf_nodes').getOne<LeafNode>(nodeId)
    formData.value = {
      name: node.name || '',
      description: node.description || '',
      code: node.code || '',
      domain: node.domain || '',
      location: node.location || '',
      nebula_host: node.nebula_host || '',
      synced_collections: Array.isArray(node.synced_collections) ? [...node.synced_collections] : [],
      metadata: node.metadata ? JSON.stringify(node.metadata, null, 2) : '',
    }
    // Don't re-derive code/domain from name in edit mode.
    codeManuallyEdited.value = true
    domainManuallyEdited.value = true
  } catch (err: any) {
    toast.error('Failed to load leaf node')
    router.push('/leaf-nodes')
  } finally {
    loading.value = false
  }
}

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

function handleSubmit() {
  if (!validateMetadata()) return
  if (isEdit.value) {
    handleUpdate()
  } else {
    handleCreate()
  }
}

async function handleCreate() {
  if (!formData.value.code) {
    toast.error('Code is required')
    return
  }
  if (!authStore.currentOrgId) {
    toast.error('No active organization')
    return
  }

  loading.value = true
  try {
    // Uniqueness check on code within the org's leaf nodes.
    try {
      await pb.collection('leaf_nodes').getFirstListItem(`code = "${formData.value.code}"`)
      toast.error(`A leaf node with code "${formData.value.code}" already exists.`)
      loading.value = false
      return
    } catch (e: any) {
      if (e.status !== 404) throw e
    }

    const password = generateRandomPassword(24)
    await pb.collection('leaf_nodes').create({
      name: formData.value.name,
      description: formData.value.description || null,
      code: formData.value.code,
      domain: formData.value.domain || `edge-${formData.value.code}`,
      location: formData.value.location || null,
      nebula_host: formData.value.nebula_host || null,
      synced_collections: formData.value.synced_collections,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
      email: leafEmail.value,
      emailVisibility: true,
      password,
      passwordConfirm: password,
      organization: authStore.currentOrgId,
    })

    // The server-side hook provisions the NATS user; show the bootstrap creds.
    successCredentials.value = { email: leafEmail.value, password }
    showSuccessModal.value = true
  } catch (err: any) {
    toast.error(err.message || 'Failed to create leaf node')
  } finally {
    loading.value = false
  }
}

async function handleUpdate() {
  loading.value = true
  try {
    await pb.collection('leaf_nodes').update(nodeId!, {
      name: formData.value.name,
      description: formData.value.description || null,
      domain: formData.value.domain || null,
      location: formData.value.location || null,
      nebula_host: formData.value.nebula_host || null,
      synced_collections: formData.value.synced_collections,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    })
    toast.success('Leaf node updated')
    router.push(`/leaf-nodes/${nodeId}`)
  } catch (err: any) {
    toast.error(err.message || 'Failed to update leaf node')
  } finally {
    loading.value = false
  }
}

function copyCredentials() {
  const text = `PocketBase URL: ${window.location.origin}\nEmail: ${successCredentials.value.email}\nPassword: ${successCredentials.value.password}`
  navigator.clipboard.writeText(text)
  toast.success('Credentials copied to clipboard')
}

function closeSuccessModal() {
  showSuccessModal.value = false
  router.push('/leaf-nodes')
}

// Quick Add: select the freshly-created host so it's linked on save.
function onNebulaCreated(record: NebulaHost) {
  nebulaHosts.value.push(record)
  nebulaHosts.value.sort((a, b) => a.hostname.localeCompare(b.hostname))
  formData.value.nebula_host = record.id
  showNebulaModal.value = false
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) loadNode()
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/leaf-nodes">Leaf Nodes</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit Leaf Node' : 'Provision Leaf Node' }}</h1>
    </div>

    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <!-- Left: identity -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label"><span class="label-text">Name *</span></label>
                <input v-model="formData.name" type="text" placeholder="e.g. Warehouse Edge" class="input input-bordered" required />
              </div>

              <div class="form-control">
                <label class="label"><span class="label-text">Description</span></label>
                <textarea v-model="formData.description" class="textarea textarea-bordered" rows="2" placeholder="Optional description"></textarea>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label"><span class="label-text">Code *</span></label>
                  <input
                    v-model="formData.code"
                    @input="onCodeInput"
                    type="text"
                    placeholder="auto-generated-from-name"
                    class="input input-bordered font-mono"
                    :disabled="isEdit"
                    required
                  />
                  <label class="label">
                    <span class="label-text-alt">{{ isEdit ? 'Identity (immutable)' : 'Used as the NATS username + domain prefix' }}</span>
                  </label>
                </div>

                <div class="form-control">
                  <label class="label"><span class="label-text">JetStream Domain</span></label>
                  <input v-model="formData.domain" @input="onDomainInput" type="text" placeholder="edge-..." class="input input-bordered font-mono" />
                  <label class="label"><span class="label-text-alt">Local domain (distinct from the hub)</span></label>
                </div>
              </div>

              <div class="form-control">
                <label class="label"><span class="label-text">Location</span></label>
                <select v-model="formData.location" class="select select-bordered">
                  <option value="">None</option>
                  <option v-for="loc in locations" :key="loc.id" :value="loc.id">{{ loc.name }}</option>
                </select>
              </div>

              <div v-if="!isEdit && formData.code" class="bg-base-200 rounded-lg p-3">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Generated Identity</span>
                <span class="font-mono text-sm select-all break-all">{{ leafEmail }}</span>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right: synced collections + metadata -->
        <div class="space-y-6">
          <BaseCard title="Synced Collections">
            <p class="text-sm text-base-content/70 mb-3">
              Config collections this edge mirrors into local NATS KV.
            </p>
            <div class="space-y-2">
              <label v-for="col in SYNCABLE_COLLECTIONS" :key="col" class="flex items-center gap-3 cursor-pointer">
                <input type="checkbox" :value="col" v-model="formData.synced_collections" class="checkbox checkbox-sm" />
                <code class="text-sm">{{ col }}</code>
              </label>
            </div>
          </BaseCard>

          <BaseCard title="Nebula Connectivity">
            <div class="form-control">
              <label class="label"><span class="label-text">Nebula Host</span></label>
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
                <span class="label-text-alt">Optional — links this edge to a Nebula VPN node for overlay connectivity.</span>
              </label>
            </div>
          </BaseCard>

          <BaseCard title="Metadata (JSON)">
            <div class="form-control">
              <textarea v-model="formData.metadata" class="textarea textarea-bordered font-mono" rows="6" placeholder='{"key": "value"}'></textarea>
            </div>
          </BaseCard>
        </div>
      </div>

      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button type="button" @click="router.back()" class="btn btn-ghost order-2 sm:order-1" :disabled="loading">Cancel</button>
        <button type="submit" class="btn btn-primary order-1 sm:order-2" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Leaf Node</span>
        </button>
      </div>
    </form>

    <!-- Success Modal (create only) -->
    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showSuccessModal }">
        <div class="modal-box">
          <h3 class="font-bold text-lg text-success">Leaf Node Provisioned</h3>
          <p class="py-4 text-sm text-base-content/70">
            Put these into <code>leaf-sync.yaml</code> on the edge, then run
            <code>leaf-sync config</code>.
            <strong>Save this password now — it cannot be recovered later.</strong>
          </p>
          <div class="bg-base-200 p-4 rounded-lg space-y-3 font-mono text-sm">
            <div>
              <span class="text-base-content/50 text-xs uppercase block">Email</span>
              <span class="select-all break-all">{{ successCredentials.email }}</span>
            </div>
            <div>
              <span class="text-base-content/50 text-xs uppercase block">Password</span>
              <span class="select-all text-primary font-bold break-all">{{ successCredentials.password }}</span>
            </div>
          </div>
          <div class="modal-action">
            <button class="btn btn-ghost" @click="closeSuccessModal">Close</button>
            <button class="btn btn-primary" @click="copyCredentials">Copy &amp; Close</button>
          </div>
        </div>
        <div class="modal-backdrop" @click="closeSuccessModal"></div>
      </dialog>
    </Teleport>

    <!-- Quick Add Nebula Host -->
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
  </div>
</template>
