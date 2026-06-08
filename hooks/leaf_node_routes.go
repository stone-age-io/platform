package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// operatorCollection is the (singleton) collection holding the NATS operator.
// It stays superuser-only at the collection level; the operator JWT is exposed
// solely through the route below.
const operatorCollection = "nats_system_operator"

// RegisterLeafNodeRoutes adds a narrow, leaf-node-authenticated route that returns
// ONLY the operator JWT.
//
// The operator JWT is a public trust anchor that a leaf node needs to bootstrap
// its NATS server config (operator + MEMORY resolver_preload). Rather than open a
// read rule on the crown-jewel `nats_system_operator` collection, we expose just
// this one value through a dedicated route gated to `leaf_nodes` auth identities.
func RegisterLeafNodeRoutes(app *pocketbase.PocketBase) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/leaf/operator-jwt", func(re *core.RequestEvent) error {
			// Defense-in-depth: RequireAuth("leaf_nodes") already enforces this,
			// but re-check the collection so the handler is safe regardless.
			if re.Auth == nil || re.Auth.Collection().Name != "leaf_nodes" {
				return re.UnauthorizedError("leaf node authentication required", nil)
			}

			op, err := re.App.FindFirstRecordByFilter(operatorCollection, "1=1")
			if err != nil {
				return re.NotFoundError("operator not found", nil)
			}

			jwt := op.GetString("jwt")
			if jwt == "" {
				return re.NotFoundError("operator JWT not available", nil)
			}

			return re.JSON(200, map[string]string{"operator_jwt": jwt})
		}).Bind(apis.RequireAuth("leaf_nodes"))

		return se.Next()
	})
}
