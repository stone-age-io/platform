package hooks_test

import (
	"fmt"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	nebulacert "github.com/slackhq/nebula/cert"
	"gopkg.in/yaml.v3"

	"platform/internal/testutil"
)

// meshDevice is one machine on the overlay, as Nebula sees it after reading a
// config: the certificate it presents and the pool it verifies peers against.
// It is a SNAPSHOT of the config the device fetched at one moment. Rotation is
// about devices holding snapshots of different ages, so a test that reads
// every config fresh from the database cannot see the property at all.
type meshDevice struct {
	name  string
	cert  nebulacert.Certificate
	pool  *nebulacert.CAPool
	caPEM string
}

// startNebula replays what nebula/pki.go does with a config at startup, using
// Nebula's own cert package: build the CA pool from pki.ca, apply
// pki.blocklist, parse pki.cert and pki.key, and refuse a pair that does not
// match. A config that fails here is one Nebula would not start on.
func startNebula(t *testing.T, name, configYAML string) meshDevice {
	t.Helper()
	if configYAML == "" {
		t.Fatalf("%s: no config_yaml to start on", name)
	}
	var cfg struct {
		PKI struct {
			CA        string   `yaml:"ca"`
			Cert      string   `yaml:"cert"`
			Key       string   `yaml:"key"`
			Blocklist []string `yaml:"blocklist"`
		} `yaml:"pki"`
	}
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		t.Fatalf("%s: config_yaml is not valid YAML: %v", name, err)
	}

	pool, err := nebulacert.NewCAPoolFromPEM([]byte(cfg.PKI.CA))
	if err != nil {
		t.Fatalf("%s: Nebula refuses pki.ca: %v", name, err)
	}
	for _, fp := range cfg.PKI.Blocklist {
		pool.BlocklistFingerprint(fp)
	}

	c, _, err := nebulacert.UnmarshalCertificateFromPEM([]byte(cfg.PKI.Cert))
	if err != nil {
		t.Fatalf("%s: Nebula cannot parse pki.cert: %v", name, err)
	}
	key, _, curve, err := nebulacert.UnmarshalPrivateKeyFromPEM([]byte(cfg.PKI.Key))
	if err != nil {
		t.Fatalf("%s: Nebula cannot parse pki.key: %v", name, err)
	}
	if curve != c.Curve() {
		t.Fatalf("%s: pki.key is curve %v but pki.cert is %v", name, curve, c.Curve())
	}
	if err := c.VerifyPrivateKey(curve, key); err != nil {
		t.Fatalf("%s: Nebula would refuse to start: %v", name, err)
	}
	// Nebula also checks its own certificate against its own pool at startup.
	if _, err := pool.VerifyCertificate(time.Now(), c); err != nil {
		t.Fatalf("%s: its own certificate does not verify against its own pki.ca: %v", name, err)
	}

	return meshDevice{name: name, cert: c, pool: pool, caPEM: cfg.PKI.CA}
}

// handshake is Nebula's mutual verification: each side checks the certificate
// the other presents against its OWN pool, with no chain and no fallback. One
// direction failing is a failed tunnel.
func handshake(a, b meshDevice) error {
	now := time.Now()
	if _, err := a.pool.VerifyCertificate(now, b.cert); err != nil {
		return fmt.Errorf("%s refuses %s: %w", a.name, b.name, err)
	}
	if _, err := b.pool.VerifyCertificate(now, a.cert); err != nil {
		return fmt.Errorf("%s refuses %s: %w", b.name, a.name, err)
	}
	return nil
}

// wantMesh asserts every pair of devices can open a tunnel.
func wantMesh(t *testing.T, stage string, devices ...meshDevice) {
	t.Helper()
	for i := range devices {
		for j := i + 1; j < len(devices); j++ {
			if err := handshake(devices[i], devices[j]); err != nil {
				t.Errorf("%s: the mesh is split: %v", stage, err)
			}
		}
	}
}

// wantShutOut asserts no member accepts the outsider's certificate.
func wantShutOut(t *testing.T, stage string, outsider meshDevice, members ...meshDevice) {
	t.Helper()
	for _, m := range members {
		if _, err := m.pool.VerifyCertificate(time.Now(), outsider.cert); err == nil {
			t.Errorf("%s: %s accepts %s", stage, m.name, outsider.name)
		}
	}
}

