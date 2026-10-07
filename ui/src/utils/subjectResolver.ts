// Subject template resolver. There is no Go counterpart — this file is the only
// implementation. (It claimed to mirror internal/subjectresolver/resolver.go for
// as long as it existed; that package has never been written, so "keep the two in
// sync" was an instruction nobody could follow.) The contract it implements is
// documented in platform-docs: docs/thing-types.md and docs/connectivity.md.

export const VAR_ORG = '{org}'
export const VAR_LOCATION = '{location}'
export const VAR_THING = '{thing}'
export const VAR_THING_TYPE_CODE = '{thing_type_code}'

// Family-first, and location-free (ADR 0003 in platform-docs).
//
// Family-first so that ONE JetStream stream can bind `sensor.>` and capture
// every sensor. It also has to lead with a literal token: a wildcard-leading
// subject filter intersects every literal-rooted one, and JetStream refuses to
// create two streams whose filters overlap, so no two thing types could ever own
// a stream apiece.
//
// No `{location}`, because things move. The subject is resolved from a Thing's
// CURRENT location, so a relocated camera would start publishing under new
// subjects, its history split across two sites and its NATS permissions aimed at
// the old one. Uniqueness never needed the location: a Thing code is unique in
// its organization, and the organization is the NATS account. `{location}` stays
// a supported variable for a Thing Type that opts in with an explicit prefix
// (fixed equipment such as a freezer bank), and resolveRolePattern below still
// wildcards it when no location is supplied.
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

function applyReplacements(tmpl: string, pairs: Array<[string, string]>): string {
  return pairs.reduce((acc, [key, val]) => acc.split(key).join(val), tmpl)
}

// Substitute every reserved variable against a concrete Thing. Unset fields
// are left as literal template tokens so callers can detect incomplete input.
export function resolveThing(tmpl: string, ctx: ThingContext): string {
  const pairs: Array<[string, string]> = []
  if (ctx.org) pairs.push([VAR_ORG, ctx.org])
  if (ctx.location) pairs.push([VAR_LOCATION, ctx.location])
  if (ctx.thing) pairs.push([VAR_THING, ctx.thing])
  if (ctx.thingTypeCode) pairs.push([VAR_THING_TYPE_CODE, ctx.thingTypeCode])
  return applyReplacements(tmpl, pairs)
}

// A variable token still present after resolution. On a Thing's own page this
// means a context value was missing (no location for a `{location}` prefix, an
// organization without a code) and the subject shown is not one the device can
// actually use. On the type form, where no Thing exists, it is expected.
const UNRESOLVED = /\{(org|location|thing|thing_type_code)\}/

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
   * The two halves of `subject`, resolved: the type's prefix, shared by every
   * operation of the type, and this operation's own suffix ('' when it has
   * none). `subject` is exactly `prefix + '.' + suffix`, or `prefix` alone, so
   * a view can draw the shared half quietly without re-parsing the string.
   */
  prefix: string
  suffix: string
}

// Speaking order: what the device sends, then what it answers. Anything not
// listed (a capability added later) sorts last rather than disappearing.
const CAPABILITY_ORDER = ['publish', 'request', 'subscribe', 'reply']

// Every operation of a type as the full subject it resolves to for `ctx` —
// the list a Thing page and the type form both show, so the two cannot
// disagree about what a device speaks on.
export function resolveOperationSubjects(
  prefix: string | undefined | null,
  operations: OperationLike[],
  ctx: ThingContext,
): OperationSubject[] {
  const rank = (c: string) => {
    const i = CAPABILITY_ORDER.indexOf(c)
    return i === -1 ? CAPABILITY_ORDER.length : i
  }
  // Resolving the halves separately gives the same string as resolving the
  // joined template: every variable is a whole token and `.` never sits inside
  // one, so no substitution can span the join.
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
  const location = ctx.location && ctx.location.length > 0 ? ctx.location : '*'
  const pairs: Array<[string, string]> = [
    [VAR_THING, '*'],
    [VAR_LOCATION, location],
  ]
  if (ctx.org) pairs.push([VAR_ORG, ctx.org])
  if (ctx.thingTypeCode) pairs.push([VAR_THING_TYPE_CODE, ctx.thingTypeCode])
  return applyReplacements(tmpl, pairs)
}
