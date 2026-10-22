// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import NebulaHostFormView from './NebulaHostFormView.vue'
import { useAuthStore } from '@/stores/auth'

// The other half of the pair ThingFormView.spec.ts opens: the two forms where
// the console meets provisioning, and where a silent regression stops somebody
// putting a device on the network. The reasoning for mounting at all is in that
// file's header and in vitest.config.ts.
//
// WHAT IT PINS. This form's job is almost entirely translation — an operator
// edits text, pb-nebula needs arrays, integers and a nested object — and every
// one of those conversions is a place a value can be quietly dropped or sent in
// a shape the schema rejects with a generic 400:
//
//   - groups: one comma-separated box becomes an array;
//   - the textareas become arrays of trimmed, non-empty lines;
//   - a half-filled unsafe route is dropped rather than sent;
//   - an empty MTU sends 0, not null — 0 is pb-nebula's "inherit the default"
//     and the schema's Min bound is short-circuited on it, so null is rejected;
//   - validity_years is OMITTED when blank, for the same reason in reverse
//     (min:1 rejects a null or 0);
//   - password is omitted when blank, so an edit does not rotate it;
//   - the allow list is parsed here, because a parse failure on the server
//     arrives as a 400 with nothing an operator can act on.
//
// It does NOT assert on layout, classes or wording.

const { getOne, getFullList, getFirstListItem, create, update, push, toast } = vi.hoisted(() => ({
  getOne: vi.fn(),
  getFullList: vi.fn(),
  getFirstListItem: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  push: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}))

const route = { params: {} as Record<string, string>, query: {}, path: '/nebula/hosts/new' }

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }),
}))

vi.mock('@/utils/pb', () => {
  const pb = {
    send: vi.fn(),
    filter: (expr: string) => expr,
    collection: (name: string) => ({
      getFullList: (opts?: unknown) => getFullList(name, opts),
      getOne: (id: string) => getOne(name, id),
      getFirstListItem: (filter: string) => getFirstListItem(name, filter),
      create: (data: unknown) => create(name, data),
      update: (id: string, data: unknown) => update(name, id, data),
    }),
    authStore: { token: 'test-token', isValid: true, model: null },
  }
  return { pb, default: pb }
})

vi.mock('@/composables/useToast', () => ({ useToast: () => toast }))

const HOST = {
  id: 'host1',
  hostname: 'gw-s01',
  email: 'gw-s01@nebula.local',
  network_id: 'net1',
  groups: ['gateway', 'ssh'],
  overlay_ip: '10.100.0.5',
  is_lighthouse: false,
  is_relay: false,
  public_host_port: '',
  active: true,
  validity_years: 2,
  unsafe_networks: ['192.168.1.0/24'],
  preferred_ranges: ['10.0.0.0/8'],
  unsafe_routes: [{ route: '192.168.5.0/24', via: '10.100.0.7' }],
  mtu: 1300,
  tun_device: 'nebula1',
  additional_host_ports: ['198.51.100.7:4242'],
  advertise_addrs: [],
  local_allow_list: { interfaces: { 'docker.*': false } },
}

function signIn() {
  const auth = useAuthStore()
  auth.user = { id: 'u1', email: 'u@example.test' } as any
  auth.memberships = [
    {
      id: 'm1',
      organization: 'org1',
      role: 'owner',
      expand: { organization: { id: 'org1', code: 'acme', name: 'Acme' } },
    },
  ] as any
  auth.currentOrgId = 'org1'
}

async function mountForm(opts: { edit?: boolean } = {}) {
  route.params = opts.edit ? { id: 'host1' } : {}
  route.path = opts.edit ? '/nebula/hosts/host1/edit' : '/nebula/hosts/new'

  const wrapper = mount(NebulaHostFormView, {
    global: { stubs: { Teleport: true, RouterLink: true } },
  })
  await flushPromises()
  return wrapper
}

/**
 * Set the input or textarea in the `.form-control` whose label says `label`.
 *
 * By label rather than by placeholder, because several placeholders here are
 * multi-line (they show a one-per-line example), and a newline inside an
 * attribute selector is a syntax error. The label is also what the operator
 * reads, so a test that can no longer find it is describing a real change.
 */
async function fill(wrapper: any, label: string, value: string) {
  const control = wrapper
    .findAll('.form-control')
    .find((c: any) => c.find('.label-text').exists() && c.find('.label-text').text() === label)
  if (!control) throw new Error(`no form-control labelled ${JSON.stringify(label)}`)
  const field = control.find('textarea').exists() ? control.find('textarea') : control.find('input')
  if (!field.exists()) throw new Error(`no field under ${JSON.stringify(label)}`)
  await field.setValue(value)
}

