<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import { expiryState, expiryLabel } from '@/utils/expiry'
import { caRotationState, rotateCA, type RotationStep } from '@/utils/nebula'
import { useAuthStore } from '@/stores/auth'
import type { NebulaCA } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const toast = useToast()
const { confirm } = useConfirm()
const authStore = useAuthStore()

const ca = ref<NebulaCA | null>(null)
const loading = ref(true)

const isValid = computed(() => {
  if (!ca.value?.expires_at) return false
  return new Date(ca.value.expires_at) > new Date()
})

/**
 * A CA cannot be renewed, only rotated -- and rotation is a three-step procedure
 * with a wait in the middle that nothing can compress, because it has to outlast
 * every host fetching a config nobody told it to fetch. So the countdown is
 * worth showing well before it is urgent: by the time a CA is a fortnight out,
 * the remedy no longer fits.
 */
const expiry = computed(() => expiryState(ca.value?.expires_at))
const expiryText = computed(() => expiryLabel(ca.value?.expires_at))

/** idle | prepared | rotated -- derived from the certificates, never stored. */
const rotationState = computed(() => caRotationState(ca.value))

const rotating = ref(false)

/**
 * What each step does, and what it costs. Shown in the confirm dialog rather
 * than only in the panel copy, because the three verbs are not equally
 * reversible and the button labels alone do not say so.
 */
const stepPrompts: Record<RotationStep, { title: string; message: string; details: string }> = {
  prepare: {
    title: 'Prepare CA rotation?',
    message: 'A new CA is minted and published to every host as additional trust.',
    details:
      'Issuance does not move: no host certificate changes and no fingerprint moves, ' +
      'so this is fully reversible. Wait until every host has fetched its config before committing.',
  },
  commit: {
    title: 'Commit CA rotation?',
    message: 'Issuance moves to the new CA and every active host is re-signed.',
    details:
      'Both CAs stay trusted, so re-signed and not-yet-re-signed hosts still talk to each other. ' +
      'Every active host needs its new config deployed before you finish.',
  },
  finish: {
    title: 'Finish CA rotation?',
    message: 'The outgoing CA is dropped from every config.',
    details:
      'This is refused while any active host still holds a certificate from the outgoing CA — ' +
      'dropping it early would take that host off the mesh with no error anywhere.',
  },
}

async function confirmRotation(step: RotationStep) {
  const prompt = stepPrompts[step]
  const confirmed = await confirm({
    title: prompt.title,
    message: prompt.message,
    details: prompt.details,
    confirmText: step === 'prepare' ? 'Prepare' : step === 'commit' ? 'Commit' : 'Finish',
    variant: step === 'finish' ? 'danger' : 'warning',
  })
  if (confirmed) await applyRotation(step)
}

async function applyRotation(step: RotationStep) {
  rotating.value = true
  try {
    await rotateCA(step)
    // The server names the reason it refused -- an out-of-order step, or the
    // host still holding an outgoing-CA certificate that blocks a finish. Those
    // messages are the whole point of the interlock, so they reach the operator
    // via the catch below rather than being replaced with a generic failure.
    await loadCA()
    toast.success({
      prepare: 'Rotation prepared — every host now trusts the incoming CA',
      commit: 'Rotation committed — active hosts have been re-signed',
      finish: 'Rotation finished — the outgoing CA has been dropped',
    }[step])
  } catch (err: any) {
    toast.error(err.message || `Failed to ${step} the rotation`)
  } finally {
    rotating.value = false
  }
}

async function loadCA() {
  if (!authStore.currentOrgId) return
  
  loading.value = true
  ca.value = null
  
  try {
    ca.value = await pb.collection('nebula_ca').getFirstListItem<NebulaCA>(
      `organization = "${authStore.currentOrgId}"`
    )
  } catch (err: any) {
    if (err.status !== 404) {
      toast.error(err.message || 'Failed to load Nebula CA')
    }
  } finally {
    loading.value = false
  }
}

