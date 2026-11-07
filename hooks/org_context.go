package hooks

import (
	"strconv"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Role sets for requireMembership, named so each is one list rather than
// re-typed per route.
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
// Routes that write with app.Save() bypass every API rule, so they restate the
// rules' checks, and these three are the same every time:
//
//   - The organization comes from the AUTHENTICATED RECORD, never the request.
//   - It must be NON-BLANK. hooks/membership_lifecycle.go blanks
//     `users.current_organization` on leaving an org, and a blank scope matches
//     a blank column.
//   - The membership must CORRELATE. `memberships.organization` is required,
//     so it is never blank.
//
// Its own file so the next org-scoped route finds it instead of re-typing it.
// One error for "not a member" and "wrong role", as an API rule would. Not
// used by resolveOwnNatsUser (hooks/credential_routes.go), deliberately; see
// the note there.
func requireMembership(re *core.RequestEvent, membershipCollection string, roles []string) (string, string, error) {
	// A programming error: an empty role list would build a filter with no
	// role clause and match ANY membership.
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

	// Role values go through dbx params, never string concatenation.
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
	// sql.ErrNoRows, never (nil, nil) (core/record_query.go).
	if err != nil {
		return "", "", re.ForbiddenError(membershipDenial(roles), nil)
	}

	return orgID, membership.GetString("role"), nil
}

// membershipDenial names the roles that would have been accepted. It must not
// panic on any input, including an empty slice, without relying on the guard in
// requireMembership.
func membershipDenial(roles []string) string {
	subject := strings.Join(roles, ", ")
	if n := len(roles); n > 1 {
		subject = strings.Join(roles[:n-1], ", ") + " or " + roles[n-1]
	}
	return subject + " of the active organization required"
}
