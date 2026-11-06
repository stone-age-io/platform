<script setup lang="ts">
import { Ban, DownloadIcon, Lock, MapPin, Network, RadioTower, RefreshCw, Shield } from '@lucide/vue'
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useNatsStore } from '@/stores/nats'
import { useAuthStore } from '@/stores/auth'
import type { Thing, ThingType, ThingTypeOperation, NatsUser, NebulaHost, Location } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import KvDashboard from '@/components/nats/KvDashboard.vue'
import { TWIN_BUCKET, TWIN_DESIRED_BUCKET } from '@/utils/twin'
import MetadataCard from '@/components/common/MetadataCard.vue'
import ExpiryBadge from '@/components/common/ExpiryBadge.vue'
import RecordTimestamps from '@/components/common/RecordTimestamps.vue'
import RecordPhoto from '@/components/common/RecordPhoto.vue'
import PhotoQuickAdd from '@/components/common/PhotoQuickAdd.vue'
import DangerZone from '@/components/common/DangerZone.vue'
import QrLabelModal from '@/components/common/QrLabelModal.vue'
import OperationSubjects from '@/components/things/OperationSubjects.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()
const natsStore = useNatsStore()
const authStore = useAuthStore()

const thing = ref<Thing | null>(null)
const loading = ref(true)
const regenerating = ref(false)
const revoking = ref(false)
const showLabelModal = ref(false)
const deleting = ref(false)
const togglingActive = ref(false)

const thingId = route.params.id as string

// From the expanded type relation; no extra request.
const typeMetadataSchema = computed(
  () => (thing.value?.expand?.type as ThingType | undefined)?.metadata_schema || null,
)

// A metadata edit is an inventory write, which members hold. Delete and
// deactivate stay on decommissionInventory.
const canEditMetadata = computed(() => authStore.can.manageInventory)

// MetadataCard has already saved; patch locally instead of refetching the expands.
function onMetadataSaved(metadata: Record<string, any> | null) {
  if (thing.value) thing.value.metadata = metadata || undefined
}

// Twin keys are `thing.<code>.…` in the org-wide bucket; the location is not
// part of the key, so a code is all a Thing needs.
const thingCode = computed(() => thing.value?.code)
const hasTwinConfig = computed(() => !!thingCode.value)

// The type's operations resolved against this Thing; readable by every role.
// Resolved as the Publisher widget does: codes, never names, and the id where a
// Thing has no code.
const thingType = computed(() => thing.value?.expand?.type as ThingType | undefined)
const typeOperations = computed(
  () => (thingType.value?.expand?.operations as ThingTypeOperation[] | undefined) || [],
)
const subjectContext = computed(() => ({
  org: authStore.currentOrg?.code || '',
  location: (thing.value?.expand?.location as Location | undefined)?.code || '',
  thing: thing.value?.code || thing.value?.id || '',
  thingTypeCode: thingType.value?.code || '',
}))

