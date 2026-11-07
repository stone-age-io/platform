package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// Inventory events on the bus -- plans/inventory-events.md.
//
// Every create, update and delete of a Thing or Location is published, as the
// whole allowlisted record, on <prefix>.<org-code>.inventory.<kind>.<action> in
// the operator (provider) account. A per-org export/import pair
// (inventory_exports.go) maps that into each organization's own account as
// stone-age.inventory.<kind>.<action>, so the Control Plane publishes once on
// one connection and NATS does the routing.
//
// The whole record, never a diff: that is what makes no queue and no replay
// safe, since the next event about a record corrects a consumer that missed
// one. `changed` says what moved. Do not trim the payload to it.
//
// No queue: an event delivered an hour late is worse than none, because a
// consumer would act on a stale fact. Core NATS, fire and forget, and
// ReconnectBufSize(-1) so nats.go does not buffer publishes during an outage.
//
// Never fail the write: a missed event is a missed notification, a failed save
// is an outage. Every handler logs and returns e.Next().

// InventoryEventsOptions configures RegisterInventoryEvents.
type InventoryEventsOptions struct {
	// Enabled is nats.inventory_events_enabled. Off, nothing publishes and the
	// startup sweep removes every inventory export/import pair.
	Enabled bool

	OrgCollection      string
	AccountCollection  string
	UserCollection     string
	RoleCollection     string
	ExportCollection   string
	ImportCollection   string
	ThingCollection    string
	LocationCollection string

	// LocalSubject is nats.inventory_local_subject, what an organization's own
	// account sees ("stone-age.>"). Its prefix also roots the provider-side
	// subject (see inventorySubjectRoot).
	LocalSubject string

	// ServerURL is nats.server_url, which the platform-events connection dials.
	ServerURL string

	// Publish replaces the NATS connection. Tests set it to record events; nil
	// means connect at serve time with the platform-events credential.
	Publish func(subject string, data []byte) error
}

const (
	inventoryKindThing    = "thing"
	inventoryKindLocation = "location"

	inventoryActionCreated = "created"
	inventoryActionUpdated = "updated"
	inventoryActionDeleted = "deleted"
)

// inventoryFields is the payload allowlist per kind, never a record dump, so a
// credential field added later is not published.
// TestInventoryPayloadCarriesNoCredential pins it.
//
// Relation fields (type, location, parent) are published as the target's code,
// not its id: ids are for storage, codes for addressing (ADR 0002).
var inventoryFields = map[string][]string{
	inventoryKindThing:    {"code", "name", "description", "type", "location", "active", "metadata"},
	inventoryKindLocation: {"code", "name", "description", "type", "parent", "path", "coordinates", "metadata"},
}

// inventorySubjectRoot is the provider-side root for one organization:
// "<prefix>.<org-code>", where prefix is LocalSubject without its ".>".
//
// Never a bare "<org-code>": it would share the first token with the provider
// account's other traffic, so an org coded "helpdesk" would be exported every
// managed org's helpdesk.<code>.> stream.
func inventorySubjectRoot(localSubject, orgCode string) string {
	return strings.TrimSuffix(localSubject, ".>") + "." + orgCode
}

// inventorySubject is the provider-side subject one event is published on.
func inventorySubject(localSubject, orgCode, kind, action string) string {
	return inventorySubjectRoot(localSubject, orgCode) + ".inventory." + kind + "." + action
}

// inventoryChanged returns the allowlisted fields whose values differ from
// rec.Original(), with location_path added when a Thing's location moved.
//
// Original() is the state at the last database READ (hooks/CLAUDE.md), so for
// one in-memory record saved twice the second event also lists the first save's
// fields. Over-reporting is the safe direction; a test pins it.
func inventoryChanged(kind string, rec *core.Record) []string {
	orig := rec.Original()
	var changed []string
	for _, f := range inventoryFields[kind] {
		if fmt.Sprint(orig.Get(f)) != fmt.Sprint(rec.Get(f)) {
			changed = append(changed, f)
		}
	}
	if kind == inventoryKindThing && slices.Contains(changed, "location") {
		changed = append(changed, "location_path")
	}
	return changed
}

// inventoryPublisher holds the one connection, or the test's Publish func.
type inventoryPublisher struct {
	opts InventoryEventsOptions
	conn atomic.Pointer[nats.Conn]
}

