package migrations

import (
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_protected_files marks every uploaded file on the platform as
// protected, so serving one requires a short-lived file token and passes the
// owning collection's viewRule.
//
// An unprotected file is served to anyone at its URL; only the random filename
// suffix stands in the way, so a leaked URL is a bearer credential that never
// expires. Every file field is protected, not only the sensitive floorplans, so
// the rule is one a reviewer can check in one pass.
//
// The fields are derived from the live schema, not a list, as in
// hooks/relation_tenancy.go: a field added later is protected on upgrade by this
// migration. TestEveryFileFieldInTheSchemaIsProtected guards fresh installs.
//
// Not a re-import, on purpose: a re-import keeps the LIVE field when a name
// matches and an id does not, and logs ✅ anyway. That is not acceptable for the
// flag deciding whether strangers can fetch a file.
//
// Existing /api/files links stop working on deploy, by design. The console
// builds its URLs at render time, so no data migration is needed.
func init() {
	m.Register(ProtectFileFields, nil)
}

// ProtectFileFields is the migration body. It is exported because a fresh
// database already has `"protected": true`, so `migrate up` proves nothing; the
// test sets the fields back to false and runs this.
func ProtectFileFields(app core.App) error {
	collections, err := app.FindAllCollections()
	if err != nil {
		return fmt.Errorf("protect file fields: %w", err)
	}

	for _, col := range collections {
		// PocketBase's own collections are not ours to re-declare.
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

		// Save only what changed, so a re-run does not churn every collection.
		if !changed {
			continue
		}
		if err := app.Save(col); err != nil {
			return fmt.Errorf("protect file fields on %s: %w", col.Name, err)
		}
	}

	return nil
}
