import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useNatsStore } from '@/stores/nats'
import { useWidgetDataStore } from '@/stores/widgetData'
import { useSubscriptionManager } from './useSubscriptionManager'

// Every live value on every dashboard flows through this module, and it was
// wholly untested: refcounted listeners, a shared key for core subjects, two
// different transports, and reconnect handling. `vue-tsc && vite build` stays
// green through any bug in here.
//
// Node environment, so `window` does not exist -- the manager attaches its
// reconnect/close listeners to it. A stub is enough: nothing here asserts on
// event delivery, only that subscribing does not require a DOM.
const listeners = new Map<string, Set<(e: unknown) => void>>()
function stubWindow() {
  listeners.clear()
  ;(globalThis as any).window = {
    addEventListener(type: string, fn: (e: unknown) => void) {
      if (!listeners.has(type)) listeners.set(type, new Set())
      listeners.get(type)!.add(fn)
    },
    removeEventListener(type: string, fn: (e: unknown) => void) {
      listeners.get(type)?.delete(fn)
    },
    dispatchEvent() {
      return true
    },
  }
}

// A core subscription is an async iterable that can be closed. That is the
// whole surface the manager uses, so it is the whole surface the fake needs.
function fakeSubscription() {
  let closed = false
  return {
    unsubscribeCalls: 0,
    isClosed: () => closed,
    unsubscribe() {
      this.unsubscribeCalls++
      closed = true
    },
    async *[Symbol.asyncIterator]() {
      // Never yields, never returns: a live subscription with no traffic. The
      // manager consumes this in a detached loop, so it must not be awaited.
      await new Promise(() => {})
    },
  }
}

function fakeConnection() {
  const subs: ReturnType<typeof fakeSubscription>[] = []
  return {
    subs,
    subscribeCalls: [] as string[],
    subscribe(subject: string) {
      this.subscribeCalls.push(subject)
      const s = fakeSubscription()
      subs.push(s)
      return s
    },
  }
}

function setup() {
  setActivePinia(createPinia())
  stubWindow()
  const nc = fakeConnection()
  const nats = useNatsStore()
  nats.nc = nc as never
  return { nc, manager: useSubscriptionManager() }
}

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('useSubscriptionManager', () => {
  it('opens one NATS subscription for two widgets on the same subject', async () => {
    const { nc, manager } = setup()
    const config = { type: 'subscription' as const, subject: 'sensor.temp' }

    await manager.subscribe('widget-a', config)
    await manager.subscribe('widget-b', config)

    // Core subjects are keyed by subject alone, deliberately: two widgets
    // watching the same subject should share one server-side subscription.
    expect(nc.subscribeCalls).toEqual(['sensor.temp'])
    expect(manager.isSubscribed('widget-a', config)).toBe(true)
    expect(manager.isSubscribed('widget-b', config)).toBe(true)
  })

  it('keeps the subscription open until the LAST widget leaves', async () => {
    const { nc, manager } = setup()
    const config = { type: 'subscription' as const, subject: 'sensor.temp' }

    await manager.subscribe('widget-a', config)
    await manager.subscribe('widget-b', config)

    manager.unsubscribe('widget-a', config)
    expect(nc.subs[0].unsubscribeCalls).toBe(0)
    expect(manager.isSubscribed('widget-b', config)).toBe(true)

    manager.unsubscribe('widget-b', config)
    expect(nc.subs[0].unsubscribeCalls).toBe(1)
    expect(manager.isSubscribed('widget-b', config)).toBe(false)
  })

  it('does not close a shared subscription twice', async () => {
    const { nc, manager } = setup()
    const config = { type: 'subscription' as const, subject: 'sensor.temp' }

    await manager.subscribe('widget-a', config)
    manager.unsubscribe('widget-a', config)
    manager.unsubscribe('widget-a', config)

    expect(nc.subs[0].unsubscribeCalls).toBe(1)
  })

  it('opens separate subscriptions for different subjects', async () => {
    const { nc, manager } = setup()

    await manager.subscribe('a', { type: 'subscription', subject: 'sensor.temp' })
    await manager.subscribe('b', { type: 'subscription', subject: 'sensor.humidity' })

    expect(nc.subscribeCalls).toEqual(['sensor.temp', 'sensor.humidity'])
  })

  it('does nothing when there is no connection', async () => {
    setActivePinia(createPinia())
    stubWindow()
    const manager = useSubscriptionManager()
    const config = { type: 'subscription' as const, subject: 'sensor.temp' }

    await manager.subscribe('widget-a', config)
    expect(manager.isSubscribed('widget-a', config)).toBe(false)
  })

  it('ignores a data source with no subject rather than subscribing to undefined', async () => {
    const { nc, manager } = setup()
    await manager.subscribe('widget-a', { type: 'subscription' })
    expect(nc.subscribeCalls).toEqual([])
  })

  it('closes everything on cleanupAll', async () => {
    const { nc, manager } = setup()

    await manager.subscribe('a', { type: 'subscription', subject: 'one' })
    await manager.subscribe('b', { type: 'subscription', subject: 'two' })
    manager.cleanupAll()

    expect(nc.subs.map((s) => s.unsubscribeCalls)).toEqual([1, 1])
  })
})

