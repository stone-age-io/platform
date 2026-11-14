import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createDefaultWidget } from '@/types/dashboard'
import { decodeBytes, encodeString } from '@/utils/encoding'
import { useDashboardStore } from './dashboard'
import { useNatsStore } from './nats'

// An in-memory KV bucket: enough of @nats-io/kv for the shared-dashboard paths.
// Only the KV tests below reach it; the rest never connect.
const fakeKv = vi.hoisted(() => {
  const entries = new Map<string, { value: Uint8Array; revision: number }>()
  const listeners = new Set<(e: { key: string; revision: number }) => void>()
  let revision = 0
  const kv = {
    async keys() { return (async function* () { yield* [...entries.keys()] })() },
    async get(key: string) { return entries.get(key) ?? null },
    async put(key: string, value: Uint8Array, opts?: { previousSeq?: number }) {
      const current = entries.get(key)?.revision ?? 0
      if (opts?.previousSeq !== undefined && opts.previousSeq !== current) throw new Error('wrong last sequence')
      revision++
      entries.set(key, { value, revision })
      for (const l of listeners) l({ key, revision })
      return revision
    },
    async delete(key: string) { entries.delete(key) },
    async watch({ key }: { key: string }) {
      const queue: { key: string; revision: number }[] = []
      let wake: (() => void) | null = null
      let stopped = false
      const listener = (e: { key: string; revision: number }) => {
        if (e.key === key) { queue.push(e); wake?.() }
      }
      listeners.add(listener)
      return {
        stop() { stopped = true; listeners.delete(listener); wake?.() },
        async *[Symbol.asyncIterator]() {
          while (!stopped) {
            if (queue.length) yield queue.shift()!
            else await new Promise<void>(r => { wake = r })
          }
        },
      }
    },
  }
  return { kv, entries, reset() { entries.clear(); listeners.clear(); revision = 0 } }
})

vi.mock('@nats-io/kv', () => ({
  Kvm: class { open() { return Promise.resolve(fakeKv.kv) } },
}))

// A bad import silently loses work: `replace` clears every local dashboard
// before writing. localStorage is stubbed, not absent, because saveToStorage
// swallows its errors and the tests would otherwise only cover the failure path.
function stubLocalStorage() {
  const store = new Map<string, string>()
  ;(globalThis as any).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
    key: (i: number) => [...store.keys()][i] ?? null,
    get length() {
      return store.size
    },
  }
  return store
}

function freshStore() {
  setActivePinia(createPinia())
  stubLocalStorage()
  const store = useDashboardStore()
  store.localDashboards = []
  store.activeDashboard = null
  return store
}

// A dashboard with one real widget, so the round-trip has widget config to lose.
function seedDashboard(store: ReturnType<typeof freshStore>, name: string) {
  const dashboard = store.createDashboard(name)
  if (!dashboard) throw new Error('createDashboard returned null')
  dashboard.widgets = [createDefaultWidget('kvtable', { x: 0, y: 0 })]
  return dashboard
}

beforeEach(() => {
  freshStore()
})

