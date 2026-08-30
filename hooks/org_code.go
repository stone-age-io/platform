package hooks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// OrgCodePattern mirrors the `pattern` on organizations.code in schema.json.
// Exported so bootstrap and the backfill migration can pre-flight a derived
// code against the same rule the field enforces. Duplicated from the schema
// rather than derived from it, because both callers run before the import has
// validated anything -- but duplicated exactly once, not once per caller.
var OrgCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

// OrgCodeMaxLen is the upper bound in the `code` field's pattern,
// ^[a-z0-9][a-z0-9-]{1,30}$ -- one leading alphanumeric plus up to thirty more.
const OrgCodeMaxLen = 31

// Slugify derives an organization code from a display name: lowercased, every
// run of characters outside [a-z0-9] collapsed to a single hyphen, trimmed, and
// truncated to OrgCodeMaxLen.
//
// It can legitimately return a string the field pattern rejects -- a name with
// no alphanumerics at all (""), or one that reduces to a single character
// ("X" -> "x", below the two-character minimum). That is deliberate. The hook
// below refuses instead of padding or prefixing, because the code is immutable
// once saved and a guess would be permanent. Asking the operator costs one
// error message; guessing costs a superuser edit later.
//
// A leading digit is NOT one of those cases: "816tech" is a valid code. The
// pattern required a leading letter until an operator org named exactly that
// could not be migrated, and nothing downstream justified the restriction --
// NATS subject tokens, JetStream domains (always prefixed "edge-"), KV bucket
// names and RFC 1123 hostname labels all permit a leading digit.
func Slugify(name string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		default:
			pendingHyphen = true
		}
	}
	s := b.String()
	if len(s) > OrgCodeMaxLen {
		s = strings.TrimRight(s[:OrgCodeMaxLen], "-")
	}
	return s
}

// RegisterOrgCode fills organizations.code on create when no code was supplied,
// deriving it from the organization name.
//
// Bound to OnRecordCreate rather than OnRecordAfterCreateSuccess because the
// value has to be persisted WITH the record: the code is frozen by the update
// rule the moment it exists, so a hook that writes it afterwards would be
// writing through a rule that forbids the write.
//
// One hook covers every creation path. Organizations are created in exactly
// three places -- admin/OrganizationFormView via the record API, and the System
// and operator orgs in bootstrap.go -- and the two bootstrap paths go through
// app.Save, which fires record hooks because RegisterOrgCode is bound in main.go
// before app.Start() rather than inside OnServe.
//
// There is deliberately no auto-suffixing. A derived code that collides is
// refused so the operator picks the real one, which is the same stance
// migrations/schema_update_unique_org_code.go takes toward duplicate codes and
// for the same reason: silently saving `acme-2` is silently saving it forever.
func RegisterOrgCode(app *pocketbase.PocketBase, orgCollection string) {
	app.OnRecordCreate(orgCollection).BindFunc(func(e *core.RecordEvent) error {
		name := e.Record.GetString("name")
		code := strings.TrimSpace(e.Record.GetString("code"))

		derived := code == ""
		if derived {
			code = Slugify(name)
		}
		if code == "" {
			return apis.NewBadRequestError(fmt.Sprintf(
				"No organization code could be derived from the name %q. Set one explicitly: lowercase letters, digits and hyphens, at least two characters.", name), nil)
		}

		existing, _ := e.App.FindFirstRecordByFilter(
			orgCollection, "code = {:code}", map[string]interface{}{"code": code})
		if existing != nil {
			if derived {
				return apis.NewBadRequestError(fmt.Sprintf(
					"The code %q, derived from the name %q, is already used by %q. Set an explicit code.",
					code, name, existing.GetString("name")), nil)
			}
			return apis.NewBadRequestError(fmt.Sprintf(
				"The code %q is already used by %q.", code, existing.GetString("name")), nil)
		}

		e.Record.Set("code", code)
		return e.Next()
	})
}
