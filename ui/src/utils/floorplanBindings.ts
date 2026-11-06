// ui/src/utils/floorplanBindings.ts
//
// Everything the floor plan widget decides about a binding, as pure functions,
// so floorplanBindings.spec.ts can reach it (nothing renders in CI).
//
// Subject substitution belongs to utils/subjectResolver.ts. This file only
// picks which resolution a caller wants: `resolveRolePattern` wildcards
// {thing} for the one subscription a binding needs, and `resolveThing` gives
// one Thing's concrete subject, which is how a message is matched to a marker.

import {
  join,
  resolveRolePattern,
  resolveThing,
  type ThingContext,
} from './subjectResolver'
import type {
  FloorPlanTypeBinding,
  MapMarkerItem,
  StatusMapping,
} from '@/types/dashboard'

/** The fields of a Thing this file needs. */
export interface BindableThing {
  id: string
  code?: string
  name?: string
  type?: string
  floorplan_position?: { x: number; y: number }
}

/** The fields of a Thing Type this file needs. */
export interface BindableType {
  id: string
  code?: string
  subject_prefix?: string
}

/** What is the same for every marker on the plan. Codes, never ids. */
export interface FloorPlanContext {
  orgCode: string
  locationCode: string
}

// ---------------------------------------------------------------------------
// Subjects and keys
// ---------------------------------------------------------------------------

/** The binding's subject template. `join` supplies the default prefix. */
export function bindingTemplate(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
): string | null {
  if (binding.stateSource?.mode !== 'subject') return null
  return join(type?.subject_prefix, binding.stateSource.subjectSuffix?.trim() || '')
}

/**
 * The one subscription that covers every Thing of this type. One per binding
 * rather than one per marker is what makes a 200-marker cap affordable.
 */
export function bindingSubscriptionSubject(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
  ctx: FloorPlanContext,
): string | null {
  const tmpl = bindingTemplate(binding, type)
  if (!tmpl) return null
  return resolveRolePattern(tmpl, {
    org: ctx.orgCode,
    location: ctx.locationCode,
    thingTypeCode: type?.code,
  })
}

/** The concrete subject for one Thing — what a buffered message is matched on. */
export function bindingSubjectForThing(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
  thing: BindableThing,
  ctx: FloorPlanContext,
): string | null {
  const tmpl = bindingTemplate(binding, type)
  if (!tmpl) return null
  return resolveThing(tmpl, thingContext(type, thing, ctx))
}

/** The KV key pattern one watcher needs to cover every Thing of this type. */
export function bindingKvPattern(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
  ctx: FloorPlanContext,
): string | null {
  const src = binding.stateSource
  if (src?.mode !== 'kv' || !src.kvBucket || !src.kvKeyTemplate) return null
  return resolveRolePattern(src.kvKeyTemplate, {
    org: ctx.orgCode,
    location: ctx.locationCode,
    thingTypeCode: type?.code,
  })
}

/** The concrete KV key for one Thing. */
export function bindingKvKeyForThing(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
  thing: BindableThing,
  ctx: FloorPlanContext,
): string | null {
  const src = binding.stateSource
  if (src?.mode !== 'kv' || !src.kvBucket || !src.kvKeyTemplate) return null
  return resolveThing(src.kvKeyTemplate, thingContext(type, thing, ctx))
}

/**
 * A Thing's substitution context. Falls back to the id for a Thing with no
 * code, as the Publisher widget does, so {thing} never stays literal.
 */
export function thingContext(
  type: BindableType | undefined,
  thing: BindableThing,
  ctx: FloorPlanContext,
): ThingContext {
  return {
    org: ctx.orgCode,
    location: ctx.locationCode,
    thing: thing.code || thing.id,
    thingTypeCode: type?.code,
  }
}

// ---------------------------------------------------------------------------
// Panel items
// ---------------------------------------------------------------------------

/**
 * Resolve every addressing field on a marker item. A config left out of this
 * list fails silently (it subscribes to a literal `{thing}`), so the spec
 * asserts each one.
 */
