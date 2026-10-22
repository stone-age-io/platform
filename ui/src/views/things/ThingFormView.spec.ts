// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ThingFormView from './ThingFormView.vue'
import { useAuthStore } from '@/stores/auth'

// The third exception to this suite's no-mounting rule (ConfirmDialog and
// QrLabelModal are the others), and the narrowest one: this file mounts a view,
// which is the thing vitest.config.ts argues against.
//
// WHY THIS ONE. 57,881 lines of console and nothing renders a component in CI,
// so `vue-tsc && vite build` stays green while a form draws nothing. Of the 242
// components, this is the one where a silent regression stops somebody creating
// a device — and the body it submits is not a shape the compiler checks, because
// `pb.send` takes `any` and the route parses JSON on the far side.
//
// WHAT IT PINS, and nothing more:
//   - the form renders, with the fields a Thing cannot be created without;
//   - the create body is the one POST /api/org/things parses (hooks/thing_routes.go),
//     including the nats/nebula mode objects;
//   - a member's update body OMITS nats_user and nebula_host. That is not a
//     tidiness point: the member branch of things.updateRule requires
//     `nats_user:changed = false`, a field absent from a JSON body counts as
//     unchanged, and sending it turns an ordinary inventory edit into a 404.
//     scripts/test-authz.sh proves the server half; this proves the client
//     still sends what that half expects.
//
// It does NOT assert on layout, classes or wording. Those change on purpose.

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

// The collection name is passed through to every stub, so a test can say which
// collection it is answering for — and assert which ones were asked at all.
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
      // The quick-add modals mount three OTHER whole form views. They are
      // tested, or not, on their own; pulling them in here would make this
      // spec fail for reasons that have nothing to do with the Thing form.
      stubs: {
        NatsUserFormView: true,
        NebulaHostFormView: true,
        LocationFormView: true,
        // Needs a file token and a real <img> pipeline. Neither is what this
        // file is about.
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
    // A default role, so auto-provisioning is actually reachable. Without one
    // the form refuses before the round trip, which is its own test below.
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

    // Inventory reference data only. nats_users is row-scoped to the caller, so
    // asking as a member returns their own personal identity as the sole
    // option, and the Nebula lists come back empty — offering either would be
    // a picker full of choices the API will refuse.
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
      // Both halves are always present and always objects. The route reads
      // body.nats.mode and body.nebula.mode unconditionally.
      nats: { mode: 'auto' },
      nebula: { mode: 'none' },
    })
    expect(Object.keys(init.body.nats).sort()).toEqual(['mode', 'role_id', 'user_id'])
    expect(Object.keys(init.body.nebula).sort()).toEqual(['host_id', 'mode', 'network_id', 'overlay_ip'])
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
