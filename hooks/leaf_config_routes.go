package hooks

import (
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// operatorCollection is the (singleton) collection holding the NATS operator.
// It stays superuser-only at the collection level; the operator JWT is exposed
// solely through the route below.
const operatorCollection = "nats_system_operator"

// LeafConfigRoutesOptions names the collections the route reads — with the app's
// own privileges, deliberately bypassing the API rules — plus the two
// deployment facts an edge box cannot derive for itself.
type LeafConfigRoutesOptions struct {
	ThingCollection       string
	NatsUserCollection    string
	NatsAccountCollection string

	// HubLeafURL is nats.leaf_url from config.yaml: where an edge's leaf remote
	// dials this deployment's hub. HubDomain is nats.jetstream_domain: the hub's
	// own JetStream domain, which an edge needs in order to address the hub
	// across the leaf link.
	//
	// Both are deliberately separate from nats.server_url, and neither is
	// derived from it. That address is what THIS process dials to publish
	// account claims; it is a different port, and behind a proxy or inside a
	// container a different hostname. A Control Plane publishing to
	// nats://nats:4222 says nothing about what an edge box across a WAN can
	// reach. Same reasoning as nats.websocket_urls — see
	// RegisterClientConfigRoutes.
	HubLeafURL string
	HubDomain  string
}

// RegisterLeafConfigRoutes serves everything a Thing needs to stand up a NATS
// leaf server: GET /api/me/leaf-config, plus GET /api/me/account-jwt, which a
// running gateway polls to learn when that config's account JWT has changed.
//
// WHY THIS IS A ROUTE. Exactly one value here is otherwise unreachable — the
// operator JWT, which lives in a superuser-only collection. A Thing can already
// read its own nats_user.creds_file and its own nebula_host.config_yaml through
// rules that ship today, which is why the Nebula side of a gateway needs no
// route at all. This one exists to get one public field out of
// nats_system_operator, and it carries the rest along because the caller needs
// them in the same breath.
//
// Everything served is either public trust material — operator and account JWTs
// are verified by every server in the network — or the caller's own credential,
// which it must hold to connect at all. Account *seeds* and signing keys are
// never exposed: the handler reads named fields, never whole records. That is
// also why the route needs no marker, flag or capability check: there is
// nothing here to gate. A Thing that will never run a leaf node can ask, and
// learns nothing it could not already learn.
//
// WHY /api/me. It takes no record id. The target is derived entirely from the
// caller's own authenticated record, so there is no parameter to aim at someone
// else's Thing — the same shape as POST /api/me/nats-creds/rotate.
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

			// The JetStream domain IS the Thing's code — computed here, stored
			// nowhere. A stored domain would be a second name for an identifier
			// the Thing already has, free to drift from it, and unique only if
			// whoever typed it got it right. `code` is immutable and unique
			// within the organization, and a domain only has to be unique
			// within the account: $JS.<domain>.API lives in account-scoped
			// subject space, so two tenants using the same domain never meet.
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
			// Defense in depth: the relation is server-provisioned, but never
			// serve an account from outside the caller's own organization.
			if account.GetString("organization") != thing.GetString("organization") {
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
		// refresh. A site's leaf checks credentials against its own preloaded
		// copy of the account JWT, and revocations live in that JWT, so the
		// agent re-checks it every few minutes. Asking leaf-config each time
		// would carry the Thing's nkey seed over the wire on every check; this
		// returns only the account JWT and its public key -- public trust
		// material -- and the agent fetches leaf-config only when the JWT it
		// gets here differs from the one it last loaded. Comparing the JWT
		// itself, not a timestamp, means there is no revision to keep in step.
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
// lookup, with the same checks, as the leaf-config handler above, so the probe
// can never name a different account than the config it stands in for. Change
// the two together.
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
	// Defense in depth: the relation is server-provisioned, but never
	// serve an account from outside the caller's own organization.
	if account.GetString("organization") != thing.GetString("organization") {
		return nil, nil, re.NotFoundError("nats_account not found", nil)
	}
	if account.GetString("jwt") == "" || account.GetString("public_key") == "" {
		return nil, nil, re.NotFoundError("nats_account missing jwt/public_key", nil)
	}
	return natsUser, account, nil
}

// accountJWTResponse names the probe's fields on the wire. Like
// leafConfigResponse, the keys are a cross-repo contract with the agent, and
// they are the same two names leaf-config uses for the same values.
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

// leafConfigResponse names the fields on the wire. It is split out from the
// handler because these keys are a CROSS-REPO CONTRACT: the agent decodes them
// by name, in a different module, and nothing else would notice a rename. The
// route this one replaces had its field list pinned only by a fixture inside the
// consumer, so a server-side rename would have shipped green.
//
// `domain` is deliberately not a separate input. It IS the code — see the
// handler — and returning it as its own key is a convenience for the agent,
// which writes it into two different directives of nats-leaf.conf.
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
// returns "" when both are set.
//
// The route refuses rather than serve an empty value. The agent cannot use a
// leaf config without either one, and rejects it, but all it can report is a
// field name -- and whoever is at the edge box may not be the one who can
// change this platform's config.
func hubNotSetUp(opts LeafConfigRoutesOptions) string {
	if opts.HubLeafURL == "" {
		return "nats.leaf_url (STONE_AGE_NATS_LEAF_URL)"
	}
	if opts.HubDomain == "" {
		return "nats.jetstream_domain (STONE_AGE_NATS_JETSTREAM_DOMAIN)"
	}
	return ""
}

// requireThing re-checks the caller's collection. RequireAuth already enforces
// it; this keeps the handler correct on its own terms, so a future change to the
// route's Bind cannot silently widen who reaches the body.
func requireThing(re *core.RequestEvent, collection string) error {
	if re.Auth == nil || re.Auth.Collection().Name != collection {
		return re.UnauthorizedError("thing authentication required", nil)
	}
	return nil
}

// systemAccountName is how pb-nats names the $SYS account record when it seeds
// the operator. Matching on the name is how pb-nats finds it too (its
// getOperatorAndSystemAccount), so the two stay in step by using the same key.
const systemAccountName = "System Account"

// systemAccount returns the $SYS account's JWT and public key.
//
// An edge needs these even though it gets no $SYS identity. The operator JWT
// names a system account, and a leaf running `resolver: MEMORY` has nowhere to
// fetch that account from — so without it preloaded, nats-server fails while
// building the server with "error resolving system account: account missing"
// and the edge never starts. Returning it here is what makes the generated
// nats-leaf.conf loadable at all.
//
// It is an account JWT: public trust material, the same class as the operator
// and org account JWTs already returned. It confers nothing on its own —
// connecting to $SYS requires a $SYS *user* credential, which is never served.
// Seeds and signing keys remain unreachable.
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

// operatorJWT returns the platform's operator JWT — a public trust anchor every
// NATS server in the network validates against.
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
