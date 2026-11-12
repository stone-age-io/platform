package hooks_test

import (
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	pbnebula "github.com/skeeeon/pb-nebula"

	"platform/internal/testutil"
)

// These are DEPENDENCY CONTRACT tests: they assert pb-nebula behaviour this
// platform relies on, so a downgrade or upstream regression fails here rather
// than in production.

// nebulaOrg stands up an app, creates an organization, and returns the Nebula
// CA that hooks.RegisterOrgProvisioning gave it, the same path production uses.
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
		t.Fatalf("organization did not get a Nebula CA: %v", err)
	}
	return app, ca
}

// rotate applies one step the way hooks/nebula_routes.go does: app.Save with no
// request hook in front, the path pb-nebula before v0.3.2 did not validate.
func rotate(app core.App, ca *core.Record, step string) error {
	ca.Set("rotate", step)
	return app.Save(ca)
}

// TestProgrammaticRotationIsValidated pins the contract POST
// /api/org/nebula-ca/rotate is built on.
//
// The route only allowlists the three verbs and trusts pb-nebula for ordering
// and the interlock, which needs Nebula certificates parsed. Without it, Finish
// on an idle CA reaches the executor, which runs post-commit and cannot refuse.
func TestProgrammaticRotationIsValidated(t *testing.T) {
	app, ca := nebulaOrg(t)

	// Out of order: nothing is prepared, so neither commit nor finish applies.
	if err := rotate(app, ca, "commit"); err == nil {
		t.Error("commit with nothing prepared was accepted through app.Save; " +
			"the executor runs post-commit and cannot refuse it")
	} else if !errors.Is(err, pbnebula.ErrInvalidRotation) {
		// The model-layer hook returns the raw error so the sentinel survives;
		// an ApiError wrapper would flatten it away.
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

	// Hand-editing the CA material must be refused on this path too: a
	// malformed PEM in the bundle stops every host under the CA from starting.
	ca = reload(t, app, ca)
	ca.Set("certificate", "not a certificate")
	if err := app.Save(ca); err == nil {
		t.Error("the CA certificate could be overwritten directly through app.Save")
	}
}

// TestRotationCycleCompletes walks the three steps in order, asserting the
// property each one exists for. Prepare publishes trust WITHOUT moving
// issuance: config distribution is pull-based, so one write carrying both
// would split the mesh while it propagates.
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

	// The org has no hosts, so the interlock permits it.
	if err := rotate(app, ca, "finish"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	ca = reload(t, app, ca)
	if ca.GetString("previous_certificate") != "" {
		t.Error("finish did not drop the outgoing CA")
	}
}

// TestMintedHostCertificateIsNotStale proves the pb-nebula build in go.mod
// signs at the network mask. A /32 certificate (pb-nebula before v0.3.0)
// verifies and handshakes but moves no packet, since Nebula installs the
// certificate's network as the host's route, and nothing else would notice.
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

	// The paired positive, so a predicate that always returned false fails.
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
