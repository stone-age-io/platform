package hooks

import (
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// MembershipLifecycleOptions names the collections this file keeps in step.
type MembershipLifecycleOptions struct {
	MembershipCollection string
	UserCollection       string
}

// RegisterMembershipLifecycle closes a user's tenant context when the
// membership that justified it is removed.
//
// users.current_organization is the read scope every org-scoped rule keys on,
// so a value the rules trust needs something keeping it true. The read rules
// also correlate a membership (migrations/schema_update_tenancy_sentinel.go),
// so a removed member loses reads with the membership; this hook keeps the
// context honest, so the console does not land them in an org they cannot read.
//
// Clear, do not switch: the console's loadContext() falls back to the first
// remaining membership (ui/src/stores/auth.ts), and a server-side guess could
// drop someone into a tenant they did not choose.
//
// No RefreshTokenKey(), unlike hooks/active_flag.go: rules read
// @request.auth.current_organization from the user record, which is loaded on
// every request, so clearing it takes effect at once. Refreshing the key would
// also sign the user out of the orgs they still belong to.
//
// Bound to OnRecordDelete, BEFORE the delete, not OnRecordAfterDeleteSuccess:
// After*Success runs after the commit, so a failure there would fail open. Here
// a failure aborts the removal, and the worst case is a context cleared for a
// membership that still exists, which the org switcher fixes.
func RegisterMembershipLifecycle(app *pocketbase.PocketBase, opts MembershipLifecycleOptions) {
	app.OnRecordDelete(opts.MembershipCollection).BindFunc(func(e *core.RecordEvent) error {
		userID := e.Record.GetString("user")
		orgID := e.Record.GetString("organization")

		if userID == "" || orgID == "" {
			return e.Next()
		}

		user, err := e.App.FindRecordById(opts.UserCollection, userID)
		if err != nil {
			// The user is being deleted and this membership is cascading.
			return e.Next()
		}

		if user.GetString("current_organization") != orgID {
			// Removed from an organization other than the one they are in.
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
