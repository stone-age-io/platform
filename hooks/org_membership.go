package hooks

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
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
// The membership is the ONLY thing that grants authority: no API rule consults
// `organizations.owner`, because a standing grant on that field would outlive
// the owner membership (a removed owner could re-add themselves). So an owner
// with no membership row can do nothing in their own tenant, and the two are
// created together.
//
// The handler must call e.Next(): the pb-tenancy original did not, which
// silently dropped every handler bound after it (hooks/CLAUDE.md,
// TestOrgCreateDoesNotTerminateTheHookChain). It checks for an existing row
// because the unique (user, organization) index would refuse a second create.
// Registered in main.go after provisioning and managed exports.
func RegisterOrgMembership(app *pocketbase.PocketBase, opts OrgMembershipOptions) {
	app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		if err := ensureOwnerMembershipAndContext(e.App, e.Record, opts); err != nil {
			return err
		}
		return e.Next()
	})

	// An operator reassigning the owner (admin/OrganizationFormView) gives the
	// new owner an owner membership. Membership only: a reassignment does not
	// move anyone's console context.
	//
	// The REQUEST hook, edge-keyed, so it fires only when the owner changes: a
	// level-triggered version would re-add an owner an admin had removed on the
	// next unrelated save. A request loads the row fresh, so Original() is the
	// pre-edit owner. The previous owner keeps whatever membership they had.
	app.OnRecordUpdateRequest(opts.OrgCollection).BindFunc(func(e *core.RecordRequestEvent) error {
		was := e.Record.Original().GetString("owner")
		if err := e.Next(); err != nil {
			return err
		}
		if e.Record.GetString("owner") == was {
			return nil
		}
		return ensureOwnerMembership(e.App, e.Record, opts)
	})
}

func ensureOwnerMembershipAndContext(app core.App, org *core.Record, opts OrgMembershipOptions) error {
	ownerID := org.GetString("owner")
	if ownerID == "" {
		// Unreachable through the API (owner is required), but a SaveNoValidate
		// seed or migration could create a membership belonging to nobody.
		return nil
	}

	if err := ensureOwnerMembership(app, org, opts); err != nil {
		return err
	}

	// Point the owner at the organization they now own, unconditionally. Invite
	// acceptance sets this only when blank, since joining someone else's org
	// should not move you out of your own. The asymmetry is deliberate.
	user, err := app.FindRecordById(opts.UserCollection, ownerID)
	if err != nil {
		// A dangling owner relation: logged, not worth failing a create over.
		app.Logger().Warn("organization owner has no user record, context not set",
			"organization", org.Id, "owner", ownerID, "error", err)
		return nil
	}

	user.Set("current_organization", org.Id)
	if err := app.Save(user); err != nil {
		// The membership, which grants capability, is already written; a context
		// that did not move is a switcher click.
		app.Logger().Warn("could not set current_organization for new organization owner",
			"organization", org.Id, "owner", ownerID, "error", err)
	}

	return nil
}

// ensureOwnerMembership makes the organization's owner hold an owner membership
// in it: created if missing, promoted if they were already a lesser member.
func ensureOwnerMembership(app core.App, org *core.Record, opts OrgMembershipOptions) error {
	ownerID := org.GetString("owner")
	if ownerID == "" {
		return nil
	}

	membership, _ := app.FindFirstRecordByFilter(
		opts.MembershipCollection,
		"user = {:user} && organization = {:org}",
		dbx.Params{"user": ownerID, "org": org.Id},
	)

	if membership == nil {
		collection, err := app.FindCollectionByNameOrId(opts.MembershipCollection)
		if err != nil {
			return fmt.Errorf("find %s: %w", opts.MembershipCollection, err)
		}
		membership = core.NewRecord(collection)
		membership.Set("user", ownerID)
		membership.Set("organization", org.Id)
	} else if membership.GetString("role") == "owner" {
		return nil
	}

	membership.Set("role", "owner")
	if err := app.Save(membership); err != nil {
		return fmt.Errorf("save owner membership: %w", err)
	}
	return nil
}
