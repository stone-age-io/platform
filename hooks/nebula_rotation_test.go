package hooks_test

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	pbnebula "github.com/skeeeon/pb-nebula"

	"platform/internal/testutil"
)

// These are DEPENDENCY CONTRACT tests, in the same spirit as the "a minted
// Nebula host is born active" check in scripts/test-authz.sh. Neither asserts
// platform code; both assert a pb-nebula behaviour this platform now relies on,
// so a downgrade or an upstream regression fails here rather than in
// production.

// nebulaOrg stands up an app, creates an organization, and returns its
// auto-provisioned Nebula CA.
//
// The CA arrives via hooks.RegisterOrgProvisioning, which testutil binds -- the
// same path production uses, so the record under test is the one an operator
// would actually be rotating.
func nebulaOrg(t *testing.T) (core.App, *core.Record) {
	t.Helper()
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatal(err)
		}
		return core.NewRecord(col)
	}

	owner := newRecord("users")
	owner.Set("email", "owner@rotation.test")
	owner.Set("password", "owner-password-123")
	if err := app.Save(owner); err != nil {
		t.Fatal(err)
	}

	org := newRecord("organizations")
	org.Set("name", "Rotation Co")
	org.Set("active", true)
	org.Set("owner", owner.Id)
	if err := app.Save(org); err != nil {
		t.Fatal(err)
	}

	ca, err := app.FindFirstRecordByFilter("nebula_ca", "organization = {:org}",
		map[string]any{"org": org.Id})
	if err != nil || ca == nil {
		// The provisioning hook logs and swallows its own failures so a
		// problem cannot undo the organization create, which means an absent
		// record is the only signal there is.
		t.Fatalf("organization did not get a Nebula CA: %v", err)
	}
	return app, ca
}

// rotate applies one step the way hooks/nebula_routes.go does -- app.Save on
// the record, with no request hook in front of it. That path is the whole point:
// pb-nebula's rotation validation used to be bound to the request hooks alone,
// so a save from a route skipped every check while execution went ahead anyway.
func rotate(app core.App, ca *core.Record, step string) error {
	ca.Set("rotate", step)
	return app.Save(ca)
}

// TestProgrammaticRotationIsValidated pins the contract POST
// /api/org/nebula-ca/rotate is built on.
//
// The route cannot validate the transition itself: "has every active host
// migrated off the outgoing CA?" needs each host certificate's issuer compared
// against the CA's fingerprint, which means parsing Nebula certificates that
// only pb-nebula holds the code for. So the route allowlists the three verbs
// and trusts the library for ordering and the interlock. If that trust is
// misplaced -- as it was before pb-nebula v0.3.2 -- an owner clicking Finish on
// an idle CA reaches the executor, which runs after the write has committed and
// cannot refuse anything.
func TestProgrammaticRotationIsValidated(t *testing.T) {
	app, ca := nebulaOrg(t)

	// Out of order, both directions. Nothing is prepared, so neither commit nor
	// finish has anything to act on.
	if err := rotate(app, ca, "commit"); err == nil {
		t.Error("commit with nothing prepared was accepted through app.Save; " +
			"the executor runs post-commit and cannot refuse it")
	} else if !errors.Is(err, pbnebula.ErrInvalidRotation) {
		// The model-layer hook returns the raw error precisely so this works.
		// An ApiError wrapper would flatten the sentinel away and leave a Go
		// caller matching on message text.
		t.Errorf("commit refusal did not carry ErrInvalidRotation: %v", err)
	}

	ca = reload(t, app, ca)
	if err := rotate(app, ca, "finish"); err == nil {
		t.Error("finish on an idle CA was accepted; dropping an outgoing CA " +
			"early takes hosts off the mesh silently")
	}

	ca = reload(t, app, ca)
	if err := rotate(app, ca, "not-a-verb"); err == nil {
		t.Error("an unknown verb was accepted rather than rejected")
	}

	// Hand-editing the material pb-nebula owns must be refused on this path too.
	// A non-CA PEM in the bundle makes every host under the CA fail to start,
	// because NewCAPoolFromPEM treats a malformed block as a hard error.
	ca = reload(t, app, ca)
	ca.Set("certificate", "not a certificate")
	if err := app.Save(ca); err == nil {
		t.Error("the CA certificate could be overwritten directly through app.Save")
	}
}

