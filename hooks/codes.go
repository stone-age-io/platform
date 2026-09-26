package hooks

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Generated Thing and Location codes -- ADR 0003 in platform-docs.
//
// A generated code is `PFX-XXX-XXX`, or `XXX-XXX` when the record's type has no
// prefix. The shape comes from BEC Systems' "Human-Friendly Industrial Device
// IDs": short, random, chunked, from an alphabet with the look-alikes removed.
// The prefix is ours; everything after it means nothing, deliberately.
//
// WHY RANDOM. A sparse random space is a check digit for free: an organization
// with a thousand cameras uses about 0.0005% of the codes under one prefix, so a
// mistyped code almost always matches nothing and the scanner says "not found".
// Sequential codes are the opposite -- a typo of CAM-042 is CAM-043, a real
// record, and the tech lands on the wrong device with nothing looking wrong.
//
// WHY ONE GENERATOR, HERE. ui/src/utils/subjectResolver.ts once carried a header
// claiming it mirrored a Go package nobody ever wrote. A second generator in
// TypeScript would drift the same way, and a code is frozen and printed, so drift
// would be permanent. Clients ask GET /api/codes/suggest instead.

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
// location_types.prefix in schema.json. Letters only, so the boundary between
// prefix and random part is always visible: CA-9KD-4PX reads as "type, then
// noise", which a digit inside the prefix would blur.
var TypePrefixPattern = regexp.MustCompile(`^[A-Z]{1,4}$`)

// codeAttempts bounds the collision retry. A collision needs two draws from
// roughly 210 million codes per prefix to meet, so a second attempt is already
// astronomically rare; eight only exists so that a bug (say, an alphabet cut to
// one symbol) fails loudly instead of looping.
const codeAttempts = 8

// suggestMax caps GET /api/codes/suggest. Enough for a pallet of labels; small
// enough that one request cannot turn into a long run of lookups.
const suggestMax = 500

// codeKind names the pair of collections behind one kind of code.
type codeKind struct {
	collection     string
	typeCollection string
}

var codeKinds = map[string]codeKind{
	"thing":    {thingsCollection, thingTypesCollection},
	"location": {locationsCollection, locationTypesCollection},
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

// codeChunk draws three symbols until the chunk holds at least one letter AND
// one digit. That keeps a chunk recognisably a code, and it means a chunk can
// never spell a three-letter word. About half of all draws qualify (14,490 of
// 27,000), so the loop rarely runs twice.
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
// holds code, ignoring case. Both collections are checked so a generated code
// never clashes across kinds, prefix or no prefix: the unique indexes are per
// collection and cannot see each other.
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
// organization holds. Exported for POST /api/org/things, which needs the code
// before it saves anything: the Thing's synthetic email and its NATS username
// are both built from it.
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
// blank typeID is an untyped record and has no prefix. A type that does not
// exist, or belongs to another organization, is reported as an error rather than
// quietly treated as "no prefix": the caller asked for that type's codes.
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
//     under the record's type prefix. Every Thing and Location therefore has a
//     code from here on -- which is what lets each one carry a label, and what
//     ADR 0004's location path needs. Presence is guaranteed by this hook, not
//     by `required` in the schema, the same technique organizations.code uses.
//   - thing_types / location_types create and update: a prefix already used by
//     the OTHER type collection in the organization is refused. Thing prefixes
//     and Location prefixes are separate sets; the partial unique index on each
//     collection covers its own half and cannot see the other table.
//   - GET /api/codes/suggest: codes a client can show or pre-print before it
//     creates anything.
//
// OnRecordCreate rather than an after-success hook, for the reason
// RegisterOrgCode gives: `code` is frozen by the update rule the moment it
// exists, so it has to be written WITH the record. Model hooks fire for
// app.Save too, so the admin panel, the record API and every server-side save
// all go through here.
func RegisterCodes(app *pocketbase.PocketBase, membershipCollection string) {
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
			clash, _ := e.App.FindFirstRecordByFilter(other,
				"organization = {:org} && prefix = {:prefix}",
				dbx.Params{"org": e.Record.GetString("organization"), "prefix": prefix})
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

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// GET /api/codes/suggest?kind=thing|location&type=<type id>&count=<n>
		//
		// Codes that are free at the moment they are returned. They are NOT
		// reserved: in a space of about 210 million per prefix, a clash between
		// suggesting and creating is negligible, and if one happens the unique
		// index refuses the create and the client asks again. Reserving would
		// need a table, an expiry and a cleanup job to prevent something that
		// does not happen.
		//
		// The inventory roles only, the same ones that can create the records
		// these codes are for.
		se.Router.GET("/api/codes/suggest", func(re *core.RequestEvent) error {
			orgID, _, err := requireMembership(re, membershipCollection, rolesInventory)
			if err != nil {
				return err
			}

			q := re.Request.URL.Query()
			kindName := q.Get("kind")
			if kindName == "" {
				kindName = "thing"
			}
			kind, ok := codeKinds[kindName]
			if !ok {
				return re.BadRequestError("kind must be thing or location", nil)
			}

			count := 1
			if s := q.Get("count"); s != "" {
				count, err = strconv.Atoi(s)
				if err != nil || count < 1 || count > suggestMax {
					return re.BadRequestError(fmt.Sprintf("count must be between 1 and %d", suggestMax), nil)
				}
			}

			prefix, err := typePrefix(re.App, kind.typeCollection, q.Get("type"), orgID)
			if err != nil {
				return re.BadRequestError(err.Error(), nil)
			}

			// Deduplicated within the response too: two identical codes in one
			// batch of labels would pass every database check and still put the
			// same sticker on two devices.
			seen := make(map[string]bool, count)
			codes := make([]string, 0, count)
			for len(codes) < count {
				code, err := NewUniqueCode(re.App, orgID, prefix)
				if err != nil {
					return re.InternalServerError("failed to generate a code", err)
				}
				if seen[code] {
					continue
				}
				seen[code] = true
				codes = append(codes, code)
			}

			return re.JSON(200, map[string]any{
				"prefix": prefix,
				"codes":  codes,
			})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}
