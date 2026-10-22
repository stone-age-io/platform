// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import AddWidgetModal from './AddWidgetModal.vue'
import { WIDGETS, WIDGET_CATEGORIES } from './widgets'
import { WIDGET_TYPES, type WidgetType } from '@/types/dashboard'

// The palette used to be sixteen hand-written <button> blocks; it is now a loop
// over WIDGETS. Most of what that object says is compile-checked —
// `Record<WidgetType, …>` refuses a missing type, and `category` must be one of
// WIDGET_CATEGORIES — but nothing checks that the loop puts them on screen, and
// a widget absent from this modal cannot be added to a dashboard at all. That
// is the one failure here with no partial version: the widget simply does not
// exist as far as anyone using the console is concerned.
//
// This is the only component test outside the two form views, and it is here
// because the component was rewritten, not to chase coverage.

const open = (props: Record<string, unknown> = {}) =>
  mount(AddWidgetModal, {
    attachTo: document.body,
    props: { modelValue: true, ...props },
    global: { stubs: { Teleport: false } },
  })

// The modal teleports to <body>, so its markup is not inside the wrapper tree.
const buttons = () => Array.from(document.querySelectorAll<HTMLElement>('.widget-type-btn'))
const names = () => buttons().map(b => b.querySelector('.widget-type-name')?.textContent?.trim())
const categoryLabels = () =>
  Array.from(document.querySelectorAll('.category-label')).map(e => e.textContent?.trim())

afterEach(() => {
  document.body.innerHTML = ''
})

describe('AddWidgetModal', () => {
  it('offers every widget type', () => {
    open()
    expect(buttons()).toHaveLength(WIDGET_TYPES.length)
    expect(names().sort()).toEqual(WIDGET_TYPES.map(t => WIDGETS[t].label).sort())
  })

  it('groups them under the declared categories, in order', () => {
    open()
    // Every category has something in it, or the heading is a label over
    // nothing.
    expect(categoryLabels()).toEqual([...WIDGET_CATEGORIES])
  })

  it('shows an icon and a description for each, so the grid is readable', () => {
    open()
    for (const btn of buttons()) {
      expect(btn.querySelector('.widget-type-icon')?.textContent?.trim()).toBeTruthy()
      expect(btn.querySelector('.widget-type-desc')?.textContent?.trim()).toBeTruthy()
    }
  })

  it('honours allowedTypes, and drops a category left empty by it', () => {
    const allowed: WidgetType[] = ['gauge', 'chart']
    open({ allowedTypes: allowed })

    expect(names().sort()).toEqual(allowed.map(t => WIDGETS[t].label).sort())
    // Both are Data Display, so the other three headings are gone rather than
    // standing over an empty row — which is what v-show on each button did.
    expect(categoryLabels()).toEqual(['Data Display'])
  })

  it('emits the type that was clicked, and closes', async () => {
    const wrapper = open()
    const gauge = buttons().find(b => b.querySelector('.widget-type-name')?.textContent?.trim() === WIDGETS.gauge.label)
    gauge!.click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('select')).toEqual([['gauge']])
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
