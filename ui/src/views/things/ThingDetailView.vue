<!-- ui/src/views/things/ThingDetailView.vue -->
<script setup lang="ts">
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

// The inventory schema declared by this Thing's type, if it has one. The type
// relation is expanded on load, so this needs no extra request.
const typeMetadataSchema = computed(
  () => (thing.value?.expand?.type as ThingType | undefined)?.metadata_schema || null,
)

// A quick metadata edit is an inventory write, which members hold. Deleting or
// deactivating is not — those stay on decommissionInventory.
const canEditMetadata = computed(() => authStore.can.manageInventory)

// MetadataCard has already persisted by the time this fires; patch the local
// record rather than refetching the whole Thing with its expands.
function onMetadataSaved(metadata: Record<string, any> | null) {
  if (thing.value) thing.value.metadata = metadata || undefined
}

/**
 * Live State (digital twin) keys are `thing.<code>.…` in the org-wide `twin`
 * bucket — the Thing's location is not part of the key, so a Thing needs only a
 * code. This used to also require a location code, a leftover from an earlier
 * bucket-per-location design that was never built; the effect was that a coded
 * Thing with no location rendered neither the card nor the offline hint.
 */
const thingCode = computed(() => thing.value?.code)
const hasTwinConfig = computed(() => !!thingCode.value)

// What this device speaks on: its type's operations resolved against this
// Thing. Types and operations are readable by every role in the organization,
// so unlike the identity details above this needs no administrator. Resolved
// exactly as the Publisher widget resolves a bound operation: codes, never
// names, and the id where a Thing has no code.
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
 * Whether the credential controls are offered at all.
 *
 * Both of them mint. `regenerate` re-signs for the same key and `revoke` hands
 * back a working replacement, and pb-nats does either on an inactive identity,
 * which issues a credential past the revocation cutoff -- the same thing
 * Reactivate does. So on a deactivated Thing they would be two ways back onto
 * the bus that skip Reactivate, under a badge saying the device is cut off.
 * Reactivate is the one lever, because it is the one that also restores the
 * Thing's own sign-in and its Nebula host (hooks/active_flag.go).
 *
 * There is deliberately no Re-enable here, unlike the NATS user page: for a
 * device `nats_users.active` mirrors `things.active`, and re-enabling the
 * identity alone would put the two out of step.
 */
const credentialsLive = computed(() => {
  const natsUser = thing.value?.expand?.nats_user as NatsUser | undefined
  return thing.value?.active !== false && !!natsUser?.active
})

// Regenerate is NOT a leak response: it re-signs for the SAME key, so every
// copy of the old file keeps working. What it is for on a device is renewing a
// credential before the expiry shown beside the username.
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

