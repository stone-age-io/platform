import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { createDefaultWidget } from '@/types/dashboard'
import { useDashboardStore } from './dashboard'

// Import/export had no coverage, and a bad import loses a user's work silently:
// the `replace` strategy clears every local dashboard before writing, and the
// limit check decides whether anything is written at all. Nothing in the build
// or the type checker touches any of it.
//
// localStorage is stubbed rather than left absent. saveToStorage swallows its
// own errors, so without a stub these tests would pass through the failure
// branch and prove nothing about the path that actually runs in a browser.
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
    expect(got.widgets[0].type).toBe('kvtable')
    // Widget config has to survive, or an imported dashboard looks right and
    // shows nothing.
    expect(got.widgets[0].kvtableConfig?.kvBucket).toBe(
      original.widgets[0].kvtableConfig?.kvBucket,
    )
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

  // storage / kvKey / kvRevision describe where a dashboard LIVED, not what it
  // is. Exporting them would make an imported copy claim to be a shared
  // dashboard backed by a KV key it does not own.
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

  // The limit is checked BEFORE anything is written, which is the part that
  // matters: a partial import that then hit the limit would leave the user with
  // some of their dashboards replaced and some not.
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
