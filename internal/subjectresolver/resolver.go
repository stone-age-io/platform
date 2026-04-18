// Package subjectresolver turns Thing Type subject templates into concrete
// NATS subject strings.
//
// Templates are stored literally on thing_types.subject_prefix and
// thing_type_operations.subject_suffix — e.g. "{location}.camera.{thing}".
// Two resolution modes exist:
//
//   - ResolveThing: substitutes every reserved variable against a concrete
//     Thing's context. Used by UI features that target a single Thing.
//   - ResolveRolePattern: substitutes {thing} and (optionally) {location} with
//     NATS wildcards to produce a role-level subject pattern. Used by the
//     thing_types -> nats_role sync hook.
//
// The reserved variable set is pinned here and mirrored in the TS resolver.
package subjectresolver

import "strings"

// Reserved variable names. Kept as constants so both resolvers use the same
// identifiers and misuse surfaces at compile time.
const (
	VarOrg           = "{org}"
	VarLocation      = "{location}"
	VarThing         = "{thing}"
	VarThingTypeCode = "{thing_type_code}"
)

// DefaultPrefix is the convention consumers apply when a Thing Type has an
// empty subject_prefix. The spec defines this in §4.
const DefaultPrefix = VarLocation + "." + VarThingTypeCode + "." + VarThing

// ThingContext captures the variable values needed to resolve a template for
// a specific Thing. Empty fields are left unsubstituted.
type ThingContext struct {
	Org           string
	Location      string
	Thing         string
	ThingTypeCode string
}

// Join combines a prefix template and a suffix template into a single subject
// template. If prefix is empty, DefaultPrefix is used. Suffix is appended with
// a single dot separator when present.
func Join(prefix, suffix string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	if suffix == "" {
		return prefix
	}
	return prefix + "." + suffix
}

// ResolveThing substitutes every reserved variable in tmpl against ctx. Unset
// context fields are left as literal template tokens, which lets callers
// detect incomplete context rather than silently emitting a broken subject.
func ResolveThing(tmpl string, ctx ThingContext) string {
	repl := make([]string, 0, 8)
	if ctx.Org != "" {
		repl = append(repl, VarOrg, ctx.Org)
	}
	if ctx.Location != "" {
		repl = append(repl, VarLocation, ctx.Location)
	}
	if ctx.Thing != "" {
		repl = append(repl, VarThing, ctx.Thing)
	}
	if ctx.ThingTypeCode != "" {
		repl = append(repl, VarThingTypeCode, ctx.ThingTypeCode)
	}
	if len(repl) == 0 {
		return tmpl
	}
	return strings.NewReplacer(repl...).Replace(tmpl)
}

// RolePatternContext scopes a role-pattern resolution. Location may be set to
// a specific location code when the role is scoped to one location; leave it
// empty to wildcard the location segment.
type RolePatternContext struct {
	Org           string
	Location      string
	ThingTypeCode string
}

// ResolveRolePattern substitutes reserved variables with values suitable for
// a NATS role-level permission pattern:
//
//   - {thing} always becomes "*" (roles are per-type, not per-Thing).
//   - {location} becomes the supplied Location or "*" if empty.
//   - {org} and {thing_type_code} substitute when supplied, else remain literal.
func ResolveRolePattern(tmpl string, ctx RolePatternContext) string {
	loc := ctx.Location
	if loc == "" {
		loc = "*"
	}
	repl := []string{
		VarThing, "*",
		VarLocation, loc,
	}
	if ctx.Org != "" {
		repl = append(repl, VarOrg, ctx.Org)
	}
	if ctx.ThingTypeCode != "" {
		repl = append(repl, VarThingTypeCode, ctx.ThingTypeCode)
	}
	return strings.NewReplacer(repl...).Replace(tmpl)
}
