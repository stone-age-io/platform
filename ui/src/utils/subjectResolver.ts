// Subject template resolver — TS mirror of internal/subjectresolver/resolver.go.
// Keep the reserved variable set and behavior in sync with the Go package.

export const VAR_ORG = '{org}'
export const VAR_LOCATION = '{location}'
export const VAR_THING = '{thing}'
export const VAR_THING_TYPE_CODE = '{thing_type_code}'

export const DEFAULT_PREFIX = `${VAR_LOCATION}.${VAR_THING_TYPE_CODE}.${VAR_THING}`

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
