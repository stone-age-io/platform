package hooks

import (
	"net/http"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// operatorCollection is the (singleton) collection holding the NATS operator.
// It stays superuser-only at the collection level; the operator JWT is exposed
// solely through the route below.
const operatorCollection = "nats_system_operator"

// LeafConfigRoutesOptions names the collections the route reads, with the app's
// own privileges (bypassing the API rules), plus the two deployment facts an
// edge box cannot derive for itself.
type LeafConfigRoutesOptions struct {
	ThingCollection       string
	NatsUserCollection    string
	NatsAccountCollection string

	// HubLeafURL is nats.leaf_url: where an edge's leaf remote dials the hub.
	// HubDomain is nats.jetstream_domain: the hub's JetStream domain, which an
	// edge needs to address the hub across the leaf link. Neither is derived
	// from nats.server_url, the address this process dials (see
	// RegisterClientConfigRoutes).
	HubLeafURL string
	HubDomain  string
}

// RegisterLeafConfigRoutes serves everything a Thing needs to stand up a NATS
// leaf server: GET /api/me/leaf-config, plus GET /api/me/account-jwt, which a
// running gateway polls to learn when that config's account JWT has changed.
//
// A route because one value is otherwise unreachable: the operator JWT, in a
// superuser-only collection. A Thing can already read its own creds_file and
// Nebula config through the rules; the rest rides along for convenience.
//
// It gates on nothing beyond Thing auth, because there is nothing to gate.
// Everything served is public trust material (operator and account JWTs, which
// every server verifies) or the caller's own credential. Seeds and signing keys
// are never exposed: the handler reads named fields, never whole records. A
// Thing that never runs a leaf node learns nothing new by asking.
//
// Under /api/me because it takes no record id: the target is derived from the
// caller's own record, so it cannot be aimed at another Thing.
func RegisterLeafConfigRoutes(app *pocketbase.PocketBase, opts LeafConfigRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/me/leaf-config", func(re *core.RequestEvent) error {
			if err := requireThing(re, opts.ThingCollection); err != nil {
				return err
			}
			if setting := hubNotSetUp(opts); setting != "" {
				return re.Error(http.StatusServiceUnavailable,
					"edge sites are not set up on this platform: "+setting+" is empty", nil)
			}
			thing := re.Auth // no id parameter to aim elsewhere

			// The JetStream domain IS the Thing's code, computed and never
			// stored, so it cannot drift. `code` is immutable and unique in the
			// organization, and a domain need only be unique in the account
			// ($JS.<domain>.API is account-scoped).
			code := thing.GetString("code")
			if code == "" {
				return re.BadRequestError("this thing has no code; a leaf node's JetStream domain is its code", nil)
			}

			natsUserID := thing.GetString("nats_user")
			if natsUserID == "" {
				return re.NotFoundError("no NATS identity assigned to this thing yet", nil)
			}
			natsUser, err := re.App.FindRecordById(opts.NatsUserCollection, natsUserID)
			if err != nil {
				return re.NotFoundError("linked NATS identity not found", nil)
			}
			creds := natsUser.GetString("creds_file")
			if creds == "" {
				return re.NotFoundError("linked NATS identity has no creds_file (provisioning incomplete)", nil)
			}

			account, err := re.App.FindRecordById(opts.NatsAccountCollection, natsUser.GetString("account_id"))
			if err != nil {
				return re.NotFoundError("nats_account not found", nil)
			}
			// Defense in depth: never serve an account from another organization.
			// Blank is refused first, since blank equals blank.
			if thing.GetString("organization") == "" || account.GetString("organization") != thing.GetString("organization") {
				return re.NotFoundError("nats_account not found", nil)
			}
			accountJWT := account.GetString("jwt")
			accountPub := account.GetString("public_key")
			if accountJWT == "" || accountPub == "" {
				return re.NotFoundError("nats_account missing jwt/public_key", nil)
			}

			opJWT, err := operatorJWT(re)
			if err != nil {
				return err
			}
			sysJWT, sysPub, err := systemAccount(re, opts.NatsAccountCollection)
			if err != nil {
				return err
			}

			return re.JSON(200, leafConfigResponse(leafConfig{
				Code:          code,
				Creds:         creds,
				AccountJWT:    accountJWT,
				AccountPub:    accountPub,
				OperatorJWT:   opJWT,
				SysAccountJWT: sysJWT,
				SysAccountPub: sysPub,
				HubLeafURL:    opts.HubLeafURL,
				HubDomain:     opts.HubDomain,
			}))
		}).Bind(apis.RequireAuth(opts.ThingCollection))

		// GET /api/me/account-jwt is the cheap half of the gateway's leaf
		// refresh. A leaf checks credentials against its preloaded account JWT,
		// where revocations live, so the agent polls this every few minutes and
		// fetches leaf-config (which carries the nkey seed) only when the JWT
		// differs. Comparing the JWT itself means no revision to keep in step.
		se.Router.GET("/api/me/account-jwt", func(re *core.RequestEvent) error {
			if err := requireThing(re, opts.ThingCollection); err != nil {
				return err
			}
			_, account, err := callerAccount(re, opts)
			if err != nil {
				return err
			}
			return re.JSON(200, accountJWTResponse(account.GetString("jwt"), account.GetString("public_key")))
		}).Bind(apis.RequireAuth(opts.ThingCollection))

		return se.Next()
	})
}

