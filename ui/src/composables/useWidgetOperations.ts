import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useNatsStore } from '@/stores/nats'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { createDefaultWidget } from '@/types/dashboard'
import { resolveTemplate } from '@/utils/variables'
import { parseDurationMs } from '@/utils/duration'
import type { WidgetType, WidgetConfig, DataSourceConfig } from '@/types/dashboard'

// Each widget's subscriptions, recorded exactly as subscribed. Unsubscribe uses
// this record, never a re-derivation: by the time a variable watcher or a
// dashboard switch unsubscribes, the variables and `activeWidgets` have already
// changed. Module-level so every caller shares it.
const activeSubscriptions = new Map<string, DataSourceConfig[]>()

function subscriptionKey(c: DataSourceConfig): string {
  return `${c.useJetStream ? 'js' : 'core'}:${c.subject}`
}

export function useWidgetOperations() {
  const dashboardStore = useDashboardStore()
  const dataStore = useWidgetDataStore()
  const natsStore = useNatsStore()
  const subManager = getSubscriptionManager()

  const DEFAULT_REPLAY_WINDOW_MS = 5 * 60 * 1000

  function subscribeWidget(widgetId: string) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return

    // Buffered data older than the replay window would leave a gap in the chart.
    if (widget.dataSource.useJetStream && widget.dataSource.deliverPolicy === 'by_start_time') {
      const buffer = dataStore.getBuffer(widgetId)
      if (buffer.length > 0) {
        const latest = buffer[buffer.length - 1]
        const windowMs = parseDurationMs(replayWindow(widget)) ?? DEFAULT_REPLAY_WINDOW_MS
        if (Date.now() - latest.timestamp > windowMs) {
          dataStore.clearBuffer(widgetId)
        }
      }
    }

    // Only a chart ages out its buffer; other widgets show the latest value,
    // however old.
    const maxAge = widget.type === 'chart'
      ? parseDurationMs(widget.chartConfig?.window) ?? undefined
      : undefined
    // The buffer is keyed on the resolved subjects, so it survives leaving the
    // page but not a variable change to another device.
    const source = getWidgetSubjects(widget)
      .map(s => resolveTemplate(s, dashboardStore.currentVariableValues))
      .join('\n')
    dataStore.initializeBuffer(widgetId, widget.buffer.maxCount, maxAge, source)

    const wanted: Array<{ config: DataSourceConfig; jsonPath?: string; timestampPath?: string }> = []
    if (widget.dataSource.type === 'subscription') {
      for (const config of dataSourceConfigs(widget)) {
        wanted.push({ config, jsonPath: widget.jsonPath, timestampPath: widget.timestampPath })
      }
    }
    if (widget.type === 'map' && widget.mapConfig?.markers) {
      for (const config of mapMarkerConfigs(widget.mapConfig.markers)) wanted.push({ config })
    }

    // Drop what is no longer wanted, then subscribe (a no-op for one already held).
    const wantedKeys = new Set(wanted.map(w => subscriptionKey(w.config)))
    for (const old of activeSubscriptions.get(widgetId) ?? []) {
      if (!wantedKeys.has(subscriptionKey(old))) subManager.unsubscribe(widgetId, old)
    }
    for (const w of wanted) subManager.subscribe(widgetId, w.config, w.jsonPath, w.timestampPath)
    if (wanted.length > 0) activeSubscriptions.set(widgetId, wanted.map(w => w.config))
    else activeSubscriptions.delete(widgetId)
  }

  function dataSourceConfigs(widget: WidgetConfig): DataSourceConfig[] {
    const out: DataSourceConfig[] = []
    for (const rawSubject of getWidgetSubjects(widget)) {
      const subject = resolveTemplate(rawSubject, dashboardStore.currentVariableValues)
      if (!subject) continue
      out.push({ ...widget.dataSource, subject, timeWindow: replayWindow(widget) })
    }
    return out
  }

  // A chart's window is also its replay window, so the two cannot disagree.
  function replayWindow(widget: WidgetConfig): string | undefined {
    if (widget.type === 'chart' && widget.chartConfig?.window) return widget.chartConfig.window
    return widget.dataSource.timeWindow
  }

  function mapMarkerConfigs(markers: any[]): DataSourceConfig[] {
    const out: DataSourceConfig[] = []
    for (const marker of markers) {
      const pos = marker.positionConfig
      if (pos?.mode === 'dynamic' && pos.subject) {
        const subject = resolveTemplate(pos.subject, dashboardStore.currentVariableValues)
        if (subject) {
          out.push({
            type: 'subscription',
            subject,
            useJetStream: pos.useJetStream,
            deliverPolicy: pos.deliverPolicy || 'last'
          })
        }
      }
    }
    return out
  }

  function getWidgetSubjects(widget: WidgetConfig): string[] {
    if (widget.dataSource.subjects && widget.dataSource.subjects.length > 0) {
      return widget.dataSource.subjects
    }
    if (widget.dataSource.subject) {
      return [widget.dataSource.subject]
    }
    return []
  }

  // Uses the record, not the widget, which may be deleted or changed by now.
  function unsubscribeWidget(widgetId: string, keepData: boolean = false) {
    for (const config of activeSubscriptions.get(widgetId) ?? []) {
      subManager.unsubscribe(widgetId, config)
    }
    activeSubscriptions.delete(widgetId)

    if (!keepData) {
      dataStore.removeBuffer(widgetId)
    }
  }

  function subscribeAllWidgets() {
    if (dashboardStore.activeWidgets.length === 0) return
    for (const widget of dashboardStore.activeWidgets) {
      if (needsSubscription(widget.type, widget)) {
        subscribeWidget(widget.id)
      }
    }
  }

  // Everything recorded too: on a dashboard switch `activeWidgets` is already the new one's.
  function unsubscribeAllWidgets(keepData: boolean = false) {
    const ids = new Set(activeSubscriptions.keys())
    for (const widget of dashboardStore.activeWidgets) {
      if (needsSubscription(widget.type, widget)) ids.add(widget.id)
    }
    for (const id of ids) unsubscribeWidget(id, keepData)
  }

  function needsSubscription(widgetType: WidgetType, config?: WidgetConfig): boolean {
    if (widgetType === 'map') return true
    const selfManagedTypes: WidgetType[] = ['button', 'kv', 'kvtable', 'switch', 'slider', 'publisher', 'scanner']
    if (selfManagedTypes.includes(widgetType)) return false
    if (widgetType === 'status') return config?.dataSource?.type !== 'kv'
    if (widgetType === 'markdown') return !!(config?.dataSource?.subject)
    return true
  }

  function createWidget(type: WidgetType) {
    const position = { x: 0, y: 100 }
    // createDefaultWidget is the only source of defaults; do not layer more here.
    const widget = createDefaultWidget(type, position)
    dashboardStore.addWidget(widget)
    if (natsStore.isConnected && needsSubscription(type, widget)) {
      subscribeWidget(widget.id)
    }
    return widget
  }

  function deleteWidget(widgetId: string) {
    unsubscribeWidget(widgetId, false)
    dashboardStore.removeWidget(widgetId)
  }

  function duplicateWidget(widgetId: string) {
    const original = dashboardStore.getWidget(widgetId)
    if (!original) return null
    const copy = JSON.parse(JSON.stringify(original))
    const now = Date.now()
    copy.id = `widget_${now}_${Math.random().toString(36).substr(2, 9)}`
    copy.title = `${original.title} (Copy)`
    copy.y = original.y + original.h + 1
    copy.x = original.x
    dashboardStore.addWidget(copy)
    if (natsStore.isConnected && needsSubscription(copy.type, copy)) {
      subscribeWidget(copy.id)
    }
    return copy
  }

  function updateWidgetConfiguration(widgetId: string, updates: Partial<WidgetConfig>) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return
    if (needsSubscription(widget.type, widget)) {
      unsubscribeWidget(widgetId, false)
    }
    dashboardStore.updateWidget(widgetId, updates)
    const updatedWidget = dashboardStore.getWidget(widgetId)!
    if (natsStore.isConnected && needsSubscription(updatedWidget.type, updatedWidget)) {
      subscribeWidget(widgetId)
    }
  }

  function resubscribeWidget(widgetId: string) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget || !natsStore.isConnected) return
    if (needsSubscription(widget.type, widget)) {
      unsubscribeWidget(widgetId, false)
      subscribeWidget(widgetId)
    }
  }

  return {
    subscribeWidget, unsubscribeWidget, resubscribeWidget, subscribeAllWidgets, unsubscribeAllWidgets, needsSubscription,
    createWidget, deleteWidget, duplicateWidget, updateWidgetConfiguration,
  }
}
