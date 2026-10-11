package migrations_test

import (
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// The backfill fixes the credentials, not just the rows. A role written before
// hooks/nats_limits.go holds 0, and every user minted from it carries a JWT that
// nats-server enforces as zero. Correcting the row alone leaves those JWTs as they
// are, so this asserts on the re-signed JWT -- which a plain UPDATE would fail --
// and on the roles that must not move: a positive limit is kept, and a role with
// no 0 at all is not saved.
func TestNatsRoleLimitsBackfill(t *testing.T) {
	app := testutil.SetupApp(t)

	newRecord := func(collection string, fields map[string]any) *core.Record {
		t.Helper()
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		rec := core.NewRecord(col)
		for k, v := range fields {
			rec.Set(k, v)
		}
		if err := app.Save(rec); err != nil {
			t.Fatalf("create %s: %v", collection, err)
		}
		return rec
	}
	reload := func(collection, id string) *core.Record {
		t.Helper()
		rec, err := app.FindRecordById(collection, id)
		if err != nil {
			t.Fatalf("reload %s %s: %v", collection, id, err)
		}
		return rec
	}

	owner := newRecord("users", map[string]any{"email": "limits-owner@example.test", "password": "owner-password-123"})
	org := newRecord("organizations", map[string]any{"name": "Limits Backfill", "active": true, "owner": owner.Id})
	account, err := app.FindFirstRecordByFilter("nats_accounts", "organization = {:o}", dbx.Params{"o": org.Id})
	if err != nil {
		t.Fatalf("provisioning should have created a NATS account: %v", err)
	}

	role := func(name string, subs, data, payload int) *core.Record {
		rec := newRecord("nats_roles", map[string]any{
			"name": name, "organization": org.Id,
			"publish_permissions": []string{">"}, "subscribe_permissions": []string{">"},
		})
		// Under the hook, which would store -1: this is the row as the record API
		// wrote it before the hook existed.
		if _, err := app.DB().NewQuery("UPDATE {{nats_roles}} SET [[max_subscriptions]] = {:s}, [[max_data]] = {:d}, [[max_payload]] = {:p} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"s": subs, "d": data, "p": payload, "id": rec.Id}).Execute(); err != nil {
			t.Fatal(err)
		}
		return reload("nats_roles", rec.Id)
	}
	broken := role("pre-fix", 0, 0, 0)
	partly := role("partly", 64, 0, 0)
	healthy := role("healthy", 128, -1, 4096)

	natsUser := func(name string, r *core.Record, active bool) *core.Record {
		return newRecord("nats_users", map[string]any{
			"organization": org.Id, "nats_username": name, "email": name + "@limits.test",
			"password": "probe-password-123", "account_id": account.Id, "role_id": r.Id, "active": active,
		})
	}
	claims := func(user *core.Record) *jwt.UserClaims {
		t.Helper()
		uc, err := jwt.DecodeUserClaims(reload("nats_users", user.Id).GetString("jwt"))
		if err != nil {
			t.Fatalf("decode the JWT of %q: %v", user.GetString("nats_username"), err)
		}
		return uc
	}
	device := natsUser("device-1", broken, true)
	if got := claims(device).Limits.Subs; got != 0 {
		t.Fatalf("setup: the pre-fix credential has Subs %d, want 0", got)
	}
	partlyUser := natsUser("partly-1", partly, true)
	healthyUpdated := healthy.GetDateTime("updated")

	// Run it the way `migrate up` and `serve` do, not by calling the body: the
	// runner applies migrations inside a transaction, and pb-nats re-signs a
	// role's users in an after-success hook that the transaction defers to its
	// commit. A re-sign that only worked outside a transaction would pass a direct
	// call and fail on every real upgrade.
	const file = "schema_update_nats_role_limits.go"
	if _, err := app.DB().NewQuery("DELETE FROM {{_migrations}} WHERE [[file]] = {:f}").
		Bind(dbx.Params{"f": file}).Execute(); err != nil {
		t.Fatal(err)
	}
	applied, err := core.NewMigrationsRunner(app, core.AppMigrations).Up()
	if err != nil {
		t.Fatalf("migrations up: %v", err)
	}
	if len(applied) != 1 || applied[0] != file {
		t.Fatalf("the runner applied %v, want only %s", applied, file)
	}

	for _, c := range []struct {
		role *core.Record
		want map[string]int
	}{
		{broken, map[string]int{"max_subscriptions": -1, "max_data": -1, "max_payload": -1}},
		{partly, map[string]int{"max_subscriptions": 64, "max_data": -1, "max_payload": -1}},
		{healthy, map[string]int{"max_subscriptions": 128, "max_data": -1, "max_payload": 4096}},
	} {
		got := reload("nats_roles", c.role.Id)
		for field, v := range c.want {
			if got.GetInt(field) != v {
				t.Errorf("role %q: %s = %d, want %d", got.GetString("name"), field, got.GetInt(field), v)
			}
		}
	}

	if uc := claims(device); uc.Limits.Subs != jwt.NoLimit || uc.Limits.Payload != jwt.NoLimit {
		t.Errorf("the pre-fix credential was not re-signed: subs %d, payload %d, want %d",
			uc.Limits.Subs, uc.Limits.Payload, jwt.NoLimit)
	}
	if uc := claims(partlyUser); uc.Limits.Subs != 64 || uc.Limits.Payload != jwt.NoLimit {
		t.Errorf("partly: subs %d, payload %d; want 64, %d", uc.Limits.Subs, uc.Limits.Payload, jwt.NoLimit)
	}
	if !reload("nats_roles", healthy.Id).GetDateTime("updated").Equal(healthyUpdated) {
		t.Error("a role with no 0 limit was saved, re-signing its users for nothing")
	}
}