// callerAccount returns the calling Thing's own NATS identity and the account
// that identity belongs to, read with the app's privileges. It makes the same
// lookup and checks as the leaf-config handler, so the probe never names a
// different account. Change the two together.
func callerAccount(re *core.RequestEvent, opts LeafConfigRoutesOptions) (natsUser, account *core.Record, err error) {
	thing := re.Auth // no id parameter to aim elsewhere

	natsUserID := thing.GetString("nats_user")
	if natsUserID == "" {
		return nil, nil, re.NotFoundError("no NATS identity assigned to this thing yet", nil)
	}
	natsUser, err = re.App.FindRecordById(opts.NatsUserCollection, natsUserID)
	if err != nil {
		return nil, nil, re.NotFoundError("linked NATS identity not found", nil)
	}

	account, err = re.App.FindRecordById(opts.NatsAccountCollection, natsUser.GetString("account_id"))
	if err != nil {
		return nil, nil, re.NotFoundError("nats_account not found", nil)
	}
	// Defense in depth: never serve an account from another organization.
	// Blank is refused first, since blank equals blank.
	if thing.GetString("organization") == "" || account.GetString("organization") != thing.GetString("organization") {
		return nil, nil, re.NotFoundError("nats_account not found", nil)
	}
	if account.GetString("jwt") == "" || account.GetString("public_key") == "" {
		return nil, nil, re.NotFoundError("nats_account missing jwt/public_key", nil)
	}
	return natsUser, account, nil
}

// accountJWTResponse names the probe's fields on the wire: a cross-repo
// contract with the agent, using the same names as leaf-config.
func accountJWTResponse(jwt, pub string) map[string]string {
	return map[string]string{
		"account_jwt": jwt,
		"account_pub": pub,
	}
}

// leafConfig is the material a gateway needs, before it is named for the wire.
type leafConfig struct {
	Code          string
	Creds         string
	AccountJWT    string
	AccountPub    string
	OperatorJWT   string
	SysAccountJWT string
	SysAccountPub string
	HubLeafURL    string
	HubDomain     string
}

// leafConfigResponse names the fields on the wire. Split out because these keys
// are a cross-repo contract: the agent decodes them by name in another module,
// and nothing here would notice a rename.
//
// `domain` is not a separate input: it IS the code, served as its own key for
// the agent, which writes it into two directives of nats-leaf.conf.
func leafConfigResponse(c leafConfig) map[string]string {
	return map[string]string{
		"code":            c.Code,
		"domain":          c.Code,
		"creds":           c.Creds,
		"account_jwt":     c.AccountJWT,
		"account_pub":     c.AccountPub,
		"operator_jwt":    c.OperatorJWT,
		"sys_account_jwt": c.SysAccountJWT,
		"sys_account_pub": c.SysAccountPub,
		"hub_leaf_url":    c.HubLeafURL,
		"hub_domain":      c.HubDomain,
	}
}

// hubNotSetUp names the platform setting that keeps edge sites from working, or
// returns "" when both are set. The route refuses rather than serve an empty
// value, naming the setting for whoever can change this platform's config.
func hubNotSetUp(opts LeafConfigRoutesOptions) string {
	if opts.HubLeafURL == "" {
		return "nats.leaf_url (STONE_AGE_NATS_LEAF_URL)"
	}
	if opts.HubDomain == "" {
		return "nats.jetstream_domain (STONE_AGE_NATS_JETSTREAM_DOMAIN)"
	}
	return ""
}

// requireThing re-checks the caller's collection, so a change to the route's
// RequireAuth cannot silently widen who reaches the body.
func requireThing(re *core.RequestEvent, collection string) error {
	if re.Auth == nil || re.Auth.Collection().Name != collection {
		return re.UnauthorizedError("thing authentication required", nil)
	}
	return nil
}

// systemAccountName is how pb-nats names the $SYS account record, and how its
// getOperatorAndSystemAccount finds it. Keep the two in step.
const systemAccountName = "System Account"

// systemAccount returns the $SYS account's JWT and public key.
//
// An edge gets no $SYS identity but needs these: the operator JWT names a system
// account, and a `resolver: MEMORY` leaf cannot fetch it, so without it
// nats-server fails with "error resolving system account: account missing".
// It is public trust material; connecting to $SYS needs a $SYS user credential,
// which is never served.
func systemAccount(re *core.RequestEvent, accountCollection string) (jwt string, pub string, err error) {
	rec, err := re.App.FindFirstRecordByFilter(
		accountCollection,
		"name = {:name}",
		dbx.Params{"name": systemAccountName},
	)
	if err != nil {
		return "", "", re.NotFoundError("system account not found", nil)
	}
	jwt = rec.GetString("jwt")
	pub = rec.GetString("public_key")
	if jwt == "" || pub == "" {
		return "", "", re.NotFoundError("system account missing jwt/public_key", nil)
	}
	return jwt, pub, nil
}

// operatorJWT returns the platform's operator JWT, a public trust anchor.
func operatorJWT(re *core.RequestEvent) (string, error) {
	op, err := re.App.FindFirstRecordByFilter(operatorCollection, "1=1")
	if err != nil {
		return "", re.NotFoundError("operator not found", nil)
	}
	jwt := op.GetString("jwt")
	if jwt == "" {
		return "", re.NotFoundError("operator JWT not available", nil)
	}
	return jwt, nil
}
