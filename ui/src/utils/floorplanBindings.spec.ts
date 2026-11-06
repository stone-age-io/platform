import { describe, it, expect } from 'vitest'
import {
  bindingForThing,
  bindingKvKeyForThing,
  bindingKvPattern,
  bindingSubjectForThing,
  bindingSubscriptionSubject,
  displayValue,
  markerStateFor,
  isPlaced,
  resolveItemsForThing,
  type BindableThing,
  type BindableType,
  type FloorPlanContext,
} from './floorplanBindings'
import type { FloorPlanTypeBinding, MapMarkerItem } from '@/types/dashboard'

// Nothing renders in CI (ui/CLAUDE.md), so these are the only guard this
// widget gets. They cover the four decisions whose failure is SILENT on a real
// plan -- a subscription that covers nothing, a message matched to the wrong
// marker, an item that publishes a literal {thing}, and a dead device still
// drawn green -- because every one of those renders as a plan that simply never
// changes colour.

const ctx: FloorPlanContext = { orgCode: 'acme', locationCode: 'BLDG-A' }

const doorType: BindableType = {
  id: 'tt_door',
  code: 'door',
  subject_prefix: '{thing_type_code}.{thing}',
}

function binding(over: Partial<FloorPlanTypeBinding> = {}): FloorPlanTypeBinding {
  return {
    id: 'b1',
    thingTypeId: 'tt_door',
    stateSource: { mode: 'subject', subjectSuffix: 'state' },
    items: [],
    ...over,
  }
}

const door: BindableThing = {
  id: 'rec_1',
  code: 'DOOR-01',
  name: 'North door',
  type: 'tt_door',
  floorplan_position: { x: 10, y: 20 },
}

describe('subscription subjects', () => {
  it('wildcards the thing so ONE subscription covers every thing of the type', () => {
    expect(bindingSubscriptionSubject(binding(), doorType, ctx)).toBe('door.*.state')
  })

  it('resolves the same template to a concrete subject per thing', () => {
    expect(bindingSubjectForThing(binding(), doorType, door, ctx)).toBe('door.DOOR-01.state')
  })

  it('the per-thing subject matches what the subscription covers', () => {
    // The point of the pair: a message arriving on the wildcard has to be
    // findable under the key the marker computes, or markers never light up.
    const pattern = bindingSubscriptionSubject(binding(), doorType, ctx)!
    const concrete = bindingSubjectForThing(binding(), doorType, door, ctx)!
    const re = new RegExp('^' + pattern.split('.').map(t => (t === '*' ? '[^.]+' : t)).join('\\.') + '$')
    expect(concrete).toMatch(re)
  })

  it('falls back to the default prefix when the type declares none', () => {
    const bare: BindableType = { id: 'tt_x', code: 'camera' }
    expect(bindingSubscriptionSubject(binding(), bare, ctx)).toBe('camera.*.state')
    expect(bindingSubjectForThing(binding(), bare, door, ctx)).toBe('camera.DOOR-01.state')
  })

  it('addresses a thing by code, falling back to its id', () => {
    const uncoded: BindableThing = { ...door, code: undefined }
    expect(bindingSubjectForThing(binding(), doorType, uncoded, ctx)).toBe('door.rec_1.state')
  })

  it('substitutes {location} for a type whose prefix opts into it', () => {
    const fixed: BindableType = { id: 'tt_f', code: 'freezer', subject_prefix: '{location}.freezer.{thing}' }
    expect(bindingSubjectForThing(binding(), fixed, door, ctx)).toBe('BLDG-A.freezer.DOOR-01.state')
    // The subscription wildcards {thing} and keeps the known location.
    expect(bindingSubscriptionSubject(binding(), fixed, ctx)).toBe('BLDG-A.freezer.*.state')
  })

  it('an empty suffix watches the prefix itself rather than a trailing dot', () => {
    const b = binding({ stateSource: { mode: 'subject', subjectSuffix: '' } })
    expect(bindingSubjectForThing(b, doorType, door, ctx)).toBe('door.DOOR-01')
  })

  it('is null for a kv binding, so a caller cannot subscribe to one by accident', () => {
    const b = binding({ stateSource: { mode: 'kv', kvBucket: 'twin', kvKeyTemplate: 'thing.{thing}.state' } })
    expect(bindingSubscriptionSubject(b, doorType, ctx)).toBeNull()
    expect(bindingSubjectForThing(b, doorType, door, ctx)).toBeNull()
  })

  it('is null when the binding has no state source at all', () => {
    const b = binding({ stateSource: undefined })
    expect(bindingSubscriptionSubject(b, doorType, ctx)).toBeNull()
  })
})