describe('dashboard export/import round trip', () => {
  it('preserves the name and the widgets', () => {
    const store = freshStore()
    const original = seedDashboard(store, 'Cold Chain')
    const json = store.exportDashboards([original.id])

    const reimported = freshStore()
    const result = reimported.importDashboards(json)

    expect(result.errors).toEqual([])
    expect(result.success).toBe(1)
    expect(reimported.localDashboards).toHaveLength(1)

    const got = reimported.localDashboards[0]
    expect(got.name).toBe('Cold Chain')
    expect(got.widgets).toHaveLength(1)
    // Widget config must survive, or the import looks right and shows nothing.
    const imported = got.widgets[0]
    const seeded = original.widgets[0]
    expect(imported.type).toBe('kvtable')
    // Narrows the WidgetConfig union for the compiler; the expect above is the test.
    if (imported.type !== 'kvtable' || seeded.type !== 'kvtable') throw new Error('not a kvtable widget')
    expect(imported.kvtableConfig.kvBucket).toBe(seeded.kvtableConfig.kvBucket)
  })

  it('assigns new ids, so importing the same file twice does not collide', () => {
    const store = freshStore()
    const original = seedDashboard(store, 'Twice')
    const json = store.exportDashboards([original.id])

    const target = freshStore()
    target.importDashboards(json)
    target.importDashboards(json)

    expect(target.localDashboards).toHaveLength(2)
    const [a, b] = target.localDashboards
    expect(a.id).not.toBe(b.id)
    expect(a.id).not.toBe(original.id)
    expect(a.widgets[0].id).not.toBe(b.widgets[0].id)
  })

  // storage / kvKey / kvRevision say where a dashboard lived; an imported copy
  // must not claim a KV key it does not own.
  it('strips the storage location from the exported file', () => {
    const store = freshStore()
    const dashboard = seedDashboard(store, 'Local One')
    const parsed = JSON.parse(store.exportDashboards([dashboard.id]))

    expect(parsed.dashboards[0]).not.toHaveProperty('storage')
    expect(parsed.dashboards[0]).not.toHaveProperty('kvKey')
    expect(parsed.dashboards[0]).not.toHaveProperty('kvRevision')
  })

  it('marks everything it imports as local', () => {
    const store = freshStore()
    const dashboard = seedDashboard(store, 'Local One')
    const json = store.exportDashboards([dashboard.id])

    const target = freshStore()
    target.importDashboards(json)
    expect(target.localDashboards[0].storage).toBe('local')
  })

  it('exports every local dashboard with exportAllDashboards', () => {
    const store = freshStore()
    seedDashboard(store, 'One')
    seedDashboard(store, 'Two')

    const parsed = JSON.parse(store.exportAllDashboards())
    expect(parsed.dashboards.map((d: { name: string }) => d.name)).toEqual(['One', 'Two'])
  })
})

describe('dashboard import safety', () => {
  it('reports a parse failure instead of throwing', () => {
    const store = freshStore()
    const result = store.importDashboards('{ not json')

    expect(result.success).toBe(0)
    expect(result.errors.length).toBeGreaterThan(0)
    expect(store.localDashboards).toHaveLength(0)
  })

  it('rejects a file that is not an export file', () => {
    const store = freshStore()
    const result = store.importDashboards(JSON.stringify({ dashboards: [] }))
    expect(result.errors.length).toBeGreaterThan(0)
  })

  it('skips a malformed dashboard without losing the good ones', () => {
    const store = freshStore()
    const json = JSON.stringify({
      version: '1.0',
      dashboards: [
        { name: 'Good', widgets: [] },
        { name: 'No widgets array' },
        { widgets: [] },
      ],
    })

    const result = store.importDashboards(json)
    expect(result.success).toBe(1)
    expect(result.skipped).toBe(2)
    expect(store.localDashboards.map((d) => d.name)).toEqual(['Good'])
  })

  // The limit is checked before anything is written, so an import never half-applies.
  it('refuses an import that would exceed the limit, and writes nothing', () => {
    const store = freshStore()
    const many = Array.from({ length: 30 }, (_, i) => ({ name: `d${i}`, widgets: [] }))
    const result = store.importDashboards(JSON.stringify({ version: '1.0', dashboards: many }))

    expect(result.success).toBe(0)
    expect(result.errors.length).toBeGreaterThan(0)
    expect(store.localDashboards).toHaveLength(0)
  })

  it('replace clears what was there; merge keeps it', () => {
    const merge = freshStore()
    seedDashboard(merge, 'Existing')
    const incoming = JSON.stringify({ version: '1.0', dashboards: [{ name: 'Incoming', widgets: [] }] })

    merge.importDashboards(incoming, 'merge')
    expect(merge.localDashboards.map((d) => d.name)).toEqual(['Existing', 'Incoming'])

    const replace = freshStore()
    seedDashboard(replace, 'Existing')
    replace.importDashboards(incoming, 'replace')
    expect(replace.localDashboards.map((d) => d.name)).toEqual(['Incoming'])
  })

  it('imports a single dashboard file too, and re-ids it', () => {
    const store = freshStore()
    const dashboard = seedDashboard(store, 'Single')
    const json = JSON.stringify(dashboard)

    const target = freshStore()
    const got = target.importDashboard(json)

    expect(got).not.toBeNull()
    expect(got!.name).toBe('Single')
    expect(got!.id).not.toBe(dashboard.id)
    expect(got!.storage).toBe('local')
  })

  it('returns null rather than throwing on a bad single-dashboard file', () => {
    const store = freshStore()
    expect(store.importDashboard('{ not json')).toBeNull()
    expect(store.importDashboard(JSON.stringify({ name: 'no widgets' }))).toBeNull()
    expect(store.localDashboards).toHaveLength(0)
  })
})

