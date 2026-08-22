package hooks

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// MembershipLifecycleOptions names the collections this file keeps in step.
type MembershipLifecycleOptions struct {
	MembershipCollection string
	UserCollection       string
}

// RegisterMembershipLifecycle closes a user's tenant context when the
// membership that justified it is removed.
//
// Every read rule on the inventory collections is
// `organization = @request.auth.current_organization`, with no membership or
// role branch -- deliberately, because reads are org-scoped rather than
// role-scoped (see CLAUDE.md). That makes users.current_organization the entire
// read boundary for a console user, and nothing was clearing it when the
// membership behind it went away. Removing someone from an organization
// therefore removed their ability to WRITE (every write branch names a role, and
// resolves it through memberships) while leaving them able to read the whole
// tenant's inventory, indefinitely:
//
//   - the token keeps working for its remaining lifetime, and
//   - `users` has an empty authRule, so they can log back in afterwards and land
//     straight back in the organization they were removed from.
//
// This is the same class of bug as nats_users.active: a value the rules trust
// with nothing keeping it true. The platform already wrote the doctrine down for
// devices -- a flag is not a control unless something acts on it -- and
// current_organization is that flag for humans.
//
// Three choices worth not re-litigating:
//
// Clear, do not switch. The console's loadContext() already falls back to the
// first remaining membership, or to no organization at all when there are none
// (ui/src/stores/auth.ts). Picking a replacement here would be the server
// guessing, and a wrong guess silently drops someone into a tenant they did not
// choose.
//
// No RefreshTokenKey(), unlike hooks/active_flag.go. That file needs it because
// a device's authRule is evaluated at the auth endpoint only, so nothing about
// an existing token re-checks it. Here the rules read
// @request.auth.current_organization out of the user record, which PocketBase
// loads from the database on every request -- so clearing the column takes
// effect on the next request with no token surgery. Refreshing the key would
// also sign the user out of the organizations they are still a legitimate member
// of, which is a second bug rather than extra safety.
//
// Bound to OnRecordDelete and run BEFORE the delete, not to
// OnRecordAfterDeleteSuccess. After*Success fires once the transaction has
// committed, so a failure there leaves the membership gone and the access open
// with nobody watching -- fail-open, for the one control that matters. Running
// first means a failure aborts the removal instead: the operator sees an error
// and retries, and the worst case is a context cleared for a membership that
// still exists, which the org switcher fixes by itself.
func RegisterMembershipLifecycle(app *pocketbase.PocketBase, opts MembershipLifecycleOptions) {
	app.OnRecordDelete(opts.MembershipCollection).BindFunc(func(e *core.RecordEvent) error {
		userID := e.Record.GetString("user")
		orgID := e.Record.GetString("organization")

		if userID == "" || orgID == "" {
			return e.Next()
		}

		user, err := e.App.FindRecordById(opts.UserCollection, userID)
		if err != nil {
			// The user is being deleted, and this membership is one of the
			// records cascading behind them. There is no context left to close.
			return e.Next()
		}

		if user.GetString("current_organization") != orgID {
			// They were removed from some other organization than the one they
			// are looking at. Leave the context alone.
			return e.Next()
		}

		user.Set("current_organization", "")
		if err := e.App.Save(user); err != nil {
			// Aborts the delete. See the note above on why this direction.
			return fmt.Errorf("could not clear the departing member's organization context: %w", err)
		}

		log.Printf("🔒 membership removed: user %s no longer has a context in organization %s",
			userID, orgID)

		return e.Next()
	})
}
