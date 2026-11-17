import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/utils/pb', () => ({
  pb: { send: vi.fn().mockResolvedValue({ natsWebsocketUrls: [] }) },
}))

import { useNatsStore } from './nats'

const KEY = 'stone_age_nats_autoconnect'

// A stub, as in dashboard.spec.ts: the node environment has no usable
// localStorage, and Node 25's own one shadows jsdom's.
const storage = new Map<string, string>()
;(globalThis as any).localStorage = {
  getItem: (k: string) => storage.get(k) ?? null,
  setItem: (k: string, v: string) => { storage.set(k, String(v)) },
  removeItem: (k: string) => { storage.delete(k) },
  clear: () => { storage.clear() },
}

// Auto-connect is on unless this device turned it off. Off by default left a
// first login with nothing live until the user found Settings.
describe('nats store auto-connect setting', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('is on when this device never chose', async () => {
    const nats = useNatsStore()
    await nats.loadSettings()
    expect(nats.autoConnect).toBe(true)
  })

  it('stays off once this device turned it off', async () => {
    localStorage.setItem(KEY, 'false')
    const nats = useNatsStore()
    await nats.loadSettings()
    expect(nats.autoConnect).toBe(false)
  })

  it('stays on when this device turned it on', async () => {
    localStorage.setItem(KEY, 'true')
    const nats = useNatsStore()
    await nats.loadSettings()
    expect(nats.autoConnect).toBe(true)
  })

  it('saves an explicit choice, so turning it off survives a reload', async () => {
    const nats = useNatsStore()
    await nats.loadSettings()
    nats.autoConnect = false
    nats.saveSettings()
    expect(localStorage.getItem(KEY)).toBe('false')
  })
})
