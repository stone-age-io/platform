import { ref, watch, onScopeDispose } from 'vue'
import { Kvm, type KV, type KvEntry } from '@nats-io/kv'
import { useNatsStore } from '@/stores/nats'
import { decodeBytes, encodeString } from '@/utils/encoding'

/**
 * Watch one key of a NATS KV bucket.
 *
 * The slider, the status light, the map marker's KV row and the switch each
 * carried their own copy of this, and the copies had drifted: two kept showing
 * a deleted key's last value, one turned a deleted key into "off" whatever the
 * switch's default said, three read every value twice, and only one stopped a
 * restart from running beside the slower one it replaced.
 *
 * - `get` first, because it is the only call that can say a key is MISSING. A
 *   watch on an absent key delivers nothing, which looks exactly like slow.
 * - Then `watch`, skipping revisions `get` already delivered, so each value is
 *   handled once and a write landing between the two calls is still seen.
 * - A deleted or purged key is `missing`, the same as one that never existed.
 * - Bucket and key are getters, so template variables are tracked. A change
 *   restarts the watch and clears the old value, which must not stand in for a
 *   key that may not exist.
 */
export type KvKeyStatus = 'idle' | 'loading' | 'ready' | 'missing' | 'error'

export interface KvKeyEntry {
  /** Parsed JSON, or the text itself when it is not JSON. */
  value: unknown
  text: string
  revision: number
  /** When the value was written, not when it was read. */
  updated: Date
}

export function useKvKey(bucket: () => string, key: () => string) {
  const natsStore = useNatsStore()

  const entry = ref<KvKeyEntry | null>(null)
  const status = ref<KvKeyStatus>('idle')
  const error = ref<string | null>(null)

  let kv: KV | null = null
  let kvKey = ''
  let watcher: { stop(): void } | null = null
  let generation = 0

  function stop() {
    generation++
    if (watcher) {
      try { watcher.stop() } catch { /* already stopped */ }
      watcher = null
    }
    kv = null
    entry.value = null
    error.value = null
    status.value = 'idle'
  }

  function apply(e: KvEntry) {
    if (e.operation !== 'PUT') {
      entry.value = null
      status.value = 'missing'
      return
    }
    const text = decodeBytes(e.value)
    let value: unknown = text
    try { value = JSON.parse(text) } catch { /* plain text */ }
    entry.value = { value, text, revision: e.revision, updated: e.created }
    status.value = 'ready'
  }

  async function start() {
    stop()
    const b = bucket()
    const k = key()
    if (!b || !k || !natsStore.isConnected || !natsStore.nc) return

    const gen = generation
    status.value = 'loading'
    try {
      const opened = await new Kvm(natsStore.nc).open(b)
      if (gen !== generation) return

      const first = await getOrNull(opened, k)
      if (gen !== generation) return
      kv = opened
      kvKey = k
      if (first) apply(first)
      else status.value = 'missing'

      const iter = await opened.watch({ key: k })
      if (gen !== generation) { try { iter.stop() } catch { /* stopped */ } return }
      watcher = iter

      const seen = first?.revision ?? 0
      ;(async () => {
        try {
          for await (const e of iter) {
            if (gen !== generation) break
            if (e.revision > seen) apply(e)
          }
        } catch { /* the watch was stopped */ }
      })()
    } catch (err: any) {
      if (gen !== generation) return
      status.value = 'error'
      error.value = err?.message?.includes('stream not found')
        ? `Bucket "${b}" not found`
        : err?.message || 'KV watch failed'
    }
  }

  /** Write to the watched key. Throws until the bucket is open. */
  async function put(data: string | Uint8Array) {
    if (!kv) throw new Error('KV not ready')
    await kv.put(kvKey, typeof data === 'string' ? encodeString(data) : data)
  }

  // isConnected also covers a reconnect: it goes false while the client
  // retries and true again once it is back.
  watch([() => natsStore.isConnected, bucket, key], () => start(), { immediate: true })

  const refresh = () => start()
  window.addEventListener('dashboard:refresh', refresh)
  onScopeDispose(() => {
    stop()
    window.removeEventListener('dashboard:refresh', refresh)
  })

  return { entry, status, error, put, restart: start }
}

// Recent servers answer a missing key with null; older ones with an error.
async function getOrNull(kv: KV, key: string): Promise<KvEntry | null> {
  try {
    return await kv.get(key)
  } catch (err: any) {
    if (/no message found|key not found/i.test(err?.message ?? '')) return null
    throw err
  }
}