function resolveItem(item: MapMarkerItem, ctx: ThingContext): MapMarkerItem {
  const r = (v: string | undefined) => (v ? resolveThing(v, ctx) : v)
  const out: MapMarkerItem = { ...item }

  if (out.subject) out.subject = resolveThing(out.subject, ctx)
  if (out.payload) out.payload = resolveThing(out.payload, ctx)

  if (out.textConfig) {
    out.textConfig = { ...out.textConfig, subject: resolveThing(out.textConfig.subject, ctx) }
  }
  if (out.kvConfig) {
    out.kvConfig = {
      ...out.kvConfig,
      kvBucket: resolveThing(out.kvConfig.kvBucket, ctx),
      kvKey: resolveThing(out.kvConfig.kvKey, ctx),
    }
  }
  if (out.switchConfig) {
    out.switchConfig = {
      ...out.switchConfig,
      kvBucket: r(out.switchConfig.kvBucket),
      kvKey: r(out.switchConfig.kvKey),
      publishSubject: r(out.switchConfig.publishSubject),
      stateSubject: r(out.switchConfig.stateSubject),
    }
  }
  if (out.sliderConfig) {
    out.sliderConfig = {
      ...out.sliderConfig,
      kvBucket: r(out.sliderConfig.kvBucket),
      kvKey: r(out.sliderConfig.kvKey),
      publishSubject: r(out.sliderConfig.publishSubject),
      stateSubject: r(out.sliderConfig.stateSubject),
    }
  }
  if (out.gaugeConfig) {
    out.gaugeConfig = {
      ...out.gaugeConfig,
      kvBucket: r(out.gaugeConfig.kvBucket),
      kvKey: r(out.gaugeConfig.kvKey),
      subject: r(out.gaugeConfig.subject),
    }
  }
  return out
}

/**
 * A binding's items, addressed at one Thing. The id gets the Thing's id
 * appended: item components subscribe under `item.id`, so sixty doors sharing
 * one id would share one buffer.
 */
export function resolveItemsForThing(
  items: MapMarkerItem[],
  type: BindableType | undefined,
  thing: BindableThing,
  ctx: FloorPlanContext,
): MapMarkerItem[] {
  const tctx = thingContext(type, thing, ctx)
  return items.map(item => ({ ...resolveItem(item, tctx), id: `${item.id}__${thing.id}` }))
}

// ---------------------------------------------------------------------------
// Marker state
// ---------------------------------------------------------------------------

/** One reading for one marker: what arrived, and when. */
export interface StateSample {
  value: unknown
  timestamp: number
}

export interface MarkerState {
  color: string
  label: string
  blink: boolean
  stale: boolean
}

/**
 * The colour for "nothing to show". A literal, not a theme token: a marker
 * sits on an uploaded image, not on the console (see markerIcon.ts).
 */
export const NEUTRAL_MARKER = '#8b9099'

/** What a marker shows before anything has arrived for it. */
export const UNKNOWN_STATE: MarkerState = {
  color: NEUTRAL_MARKER,
  label: 'No data',
  blink: false,
  stale: true,
}

/** The displayed form of a value, matching StatusWidget's: objects as JSON. */
export function displayValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

/**
 * Value plus arrival time to a marker's colour and label, resolved the same
 * way StatusWidget does. Staleness is checked before the value: a device that
 * went quiet an hour ago must not keep drawing its last colour.
 */
export function markerStateFor(
  binding: FloorPlanTypeBinding | undefined,
  value: unknown,
  timestamp: number | null,
  now: number,
): MarkerState {
  if (!binding || timestamp === null) return UNKNOWN_STATE

  const showStale = binding.showStale ?? true
  const threshold = binding.stalenessThreshold || 60_000
  if (showStale && now - timestamp > threshold) {
    return {
      color: binding.staleColor || NEUTRAL_MARKER,
      label: binding.staleLabel || 'Stale',
      blink: false,
      stale: true,
    }
  }

  const valStr = displayValue(value)
  const mappings: StatusMapping[] = binding.mappings || []
  const match = mappings.find(m => m.value === valStr) || mappings.find(m => m.value === '*')

  if (match) {
    return {
      color: match.color,
      label: match.label || valStr,
      blink: !!match.blink,
      stale: false,
    }
  }

  return {
    color: binding.defaultColor || 'var(--color-info)',
    label: binding.defaultLabel || valStr || 'Unknown',
    blink: false,
    stale: false,
  }
}

/**
 * Which binding applies to a Thing. First match wins: two bindings on one type
 * is a config mistake (the editor prevents it), not something to merge.
 */
export function bindingForThing(
  bindings: FloorPlanTypeBinding[],
  thing: BindableThing,
): FloorPlanTypeBinding | undefined {
  if (!thing.type) return undefined
  return bindings.find(b => b.thingTypeId === thing.type)
}

/** True when a Thing has a complete position on its location's floor plan. */
export function isPlaced<T extends BindableThing>(
  t: T,
): t is T & { floorplan_position: { x: number; y: number } } {
  const p = t.floorplan_position
  return !!p && typeof p.x === 'number' && typeof p.y === 'number'
}
