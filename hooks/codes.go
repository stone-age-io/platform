package hooks

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Generated Thing and Location codes -- ADR 0003 in platform-docs.
//
// A generated code is `PFX-XXX-XXX`, or `XXX-XXX` when the record's type has no
// prefix: short, random, chunked, from an alphabet without look-alikes.
//
// Random because a sparse space is a free check digit: a typo almost always
// matches nothing, where a sequential code's typo lands on the neighbouring
// device.
//
// This is the only generator. Do not add a TypeScript copy: a code is frozen and
// printed, so drift would be permanent. A client wanting a code leaves it blank.

const (
	thingsCollection        = "things"
	locationsCollection     = "locations"
	thingTypesCollection    = "thing_types"
	locationTypesCollection = "location_types"
)

// codeLetters and codeDigits are A-Z and 0-9 without 0 O 1 I 2 Z: 23 letters
// and 7 digits, 30 symbols in all.
const (
	codeLetters = "ABCDEFGHJKLMNPQRSTUVWXY"
	codeDigits  = "3456789"
	codeSymbols = codeLetters + codeDigits
)

// TypePrefixPattern mirrors the `pattern` on thing_types.prefix and
// location_types.prefix in schema.json. Letters only, so the prefix stays
// visibly separate from the random part.
var TypePrefixPattern = regexp.MustCompile(`^[A-Z]{1,4}$`)

// codeAttempts bounds the collision retry. A second attempt is already rare;
// the bound makes a bug fail loudly instead of looping.
const codeAttempts = 8

// codeKind names the pair of collections behind one kind of code.
type codeKind struct {
	collection     string
	typeCollection string
}

var codeKinds = []codeKind{
	{thingsCollection, thingTypesCollection},
	{locationsCollection, locationTypesCollection},
}

// GenerateCode returns a fresh code under prefix, or a bare `XXX-XXX` when
// prefix is empty. It does not check for collisions; NewUniqueCode does.
func GenerateCode(prefix string) (string, error) {
	first, err := codeChunk()
	if err != nil {
		return "", err
	}
	second, err := codeChunk()
	if err != nil {
		return "", err
	}
	code := first + "-" + second
	if prefix != "" {
		code = prefix + "-" + code
	}
	return code, nil
}

// codeChunk draws three symbols until the chunk holds at least one letter and
// one digit, so it never spells a word. About half of all draws qualify.
func codeChunk() (string, error) {
	n := big.NewInt(int64(len(codeSymbols)))
	for {
		var b [3]byte
		for i := range b {
			v, err := rand.Int(rand.Reader, n)
			if err != nil {
				return "", err
			}
			b[i] = codeSymbols[v.Int64()]
		}
		chunk := string(b[:])
		if strings.ContainsAny(chunk, codeLetters) && strings.ContainsAny(chunk, codeDigits) {
			return chunk, nil
		}
	}
}

// codeInUse reports whether any Thing or Location in the organization already
// holds code, ignoring case. Both are checked because the unique indexes are
// per collection and cannot see each other.
func codeInUse(app core.App, orgID, code string) (bool, error) {
	for _, collection := range []string{thingsCollection, locationsCollection} {
		n, err := app.CountRecords(collection,
			dbx.HashExp{"organization": orgID},
			dbx.NewExp("LOWER([[code]]) = {:code}", dbx.Params{"code": strings.ToLower(code)}),
		)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// NewUniqueCode generates a code under prefix that no Thing or Location in the
// organization holds. POST /api/org/things calls it because it builds the
// synthetic email and NATS username from the code before saving anything.
func NewUniqueCode(app core.App, orgID, prefix string) (string, error) {
	for range codeAttempts {
		code, err := GenerateCode(prefix)
		if err != nil {
			return "", err
		}
		taken, err := codeInUse(app, orgID, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("no free code under prefix %q after %d attempts", prefix, codeAttempts)
}

// typePrefix returns the prefix of the type record typeID, scoped to orgID. A
// blank typeID has no prefix; a missing or foreign type is an error, not "no
// prefix".
func typePrefix(app core.App, typeCollection, typeID, orgID string) (string, error) {
	if typeID == "" {
		return "", nil
	}
	rec, err := app.FindRecordById(typeCollection, typeID)
	if err != nil || rec.GetString("organization") != orgID {
		return "", fmt.Errorf("%s %q is not a type in this organization", typeCollection, typeID)
	}
	return rec.GetString("prefix"), nil
}

// RegisterCodes binds code generation and the prefix rules:
//
//   - things / locations create: a blank code is replaced with a generated one
//     under the record's type prefix. This hook, not `required` in the schema,
//     guarantees every record a code (ADR 0004's location path needs one).
//   - thing_types / location_types create and update: a prefix the OTHER type
//     collection in the organization holds is refused, because each partial
//     unique index cannot see the other table.
//
// OnRecordCreate, for the reason RegisterOrgCode gives: `code` is frozen the
// moment it exists, so it must be written with the record. Model hooks also
// cover app.Save.
//
// Deliberately no route hands out codes ahead of a create; blank-at-save covers
// every need.
func RegisterCodes(app *pocketbase.PocketBase) {
	for _, kind := range codeKinds {
		app.OnRecordCreate(kind.collection).BindFunc(func(e *core.RecordEvent) error {
			if strings.TrimSpace(e.Record.GetString("code")) != "" {
				return e.Next()
			}
			orgID := e.Record.GetString("organization")
			prefix, err := typePrefix(e.App, kind.typeCollection, e.Record.GetString("type"), orgID)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			code, err := NewUniqueCode(e.App, orgID, prefix)
			if err != nil {
				return apis.NewBadRequestError("Could not generate a code: "+err.Error(), nil)
			}
			e.Record.Set("code", code)
			return e.Next()
		})
	}

	otherTypeCollection := map[string]string{
		thingTypesCollection:    locationTypesCollection,
		locationTypesCollection: thingTypesCollection,
	}
	for collection, other := range otherTypeCollection {
		check := func(e *core.RecordEvent) error {
			prefix := e.Record.GetString("prefix")
			if prefix == "" {
				return e.Next()
			}
			clash, err := e.App.FindFirstRecordByFilter(other,
				"organization = {:org} && prefix = {:prefix}",
				dbx.Params{"org": e.Record.GetString("organization"), "prefix": prefix})
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				// Fail closed: an unchecked prefix could collide with the other set.
				return fmt.Errorf("check prefix %q against %s: %w", prefix, other, err)
			}
			if clash != nil {
				return apis.NewBadRequestError(fmt.Sprintf(
					"The prefix %q is already used by the %s %q. Thing and Location prefixes are separate sets.",
					prefix, strings.ReplaceAll(strings.TrimSuffix(other, "s"), "_", " "), clash.GetString("name")), nil)
			}
			return e.Next()
		}
		app.OnRecordCreate(collection).BindFunc(check)
		app.OnRecordUpdate(collection).BindFunc(check)
	}
}
