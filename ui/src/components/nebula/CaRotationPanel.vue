<script setup lang="ts">
/**
 * CA rotation controls.
 *
 * Three steps rather than one button, and the wait between them is the feature.
 * Nebula verification is mutual -- each peer checks the other against its OWN
 * local CA pool, with no chain and no fallback -- and hosts pull their config
 * whenever they like, with nothing telling us when they did. So a single write
 * carrying both the new trust bundle and the new certificate splits the mesh: a
 * host that has fetched presents a new-CA certificate to one that has not, and
 * the handshake fails in BOTH directions until propagation finishes.
 *
 * Publishing trust first and switching issuance second removes that window
 * entirely. How long to wait is operator judgement and cannot be designed away,
 * only made visible.
 *
 * Presentation rule: ONE button, for the one step that is legal next. The panel
 * used to show all three at once with two of them disabled, which renders the
 * whole procedure as mostly-broken UI -- and the disabled pair said nothing the
 * tracker above them was not already saying. The tracker carries the shape, the
 * button carries the action. At idle there is no progress to draw, so the
 * tracker goes too and the panel collapses to a sentence and a button.
 */
import { ref, computed } from 'vue'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import { expiryState } from '@/utils/expiry'
import { caRotationState, rotateCA, type RotationStep } from '@/utils/nebula'
import type { NebulaCA } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const props = defineProps<{ ca: NebulaCA }>()
const emit = defineEmits<{ (e: 'rotated'): void }>()

const toast = useToast()
const { confirm } = useConfirm()

const rotating = ref(false)

/** idle | prepared | rotated -- derived from the certificates, never stored. */
const state = computed(() => caRotationState(props.ca))

/**
 * A CA cannot be renewed, only rotated -- and rotation needs long enough in the
 * middle to outlast every host fetching a config nobody told it to fetch. So an
 * approaching expiry is the one thing that makes this panel urgent while
 * nothing is in flight.
 */
const expiry = computed(() => expiryState(props.ca.expires_at))
const expiringSoon = computed(() => expiry.value === 'expiring' || expiry.value === 'expired')

/** The one step that is legal next, given where the certificates say we are. */
const nextStep = computed<RotationStep>(() =>
  state.value === 'idle' ? 'prepare' : state.value === 'prepared' ? 'commit' : 'finish',
)

const buttonLabel = computed(
  () =>
    ({
      prepare: 'Start rotation',
      commit: 'Commit rotation',
      finish: 'Finish rotation',
    })[nextStep.value],
)

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
    emit('rotated')
    toast.success(
      {
        prepare: 'Rotation prepared — every host now trusts the incoming CA',
        commit: 'Rotation committed — active hosts have been re-signed',
        finish: 'Rotation finished — the outgoing CA has been dropped',
      }[step],
    )
  } catch (err: any) {
    toast.error(err.message || `Failed to ${step} the rotation`)
  } finally {
    rotating.value = false
  }
}
</script>

<template>
  <!--
    Idle, the common case by a wide margin: a sentence and a button, sized to
    sit beside the rest of the page rather than tower over it.
  -->
  <BaseCard v-if="state === 'idle'" title="CA Rotation">
    <div class="space-y-3">
      <div v-if="expiringSoon" class="alert alert-warning items-start py-2">
        <span class="text-lg">⏰</span>
        <span class="text-sm">
          This CA is close to expiry and cannot be renewed. Begin rotating now — the
          middle of the procedure has to outlast every host fetching its config.
        </span>
      </div>

      <p class="text-sm text-base-content/70">
        No rotation in progress. A CA cannot be renewed — rotating is the only remedy for
        one approaching expiry, and it needs long enough in the middle for every host to
        fetch a config nobody told it to fetch. Nebula's own guidance is to begin two to
        three months out.
      </p>

      <div class="flex flex-wrap items-center gap-3">
        <button
          class="btn btn-sm"
          :class="expiringSoon ? 'btn-warning' : 'btn-outline'"
          :disabled="rotating"
          @click="confirmRotation('prepare')"
        >
          {{ buttonLabel }}
        </button>
        <span v-if="rotating" class="loading loading-spinner loading-sm"></span>
        <span v-if="ca.rotated_at" class="text-xs text-base-content/50">
          Last step: {{ formatDate(ca.rotated_at) }}
        </span>
      </div>
    </div>
  </BaseCard>

  <!-- In flight: there is progress to draw, and exactly one thing to do next. -->
  <BaseCard v-else title="CA Rotation In Progress">
    <div class="space-y-4">
      <ul class="steps steps-vertical sm:steps-horizontal w-full max-w-2xl">
        <li class="step step-primary">Trust published</li>
        <li class="step" :class="state === 'rotated' ? 'step-primary' : ''">Issuance switched</li>
        <li class="step">Outgoing CA dropped</li>
      </ul>

      <div v-if="state === 'prepared'" class="alert alert-info items-start">
        <span class="text-xl">⏳</span>
        <div class="text-sm">
          <div class="font-bold">An incoming CA is prepared and trusted.</div>
          Nothing is signed by it yet and no fingerprint has moved, so this is still fully
          reversible. Wait until every host has fetched its config, then commit.
        </div>
      </div>

      <div v-else class="alert alert-warning items-start">
        <span class="text-xl">🔁</span>
        <div class="text-sm">
          <div class="font-bold">Issuance has moved to the new CA.</div>
          Active hosts have been re-signed and both CAs are still trusted, so the mesh is
          whole. Deploy the new config to every active host, then finish — finishing is
          refused while any of them still holds a certificate from the outgoing CA.
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button
          class="btn btn-sm"
          :class="nextStep === 'finish' ? 'btn-error' : 'btn-primary'"
          :disabled="rotating"
          @click="confirmRotation(nextStep)"
        >
          {{ buttonLabel }}
        </button>
        <span v-if="rotating" class="loading loading-spinner loading-sm"></span>
        <span v-if="ca.rotated_at" class="text-xs text-base-content/50">
          Last step: {{ formatDate(ca.rotated_at) }}
        </span>
      </div>
    </div>
  </BaseCard>
</template>
