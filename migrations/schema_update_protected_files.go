package migrations

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_protected_files marks every uploaded file on the platform as
// protected, so serving one requires a short-lived file token and passes the
// owning collection's viewRule.
//
// Today that is users.avatar, organizations.logo and locations.floorplan.
//
// WHAT AN UNPROTECTED FIELD ACTUALLY MEANS. PocketBase serves
// /api/files/{collection}/{record}/{filename} to ANYONE, with no auth at all;
// the only thing standing in the way is that the stored filename carries a
// random suffix. So the URL is a bearer credential that never expires and
// cannot be revoked: once it escapes -- a Referer header, a screenshot, a
// support ticket, a proxy log, a browser profile on a shared machine -- it
// keeps serving that file for the life of the record. With `protected`,
// apis/file.go resolves the `token` query param to an auth record and runs
// CanAccessRecord against the collection's ViewRule, which is the boundary the
// rest of the API already enforces.
//
// WHY ALL OF THEM AND NOT JUST THE OBVIOUS ONE. Ranked by what a leak costs:
// locations.floorplan is a customer site's physical layout and is far and away
// the one that matters; users.avatar is a face; organizations.logo is close to
// public branding. Protecting only the sensitive one leaves a rule nobody can
// state, and the next upload field needs the whole argument had again. "Every
// uploaded file in this platform is authorized" is a rule a reviewer can check
// in one pass.
//
// DERIVED FROM THE LIVE SCHEMA, NOT A LIST. The same call
// hooks/relation_tenancy.go makes, for the same reason: a hand-kept list of
// (collection, field) pairs is a second copy of the schema, and the copy that
// drifts is the one nobody looks at. A hardcoded list would also split the two
// halves of adding an upload field -- set "protected": true in schema.json and
// fresh installs are fine, forget the migration and every EXISTING deployment
// keeps serving it in the clear, with no symptom on either side. Walking the
// collections means a field added next year is protected on upgrade by a
// migration written today. TestEveryFileFieldInTheSchemaIsProtected is the
// matching guard for fresh installs.
//
// NOT A RE-IMPORT, deliberately, although schema.json carries these same field
// ids and a re-import WOULD apply (see schema_update_floorplan_mime_types.go,
// which verified exactly that against a real install). The re-import path
// silently keeps the LIVE field definition whenever a name matches and an id
// does not, and its failure mode is a green ✅ over a change that did not
// happen. That is survivable for a mime-type list. It is not survivable for the
// flag deciding whether a file is served to unauthenticated strangers, so this
// states its intent directly and cannot half-apply.
//
// EXISTING URLS STOP WORKING, which is the point. Any /api/files link already
// pasted somewhere outside the console dies on deploy. Nothing in the console
// stores one -- every call site builds its URL at render time from the record
// -- so there is no data migration to do, only this flag.
func init() {
	m.Register(ProtectFileFields, nil)
}

// ProtectFileFields is the migration body.
//
// Exported, and named rather than inlined into m.Register, for the reason
// DropContractLayer is: the migration set alone only ever exercises the no-op
// path. A fresh database imports a schema.json that already carries
// `"protected": true`, so by the time this runs there is nothing to change and
// a green `migrate up` would prove nothing about the change itself. The test
// puts the fields back to false first and then runs this.
func ProtectFileFields(app core.App) error {
	collections, err := app.FindAllCollections()
	if err != nil {
		return fmt.Errorf("protect file fields: %w", err)
	}

	for _, col := range collections {
		// PocketBase's own collections are not ours to re-declare, and none of
		// them carries a file field we own.
		if col.System {
			continue
		}

		changed := false
		for _, field := range col.Fields {
			fileField, ok := field.(*core.FileField)
			if !ok || fileField.Protected {
				continue
			}
			fileField.Protected = true
			changed = true
			log.Printf("✅ Protected %s.%s", col.Name, fileField.Name)
		}

		// Save only a collection that actually moved: a re-run must not churn
		// every collection in the database to do nothing.
		if !changed {
			continue
		}
		if err := app.Save(col); err != nil {
			return fmt.Errorf("protect file fields on %s: %w", col.Name, err)
		}
	}

	return nil
}