var errInventoryNotConnected = errors.New("no platform-events connection")

func (p *inventoryPublisher) publish(subject string, data []byte) error {
	if p.opts.Publish != nil {
		return p.opts.Publish(subject, data)
	}
	nc := p.conn.Load()
	if nc == nil {
		return errInventoryNotConnected
	}
	return nc.Publish(subject, data)
}

// RegisterInventoryEvents wires the export/import pairs (always, so turning the
// flag off removes them) and, when enabled, the record hooks that publish and
// the platform-events connection they publish on.
//
// Bind it after RegisterOrgProvisioning: the organization hooks need the org's
// NATS account to exist.
func RegisterInventoryEvents(app *pocketbase.PocketBase, opts InventoryEventsOptions) {
	registerInventoryExports(app, opts)
	if !opts.Enabled {
		return
	}

	p := &inventoryPublisher{opts: opts}
	collections := []string{opts.ThingCollection, opts.LocationCollection}

	// Every read below goes through the root `app`, never e.App. A write made in
	// a transaction (a location move, POST /api/org/things) runs its Success
	// hooks after the commit, when every query on e.App fails.

	app.OnRecordAfterCreateSuccess(collections...).BindFunc(func(e *core.RecordEvent) error {
		p.emit(app, e.Record, inventoryActionCreated, nil)
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess(collections...).BindFunc(func(e *core.RecordEvent) error {
		kind := p.kindOf(e.Record)
		changed := inventoryChanged(kind, e.Record)
		// Only fields outside the allowlist moved: nothing to say.
		if len(changed) == 0 {
			return e.Next()
		}
		p.emit(app, e.Record, inventoryActionUpdated, changed)
		if kind == inventoryKindLocation && slices.Contains(changed, "path") {
			p.republishSubtree(app, e.Record)
		}
		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess(collections...).BindFunc(func(e *core.RecordEvent) error {
		p.emit(app, e.Record, inventoryActionDeleted, nil)
		return e.Next()
	})

	if opts.Publish != nil {
		return
	}

	// At serve, not bootstrap: every CLI command bootstraps the app, and none
	// should open a bus connection or mint a credential.
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		nc, err := connectPlatformEvents(e.App, opts)
		if err != nil {
			log.Printf("❌ Inventory events will not publish: %v", err)
		} else {
			p.conn.Store(nc)
			log.Printf("✅ Inventory events publishing on %s.*.inventory.>", strings.TrimSuffix(opts.LocalSubject, ".>"))
		}
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		if nc := p.conn.Swap(nil); nc != nil {
			nc.Close()
		}
		return e.Next()
	})
}

func (p *inventoryPublisher) kindOf(rec *core.Record) string {
	if rec.Collection().Name == p.opts.LocationCollection {
		return inventoryKindLocation
	}
	return inventoryKindThing
}

// emit builds and publishes one event. Every failure is logged and swallowed.
func (p *inventoryPublisher) emit(app core.App, rec *core.Record, action string, changed []string) {
	kind := p.kindOf(rec)

	org, err := app.FindRecordById(p.opts.OrgCollection, rec.GetString("organization"))
	if err != nil {
		return // an org-less record has no subject to publish on
	}
	orgCode := org.GetString("code")
	if orgCode == "" {
		return
	}

	payload := inventoryPayload(app, p.opts, kind, rec)
	if action == inventoryActionUpdated {
		payload["changed"] = changed
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Inventory event for %s %s not encoded: %v", kind, rec.Id, err)
		return
	}

	subject := inventorySubject(p.opts.LocalSubject, orgCode, kind, action)
	if err := p.publish(subject, data); err != nil {
		log.Printf("⚠️ Inventory event %s not published: %v", subject, err)
	}
}

// inventoryPayload is the allowlisted record, relations resolved to codes, plus
// the derived fields a consumer would otherwise need a REST round trip for.
func inventoryPayload(app core.App, opts InventoryEventsOptions, kind string, rec *core.Record) map[string]any {
	payload := map[string]any{
		"id": rec.Id,
		"ts": time.Now().UTC().Format(time.RFC3339),
	}
	for _, f := range inventoryFields[kind] {
		switch f {
		case "type":
			typeCol := "thing_types"
			if kind == inventoryKindLocation {
				typeCol = "location_types"
			}
			payload[f] = codeOf(app, typeCol, rec.GetString(f))
		case "location", "parent":
			payload[f] = codeOf(app, opts.LocationCollection, rec.GetString(f))
		default:
			payload[f] = rec.Get(f)
		}
	}

	// The path makes alerting by area one rule-router `contains` on "/HQ/"
	// instead of a KV lookup per message (ADR 0004).
	if kind == inventoryKindThing {
		path := ""
		if id := rec.GetString("location"); id != "" {
			if loc, err := app.FindRecordById(opts.LocationCollection, id); err == nil {
				path = loc.GetString("path")
			}
		}
		payload["location_path"] = path
	}
	return payload
}

// codeOf returns a related record's code, or "" when unset or gone.
func codeOf(app core.App, collection, id string) string {
	if id == "" {
		return ""
	}
	rec, err := app.FindRecordById(collection, id)
	if err != nil {
		return ""
	}
	return rec.GetString("code")
}

// republishSubtree publishes every descendant location and every Thing under the
// moved location, because the move rewrote their paths in raw SQL.
//
// hooks/location_path.go rewrites the subtree with one UPDATE, which fires no
// record hooks, so without this a Thing that never changed again would carry a
// stale location_path forever. Nothing is saved, so nothing re-enters a hook.
func (p *inventoryPublisher) republishSubtree(app core.App, moved *core.Record) {
	path := moved.GetString("path")
	if path == "" {
		return
	}
	org := moved.GetString("organization")

	// substr, never LIKE: codes may contain `_` (ADR 0004).
	descendants, err := app.FindAllRecords(p.opts.LocationCollection,
		dbx.HashExp{"organization": org},
		dbx.NewExp("substr([[path]], 1, length({:p})) = {:p} AND [[id]] != {:id}",
			dbx.Params{"p": path, "id": moved.Id}),
	)
	if err != nil {
		log.Printf("⚠️ Inventory subtree under %s not republished: %v", path, err)
		return
	}

	locationIDs := []any{moved.Id}
	for _, loc := range descendants {
		p.emit(app, loc, inventoryActionUpdated, []string{"path"})
		locationIDs = append(locationIDs, loc.Id)
	}

	things, err := app.FindAllRecords(p.opts.ThingCollection,
		dbx.HashExp{"organization": org},
		dbx.In("location", locationIDs...),
	)
	if err != nil {
		log.Printf("⚠️ Inventory Things under %s not republished: %v", path, err)
		return
	}
	for _, thing := range things {
		p.emit(app, thing, inventoryActionUpdated, []string{"location_path"})
	}
}

const platformEventsName = "platform-events"

// ensurePlatformEventsUser returns the creds of the Control Plane's own
// publishing identity in the operator account, creating it on first use.
//
// An ordinary nats_users record rather than a self-signed JWT, so it is visible
// in the console and rotates like any other. Created at serve rather than by
// `bootstrap`, so older deployments get one too. Its role is created once and
// never reconciled: after that it is the provider's to edit.
func ensurePlatformEventsUser(app core.App, opts InventoryEventsOptions) (string, error) {
	operatorOrg, hubAccount, err := operatorHub(app, opts.OrgCollection, opts.AccountCollection)
	if err != nil {
		return "", err
	}

	user, _ := app.FindFirstRecordByFilter(opts.UserCollection,
		"account_id = {:acct} && nats_username = {:u}",
		dbx.Params{"acct": hubAccount.Id, "u": platformEventsName})

	if user == nil {
		role, err := ensurePlatformEventsRole(app, opts, operatorOrg)
		if err != nil {
			return "", err
		}
		col, err := app.FindCollectionByNameOrId(opts.UserCollection)
		if err != nil {
			return "", fmt.Errorf("find NATS user collection: %w", err)
		}
		pw, err := randomSecret(32)
		if err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		u := core.NewRecord(col)
		u.Set("nats_username", platformEventsName)
		u.Set("email", fmt.Sprintf("%s@%s.nats.local", platformEventsName, operatorOrg.GetString("code")))
		u.Set("emailVisibility", true)
		u.SetPassword(pw)
		u.Set("description", "The Control Plane's own identity for publishing inventory events (plans/inventory-events.md).")
		u.Set("account_id", hubAccount.Id)
		u.Set("role_id", role.Id)
		u.Set("organization", operatorOrg.Id)
		u.Set("active", true)
		if err := app.Save(u); err != nil {
			return "", fmt.Errorf("create %s NATS user: %w", platformEventsName, err)
		}
		log.Printf("✅ Created the %s NATS user in the operator account", platformEventsName)

		// pb-nats mints keys and creds in its AfterCreateSuccess hook and saves
		// them separately, so the record in hand predates them.
		if user, err = app.FindRecordById(opts.UserCollection, u.Id); err != nil {
			return "", fmt.Errorf("reload %s NATS user: %w", platformEventsName, err)
		}
	}

	creds := user.GetString("creds_file")
	if creds == "" {
		return "", fmt.Errorf("the %s NATS user has no creds_file", platformEventsName)
	}
	return creds, nil
}

func ensurePlatformEventsRole(app core.App, opts InventoryEventsOptions, operatorOrg *core.Record) (*core.Record, error) {
	if role, _ := app.FindFirstRecordByFilter(opts.RoleCollection,
		"organization = {:org} && name = {:n}",
		dbx.Params{"org": operatorOrg.Id, "n": platformEventsName}); role != nil {
		return role, nil
	}
	col, err := app.FindCollectionByNameOrId(opts.RoleCollection)
	if err != nil {
		return nil, fmt.Errorf("find NATS role collection: %w", err)
	}
	role := core.NewRecord(col)
	role.Set("name", platformEventsName)
	role.Set("description", "Publish-only role for the Control Plane's inventory events.")
	role.Set("organization", operatorOrg.Id)
	role.Set("is_default", false)
	// An empty allow list grants nothing (pb-nats v0.3.0+): publish only.
	role.Set("publish_permissions", []string{strings.TrimSuffix(opts.LocalSubject, ".>") + ".>"})
	role.Set("subscribe_permissions", []string{})
	role.Set("publish_deny_permissions", []string{})
	role.Set("subscribe_deny_permissions", []string{})
	// An unset NATS limit is -1; 0 means none and is refused.
	role.Set("max_subscriptions", -1)
	role.Set("max_data", -1)
	role.Set("max_payload", -1)
	if err := app.Save(role); err != nil {
		return nil, fmt.Errorf("create %s NATS role: %w", platformEventsName, err)
	}
	return role, nil
}

// operatorHub returns the operator organization and its NATS account, the hub.
func operatorHub(app core.App, orgCollection, accountCollection string) (*core.Record, *core.Record, error) {
	operatorOrg, _ := app.FindFirstRecordByFilter(orgCollection, "is_operator_org = true")
	if operatorOrg == nil {
		return nil, nil, fmt.Errorf("no operator organization exists (re-run bootstrap with --operator-org)")
	}
	hubAccount, _ := app.FindFirstRecordByFilter(accountCollection, "organization = {:org}", dbx.Params{"org": operatorOrg.Id})
	if hubAccount == nil {
		return nil, nil, fmt.Errorf("operator organization has no NATS account")
	}
	return operatorOrg, hubAccount, nil
}

// connectPlatformEvents ensures the identity and opens the connection.
// RetryOnFailedConnect so a NATS server not yet up does not stop startup;
// ReconnectBufSize(-1) so a publish during an outage fails instead of arriving
// late.
func connectPlatformEvents(app core.App, opts InventoryEventsOptions) (*nats.Conn, error) {
	creds, err := ensurePlatformEventsUser(app, opts)
	if err != nil {
		return nil, err
	}
	jwt, err := nkeys.ParseDecoratedJWT([]byte(creds))
	if err != nil {
		return nil, fmt.Errorf("%s creds_file: %w", platformEventsName, err)
	}
	kp, err := nkeys.ParseDecoratedNKey([]byte(creds))
	if err != nil {
		return nil, fmt.Errorf("%s creds_file: %w", platformEventsName, err)
	}
	seed, err := kp.Seed()
	if err != nil {
		return nil, fmt.Errorf("%s creds_file: %w", platformEventsName, err)
	}
	return nats.Connect(opts.ServerURL,
		nats.UserJWTAndSeed(jwt, string(seed)),
		nats.Name("stone-age-inventory-events"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectBufSize(-1),
	)
}