describe('kv keys', () => {
  const kv = binding({
    stateSource: { mode: 'kv', kvBucket: 'twin', kvKeyTemplate: 'thing.{thing}.state' },
  })

  it('watches a prefix with the thing wildcarded', () => {
    expect(bindingKvPattern(kv, doorType, ctx)).toBe('thing.*.state')
  })

  it('resolves a concrete key per thing', () => {
    expect(bindingKvKeyForThing(kv, doorType, door, ctx)).toBe('thing.DOOR-01.state')
  })

  it('is null without a bucket, since a key alone watches nothing', () => {
    const noBucket = binding({ stateSource: { mode: 'kv', kvKeyTemplate: 'thing.{thing}' } })
    expect(bindingKvPattern(noBucket, doorType, ctx)).toBeNull()
    expect(bindingKvKeyForThing(noBucket, doorType, door, ctx)).toBeNull()
  })
})

describe('resolveItemsForThing', () => {
  const items: MapMarkerItem[] = [
    {
      id: 'i1',
      type: 'text',
      label: 'State',
      textConfig: { subject: '{thing_type_code}.{thing}.state', jsonPath: '$.state' },
    },
    {
      id: 'i2',
      type: 'publish',
      label: 'Unlock',
      subject: 'cmd.{thing}.unlock',
      payload: '{"door":"{thing}"}',
    },
    {
      id: 'i3',
      type: 'switch',
      label: 'Hold',
      switchConfig: {
        mode: 'kv',
        kvBucket: 'twin_desired',
        kvKey: 'thing.{thing}.hold',
        publishSubject: 'cmd.{thing}.hold',
        onPayload: {},
        offPayload: {},
      },
    },
    {
      id: 'i4',
      type: 'kv',
      label: 'Last seen',
      kvConfig: { kvBucket: 'twin', kvKey: 'thing.{thing}.seen' },
    },
    {
      id: 'i5',
      type: 'slider',
      label: 'Setpoint',
      sliderConfig: {
        mode: 'kv',
        kvBucket: 'twin_desired',
        kvKey: 'thing.{thing}.setpoint',
        publishSubject: 'cmd.{thing}.setpoint',
        stateSubject: 'state.{thing}.setpoint',
        min: 0, max: 100, step: 1, defaultValue: 20,
      },
    },
    {
      id: 'i6',
      type: 'gauge',
      label: 'Level',
      gaugeConfig: {
        mode: 'core',
        subject: '{thing_type_code}.{thing}.level',
        kvBucket: 'twin',
        kvKey: 'thing.{thing}.level',
        min: 0, max: 100,
      },
    },
  ]

  const out = resolveItemsForThing(items, doorType, door, ctx)

  it('resolves every addressing field, not only the top-level subject', () => {
    expect(out[0].textConfig?.subject).toBe('door.DOOR-01.state')
    expect(out[1].subject).toBe('cmd.DOOR-01.unlock')
    expect(out[2].switchConfig?.kvKey).toBe('thing.DOOR-01.hold')
    expect(out[2].switchConfig?.publishSubject).toBe('cmd.DOOR-01.hold')
    expect(out[3].kvConfig?.kvKey).toBe('thing.DOOR-01.seen')
  })

  it('resolves the slider row, both the write side and the read side', () => {
    expect(out[4].sliderConfig?.kvKey).toBe('thing.DOOR-01.setpoint')
    expect(out[4].sliderConfig?.publishSubject).toBe('cmd.DOOR-01.setpoint')
    expect(out[4].sliderConfig?.stateSubject).toBe('state.DOOR-01.setpoint')
  })

  it('resolves the gauge row in whichever mode it is not using too', () => {
    // Both are resolved regardless of mode: the operator can switch a row
    // from core to kv in the editor without the unused half going stale.
    expect(out[5].gaugeConfig?.subject).toBe('door.DOOR-01.level')
    expect(out[5].gaugeConfig?.kvKey).toBe('thing.DOOR-01.level')
  })

  it('resolves the payload, so a command names the device it is for', () => {
    expect(out[1].payload).toBe('{"door":"DOOR-01"}')
  })

  it('gives every thing its own item ids', () => {
    // Load-bearing: MarkerItem* subscribe under `marker-text-${item.id}`, so a
    // shared id would have sixty doors share one buffer and all show whichever
    // published last.
    const other = resolveItemsForThing(items, doorType, { ...door, id: 'rec_2', code: 'DOOR-02' }, ctx)
    expect(out[0].id).not.toBe(other[0].id)
    expect(new Set([...out, ...other].map(i => i.id)).size).toBe(12)
  })

  it('does not mutate the binding it was given', () => {
    expect(items[0].textConfig?.subject).toBe('{thing_type_code}.{thing}.state')
  })

  it('leaves an untemplated field alone', () => {
    const fixed = resolveItemsForThing(
      [{ id: 'x', type: 'text', label: 'T', textConfig: { subject: 'plain.subject' } }],
      doorType,
      door,
      ctx
    )
    expect(fixed[0].textConfig?.subject).toBe('plain.subject')
  })
})

