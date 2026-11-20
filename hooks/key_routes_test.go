package hooks_test

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"testing"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/nats-io/nkeys"
	nebulacert "github.com/slackhq/nebula/cert"
	"golang.org/x/crypto/curve25519"

	"platform/hooks"
	"platform/internal/testutil"
)

// TestEnrollDeviceKeys drives POST /api/me/keys through the real router, with
// pb-nats and pb-nebula enforcing the invariants. The route repeats none of
// the library checks, so these are the end-to-end proof that they hold for the
// route's app.Save() as they do for REST.
//
// Every "cannot" is paired with a "can" on the same identity.
func TestEnrollDeviceKeys(t *testing.T) {
	app := testutil.SetupApp(t)
	org, account := newOrgWithAccount(t, app, "enroll", "Enroll Co", nil)

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
		return reloadRecord(t, app, collection, rec.Id)
	}

	role := newRec("nats_roles", map[string]any{"name": "device", "publish_permissions": []string{">"}})
	natsUser := func(name string, deviceKey bool) *core.Record {
		return newRec("nats_users", map[string]any{
			"nats_username": name, "email": name + "@enroll.test", "password": "probe-password-123",
			"account_id": account.Id, "role_id": role.Id, "active": true, "device_key": deviceKey,
		})
	}
	ca, err := app.FindFirstRecordByFilter("nebula_ca", "organization = {:o}", dbx.Params{"o": org.Id})
	if err != nil {
		t.Fatalf("provisioning should have created a Nebula CA: %v", err)
	}
	network := newRec("nebula_networks", map[string]any{
		"name": "enroll-net", "cidr_range": "10.45.0.0/16", "active": true, "ca_id": ca.Id,
	})
	nebulaHost := func(name, ip string, deviceKey bool) *core.Record {
		return newRec("nebula_hosts", map[string]any{
			"hostname": name, "email": name + "@hosts.enroll.test", "emailVisibility": true,
			"password": "probe-password-123", "overlay_ip": ip, "network_id": network.Id,
			"validity_years": 1, "device_key": deviceKey,
		})
	}
	thing := func(code string, user, host *core.Record) *core.Record {
		fields := map[string]any{
			"name": code, "code": code, "email": code + "@things.enroll.test",
			"emailVisibility": true, "password": "probe-password-123",
		}
		if user != nil {
			fields["nats_user"] = user.Id
		}
		if host != nil {
			fields["nebula_host"] = host.Id
		}
		return newRec("things", fields)
	}

	deviceUser := natsUser("gw-1", true)
	deviceHost := nebulaHost("gw-1", "10.45.0.10", true)
	gateway := thing("GW-1", deviceUser, deviceHost)

	platformUser := natsUser("gw-2", false)
	platformThing := thing("GW-2", platformUser, nil)

	api := keyRouteAPI(t, app)
	enroll := func(t *testing.T, as *core.Record, body map[string]any) (int, apiResponse) {
		t.Helper()
		token := ""
		if as != nil {
			token = authToken(t, as)
		}
		return api.json(t, http.MethodPost, "/api/me/keys", token, body)
	}

	natsKey := newNatsUserKey(t)
	nebulaKey, nebulaRaw := newNebulaHostKey(t)

	t.Run("a Thing enrolls both keys", func(t *testing.T) {
		code, resp := enroll(t, gateway, map[string]any{"nats_public_key": natsKey, "nebula_public_key": nebulaKey})
		if code != http.StatusOK {
			t.Fatalf("enroll: %d %v", code, resp)
		}
		if got := userClaims(t, reloadRecord(t, app, "nats_users", deviceUser.Id)).Subject; got != natsKey {
			t.Errorf("JWT signed for %s, want the enrolled key", got)
		}
		host := reloadRecord(t, app, "nebula_hosts", deviceHost.Id)
		c, _, err := nebulacert.UnmarshalCertificateFromPEM([]byte(host.GetString("certificate")))
		if err != nil {
			t.Fatalf("host has no certificate after enrolling: %v", err)
		}
		if !bytes.Equal(c.PublicKey(), nebulaRaw) {
			t.Error("the host certificate was not signed for the enrolled key")
		}
	})

	t.Run("sending the same key again is a 200", func(t *testing.T) {
		code, resp := enroll(t, gateway, map[string]any{"nats_public_key": natsKey, "nebula_public_key": nebulaKey + "\n"})
		if code != http.StatusOK {
			t.Fatalf("retry with the enrolled keys: %d %v", code, resp)
		}
	})

	t.Run("a different key over an enrolled one is a 409", func(t *testing.T) {
		code, resp := enroll(t, gateway, map[string]any{"nats_public_key": newNatsUserKey(t)})
		if code != http.StatusConflict {
			t.Errorf("NATS: %d %v, want 409", code, resp)
		}
		other, _ := newNebulaHostKey(t)
		code, resp = enroll(t, gateway, map[string]any{"nebula_public_key": other})
		if code != http.StatusConflict {
			t.Errorf("Nebula: %d %v, want 409", code, resp)
		}
	})

	t.Run("a platform-held identity cannot take a key", func(t *testing.T) {
		code, resp := enroll(t, platformThing, map[string]any{"nats_public_key": newNatsUserKey(t)})
		if code != http.StatusBadRequest {
			t.Errorf("%d %v, want 400", code, resp)
		}
	})

	t.Run("a malformed key is a 400", func(t *testing.T) {
		waiting := natsUser("gw-3", true)
		waitingThing := thing("GW-3", waiting, nil)
		code, resp := enroll(t, waitingThing, map[string]any{"nats_public_key": "not-a-key"})
		if code != http.StatusBadRequest {
			t.Errorf("%d %v, want 400", code, resp)
		}
		code, resp = enroll(t, waitingThing, map[string]any{"nats_public_key": newNatsUserKey(t)})
		if code != http.StatusOK {
			t.Errorf("the same Thing with a valid key: %d %v", code, resp)
		}
	})

	t.Run("a Thing with no Nebula host gets a 404, and nothing is written", func(t *testing.T) {
		waiting := natsUser("gw-4", true)
		noHost := thing("GW-4", waiting, nil)
		code, _ := enroll(t, noHost, map[string]any{"nats_public_key": newNatsUserKey(t), "nebula_public_key": nebulaKey})
		if code != http.StatusNotFound {
			t.Errorf("%d, want 404", code)
		}
		if reloadRecord(t, app, "nats_users", waiting.Id).GetString("public_key") != "" {
			t.Error("the NATS key was written although the request was refused")
		}
	})

	t.Run("only a Thing may call it", func(t *testing.T) {
		owner := reloadRecord(t, app, "users", org.GetString("owner"))
		if code, _ := enroll(t, owner, map[string]any{"nats_public_key": newNatsUserKey(t)}); code != http.StatusUnauthorized && code != http.StatusForbidden {
			t.Errorf("a console user: %d, want 401 or 403", code)
		}
		if code, _ := enroll(t, nil, map[string]any{"nats_public_key": newNatsUserKey(t)}); code != http.StatusUnauthorized {
			t.Errorf("no auth: %d, want 401", code)
		}
	})
}

