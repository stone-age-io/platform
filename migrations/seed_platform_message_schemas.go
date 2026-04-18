package migrations

import (
	"encoding/json"
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// seed_platform_message_schemas inserts the platform-shipped JSON Schemas
// referenced by shared operations (per Thing Type Spec §7). Idempotent —
// matches on (organization, namespace, name, version).
//
// Operation→schema linking is handled by seed_platform_operations, which runs
// after this migration alphabetically and can look up the schema IDs by
// identity at the time each operation is created.
func init() {
	m.Register(func(app core.App) error {
		schemasCol, err := app.FindCollectionByNameOrId("message_schemas")
		if err != nil {
			// Stage 3 collection not present yet.
			return nil
		}

		if _, err := upsertSchema(app, schemasCol, "common", "heartbeat", "1.0.0",
			"Generic liveness + identity heartbeat used by any Thing.",
			map[string]any{
				"type":     "object",
				"required": []string{"timestamp", "uptime_seconds"},
				"properties": map[string]any{
					"timestamp":        map[string]any{"type": "string", "format": "date-time"},
					"uptime_seconds":   map[string]any{"type": "integer", "minimum": 0},
					"firmware_version": map[string]any{"type": "string"},
				},
			}); err != nil {
			return err
		}

		if _, err := upsertSchema(app, schemasCol, "common", "status", "1.0.0",
			"Online/offline plus optional status message.",
			map[string]any{
				"type":     "object",
				"required": []string{"online"},
				"properties": map[string]any{
					"online":  map[string]any{"type": "boolean"},
					"message": map[string]any{"type": "string"},
				},
			}); err != nil {
			return err
		}

		if _, err := upsertSchema(app, schemasCol, "common", "command_ack", "1.0.0",
			"Generic acknowledgement for subscribed commands.",
			map[string]any{
				"type":     "object",
				"required": []string{"ok"},
				"properties": map[string]any{
					"ok":      map[string]any{"type": "boolean"},
					"message": map[string]any{"type": "string"},
					"ref_id":  map[string]any{"type": "string"},
				},
			}); err != nil {
			return err
		}

		return nil
	}, nil)
}

func upsertSchema(app core.App, col *core.Collection, namespace, name, version, description string, schema map[string]any) (string, error) {
	existing, _ := app.FindFirstRecordByFilter(
		col.Id,
		"organization = '' && namespace = {:ns} && name = {:name} && version = {:v}",
		map[string]any{"ns": namespace, "name": name, "v": version},
	)
	if existing != nil {
		return existing.Id, nil
	}
	rec := core.NewRecord(col)
	rec.Set("namespace", namespace)
	rec.Set("name", name)
	rec.Set("version", version)
	rec.Set("format", "json_schema")
	rec.Set("description", description)
	payload, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	rec.Set("schema", payload)
	if err := app.Save(rec); err != nil {
		return "", err
	}
	log.Printf("✅ Seeded platform message schema: %s/%s@%s", namespace, name, version)
	return rec.Id, nil
}

