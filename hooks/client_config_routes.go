package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// ClientConfigRoutesOptions carries the deployment-level values the browser
// cannot derive for itself.
type ClientConfigRoutesOptions struct {
	// NatsWebsocketURLs is nats.websocket_urls from config.yaml: the WebSocket
	// listeners a BROWSER should dial. Deliberately separate from
	// nats.server_url, which is the TCP address this process dials to publish
	// account claims — the two are different ports and, behind a proxy or in a
	// container, different hostnames. A deployment that publishes to
	// nats://nats:4222 tells you nothing about what a browser can reach.
	NatsWebsocketURLs []string
}

// RegisterClientConfigRoutes serves the handful of deployment facts the SPA
// needs at runtime but cannot be compiled with.
//
// WHY A ROUTE AND NOT A BUILD-TIME CONSTANT. The product ships as one binary
// with the UI embedded in it. Baking the bus hostname in at `npm run build`
// would mean every operator needs their own frontend build to change a
// hostname — the same problem the branding overlay (main.go, /branding/*)
// exists to avoid. Config file in, JSON out, no rebuild.
//
// WHY IT REQUIRES AUTH. Unlike branding, nothing needs this before login: the
// UI's connect() already requires an authenticated session and a membership
// with a linked nats_user before it dials anything. Since there is no
// pre-login need, there is no reason to hand an unauthenticated scanner the
// address of the bus. It is not a secret — the credential is, and that is
// row-scoped in nats_users — but free is free.
//
// The response is deployment-wide, not per-org, and that follows from the NATS
// hierarchy rather than being a simplification: every organization is an
// account under one operator, so every organization lives on the same cluster.
// There is no per-org server URL to return. The axis that genuinely varies is
// location — hub versus a specific leaf node — which is a property of the box
// the browser runs on, so it is answered by a device-level override in
// localStorage rather than by anything the server knows.
//
// An empty list is a valid answer and means "not configured": the client falls
// back to its compiled-in localhost default. Failing here would be wrong —
// a deployment that never sets the key is the stock dev deployment.
func RegisterClientConfigRoutes(app *pocketbase.PocketBase, opts ClientConfigRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/client-config", func(re *core.RequestEvent) error {
			urls := opts.NatsWebsocketURLs
			if urls == nil {
				// Encode as [] rather than null; the client treats the field as
				// a list and an absent one is the same as an empty one.
				urls = []string{}
			}
			return re.JSON(200, map[string]any{
				"natsWebsocketUrls": urls,
			})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}