/** The data object handed to create() or update(). */
function submitted(): any {
  if (create.mock.calls.length) return create.mock.calls[0][1]
  return update.mock.calls[0][2]
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  getFullList.mockResolvedValue([{ id: 'net1', name: 'overlay', cidr_range: '10.100.0.0/16', active: true }])
  getOne.mockResolvedValue({ ...HOST })
  // findLinkedThing: no Thing holds this host.
  getFirstListItem.mockRejectedValue(new Error('not found'))
  create.mockResolvedValue({ ...HOST })
  update.mockResolvedValue({ ...HOST })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('NebulaHostFormView renders', () => {
  it('shows the fields a host cannot be created without', async () => {
    signIn()
    const wrapper = await mountForm()
    const text = wrapper.text()

    expect(wrapper.find('form').exists()).toBe(true)
    for (const label of ['Hostname *', 'Email (Identity) *', 'Network *', 'Overlay IP *']) {
      expect(text).toContain(label)
    }
    expect(wrapper.find('button[type="submit"]').exists()).toBe(true)
  })

  it('selects the only network there is, so creating one does not need a pick', async () => {
    signIn()
    const wrapper = await mountForm()

    await fill(wrapper, 'Hostname *', 'gw-s02')
    await fill(wrapper, 'Email (Identity) *', 'gw-s02@nebula.local')
    await fill(wrapper, 'Overlay IP *', '10.100.0.9')
    await wrapper.find('input[type="password"]').setValue('a-long-password')
    await wrapper.findAll('input[type="password"]')[1].setValue('a-long-password')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(submitted().network_id).toBe('net1')
  })
})

describe('NebulaHostFormView submits', () => {
  it('refuses to create a host with no password, before the round trip', async () => {
    signIn()
    const wrapper = await mountForm()

    await fill(wrapper, 'Hostname *', 'gw-s02')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(create).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalled()
  })

  it('turns the text boxes back into the arrays, integers and object pb-nebula wants', async () => {
    signIn()
    const wrapper = await mountForm({ edit: true })
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(submitted()).toMatchObject({
      hostname: 'gw-s01',
      // One comma-separated box, back to an array.
      groups: ['gateway', 'ssh'],
      // Textareas, back to arrays of lines.
      unsafe_networks: ['192.168.1.0/24'],
      preferred_ranges: ['10.0.0.0/8'],
      additional_host_ports: ['198.51.100.7:4242'],
      advertise_addrs: [],
      unsafe_routes: [{ route: '192.168.5.0/24', via: '10.100.0.7' }],
      // String in the box, integer on the wire.
      mtu: 1300,
      validity_years: 2,
      // Pretty-printed JSON in the box, object on the wire.
      local_allow_list: { interfaces: { 'docker.*': false } },
    })
  })

  it('drops blank lines and whitespace rather than sending empty entries', async () => {
    signIn()
    const wrapper = await mountForm({ edit: true })

    await fill(wrapper, 'Unsafe Networks', '  10.1.0.0/16  \n\n\n  10.2.0.0/16\n')
    await fill(wrapper, 'Groups', ' gateway , , ssh ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(submitted().unsafe_networks).toEqual(['10.1.0.0/16', '10.2.0.0/16'])
    expect(submitted().groups).toEqual(['gateway', 'ssh'])
  })

  it('sends 0 for an empty MTU and omits validity_years entirely', async () => {
    signIn()
    getOne.mockResolvedValue({ ...HOST, mtu: 0, validity_years: 0 })
    const wrapper = await mountForm({ edit: true })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // 0 is "inherit the default". null is rejected by the schema's Min bound.
    expect(submitted().mtu).toBe(0)
    expect(submitted()).not.toHaveProperty('validity_years')
  })

  it('omits the password on an edit that did not set one', async () => {
    signIn()
    const wrapper = await mountForm({ edit: true })
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(submitted()).not.toHaveProperty('password')
    expect(submitted()).not.toHaveProperty('passwordConfirm')
  })

  it('refuses an allow list that is not a JSON object, and says so', async () => {
    signIn()
    const wrapper = await mountForm({ edit: true })

    await fill(wrapper, 'Local Allow List', '["nope"]')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(update).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith('Local allow list must be a JSON object')
  })
})
