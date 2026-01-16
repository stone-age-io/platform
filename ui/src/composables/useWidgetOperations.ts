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

  function subscribeWidget(widgetId: string) {
    const widget = dashboardStore.getWidget(widgetId)
    if (!widget) return
    
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
    const selfManagedTypes: WidgetType[] = ['button', 'kv', 'switch', 'slider', 'publisher', 'pocketbase'] 
    if (selfManagedTypes.includes(widgetType)) return false
    if (widgetType === 'status') return config?.dataSource?.type !== 'kv'
    if (widgetType === 'markdown') return !!(config?.dataSource?.subject)
    return true
  }

  function createWidget(type: WidgetType) {
    const position = { x: 0, y: 100 }
    const widget = createDefaultWidget(type, position)
    applyWidgetDefaults(widget, type)
    dashboardStore.addWidget(widget)
    if (natsStore.isConnected && needsSubscription(type, widget)) {
      subscribeWidget(widget.id)
    }
    return widget
  }

  function applyWidgetDefaults(widget: WidgetConfig, type: WidgetType) {
    switch (type) {
      case 'text':
        widget.title = 'Text Widget'
        widget.dataSource = { type: 'subscription', subject: 'test.subject' }
        widget.jsonPath = '$.value'
        break
      case 'chart':
        widget.title = 'Chart Widget'
        widget.dataSource = { type: 'subscription', subject: 'test.subject' }
        widget.jsonPath = '$.value'
        widget.chartConfig = { chartType: 'line' }
        break
      case 'button':
        widget.title = 'Button Widget'
        widget.buttonConfig = { label: 'Send', publishSubject: 'button.clicked', payload: '{"val": 1}' }
        break
      case 'kv':
        widget.title = 'KV Widget'
        widget.dataSource = { type: 'kv', kvBucket: 'my-bucket', kvKey: 'my-key' }
        break
      case 'switch':
        widget.title = 'Switch Control'
        break
      case 'slider':
        widget.title = 'Slider Control'
        break
      case 'stat':
        widget.title = 'Stat Card'
        widget.dataSource = { type: 'subscription', subject: 'metrics.value' }
        widget.jsonPath = '$.value'
        break
      case 'gauge':
        widget.title = 'Gauge Meter'
        widget.dataSource = { type: 'subscription', subject: 'sensor.value' }
        widget.jsonPath = '$.value'
        break
      case 'map':
        widget.title = 'Map Widget'
        widget.mapConfig = { center: { lat: 39.8283, lon: -98.5795 }, zoom: 4, markers: [] }
        break
      case 'console':
        widget.title = 'Console Stream'
        widget.dataSource = { type: 'subscription', subject: '>', subjects: ['>'] }
        widget.consoleConfig = { fontSize: 12, showTimestamp: true }
        widget.buffer.maxCount = 200
        break
      case 'publisher':
        widget.title = 'Publisher'
        widget.publisherConfig = { defaultSubject: 'test.subject', defaultPayload: '{\n  "action": "test"\n}', history: [], timeout: 2000 }
        break
      case 'status':
        widget.title = 'Status'
        widget.dataSource = { type: 'subscription', subject: 'service.status' }
        widget.statusConfig = {
          mappings: [
            { id: '1', value: 'online', color: 'var(--color-success)', label: 'Online', blink: false },
            { id: '2', value: 'error', color: 'var(--color-error)', label: 'Error', blink: true }
          ],
          defaultColor: 'var(--color-info)', defaultLabel: 'Unknown', showStale: true, stalenessThreshold: 60000, staleColor: 'var(--muted)', staleLabel: 'Stale'
        }
        break
      case 'markdown':
        widget.title = ''
        widget.dataSource = { type: 'subscription', subject: '' }
        widget.markdownConfig = { content: '### Hello World\n\nThis is a **markdown** widget.\n\nYou can use variables like {{device_id}}.' }
        break
      case 'pocketbase':
        widget.title = 'PocketBase Query'
        widget.pocketbaseConfig = {
          collection: 'audit_logs',
          limit: 10,
          sort: '-created',
          refreshInterval: 0
        }
        break
    }
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
