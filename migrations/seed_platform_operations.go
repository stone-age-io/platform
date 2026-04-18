package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// seed_platform_operations inserts the platform-shipped shared operations
// (organization = empty), links each to its platform-shipped message schema
// (seeded by seed_platform_message_schemas — runs first alphabetically), and
// creates the fallback `generic_thing` Thing Type linking them.
//
// Idempotent — matches on (organization, name, capability) for operations and
// `code` for the Thing Type.
func init() {
	m.Register(func(app core.App) error {
		opsCol, err := app.FindCollectionByNameOrId("thing_type_operations")
		if err != nil {
			// Stage 2 collections not present yet.
			return nil
		}

		heartbeat, err := upsertOperation(app, opsCol, "heartbeat", "publish", "heartbeat",
			"Generic liveness heartbeat, used by any Thing.",
			"common", "heartbeat", "1.0.0")
		if err != nil {
			return err
		}
		status, err := upsertOperation(app, opsCol, "status", "publish", "status",
			"Online/offline + status message.",
			"common", "status", "1.0.0")
		if err != nil {
			return err
		}

		ttCol, err := app.FindCollectionByNameOrId("thing_types")
		if err != nil {
			return nil
		}

		if err := upsertGenericThing(app, ttCol, heartbeat.Id, status.Id); err != nil {
			return err
		}

		return nil
	}, nil)
}

func upsertOperation(app core.App, col *core.Collection, name, capability, suffix, desc,
	schemaNS, schemaName, schemaVersion string) (*core.Record, error) {
	schemaID := lookupSchemaID(app, schemaNS, schemaName, schemaVersion)

	existing, _ := app.FindFirstRecordByFilter(
		col.Id,
		"organization = '' && name = {:name} && capability = {:cap}",
		map[string]any{"name": name, "cap": capability},
	)
	if existing != nil {
		if schemaID != "" && existing.GetString("schema") != schemaID {
			existing.Set("schema", schemaID)
			if err := app.Save(existing); err != nil {
				log.Printf("⚠️ link schema to %s: %v", name, err)
			}
		}
		return existing, nil
	}
	rec := core.NewRecord(col)
	rec.Set("name", name)
	rec.Set("capability", capability)
	rec.Set("subject_suffix", suffix)
	rec.Set("description", desc)
	if schemaID != "" {
		rec.Set("schema", schemaID)
	}
	if err := app.Save(rec); err != nil {
		return nil, err
	}
	log.Printf("✅ Seeded platform operation: %s (%s)", name, capability)
	return rec, nil
}

func lookupSchemaID(app core.App, namespace, name, version string) string {
	if _, err := app.FindCollectionByNameOrId("message_schemas"); err != nil {
		return ""
	}
	rec, err := app.FindFirstRecordByFilter(
		"message_schemas",
		"organization = '' && namespace = {:ns} && name = {:name} && version = {:v}",
		map[string]any{"ns": namespace, "name": name, "v": version},
	)
	if err != nil || rec == nil {
		return ""
	}
	return rec.Id
}

func upsertGenericThing(app core.App, col *core.Collection, opIDs ...string) error {
	existing, _ := app.FindFirstRecordByFilter(
		col.Id,
		"organization = '' && code = 'generic_thing'",
		nil,
	)
	if existing != nil {
		// Ensure the links stay in place in case someone wiped them.
		existing.Set("operations", opIDs)
		return app.Save(existing)
	}
	rec := core.NewRecord(col)
	rec.Set("name", "Generic Thing")
	rec.Set("code", "generic_thing")
	rec.Set("description", "Fallback Thing Type with shared heartbeat and status operations.")
	rec.Set("capabilities", []string{"publish"})
	rec.Set("operations", opIDs)
	if err := app.Save(rec); err != nil {
		return err
	}
	log.Printf("✅ Seeded platform Thing Type: generic_thing")
	return nil
}
