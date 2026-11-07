package migrations_test

import (
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/testutil"
	"platform/migrations"
)

var photoCollections = []string{"things", "locations"}

// dropPhoto puts a collection back to the pre-migration state, since a fresh
// database already has the field.
func dropPhoto(t *testing.T, app *pocketbase.PocketBase, collection string) {
	t.Helper()

	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	f := col.Fields.GetByName("photo")
	if f == nil {
		t.Fatalf("%s has no photo field to drop", collection)
	}
	col.Fields.RemoveById(f.GetId())
	if err := app.Save(col); err != nil {
		t.Fatalf("drop %s.photo: %v", collection, err)
	}
	if !tableHasColumn(t, app, collection, "photo") {
		return
	}
	t.Fatalf("fixture removed the %s.photo definition but not its column", collection)
}

func photoField(t *testing.T, app *pocketbase.PocketBase, collection string) *core.FileField {
	t.Helper()

	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatalf("find %s: %v", collection, err)
	}
	f, ok := col.Fields.GetByName("photo").(*core.FileField)
	if !ok {
		t.Fatalf("%s.photo is missing or is not a file field", collection)
	}
	return f
}

// TestAddInventoryPhotosAddsTheField is the test `migrate up` cannot provide.
func TestAddInventoryPhotosAddsTheField(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, c := range photoCollections {
		dropPhoto(t, app, c)
	}

	if err := migrations.AddInventoryPhotos(app); err != nil {
		t.Fatalf("AddInventoryPhotos: %v", err)
	}

	for _, c := range photoCollections {
		f := photoField(t, app, c)
		if !tableHasColumn(t, app, c, "photo") {
			t.Errorf("%s.photo has a definition but no column", c)
		}
		// One photo, not a gallery: going back from many to one loses files.
		if f.MaxSelect != 1 {
			t.Errorf("%s.photo maxSelect = %d, want 1", c, f.MaxSelect)
		}
		if f.MaxSize != 2<<20 {
			t.Errorf("%s.photo maxSize = %d, want %d", c, f.MaxSize, 2<<20)
		}
	}
}

// TestAddInventoryPhotosIsIdempotent covers the fresh-database path and a
// replayed migration set.
func TestAddInventoryPhotosIsIdempotent(t *testing.T) {
	app := testutil.SetupApp(t)

	for i := 0; i < 2; i++ {
		if err := migrations.AddInventoryPhotos(app); err != nil {
			t.Fatalf("AddInventoryPhotos run %d: %v", i+1, err)
		}
	}
	for _, c := range photoCollections {
		photoField(t, app, c)
	}
}

// TestPhotoIsNotSvg pins the mime list: this field is camera output, and SVG
// would re-open the decoder surface schema_update_floorplan_mime_types.go
// narrowed.
func TestPhotoIsNotSvg(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, c := range photoCollections {
		for _, mt := range photoField(t, app, c).MimeTypes {
			if mt == "image/svg+xml" {
				t.Errorf("%s.photo accepts SVG; it is a photograph field, not a drawing one", c)
			}
		}
		if len(photoField(t, app, c).MimeTypes) == 0 {
			t.Errorf("%s.photo has an empty mime list, which accepts ANY file", c)
		}
	}
}

// TestPhotoThumbsAreDeclared guards a silent regression: a request for an
// undeclared thumb size (anything but 100x100) quietly serves the full-size
// original.
func TestPhotoThumbsAreDeclared(t *testing.T) {
	app := testutil.SetupApp(t)

	for _, c := range photoCollections {
		f := photoField(t, app, c)
		var found bool
		for _, th := range f.Thumbs {
			if th == "400x400" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s.photo does not declare the 400x400 thumb the detail views request; "+
				"PocketBase will serve the full-size original instead, with no error", c)
		}
	}
}
