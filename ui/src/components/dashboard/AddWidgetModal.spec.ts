// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import AddWidgetModal from './AddWidgetModal.vue'
import { WIDGETS, WIDGET_CATEGORIES } from './widgets'
import { WIDGET_TYPES } from '@/types/dashboard'

// A widget missing from this palette cannot be added at all. The types catch a
// missing WIDGETS entry; this checks that the loop puts every one on screen.

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
      expect(btn.querySelector('svg.widget-type-icon')).not.toBeNull()
      expect(btn.querySelector('.widget-type-desc')?.textContent?.trim()).toBeTruthy()
    }
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