// The "credentials leaked" button, not a suspend: a new key pair, the old
// public key onto the account's revocation list, and a working replacement on
// the same record. Taking the device out of service is Deactivate.
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
 * Deactivate or reactivate the Thing.
 *
 * This is not a cosmetic flag. The server (hooks/active_flag.go) treats the flip
 * as a real decommission: it refreshes the record's tokenKey, which invalidates
 * every auth token the device already holds, and it revokes the linked NATS
 * identity so the signed credential stops working. Reactivating issues a FRESH
 * NATS credential — the old .creds file stays dead, because the revocation
 * cutoff baked into the account JWT is permanent. Say so in the dialog; an
 * operator who expects the old creds to resume will otherwise be surprised at
 * the worst possible moment.
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
    // The code where there is one, the name where there is not. Code is
    // optional on things, and a gate that quietly vanishes on the records
    // without one would be worse than having no gate at all.
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

      <!--
        A deactivated Thing is fully cut off, not merely flagged: its auth tokens
        were invalidated and its NATS credential revoked on the flip. Spelling
        that out here is the point of the banner -- the failure mode this feature
        exists to avoid is an operator reading a red badge and assuming the
        device is silent when it is not.
      -->
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
              The photo is a COLUMN BESIDE the fields, not a row above them.

              It shipped first as a full-width `Photo` entry at the top of this
              list, which left a 160px band of empty card to its right and
              pushed every field below it down for no gain. Beside them it
              fills that gutter instead and the card is roughly half as tall.
              Dropping the `Photo` label costs nothing -- it was only telling
              the reader that a photograph is a photograph.

              It stacks ABOVE the fields on a phone (DOM order, plus
              sm:order-last to move it right once there is room), because a
              narrow screen has no gutter to fill and the photo is the fastest
              confirmation you are on the right record.

              Wrapped in a div rather than given the order class directly:
              RecordPhoto has a fragment root -- the plate and its Teleported
              dialog -- so class fallthrough does not apply to it.

              With no photo the list simply takes the full width. That is
              better than the em-dash the labelled version had to render, since
              there is no longer a field sitting empty. The exception is a
              reader who can add one: they get PhotoQuickAdd's dashed plate in
              the same gutter, at the same size.
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
                      📍 {{ thing.expand.location.name }}
                    </router-link>
                    <span v-else class="text-sm text-base-content/40">No location assigned</span>
                  </dd>
                </div>
              </dl>
            </div>

            <!--
              Timestamps span the FULL card, below the photo, rather than
              sharing the squeezed column with the fields above.

              Measured, not guessed: this card sits in the page's two-column
              grid, so a NARROWER viewport gives it a NARROWER body. With the
              photo taking 140px beside them, the two timestamp columns came to
              108px each at a 1440px viewport -- "Sep 19, 2026 6:40:11 PM"
              needs about 125px, so both columns wrapped to three lines on one
              of the commonest laptop widths. Below the photo they get the full
              body width and stop wrapping, and the card is still far shorter
              than the stacked version this replaced.

              A second <dl> rather than one list split across two containers:
              each is a complete description list, which is what the element is
              for, and RecordTimestamps needs a <dl> parent to emit its dt/dd
              pairs into.
            -->
            <dl class="grid grid-cols-2 gap-4 mt-4">
              <!-- Both timestamps, not only Created. This is the record side of
                   the activity feed: the feed reports an update and the record
                   has to carry something to correlate that against. -->
              <RecordTimestamps :created="thing.created" :updated="thing.updated" />
            </dl>
          </BaseCard>

          <!-- Metadata: read-only field list / JSON, with quick edit in place.
               Rendered unconditionally now — an empty card is how you ADD the
               first field to a Thing that has none. -->
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
          <BaseCard>
            <!--
              The three controls used to sit up here in a row under
              "Connectivity", which is one heading over two identities. Two of
              them were download buttons with the same 📥 on them, and the NATS
              one hid its label below 640px, so a phone showed 📥 / 🔄 /
              📥 Config -- two downloads, one of them unlabelled, for different
              files. Each now sits in the section that names what it acts on,
              with a label that fits because it has a row to itself.

              This is not a mobile fix that costs the desktop something. The
              card lives in `lg:grid-cols-2`, so it is 394px of content on a
              1280px desktop against 295px on a phone -- close enough that one
              layout genuinely serves both, and measuring both confirmed it
              (552px and 557px tall). It costs 56px either way, which buys the
              two labels and the touch targets.
            -->
            <template #header>
              <h3 class="card-title text-base mb-2">Connectivity</h3>
            </template>

            <!-- NATS Section.

                 flex-wrap rather than a breakpoint: at 393px and up the label
                 and both buttons share the row, and at 320px they do not fit,
                 so the buttons take their own line. Measured, the wrap costs
                 nothing at all until it is needed -- 557px at 360px wide with
                 it and without it -- which is the argument for wrapping over a
                 `flex-col sm:flex-row`, since that one pays 64px on every
                 phone to fix the narrowest. -->
            <div class="flex flex-wrap items-center justify-between gap-2 mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">NATS</span>
              <!-- Same condition the section body below uses, so the two
                   not-expanded states show a heading with no controls under
                   it rather than a download for something unreadable. Plus
                   credentialsLive: see its note for why a cut-off device
                   offers none of them. -->
              <div v-if="thing.expand?.nats_user && credentialsLive" class="connectivity-actions credential-actions flex gap-2 w-full sm:w-auto">
                <button @click="downloadNatsCreds" class="btn btn-sm btn-outline" title="Download .creds file">
                  <span>📥</span>
                  <span>.creds</span>
                </button>
                <!-- Plain outline: regenerate invalidates nothing, so the
                     danger colour it used to wear oversold it. Revoke is the
                     one that breaks a deployed device. -->
                <button
                  @click="regenerateCreds"
                  class="btn btn-sm btn-outline"
                  title="Issue a fresh .creds file for the same key"
                  :disabled="regenerating || revoking"
                >
                  <span>🔄</span>
                  <span>Regenerate</span>
                </button>
                <button
                  @click="revokeCreds"
                  class="btn btn-sm btn-outline btn-error"
                  title="Reject the current credentials and issue replacements"
                  :disabled="regenerating || revoking"
                >
                  <span>⛔</span>
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
                  Link only for a reader who can actually open /nats/roles/:id —
                  the route guards on manageInfrastructure, so for anyone else the
                  link is a dead end that bounces them off the page. Same reason
                  the raw role_id is no longer the fallback: nats_roles is
                  owner/admin-only, so a reader without the expand gets a PB id
                  they cannot look up.
                -->
                <router-link
                  v-if="thing.expand.nats_user.expand?.role_id && authStore.can.manageInfrastructure"
                  :to="`/nats/roles/${thing.expand.nats_user.role_id}`"
                  class="link link-primary text-sm font-mono"
                >
                  🎭 {{ thing.expand.nats_user.expand.role_id.name }}
                </router-link>
                <span v-else-if="thing.expand.nats_user.expand?.role_id" class="font-mono text-sm">
                  🎭 {{ thing.expand.nats_user.expand.role_id.name }}
                </span>
                <span v-else class="font-mono text-sm text-base-content/60">—</span>
              </div>
              <!-- Says where the controls went. A deactivated Thing already has
                   the page banner; this covers the identity being inactive on
                   its own, which a Thing reached only out of step. -->
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
                 their own row). The relation ID is on the things record and IS
                 readable, so "linked" and "not linked" stay distinguishable
                 without it — saying "No NATS user linked" here was simply false. -->
            <div v-else-if="thing.nats_user" class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">🔒</span>
              <p class="text-sm">NATS identity linked</p>
              <p class="text-xs text-base-content/70 mt-1">Details require an administrator</p>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">📡</span>
              <p class="text-sm">No NATS user linked</p>
            </div>

            <!-- Divider -->
            <div class="border-t border-base-300 my-4"></div>

            <!-- Nebula Section. See the NATS heading above for the wrap. -->
            <div class="flex flex-wrap items-center justify-between gap-2 mb-1">
              <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Nebula</span>
              <div v-if="thing.expand?.nebula_host" class="connectivity-actions flex gap-2">
                <button @click="downloadNebulaConfig" class="btn btn-sm btn-outline" title="Download Nebula config">
                  <span>📥</span>
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
              <span class="text-2xl block mb-2">🔒</span>
              <p class="text-sm">Nebula host linked</p>
              <p class="text-xs text-base-content/70 mt-1">Details require an administrator</p>
            </div>
            <div v-else class="text-center py-6 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">🌐</span>
              <p class="text-sm">No Nebula host linked</p>
            </div>
          </BaseCard>

          <!-- Its own card rather than a third section of Connectivity: those
               two sections describe identities a member may not be able to
               read, while this reads only the type and is open to every role. -->
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

    <!--
      No organization-name prop: a console user is scoped to one organization and
      already knows whose device this is. The helpdesk passes it because its staff
      are cross-customer.
    -->
    <QrLabelModal
      v-if="showLabelModal && thing"
      :records="[{ code: thing.code || '', name: thing.name || '', kind: 'thing' }]"
      @close="showLabelModal = false"
    />
  </div>
</template>

<style scoped>
/*
  The connectivity controls, sized for a thumb below `lg` -- the same rule and
  the same breakpoint as ListPager, for the same reason: these render at every
  width, so the size has to be conditional, and `lg` is where this app stops
  drawing touch layouts. `btn-sm` is 32px, under both guidelines.

  The buttons also dropped a redundant `h-8 min-h-0`, which set the height
  `btn-sm` was already setting.
*/
@media (max-width: 1023px) {
  .connectivity-actions .btn {
    height: 44px;
    min-height: 44px;
  }
}

/*
  Three credential buttons do not fit beside the NATS heading on a phone, and
  left to wrap they stranded Revoke on a row of its own. Below `sm` they take
  the full row under the heading instead, as three equal buttons.
*/
@media (max-width: 639px) {
  .credential-actions .btn {
    flex: 1 1 0;
    padding-left: 0.25rem;
    padding-right: 0.25rem;
  }
}
</style>