// TestCARotationKeepsALiveMeshTogether walks one organization's overlay through
// a whole CA rotation and, at every step, opens the tunnels Nebula would.
//
// The neighbouring tests check the state machine on the CA record, and
// pb-nebula checks that a two-certificate bundle verifies both. Neither puts
// them together: real records, the config each device actually receives, and a
// fleet in which some devices have fetched and others have not. That mix is
// the only thing rotation is hard about.
//
// What it asserts:
//
//   - after `prepare` and after `commit`, a device that has fetched and one
//     that has not still reach each other, in both directions;
//   - a device that skipped a wait does NOT, which is what the waits are for
//     (and what keeps the positives from passing against a pool that trusts
//     everything);
//   - a deactivated host stays refused at every step and is never re-signed;
//   - another organization's host is refused at every step.
//
// Each device reads its config the way the Agent does: signed in as its Thing,
// through the record API and the row rules, not from the database.
//
// Not covered: the rotate route's own authorization (scripts/test-authz.sh
// section 13b), and real packets. A handshake here is the certificate check
// Nebula makes, not a tunnel between two processes.
func TestCARotationKeepsALiveMeshTogether(t *testing.T) {
	app := testutil.SetupApp(t)
	api := newRecordAPI(t, app)

	newRec := func(collection string, org *core.Record, fields map[string]any) *core.Record {
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
			t.Fatalf("create %s %v: %v", collection, fields["name"], err)
		}
		return rec
	}

	// The same range in both organizations, so nothing but the CA tells a
	// host in one from a host in the other.
	const cidr = "10.60.0.0/24"

	type site struct {
		org     *core.Record
		ca      *core.Record
		network *core.Record
	}
	newSite := func(slug, name string) site {
		t.Helper()
		org := newFlagOrg(t, app, slug, name, nil)
		ca, err := app.FindFirstRecordByFilter("nebula_ca", "organization = {:o}", dbx.Params{"o": org.Id})
		if err != nil {
			t.Fatalf("%s: provisioning should have created a Nebula CA: %v", name, err)
		}
		network := newRec("nebula_networks", org, map[string]any{
			"name": slug + "-net", "cidr_range": cidr, "active": true, "ca_id": ca.Id,
		})
		return site{org: org, ca: ca, network: network}
	}

	// A device is a Thing with a Nebula host attached, as POST /api/org/things
	// builds one.
	type device struct {
		thing *core.Record
		host  *core.Record
	}
	newDevice := func(s site, code, ip string, hostFields map[string]any) device {
		t.Helper()
		fields := map[string]any{
			"hostname":        code,
			"email":           code + "@" + s.org.Id + ".nebula.test",
			"emailVisibility": true,
			"password":        "probe-password-123",
			"overlay_ip":      ip,
			"network_id":      s.network.Id,
			"validity_years":  1,
			"active":          true,
		}
		for k, v := range hostFields {
			fields[k] = v
		}
		host := newRec("nebula_hosts", s.org, fields)
		thing := newRec("things", s.org, map[string]any{
			"name":            code,
			"code":            code,
			"email":           code + "@" + s.org.Id + ".thing.test",
			"emailVisibility": true,
			"password":        "probe-password-123",
			"nebula_host":     host.Id,
		})
		return device{thing: thing, host: host}
	}

	// fetch is a device pulling its config: signed in as the Thing, reading
	// its own record with the Nebula host expanded.
	fetch := func(d device, label string) meshDevice {
		t.Helper()
		thing := reloadRecord(t, app, "things", d.thing.Id)
		code, resp := api.json(t, "GET",
			"/api/collections/things/records/"+thing.Id+"?expand=nebula_host",
			authToken(t, thing), nil)
		if code != 200 {
			t.Fatalf("%s: a Thing could not read its own record: %d %s", label, code, resp)
		}
		expand, _ := resp["expand"].(map[string]any)
		host, _ := expand["nebula_host"].(map[string]any)
		cfg, _ := host["config_yaml"].(string)
		if cfg == "" {
			t.Fatalf("%s: the Thing read its record but got no Nebula config with it", label)
		}
		return startNebula(t, label, cfg)
	}

	acme := newSite("acme", "Acme Buildings")
	lighthouse := newDevice(acme, "lh-1", "10.60.0.1", map[string]any{
		"is_lighthouse": true, "public_host_port": "203.0.113.10:4242",
	})
	gwA := newDevice(acme, "gw-a", "10.60.0.11", nil)
	gwB := newDevice(acme, "gw-b", "10.60.0.12", nil)
	retired := newDevice(acme, "gw-old", "10.60.0.13", nil)

	other := newSite("other", "Other Tenant")
	strangerDevice := newDevice(other, "gw-x", "10.60.0.11", nil)
	stranger := fetch(strangerDevice, "another organization's host")

	// Take one device out of service before the rotation starts. The
	// certificate it walks away with is the one every peer must keep refusing.
	retiredCert := fetch(retired, "retired host")
	retiredThing := reloadRecord(t, app, "things", retired.thing.Id)
	retiredThing.Set("active", false)
	if err := app.Save(retiredThing); err != nil {
		t.Fatalf("deactivate the retired Thing: %v", err)
	}
	if reloadRecord(t, app, "nebula_hosts", retired.host.Id).GetBool("active") {
		t.Fatal("deactivating the Thing did not deactivate its Nebula host")
	}
	retiredIssued := reloadRecord(t, app, "nebula_hosts", retired.host.Id).GetString("certificate")

	rotateCA := func(step string) {
		t.Helper()
		if err := rotate(app, reloadRecord(t, app, "nebula_ca", acme.ca.Id), step); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}

	// ------------------------------------------------------------ before
	lh0 := fetch(lighthouse, "lighthouse")
	a0 := fetch(gwA, "gw-a")
	b0 := fetch(gwB, "gw-b")

	wantMesh(t, "before rotation", lh0, a0, b0)
	wantShutOut(t, "before rotation", stranger, lh0, a0, b0)
	wantShutOut(t, "before rotation", retiredCert, lh0, a0, b0)

	// The retired host is refused because of the blocklist and for no other
	// reason: the same CA without the blocklist still accepts it. Without
	// this, a certificate that never verified would pass the line above.
	bare, err := nebulacert.NewCAPoolFromPEM([]byte(a0.caPEM))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bare.VerifyCertificate(time.Now(), retiredCert.cert); err != nil {
		t.Fatalf("the retired certificate should verify with no blocklist applied: %v", err)
	}

	// ----------------------------------------------------------- prepare
	// Trust is published; nothing is signed by the incoming CA yet. One
	// gateway fetches straight away and the rest have not.
	rotateCA("prepare")
	a1 := fetch(gwA, "gw-a (fetched after prepare)")

	if n := len(a1.pool.CAs); n != 2 {
		t.Errorf("after prepare a fetched config trusts %d CAs, want the outgoing and the incoming", n)
	}
	if fp(t, a1.cert) != fp(t, a0.cert) {
		t.Error("prepare re-signed a host; issuance must not move until commit")
	}
	wantMesh(t, "after prepare, fleet half fetched", lh0, a1, b0)
	wantShutOut(t, "after prepare", retiredCert, a1)
	wantShutOut(t, "after prepare", stranger, a1)

	// The wait: every device picks up the two-CA bundle. gw-b's snapshot from
	// before the rotation is kept, to stand for a device that missed it.
	lh1 := fetch(lighthouse, "lighthouse (fetched after prepare)")
	b1 := fetch(gwB, "gw-b (fetched after prepare)")

	// ------------------------------------------------------------ commit
	// Issuance moves. gw-a fetches and now presents a certificate from the
	// incoming CA to peers that have not fetched since prepare.
	rotateCA("commit")
	a2 := fetch(gwA, "gw-a (fetched after commit)")

	if a2.cert.Issuer() == a1.cert.Issuer() {
		t.Fatal("commit did not re-sign gw-a under the incoming CA")
	}
	wantMesh(t, "after commit, fleet half fetched", lh1, a2, b1)

	// What the wait after prepare bought. A device that never fetched the
	// bundle cannot verify a re-signed peer, so committing early splits it off.
	if err := handshake(a2, b0); err == nil {
		t.Error("a device that skipped the prepare fetch still reached a re-signed peer; " +
			"this test can no longer tell a working rotation from a pool that trusts everything")
	}

	wantShutOut(t, "after commit", retiredCert, lh1, a2, b1)
	wantShutOut(t, "after commit", stranger, lh1, a2, b1)
	if got := reloadRecord(t, app, "nebula_hosts", retired.host.Id).GetString("certificate"); got != retiredIssued {
		t.Error("commit re-signed a deactivated host; its peers blocklist the old fingerprint, " +
			"so a new certificate would put it back on the mesh")
	}

	// The second wait: everyone picks up their re-signed certificate.
	lh2 := fetch(lighthouse, "lighthouse (fetched after commit)")
	b2 := fetch(gwB, "gw-b (fetched after commit)")
	wantMesh(t, "after commit, fleet fetched", lh2, a2, b2)

	// ------------------------------------------------------------ finish
	// The outgoing CA leaves the bundle. gw-a fetches first again.
	rotateCA("finish")
	a3 := fetch(gwA, "gw-a (fetched after finish)")

	if n := len(a3.pool.CAs); n != 1 {
		t.Errorf("after finish a fetched config trusts %d CAs, want only the new one", n)
	}
	wantMesh(t, "after finish, fleet half fetched", lh2, a3, b2)

	// What the wait after commit bought. A device still presenting its
	// outgoing-CA certificate is refused by a peer that fetched after finish.
	if err := handshake(a3, b1); err == nil {
		t.Error("a device still on the outgoing CA reached a peer that fetched after finish; " +
			"finish did not drop the outgoing CA")
	}

	lh3 := fetch(lighthouse, "lighthouse (fetched after finish)")
	b3 := fetch(gwB, "gw-b (fetched after finish)")
	wantMesh(t, "after rotation", lh3, a3, b3)
	wantShutOut(t, "after rotation", retiredCert, lh3, a3, b3)
	wantShutOut(t, "after rotation", stranger, lh3, a3, b3)

	// The other organization was never part of this and still starts.
	fetch(strangerDevice, "another organization's host, after the rotation")
}

func fp(t *testing.T, c nebulacert.Certificate) string {
	t.Helper()
	s, err := c.Fingerprint()
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	return s
}
