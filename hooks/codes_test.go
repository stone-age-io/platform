package hooks_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
)

var bareGenerated = regexp.MustCompile(`^[3-9A-HJ-NP-Y]{3}-[3-9A-HJ-NP-Y]{3}$`)

// TestCodes covers ADR 0003 against a real PocketBase: generated codes, the
// type prefix, the separate prefix sets, and case-insensitive uniqueness. The
// type freeze is an API rule, which app.Save bypasses, so it lives in
// scripts/test-authz.sh instead.
func TestCodes(t *testing.T) {
	app := testutil.SetupApp(t)

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatalf("%s collection: %v", collection, err)
		}
		return core.NewRecord(col)
	}
	mustSave := func(t *testing.T, what string, rec *core.Record) *core.Record {
		t.Helper()
		if err := app.Save(rec); err != nil {
			t.Fatalf("create %s: %v", what, err)
		}
		return rec
	}

	owner := newRecord("users")
	owner.Set("email", "codes-owner@example.test")
	owner.Set("password", "owner-password-123")
	mustSave(t, "owner", owner)

	newOrg := func(name string) *core.Record {
		rec := newRecord("organizations")
		rec.Set("name", name)
		rec.Set("active", true)
		rec.Set("owner", owner.Id)
		return mustSave(t, "org "+name, rec)
	}
	org := newOrg("Codes Alpha")
	other := newOrg("Codes Beta")

	newType := func(collection, name, code, prefix string, orgRec *core.Record) *core.Record {
		rec := newRecord(collection)
		rec.Set("name", name)
		rec.Set("code", code)
		rec.Set("prefix", prefix)
		rec.Set("organization", orgRec.Id)
		return rec
	}
	newLocation := func(name, code, typeID string, orgRec *core.Record) *core.Record {
		rec := newRecord("locations")
		rec.Set("name", name)
		rec.Set("code", code)
		rec.Set("type", typeID)
		rec.Set("organization", orgRec.Id)
		return rec
	}

	building := mustSave(t, "location type", newType("location_types", "Building", "building", "BLD", org))
	camera := mustSave(t, "thing type", newType("thing_types", "Camera", "ip_camera", "CA", org))
	otherBuilding := mustSave(t, "other org's type", newType("location_types", "Building", "building", "BLD", other))

	t.Run("a blank location code is generated under the type prefix", func(t *testing.T) {
		loc := mustSave(t, "location", newLocation("Warehouse", "", building.Id, org))
		code := loc.GetString("code")
		if !strings.HasPrefix(code, "BLD-") || !bareGenerated.MatchString(strings.TrimPrefix(code, "BLD-")) {
			t.Fatalf("generated code = %q, want BLD-XXX-XXX", code)
		}
	})

	t.Run("an untyped record gets a bare code", func(t *testing.T) {
		loc := mustSave(t, "location", newLocation("Yard", "", "", org))
		if code := loc.GetString("code"); !bareGenerated.MatchString(code) {
			t.Fatalf("generated code = %q, want XXX-XXX", code)
		}
	})

	t.Run("a supplied code is kept exactly as typed", func(t *testing.T) {
		loc := mustSave(t, "location", newLocation("Room", "rm-204", building.Id, org))
		if got := loc.GetString("code"); got != "rm-204" {
			t.Fatalf("code = %q, want the installer's rm-204 untouched", got)
		}
	})

	t.Run("a blank thing code is generated under the type prefix", func(t *testing.T) {
		thing := newRecord("things")
		thing.Set("organization", org.Id)
		thing.Set("name", "Dock Camera")
		thing.Set("type", camera.Id)
		thing.Set("email", "dock-camera@codes.thing.local")
		thing.Set("emailVisibility", true)
		thing.Set("password", "thing-password-123")
		mustSave(t, "thing", thing)
		if code := thing.GetString("code"); !strings.HasPrefix(code, "CA-") {
			t.Fatalf("generated code = %q, want CA-XXX-XXX", code)
		}
	})

	t.Run("a type from another organization is refused, not treated as untyped", func(t *testing.T) {
		loc := newLocation("Borrowed", "", otherBuilding.Id, org)
		if err := app.Save(loc); err == nil {
			t.Fatalf("saved a location typed with another organization's type; code %q", loc.GetString("code"))
		}
	})

	t.Run("codes that differ only by case cannot coexist", func(t *testing.T) {
		mustSave(t, "location", newLocation("HQ lower", "hq", "", org))
		if err := app.Save(newLocation("HQ upper", "HQ", "", org)); err == nil {
			t.Fatal("saved HQ beside hq in the same organization")
		}
		// The scoping is per organization, exactly as before.
		mustSave(t, "other org location", newLocation("HQ elsewhere", "HQ", "", other))
	})

	t.Run("a thing prefix cannot repeat a location prefix", func(t *testing.T) {
		if err := app.Save(newType("thing_types", "Beacon", "beacon", "BLD", org)); err == nil {
			t.Fatal("saved thing type prefix BLD while a location type holds it")
		}
		if err := app.Save(newType("location_types", "Cabinet", "cabinet", "CA", org)); err == nil {
			t.Fatal("saved location type prefix CA while a thing type holds it")
		}
		// Paired: another org's location type holds BLD too, and a new prefix is fine.
		mustSave(t, "fresh prefix", newType("thing_types", "Sensor", "sensor", "SN", org))
	})

	t.Run("an update cannot move a prefix into the other set", func(t *testing.T) {
		camera.Set("prefix", "BLD")
		if err := app.Save(camera); err == nil {
			t.Fatal("updated a thing type prefix to one a location type holds")
		}
		camera.Set("prefix", "CAM")
		mustSave(t, "prefix change", camera)
	})

	t.Run("a prefix outside ^[A-Z]{1,4}$ is refused by the field", func(t *testing.T) {
		for _, bad := range []string{"ca", "C4", "CAMER"} {
			if err := app.Save(newType("thing_types", "Bad "+bad, "bad_"+strings.ToLower(bad), bad, org)); err == nil {
				t.Errorf("saved prefix %q", bad)
			}
		}
	})
}
