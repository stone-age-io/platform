package migrations

import (
	"log"
	"regexp"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	m "ganigeorgiev.com/pocketbase/pocketbase/migrations"
)

// CodePattern is the validator on `code` in things, locations, thing_types and
// location_types.
//
// It is deliberately not the lowercase organizations.code pattern: these codes
// are stencilled on hardware (`DOOR-1`, `GW-KC-01`), and case-folding them would
// make the label and the record disagree. What it excludes, and why:
//
//  1. `.`: a code is a token in a NATS subject (ui/src/utils/subjectResolver.ts)
//     and a twin KV key. A dot splits it in two and messages silently stop
//     arriving.
//  2. `*` and `>`: NATS wildcards. Through resolveRolePattern a code is copied
//     into a signed JWT permission, so a wildcard code widens it to every
//     identity, a privilege escalation no rule review would catch.
//  3. Spaces, a leading hyphen, more than 63 characters: the code is also a
//     JetStream domain (hooks/leaf_config_routes.go), a KV key and a QR payload.
//     63 is the RFC 1123 label limit.
var CodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$`)

// codeValidatorCollections are the collections this migration patterns.
// organizations has its own, stricter pattern (schema_update_org_code.go).
var codeValidatorCollections = []string{"things", "locations", "thing_types", "location_types"}

// ApplyCodeValidators is schema_update_code_validators: it adds `pattern` and
// `max` to `code` on the four inventory collections, which accepted any string,
// NATS wildcards included.
//
// It reports non-conforming codes rather than rewriting them. `code` is frozen
// (schema_update_inventory_code_freeze.go, ADR 0002): it is printed on labels and
// baked into signed subjects, and a sweep would silently do what the freeze
// forbids. Such a row stays readable but cannot be saved until a human fixes it.
//
// Exported because `migrate up` on a fresh database already has the constraint
// and proves nothing; the test restores the old shape and calls this directly.
func ApplyCodeValidators(app core.App) error {
	if len(SchemaJSON) == 0 {
		log.Println("⚠️ SchemaJSON is empty, skipping code validators")
		return nil
	}

	if err := app.ImportCollectionsByMarshaledJSON(SchemaJSON, false); err != nil {
		return err
	}

	for _, name := range codeValidatorCollections {
		offenders, err := NonConformingCodes(app, name)
		if err != nil {
			return err
		}
		for _, o := range offenders {
			log.Printf("⚠️ %s record %s has a code the new validator rejects: %q — it stays readable, but cannot be saved until the code is corrected", name, o.ID, o.Code)
		}
	}

	log.Println("✅ code validated on things, locations, thing_types, location_types")
	return nil
}

func init() {
	m.Register(ApplyCodeValidators, func(app core.App) error {
		// Down: no-op. Reverting would re-import all of schema.json.
		return nil
	})
}

// Offender is one record whose stored code does not satisfy CodePattern.
type Offender struct {
	ID   string
	Code string
}

// NonConformingCodes returns the records in one collection whose non-blank code
// the validator would reject. Blank is skipped because `code` is optional. The
// regex runs in Go because SQLite has no REGEXP without an extension.
func NonConformingCodes(app core.App, collection string) ([]Offender, error) {
	records, err := app.FindAllRecords(collection)
	if err != nil {
		return nil, err
	}

	var offenders []Offender
	for _, r := range records {
		code := r.GetString("code")
		if code == "" || CodePattern.MatchString(code) {
			continue
		}
		offenders = append(offenders, Offender{ID: r.Id, Code: code})
	}
	return offenders, nil
}