// A KV dashboard belongs to the organization being left.
describe('leaveRemoteDashboard', () => {
  it('drops an active KV dashboard for a local one', () => {
    const store = freshStore()
    const local = seedDashboard(store, 'Local')
    store.remoteKeys = ['org-a-dash']
    store.setActiveDashboard({ ...seedDashboard(store, 'Remote'), storage: 'kv', kvKey: 'org-a-dash' })

    store.leaveRemoteDashboard()

    expect(store.activeDashboard?.id).toBe(local.id)
    expect(store.activeDashboard?.storage).not.toBe('kv')
    expect(store.remoteKeys).toEqual([])
  })

  it('leaves an active local dashboard alone', () => {
    const store = freshStore()
    seedDashboard(store, 'First')
    const second = seedDashboard(store, 'Second')
    store.setActiveDashboard(second)

    store.leaveRemoteDashboard()

    expect(store.activeDashboard?.id).toBe(second.id)
  })
})

describe('which dashboard opens on load', () => {
  // A fresh pinia is a fresh app load: loadFromStorage's first call is the
  // initial one.
  function reload(storage: Map<string, string>) {
    setActivePinia(createPinia())
    ;(globalThis as any).localStorage.getItem = (k: string) => storage.get(k) ?? null
    const store = useDashboardStore()
    store.loadFromStorage()
    return store
  }

  function saved(local: { id: string; name: string }[], settings: Record<string, unknown>) {
    const storage = stubLocalStorage()
    storage.set('nats_dashboards', JSON.stringify({ dashboards: local.map(d => ({ ...d, widgets: [] })) }))
    storage.set('nats_dashboard_settings', JSON.stringify(settings))
    return storage
  }

  it('reopens the local dashboard that was open last', () => {
    const storage = saved([{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }], {
      lastSelectedDashboard: { id: 'b', storage: 'local' },
    })
    expect(reload(storage).activeDashboard?.id).toBe('b')
  })

  it('leaves a shared last dashboard for the NATS watcher, rather than opening a local one over it', () => {
    const storage = saved([{ id: 'a', name: 'A' }], {
      lastSelectedDashboard: { id: 'northwind', storage: 'kv' },
    })
    const store = reload(storage)
    // No local dashboard opens, so lastSelected still points the watcher at the shared one.
    expect(store.activeDashboard).toBeNull()
    expect(JSON.parse(storage.get('nats_dashboard_settings')!).lastSelectedDashboard)
      .toEqual({ id: 'northwind', storage: 'kv' })
  })

  it('falls back to the first dashboard when the last one was deleted', () => {
    const storage = saved([{ id: 'a', name: 'A' }], {
      lastSelectedDashboard: { id: 'gone', storage: 'local' },
    })
    expect(reload(storage).activeDashboard?.id).toBe('a')
  })

  it('lets a pinned startup dashboard win over the last one', () => {
    const storage = saved([{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }], {
      startupDashboard: { id: 'a', storage: 'local' },
      lastSelectedDashboard: { id: 'b', storage: 'local' },
    })
    expect(reload(storage).activeDashboard?.id).toBe('a')
  })
})

// Locking is an editing mode, not saved: a saved lock let one editor's Save
// leave a shared dashboard unlocked on every wall display showing it.
describe('locking', () => {
  it('opens every existing dashboard locked, whatever an older file says', () => {
    const store = freshStore()
    const d = store.createDashboard('Old file')!
    d.isLocked = false
    store.setActiveDashboard(d)
    expect(store.isLocked).toBe(true)
  })

  it('opens a dashboard you just created unlocked, to be built', () => {
    const store = freshStore()
    store.createDashboard('New one')
    expect(store.isLocked).toBe(false)
  })

  it('locks again when a dashboard is reopened', () => {
    const store = freshStore()
    const d = store.createDashboard('One')!
    store.setActiveDashboard(d)
    store.toggleLock()
    expect(store.isLocked).toBe(false)
    store.setActiveDashboard(d)
    expect(store.isLocked).toBe(true)
  })

  it('does not mark a shared dashboard as unsaved', () => {
    const store = freshStore()
    store.setActiveDashboard({ ...store.createDashboard('Shared one')!, storage: 'kv', kvKey: 'shared-one' })
    store.toggleLock()
    expect(store.isLocked).toBe(false)
    expect(store.isDirty).toBe(false)
  })

  it('writes no lock to local storage or to an export', () => {
    const storage = stubLocalStorage()
    const store = freshStore()
    ;(globalThis as any).localStorage.getItem = (k: string) => storage.get(k) ?? null
    ;(globalThis as any).localStorage.setItem = (k: string, v: string) => void storage.set(k, v)
    const d = store.createDashboard('Local one')!
    d.isLocked = true // as an older version left it
    store.toggleLock()
    store.saveToStorage()
    const saved = JSON.parse(storage.get('nats_dashboards')!).dashboards.find((x: any) => x.id === d.id)
    expect(saved).not.toHaveProperty('isLocked')
    expect(JSON.parse(store.exportDashboards([d.id])).dashboards[0]).not.toHaveProperty('isLocked')
  })
})

