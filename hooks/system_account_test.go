package hooks_test

import (
	"strings"
	"testing"

	"platform/hooks"
	"platform/internal/testutil"
)

// Every org's NATS account is named after the org, so an org called "System
// Account" gets an account with the $SYS account's name. leaf-config serves the
// $SYS JWT and the readiness check reads the $SYS user's creds, so both must
// find the real one.
func TestSystemAccountLookupIgnoresAnOrgNamedLikeIt(t *testing.T) {
	app := testutil.SetupApp(t)

	_, impostor := newOrgWithAccount(t, app, "impostor", "System Account", nil)
	if impostor.GetString("name") != "System Account" {
		t.Fatalf("expected the org's account to be named after it, got %q", impostor.GetString("name"))
	}

	op, err := app.FindFirstRecordByFilter("nats_system_operator", "1=1")
	if err != nil {
		t.Fatalf("no operator: %v", err)
	}
	wantID := op.GetString("system_account_id")
	if wantID == "" {
		t.Fatal("pb-nats should have stored system_account_id on the operator")
	}

	t.Run("by the operator's system_account_id", func(t *testing.T) {
		got, err := hooks.FindSystemAccount(app, "nats_accounts")
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if got.Id != wantID {
			t.Fatalf("found account %s (%q), want the $SYS account %s", got.Id, got.GetString("organization"), wantID)
		}
	})

	// A database from before pb-nats stored the id: the name is all there is,
	// and two matches must be refused rather than guessed between.
	t.Run("the name fallback refuses an ambiguous match", func(t *testing.T) {
		if _, err := app.DB().NewQuery("UPDATE nats_system_operator SET system_account_id = ''").Execute(); err != nil {
			t.Fatalf("clear system_account_id: %v", err)
		}
		_, err := hooks.FindSystemAccount(app, "nats_accounts")
		if err == nil || !strings.Contains(err.Error(), "2 accounts") {
			t.Fatalf("want an ambiguity error, got %v", err)
		}
	})
}
