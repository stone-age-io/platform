package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// widen_capabilities rewrites the values stored in thing_types.capabilities from
// the legacy pub/sub/req-reply set to the new publish/subscribe/request/reply set.
// The schema.json import (handled by the initial_schema migration) widens the enum;
// this migration rewrites existing row data to match.
func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("thing_types")
		if err != nil {
			// Collection not present yet; nothing to migrate.
			return nil
		}

		records, err := app.FindAllRecords(collection.Id)
		if err != nil {
			return err
		}

		migrated := 0
		for _, rec := range records {
			caps := rec.GetStringSlice("capabilities")
			if len(caps) == 0 {
				continue
			}
			next, changed := remapCapabilities(caps)
			if !changed {
				continue
			}
			rec.Set("capabilities", next)
			if err := app.Save(rec); err != nil {
				return err
			}
			migrated++
		}

		if migrated > 0 {
			log.Printf("✅ Rewrote capabilities on %d thing_types record(s)", migrated)
		}
		return nil
	}, nil)
}

func remapCapabilities(old []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(old)+1)
	out := make([]string, 0, len(old)+1)
	changed := false
	add := func(v string) {
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range old {
		switch v {
		case "pub":
			changed = true
			add("publish")
		case "sub":
			changed = true
			add("subscribe")
		case "req-reply":
			changed = true
			add("request")
			add("reply")
		default:
			add(v)
		}
	}
	return out, changed
}