async function loadThing() {
  loading.value = true
  try {
    thing.value = await pb.collection('things').getOne<Thing>(thingId, {
      expand: 'type.operations,location,nats_user.role_id,nebula_host',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load thing')
    router.push('/things')
  } finally {
    loading.value = false
  }
}

function downloadFile(filename: string, content: string, contentType: string) {
  if (!content) {
    toast.error('No content to download')
    return
  }
  const blob = new Blob([content], { type: contentType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function downloadNatsCreds() {
  const natsUser = thing.value?.expand?.nats_user as NatsUser
  if (!natsUser?.creds_file) {
    toast.error('No credentials file available')
    return
  }
  downloadFile(`${natsUser.nats_username}.creds`, natsUser.creds_file, 'text/plain')
  toast.success('Credentials downloaded')
}

/**
 * Whether the credential controls are offered. Regenerate and revoke both mint a
 * working credential, even on an inactive identity, so on a deactivated Thing
 * they would bypass Reactivate, the one lever that also restores sign-in and the
 * Nebula host (hooks/active_flag.go). No Re-enable either: for a device
 * `nats_users.active` mirrors `things.active`.
 */
const credentialsLive = computed(() => {
  const natsUser = thing.value?.expand?.nats_user as NatsUser | undefined
  return thing.value?.active !== false && !!natsUser?.active
})

// Not a leak response: it re-signs the same key, so old copies keep working.
// It renews a credential before its expiry.
async function regenerateCreds() {
  const natsUser = thing.value?.expand?.nats_user as NatsUser
  if (!natsUser) return

  const confirmed = await confirm({
    title: 'Regenerate Credentials',
    message: `Issue a fresh .creds file for "${natsUser.nats_username}"?`,
    details: 'This does not invalidate the current file: every copy keeps working, because the key does not change. If the credentials leaked, use Revoke instead.',
    confirmText: 'Regenerate',
    variant: 'warning',
  })
  if (!confirmed) return

  regenerating.value = true
  try {
    await pb.collection('nats_users').update(natsUser.id, { regenerate: true })
    toast.success('Credentials regenerated')
    await loadThing()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

// The leak response, not a suspend: new key pair, old key revoked, replacement
// issued on the same record. Taking a device out of service is Deactivate.
async function revokeCreds() {
  const natsUser = thing.value?.expand?.nats_user as NatsUser
  if (!natsUser) return

  const confirmed = await confirm({
    title: 'Revoke Credentials',
    message: `Reject every copy of the current .creds file for "${natsUser.nats_username}"?`,
    details: 'A new key and a new .creds file are issued in the same step, and the device stays active. It is offline until the new file is installed on it. To take the device out of service instead, use Deactivate.',
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return

  revoking.value = true
  try {
    await pb.collection('nats_users').update(natsUser.id, { revoke: true })
    toast.success('Credentials revoked and replaced')
    await loadThing()
  } catch (err: any) {
    toast.error(err.message || 'Failed to revoke credentials')
  } finally {
    revoking.value = false
  }
}

function downloadNebulaConfig() {
  const host = thing.value?.expand?.nebula_host as NebulaHost
  if (!host?.config_yaml) {
    toast.error('No configuration available')
    return
  }
  downloadFile(`${host.hostname}.yaml`, host.config_yaml, 'text/yaml')
  toast.success('Config downloaded')
}

/**
 * A real decommission (hooks/active_flag.go): deactivating invalidates the
 * device's auth tokens and revokes its NATS identity. Reactivating issues a NEW
 * credential; the old .creds stays dead (the revocation cutoff is permanent), so
 * the dialog says so.
 */
async function toggleActive() {
  if (!thing.value) return
  const deactivating = thing.value.active !== false

  const confirmed = await confirm({
    title: deactivating ? 'Deactivate Thing' : 'Reactivate Thing',
    message: deactivating
      ? `Take "${thing.value.name}" out of service?`
      : `Return "${thing.value.name}" to service?`,
    details: deactivating
      ? 'The device is signed out immediately and its NATS credentials are revoked. It stops publishing.'
      : 'A new NATS credential is issued. The previous .creds file stays revoked and must be re-downloaded onto the device.',
    confirmText: deactivating ? 'Deactivate' : 'Reactivate',
    variant: deactivating ? 'danger' : 'info'
  })
  if (!confirmed) return

  togglingActive.value = true
  try {
    await pb.collection('things').update(thing.value.id, { active: deactivating ? false : true })
    toast.success(deactivating ? 'Thing deactivated' : 'Thing reactivated')
    await loadThing()
  } catch (err: any) {
    toast.error(err.message || 'Failed to change status')
  } finally {
    togglingActive.value = false
  }
}

async function handleDelete() {
  if (!thing.value) return
  const confirmed = await confirm({
    title: 'Delete Thing',
    message: `Are you sure you want to delete "${thing.value.name}"?`,
    details: 'This action cannot be undone.',
    confirmText: 'Delete',
    variant: 'danger',
    // Code is optional, so fall back to the name rather than lose the gate.
    requireText: thing.value.code || thing.value.name || undefined,
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('things').delete(thing.value.id)
    toast.success('Thing deleted')
    router.push('/things')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete thing')
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadThing()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="thing">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/things">Things</router-link></li>
            <li class="truncate max-w-[200px]">{{ thing.name || 'Unnamed' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ thing.name || 'Unnamed Thing' }}</h1>
            <span v-if="thing.active === false" class="badge badge-error badge-outline gap-1">
              Deactivated
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <!-- No code, no label: the QR payload IS the code (ADR 0002). -->
            <button
              v-if="thingCode"
              @click="showLabelModal = true"
              class="btn btn-outline flex-1 sm:flex-initial"
            >
              Label
            </button>
            <router-link
              v-if="canEditMetadata"
              :to="`/things/${thing.id}/edit`"
              class="btn btn-primary flex-1 sm:flex-initial"
            >
              Edit
            </router-link>
            <button
              v-if="authStore.can.decommissionInventory"
              @click="toggleActive"
              class="btn flex-1 sm:flex-initial"
              :class="thing.active === false ? 'btn-outline btn-success' : 'btn-outline btn-warning'"
              :disabled="togglingActive"
            >
              {{ thing.active === false ? 'Reactivate' : 'Deactivate' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Spells out that a deactivated Thing is cut off, not merely flagged. -->
      <div v-if="thing.active === false" class="alert alert-warning py-2 text-sm">
        <span>
          This thing is deactivated — it cannot sign in, and its NATS credentials are revoked.
          Reactivating issues a new <code>.creds</code> file that must be deployed to the device.
        </span>
      </div>
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <!--
              The photo is a column beside the fields, stacked above them on a
              phone (DOM order, then sm:order-last). Wrapped in a div because
              RecordPhoto has a fragment root (plate plus teleported dialog), so
              classes do not fall through. With no photo the fields take the
              full width, unless the reader can add one (PhotoQuickAdd).
            -->
            <div class="flex flex-col sm:flex-row sm:items-start gap-5">
              <div v-if="thing.photo" class="shrink-0 sm:order-last">
                <RecordPhoto
                  :record="thing"
                  :filename="thing.photo"
                  :alt="`Photo of ${thing.name || 'this thing'}`"
                />
              </div>
              <!-- The empty slot, only for someone who can fill it. -->
              <div v-else-if="canEditMetadata" class="shrink-0 sm:order-last">
                <PhotoQuickAdd
                  collection="things"
                  :record-id="thing.id"
                  hint="What it looks like where it is installed."
                  @saved="(f) => { if (thing) thing.photo = f }"
                />
              </div>

              <dl class="space-y-4 flex-1 min-w-0">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Description</dt>
                  <dd class="mt-1 text-sm">{{ thing.description || '-' }}</dd>
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div>
                    <dt class="text-sm font-medium text-base-content/70">Type</dt>
                    <dd class="mt-1">
                      <span v-if="thing.expand?.type" class="badge badge-neutral">{{ thing.expand.type.name }}</span>
                      <span v-else class="text-sm text-base-content/40">—</span>
                    </dd>
                  </div>
                  <div>
                    <dt class="text-sm font-medium text-base-content/70">Code</dt>
                    <dd class="mt-1">
                      <code v-if="thing.code" class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ thing.code }}</code>
                      <span v-else class="text-sm text-base-content/40">—</span>
                    </dd>
                  </div>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Location</dt>
                  <dd class="mt-1">
                    <router-link v-if="thing.expand?.location" :to="`/locations/${thing.location}`" class="link link-primary hover:no-underline flex items-center gap-1">
                      <MapPin :size="14" class="inline-block shrink-0" /> {{ thing.expand.location.name }}
                    </router-link>
                    <span v-else class="text-sm text-base-content/40">No location assigned</span>
                  </dd>
                </div>
              </dl>
            </div>

            <!--
              Timestamps span the full card below the photo. Measured: beside a
              140px photo at a 1440px viewport, each column was 108px and a full
              timestamp (about 125px) wrapped to three lines. A second <dl>
              because RecordTimestamps emits dt/dd pairs into a <dl> parent.
            -->
            <dl class="grid grid-cols-2 gap-4 mt-4">
              <!-- Updated too, so the record can be correlated with the activity feed. -->
              <RecordTimestamps :created="thing.created" :updated="thing.updated" />
            </dl>
          </BaseCard>

          <!-- Shown empty to a writer: that is how the first field gets added. -->
          <MetadataCard
            v-if="canEditMetadata || (thing.metadata && Object.keys(thing.metadata).length > 0)"
            collection="things"
            :record-id="thing.id"
            :model-value="thing.metadata || null"
            :schema="typeMetadataSchema"
            :can-edit="canEditMetadata"
            @saved="onMetadataSaved"
          />
        </div>
        
        <div class="space-y-6">
          <BaseCard title="Connectivity">
            <!-- Each control sits in the section that names what it acts on. -->
            <!-- NATS. flex-wrap rather than a breakpoint: the buttons drop to their
                 own line only when they do not fit, where flex-col sm:flex-row
                 would cost every phone 64px. -->
            <div class="flex flex-wrap items-center justify-between gap-2 mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">NATS</span>
              <!-- No controls for an unreadable identity, nor (credentialsLive) a cut-off device. -->
              <div v-if="thing.expand?.nats_user && credentialsLive" class="connectivity-actions credential-actions flex gap-2 w-full sm:w-auto">
                <button @click="downloadNatsCreds" class="btn btn-sm btn-outline" title="Download .creds file">
                  <DownloadIcon :size="16" class="inline-block shrink-0" />
                  <span>.creds</span>
                </button>
                <!-- Plain outline: regenerate invalidates nothing. Revoke is the
                     one that breaks a deployed device. -->
                <button
                  @click="regenerateCreds"
                  class="btn btn-sm btn-outline"
                  title="Issue a fresh .creds file for the same key"
                  :disabled="regenerating || revoking"
                >
                  <RefreshCw :size="16" class="inline-block shrink-0" />
                  <span>Regenerate</span>
                </button>
                <button
                  @click="revokeCreds"
                  class="btn btn-sm btn-outline btn-error"
                  title="Reject the current credentials and issue replacements"
                  :disabled="regenerating || revoking"
                >
                  <Ban :size="16" class="inline-block shrink-0" />
                  <span>Revoke</span>
                </button>
              </div>
            </div>
            <div v-if="thing.expand?.nats_user" class="flex flex-col gap-3">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <div class="flex justify-between items-start mb-1 gap-2">
                  <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Username</span>
                  <div class="flex items-center gap-2">
                    <ExpiryBadge :value="thing.expand.nats_user.jwt_expires_at" size="sm" />
                    <div class="flex items-center gap-1.5" v-if="thing.expand.nats_user.active">
                      <span class="w-2 h-2 rounded-full bg-success"></span>
                      <span class="text-xs font-medium text-base-content/70">Active</span>
                    </div>
                    <div class="flex items-center gap-1.5" v-else>
                      <span class="w-2 h-2 rounded-full bg-error"></span>
                      <span class="text-xs font-medium text-base-content/70">Inactive</span>
                    </div>
                  </div>
                </div>
                <div class="font-mono text-base break-all select-all">{{ thing.expand.nats_user.nats_username }}</div>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider block mb-1">Role</span>
                <!--
                  Link only for readers the /nats/roles/:id route admits
                  (manageInfrastructure). No raw role_id fallback: nats_roles is
                  owner/admin-only, so the id is useless to anyone else.
                -->
                <router-link
                  v-if="thing.expand.nats_user.expand?.role_id && authStore.can.manageInfrastructure"
                  :to="`/nats/roles/${thing.expand.nats_user.role_id}`"
                  class="link link-primary text-sm font-mono"
                >
                  <Shield :size="14" class="inline-block shrink-0" /> {{ thing.expand.nats_user.expand.role_id.name }}
                </router-link>
                <span v-else-if="thing.expand.nats_user.expand?.role_id" class="font-mono text-sm">
                  <Shield :size="14" class="inline-block shrink-0" /> {{ thing.expand.nats_user.expand.role_id.name }}
                </span>
                <span v-else class="font-mono text-sm text-base-content/60">—</span>
              </div>
              <!-- Says where the controls went, including an identity inactive
                   on its own (out of step with the Thing). -->
              <p v-if="!credentialsLive" class="text-xs text-base-content/60">
                <template v-if="thing.active === false">
                  Credential actions return when the Thing is reactivated.
                </template>
                <template v-else>
                  This NATS identity is inactive, so its credentials are rejected.
                  <router-link
                    v-if="authStore.can.manageInfrastructure"
                    :to="`/nats/users/${thing.nats_user}`"
                    class="link link-primary"
                  >Open the NATS user</router-link>
                </template>
              </p>
            </div>
            <!-- Linked, but the caller cannot read nats_users (members see only
                 their own row). The relation id on the Thing is still readable. -->
            <div v-else-if="thing.nats_user" class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <Lock :size="28" :stroke-width="1.5" class="mx-auto text-base-content/30 mb-2" />
              <p class="text-sm">NATS identity linked</p>
              <p class="text-xs text-base-content/70 mt-1">Details require an administrator</p>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <RadioTower :size="28" :stroke-width="1.5" class="mx-auto text-base-content/30 mb-2" />
              <p class="text-sm">No NATS user linked</p>
              <!-- Same gate as the edit form's identity cards. -->
              <router-link
                v-if="authStore.can.manageInfrastructure"
                :to="`/things/${thing.id}/edit`"
                class="btn btn-sm btn-outline mt-3"
              >Attach a NATS user</router-link>
            </div>

            <div class="border-t border-base-300 my-4"></div>

            <!-- Nebula. Same wrap as the NATS heading. -->
            <div class="flex flex-wrap items-center justify-between gap-2 mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Nebula</span>
              <div v-if="thing.expand?.nebula_host" class="connectivity-actions flex gap-2">
                <button @click="downloadNebulaConfig" class="btn btn-sm btn-outline" title="Download Nebula config">
                  <DownloadIcon :size="16" class="inline-block shrink-0" />
                  <span>Config</span>
                </button>
              </div>
            </div>
            <div v-if="thing.expand?.nebula_host" class="flex flex-col gap-4">
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                  <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Hostname</span>
                  <div class="font-mono text-sm break-all">{{ thing.expand.nebula_host.hostname }}</div>
                </div>
                <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                  <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Overlay IP</span>
                  <div class="font-mono text-sm">{{ thing.expand.nebula_host.overlay_ip }}</div>
                </div>
              </div>
            </div>
            <!-- Same three states as NATS above. -->
            <div v-else-if="thing.nebula_host" class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <Lock :size="28" :stroke-width="1.5" class="mx-auto text-base-content/30 mb-2" />
              <p class="text-sm">Nebula host linked</p>
              <p class="text-xs text-base-content/70 mt-1">Details require an administrator</p>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <Network :size="28" :stroke-width="1.5" class="mx-auto text-base-content/30 mb-2" />
              <p class="text-sm">No Nebula host linked</p>
              <router-link
                v-if="authStore.can.manageInfrastructure"
                :to="`/things/${thing.id}/edit`"
                class="btn btn-sm btn-outline mt-3"
              >Attach a Nebula host</router-link>
            </div>
          </BaseCard>

          <!-- Not part of Connectivity: this reads only the type, open to every role. -->
          <BaseCard v-if="thingType" title="Subjects">
            <OperationSubjects
              :prefix="thingType.subject_prefix"
              :operations="typeOperations"
              :context="subjectContext"
              for-thing
            />
          </BaseCard>
        </div>
      </div>

      <div v-if="thing.code && natsStore.isConnected" class="mt-6">
        <KvDashboard
          :key="thing.code"
          title="Live State"
          :bucket="TWIN_BUCKET"
          :desired-bucket="TWIN_DESIRED_BUCKET"
          :base-key="`thing.${thing.code}`"
        />
      </div>
      <div v-else class="mt-6">
        <div v-if="hasTwinConfig && !natsStore.isConnected" class="alert shadow-sm border border-base-300 bg-base-100">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          <div class="text-xs">
            <div class="font-bold">Live State Offline</div>
            <span class="opacity-70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live data.</span>
          </div>
        </div>
      </div>

      <DangerZone v-if="authStore.can.decommissionInventory">
        <button @click="handleDelete" class="btn btn-error" :disabled="deleting">
          Delete Thing
        </button>
      </DangerZone>
    </template>

    <!-- No organization-name prop: a console user is in one org (the helpdesk passes it). -->
    <QrLabelModal
      v-if="showLabelModal && thing"
      :records="[{ code: thing.code || '', name: thing.name || '', kind: 'thing' }]"
      @close="showLabelModal = false"
    />
  </div>
</template>

<style scoped>
/* Thumb-sized below `lg`, where the app stops drawing touch layouts (as ListPager). */
@media (max-width: 1023px) {
  .connectivity-actions .btn {
    height: 44px;
    min-height: 44px;
  }
}

/* Below `sm`, three equal buttons on their own row, so Revoke is not stranded alone. */
@media (max-width: 639px) {
  .credential-actions .btn {
    flex: 1 1 0;
    padding-left: 0.25rem;
    padding-right: 0.25rem;
  }
}
</style>
