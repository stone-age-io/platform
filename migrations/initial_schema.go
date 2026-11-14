package migrations

import (
	"errors"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// SchemaJSON holds the embedded schema.json bytes, set from main.go before app.Start().
var SchemaJSON []byte

// errNoSchema stops a migration that finds SchemaJSON empty. Skipping instead
// would record the migration as applied having done nothing, and it would
// never run again.
var errNoSchema = errors.New("migrations.SchemaJSON is empty: set it from the embedded schema.json before the migrations run")

func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			return errNoSchema
		}

		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}

		log.Println("✅ Initial schema imported from embedded schema.json")
		return nil
	}, nil)
}
