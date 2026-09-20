package migrations

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_inventory_photos adds `photo` to things and locations: one
// image of the physical object, for the moment somebody is standing in front of
// it wondering whether this is the right one.
//
// WHY A THING NEEDS ONE. A record's name, type and location describe it but do
// not show it, and the install context that would ("grey unit behind the panel,
// third from left") currently lives in one technician's head and leaves when
// they do. The thing and location DETAIL views are where it is set and the only
// place it is shown -- it was briefly also in ScannerWidget and ThingMapDrawer
// and was removed from both, because after scanning a label you are already
// looking at the device, and a 40px thumbnail beside a name answers nothing.
//
// ONE PHOTO, NOT A GALLERY -- and the column type is NOT the reason. An earlier
// version of this comment said going 1 -> N was blocked by the move from TEXT to
// JSON; that is wrong. core.normalizeSingleVsMultipleFieldChanges rebuilds the
// column and wraps every existing value in json_array(...), and the deterministic
// field id below means a plain re-import applies the change, so going up is
// cheap. Two things argue for staying at 1. An array has no primary element, so
// anything showing "the" photo silently becomes photo[0] -- whatever was uploaded
// first. And coming back down is destructive: multiple -> single keeps
// json_extract(..., '$[#-1]'), the LAST file, and PocketBase deliberately leaves
// the rest orphaned in pb_data/storage, in every backup, forever. Cheap to add
// once a second photo has a reader; expensive to undo once every record has five.
//
// NOT SVG, though locations.floorplan allows it. An SVG "photo" is not a
// photograph, and SVG reaching an image decoder is the same family of problem
// schema_update_floorplan_mime_types.go narrowed -- there is no reason to widen
// it again on a field whose entire purpose is camera output.
//
// SIZE. 2 MiB with a 400x400 thumb. Phone cameras produce 3-5 MB, so the cap
// would reject real uploads on its own; ui/src/utils/imageResize.ts downscales
// before upload so the cap is a backstop rather than the mechanism. This
// matters more here than for the other file fields because things and locations
// are the collections with thousands of rows, files live in pb_data/storage
// with no object store configured, and every one of them rides in every backup.
//
// NOTE the thumb list. PocketBase serves `100x100` for any file field whether
// or not it is declared (apis/file.go, defaultThumbSizes); every OTHER size has
// to be in this list or the request silently falls through to the full image.
// 100x100 covers list rows, 400x400 covers a detail view at 2x DPR.
//
// PROTECTION IS NOT SET HERE, deliberately. schema.json carries
// `"protected": true` for fresh installs, and ProtectFileFields walks the live
// schema on every upgrade, so this field is protected by a migration written
// before it existed. TestEveryFileFieldInTheSchemaIsProtected is the guard.
// That is the whole point of deriving that migration from the schema rather
// than listing fields in it.
//
// NO BACKFILL. An absent file is the empty string, which is what an existing
// row already has -- unlike schema_update_device_active_flag.go, where a new
// non-null bool under a live authRule locked every provisioned device out.
func init() {
	m.Register(AddInventoryPhotos, nil)
}

// inventoryPhotoCollections are the two collections that get the field.
var inventoryPhotoCollections = []string{"things", "locations"}

// AddInventoryPhotos is the migration body.
//
// Exported and named for the reason DropContractLayer and ProtectFileFields
// are: on a fresh database schema.json already carries the field, so the
// migration set alone only ever exercises the no-op path and a green
// `migrate up` proves nothing. The test removes the field first and then runs
// this.
//
// It adds the field EXPLICITLY rather than re-importing schema.json. A
// re-import would work for an addition -- the trap in CLAUDE.md is about
// removals and id mismatches -- but it would also replay every other collection
// definition in the file as a side effect of adding one field, which makes the
// migration's blast radius "whatever else changed in schema.json since the last
// release" rather than the thing it says it does.
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
			// The id PocketBase would generate anyway ('file' + crc32 of the
			// name). Stating it means schema.json and a migrated database agree
			// on the id, which is what decides whether a future re-import can
			// touch this field at all.
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
