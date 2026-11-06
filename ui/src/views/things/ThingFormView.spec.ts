// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ThingFormView from './ThingFormView.vue'
import { useAuthStore } from '@/stores/auth'

// One of the mounted exceptions listed in vitest.config.ts: the submitted body
// is not a shape the compiler checks (`pb.send` takes `any`). It pins only:
//   - the form renders the fields a Thing cannot be created without;
//   - the create body matches what POST /api/org/things parses (hooks/thing_routes.go);
//   - a member's update body OMITS nats_user and nebula_host: things.updateRule
//     requires `nats_user:changed = false` for members, so sending them 404s.
//     scripts/test-authz.sh proves the server half.
// Not layout, classes or wording.

const { send, getOne, getFullList, getFirstListItem, update, push, toast } = vi.hoisted(() => ({
  send: vi.fn(),
  getOne: vi.fn(),
  getFullList: vi.fn(),
  getFirstListItem: vi.fn(),
  update: vi.fn(),
  push: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}))

const route = { params: {} as Record<string, string>, query: {}, path: '/things/new' }

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }),
}))

// The collection name is passed to every stub so tests can assert which were asked.
vi.mock('@/utils/pb', () => {
  const pb = {
    send,
    filter: (expr: string) => expr,
    collection: (name: string) => ({
      getFullList: (opts?: unknown) => getFullList(name, opts),
      getOne: (id: string) => getOne(name, id),
      getFirstListItem: (filter: string) => getFirstListItem(name, filter),
      update: (id: string, data: unknown) => update(name, id, data),
      create: vi.fn(),
    }),
    authStore: { token: 'test-token', isValid: true, model: null },
    files: { getURL: () => '' },
  }
  return { pb, default: pb }
})

vi.mock('@/composables/useToast', () => ({ useToast: () => toast }))

const THING = {
  id: 'thing1',
  name: 'Warehouse HVAC',
  description: 'Rooftop unit',
  code: 'AHU-9KD-4PX',
  type: 'type1',
  location: 'loc1',
  email: 'ahu-9kd-4px@acme.thing.local',
  nats_user: 'nu1',
  nebula_host: 'nh1',
  metadata: { floor: '3' },
  active: true,
}

/** Put a signed-in user with `role` in org `acme` into the store. */
function signIn(role: 'owner' | 'admin' | 'member' | 'viewer') {
  const auth = useAuthStore()
  auth.user = { id: 'u1', email: 'u@example.test' } as any
  auth.memberships = [
    {
      id: 'm1',
      organization: 'org1',
      role,
      expand: { organization: { id: 'org1', code: 'acme', name: 'Acme' } },
    },
  ] as any
  auth.currentOrgId = 'org1'
}