describe('markerStateFor', () => {
  const now = 1_000_000
  const b = binding({
    mappings: [
      { id: 'm1', value: 'secure', color: 'green', label: 'Secure' },
      { id: 'm2', value: 'forced', color: 'red', label: 'Forced', blink: true },
    ],
    defaultColor: 'blue',
    showStale: true,
    stalenessThreshold: 60_000,
    staleColor: 'grey',
    staleLabel: 'No contact',
  })

  it('maps a value to its colour and label', () => {
    const s = markerStateFor(b, 'secure', now - 1_000, now)
    expect(s).toMatchObject({ color: 'green', label: 'Secure', stale: false })
  })

  it('carries blink through', () => {
    expect(markerStateFor(b, 'forced', now - 1_000, now).blink).toBe(true)
  })

  it('goes stale BEFORE matching, so a dead device is never drawn green', () => {
    // The whole point of the ordering: `secure` is still the last value.
    const s = markerStateFor(b, 'secure', now - 61_000, now)
    expect(s).toMatchObject({ color: 'grey', label: 'No contact', stale: true, blink: false })
  })

  it('stays on its value when staleness is switched off', () => {
    const noStale = { ...b, showStale: false }
    expect(markerStateFor(noStale, 'secure', now - 10 * 60_000, now).color).toBe('green')
  })

  it('reads as unknown before anything has arrived', () => {
    expect(markerStateFor(b, null, null, now)).toMatchObject({ label: 'No data', stale: true })
  })

  it('falls back to the default for an unmapped value', () => {
    const s = markerStateFor(b, 'propped', now, now)
    expect(s).toMatchObject({ color: 'blue', label: 'propped' })
  })

  it('honours a wildcard mapping only when nothing matches exactly', () => {
    const withStar = {
      ...b,
      mappings: [...(b.mappings || []), { id: 'm3', value: '*', color: 'amber', label: 'Seen' }],
    }
    expect(markerStateFor(withStar, 'secure', now, now).color).toBe('green')
    expect(markerStateFor(withStar, 'anything', now, now).color).toBe('amber')
  })

  it('matches an object payload by its JSON form, as the status widget does', () => {
    expect(displayValue({ a: 1 })).toBe('{"a":1}')
    const objMatch = { ...b, mappings: [{ id: 'm', value: '{"a":1}', color: 'teal', label: 'Obj' }] }
    expect(markerStateFor(objMatch, { a: 1 }, now, now).color).toBe('teal')
  })

  it('is unknown for no binding at all', () => {
    expect(markerStateFor(undefined, 'secure', now, now).label).toBe('No data')
  })
})

describe('bindingForThing', () => {
  it('matches on the thing type', () => {
    expect(bindingForThing([binding()], door)?.id).toBe('b1')
  })

  it('is undefined for an untyped thing', () => {
    expect(bindingForThing([binding()], { ...door, type: undefined })).toBeUndefined()
  })

  it('takes the first of two bindings on one type rather than merging them', () => {
    const second = binding({ id: 'b2' })
    expect(bindingForThing([binding(), second], door)?.id).toBe('b1')
  })
})

describe('isPlaced', () => {
  it('needs both coordinates', () => {
    expect(isPlaced({ id: 'a', floorplan_position: { x: 1, y: 2 } })).toBe(true)
    expect(isPlaced({ id: 'b' })).toBe(false)
    expect(isPlaced({ id: 'c', floorplan_position: { x: 1 } as any })).toBe(false)
  })

  it('accepts a zero coordinate, which is the top-left corner and not absence', () => {
    expect(isPlaced({ id: 'z', floorplan_position: { x: 0, y: 0 } })).toBe(true)
  })
})
