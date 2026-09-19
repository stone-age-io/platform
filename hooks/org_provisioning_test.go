package hooks_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// A provisioning failure has to be three things at once: reported to whoever
// created the organization, survivable by the handlers bound after this one,
// and retryable. Each was broken in its own way before -- the failure was a
// log.Printf nobody reads, there was no error to propagate at all, and the only
// trigger was create.
func TestOrgProvisioningSurfacesFailures(t *testing.T) {
	app := testutil.SetupApp(t)
	t.Cleanup(func() { _ = app.ResetBootstrapState() })

	// Stands in for "the NATS side is having a bad day". Bound after everything
	// testutil binds, so it runs last on the chain and aborts the account save
	// the way a validation error or a driver failure would.
	natsDown := true
	app.OnRecordCreate("nats_accounts").BindFunc(func(e *core.RecordEvent) error {
		if natsDown {
			return errors.New("simulated NATS outage")
		}
		return e.Next()
	})

	newRecord := func(collection string) *core.Record {
		t.Helper()
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}

	owner := newRecord("users")
	owner.Set("email", "provisioning-owner@example.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}

	org := newRecord("organizations")
	org.Set("name", "Halfway House")
	org.Set("active", true)
	org.Set("owner", owner.Id)

	err := app.Save(org)

	// 1. Reported. This used to be a silent success.
	if err == nil {
		t.Fatal("creating an organization whose NATS account could not be written reported success")
	}
	if !strings.Contains(err.Error(), "NATS account") {
		t.Errorf("the error should name what failed to provision, got: %v", err)
	}

	// The row is committed regardless -- AfterCreateSuccess runs past the point
	// of no return. The fix is that somebody is told, not that it rolls back.
	if _, findErr := app.FindRecordById("organizations", org.Id); findErr != nil {
		t.Fatalf("the organization should still exist: %v", findErr)
	}

	// 2. The rest of the chain still ran. Returning the error early instead of
	// after e.Next() would skip RegisterOrgMembership, and the owner of a brand
	// new organization would hold no role in it -- every write branch resolves
	// authority through memberships.role, so they could not write in their own
	// tenant. That trade is strictly worse than the bug being fixed.
	membership, mErr := app.FindFirstRecordByFilter("memberships",
		"user = {:u} && organization = {:o}", dbx.Params{"u": owner.Id, "o": org.Id})
	if mErr != nil || membership == nil {
		t.Fatalf("the owner membership should still have been created: %v", mErr)
	}
	if role := membership.GetString("role"); role != "owner" {
		t.Errorf("owner membership role = %q, want owner", role)
	}

	// The two halves are independent: NATS being down must not cost the Nebula
	// CA, which has nothing to do with it.
	if ca, caErr := app.FindFirstRecordByFilter("nebula_ca",
		"organization = {:o}", dbx.Params{"o": org.Id}); caErr != nil || ca == nil {
		t.Errorf("the Nebula CA should have been provisioned anyway: %v", caErr)
	}

	// 3. Retryable. Re-saving the organization is the lever, which is why this
	// is bound to update as well as create.
	natsDown = false
	org.Set("description", "retrying")
	if err := app.Save(org); err != nil {
		t.Fatalf("re-saving the organization should now provision it: %v", err)
	}
	account, aErr := app.FindFirstRecordByFilter("nats_accounts",
		"organization = {:o}", dbx.Params{"o": org.Id})
	if aErr != nil || account == nil {
		t.Fatalf("the retry should have created the NATS account: %v", aErr)
	}

	// 4. Create-if-missing, not create-always. A second save must not mint a
	// second signing identity for the tenant.
	org.Set("description", "saved again")
	if err := app.Save(org); err != nil {
		t.Fatalf("a no-op re-save should succeed: %v", err)
	}
	accounts, listErr := app.FindAllRecords("nats_accounts", dbx.HashExp{"organization": org.Id})
	if listErr != nil {
		t.Fatalf("list accounts: %v", listErr)
	}
	if len(accounts) != 1 {
		t.Errorf("organization has %d NATS accounts, want exactly 1", len(accounts))
	}
}
