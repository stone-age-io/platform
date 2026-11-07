package hooks

import (
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// ClientConfigRoutesOptions carries the deployment-level values the browser
// cannot derive for itself.
type ClientConfigRoutesOptions struct {
	// NatsWebsocketURLs is nats.websocket_urls: the WebSocket listeners a
	// browser dials. Never derive it from nats.server_url, the TCP address this
	// process dials, which is often a different port and host.
	NatsWebsocketURLs []string
}

// RegisterClientConfigRoutes serves the deployment facts the SPA needs at
// runtime but cannot be compiled with.
//
// A route, not a build-time constant: the UI is embedded in the binary, so a
// baked-in hostname would mean a frontend build per operator (the problem the
// branding overlay also avoids).
//
// It requires auth because nothing needs it before login, so there is no
// reason to hand a scanner the bus address.
//
// Deployment-wide, not per-org: every organization is an account on the same
// cluster. What varies is hub versus leaf, a property of the browser's box,
// answered by a device-level override in localStorage.
//
// An empty list means "not configured" and the client falls back to its
// localhost default; it is not an error.
func RegisterClientConfigRoutes(app *pocketbase.PocketBase, opts ClientConfigRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/client-config", func(re *core.RequestEvent) error {
			urls := opts.NatsWebsocketURLs
			if urls == nil {
				// Encode as [] rather than null.
				urls = []string{}
			}
			return re.JSON(200, map[string]any{
				"natsWebsocketUrls": urls,
			})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}
