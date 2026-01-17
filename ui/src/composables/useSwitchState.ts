import { ref, type Ref, onMounted, onUnmounted, watch, isRef } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { Kvm } from '@nats-io/kv'
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

export interface SwitchState {
  state: Ref<SwitchStateValue>
  error: Ref<string | null>
  isPending: Ref<boolean>
  start: () => Promise<void>
  stop: () => void
  toggle: (requestConfirm?: (onConfirm: () => void) => void) => Promise<void>
  isActive: () => boolean
}

export function createSwitchState(config: SwitchStateConfig): SwitchState {
  const natsStore = useNatsStore()
  
  const state = ref<SwitchStateValue>('unknown')
  const error = ref<string | null>(null)
  const isPending = ref(false)
  
  let kvWatcher: any = null
  let kvInstance: any = null
  let subscription: any = null
  let active = false
  let reconnectHandler: (() => void) | null = null
  let closeHandler: (() => void) | null = null
  
  async function start(): Promise<void> {
    if (active) return
    if (!natsStore.nc || !natsStore.isConnected) {
      error.value = 'Not connected to NATS'
      return
    }
    
    active = true
    error.value = null
    isPending.value = true
    
    setupEventHandlers()
    
    try {
      if (config.mode === 'kv') {
        await initializeKvMode()
      } else {
        await initializeCoreMode()
      }
    } catch (err: any) {
      error.value = err.message
      isPending.value = false
    }
  }
  
  function stop(): void {
    active = false
    if (reconnectHandler) {
      window.removeEventListener('nats:reconnected', reconnectHandler)
      reconnectHandler = null
    }
    if (closeHandler) {
      window.removeEventListener('nats:closed', closeHandler)
      closeHandler = null
    }
    cleanupWatchers()
    isPending.value = false
  }
  
  function isActive(): boolean {
    return active
  }
  
  function setupEventHandlers() {
    if (reconnectHandler) return
    
    reconnectHandler = async () => {
      if (!active) return
      cleanupWatchers()
      isPending.value = true
      try {
        if (config.mode === 'kv') await initializeKvMode()
        else await initializeCoreMode()
      } catch (err: any) {
        error.value = `Reconnect failed: ${err.message}`
        isPending.value = false
      }
    }
    
    closeHandler = () => {
      if (!active) return
      cleanupWatchers()
      state.value = 'unknown'
      error.value = 'Connection closed'
    }
    
    window.addEventListener('nats:reconnected', reconnectHandler)
    window.addEventListener('nats:closed', closeHandler)
  }
  
  function cleanupWatchers() {
    if (kvWatcher) { try { kvWatcher.stop() } catch {} kvWatcher = null }
    kvInstance = null
    if (subscription) { try { subscription.unsubscribe() } catch {} subscription = null }
  }
  
  async function initializeKvMode(): Promise<void> {
    const bucket = config.kvBucket
    const key = config.kvKey
    
    if (!bucket || !key) throw new Error('KV bucket and key required')
    
    try {
      const kvm = new Kvm(natsStore.nc!)
      const kv = await kvm.open(bucket)
      kvInstance = kv
      
      try {
        const entry = await kv.get(key)
        if (entry) {
          const value = JSON.parse(decodeBytes(entry.value))
          updateStateFromValue(value)
        } else {
          state.value = config.defaultState || 'off'
        }
      } catch (e: any) {
        if (!e.message?.includes('key not found')) throw e
        state.value = config.defaultState || 'off'
      }
      
      isPending.value = false
      
      const iter = await kv.watch({ key })
      kvWatcher = iter
      
      ;(async () => {
        try {
          for await (const e of iter) {
            if (!active) break
            if (e.key === key) {
              if (e.operation === 'PUT') {
                const value = JSON.parse(decodeBytes(e.value!))
                updateStateFromValue(value)
                isPending.value = false
              } else if (e.operation === 'DEL' || e.operation === 'PURGE') {
                state.value = 'off'
                isPending.value = false
              }
            }
          }
        } catch { /* ignore */ }
      })()
      
    } catch (err: any) {
      if (err.message?.includes('stream not found')) {
        throw new Error(`Bucket "${bucket}" not found`)
      }
      throw err
    }
  }
  
  async function initializeCoreMode(): Promise<void> {
    state.value = config.defaultState || 'off'
    const stateSubject = config.stateSubject || config.publishSubject
    
    if (!stateSubject) throw new Error('No state subject configured')
    
    isPending.value = false
    
    try {
      subscription = natsStore.nc!.subscribe(stateSubject)
      ;(async () => {
        try {
          for await (const msg of subscription) {
            if (!active) break
            const data = parseMessage(msg.data)
            updateStateFromValue(data)
            isPending.value = false
          }
        } catch { /* ignore */ }
      })()
    } catch (err: any) {
      throw err
    }
  }
  
  function parseMessage(data: Uint8Array): any {
    try {
      const text = decodeBytes(data)
      try { return JSON.parse(text) } catch { return text }
    } catch { return null }
  }
  
  function updateStateFromValue(value: any): void {
    if (matchesPayload(value, config.onPayload)) {
      state.value = 'on'
    } else if (matchesPayload(value, config.offPayload)) {
      state.value = 'off'
    }
  }
  
  function matchesPayload(value: any, payload: any): boolean {
    return JSON.stringify(value) === JSON.stringify(payload)
  }
  
  async function toggle(requestConfirm?: (onConfirm: () => void) => void): Promise<void> {
    if (!natsStore.isConnected) {
      error.value = 'Not connected'
      return
    }
    
    const targetState = state.value === 'on' ? 'off' : 'on'
    
    if (requestConfirm) {
      requestConfirm(() => executeToggle(targetState))
    } else {
      await executeToggle(targetState)
    }
  }
  
  async function executeToggle(targetState: 'on' | 'off'): Promise<void> {
    error.value = null
    isPending.value = true
    const previousState = state.value
    state.value = 'pending'
    
    const payload = targetState === 'on' ? config.onPayload : config.offPayload
    
    try {
      if (config.mode === 'kv') {
        if (!kvInstance) throw new Error('KV instance not initialized')
        const key = config.kvKey!
        const data = encodeString(JSON.stringify(payload))
        await kvInstance.put(key, data)
      } else {
        if (!natsStore.nc) throw new Error('Not connected to NATS')
        const subject = config.publishSubject
        if (!subject) throw new Error('No publish subject')
        const data = serializePayload(payload)
        natsStore.nc.publish(subject, data)
        
        if (!config.stateSubject) {
          state.value = targetState
          isPending.value = false
        }
      }
      
      setTimeout(() => {
        if (isPending.value && state.value === 'pending') {
          isPending.value = false
        }
      }, 5000)
      
    } catch (err: any) {
      error.value = err.message || 'Failed to toggle switch'
      isPending.value = false
      state.value = previousState === 'pending' ? 'unknown' : previousState
    }
  }
  
  function serializePayload(payload: any): Uint8Array {
    if (typeof payload === 'string') return encodeString(payload)
    if (typeof payload === 'number' || typeof payload === 'boolean') {
      return encodeString(String(payload))
    }
    return encodeString(JSON.stringify(payload))
  }
  
  return { state, error, isPending, start, stop, toggle, isActive }
}

