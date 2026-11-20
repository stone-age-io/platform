package hooks

import (
	"errors"
	"net/http"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"
)

// KeyRoutesOptions names the collections POST /api/me/keys reads and writes.
type KeyRoutesOptions struct {
	ThingCollection      string
	NatsUserCollection   string
	NebulaHostCollection string
}

// RegisterKeyRoutes serves POST /api/me/keys: a Thing enrolls the public keys
// it made itself, for a NATS identity or Nebula host created with device_key
// (plans/device-held-keys.md).
//
// Body: {"nats_public_key": "U...", "nebula_public_key": "-----BEGIN NEBULA
// X25519 PUBLIC KEY-----..."}, both optional, at least one.
//
// The route does only what a rule cannot:
//   - find the caller's own identity, from the authenticated Thing and nothing
//     in the request, so it cannot be aimed at another Thing's;
//   - answer 200 when the key sent is the one already enrolled, so an agent
//     retrying after a lost response needs no special case;
//   - otherwise set public_key and save.
//
// Everything else is the libraries' to refuse, and it repeats none of it: the
// identity must be device_key, the key must be valid and not revoked, and an
// enrolled key cannot be replaced (409). Two copies of a check drift apart.
// Nor does it check `active`: an inactive identity stores the key and mints
// nothing until it is reactivated, and an inactive Thing cannot sign in.
//
// app.Save() bypasses the API rules, which is the point: the nats_users and
// nebula_hosts update rules are owner/admin, and a device enrolls its own key.
// The checks a rule would have made are the caller lookup above (the identity
// is the caller's own, so tenancy holds) and the libraries' invariants, which
// hold for app.Save() as for REST.
//
// No transaction across the two saves. If the second fails, the agent sends
// both again: the first is the same key, so 200, and the second is retried.
func RegisterKeyRoutes(app *pocketbase.PocketBase, opts KeyRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/me/keys", func(re *core.RequestEvent) error {
			if err := requireThing(re, opts.ThingCollection); err != nil {
				return err
			}

			var body struct {
				NatsPublicKey   string `json:"nats_public_key"`
				NebulaPublicKey string `json:"nebula_public_key"`
			}
			if err := re.BindBody(&body); err != nil {
				return re.BadRequestError("invalid body", err)
			}
			body.NatsPublicKey = strings.TrimSpace(body.NatsPublicKey)
			body.NebulaPublicKey = strings.TrimSpace(body.NebulaPublicKey)
			if body.NatsPublicKey == "" && body.NebulaPublicKey == "" {
				return re.BadRequestError("send nats_public_key, nebula_public_key, or both", nil)
			}

			enrollments := []struct {
				key, relation, collection, label string
			}{
				{body.NatsPublicKey, "nats_user", opts.NatsUserCollection, "NATS identity"},
				{body.NebulaPublicKey, "nebula_host", opts.NebulaHostCollection, "Nebula host"},
			}

			// Find every target before writing anything, so a missing one
			// refuses the request whole.
			targets := make([]*core.Record, len(enrollments))
			for i, en := range enrollments {
				if en.key == "" {
					continue
				}
				rec, err := ownIdentity(re, en.relation, en.collection, en.label)
				if err != nil {
					return err
				}
				targets[i] = rec
			}

			enrolled := map[string]string{}
			for i, en := range enrollments {
				rec := targets[i]
				if rec == nil {
					continue
				}
				// Nebula PEM is compared trimmed: a trailing newline is not a
				// different key.
				if strings.TrimSpace(rec.GetString("public_key")) != en.key {
					rec.Set("public_key", en.key)
					if err := re.App.Save(rec); err != nil {
						return enrollError(re, en.label, err)
					}
				}
				enrolled[en.relation] = rec.Id
			}

			return re.JSON(http.StatusOK, map[string]any{"enrolled": enrolled})
		}).Bind(apis.RequireAuth(opts.ThingCollection))

		return se.Next()
	})
}

// ownIdentity returns the identity the calling Thing links through relation.
// No record id comes from the request.
func ownIdentity(re *core.RequestEvent, relation, collection, label string) (*core.Record, error) {
	if collection == "" {
		return nil, re.NotFoundError(label+"s are not configured on this platform", nil)
	}
	id := re.Auth.GetString(relation)
	if id == "" {
		return nil, re.NotFoundError("no "+label+" is assigned to this thing", nil)
	}
	rec, err := re.App.FindRecordById(collection, id)
	if err != nil {
		return nil, re.NotFoundError("linked "+label+" not found", nil)
	}
	return rec, nil
}

// enrollError maps a library refusal to a status. Already enrolled with a
// different key is a 409: the agent stops and says an admin enrolled another
// key. Every other refusal is the request's fault, a 400 carrying the
// library's message, which names what was wrong.
func enrollError(re *core.RequestEvent, label string, err error) error {
	if errors.Is(err, pbnats.ErrDeviceKeyEnrolled) || errors.Is(err, pbnebula.ErrDeviceKeyEnrolled) {
		return re.Error(http.StatusConflict,
			"a different key is already enrolled for this thing's "+label, nil)
	}
	for _, refusal := range []error{
		pbnats.ErrInvalidDeviceKey, pbnats.ErrDeviceKeyRevoked, pbnats.ErrPublicKeyImmutable,
		pbnebula.ErrInvalidDeviceKey, pbnebula.ErrPublicKeyNotDeviceHeld,
	} {
		if errors.Is(err, refusal) {
			return re.BadRequestError(label+": "+refusal.Error(), nil)
		}
	}
	// A unique-index refusal, or anything else PocketBase validated: its own
	// 400 says which field.
	return re.BadRequestError("could not enroll the "+label+" key", err)
}
