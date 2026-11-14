package hooks

import (
	"fmt"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// OrgCodePattern mirrors the `pattern` on organizations.code in schema.json;
// keep the two in step. Exported so bootstrap and the backfill migration can
// pre-flight a derived code before the import has validated anything.
var OrgCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

// OrgCodeMaxLen is the upper bound in the `code` field's pattern,
// ^[a-z0-9][a-z0-9-]{1,30}$ -- one leading alphanumeric plus up to thirty more.
const OrgCodeMaxLen = 31

// Slugify derives an organization code from a display name: lowercased, every
// run of characters outside [a-z0-9] collapsed to a single hyphen, trimmed, and
// truncated to OrgCodeMaxLen.
//
// It can return a string the pattern rejects ("" or a single character),
// deliberately: the hook refuses rather than padding, because the code is
// immutable once saved and a guess would be permanent.
//
// A leading digit is valid ("816tech"): NATS subject tokens, JetStream domains,
// KV bucket names and RFC 1123 hostname labels all permit one.
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
// Bound to OnRecordCreate, not AfterCreateSuccess, so the value is persisted
// WITH the record: the update rule freezes the code the moment it exists.
//
// One hook covers every creation path, bootstrap.go's app.Save included,
// because it is bound in main.go before app.Start() rather than inside OnServe.
//
// Deliberately no auto-suffixing: a colliding derived code is refused, as
// migrations/schema_update_unique_org_code.go does, because a silently saved
// `acme-2` is saved forever.
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

		existing, err := findFirst(e.App, orgCollection, "code = {:code}", dbx.Params{"code": code})
		if err != nil {
			return fmt.Errorf("checking organization code %q: %w", code, err)
		}
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