/** Mount in create or edit mode and wait for onMounted's loads to settle. */
async function mountForm(opts: { edit?: boolean } = {}) {
  route.params = opts.edit ? { id: 'thing1' } : {}
  route.path = opts.edit ? '/things/thing1/edit' : '/things/new'

  const wrapper = mount(ThingFormView, {
    global: {
      // The quick-add modals mount other whole form views; keep them out.
      stubs: {
        NatsUserFormView: true,
        NebulaHostFormView: true,
        LocationFormView: true,
        // Needs a file token and a real <img> pipeline.
        ImageUploadField: true,
        Teleport: true,
        RouterLink: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

/** Which collections the component asked for a full list of. */
function listedCollections(): string[] {
  return getFullList.mock.calls.map(c => c[0])
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()

  const lists: Record<string, unknown[]> = {
    thing_types: [{ id: 'type1', name: 'HVAC', prefix: 'AHU' }],
    locations: [{ id: 'loc1', name: 'Roof', parent_id: '' }],
    nats_users: [],
    nebula_hosts: [],
    // A default role makes auto-provisioning reachable (no role is tested below).
    nats_roles: [{ id: 'role1', name: 'device', is_default: true }],
    nebula_networks: [],
  }
  getFullList.mockImplementation((name: string) => Promise.resolve(lists[name] ?? []))
  getOne.mockResolvedValue({ ...THING })
  getFirstListItem.mockResolvedValue({ id: 'acct1', active: true })
  send.mockResolvedValue({ email: THING.email, password: 'generated-password' })
  update.mockResolvedValue({ ...THING })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ThingFormView renders', () => {
  it('shows the fields a Thing cannot be created without', async () => {
    signIn('owner')
    const wrapper = await mountForm()
    const text = wrapper.text()

    expect(wrapper.find('form').exists()).toBe(true)
    for (const label of ['Name *', 'Code', 'Type', 'Location']) {
      expect(text).toContain(label)
    }
    expect(wrapper.find('button[type="submit"]').exists()).toBe(true)
    expect(text).toContain('Provision Thing')
  })

  it('offers the identity cards to an admin and withholds them from a member', async () => {
    signIn('owner')
    expect((await mountForm()).text()).toContain('NATS Connectivity')

    setActivePinia(createPinia())
    signIn('member')
    expect((await mountForm()).text()).not.toContain('NATS Connectivity')
  })

  it('does not ask a member for identity options it cannot read', async () => {
    signIn('member')
    await mountForm()

    // Inventory reference data only. As a member, nats_users would return only
    // their own identity and the Nebula lists would be empty.
    expect(listedCollections().sort()).toEqual(['locations', 'thing_types'])
  })
})

describe('ThingFormView submits', () => {
  it('creates through POST /api/org/things, in the shape that route parses', async () => {
    signIn('owner')
    const wrapper = await mountForm()

    await wrapper.find('input[placeholder="e.g. Warehouse HVAC"]').setValue('Lobby Camera')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(send).toHaveBeenCalledTimes(1)
    const [path, init] = send.mock.calls[0]
    expect(path).toBe('/api/org/things')
    expect(init.method).toBe('POST')
    expect(init.body).toMatchObject({
      name: 'Lobby Camera',
      // Always both objects: the route reads body.nats.mode and body.nebula.mode unconditionally.
      nats: { mode: 'auto' },
      nebula: { mode: 'none' },
    })
    expect(Object.keys(init.body.nats).sort()).toEqual(['mode', 'role_id', 'user_id'])
    expect(Object.keys(init.body.nebula).sort()).toEqual(['host_id', 'mode', 'network_id', 'overlay_ip'])
  })

  it('does not open the credentials dialog when the route withholds the password', async () => {
    signIn('member')
    send.mockResolvedValue({ id: 'thing9', email: THING.email })
    const wrapper = await mountForm()

    await wrapper.find('input[placeholder="e.g. Warehouse HVAC"]').setValue('Lobby Camera')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(send).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).not.toContain('Thing Provisioned Successfully')
    expect(push).toHaveBeenCalledWith('/things')
  })

  it('refuses to auto-provision NATS with no role selected, before the round trip', async () => {
    signIn('owner')
    // No roles to pick from, so autoNatsRoleId stays empty while mode is auto.
    getFullList.mockImplementation((name: string) =>
      Promise.resolve(name === 'nats_roles' ? [] : name === 'thing_types' ? [{ id: 'type1', name: 'HVAC' }] : [])
    )
    const wrapper = await mountForm()

    await wrapper.find('input[placeholder="e.g. Warehouse HVAC"]').setValue('Lobby Camera')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(send).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalled()
  })

  it('omits nats_user and nebula_host from a member edit', async () => {
    signIn('member')
    const wrapper = await mountForm({ edit: true })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(update).toHaveBeenCalledTimes(1)
    const body = update.mock.calls[0][2]
    expect(body).not.toHaveProperty('nats_user')
    expect(body).not.toHaveProperty('nebula_host')
    // The inventory fields a member IS allowed to change still go.
    expect(body).toMatchObject({ name: THING.name, code: THING.code })
  })

  it('sends nats_user and nebula_host on an admin edit', async () => {
    signIn('owner')
    const wrapper = await mountForm({ edit: true })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const body = update.mock.calls[0][2]
    expect(body.nats_user).toBe('nu1')
    expect(body.nebula_host).toBe('nh1')
  })
})
