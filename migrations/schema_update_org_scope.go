package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_org_scope re-imports schema.json for the tightened list/view
// rules on thing_type_operations and message_schemas, which drop the unused
// `organization = ""` platform-shipped clause.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Tightened thing_type_operations + message_schemas rules to current org only")
		return nil
	}, nil)
}
