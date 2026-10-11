package hooks

import (
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// NATS resource limits: an unset limit is -1, and 0 is refused.
//
// THE TRAP. Two conventions meet here and they disagree about 0. In a
// nats-server config file 0 means "not set": the server substitutes its own
// default, which for max_subscriptions is unlimited (opts.go,
// setBaselineOptions; client.go, applyAccountLimits). In a JWT, -1 is unlimited
// and 0 means ZERO, and pb-nats copies the record value straight into the JWT:
// limitValue in internal/jwt/generator.go maps only negatives to NoLimit,
// deliberately and documented. PocketBase number fields have no default, so a
// nats_roles record created without the three limit fields -- through the
// record API, `stone nats-role create`, a `stone apply` workspace that never
// mentions them -- stored 0 and was signed into every user JWT as a hard limit
// of zero. The console form avoided it only because it pre-fills -1.
//
// What zero does in nats-server v2.15, asserted against a real server in
// nats_limits_test.go rather than inferred:
//
//   - Subs 0: the server checks the limit at CONNECT, finds 0 subscriptions
//     already at a limit of 0, and closes the connection 20ms later with
//     "maximum subscriptions exceeded". Before the client subscribes to
//     anything, so before it can publish either. The browser reconnected every
//     ~3s forever, and the only trace was a server log line.
//   - Payload 0: the first non-empty publish closes the connection.
//   - Data 0: nothing at all. nats-server never reads Limits.Data, user or
//     account, so 0 would claim a restriction that no server applies.
//
// So 0 is not "publish-only". Its only effect is a credential that does not
// work, and the intent it might have been written for is already expressible
// correctly: an empty subscribe (or publish) allow list signs deny ">" in that
// direction, since pb-nats v0.3.0. That is why 0 is refused rather than kept.
//
// TWO LAYERS, because only one of them can tell "omitted" from "0":
//
//   - MODEL, nats_roles only, create and update: a 0 limit is stored as -1. An
//     omitted field and a 0 are the same value by the time a model hook sees
//     the record, so this is the default every path gets: the record API,
//     app.Save, demoseed, the tests. On a ROLE, -1 does not mean unlimited:
//     nats-server applies the lower of the user limit and the account limit, so
//     -1 means "the account limit applies", and the account limits are
//     operator-only. Nothing this hook writes can widen a tenant's envelope.
//   - REQUEST, nats_roles and nats_accounts, create and update: a write that
//     explicitly asks for 0 is refused, with a field error saying what 0 would
//     do. Rewriting a deliberate 0 to -1 would quietly grant the thing the
//     writer meant to withhold -- the "blank scope is a wildcard" bug in
//     reverse. On a role, null and "" are not explicit: they mean unset, like
//     an omitted field, and the model layer stores -1. That is judged from the
//     raw body, because RequestInfo() has already turned them into 0
//     (rawRequestBody).
//
// Accounts get only the request layer, and their blank counts as a 0. Both of
// their creators set every limit (org provisioning from config, pb-nats's system
// account at -1), the API lets only a superuser create one, and an ACCOUNT's -1
// really is unlimited, so a model-level rewrite would widen what the tenant was
// sold.
// The JetStream limits are excluded: 0 there is a real NATS setting, JetStream
// off. The config defaults are checked at startup instead
// (OrgProvisioningOptions.Validate), where an operator who knows nats-server.conf
// is most likely to write 0 meaning "use the default".
//
// The request check is EDGE-keyed: it refuses a 0 only where the stored value is
// not already 0. Original() is reliable here because a request always loads the
// row first (hooks/CLAUDE.md). Without that, an account an operator had already
// set to 0 -- a crude suspend from before organizations.active did anything --
// would refuse every later save of the console limits form, which resubmits all
// six fields.

// natsLimitError is a field error PocketBase renders as {code, message}: it
// satisfies tools/router.SafeErrorItem.
type natsLimitError string

func (natsLimitError) Code() string    { return "validation_nats_limit_zero" }
func (e natsLimitError) Error() string { return string(e) }

type natsLimitField struct {
	name string
	// zero is what 0 would do, and what to write instead.
	zero string
}

const natsDataZero = "NATS does not enforce a data limit, so 0 would claim a restriction that nothing applies. Use -1"

var natsRoleLimitFields = []natsLimitField{
	{"max_subscriptions", "NATS reads 0 as zero subscriptions, and a credential with none is disconnected as soon as it connects, so it cannot publish either. " +
		"Use -1 for no per-user limit (the account limit still applies). To stop a role subscribing, leave its subscribe allow list empty"},
	{"max_data", natsDataZero},
	{"max_payload", "NATS reads 0 as a zero-byte limit and disconnects the client on its first non-empty message. " +
		"Use -1 for no per-user limit (the account limit still applies). To stop a role publishing, leave its publish allow list empty"},
}

var natsAccountLimitFields = []natsLimitField{
	{"max_connections", "0 would refuse every connection in this account. Use -1 for unlimited. " +
		"To take an organization off NATS, suspend it instead (organizations.active)"},
	{"max_subscriptions", "NATS reads 0 as zero subscriptions, so every client in this account would be disconnected as soon as it connects. Use -1 for unlimited"},
	{"max_data", natsDataZero},
	{"max_payload", "NATS reads 0 as a zero-byte limit, so every client in this account would be disconnected on its first non-empty message. Use -1 for unlimited"},
}

// NatsLimitsOptions names the two collections whose limits are signed into JWTs.
type NatsLimitsOptions struct {
	RoleCollection    string
	AccountCollection string
}

// RegisterNatsLimits binds both layers described at the top of this file.
func RegisterNatsLimits(app *pocketbase.PocketBase, opts NatsLimitsOptions) {
	defaultUnset := func(e *core.RecordEvent) error {
		for _, f := range natsRoleLimitFields {
			if e.Record.GetInt(f.name) == 0 {
				e.Record.Set(f.name, -1)
			}
		}
		return e.Next()
	}
	app.OnRecordCreate(opts.RoleCollection).BindFunc(defaultUnset)
	app.OnRecordUpdate(opts.RoleCollection).BindFunc(defaultUnset)

	for _, set := range []struct {
		collection string
		fields     []natsLimitField
		// blankIsUnset: null or "" means unset, which the model layer turns
		// into -1. False where no model layer exists, so a blank is the 0 it
		// would be stored as.
		blankIsUnset bool
	}{
		{opts.RoleCollection, natsRoleLimitFields, true},
		{opts.AccountCollection, natsAccountLimitFields, false},
	} {
		refuse := func(e *core.RecordRequestEvent) error {
			if err := refuseZeroLimits(e, set.fields, set.blankIsUnset); err != nil {
				return err
			}
			return e.Next()
		}
		app.OnRecordCreateRequest(set.collection).BindFunc(refuse)
		app.OnRecordUpdateRequest(set.collection).BindFunc(refuse)
	}
}

// refuseZeroLimits returns a 400 naming every limit this request would set to 0.
// By the time a request hook runs, PocketBase has loaded the body into e.Record,
// so the record holds the value the write would store.
func refuseZeroLimits(e *core.RecordRequestEvent, fields []natsLimitField, blankIsUnset bool) error {
	var raw map[string]any // read only when a 0 needs telling apart from a blank
	errs := map[string]error{}
	var names []string
	for _, f := range fields {
		if e.Record.GetInt(f.name) != 0 {
			continue
		}
		if !e.Record.IsNew() && e.Record.Original().GetInt(f.name) == 0 {
			continue // already 0 in the database: not this write's doing
		}
		if blankIsUnset {
			if raw == nil {
				var err error
				if raw, err = rawRequestBody(e); err != nil {
					return err
				}
			}
			if leftUnset(raw, f.name) {
				continue
			}
		}
		errs[f.name] = natsLimitError(f.zero)
		names = append(names, f.name)
	}
	if len(errs) == 0 {
		return nil
	}
	return apis.NewBadRequestError(
		"A NATS limit cannot be 0 ("+strings.Join(names, ", ")+"). Use -1 for no limit. Each field error says what 0 would do.",
		errs)
}

// rawRequestBody binds the request body again, as the client sent it.
//
// RequestInfo().Body is no use for this. The record create and update handlers
// overwrite it with Record.ReplaceModifiers(body) before any request hook runs
// (apis/record_crud.go), and ReplaceModifiers stores each field's PREPARED value:
// a number field's null and "" are already 0 there, indistinguishable from an
// explicit 0. The router wraps every request body, batch sub-requests included,
// in a RereadableReadCloser, so binding it a second time with PocketBase's own
// binder reads the same bytes the first bind did.
func rawRequestBody(e *core.RecordRequestEvent) (map[string]any, error) {
	raw := map[string]any{}
	if err := e.BindBody(&raw); err != nil {
		return nil, apis.NewBadRequestError("Failed to read the submitted data.", err)
	}
	return raw, nil
}

// leftUnset reports whether the request left a limit unset: did not send it,
// or sent null or "" (what a blank form field sends). A modifier key such as
// `max_subscriptions-` is an explicit value, so decrementing a limit to exactly
// 0 is refused rather than quietly read as "unset".
func leftUnset(raw map[string]any, name string) bool {
	if _, ok := raw[name+"+"]; ok {
		return false
	}
	if _, ok := raw[name+"-"]; ok {
		return false
	}
	v, ok := raw[name]
	if !ok || v == nil {
		return true
	}
	s, isString := v.(string)
	return isString && strings.TrimSpace(s) == ""
}