describe('shared dashboards', () => {
  // A connected store over an empty fake bucket.
  function connected() {
    const store = freshStore()
    fakeKv.reset()
    const nats = useNatsStore()
    nats.nc = {} as any
    nats.status = 'connected'
    return store
  }

  async function putShared(key: string, doc: Record<string, unknown>) {
    await fakeKv.kv.put(key, encodeString(JSON.stringify({ widgets: [], ...doc })))
  }

  function stored(key: string) {
    return JSON.parse(decodeBytes(fakeKv.entries.get(key)!.value))
  }

  // The key ends in a random suffix; the sidebar shows the name inside.
  it('names each shared dashboard by its own name, not its key', async () => {
    const store = connected()
    await putShared('ops.lobby-x7k2p9', { id: 'a', name: 'Lobby' })
    await putShared('untitled-q1w2e3', { id: 'b', name: '' })

    await store.refreshRemoteKeys()

    expect(store.remoteKeys.sort()).toEqual(['ops.lobby-x7k2p9', 'untitled-q1w2e3'])
    expect(store.remoteNames).toEqual({ 'ops.lobby-x7k2p9': 'Lobby' })
  })

  // Sharing used to copy the local dashboard's ids, so the two copies were
  // indistinguishable and switching between them rebuilt nothing.
  it('gives a shared copy its own dashboard and widget ids', async () => {
    const store = connected()
    const local = seedDashboard(store, 'TV remote')

    await store.uploadLocalToRemote(local.id, 'home', 'TV remote')

    const shared = store.activeDashboard!
    expect(shared.storage).toBe('kv')
    expect(shared.id).not.toBe(local.id)
    expect(shared.widgets[0].id).not.toBe(local.widgets[0].id)
    expect(stored(shared.kvKey!).id).toBe(shared.id)
    expect(stored(shared.kvKey!)).not.toHaveProperty('isLocked')
    // The local one is untouched.
    expect(store.localDashboards[0].id).toBe(local.id)
  })

  it('re-ids a shared copy that still carries its local source ids', async () => {
    const store = connected()
    const local = seedDashboard(store, 'Old share')
    await putShared('old-share-abc123', { ...JSON.parse(JSON.stringify(local)), storage: undefined })

    await store.loadRemoteDashboard('old-share-abc123')

    expect(store.activeDashboard!.kvKey).toBe('old-share-abc123')
    expect(store.activeDashboard!.id).not.toBe(local.id)
    expect(store.activeDashboard!.widgets[0].id).not.toBe(local.widgets[0].id)
  })

  // The grid and the subscriptions key on this; a reload keeps the id.
  it('counts every open, including reopening the same shared dashboard', async () => {
    const store = connected()
    const local = seedDashboard(store, 'Local')
    await putShared('shared-1', { id: 'remote-1', name: 'Shared' })

    store.setActiveDashboard(local)
    const a = store.activeLoadId
    await store.loadRemoteDashboard('shared-1')
    const b = store.activeLoadId
    await store.loadRemoteDashboard('shared-1')
    const c = store.activeLoadId
    store.setActiveDashboard(local)

    expect(b).toBeGreaterThan(a)
    expect(c).toBeGreaterThan(b)
    expect(store.activeLoadId).toBeGreaterThan(c)
  })

  it('does not report its own save as somebody else’s change', async () => {
    const store = connected()
    await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
    await store.loadRemoteDashboard('shared-1')
    await new Promise(r => setTimeout(r, 0)) // let the watcher start

    store.renameDashboard('remote-1', 'Renamed')
    await store.saveRemoteDashboard()
    await new Promise(r => setTimeout(r, 0))
    expect(store.remoteChanged).toBe(false)

    // Someone else's save still shows.
    await putShared('shared-1', { id: 'remote-1', name: 'From elsewhere' })
    await new Promise(r => setTimeout(r, 0))
    expect(store.remoteChanged).toBe(true)
  })

  // The Save button reports what this returns, so a lost conflict must not
  // read as success.
  it('reports a revision conflict as a failed save', async () => {
    const store = connected()
    await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
    await store.loadRemoteDashboard('shared-1')
    await putShared('shared-1', { id: 'remote-1', name: 'From elsewhere' })

    store.renameDashboard('remote-1', 'Mine')
    expect(await store.saveRemoteDashboard()).toBe(false)

    expect(store.storageError).toMatch(/Conflict/)
    expect(store.remoteChanged).toBe(true)
    expect(stored('shared-1').name).toBe('From elsewhere')
  })

  it('reports a clean save as success and clears an earlier error', async () => {
    const store = connected()
    await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
    await store.loadRemoteDashboard('shared-1')
    store.storageError = 'Save failed: earlier'

    store.renameDashboard('remote-1', 'Mine')
    expect(await store.saveRemoteDashboard()).toBe(true)

    expect(store.storageError).toBeNull()
    expect(stored('shared-1').name).toBe('Mine')
  })

  it('stores a duplicate with new ids and no storage fields', async () => {
    const store = connected()
    const local = seedDashboard(store, 'Source')
    await putShared('source-1', JSON.parse(JSON.stringify(local)))

    await store.duplicateRemoteDashboard('source-1', 'Copy', 'ops')

    const key = store.remoteKeys.find(k => k.startsWith('ops.copy-'))!
    const doc = stored(key)
    expect(doc.name).toBe('Copy')
    expect(doc.id).not.toBe(local.id)
    expect(doc.widgets[0].id).not.toBe(local.widgets[0].id)
    expect(doc).not.toHaveProperty('storage')
    expect(doc).not.toHaveProperty('kvKey')
  })

  describe('renameRemoteDashboard', () => {
    it('renames one that is not open in KV, keeping its key', async () => {
      const store = connected()
      await putShared('ops.lobby-x7k2p9', { id: 'a', name: 'Lobby' })
      await store.refreshRemoteKeys()

      expect(await store.renameRemoteDashboard('ops.lobby-x7k2p9', '  Front lobby ')).toBe(true)

      expect(stored('ops.lobby-x7k2p9').name).toBe('Front lobby')
      expect(store.remoteNames['ops.lobby-x7k2p9']).toBe('Front lobby')
    })

    it('saves the open one at once when it had no other edits', async () => {
      const store = connected()
      await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
      await store.loadRemoteDashboard('shared-1')

      await store.renameRemoteDashboard('shared-1', 'Renamed')

      expect(store.activeDashboard!.name).toBe('Renamed')
      expect(stored('shared-1').name).toBe('Renamed')
      expect(store.isDirty).toBe(false)
    })

    // Saving would publish the unsaved edits along with the name.
    it('leaves the open one unsaved when it already had edits', async () => {
      const store = connected()
      await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
      await store.loadRemoteDashboard('shared-1')
      store.addWidget(createDefaultWidget('text', { x: 0, y: 0 }))

      await store.renameRemoteDashboard('shared-1', 'Renamed')

      expect(store.activeDashboard!.name).toBe('Renamed')
      expect(store.isDirty).toBe(true)
      expect(stored('shared-1').name).toBe('Shared')
    })

    it('refuses a blank name', async () => {
      const store = connected()
      await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
      expect(await store.renameRemoteDashboard('shared-1', '   ')).toBe(false)
      expect(stored('shared-1').name).toBe('Shared')
    })
  })

  it('opens a local dashboard and clears the startup pin when the open shared one is deleted', async () => {
    const store = connected()
    const local = seedDashboard(store, 'Local')
    await putShared('shared-1', { id: 'remote-1', name: 'Shared' })
    await store.loadRemoteDashboard('shared-1')
    store.setStartupDashboard('shared-1', 'kv')

    await store.deleteRemoteDashboard('shared-1')

    expect(store.activeDashboard?.id).toBe(local.id)
    expect(store.startupDashboard).toBeNull()
    expect(store.remoteKeys).toEqual([])
  })
})
