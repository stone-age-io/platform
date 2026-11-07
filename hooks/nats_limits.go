package hooks

import (
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// NATS resource limits: an unset limit is -1, and 0 is refused. The long form
// is the limits bullet in AUTHORIZATION.md.
//
// In nats-server.conf 0 means "server default"; in a JWT 0 means ZERO, and
// pb-nats copies the record value straight in (limitValue in
// internal/jwt/generator.go maps only negatives to NoLimit). PocketBase number
// fields have no default, so a role created without its limits signed a hard
// zero. In nats-server v2.15, asserted in nats_limits_test.go:
//
//   - Subs 0: the connection is closed at CONNECT, before it can subscribe or
//     publish.
//   - Payload 0: the first non-empty publish closes the connection.
//   - Data 0: nothing. nats-server never reads Limits.Data.
//
// So 0 only yields a credential that does not work. "Publish-only" is an empty
// subscribe allow list, which signs deny ">" since pb-nats v0.3.0.
//
// Two layers, because only a request can tell "omitted" from "0":
//
//   - MODEL, nats_roles, create and update: a 0 is stored as -1, for every
//     path. On a role -1 means "the account limit applies", since nats-server
//     applies the lower of the two, so this cannot widen a tenant's envelope.
//   - REQUEST, nats_roles and nats_accounts: an explicit 0 is refused with a
//     field error. Rewriting it to -1 would grant what the writer meant to
//     withhold. On a role, null and "" mean unset, judged from the raw body
//     (rawRequestBody).
//
// Accounts get only the request layer, and a blank counts as 0: an account's
// -1 really is unlimited, so a model rewrite would widen what the tenant was
// sold. JetStream limits are excluded (0 there is JetStream off); the config
// defaults are checked at startup (OrgProvisioningOptions.Validate).
//
// The request check is EDGE-keyed: it refuses a 0 only where the stored value
// is not already 0, so a 0 already stored does not block every later save of a
// form that resubmits all the fields. Original() is reliable here because a
// request loads the row first.

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
// e.Record already holds the value the write would store.
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
// RequestInfo().Body holds PREPARED values (apis/record_crud.go,
// ReplaceModifiers), where a number field's null and "" are already 0. The
// router wraps every body, batch sub-requests included, in a
// RereadableReadCloser, so a second bind reads the same bytes.
func rawRequestBody(e *core.RecordRequestEvent) (map[string]any, error) {
	raw := map[string]any{}
	if err := e.BindBody(&raw); err != nil {
		return nil, apis.NewBadRequestError("Failed to read the submitted data.", err)
	}
	return raw, nil
}

// leftUnset reports whether the request left a limit unset: did not send it,
// or sent null or "". A modifier key such as `max_subscriptions-` is explicit,
// so decrementing a limit to exactly 0 is refused.
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