// TestRotationCycleCompletes walks the three steps in order, asserting the
// property each one exists to have.
//
// Prepare publishing trust WITHOUT moving issuance is the reason rotation is
// three steps rather than one: config distribution is pull-based, so a single
// write carrying both the new bundle and the new certificate splits the mesh
// for as long as propagation takes.
func TestRotationCycleCompletes(t *testing.T) {
	app, ca := nebulaOrg(t)
	issuedBy := ca.GetString("certificate")
	if issuedBy == "" {
		t.Fatal("provisioned CA has no certificate")
	}

	if err := rotate(app, ca, "prepare"); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	ca = reload(t, app, ca)
	if ca.GetString("next_certificate") == "" {
		t.Error("prepare did not mint an incoming CA")
	}
	if ca.GetString("certificate") != issuedBy {
		t.Error("prepare moved issuance; it must publish trust only, or a host " +
			"that has not fetched yet cannot verify one that has")
	}

	if err := rotate(app, ca, "commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	ca = reload(t, app, ca)
	if ca.GetString("certificate") == issuedBy {
		t.Error("commit did not swap the incoming CA in")
	}
	if ca.GetString("previous_certificate") == "" {
		t.Error("commit did not retain the outgoing CA; both must stay trusted " +
			"until every host has migrated")
	}

	// No active host holds an outgoing-CA certificate here, so the interlock
	// permits it. The org has no hosts at all.
	if err := rotate(app, ca, "finish"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	ca = reload(t, app, ca)
	if ca.GetString("previous_certificate") != "" {
		t.Error("finish did not drop the outgoing CA")
	}
}

// TestMintedHostCertificateIsNotStale proves the /32 fix is live in THIS
// deployment.
//
// pb-nebula signed host certificates at /32 until v0.3.0. Nebula reads a
// certificate's network straight onto the tun device and installs a link route
// for it, so a /32 gives a host a route covering only itself: the certificate
// verifies, the config renders, the handshake completes, and no packet ever
// crosses the mesh. Nothing errors, which is exactly why it survived from the
// first commit.
//
// The console badges affected hosts using this same predicate through GET
// /api/org/nebula/cert-audit. A freshly minted host must come back clean, or
// the console would light up every row it just created.
func TestMintedHostCertificateIsNotStale(t *testing.T) {
	app, ca := nebulaOrg(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatal(err)
		}
		return core.NewRecord(col)
	}

	const cidr = "10.42.0.0/16"

	network := newRecord("nebula_networks")
	network.Set("name", "audit-net")
	network.Set("cidr_range", cidr)
	network.Set("ca_id", ca.Id)
	network.Set("organization", ca.GetString("organization"))
	network.Set("active", true)
	if err := app.Save(network); err != nil {
		t.Fatal(err)
	}

	host := newRecord("nebula_hosts")
	host.Set("email", "audit-host@hosts.test")
	host.Set("emailVisibility", true)
	host.Set("password", "Password123!")
	host.Set("hostname", "audit-host")
	host.Set("overlay_ip", "10.42.0.12")
	host.Set("network_id", network.Id)
	host.Set("organization", ca.GetString("organization"))
	if err := app.Save(host); err != nil {
		t.Fatal(err)
	}

	host = reload(t, app, host)
	certPEM := host.GetString("certificate")
	if certPEM == "" {
		t.Fatal("minted host has no certificate")
	}

	stale, err := pbnebula.HostCertNetworkIsStale(certPEM, host.GetString("overlay_ip"), cidr)
	if err != nil {
		t.Fatalf("audit predicate failed on a freshly minted certificate: %v", err)
	}
	if stale {
		t.Errorf("a host minted right now carries a certificate that does not match %s -- "+
			"pb-nebula is signing at the wrong mask, and no packet will cross the mesh", cidr)
	}

	// The paired positive. Without it a predicate that always returned false
	// would pass the check above, and the console's badge would be dark for
	// every genuinely stale host rather than for none.
	stale, err = pbnebula.HostCertNetworkIsStale(certPEM, host.GetString("overlay_ip"), "10.42.0.0/24")
	if err != nil {
		t.Fatalf("audit predicate failed against a changed mask: %v", err)
	}
	if !stale {
		t.Error("the audit predicate did not notice a certificate signed at a different mask")
	}
}

func reload(t *testing.T, app core.App, record *core.Record) *core.Record {
	t.Helper()
	fresh, err := app.FindRecordById(record.Collection().Name, record.Id)
	if err != nil {
		t.Fatalf("reload %s: %v", record.Collection().Name, err)
	}
	return fresh
}