// keyRouteAPI is the record API plus POST /api/me/keys. The harness never
// serves, so the route's OnServe binding is triggered here by hand. No library
// binds OnServe, so this runs only the route's own handler.
func keyRouteAPI(t *testing.T, app *pocketbase.PocketBase) *recordAPI {
	t.Helper()
	hooks.RegisterKeyRoutes(app, hooks.KeyRoutesOptions{
		ThingCollection:      "things",
		NatsUserCollection:   "nats_users",
		NebulaHostCollection: "nebula_hosts",
	})
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}
	se := &core.ServeEvent{App: app, Router: r}
	if err := app.OnServe().Trigger(se, func(e *core.ServeEvent) error { return nil }); err != nil {
		t.Fatalf("bind routes: %v", err)
	}
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("build mux: %v", err)
	}
	return &recordAPI{mux: mux}
}

func newNatsUserKey(t *testing.T) string {
	t.Helper()
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

// newNebulaHostKey makes a key pair as `nebula-cert keygen` does and returns
// the public PEM and the raw public key.
func newNebulaHostKey(t *testing.T) (string, []byte) {
	t.Helper()
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(priv); err != nil {
		t.Fatal(err)
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	return string(nebulacert.MarshalPublicKeyToPEM(nebulacert.Curve_CURVE25519, pub)), pub
}
