package hooks

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// OrgMembershipOptions names the collections RegisterOrgMembership keeps in step.
type OrgMembershipOptions struct {
	OrgCollection        string
	MembershipCollection string
	UserCollection       string
}

// RegisterOrgMembership gives a new organization's owner the membership row that
// says so, and points them at it.
//
// The membership is not decoration. `organizations.owner` is a relation the rules
// consult directly in a few places, but every write branch on the inventory
// collections resolves authority through `memberships.role` -- so an owner
// without a membership row can be read out of the schema and still cannot write
// anything in their own tenant. The two have to be created together.
//
// ABSORBED FROM pb-tenancy, WITH ONE FIX. This was
// `tenancy.autoCreateOwnerMembership`, and its handler was
// `return autoCreateOwnerMembership(...)` -- no e.Next(). That TERMINATED the
// organizations AfterCreateSuccess chain. Because the library registered it from
// inside its own OnBootstrap callback, it landed after every hooks.Register* call
// in main.go, so it was always last and nothing downstream noticed. Anything
// binding that event after Bootstrap got silence: no handler, no error, no log.
// It cost real time -- RegisterManagedOrgExports on an already-bootstrapped test
// app provisioned nothing, and a Priority: -9999 bind was the only thing that
// fired -- and it is why internal/testutil has to mirror main.go's registration
// order. The e.Next() below is that fix. The ordering constraint in testutil is
// no longer load-bearing, but it is still correct, so it stays.
//
// The existence check is new too. The original always created, and would have
// collided with the unique (user, organization) index on any path that ran twice;
// bootstrap.go's ensureOwnerMembership already took the same defensive stance for
// the same reason.
//
// Registered in main.go in the same position the library's hook effectively
// occupied -- after provisioning and managed exports -- so the observable order
// of work on an organization create is unchanged.
func RegisterOrgMembership(app *pocketbase.PocketBase, opts OrgMembershipOptions) {
	app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		if err := ensureOwnerMembershipAndContext(e.App, e.Record, opts); err != nil {
			return err
		}
		return e.Next()
	})
}

func ensureOwnerMembershipAndContext(app core.App, org *core.Record, opts OrgMembershipOptions) error {
	ownerID := org.GetString("owner")
	if ownerID == "" {
		// organizations.owner is required by the schema, so this is unreachable
		// through the API. It stays because a migration or a seed writing through
		// SaveNoValidate is not, and a blank relation here would produce a
		// membership belonging to nobody.
		return nil
	}

	existing, _ := app.FindFirstRecordByFilter(
		opts.MembershipCollection,
		"user = {:user} && organization = {:org}",
		dbx.Params{"user": ownerID, "org": org.Id},
	)

	if existing == nil {
		collection, err := app.FindCollectionByNameOrId(opts.MembershipCollection)
		if err != nil {
			return fmt.Errorf("find %s: %w", opts.MembershipCollection, err)
		}

		membership := core.NewRecord(collection)
		membership.Set("user", ownerID)
		membership.Set("organization", org.Id)
		membership.Set("role", "owner")

		if err := app.Save(membership); err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}
	}

	// Point the owner at the organization they now own.
	//
	// Unconditional, preserving the absorbed behaviour: creating an organization
	// switches its owner's console context to it, even if they were already in
	// another one. Contrast the invite-acceptance path, which sets this only when
	// blank -- joining someone else's organization should not move you out of your
	// own. Both were inherited as-is; the asymmetry is deliberate enough to keep,
	// since an org you just had created FOR you is almost always where you want to
	// land, and the console has a switcher either way.
	user, err := app.FindRecordById(opts.UserCollection, ownerID)
	if err != nil {
		// The owner relation points at a user that is not there. Not this hook's
		// problem to resolve, and not worth failing a create over, but it is a
		// broken record and nobody else is looking at it.
		app.Logger().Warn("organization owner has no user record, context not set",
			"organization", org.Id, "owner", ownerID, "error", err)
		return nil
	}

	user.Set("current_organization", org.Id)
	if err := app.Save(user); err != nil {
		// The membership is the part that grants capability and it is already
		// written. A context that did not move is a switcher click, not a failure
		// worth unwinding an organization create for.
		app.Logger().Warn("could not set current_organization for new organization owner",
			"organization", org.Id, "owner", ownerID, "error", err)
	}

	return nil
}
