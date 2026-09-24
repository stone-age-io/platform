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

// Buffers are kept warm across navigation and variable changes
// (unsubscribeAllWidgets(true)). Two ways that went wrong: a variable change
// kept device A's data and appended device B's after it, and returning to a
// dashboard let the JetStream replay re-add everything the buffer still held.
describe('widgetData kept buffers', () => {
  beforeEach(() => setActivePinia(createPinia()))

  const js = (seq: number, subject = 'site.A.temp') =>
    ({ widgetId: 'w', value: seq, subject, seq, timestamp: 1000 + seq })

  it('keeps data when the widget resubscribes to the same subjects', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100, undefined, 'site.A.temp')
    store.batchAddMessages([js(1), js(2)])
    store.initializeBuffer('w', 100, undefined, 'site.A.temp')
    expect(store.getBuffer('w')).toHaveLength(2)
  })

  it('drops data that came from different subjects', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100, undefined, 'site.A.temp')
    store.batchAddMessages([js(1), js(2)])
    store.initializeBuffer('w', 100, undefined, 'site.B.temp')
    expect(store.getBuffer('w')).toEqual([])
  })

  it('skips a replayed message the buffer already holds, and keeps the new ones', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100)
    store.batchAddMessages([js(1), js(2), js(3)])
    // consumer recreated on return: replays 2..5
    store.batchAddMessages([js(2), js(3), js(4), js(5)])
    expect(store.getBuffer('w').map(m => m.seq)).toEqual([1, 2, 3, 4, 5])
  })

  // A sequence number is per stream, so the same number on another subject is
  // a different message.
  it('treats the same seq on another subject as a different message', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100)
    store.batchAddMessages([js(7, 'a'), js(7, 'b')])
    expect(store.getBuffer('w')).toHaveLength(2)
  })

  it('never de-duplicates core messages, which carry no seq', () => {
    const store = useWidgetDataStore()
    store.initializeBuffer('w', 100)
    const core = { widgetId: 'w', value: 1, subject: 's', timestamp: 5 }
    store.batchAddMessages([core, core])
    store.batchAddMessages([core])
    expect(store.getBuffer('w')).toHaveLength(3)
  })
})
