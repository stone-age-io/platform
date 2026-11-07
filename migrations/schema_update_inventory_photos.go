package migrations

import (
	"fmt"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_inventory_photos adds `photo` to things and locations: one image
// of the physical object, shown on the detail views (rules: ui/CLAUDE.md).
//
// One photo, not a gallery. Going to many is a cheap re-import later, but an
// array has no primary element, and going back down keeps only the last file
// and orphans the rest in pb_data/storage.
//
// Not SVG: this is camera output, and SVG would re-widen the decoder surface
// schema_update_floorplan_mime_types.go narrowed. 2 MiB is a backstop;
// ui/src/utils/imageResize.ts downscales before upload.
//
// Thumbs: PocketBase serves 100x100 for any file field; every other size must
// be listed or the request falls through to the full image. 400x400 is the
// detail view at 2x DPR.
//
// The field is created protected; ProtectFileFields, which walks the live schema
// rather than a field list, covers it too (TestEveryFileFieldInTheSchemaIsProtected).
// No backfill: an absent file is the empty string existing rows already hold.
func init() {
	m.Register(AddInventoryPhotos, nil)
}

// inventoryPhotoCollections are the two collections that get the field.
var inventoryPhotoCollections = []string{"things", "locations"}

// AddInventoryPhotos is the migration body, exported because a fresh database
// already has the field; the test removes it first and then runs this.
//
// It adds the field explicitly rather than re-importing schema.json, so its
// effect is this one field and not whatever else changed in schema.json.
func AddInventoryPhotos(app core.App) error {
	for _, name := range inventoryPhotoCollections {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return fmt.Errorf("add %s.photo: find collection: %w", name, err)
		}

		if col.Fields.GetByName("photo") != nil {
			continue // already applied
		}

		col.Fields.Add(&core.FileField{
			// PocketBase's deterministic id ('file' + crc32 of the name), so
			// schema.json and a migrated database agree and a re-import can
			// reach this field.
			Id:        "file347571224",
			Name:      "photo",
			MaxSelect: 1,
			MaxSize:   2 << 20,
			MimeTypes: []string{"image/jpeg", "image/png", "image/webp"},
			Thumbs:    []string{"400x400"},
			Protected: true,
		})

		if err := app.Save(col); err != nil {
			return fmt.Errorf("add %s.photo: %w", name, err)
		}
		log.Printf("✅ Added %s.photo", name)
	}

	return nil
}
