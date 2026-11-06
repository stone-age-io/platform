// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import QrLabelModal from './QrLabelModal.vue'
import { useAuthStore } from '@/stores/auth'

// Mounted, an exception to the no-mounting rule: the DOM contract (N records in,
// N labels out) is the subject.
//
// Print output (pagination, mm geometry, the @page box) cannot be tested here;
// it needs a test print on real stock.

const { toDataURL } = vi.hoisted(() => ({
  // jsdom has no canvas. Echo the payload so a test can see which code reached
  // which label.
  toDataURL: vi.fn(async (text: string) => `data:image/png;base64,${text}`),
}))
vi.mock('qrcode', () => ({ default: { toDataURL } }))

type Rec = { code: string; name: string; kind: 'thing' | 'location' }

const THING = (code: string, name = `Thing ${code}`): Rec => ({ code, name, kind: 'thing' })

let wrapper: ReturnType<typeof mount> | null = null

async function open(records: Rec[]) {
  wrapper = mount(QrLabelModal, { attachTo: document.body, props: { records } })
  await flushPromises()
  return wrapper
}

// The component teleports to <body>, so query the document, not the wrapper.
const pages = () => Array.from(document.querySelectorAll('.qr-label-page'))
const text = (sel: string) => document.querySelector(sel)?.textContent?.replace(/\s+/g, ' ').trim() ?? ''

beforeEach(() => {
  setActivePinia(createPinia())
  toDataURL.mockClear()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('QrLabelModal', () => {
  it('renders one label for one record', async () => {
    await open([THING('DOOR-1')])

    expect(pages()).toHaveLength(1)
    expect(text('h3')).toBe('Label')
  })

  it('renders one label per record, in order, for a list', async () => {
    await open([THING('A-1'), THING('B-2'), THING('C-3')])

    expect(pages()).toHaveLength(3)
    expect(text('h3')).toBe('Labels (3)')

    const srcs = Array.from(document.querySelectorAll<HTMLImageElement>('.qr-label-page img')).map(
      (img) => img.src,
    )
    expect(srcs).toEqual([
      'data:image/png;base64,A-1',
      'data:image/png;base64,B-2',
      'data:image/png;base64,C-3',
    ])
  })

  // A silent drop is only discovered at the site.
  it('prints the records that have a code and names the ones it skipped', async () => {
    await open([THING('A-1'), { code: '', name: 'Nameless Sensor', kind: 'thing' }, THING('C-3')])

    expect(pages()).toHaveLength(2)
    expect(text('.alert')).toContain('1 of 3 skipped')
    expect(text('.alert')).toContain('Nameless Sensor')
  })

  it('refuses the whole batch when nothing in it has a code', async () => {
    await open([
      { code: '', name: 'One', kind: 'thing' },
      { code: '', name: 'Two', kind: 'thing' },
    ])

    expect(pages()).toHaveLength(0)
    expect(text('.alert')).toContain('None of these 2 records has a code')
    expect(document.querySelector<HTMLButtonElement>('.btn-primary')?.disabled).toBe(true)
  })

  it('tells a single codeless record to get a code', async () => {
    await open([{ code: '', name: 'Untagged', kind: 'thing' }])

    expect(text('.alert')).toContain('This record has no code')
  })

  // `records` is a new array on every parent re-render (each live-data tick on a
  // detail view). Fails if the watch moves back to `() => props.records`.
  it('does not re-encode when the parent re-renders with the same codes', async () => {
    const w = await open([THING('A-1'), THING('B-2')])
    expect(toDataURL).toHaveBeenCalledTimes(2)

    await w.setProps({ records: [THING('A-1'), THING('B-2')] })
    await flushPromises()

    expect(toDataURL).toHaveBeenCalledTimes(2)
  })

  it('re-encodes when the set of codes actually changes', async () => {
    const w = await open([THING('A-1')])
    expect(toDataURL).toHaveBeenCalledTimes(1)

    await w.setProps({ records: [THING('A-1'), THING('B-2')] })
    await flushPromises()

    expect(toDataURL).toHaveBeenCalledTimes(3)
    expect(pages()).toHaveLength(2)
  })

  // Things dominate an inventory, so only the rarer kind is marked.
  it('marks a location and leaves a thing unmarked', async () => {
    await open([THING('A-1'), { code: 'S01', name: 'North Plant', kind: 'location' }])

    const marks = pages().map((p) => p.textContent?.includes('Site'))
    expect(marks).toEqual([false, true])
  })

  describe('organization line', () => {
    const inOrg = (org: { code?: string; name: string }) => {
      const auth = useAuthStore()
      auth.memberships = [{ organization: 'o1', expand: { organization: { id: 'o1', ...org } } } as any]
      auth.currentOrgId = 'o1'
    }
    const orgLines = () => Array.from(document.querySelectorAll('.qr-label-org')).map((el) => el.textContent?.trim())

    // Thing codes are unique only within an organization.
    it('prints the organization code on every label', async () => {
      inOrg({ code: 'acme', name: 'Acme Facilities' })
      await open([THING('A-1'), THING('A-2')])

      expect(orgLines()).toEqual(['acme', 'acme'])
    })

    it('falls back to the organization name when it has no code', async () => {
      inOrg({ name: 'Acme Facilities' })
      await open([THING('A-1')])

      expect(orgLines()).toEqual(['Acme Facilities'])
    })

    it('does not print the operator brand', async () => {
      inOrg({ code: 'acme', name: 'Acme Facilities' })
      await open([THING('A-1')])

      expect(pages()[0].textContent).not.toContain('Stone-Age.io')
    })
  })

  // The arithmetic is fitCodePt's spec; this pins that sizing is per record.
  it('prints a short code larger than a long one in the same batch', async () => {
    await open([THING('AHU-1'), THING('S01-AHU-SUPPLY-FAN-03')])

    const sizes = Array.from(document.querySelectorAll<HTMLElement>('.qr-label-code')).map((el) =>
      parseFloat(el.style.fontSize),
    )
    expect(sizes).toHaveLength(2)
    expect(sizes[0]).toBeGreaterThan(sizes[1])
  })
})
