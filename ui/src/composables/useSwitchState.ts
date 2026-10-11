import { ref, computed, watch, unref, onScopeDispose, type Ref } from 'vue'
import type { Subscription } from '@nats-io/nats-core'
import { useNatsStore } from '@/stores/nats'
import { useKvKey } from '@/composables/useKvKey'
import { decodeBytes, encodeString } from '@/utils/encoding'

export interface SwitchStateConfig {
  mode: 'kv' | 'core'
  kvBucket?: string
  kvKey?: string
  publishSubject?: string
  stateSubject?: string
  onPayload: any
  offPayload: any
  defaultState?: 'on' | 'off'
}

export type SwitchStateValue = 'on' | 'off' | 'pending' | 'unknown'

/**
 * On/off state for the switch widget and the map marker's switch row.
 *
 * KV mode: the key IS the state, read through useKvKey. A key that is missing
 * (never written, or deleted) reads as the configured default; it used to read
 * as "off" whatever the default said, and a value that was not JSON killed the
 * watch silently. Core mode: the last matching message on the state subject,
 * starting from the default.
 *
 * A toggle shows `pending` until the new state is confirmed, for at most five
 * seconds, and then falls back to the last confirmed state. It used to stay
 * `pending` and show "..." until some later message happened to match.
 */
export function useSwitchState(configSource: Ref<SwitchStateConfig> | SwitchStateConfig) {
  const natsStore = useNatsStore()
  const cfg = computed(() => unref(configSource))
  const isKv = computed(() => cfg.value.mode === 'kv')
  const fallback = computed(() => cfg.value.defaultState || 'off')

  const kv = useKvKey(
    () => isKv.value ? cfg.value.kvBucket || '' : '',
    () => isKv.value ? cfg.value.kvKey || '' : '',
  )

  const coreState = ref<'on' | 'off' | 'unknown'>('unknown')
  const coreError = ref<string | null>(null)
  const pending = ref(false)
  let pendingTimer: ReturnType<typeof setTimeout> | null = null
  let subscription: Subscription | null = null

  const confirmed = computed<'on' | 'off' | 'unknown'>(() => {
    if (!isKv.value) return coreState.value
    if (kv.status.value === 'missing') return fallback.value
    if (kv.status.value === 'ready' && kv.entry.value) return stateOf(kv.entry.value.value) ?? 'unknown'
    return 'unknown'
  })

  const state = computed<SwitchStateValue>(() => pending.value ? 'pending' : confirmed.value)
  const isPending = computed(() => pending.value || (isKv.value && kv.status.value === 'loading'))
  const error = computed(() => coreError.value || kv.error.value)

  function stateOf(value: unknown): 'on' | 'off' | null {
    const text = JSON.stringify(value)
    if (text === JSON.stringify(cfg.value.onPayload)) return 'on'
    if (text === JSON.stringify(cfg.value.offPayload)) return 'off'
    return null
  }

  // --- Core mode subscription ---

  function stopCore() {
    if (subscription) {
      try { subscription.unsubscribe() } catch { /* already closed */ }
      subscription = null
    }
  }

  function startCore() {
    stopCore()
    coreError.value = null
    if (isKv.value || !natsStore.isConnected || !natsStore.nc) {
      coreState.value = 'unknown'
      return
    }
    coreState.value = fallback.value
    const subject = cfg.value.stateSubject || cfg.value.publishSubject
    if (!subject) {
      coreError.value = 'No state subject configured'
      return
    }
    const sub = natsStore.nc.subscribe(subject)
    subscription = sub
    ;(async () => {
      try {
        for await (const msg of sub) {
          const next = stateOf(parseMessage(msg.data))
          if (next) coreState.value = next
        }
      } catch { /* unsubscribed */ }
    })()
  }

  watch(
    [() => natsStore.isConnected, isKv, () => cfg.value.stateSubject, () => cfg.value.publishSubject],
    () => startCore(),
    { immediate: true },
  )

  // --- Toggle ---

  function endPending() {
    pending.value = false
    if (pendingTimer) {
      clearTimeout(pendingTimer)
      pendingTimer = null
    }
  }

  // Any change to the confirmed state ends the wait, including one that is not
  // the state asked for: it is what the device actually reports.
  watch(confirmed, endPending)

  async function toggle() {
    if (!natsStore.isConnected) {
      coreError.value = 'Not connected'
      return
    }
    const target = confirmed.value === 'on' ? 'off' : 'on'
    const payload = target === 'on' ? cfg.value.onPayload : cfg.value.offPayload

    coreError.value = null
    pending.value = true
    if (pendingTimer) clearTimeout(pendingTimer)
    pendingTimer = setTimeout(endPending, 5000)

    try {
      if (isKv.value) {
        await kv.put(JSON.stringify(payload))
      } else {
        const subject = cfg.value.publishSubject
        if (!natsStore.nc) throw new Error('Not connected to NATS')
        if (!subject) throw new Error('No publish subject')
        natsStore.nc.publish(subject, serializePayload(payload))
        // Without a state subject nothing will ever confirm it, so believe it.
        if (!cfg.value.stateSubject) {
          coreState.value = target
          endPending()
        }
      }
    } catch (err: any) {
      coreError.value = err?.message || 'Failed to toggle switch'
      endPending()
    }
  }

  const refresh = () => startCore()
  window.addEventListener('dashboard:refresh', refresh)
  onScopeDispose(() => {
    stopCore()
    endPending()
    window.removeEventListener('dashboard:refresh', refresh)
  })

  return { state, error, isPending, toggle }
}

function parseMessage(data: Uint8Array): unknown {
  try {
    const text = decodeBytes(data)
    try { return JSON.parse(text) } catch { return text }
  } catch {
    return null
  }
}

function serializePayload(payload: any): Uint8Array {
  if (typeof payload === 'string') return encodeString(payload)
  if (typeof payload === 'number' || typeof payload === 'boolean') {
    return encodeString(String(payload))
  }
  return encodeString(JSON.stringify(payload))
}
