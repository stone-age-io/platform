// ui/src/composables/useWidgetOperations.ts
import { useDashboardStore } from '@/stores/dashboard'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useNatsStore } from '@/stores/nats'
import { getSubscriptionManager } from '@/composables/useSubscriptionManager'
import { createDefaultWidget } from '@/types/dashboard'
import { resolveTemplate } from '@/utils/variables'
import type { WidgetType, WidgetConfig, DataSourceConfig } from '@/types/dashboard'

export function useWidgetOperations() {
  const dashboardStore = useDashboardStore()
  const dataStore = useWidgetDataStore()
  const natsStore = useNatsStore()
  const subManager = getSubscriptionManager()

  // Grug-helper: parse "5m", "1h" etc to milliseconds
  function parseWindowToMs(windowStr: string | undefined): number {
    if (!windowStr) return 300000 // default 5m
    const match = windowStr.match(/^(\d+)([smhd])$/)
    if (!match) return 300000
    const val = parseInt(match[1])
    const unit = match[2]
    switch (unit) {
      case 's': return val * 1000
      case 'm': return val * 60 * 1000
      case 'h': return val * 60 * 60 * 1000
      case 'd': return val * 24 * 60 * 60 * 1000
      default: return 300000
    }
  }

  function subscribeWidget(widgetId: string) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return
    
    // 1. Grug check: Is existing data too stale for the JetStream window?
    if (widget.dataSource.useJetStream && widget.dataSource.deliverPolicy === 'by_start_time') {
      const buffer = dataStore.getBuffer(widgetId)
      if (buffer.length > 0) {
        const latest = buffer[buffer.length - 1]
        const windowMs = parseWindowToMs(widget.dataSource.timeWindow)
        // If the gap is bigger than the replay window, wipe it to avoid chart gaps
        if (Date.now() - latest.timestamp > windowMs) {
          dataStore.clearBuffer(widgetId)
        }
      }
    }

    // 2. Standard initialization
    dataStore.initializeBuffer(widgetId, widget.buffer.maxCount, widget.buffer.maxAge)
    
    if (widget.dataSource.type === 'subscription') {
      subscribeToDataSource(widgetId, widget)
    }

    if (widget.type === 'map' && widget.mapConfig?.markers) {
      subscribeMapMarkers(widgetId, widget.mapConfig.markers)
    }
  }

  function subscribeToDataSource(widgetId: string, widget: WidgetConfig) {
    const subjects = getWidgetSubjects(widget)
    for (const rawSubject of subjects) {
      const subject = resolveTemplate(rawSubject, dashboardStore.currentVariableValues)
      if (!subject) continue
      const config: DataSourceConfig = { ...widget.dataSource, subject }
      subManager.subscribe(widgetId, config, widget.jsonPath)
    }
  }

  function subscribeMapMarkers(widgetId: string, markers: any[]) {
    for (const marker of markers) {
      const pos = marker.positionConfig
      if (pos?.mode === 'dynamic' && pos.subject) {
        const subject = resolveTemplate(pos.subject, dashboardStore.currentVariableValues)
        if (subject) {
          subManager.subscribe(widgetId, {
            type: 'subscription',
            subject,
            useJetStream: pos.useJetStream,
            deliverPolicy: pos.deliverPolicy || 'last'
          })
        }
      }
    }
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

  function unsubscribeWidget(widgetId: string, keepData: boolean = false) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return
    
    if (widget.dataSource.type === 'subscription') {
      const subjects = getWidgetSubjects(widget)
      for (const rawSubject of subjects) {
        const subject = resolveTemplate(rawSubject, dashboardStore.currentVariableValues)
        if (!subject) continue
        const config: DataSourceConfig = { ...widget.dataSource, subject }
        subManager.unsubscribe(widgetId, config)
      }
    }

    if (widget.type === 'map' && widget.mapConfig?.markers) {
      for (const marker of widget.mapConfig.markers) {
        const pos = marker.positionConfig
        if (pos?.mode === 'dynamic' && pos.subject) {
          const subject = resolveTemplate(pos.subject, dashboardStore.currentVariableValues)
          if (subject) {
            subManager.unsubscribe(widgetId, { type: 'subscription', subject })
          }
        }
      }
    }
    
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

  function unsubscribeAllWidgets(keepData: boolean = false) {
    for (const widget of dashboardStore.activeWidgets) {
      if (needsSubscription(widget.type, widget)) {
        unsubscribeWidget(widget.id, keepData)
      }
    }
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
    // createDefaultWidget is the ONLY source of a new widget's defaults.
    // A second applyWidgetDefaults used to run here and win on every
    // conflict, which quietly reverted the newer values above -- kvtable
    // lost its reported-state twin bucket, and button, switch and slider
    // lost the cmd.thing/twin_desired subjects for older placeholders.
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