async function provisionCA() {
  if (!authStore.currentOrgId || !authStore.currentOrg) return
  loading.value = true
  try {
    await pb.collection('nebula_ca').create({
      name: `${authStore.currentOrg.name} CA`,
      organization: authStore.currentOrgId,
      validity_years: 10,
      curve: 'P256'
    })
    toast.success('Nebula CA provisioned')
    await loadCA()
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch (err) {
    toast.error('Failed to copy')
  }
}

function downloadCert() {
  if (!ca.value?.certificate) return
  
  const blob = new Blob([ca.value.certificate], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'ca.crt'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
  toast.success('Certificate downloaded')
}

function handleOrgChange() {
  loadCA()
}

onMounted(() => {
  loadCA()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <div v-else-if="!ca" class="text-center py-12">
      <span class="text-6xl">🔐</span>
      <h3 class="text-xl font-bold mt-4">No Nebula CA Found</h3>
      <p class="text-base-content/70 mt-2 max-w-md mx-auto">
        Your organization does not have a Nebula Certificate Authority provisioned yet.
      </p>
      <button @click="provisionCA" class="btn btn-primary mt-6">
        Provision CA
      </button>
    </div>

    <template v-else>
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li>Nebula</li>
            <li>Certificate Authority</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ ca.name }}</h1>
            <div class="flex items-center gap-1.5 px-3 py-1 bg-base-200 rounded-full">
              <span class="w-2 h-2 rounded-full" :class="isValid ? 'bg-success' : 'bg-error'"></span>
              <span class="text-xs font-medium">{{ isValid ? 'Valid' : 'Expired' }}</span>
            </div>
            <span
              v-if="expiryText"
              class="badge"
              :class="expiry === 'expired' ? 'badge-error' : 'badge-warning'"
            >
              {{ expiryText }}
            </span>
          </div>
        </div>
      </div>
      
      <!--
        Rotation.

        Three steps rather than one button, and the wait between them is the
        feature. Nebula verification is mutual -- each peer checks the other
        against its OWN local CA pool, with no chain and no fallback -- and hosts
        pull their config whenever they like, with nothing telling us when they
        did. So a single write carrying both the new trust bundle and the new
        certificate splits the mesh: a host that has fetched presents a new-CA
        certificate to one that has not, and the handshake fails in BOTH
        directions until propagation finishes.

        Publishing trust first and switching issuance second removes that window
        entirely. How long to wait is operator judgement and cannot be designed
        away, only made visible.
      -->
      <BaseCard title="Certificate Authority Rotation">
        <div class="space-y-4">
          <!-- Where we are. Derived from the certificates, never a stored status. -->
          <ul class="steps steps-vertical sm:steps-horizontal w-full">
            <li class="step" :class="rotationState !== 'idle' ? 'step-primary' : ''">
              Trust published
            </li>
            <li class="step" :class="rotationState === 'rotated' ? 'step-primary' : ''">
              Issuance switched
            </li>
            <li class="step">Outgoing CA dropped</li>
          </ul>

          <div v-if="rotationState === 'idle'" class="text-sm text-base-content/70">
            No rotation in progress. A CA cannot be renewed — rotating is the only remedy
            for one approaching expiry, and it needs long enough in the middle for every
            host to fetch a config nobody told it to fetch. Nebula's own guidance is to
            begin two to three months out.
          </div>

          <div v-else-if="rotationState === 'prepared'" class="alert alert-info items-start">
            <span class="text-xl">⏳</span>
            <div class="text-sm">
              <div class="font-bold">An incoming CA is prepared and trusted.</div>
              Nothing is signed by it yet and no fingerprint has moved, so this is still
              fully reversible. Wait until every host has fetched its config, then commit.
            </div>
          </div>

          <div v-else class="alert alert-warning items-start">
            <span class="text-xl">🔁</span>
            <div class="text-sm">
              <div class="font-bold">Issuance has moved to the new CA.</div>
              Active hosts have been re-signed and both CAs are still trusted, so the mesh
              is whole. Deploy the new config to every active host, then finish — finishing
              is refused while any of them still holds a certificate from the outgoing CA.
            </div>
          </div>

          <div class="flex flex-col sm:flex-row gap-2">
            <button
              class="btn btn-outline"
              :disabled="rotating || rotationState !== 'idle'"
              @click="confirmRotation('prepare')"
            >
              1 · Prepare
            </button>
            <button
              class="btn btn-outline"
              :disabled="rotating || rotationState !== 'prepared'"
              @click="confirmRotation('commit')"
            >
              2 · Commit
            </button>
            <button
              class="btn btn-outline btn-error"
              :disabled="rotating || rotationState !== 'rotated'"
              @click="confirmRotation('finish')"
            >
              3 · Finish
            </button>
            <span v-if="rotating" class="loading loading-spinner self-center"></span>
          </div>

          <p v-if="ca.rotated_at" class="text-xs text-base-content/50">
            Last rotation step: {{ formatDate(ca.rotated_at) }}
          </p>
        </div>
      </BaseCard>

      <!-- Content Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Info -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Name</dt>
                <dd class="mt-1 text-sm">{{ ca.name }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Cryptography Curve</dt>
                <dd class="mt-1">
                  <code class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ ca.curve || 'P256' }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Validity Period</dt>
                <dd class="mt-1 text-sm">{{ ca.validity_years || 10 }} years</dd>
              </div>
              <div v-if="ca.expires_at">
                <dt class="text-sm font-medium text-base-content/70">Expiration Date</dt>
                <dd class="mt-1 text-sm">{{ formatDate(ca.expires_at) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(ca.created) }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>
        
        <!-- Right Column: Certificate -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-4">
                <h3 class="card-title text-base">Public Certificate</h3>
                <div class="flex gap-2">
                  <button 
                    @click="downloadCert" 
                    class="btn btn-sm btn-outline"
                  >
                    <span class="text-lg">📥</span>
                    ca.crt
                  </button>
                  <button 
                    @click="copyToClipboard(ca.certificate!, 'Certificate')" 
                    class="btn btn-sm btn-outline"
                  >
                    📋
                  </button>
                </div>
              </div>
            </template>

            <div class="space-y-4">
              <div class="alert alert-info py-2 text-sm">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                <span>Used by Nebula nodes to verify identity trust within the network.</span>
              </div>
              
              <div v-if="ca.certificate" class="bg-base-200 p-4 rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all border border-base-300 max-h-[500px]">
{{ ca.certificate.trim() }}
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
