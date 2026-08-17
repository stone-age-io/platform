package hooks

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// operatorCollection is the (singleton) collection holding the NATS operator.
// It stays superuser-only at the collection level; the operator JWT is exposed
// solely through the routes below.
const operatorCollection = "nats_system_operator"

// LeafNodeRoutesOptions names the collections the bootstrap route reads. They are
// read with the app's own privileges, deliberately bypassing the API rules — see
// RegisterLeafNodeRoutes.
type LeafNodeRoutesOptions struct {
	LeafNodeCollection    string
	NatsUserCollection    string
	NatsAccountCollection string
}

// RegisterLeafNodeRoutes adds the leaf-node-authenticated routes that hand an edge
// box the material it needs to stand up its NATS leaf server.
//
// WHY THESE ARE ROUTES AND NOT COLLECTION READS. An edge needs a handful of
// values it cannot derive locally: the operator JWT, its organization's account
// JWT and public key, the $SYS account's JWT and public key (which the operator
// JWT names and a MEMORY resolver cannot fetch), and its own user credentials.
// Most of those live in secret-bearing collections. Serving them through a route
// means the leaf-node identity needs no read grant on `nats_users` or
// `nats_accounts` at all, so the blast radius of a leaked edge credential is
// this fixed list and nothing else — not "every row those collections' rules
// happen to expose".
//
// It also decouples the edge agent from rule shape. `leaf-sync config` used to
// read both collections through the CRUD API, which made a correct tightening of
// an unrelated read rule capable of breaking every edge box's bootstrap.
//
// Everything served here is either public trust material (operator and account
// JWTs are verified by every server in the network) or the leaf's own credential,
// which it must hold to connect. Account *seeds* and signing keys are never
// exposed: the handler reads named fields, not whole records.
func RegisterLeafNodeRoutes(app *pocketbase.PocketBase, opts LeafNodeRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Superseded by /api/leaf/bootstrap, which returns this value among
		// others. Kept so an edge box running an older leaf-sync keeps working
		// after the server is upgraded; upgrade order should not matter.
		se.Router.GET("/api/leaf/operator-jwt", func(re *core.RequestEvent) error {
			if err := requireLeafNode(re); err != nil {
				return err
			}
			jwt, err := operatorJWT(re)
			if err != nil {
				return err
			}
			return re.JSON(200, map[string]string{"operator_jwt": jwt})
		}).Bind(apis.RequireAuth(opts.LeafNodeCollection))

		// Everything `leaf-sync config` needs, in one call.
		se.Router.GET("/api/leaf/bootstrap", func(re *core.RequestEvent) error {
			if err := requireLeafNode(re); err != nil {
				return err
			}
			leaf := re.Auth // the leaf node's own record; no id parameter to aim elsewhere

			domain := leaf.GetString("domain")
			if domain == "" {
				return re.NotFoundError("leaf node has no domain configured", nil)
			}
			natsUserID := leaf.GetString("nats_user")
			if natsUserID == "" {
				return re.NotFoundError("leaf node has no nats_user assigned yet (provisioning may have failed)", nil)
			}

			natsUser, err := re.App.FindRecordById(opts.NatsUserCollection, natsUserID)
			if err != nil {
				return re.NotFoundError("nats_user not found", nil)
			}
			creds := natsUser.GetString("creds_file")
			if creds == "" {
				return re.NotFoundError("nats_user has no creds_file (provisioning incomplete)", nil)
			}

			account, err := re.App.FindRecordById(opts.NatsAccountCollection, natsUser.GetString("account_id"))
			if err != nil {
				return re.NotFoundError("nats_account not found", nil)
			}
			// Defense in depth: the relation is server-provisioned, but never serve
			// an account from outside the leaf node's own organization.
			if account.GetString("organization") != leaf.GetString("organization") {
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

			return re.JSON(200, map[string]string{
				"domain":          domain,
				"code":            leaf.GetString("code"),
				"creds":           creds,
				"account_jwt":     accountJWT,
				"account_pub":     accountPub,
				"operator_jwt":    opJWT,
				"sys_account_jwt": sysJWT,
				"sys_account_pub": sysPub,
			})
		}).Bind(apis.RequireAuth(opts.LeafNodeCollection))

		return se.Next()
	})
}

// requireLeafNode re-checks the caller's collection. RequireAuth already enforces
// it; this keeps each handler correct on its own terms, so a future change to the
// route's Bind cannot silently widen who reaches the body.
func requireLeafNode(re *core.RequestEvent) error {
	if re.Auth == nil || re.Auth.Collection().Name != "leaf_nodes" {
		return re.UnauthorizedError("leaf node authentication required", nil)
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
// connecting to $SYS requires a $SYS *user* credential, which is never served
// to a leaf node. Seeds and signing keys remain unreachable.
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
