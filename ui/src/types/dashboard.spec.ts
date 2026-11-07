import { describe, expect, it } from 'vitest'

import {
  DEFAULT_WIDGET_SIZES,
  WIDGET_TYPES,
  createDefaultWidget,
  type WidgetType,
} from './dashboard'
import { TWIN_BUCKET } from '@/utils/twin'

const POSITION = { x: 0, y: 100 }

// Every widget type, so a new one is tested automatically.
const ALL: readonly WidgetType[] = WIDGET_TYPES

describe('createDefaultWidget', () => {
  // A widget with no defaults renders an empty body with no error, and
  // `vue-tsc && vite build` stays green.
  it.each(ALL)('produces a usable widget for %s', (type) => {
    const w = createDefaultWidget(type, POSITION)

    expect(w.type).toBe(type)
    expect(w.id).toMatch(/^widget_/)
    // markdown is deliberately untitled; it renders its own heading.
    if (type !== 'markdown') {
      expect(w.title).toBeTruthy()
    }
    expect(w.w).toBe(DEFAULT_WIDGET_SIZES[type].w)
    expect(w.h).toBe(DEFAULT_WIDGET_SIZES[type].h)
    expect(w.x).toBe(POSITION.x)
    expect(w.y).toBe(POSITION.y)
  })

  // Without its config object a widget's config modal opens onto undefined. The
  // compiler checks WIDGET_DEFAULTS; this is the runtime half. The keys are plain
  // strings because `keyof` a union is only its shared keys.
  const CONFIG_KEY: Record<WidgetType, string> = {
    chart: 'chartConfig',
    text: 'textConfig',
    button: 'buttonConfig',
    buttonpad: 'buttonpadConfig',
    kv: 'kvConfig',
    switch: 'switchConfig',
    slider: 'sliderConfig',
    stat: 'statConfig',
    gauge: 'gaugeConfig',
    map: 'mapConfig',
    floorplan: 'floorplanConfig',
    console: 'consoleConfig',
    publisher: 'publisherConfig',
    status: 'statusConfig',
    markdown: 'markdownConfig',
    kvtable: 'kvtableConfig',
    streamtable: 'streamtableConfig',
    scanner: 'scannerConfig',
  }

  it.each(ALL)('gives %s its own config object', (type) => {
    const key = CONFIG_KEY[type]
    expect(key, `no config key mapped for ${type}`).toBeTruthy()
    const w = createDefaultWidget(type, POSITION) as unknown as Record<string, unknown>
    expect(w[key], `${type} has no ${key}`).toBeDefined()
  })

  // Documented behaviour: a new kvtable shows something the moment it is added.
  it('points a new kvtable at the reported-state twin bucket', () => {
    const w = createDefaultWidget('kvtable', POSITION)
    expect(w.kvtableConfig?.kvBucket).toBe(TWIN_BUCKET)
    expect(w.kvtableConfig?.keyPattern).toBe('thing.>')
  })

  // Types that read a payload value need a data source to resolve anything.
  it.each(['text', 'stat', 'gauge', 'chart'] as const)(
    '%s is created with a subscription data source',
    (type) => {
      const w = createDefaultWidget(type, POSITION)
      expect(w.dataSource?.type).toBe('subscription')
    },
  )

  // Series read the whole payload; a widget-level jsonPath would pre-extract it
  // and the chart would start empty.
  it('creates a chart with one series and no widget-level jsonPath', () => {
    const w = createDefaultWidget('chart', POSITION)
    expect(w.jsonPath).toBeUndefined()
    expect(w.chartConfig?.series).toEqual([{ label: 'Value', path: '$.value' }])
  })

  // An unbounded buffer on a kiosk dashboard is a slow memory leak.
  it.each(['console', 'streamtable'] as const)('%s is created with a bounded buffer', (type) => {
    const w = createDefaultWidget(type, POSITION)
    expect(w.buffer?.maxCount).toBeGreaterThan(0)
  })

  it('gives every type a distinct id', () => {
    const ids = new Set(ALL.map((t) => createDefaultWidget(t, POSITION).id))
    expect(ids.size).toBe(ALL.length)
  })
})