export function useSwitchState(configSource: Ref<SwitchStateConfig> | SwitchStateConfig) {
  const natsStore = useNatsStore()
  
  const state = ref<SwitchStateValue>('unknown')
  const error = ref<string | null>(null)
  const isPending = ref(true)
  
  let instance: SwitchState | null = null
  let stateWatcherStop: (() => void) | null = null

  function init() {
    if (instance) {
      instance.stop()
      if (stateWatcherStop) stateWatcherStop()
    }

    const cfg = isRef(configSource) ? configSource.value : configSource
    instance = createSwitchState(cfg)

    state.value = instance.state.value
    error.value = instance.error.value
    isPending.value = instance.isPending.value

    stateWatcherStop = watch(
      [instance.state, instance.error, instance.isPending], 
      ([s, e, p]) => {
        state.value = s
        error.value = e
        isPending.value = p
      }
    )

    if (natsStore.isConnected) {
      instance.start()
    }
  }

  onMounted(() => {
    init()
  })

  onUnmounted(() => {
    if (instance) instance.stop()
    if (stateWatcherStop) stateWatcherStop()
  })

  if (isRef(configSource)) {
    watch(configSource, () => init(), { deep: true })
  }

  watch(() => natsStore.isConnected, (connected) => {
    if (connected && instance) {
      instance.start()
    } else if (!connected && instance) {
      instance.stop()
    }
  })

  const toggle = async (cb?: (c: () => void) => void) => {
    if (instance) await instance.toggle(cb)
  }

  return { state, error, isPending, toggle }
}
