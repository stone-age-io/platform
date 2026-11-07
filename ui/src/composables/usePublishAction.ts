// Publish one message from a control and flash the result. Shared by
// ButtonWidget and each key of ButtonPadWidget.
//
// Subject and payload are raw config text: `{{variables}}` are resolved here,
// at click time, against the dashboard's current values.

import { ref, onUnmounted } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { encodeString } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'

export type PublishFlash = 'success' | 'error' | null

export function usePublishAction(options: { flashMs?: number } = {}) {
  const natsStore = useNatsStore()
  const dashboardStore = useDashboardStore()
  const flashMs = options.flashMs ?? 1500

  const flash = ref<PublishFlash>(null)
  let timer: ReturnType<typeof setTimeout> | undefined

  function setFlash(kind: Exclude<PublishFlash, null>) {
    clearTimeout(timer)
    flash.value = kind
    timer = setTimeout(() => { flash.value = null }, flashMs)
  }

  function resolve(text: string): string {
    return resolveTemplate(text, dashboardStore.currentVariableValues)
  }

  /** Fire and forget. Does nothing when there is no connection: the control is disabled then. */
  function publish(subject: string, payload: string) {
    if (!natsStore.nc) return
    try {
      natsStore.nc.publish(resolve(subject), encodeString(resolve(payload)))
      setFlash('success')
    } catch {
      setFlash('error')
    }
  }

  onUnmounted(() => clearTimeout(timer))

  return { flash, setFlash, resolve, publish }
}
