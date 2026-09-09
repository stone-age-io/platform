import { describe, expect, it } from 'vitest'

import {
  DEFAULT_WIDGET_SIZES,
  WIDGET_TYPES,
  createDefaultWidget,
  type WidgetType,
} from './dashboard'
import { TWIN_BUCKET } from '@/utils/twin'

const POSITION = { x: 0, y: 100 }

// Every widget type, from one list, so a new type cannot be tested by
// forgetting to test it.
const ALL: readonly WidgetType[] = WIDGET_TYPES

describe('createDefaultWidget', () => {
  // A widget the palette can add but that gets no defaults renders as an empty
  // body with no error -- the failure mode this whole file exists to catch,
  // since `vue-tsc && vite build` stays green through it.
  it.each(ALL)('produces a usable widget for %s', (type) => {
    const w = createDefaultWidget(type, POSITION)

    expect(w.type).toBe(type)
    expect(w.id).toMatch(/^widget_/)
    // markdown is deliberately untitled — it renders its own heading, so a
    // title bar above it is duplication. Every other type needs one, because a
    // widget with no title is unidentifiable on a grid of them.
    if (type !== 'markdown') {
      expect(w.title).toBeTruthy()
    }
    expect(w.w).toBe(DEFAULT_WIDGET_SIZES[type].w)
    expect(w.h).toBe(DEFAULT_WIDGET_SIZES[type].h)
    expect(w.x).toBe(POSITION.x)
    expect(w.y).toBe(POSITION.y)
  })

  // Each type's own config object has to be present, or its config modal opens
  // onto undefined and the widget cannot be set up at all.
  const CONFIG_KEY: Partial<Record<WidgetType, keyof ReturnType<typeof createDefaultWidget>>> = {
    chart: 'chartConfig',
    text: 'textConfig',
    button: 'buttonConfig',
    kv: 'kvConfig',
    switch: 'switchConfig',
    slider: 'sliderConfig',
    stat: 'statConfig',
    gauge: 'gaugeConfig',
    map: 'mapConfig',
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
    const w = createDefaultWidget(type, POSITION)
    expect(w[key!], `${type} has no ${String(key)}`).toBeDefined()
  })

  // The kvtable default is documented behaviour, not an arbitrary value: it
  // points at the reported-state twin bucket so the widget shows something the
  // moment it is added. This was silently overwritten with an empty bucket by a
  // second defaults function that ran afterwards.
  it('points a new kvtable at the reported-state twin bucket', () => {
    const w = createDefaultWidget('kvtable', POSITION)
    expect(w.kvtableConfig?.kvBucket).toBe(TWIN_BUCKET)
    expect(w.kvtableConfig?.keyPattern).toBe('thing.>')
  })

  // Types that read a value out of a payload need a data source and a path, or
  // the widget is added in a state that cannot resolve anything.
  it.each(['text', 'stat', 'gauge', 'chart'] as const)(
    '%s is created with a subscription data source',
    (type) => {
      const w = createDefaultWidget(type, POSITION)
      expect(w.dataSource?.type).toBe('subscription')
    },
  )

  // Buffered types must carry a bound. An unbounded buffer on a kiosk dashboard
  // is a slow memory leak.
  it.each(['console', 'streamtable'] as const)('%s is created with a bounded buffer', (type) => {
    const w = createDefaultWidget(type, POSITION)
    expect(w.buffer?.maxCount).toBeGreaterThan(0)
  })

  it('gives every type a distinct id', () => {
    const ids = new Set(ALL.map((t) => createDefaultWidget(t, POSITION).id))
    expect(ids.size).toBe(ALL.length)
  })
})
