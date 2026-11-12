// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import SwitchFields from './SwitchFields.vue'
import SliderFields from './SliderFields.vue'
import GaugeFields from './GaugeFields.vue'
import type { GaugeWidgetConfig, SliderWidgetConfig, SwitchWidgetConfig } from '@/types/dashboard'

// One form serves the widget's modal and a marker row, so what it emits is
// what both of them save.

const base: SwitchWidgetConfig = {
  mode: 'kv',
  kvBucket: 'b',
  kvKey: 'k',
  onPayload: { state: 'on' },
  offPayload: { state: 'off' },
}

const last = (w: ReturnType<typeof mount>) => {
  const all = w.emitted('update:modelValue') as unknown[][]
  return all[all.length - 1][0] as SwitchWidgetConfig
}

describe('SwitchFields', () => {
  it('shows a stored object payload as JSON text', () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    const [on, off] = w.findAll('textarea')
    expect(on.element.value).toBe('{"state":"on"}')
    expect(off.element.value).toBe('{"state":"off"}')
  })

  // A payload is stored as a value: `true` must reach the device as a boolean,
  // not as the four letters.
  it.each([
    ['{"a": 1}', { a: 1 }],
    ['true', true],
    ['1', 1],
    ['"on"', 'on'],
  ])('stores %s as its JSON value', async (text, stored) => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    await w.findAll('textarea')[0].setValue(text)
    expect(last(w).onPayload).toEqual(stored)
  })

  // Plenty of devices take a bare word. It is sent as typed, without a warning.
  it('stores text that is not JSON as that text', async () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    await w.findAll('textarea')[0].setValue('ON')
    expect(last(w).onPayload).toBe('ON')
    expect(w.text()).not.toContain('Not valid JSON')
  })

  // A missing quote would otherwise be published as text and look like success.
  it('warns when JSON was meant and does not parse', async () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    await w.findAll('textarea')[0].setValue('{"state": on}')
    expect(last(w).onPayload).toBe('{"state": on}')
    expect(w.text()).toContain('Not valid JSON')
  })

  // Half-typed JSON must stay as typed while the parent hands the config back.
  it('does not rewrite the box when its own value comes back', async () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    const on = w.findAll('textarea')[0]
    await on.setValue('{"a": 1 }')
    await w.setProps({ modelValue: last(w) })
    expect(on.element.value).toBe('{"a": 1 }')
  })

  it('refills the box when the payload changes from outside', async () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    await w.setProps({ modelValue: { ...base, onPayload: 'go' } })
    expect(w.findAll('textarea')[0].element.value).toBe('go')
  })

  it('shows the subjects in core mode and the bucket in KV mode', async () => {
    const w = mount(SwitchFields, { props: { modelValue: base } })
    expect(w.text()).toContain('KV Bucket')
    expect(w.text()).not.toContain('Publish Subject')
    await w.setProps({ modelValue: { ...base, mode: 'core' } })
    expect(w.text()).toContain('Publish Subject')
    expect(w.text()).not.toContain('KV Bucket')
  })

  it('keeps the other label when one is edited', async () => {
    const w = mount(SwitchFields, { props: { modelValue: { ...base, labels: { on: 'Open', off: 'Shut' } } } })
    const inputs = w.findAll('.label-inputs input')
    await inputs[1].setValue('Closed')
    expect(last(w).labels).toEqual({ on: 'Open', off: 'Closed' })
  })
})

describe('SliderFields', () => {
  const slider: SliderWidgetConfig = { mode: 'kv', min: 0, max: 100, step: 1, defaultValue: 50 }

  it('stores the range as numbers', async () => {
    const w = mount(SliderFields, { props: { modelValue: slider } })
    await w.findAll('input[type="number"]')[1].setValue('250')
    const all = w.emitted('update:modelValue') as unknown[][]
    expect((all[all.length - 1][0] as SliderWidgetConfig).max).toBe(250)
  })

  // {thing} is filled in on a floor plan and nowhere else.
  it('suggests {thing} only for a per-Thing row', () => {
    const key = (w: ReturnType<typeof mount>) => w.findAll('input[type="text"]')[1].attributes('placeholder')
    expect(key(mount(SliderFields, { props: { modelValue: slider } }))).not.toContain('{thing}')
    expect(key(mount(SliderFields, { props: { modelValue: slider, perThing: true } }))).toContain('{thing}')
  })
})

describe('GaugeFields', () => {
  const gauge: GaugeWidgetConfig = { min: 0, max: 100, zones: [] }
  const lastGauge = (w: ReturnType<typeof mount>) => {
    const all = w.emitted('update:modelValue') as unknown[][]
    return all[all.length - 1][0] as GaugeWidgetConfig
  }

  // Blank means "pick from the value's size", which is stored as no value at all.
  it('stores blank decimals as automatic', async () => {
    const w = mount(GaugeFields, { props: { modelValue: { ...gauge, decimals: 2 } } })
    const decimals = w.find('input[placeholder="Automatic"]')
    await decimals.setValue('')
    expect(lastGauge(w).decimals).toBeUndefined()
    await decimals.setValue('3')
    expect(lastGauge(w).decimals).toBe(3)
  })

  // A marker row is always a bar, so it is not offered the choice.
  it('offers the style only when asked', () => {
    expect(mount(GaugeFields, { props: { modelValue: gauge } }).text()).not.toContain('Style')
    expect(mount(GaugeFields, { props: { modelValue: gauge, showStyle: true } }).text()).toContain('Style')
  })

  // A marker row's config also holds where to read the value. Editing the
  // scale must hand those fields back untouched.
  it('keeps fields it does not edit', async () => {
    const row = { ...gauge, mode: 'kv', kvBucket: 'twin', kvKey: 'thing.a.level' }
    const w = mount(GaugeFields, { props: { modelValue: row } })
    await w.findAll('input[type="number"]')[1].setValue('250')
    expect(lastGauge(w)).toMatchObject({ max: 250, mode: 'kv', kvBucket: 'twin', kvKey: 'thing.a.level' })
  })
})
