package migrations

import (
	"fmt"
	"log"
	"slices"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// Drops the half of the Thing Type "contract layer" that nothing consumed:
//
//   - the message_schemas collection, never validated against or read outside
//     one widget's payload form
//   - thing_type_operations.schema, the relation into it
//   - thing_types.capabilities, a hand-kept union of its operations'
//     capabilities that nothing read and that could silently disagree with
//     them. The per-operation `capability` stays: it tells a request from a
//     reply on the same subject suffix.
//   - thing_types.nats_role, read by no code
//
// There is deliberately no schema.json re-import: a re-import cannot remove a
// field, and one placed after the removals would resurrect them in the same run.
// TestReimportCannotRemoveAField pins that against the real importer.
//
// Order is load-bearing: PocketBase refuses to delete a collection that still
// has relation references to it, so thing_type_operations.schema goes first.
// Every step is a no-op when its target is already absent, as on a fresh
// database.
func init() {
	m.Register(DropContractLayer, nil)
}

// DropContractLayer is the migration body, exported so a test can run it
// against a database that has the old schema; a fresh database has nothing to
// remove. The test is in package migrations_test because internal/testutil
// imports this package.
func DropContractLayer(app core.App) error {
	// 1. The relation into message_schemas, before the collection it points at.
	if err := dropField(app, "thing_type_operations", "relation_tto_schema"); err != nil {
		return err
	}

	// 2. The collection itself.
	if col, err := app.FindCollectionByNameOrId("message_schemas"); err == nil && col != nil {
		if err := app.Delete(col); err != nil {
			return fmt.Errorf("delete message_schemas: %w", err)
		}
		log.Println("✅ Dropped the message_schemas collection")
	}

	// 3. The two dead fields on thing_types.
	if err := dropField(app, "thing_types", "select490417661"); err != nil { // capabilities
		return err
	}
	if err := dropField(app, "thing_types", "relation_tt_nats_role"); err != nil { // nats_role
		return err
	}

	// 4. Stale values in leaf_nodes.synced_collections, which the leaf node
	// detail view rendered verbatim.
	if err := pruneSyncedCollections(app, "message_schemas"); err != nil {
		return err
	}

	return nil
}

// dropField removes one field by ID and persists the collection. An id matches
// the definition this migration was written against; a name matches whatever is
// live under it.
func dropField(app core.App, collection, fieldID string) error {
	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil || col == nil {
		return nil // collection is gone or was never created; nothing to drop
	}

	f := col.Fields.GetById(fieldID)
	if f == nil {
		return nil
	}

	col.Fields.RemoveById(fieldID)
	if err := app.Save(col); err != nil {
		return fmt.Errorf("drop %s.%s: %w", collection, f.GetName(), err)
	}

	log.Printf("✅ Dropped %s.%s", collection, f.GetName())
	return nil
}

// pruneSyncedCollections removes one value from every leaf node's
// synced_collections list.
func pruneSyncedCollections(app core.App, value string) error {
	nodes, err := app.FindAllRecords("leaf_nodes")
	if err != nil {
		return nil // collection absent on a database that predates leaf nodes
	}

	pruned := 0
	for _, n := range nodes {
		current := n.GetStringSlice("synced_collections")
		if !slices.Contains(current, value) {
			continue
		}
		next := slices.DeleteFunc(slices.Clone(current), func(s string) bool { return s == value })
		n.Set("synced_collections", next)
		if err := app.Save(n); err != nil {
			return fmt.Errorf("prune %q from leaf node %s: %w", value, n.Id, err)
		}
		pruned++
	}

	if pruned > 0 {
		log.Printf("✅ Removed %q from synced_collections on %d leaf node(s)", value, pruned)
	}
	return nil
}
