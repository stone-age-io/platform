import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useWidgetDataStore } from './widgetData'

// A time window was a `maxAge` that initializeBuffer stored and nothing ever
// read. These pin that it now ages messages out, and that it does so by
// TIMESTAMP rather than position: a timestamp may come from the payload or from
// JetStream, so arrival order is not time order and a front-trim would drop the
// wrong messages.

const NOW = Date.parse('2026-09-23T12:00:00Z')
const MIN = 60_000

function msg(widgetId: string, ageMs: number, value: number) {
  return { widgetId, value, timestamp: NOW - ageMs }
}

describe('widgetData time window', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
  })
  afterEach(() => vi.useRealTimers())

  it('drops messages older than maxAge', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100, 10 * MIN)
    store.batchAddMessages([msg('w', 20 * MIN, 1), msg('w', 5 * MIN, 2), msg('w', 0, 3)])
    expect(store.getBuffer('w').map(m => m.value)).toEqual([2, 3])
  })

  it('keeps an in-window message that arrived after a newer one', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100, 10 * MIN)
    store.batchAddMessages([msg('w', 1 * MIN, 1)])
    store.batchAddMessages([msg('w', 15 * MIN, 2), msg('w', 8 * MIN, 3)])
    expect(store.getBuffer('w').map(m => m.value)).toEqual([1, 3])
  })

  it('ages out what is already buffered as time passes', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100, 10 * MIN)
    store.batchAddMessages([msg('w', 0, 1)])
    vi.setSystemTime(NOW + 11 * MIN)
    store.batchAddMessages([{ widgetId: 'w', value: 2, timestamp: NOW + 11 * MIN }])
    expect(store.getBuffer('w').map(m => m.value)).toEqual([2])
  })

  it('keeps everything, however old, without maxAge', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100)
    store.batchAddMessages([msg('w', 1000 * MIN, 1), msg('w', 0, 2)])
    expect(store.getBuffer('w').map(m => m.value)).toEqual([1, 2])
  })
})
