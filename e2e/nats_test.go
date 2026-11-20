//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// A Thing's issued creds publish where its role allows and the server refuses
// them anywhere else. Catches a role rendered into the JWT wrong: an empty
// allow list (which grants nothing) or a limit of 0 (disconnected on connect).
//
// Run twice: with the key the platform generated, and with a key the device
// made and enrolled itself (device_key), whose .creds the device builds from
// the JWT the platform signed for it.
func TestThingPublishesOnlyWhereItsRoleAllows(t *testing.T) {
	t.Run("platform-held key", func(t *testing.T) { thingPublishesOnlyWhereItsRoleAllows(t, false) })
	t.Run("device-held key", func(t *testing.T) { thingPublishesOnlyWhereItsRoleAllows(t, true) })
}

func thingPublishesOnlyWhereItsRoleAllows(t *testing.T, deviceKey bool) {
	tn := newTenant(t)
	_, monitorCreds := tn.natsUser(t, tn.role(t, []string{">"}, []string{">"}))
	natsBody := map[string]any{"mode": "auto", "role_id": tn.role(t, []string{"probe.>"}, []string{"_INBOX.>"})}
	var thingCredsPath string
	if deviceKey {
		natsBody["device_key"] = true
		thing := tn.deviceThing(t, map[string]any{"nats": natsBody, "nebula": map[string]any{"mode": "none"}})
		thingCredsPath = thing.enrollNATS(t)
	} else {
		thingID := tn.thing(t, map[string]any{"nats": natsBody, "nebula": map[string]any{"mode": "none"}})
		thingCredsPath = thingCreds(t, thingID)
	}

	monitor := connect(t, monitorCreds)
	seen, err := monitor.SubscribeSync(">")
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.Flush(); err != nil {
		t.Fatal(err)
	}

	violations := make(chan error, 4)
	thing := connect(t, thingCredsPath, nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		violations <- err
	}))

	if err := thing.Publish("probe.temp", []byte("21.5")); err != nil {
		t.Fatal(err)
	}
	if err := thing.Publish("elsewhere.temp", []byte("21.5")); err != nil {
		t.Fatal(err)
	}
	if err := thing.Flush(); err != nil {
		t.Fatal(err)
	}

	msg, err := seen.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("the allowed publish on probe.temp never arrived: %v", err)
	}
	if msg.Subject != "probe.temp" {
		t.Fatalf("first message seen on %q, want probe.temp", msg.Subject)
	}
	select {
	case err := <-violations:
		if !strings.Contains(err.Error(), "Permissions Violation") || !strings.Contains(err.Error(), "elsewhere.temp") {
			t.Fatalf("want a publish permissions violation on elsewhere.temp, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server never refused the publish on elsewhere.temp")
	}
	if extra, err := seen.NextMsg(time.Second); err == nil {
		t.Fatalf("the refused publish was delivered anyway, on %q", extra.Subject)
	}
	if thing.IsClosed() {
		t.Fatal("the Thing was disconnected; a role limit may have rendered as 0")
	}
}

// Suspending an organization drops its connections and keeps them out, and
// nothing a tenant can do brings the account back. In v0.19.0 and earlier any
// change to a NATS user in the account republished it: a member rotating their
// own creds, or an owner deactivating a device, undid the suspension.
func TestSuspendedOrganizationStaysOffTheBus(t *testing.T) {
	tn := newTenant(t)
	roleID := tn.role(t, []string{"site.>"}, []string{"site.>", "_INBOX.>"})
	_, creds := tn.natsUser(t, roleID)
	memberToken := tn.member(t, roleID)
	thingID := tn.thing(t, map[string]any{
		"nats":   map[string]any{"mode": "auto", "role_id": roleID},
		"nebula": map[string]any{"mode": "none"},
	})

	nc := connect(t, creds)

	suspend := func(active bool) {
		platform.mustRequest(t, 200, "PATCH", "/collections/organizations/records/"+tn.OrgID, platform.SUToken,
			map[string]any{"active": active}, nil)
	}

	suspend(false)
	eventually(t, 15*time.Second, closed(nc))
	eventually(t, 15*time.Second, refused(creds))

	// 3s debounce and a publish: 10s is long enough for a republish to land.
	platform.mustRequest(t, 200, "POST", "/me/nats-creds/rotate", memberToken, nil, nil)
	holds(t, 10*time.Second, refused(creds))

	platform.mustRequest(t, 200, "PATCH", "/collections/things/records/"+thingID, tn.OwnerToken,
		map[string]any{"active": false}, nil)
	holds(t, 10*time.Second, refused(creds))

	// And the suspension is reversible.
	suspend(true)
	connect(t, creds)
}

// Deactivating a Thing drops its live connection and keeps it out.
func TestDeactivatedThingIsDisconnected(t *testing.T) {
	tn := newTenant(t)
	thingID := tn.thing(t, map[string]any{
		"nats":   map[string]any{"mode": "auto", "role_id": tn.role(t, []string{"probe.>"}, []string{"_INBOX.>"})},
		"nebula": map[string]any{"mode": "none"},
	})
	creds := thingCreds(t, thingID)
	nc := connect(t, creds)

	platform.mustRequest(t, 200, "PATCH", "/collections/things/records/"+thingID, tn.OwnerToken,
		map[string]any{"active": false}, nil)

	eventually(t, 15*time.Second, closed(nc))
	eventually(t, 15*time.Second, refused(creds))
}
