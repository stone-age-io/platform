package hooks_test

import (
	"errors"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// newFlagOrg creates an organization with a fresh owner and hands back the RELOADED
// record.
//
// The reload is load-bearing, for the reason TestRelationTenancy spells out
// about its own: a record built and saved in memory carries an Original() that
// still describes the blank pre-create state, so a hook keyed on a true->false
// transition compares false against false and never fires. Every real caller
// here is the record API, which loads the row before applying the patch. Keeping
// the in-memory object would test a path that does not exist and would have
// passed this suite against a hook that refuses nothing -- which is exactly what
// it did on the first run.
//
// `slug` is separate from `name` because organization names have spaces in them
// and the owner's address is derived from it.
func newFlagOrg(t *testing.T, app *pocketbase.PocketBase, slug, name string, flags map[string]any) *core.Record {
	t.Helper()

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	owner := core.NewRecord(usersCol)
	owner.Set("email", slug+"-owner@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	orgCol, err := app.FindCollectionByNameOrId("organizations")
	if err != nil {
		t.Fatalf("organizations collection: %v", err)
	}
	org := core.NewRecord(orgCol)
	org.Set("name", name)
	org.Set("active", true)
	org.Set("owner", owner.Id)
	for k, v := range flags {
		org.Set(k, v)
	}
	if err := app.Save(org); err != nil {
		t.Fatalf("create organization %q: %v", name, err)
	}

	reloaded, err := app.FindRecordById("organizations", org.Id)
	if err != nil {
		t.Fatalf("reload organization %q: %v", name, err)
	}
	return reloaded
}

// newOrgWithAccount is newFlagOrg plus the NATS account provisioning gave it.
func newOrgWithAccount(t *testing.T, app *pocketbase.PocketBase, slug, name string, flags map[string]any) (*core.Record, *core.Record) {
	t.Helper()

	org := newFlagOrg(t, app, slug, name, flags)

	account, err := app.FindFirstRecordByFilter("nats_accounts",
		"organization = {:org}", dbx.Params{"org": org.Id})
	if err != nil {
		t.Fatalf("provisioning should have created a NATS account for %q: %v", name, err)
	}
	return org, account
}

func accountActive(t *testing.T, app *pocketbase.PocketBase, accountID string) bool {
	t.Helper()
	rec, err := app.FindRecordById("nats_accounts", accountID)
	if err != nil {
		t.Fatalf("reload NATS account: %v", err)
	}
	return rec.GetBool("active")
}

// The whole point of the feature: `active` on an organization is not decoration.
//
// Both directions are asserted on the same record, because a mirror that only
// ever writes false would pass a deactivation test while making reactivation
// impossible -- and reactivation is the half that makes this safe to expose at
// all. Clearing the flag withdraws the account claim; setting it republishes,
// and every creds_file in the tenant is still valid because nothing here
// revokes a user.
func TestOrgDeactivationMirrorsOntoTheNatsAccount(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "mirror", "Mirror Works", nil)

	if !accountActive(t, app, account.Id) {
		t.Fatal("an organization created active should have an active NATS account")
	}

	org.Set("active", false)
	if err := app.Save(org); err != nil {
		t.Fatalf("deactivate the organization: %v", err)
	}
	if accountActive(t, app, account.Id) {
		t.Error("deactivating the organization left its NATS account active: " +
			"the badge says suspended while every device in the tenant is still connected")
	}

	org.Set("active", true)
	if err := app.Save(org); err != nil {
		t.Fatalf("reactivate the organization: %v", err)
	}
	if !accountActive(t, app, account.Id) {
		t.Error("reactivating the organization did not bring its NATS account back")
	}
}

// An organization created with `active` already clear must not be handed a live
// NATS account.
//
// The mirror binds to update only -- there is nothing to compare against on a
// create -- so this is enforced by ensureOrgNatsAccount inheriting the flag
// rather than hardcoding true, which is what it used to do. Without that one
// line the tenant is provisioned connectable and stays that way until somebody
// happens to save the organization again.
func TestAnOrgCreatedInactiveGetsAnInactiveAccount(t *testing.T) {
	app := testutil.SetupApp(t)

	_, account := newOrgWithAccount(t, app, "born-suspended", "Born Suspended", map[string]any{"active": false})

	if accountActive(t, app, account.Id) {
		t.Error("an organization created with active=false was provisioned a live NATS account")
	}
}

// The mirror is LEVEL-triggered, and this is the test that would fail if
// somebody rewrote it as an edge.
//
// Two properties ride on it. The error this hook returns is only actionable if
// re-saving retries, and on an edge trigger the retry is dead by construction:
// the second save sees was == now and returns early, so the one lever the
// operator has does nothing. And an account flipped directly in the PocketBase
// dashboard -- which is a superuser path with no rule and no hook over it --
// would otherwise stay out of step with the organization forever.
func TestTheMirrorCorrectsDriftRatherThanOnlyWatchingEdges(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "drifted", "Drifted", nil)

	// Somebody clears the account by hand. The organization still says active.
	account.Set("active", false)
	if err := app.Save(account); err != nil {
		t.Fatalf("hand-edit the account: %v", err)
	}

	// An ordinary organization edit that does not touch `active` at all.
	org.Set("description", "unrelated edit")
	if err := app.Save(org); err != nil {
		t.Fatalf("save the organization: %v", err)
	}

	if !accountActive(t, app, account.Id) {
		t.Error("an organization save did not pull its NATS account back into line; " +
			"the mirror is watching the edge instead of the value")
	}
}

// The platform's own organizations are refused, and an ordinary one is not.
//
// Paired deliberately: a refusal test on its own passes just as well against a
// hook that refuses everything, which would take the whole feature out while
// looking green. The operator organization's account is the hub every managed
// tenant's helpdesk.> import resolves against, so withdrawing it is a
// platform-wide outage triggered by an unlabelled checkbox -- and the system
// organization has no provisioned account to mirror in the first place.
func TestInfrastructureOrgsCannotBeDeactivated(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, flag := range []string{"is_operator_org", "is_system_org"} {
		t.Run(flag, func(t *testing.T) {
			org := newFlagOrg(t, app, flag, "Infra "+flag, map[string]any{flag: true})

			org.Set("active", false)
			err := app.Save(org)
			if err == nil {
				t.Fatalf("clearing active on an %s organization was allowed", flag)
			}
			if !errors.Is(err, hooks.ErrInfrastructureOrgActive) {
				t.Errorf("the refusal should be ErrInfrastructureOrgActive, got: %v", err)
			}

			// The flag really did not move. A hook that returns an error after
			// the value is already persisted would fail here.
			reloaded, findErr := app.FindRecordById("organizations", org.Id)
			if findErr != nil {
				t.Fatalf("reload the organization: %v", findErr)
			}
			if !reloaded.GetBool("active") {
				t.Error("the save was refused but the flag was written anyway")
			}
		})
	}

	// The pairing. An ordinary organization is still deactivatable, so the
	// refusal above is about these two rows and not about the operation.
	t.Run("an ordinary org still can be", func(t *testing.T) {
		org, account := newOrgWithAccount(t, app, "ordinary", "Ordinary Tenant", nil)
		org.Set("active", false)
		if err := app.Save(org); err != nil {
			t.Fatalf("an ordinary organization should be deactivatable: %v", err)
		}
		if accountActive(t, app, account.Id) {
			t.Error("the ordinary organization was deactivated but its account stayed active")
		}
	})
}
