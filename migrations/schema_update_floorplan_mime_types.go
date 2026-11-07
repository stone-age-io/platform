package migrations

import (
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// schema_update_floorplan_mime_types restricts `locations.floorplan` to the same
// image types `users.avatar` and `organizations.logo` allow. Its `mimeTypes` was
// empty, so it accepted any file.
//
// The risk is the thumbnail. PocketBase serves `?thumb=100x100` for any file
// field whatever its `thumbs` list says (`defaultThumbSizes` in apis/file.go),
// and a thumb runs the bytes through disintegration/imaging, which decodes TIFF.
// imaging v1.6.2 panics on a crafted TIFF (CVE-2023-36308) and has no patched
// release, so narrowing what reaches the decoder is the only lever. Low severity:
// net/http recovers the panic and the caller must be a member.
//
// Existing files keep serving; `mimeTypes` is checked on upload only. Verified on
// a real install: it applies because schema.json carries the live field id
// (`file3043290937`), see the re-import note in migrations/CLAUDE.md.
func init() {
	m.Register(func(app core.App) error {
		if len(SchemaJSON) == 0 {
			log.Println("⚠️ SchemaJSON is empty, skipping floorplan mime type update")
			return nil
		}
		if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
			return err
		}
		log.Println("✅ Restricted locations.floorplan to image mime types")
		return nil
	}, nil)
}
