// @vitest-environment jsdom
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import QrLabelModal from './QrLabelModal.vue'

// The second exception to this suite's no-mounting rule, for the same reason as
// ConfirmDialog: what is under test IS the DOM contract. This component's whole
// job is turning N records into N printable boxes, and the ways it can go wrong
// are structural -- a record dropped without a word, a label rendered once for a
// list of ten, a symbol re-encoded on every tick of a detail view's live data.
//
// What NO test here can cover is the print output. Pagination, the mm geometry
// and the @page box are decided by a print engine that jsdom does not have and
// that a headless browser will not show us. Those need a test print on real
// stock; everything below is the half that can be pinned.

const { toDataURL } = vi.hoisted(() => ({
  // jsdom has no canvas, so the real encoder cannot run. The payload is echoed
  // back so a test can still prove WHICH code reached WHICH label.
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

// The component teleports to <body>, so its markup is NOT inside the mounted
// wrapper's tree -- wrapper.find() cannot see any of it. Query the document.
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

    // Each symbol carries its own record's code -- not the first one three times.
    const srcs = Array.from(document.querySelectorAll<HTMLImageElement>('.qr-label-page img')).map(
      (img) => img.src,
    )
    expect(srcs).toEqual([
      'data:image/png;base64,A-1',
      'data:image/png;base64,B-2',
      'data:image/png;base64,C-3',
    ])
  })

  // A code is optional, so a filtered set will contain records that cannot carry
  // a label. Printing the rest is right; doing it silently is not -- the tech
  // finds out at the site, with a stack that is short and no idea which two.
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

  // The single-record message is the one a person hits by accident, from a
  // detail view, so it says what to do rather than counting to one.
  it('tells a single codeless record to get a code', async () => {
    await open([{ code: '', name: 'Untagged', kind: 'thing' }])

    expect(text('.alert')).toContain('This record has no code')
  })

  // `records` is an array literal in the parent template, so its identity changes
  // on every parent re-render -- and a detail view re-renders whenever its live
  // NATS data ticks. Watching the array itself would re-encode every symbol on
  // each tick. This is the guard on that; it fails if the watch moves back to
  // `() => props.records`.
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
})
