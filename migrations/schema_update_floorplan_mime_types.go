package migrations

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// schema_update_floorplan_mime_types restricts `locations.floorplan` to the same
// image types `users.avatar` and `organizations.logo` already allow. It was the
// only upload field on the platform with an empty `mimeTypes` list, i.e. one
// that accepted any file at all.
//
// The reason it matters is not the upload, it is the THUMBNAIL. PocketBase's
// file API allows `?thumb=100x100` for any file field regardless of the field's
// own `thumbs` list -- `defaultThumbSizes` in apis/file.go is consulted before
// `fileField.Thumbs`, so `"thumbs": []` on this field never meant "no thumb can
// be generated". A thumb request runs the stored bytes through
// `imaging.Decode` + `Resize` (tools/filesystem/filesystem.go), and
// disintegration/imaging registers golang.org/x/image/tiff, so an unrestricted
// field handed the decoder arbitrary formats. imaging v1.6.2 panics on a
// crafted TIFF (CVE-2023-36308, GHSA-q7pp-wcgr-pffx) and is unmaintained: there
// is no patched release to upgrade to, so narrowing what reaches the decoder is
// the only lever this repo has.
//
// The blast radius was small -- the panic is recovered by net/http, the caller
// has to be an authenticated member of the organization, and the file is their
// own -- which is why the advisory is a Low and this is hardening rather than a
// fix. It is worth doing because it costs one field definition and the field
// should have been image-only on its own merits.
//
// EXISTING FILES ARE UNAFFECTED. `mimeTypes` is validated on upload, not on
// read, and an untouched relation is not re-validated on save, so a location
// already holding an out-of-list floorplan keeps serving it. Only new uploads
// are constrained.
//
// Verified against a real install rather than trusted: a database migrated by
// the previous binary was re-migrated by this one and `locations.floorplan`
// read back with the list populated. That check is not optional here -- a
// re-import silently keeps the LIVE field definition whenever a name matches
// and the field id does not (see the re-import note in CLAUDE.md), and this
// migration is exactly the shape that fails that way. It applies because
// schema.json carries the same id the live column has (`file3043290937`).
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
