package hooks

import (
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Role sets for requireMembership. Named rather than written inline at each call
// site so that "who may act on their own organization's infrastructure" is one
// list, not a phrase re-typed per route.
var (
	// rolesOrgManager gates the operations that reconfigure an organization's
	// own infrastructure: NATS account signing keys, Nebula CA rotation, the
	// certificate audit. Matches the owner/admin allowlist in schema.json.
	rolesOrgManager = []string{"owner", "admin"}

	// rolesInventory gates creating and editing Things and Locations. `viewer`
	// and `dashboard` are deliberately absent: they read, they do not write.
	rolesInventory = []string{"owner", "admin", "member"}
)

// requireMembership returns the caller's active organization and their role in
// it, refusing anyone whose role is not in `roles`.
//
// WHY THIS IS SHARED. Routes that write with app.Save() bypass every API rule,
// so each check the rules would have made has to be restated in the route — and
// the restatement is the same three guards every time. Two of them are the ones
// this codebase has actually been burned by:
//
//   - The organization comes from the AUTHENTICATED RECORD, never the request
//     body, so a route cannot be aimed at another tenant.
//   - It must be NON-BLANK. `users.current_organization` is blanked by
//     hooks/membership_lifecycle.go on the way out of an organization, and a
//     blank scope compared against a blank column is how the empty-string
//     tenancy collapse read across tenants. One place enforcing this beats a
//     fourth route hand-rolling it and forgetting.
//   - The membership must CORRELATE. `memberships.organization` is required and
//     cascade, so it can never be blank -- which kills the blank sentinel by
//     construction rather than by a comparison someone may later tidy away.
//
// It is in its own file, rather than beside its first caller, precisely so the
// next org-scoped route finds it. The version this replaced lived inside
// nebula_routes.go and was already being called from nats_account_routes.go,
// which is exactly how a helper goes unnoticed and gets re-typed.
//
// The denial is deliberately one error for both "not a member" and "wrong
// role", matching how an API rule rejects without revealing which half failed.
//
// NOT USED BY resolveOwnNatsUser (hooks/credential_routes.go), and that is
// deliberate — see the note there.
func requireMembership(re *core.RequestEvent, membershipCollection string, roles []string) (string, string, error) {
	// A programming error, not a client one. An empty role list would build a
	// filter with no role clause and quietly match ANY membership, which is the
	// shape of every authorization bug this codebase has had. Callers that
	// genuinely want any role must say so somewhere that can be read, not by
	// passing nothing here.
	if len(roles) == 0 {
		return "", "", re.InternalServerError("requireMembership called with no roles", nil)
	}

	if re.Auth == nil || re.Auth.Collection().Name != "users" {
		return "", "", re.UnauthorizedError("user authentication required", nil)
	}

	orgID := re.Auth.GetString("current_organization")
	if orgID == "" {
		return "", "", re.BadRequestError("no active organization selected", nil)
	}

	// Built from `roles` rather than written as a literal so there is one
	// construction instead of one hand-typed variant per route. Role values go
	// through dbx params, never string concatenation.
	params := dbx.Params{"user": re.Auth.Id, "org": orgID}
	clauses := make([]string, len(roles))
	for i, role := range roles {
		key := "role" + strconv.Itoa(i)
		clauses[i] = "role = {:" + key + "}"
		params[key] = role
	}

	membership, err := re.App.FindFirstRecordByFilter(
		membershipCollection,
		"user = {:user} && organization = {:org} && ("+strings.Join(clauses, " || ")+")",
		params,
	)
	// No `|| membership == nil` guard: FindFirstRecordByFilter returns
	// sql.ErrNoRows when nothing matches and never (nil, nil)
	// (core/record_query.go). The two helpers this replaced both carried that
	// check and it could never fire.
	if err != nil {
		return "", "", re.ForbiddenError(membershipDenial(roles), nil)
	}

	return orgID, membership.GetString("role"), nil
}

// membershipDenial names the roles that would have been accepted.
//
// Listing them beats a generic refusal: the reader is usually an operator who
// thinks they should have access, and the answer they need is which role does.
//
// Written so it cannot panic on ANY input, including an empty slice. The guard
// at the top of requireMembership already makes that unreachable, but a
// rail-guard whose safety depends on a check forty lines away is the wrong
// shape -- and the first version did panic there, which is a worse failure than
// the wildcard it was protecting against.
func membershipDenial(roles []string) string {
	subject := strings.Join(roles, ", ")
	if n := len(roles); n > 1 {
		subject = strings.Join(roles[:n-1], ", ") + " or " + roles[n-1]
	}
	return subject + " of the active organization required"
}
