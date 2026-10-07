package hooks_test

import (
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

// A deactivated Thing's identities must not come back on their own.
//
// The mirror in hooks/active_flag.go makes them inactive when the Thing is
// deactivated, and before guardLinkedIdentity nothing stopped an admin bringing
// one back by itself -- Re-enable on the NATS user page, the Active toggle on
// the Nebula host form, or `regenerate`/`revoke`/`renew`, each of which pb-nats
// or pb-nebula answers with a working credential even for an inactive identity.
// The device was back on the bus or the mesh under a "Deactivated" badge.
//
// Every refusal is paired with a save that must still succeed: reactivating the
// Thing (the mirror passes the guard), and a NATS user no Thing links (the guard
// is about Things, not about inactive users). A guard that refused everything
// would pass the refusals alone.
//
// Records are RELOADED before every update, for the reason newFlagOrg gives:
// an in-memory record's Original() is the pre-create state, and the record API
// -- the path being tested -- always loads the row first.
func TestDeactivatedThingIdentitiesStayDown(t *testing.T) {
	app := testutil.SetupApp(t)

	org, account := newOrgWithAccount(t, app, "dormant", "Dormant Devices", nil)

	newRec := func(collection string, fields map[string]any) *core.Record {
		t.Helper()
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		rec := core.NewRecord(col)
		rec.Set("organization", org.Id)
		for k, v := range fields {
			rec.Set(k, v)
		}
		if err := app.Save(rec); err != nil {
			t.Fatalf("create %s: %v", collection, err)
		}
		return rec
	}

	role := newRec("nats_roles", map[string]any{"name": "device", "publish_permissions": []string{">"}})
	natsUser := func(name string) *core.Record {
		return newRec("nats_users", map[string]any{
			"nats_username": name,
			"email":         name + "@dormant.test",
			"password":      "probe-password-123",
			"account_id":    account.Id,
			"role_id":       role.Id,
			"active":        true,
		})
	}
	linked := natsUser("door-1")
	unlinked := natsUser("console-operator")

	// The organization's Nebula CA is provisioned with it; a host needs a
	// network under that CA.
	ca, err := app.FindFirstRecordByFilter("nebula_ca", "organization = {:o}", dbx.Params{"o": org.Id})
	if err != nil {
		t.Fatalf("provisioning should have created a Nebula CA: %v", err)
	}
	network := newRec("nebula_networks", map[string]any{
		"name": "dormant-net", "cidr_range": "10.44.0.0/16", "active": true, "ca_id": ca.Id,
	})
	host := newRec("nebula_hosts", map[string]any{
		"hostname":        "door-1",
		"email":           "door-1@hosts.dormant.test",
		"emailVisibility": true,
		"password":        "probe-password-123",
		"overlay_ip":      "10.44.0.10",
		"network_id":      network.Id,
		"validity_years":  1,
		"active":          true,
	})

	thing := newRec("things", map[string]any{
		"name":            "Door 1",
		"code":            "DOOR-1",
		"email":           "door-1@things.dormant.test",
		"emailVisibility": true,
		"password":        "probe-password-123",
		"nats_user":       linked.Id,
		"nebula_host":     host.Id,
	})

	reload := func(collection, id string) *core.Record {
		t.Helper()
		rec, err := app.FindRecordById(collection, id)
		if err != nil {
			t.Fatalf("reload %s %s: %v", collection, id, err)
		}
		return rec
	}
	setThingActive := func(v bool) {
		t.Helper()
		rec := reload("things", thing.Id)
		rec.Set("active", v)
		if err := app.Save(rec); err != nil {
			t.Fatalf("set thing active=%v: %v", v, err)
		}
	}

	setThingActive(false)
	if reload("nats_users", linked.Id).GetBool("active") || reload("nebula_hosts", host.Id).GetBool("active") {
		t.Fatal("deactivating the Thing should have deactivated both identities (the mirror, which the guard must not block)")
	}

	// tryRevive applies one change to a freshly loaded identity and reports
	// whether the save went through.
	tryRevive := func(collection, id string, fields map[string]any) error {
		rec := reload(collection, id)
		for k, v := range fields {
			rec.Set(k, v)
		}
		return app.Save(rec)
	}

	// Each case starts from a freshly deactivated Thing. Shared state would let
	// one case that got through leave the identity active for the next, and the
	// next would then prove nothing about an INACTIVE identity.
	freshlyDeactivated := func() {
		t.Helper()
		setThingActive(true)
		setThingActive(false)
	}

	for _, c := range []struct {
		name   string
		fields map[string]any
	}{
		{"re-enable", map[string]any{"active": true}},
		{"regenerate", map[string]any{"regenerate": true}},
		{"revoke", map[string]any{"revoke": true}},
	} {
		t.Run("nats "+c.name+" is refused", func(t *testing.T) {
			freshlyDeactivated()
			// A user JWT re-minted for the same key in the same wall-clock
			// second is byte-identical (CLAUDE.md, the re-minted credential
			// bullet), so without this a regenerate that went through would
			// still compare equal. The baseline is read right before the call.
			time.Sleep(1100 * time.Millisecond)
			before := reload("nats_users", linked.Id)
			if err := tryRevive("nats_users", linked.Id, c.fields); err == nil {
				t.Errorf("%s on a deactivated Thing's NATS identity was allowed", c.name)
			}
			after := reload("nats_users", linked.Id)
			if after.GetBool("active") {
				t.Errorf("%s left the identity active", c.name)
			}
			if after.GetString("jwt") != before.GetString("jwt") || after.GetString("public_key") != before.GetString("public_key") {
				t.Errorf("%s minted a new credential for a deactivated Thing", c.name)
			}
		})
	}

	for _, c := range []struct {
		name   string
		fields map[string]any
	}{
		{"re-enable", map[string]any{"active": true}},
		{"renew", map[string]any{"renew": true}},
	} {
		t.Run("nebula "+c.name+" is refused", func(t *testing.T) {
			freshlyDeactivated()
			// No clock wait: a re-issue generates a new key pair, so the
			// certificate differs whatever the time.
			hostBefore := reload("nebula_hosts", host.Id)
			if err := tryRevive("nebula_hosts", host.Id, c.fields); err == nil {
				t.Errorf("%s on a deactivated Thing's Nebula host was allowed", c.name)
			}
			after := reload("nebula_hosts", host.Id)
			if after.GetBool("active") {
				t.Errorf("%s left the host active", c.name)
			}
			if after.GetString("certificate") != hostBefore.GetString("certificate") {
				t.Errorf("%s re-issued the certificate of a deactivated Thing's host", c.name)
			}
		})
	}

	t.Run("an inactive NATS user no Thing links can still be re-enabled", func(t *testing.T) {
		rec := reload("nats_users", unlinked.Id)
		rec.Set("active", false)
		if err := app.Save(rec); err != nil {
			t.Fatalf("deactivate the unlinked user: %v", err)
		}
		if err := tryRevive("nats_users", unlinked.Id, map[string]any{"active": true}); err != nil {
			t.Fatalf("re-enabling a user no Thing links was refused: %v", err)
		}
		if !reload("nats_users", unlinked.Id).GetBool("active") {
			t.Error("the unlinked user did not come back")
		}
	})

	// Last, because it changes the state the refusals above depend on.
	t.Run("reactivating the Thing brings both identities back", func(t *testing.T) {
		setThingActive(true)
		if !reload("nats_users", linked.Id).GetBool("active") {
			t.Error("the guard blocked the mirror: the NATS identity stayed inactive after the Thing was reactivated")
		}
		if !reload("nebula_hosts", host.Id).GetBool("active") {
			t.Error("the guard blocked the mirror: the Nebula host stayed inactive after the Thing was reactivated")
		}
	})

}
