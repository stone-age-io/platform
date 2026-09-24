import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { DataSourceConfig, WidgetConfig, Dashboard } from '@/types/dashboard'

// Unsubscribing used to re-derive subjects from the widget config and the
// CURRENT variable values, but both unsubscribe paths run after what they read
// has already changed:
//   - a variable change unsubscribed from the NEW subject and left the old one
//     feeding the widget, so a chart showed device A and B at once;
//   - a dashboard switch walked `activeWidgets`, already the NEW dashboard's,
//     so the old dashboard's widgets were never unsubscribed;
//   - map markers dropped `useJetStream` and missed their own key.
// The manager is faked: the question is what useWidgetOperations asks of it.

const live = new Map<string, Set<string>>() // widgetId -> subscription keys
const key = (c: DataSourceConfig) => `${c.useJetStream ? 'js' : 'core'}:${c.subject}`

vi.mock('@/composables/useSubscriptionManager', () => ({
  getSubscriptionManager: () => ({
    subscribe: (id: string, c: DataSourceConfig) => {
      if (!live.has(id)) live.set(id, new Set())
      live.get(id)!.add(key(c))
    },
    unsubscribe: (id: string, c: DataSourceConfig) => {
      live.get(id)?.delete(key(c))
    },
  }),
}))

const { useWidgetOperations } = await import('./useWidgetOperations')
const { useDashboardStore } = await import('@/stores/dashboard')

let n = 0
function chart(subject: string, extra: Partial<WidgetConfig> = {}): WidgetConfig {
  return {
    id: `w${++n}`, type: 'chart', title: 't', x: 0, y: 0, w: 4, h: 4,
    dataSource: { type: 'subscription', subject, subjects: [subject] },
    buffer: { maxCount: 10 },
    chartConfig: { chartType: 'line' },
    ...extra,
  }
}

function dashboard(widgets: WidgetConfig[]): Dashboard {
  return { id: `d${++n}`, name: 'd', created: 0, modified: 0, widgets }
}

function setup(widgets: WidgetConfig[], vars: Record<string, string> = {}) {
  setActivePinia(createPinia())
  ;(globalThis as any).localStorage = { getItem: () => null, setItem() {}, removeItem() {} }
  const store = useDashboardStore()
  store.activeDashboard = dashboard(widgets)
  store.currentVariableValues = vars
  return { store, ops: useWidgetOperations() }
}

const subs = (id: string) => [...(live.get(id) ?? [])].sort()

beforeEach(() => live.clear())

describe('useWidgetOperations', () => {
  it('a variable change moves the subscription instead of adding one', () => {
    const w = chart('site.{{device}}.temp')
    const { store, ops } = setup([w], { device: 'A' })
    ops.subscribeAllWidgets()
    expect(subs(w.id)).toEqual(['core:site.A.temp'])

    // exactly what VisualizerView's variable watcher does, after the change
    store.currentVariableValues = { device: 'B' }
    ops.unsubscribeAllWidgets(true)
    ops.subscribeAllWidgets()
    expect(subs(w.id)).toEqual(['core:site.B.temp'])
  })

  it('resubscribing without an unsubscribe still drops the old subject', () => {
    const w = chart('site.{{device}}.temp')
    const { store, ops } = setup([w], { device: 'A' })
    ops.subscribeWidget(w.id)
    store.currentVariableValues = { device: 'B' }
    ops.subscribeWidget(w.id)
    expect(subs(w.id)).toEqual(['core:site.B.temp'])
  })

  it('a dashboard switch unsubscribes the dashboard being left', () => {
    const a = chart('a.temp')
    const { store, ops } = setup([a])
    ops.subscribeAllWidgets()

    // the watcher fires once activeWidgets is already the new dashboard's
    const b = chart('b.temp')
    store.activeDashboard = dashboard([b])
    ops.unsubscribeAllWidgets(true)
    ops.subscribeAllWidgets()

    expect(subs(a.id)).toEqual([])
    expect(subs(b.id)).toEqual(['core:b.temp'])
  })

  it('removes a JetStream map marker subscription by its own key', () => {
    const m: WidgetConfig = {
      ...chart(''), type: 'map', dataSource: { type: 'subscription' },
      mapConfig: { center: { lat: 0, lon: 0 }, zoom: 1, markers: [
        { id: 'k', label: 'k', positionConfig: { mode: 'dynamic', subject: 'gps.{{device}}', useJetStream: true } },
      ] } as any,
    }
    const { store, ops } = setup([m], { device: 'A' })
    ops.subscribeWidget(m.id)
    expect(subs(m.id)).toEqual(['js:gps.A'])
    store.currentVariableValues = { device: 'B' }
    ops.unsubscribeWidget(m.id)
    expect(subs(m.id)).toEqual([])
  })

  it('unsubscribes a widget that has already been deleted', () => {
    const w = chart('x')
    const { store, ops } = setup([w])
    ops.subscribeWidget(w.id)
    store.activeDashboard = dashboard([])
    ops.unsubscribeWidget(w.id)
    expect(subs(w.id)).toEqual([])
  })
})
