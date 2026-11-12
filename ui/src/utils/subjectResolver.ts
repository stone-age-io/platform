// Subject template resolver. The only implementation (there is no Go
// counterpart). The contract is documented in platform-docs:
// docs/thing-types.md and docs/connectivity.md.
//
// Substitution itself is utils/templates.ts. This file only decides which
// values a Thing, or a role covering every Thing of a type, supplies.

import { fillTemplate } from './templates'

export const VAR_ORG = '{org}'
export const VAR_LOCATION = '{location}'
export const VAR_THING = '{thing}'
export const VAR_THING_TYPE_CODE = '{thing_type_code}'

// Family-first, and location-free (ADR 0003 in platform-docs).
//
// Family-first so one JetStream stream can bind `sensor.>`. It must lead with a
// literal token: a wildcard-leading filter overlaps every other, and JetStream
// refuses overlapping streams.
//
// No `{location}`, because things move and the subject would move with them.
// A Thing code is already unique in its org, which is the NATS account. A Thing
// Type can still opt into `{location}` with an explicit prefix.
export const DEFAULT_PREFIX = `${VAR_THING_TYPE_CODE}.${VAR_THING}`

export interface ThingContext {
  org?: string
  location?: string
  thing?: string
  thingTypeCode?: string
}

export interface RolePatternContext {
  org?: string
  location?: string
  thingTypeCode?: string
}

export function join(prefix: string | undefined | null, suffix: string | undefined | null): string {
  const p = prefix && prefix.length > 0 ? prefix : DEFAULT_PREFIX
  if (!suffix) return p
  return `${p}.${suffix}`
}

// Substitute every reserved variable against a concrete Thing. Unset fields
// are left as literal template tokens so callers can detect incomplete input.
export function resolveThing(tmpl: string, ctx: ThingContext): string {
  return fillTemplate(tmpl, {
    org: ctx.org || undefined,
    location: ctx.location || undefined,
    thing: ctx.thing || undefined,
    thing_type_code: ctx.thingTypeCode || undefined,
  })
}

// A variable token left after resolution, in either spelling. On a Thing's page
// it means a context value was missing and the subject is unusable; on the type
// form it is expected.
const UNRESOLVED = /\{\s*(org|location|thing|thing_type_code)\s*\}/

export function hasUnresolved(subject: string): boolean {
  return UNRESOLVED.test(subject)
}

export interface OperationLike {
  id: string
  name: string
  capability: string
  subject_suffix: string
}

export interface OperationSubject {
  id: string
  name: string
  capability: string
  subject: string
  /**
   * The resolved halves of `subject`: the type's shared prefix and this
   * operation's suffix ('' if none). `subject` is exactly `prefix.suffix`, or
   * `prefix` alone.
   */
  prefix: string
  suffix: string
}

// What the device sends, then what it answers. Unlisted capabilities sort last.
const CAPABILITY_ORDER = ['publish', 'request', 'subscribe', 'reply']

// Every operation of a type with its resolved subject. Shared by the Thing page
// and the type form so they agree.
export function resolveOperationSubjects(
  prefix: string | undefined | null,
  operations: OperationLike[],
  ctx: ThingContext,
): OperationSubject[] {
  const rank = (c: string) => {
    const i = CAPABILITY_ORDER.indexOf(c)
    return i === -1 ? CAPABILITY_ORDER.length : i
  }
  // Resolving the halves separately is safe: no variable contains a `.`, so
  // no substitution can span the join.
  const head = resolveThing(join(prefix, ''), ctx)
  return [...operations]
    .sort((a, b) => rank(a.capability) - rank(b.capability) || a.name.localeCompare(b.name))
    .map(op => {
      const suffix = op.subject_suffix ? resolveThing(op.subject_suffix, ctx) : ''
      return {
        id: op.id,
        name: op.name,
        capability: op.capability,
        subject: suffix ? `${head}.${suffix}` : head,
        prefix: head,
        suffix,
      }
    })
}

// Produce a NATS role-level subject pattern:
//   - {thing} always becomes "*"
//   - {location} becomes the supplied value or "*" if empty
//   - {org} / {thing_type_code} substitute when supplied, else remain literal
export function resolveRolePattern(tmpl: string, ctx: RolePatternContext = {}): string {
  return fillTemplate(tmpl, {
    thing: '*',
    location: ctx.location || '*',
    org: ctx.org || undefined,
    thing_type_code: ctx.thingTypeCode || undefined,
  })
}
