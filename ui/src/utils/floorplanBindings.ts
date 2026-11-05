// ui/src/utils/floorplanBindings.ts
//
// Everything the floor plan widget decides about a binding, as pure functions.
//
// WHY THIS FILE EXISTS SEPARATELY. Vitest here runs in node and nothing renders
// in CI (ui/CLAUDE.md, Testing), so the only parts of this widget that can be
// guarded are the ones that take data in and give data back. Those are exactly
// the parts that are easy to get wrong: which subject one subscription has to
// cover, which message belongs to which marker, and what a value means once it
// arrives. Keeping them here rather than inside the component is what makes
// floorplanBindings.spec.ts possible.
//
// THE SUBJECT CONTRACT IS NOT REIMPLEMENTED HERE. utils/subjectResolver.ts owns
// it and PublisherWidget already uses it; this file only decides which of its
// two resolutions a caller wants. `resolveRolePattern` turns {thing} into `*`
// and gives the ONE subscription a binding needs; `resolveThing` gives the
// concrete subject for one Thing, which is how a message out of that shared
// buffer is matched back to a marker. Writing either substitution by hand here
// would be a second copy of a contract documented elsewhere.

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

/** The fields of a Thing this file needs. Narrower than the record on purpose. */
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

/** What the widget knows that is the same for every marker on it. */
export interface FloorPlanContext {
  /** The organization CODE, never its id: ids for storage, codes for addressing. */
  orgCode: string
  /** The location CODE, for a Thing Type whose prefix opts into {location}. */
  locationCode: string
}

// ---------------------------------------------------------------------------
// Subjects and keys
// ---------------------------------------------------------------------------

/**
 * The subject template a binding resolves against, before any Thing is known.
 *
 * `join` supplies DEFAULT_PREFIX when the type has no `subject_prefix` of its
 * own, which is the same fallback the Thing detail view and the Publisher
 * widget show, so a binding configured here addresses what those pages say it
 * will.
 */
export function bindingTemplate(
  binding: FloorPlanTypeBinding,
  type: BindableType | undefined,
): string | null {
  if (binding.stateSource?.mode !== 'subject') return null
  return join(type?.subject_prefix, binding.stateSource.subjectSuffix?.trim() || '')
}

/**
 * The one subscription that covers every Thing of this type.
 *
 * This is the whole reason a floor plan can carry 200 markers. `MapWidget`
 * subscribes per item, which is right for ten markers and wrong here: sixty
 * doors with two bindings each would be 120 subscriptions, and the per-type
 * config makes sixty doors the easy case rather than the hard one.
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

/** The KV key prefix one watcher needs to cover every Thing of this type. */
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
 * A Thing's own context for substitution.
 *
 * `thing.code ?? thing.id` matches what the Publisher widget does: a Thing
 * created before codes existed still addresses something rather than leaving
 * `{thing}` literal in a subject nobody can subscribe to. The code is the right
 * answer whenever there is one, because that is what the device publishes under.
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
 * The string fields on a marker item that address something and so need
 * resolving.
 *
 * Every config object with a subject or a key has to be listed here. A new
 * item type whose config is left out fails SILENTLY and uniformly: the row
 * subscribes to a literal `{thing}`, which is a legal subject nothing ever
 * publishes on, so it renders as a row that is simply always empty.
 * floorplanBindings.spec.ts asserts each one.
 */
function resolveItem(item: MapMarkerItem, ctx: ThingContext): MapMarkerItem {
  const r = (v: string | undefined) => (v ? resolveThing(v, ctx) : v)
  const out: MapMarkerItem = { ...item }

  if (out.subject) out.subject = resolveThing(out.subject, ctx)
  // The payload too: a command body that names the device it is for is the
  // ordinary case, and leaving it literal would publish `{thing}` to a device.
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
 * A binding's items, addressed at one Thing.
 *
 * The item id is suffixed with the Thing's id. Every MarkerItem* component
 * subscribes under an id derived from `item.id` (`marker-text-${item.id}`), so
 * leaving the binding's own id on sixty copies of one item would have sixty
 * doors share one subscription and one buffer — every door showing whichever
 * one published last.
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
 * The colour a marker falls back to with nothing to show.
 *
 * A LITERAL, not a theme token, and that is the whole point: `var(--muted)` is
 * near-white at half opacity on the dark theme, which is invisible on the white
 * sheet of an architectural drawing. A marker sits on an image the operator
 * uploaded, so nothing in the console's palette predicts its backdrop. This is
 * a mid neutral that reads on white paper and on a dark plan alike, and the
 * glyph's own halo and hairline (components/widgets/floorplan/markerIcon.ts)
 * carry the contrast the colour cannot.
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
 * Value plus arrival time to a marker's colour and label.
 *
 * Staleness is checked FIRST and that order is the point: a camera that stopped
 * publishing an hour ago still has `online` as its last value, and a plan that
 * keeps drawing it green is worse than one that draws nothing. Same reason
 * StatusWidget checks in this order, and the thresholds are its fields.
 */
export function markerStateFor(
  binding: FloorPlanTypeBinding | undefined,
  value: unknown,
  timestamp: number | null,
  now: number,
): MarkerState {
  if (!binding) return UNKNOWN_STATE

  const showStale = binding.showStale ?? true
  const threshold = binding.stalenessThreshold || 60_000
  const stale = timestamp === null || (showStale && now - timestamp > threshold)

  if (timestamp === null) return UNKNOWN_STATE

  if (stale) {
    return {
      // Same reason as NEUTRAL_MARKER: a stale marker still has to be seen.
      color: binding.staleColor || NEUTRAL_MARKER,
      label: binding.staleLabel || 'Stale',
      blink: false,
      stale: true,
    }
  }

  const valStr = displayValue(value)
  const mappings: StatusMapping[] = binding.mappings || []
  // Exact first, then the catch-all, exactly as StatusWidget resolves it.
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
 * Which binding applies to a Thing. First match wins.
 *
 * Two bindings on one Thing Type is a configuration mistake rather than a
 * merge: the second would fight the first for the marker's colour on every
 * message. The config panel is where that is prevented; this is only what
 * happens if it arrives anyway in an imported dashboard.
 */
export function bindingForThing(
  bindings: FloorPlanTypeBinding[],
  thing: BindableThing,
): FloorPlanTypeBinding | undefined {
  if (!thing.type) return undefined
  return bindings.find(b => b.thingTypeId === thing.type)
}

/** The Things that can be drawn: placed, and within the cap. */
export function placedThings<T extends BindableThing>(things: T[], max: number): T[] {
  const placed = things.filter(t => {
    const p = t.floorplan_position
    return !!p && typeof p.x === 'number' && typeof p.y === 'number'
  })
  return placed.slice(0, max)
}