// Every widget that reads a field out of a payload goes through this, so a
// change in its behaviour changes what every dashboard displays.
describe('extractJsonPath', () => {
  it('reads a value at a JSON path', () => {
    const { manager } = setup()
    expect(manager.extractJsonPath({ value: 21.5 }, '$.value')).toBe(21.5)
    expect(manager.extractJsonPath({ a: { b: 'x' } }, '$.a.b')).toBe('x')
  })

  it('returns the whole payload when no path is given', () => {
    const { manager } = setup()
    const payload = { value: 1 }
    expect(manager.extractJsonPath(payload, undefined)).toEqual(payload)
    expect(manager.extractJsonPath(payload, '')).toEqual(payload)
  })

  it('returns undefined for a path that does not resolve', () => {
    const { manager } = setup()
    expect(manager.extractJsonPath({ value: 1 }, '$.missing')).toBeUndefined()
  })

  it('does not throw on a malformed path', () => {
    const { manager } = setup()
    expect(() => manager.extractJsonPath({ value: 1 }, '$[')).not.toThrow()
  })
})

// The filter lives on the LISTENER, because core subscriptions are shared by
// subject: move it onto the subscription and two widgets on `>` would silently
// take whichever setting subscribed first.
describe('hideSystemSubjects', () => {
  const TRAFFIC = ['$JS.API.CONSUMER.INFO.x', '_INBOX.abc.1', 'sensor.temp', '$KV.twin.thing.a.temp']

  // A subscription that delivers TRAFFIC once released: the gate lets every
  // widget attach before the first message arrives.
  function trafficSetup() {
    setActivePinia(createPinia())
    stubWindow()
    vi.stubGlobal('requestAnimationFrame', (fn: (t: number) => void) => { fn(0); return 0 })
    let release!: () => void
    const gate = new Promise<void>((r) => { release = r })
    const data = new TextEncoder().encode('{"v":1}')
    const nc = {
      subscribeCalls: [] as string[],
      subscribe(subject: string) {
        this.subscribeCalls.push(subject)
        return {
          isClosed: () => false,
          unsubscribe() {},
          async *[Symbol.asyncIterator]() {
            await gate
            for (const s of TRAFFIC) yield { subject: s, data }
            await new Promise(() => {})
          },
        }
      },
    }
    useNatsStore().nc = nc as never
    const added: Array<{ widgetId: string; subject?: string }> = []
    vi.spyOn(useWidgetDataStore(), 'batchAddMessages').mockImplementation((items) => { added.push(...items) })
    const deliver = async () => { release(); await new Promise((r) => setTimeout(r, 0)) }
    const seenBy = (id: string) => added.filter((m) => m.widgetId === id).map((m) => m.subject)
    return { nc, manager: useSubscriptionManager(), deliver, seenBy }
  }

  afterEach(() => { vi.unstubAllGlobals() })

  it('filters per widget on one shared subscription', async () => {
    const { nc, manager, deliver, seenBy } = trafficSetup()
    await manager.subscribe('quiet', { type: 'subscription', subject: '>', hideSystemSubjects: true })
    await manager.subscribe('raw', { type: 'subscription', subject: '>' })
    await deliver()

    expect(nc.subscribeCalls).toEqual(['>'])
    expect(seenBy('quiet')).toEqual(['sensor.temp'])
    expect(seenBy('raw')).toEqual(TRAFFIC)
  })

  it('never filters a subscription whose first token is literal', async () => {
    const { manager, deliver, seenBy } = trafficSetup()
    await manager.subscribe('twin', { type: 'subscription', subject: '$KV.twin.>', hideSystemSubjects: true })
    await deliver()

    // The fake ignores the subject and delivers everything; what matters is
    // that asking for a `$` prefix by name is never second-guessed.
    expect(seenBy('twin')).toEqual(TRAFFIC)
  })
})
