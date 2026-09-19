package migrations

import (
	"log"
	"regexp"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// CodePattern is the validator now on `code` in things, locations, thing_types
// and location_types. It is exported so the UI-facing docs and any future
// server-side derivation can point at one definition rather than three copies.
//
// It is deliberately NOT the organizations.code pattern
// (`^[a-z0-9][a-z0-9-]{1,30}$`). An organization code is slugified from a name
// by hooks/org_code.go and never typed by an installer, so lowercase-only costs
// nothing. Every other code is the string stencilled on the hardware — `DOOR-1`,
// `KC-DC1`, `GW-KC-01` are what a tech reads out loud — and case-folding those
// would mean the label and the record disagree. Underscore is admitted because
// thing_type_operations.name already allows it and NATS tokens and KV keys both
// accept it.
//
// What the pattern actually buys, in order of how much it matters:
//
//  1. **No `.`** A code is interpolated into a NATS subject (the
//     `{thing}` / `{location}` / `{thing_type_code}` variables in
//     ui/src/utils/subjectResolver.ts) and into a twin KV key
//     (`<kind>.<code>.<prop>`). A dot inside a code silently splits one token
//     into two, so the subject a publisher computes and the subject a
//     subscriber's filter expects stop being the same string. Nothing errors;
//     messages simply stop arriving.
//  2. **No `*` and no `>`** Those are the NATS wildcards. A code of `*` reaching
//     resolveRolePattern widens a permission from one identity to every identity
//     of that kind, and a code of `>` widens it to the rest of the subject. This
//     is the security-relevant half: `nats_users.publish_permissions` is copied
//     verbatim into the signed JWT, so a pattern built from an unvalidated code
//     is a privilege-escalation path that no rule review would catch.
//  3. **No spaces, no leading hyphen, 63 characters** The code is a JetStream
//     domain (an edge site's domain IS the Thing's code — see
//     hooks/leaf_config_routes.go), a KV key component, and a QR payload. 63 is
//     the RFC 1123 label limit, which is the tightest of those and leaves plenty
//     of room: the longest code anywhere in the repo is 17 characters.
var CodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$`)

// codeValidatorCollections are the four collections this migration patterns.
// organizations is absent because it has carried its own, stricter pattern
// since schema_update_org_code.go.
var codeValidatorCollections = []string{"things", "locations", "thing_types", "location_types"}

// schema_update_code_validators adds `pattern` + `max` to `code` on the four
// inventory collections. They had neither: any string of any length was
// accepted, including one containing a NATS wildcard.
//
// **No sweep, deliberately, and this is the interesting part.** The usual rule
// (schema_update_device_active_flag.go) is that a new constraint over existing
// rows needs a backfill in the same migration. That rule inverts here, because
// `code` is FROZEN — schema_update_inventory_code_freeze.go put
// `@request.body.code:changed = false` on all four update rules for exactly the
// reasons in ADR 0002: a code is printed on a label, baked into a signed subject
// and resolved by sibling apps on `(organization, code)`. A migration that
// rewrote non-conforming codes to make them fit would be doing, silently and at
// scale, the precise thing the freeze exists to prevent.
//
// So the migration reports instead. A row with a non-conforming code keeps
// working and keeps being readable; it simply cannot be SAVED again until a
// human decides what its code should be, which is the right person to decide.
// Nothing in the repo trips this — all 108 code literals across demoseed, hooks
// and migrations already conform — so the log line is for installs with
// hand-entered inventory.
// ApplyCodeValidators is the migration body, exported for the same reason
// DropContractLayer is: a fresh database imports the already-patterned
// schema.json, so `migrate up` reaches this migration with the constraint
// already in place and proves nothing. The test restores the unpatterned shape
// first and calls this directly.
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
		// Down: no-op. Reverting would re-import the whole of schema.json and
		// drag every unrelated rule back with it, the same reason
		// schema_update_inventory_code_freeze.go declines to.
		return nil
	})
}

// Offender is one record whose stored code does not satisfy CodePattern.
type Offender struct {
	ID   string
	Code string
}

// NonConformingCodes returns the records in one collection whose non-blank code
// the validator would now reject. Blank is skipped: `code` is optional on all
// four collections and the unique index covers non-blank codes only, so a record
// without one is an ordinary state rather than a violation.
//
// The regex runs in Go rather than SQL because SQLite has no REGEXP operator
// without an extension, and these tables are small enough that reading the
// column is cheaper than shipping one.
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
