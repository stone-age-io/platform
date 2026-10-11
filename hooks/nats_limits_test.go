package hooks_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"platform/hooks"
	"platform/internal/testutil"
)

// apiResponse is a decoded response body that prints as its error message and
// field errors only. A nats_accounts record carries its seeds and private keys,
// and a failing assertion should not paste key material into a CI log, test keys
// or not.
type apiResponse map[string]any

func (r apiResponse) String() string {
	if id, ok := r["id"].(string); ok {
		return "record " + id
	}
	if _, ok := r["_raw"]; ok {
		return "non-object body (a batch response), not printed"
	}
	return fmt.Sprintf("message=%q data=%v", r["message"], r["data"])
}

// recordAPI serves PocketBase's own API router in-process, so a test can drive
// the request hooks -- the only layer that can tell an omitted field from a 0 --
// without a listening server. The harness never serves (see the boundary note at
// the bottom of testutil.NewApp); this is the record API alone, which is all the
// request hooks bind to.
type recordAPI struct {
	mux http.Handler
}

func newRecordAPI(t *testing.T, app core.App) *recordAPI {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("build mux: %v", err)
	}
	return &recordAPI{mux: mux}
}

// json sends body as JSON.
func (a *recordAPI) json(t *testing.T, method, path, token string, body any) (int, apiResponse) {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	return a.do(t, method, path, token, "application/json", raw)
}

// form sends fields as multipart/form-data, the way an HTML form or FormData
// does: every value a string, a blank field an empty one.
func (a *recordAPI) form(t *testing.T, method, path, token string, fields map[string]string) (int, apiResponse) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return a.do(t, method, path, token, w.FormDataContentType(), buf.Bytes())
}

func (a *recordAPI) do(t *testing.T, method, path, token, contentType string, body []byte) (int, apiResponse) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, req)

	out := apiResponse{}
	if trimmed := bytes.TrimSpace(rec.Body.Bytes()); len(trimmed) > 0 && trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &out); err != nil {
			t.Fatalf("%s %s: decode a %d-byte response: %v", method, path, rec.Body.Len(), err)
		}
	} else if len(trimmed) > 0 {
		out["_raw"] = string(trimmed)
	}
	return rec.Code, out
}

func authToken(t *testing.T, rec *core.Record) string {
	t.Helper()
	token, err := rec.NewAuthToken()
	if err != nil {
		t.Fatalf("auth token for %s: %v", rec.Id, err)
	}
	return token
}

func reloadRecord(t *testing.T, app core.App, collection, id string) *core.Record {
	t.Helper()
	rec, err := app.FindRecordById(collection, id)
	if err != nil {
		t.Fatalf("reload %s %s: %v", collection, id, err)
	}
	return rec
}

func wantLimits(t *testing.T, rec *core.Record, want map[string]int) {
	t.Helper()
	for field, v := range want {
		if got := rec.GetInt(field); got != v {
			t.Errorf("%s %q: %s = %d, want %d", rec.Collection().Name, rec.GetString("name"), field, got, v)
		}
	}
}

func fieldErrorCode(resp apiResponse, field string) string {
	data, _ := resp["data"].(map[string]any)
	item, _ := data[field].(map[string]any)
	code, _ := item["code"].(string)
	return code
}

// newNatsUser mints a credential for role, the way the console does: pb-nats
// signs the JWT on create, from the role as it is stored.
func newNatsUser(t *testing.T, app core.App, org, account, role *core.Record, name string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("nats_users")
	if err != nil {
		t.Fatalf("nats_users collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("organization", org.Id)
	rec.Set("nats_username", name)
	rec.Set("email", name+"@limits.test")
	rec.Set("password", "probe-password-123")
	rec.Set("account_id", account.Id)
	rec.Set("role_id", role.Id)
	rec.Set("active", true)
	if err := app.Save(rec); err != nil {
		t.Fatalf("create nats user %q: %v", name, err)
	}
	return reloadRecord(t, app, "nats_users", rec.Id)
}

func userClaims(t *testing.T, user *core.Record) *jwt.UserClaims {
	t.Helper()
	uc, err := jwt.DecodeUserClaims(user.GetString("jwt"))
	if err != nil {
		t.Fatalf("decode the JWT of %q: %v", user.GetString("nats_username"), err)
	}
	return uc
}

var roleLimitFields = []string{"max_subscriptions", "max_data", "max_payload"}

func unsetLimits() map[string]int {
	return map[string]int{"max_subscriptions": -1, "max_data": -1, "max_payload": -1}
}

// One app for every case below: standing one up is most of what a hooks test
// costs (TESTING.md), and each case works in an organization of its own.
func TestNatsLimits(t *testing.T) {
	app := testutil.SetupApp(t)
	api := newRecordAPI(t, app)

	t.Run("roles", func(t *testing.T) { testRoleLimits(t, app, api) })
	t.Run("accounts", func(t *testing.T) { testAccountLimits(t, app, api) })
	t.Run("real server", func(t *testing.T) { testRoleLimitsOnARealServer(t, app, api) })
}

// A role created without its limits must sign a user JWT with no per-user limit.
// It used to store 0, which pb-nats signs as a hard limit of zero, and nats-server
// then disconnected every holder at connect (see the real-server case).
//
// Every path that leaves the fields unset is exercised, because they reach the
// hook differently: a JSON create with the keys absent (`stone nats-role
// create` without the flags), null (the console form, for a cleared input), a
// blank multipart field, the batch endpoint (`stone apply`, when batch is
// enabled), and a plain app.Save (demoseed and every test in this repo). Each
// refusal is paired with an accepted write on the same field, so a hook that
// refused everything would fail.
func testRoleLimits(t *testing.T, app *pocketbase.PocketBase, api *recordAPI) {
	org, account := newOrgWithAccount(t, app, "limits", "Limits Org", nil)
	owner := reloadRecord(t, app, "users", org.GetString("owner"))
	if owner.GetString("current_organization") != org.Id {
		t.Fatalf("owner's current_organization = %q, want the new org", owner.GetString("current_organization"))
	}
	token := authToken(t, owner)
	const roles = "/api/collections/nats_roles/records"

	roleBody := func(name string, extra map[string]any) map[string]any {
		body := map[string]any{
			"name":                  name,
			"organization":          org.Id,
			"publish_permissions":   []string{">"},
			"subscribe_permissions": []string{">"},
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	createRole := func(t *testing.T, name string, extra map[string]any) *core.Record {
		t.Helper()
		code, resp := api.json(t, http.MethodPost, roles, token, roleBody(name, extra))
		if code != http.StatusOK {
			t.Fatalf("create role %q: %d %v", name, code, resp)
		}
		return reloadRecord(t, app, "nats_roles", resp["id"].(string))
	}

	t.Run("omitted fields are stored as -1 and signed as NoLimit", func(t *testing.T) {
		role := createRole(t, "omitted", nil)
		wantLimits(t, role, unsetLimits())

		uc := userClaims(t, newNatsUser(t, app, org, account, role, "omitted-user"))
		if uc.Limits.Subs != jwt.NoLimit || uc.Limits.Payload != jwt.NoLimit || uc.Limits.Data != jwt.NoLimit {
			t.Errorf("user JWT limits = subs %d, payload %d, data %d; want all %d (NoLimit)",
				uc.Limits.Subs, uc.Limits.Payload, uc.Limits.Data, jwt.NoLimit)
		}
	})

	t.Run("null and blank are unset, not 0", func(t *testing.T) {
		wantLimits(t, createRole(t, "null", map[string]any{"max_subscriptions": nil, "max_data": nil, "max_payload": nil}), unsetLimits())
		wantLimits(t, createRole(t, "blank", map[string]any{"max_subscriptions": "", "max_data": "", "max_payload": ""}), unsetLimits())

		code, resp := api.form(t, http.MethodPost, roles, token, map[string]string{
			"name": "blank-form", "organization": org.Id,
			"max_subscriptions": "", "max_data": "", "max_payload": "",
		})
		if code != http.StatusOK {
			t.Fatalf("multipart create with blank limits: %d %v", code, resp)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", resp["id"].(string)), unsetLimits())
	})

	t.Run("an explicit limit is kept", func(t *testing.T) {
		role := createRole(t, "explicit", map[string]any{"max_subscriptions": 64, "max_data": -1, "max_payload": 4096})
		wantLimits(t, role, map[string]int{"max_subscriptions": 64, "max_data": -1, "max_payload": 4096})

		uc := userClaims(t, newNatsUser(t, app, org, account, role, "explicit-user"))
		if uc.Limits.Subs != 64 || uc.Limits.Payload != 4096 {
			t.Errorf("user JWT limits = subs %d, payload %d; want 64, 4096", uc.Limits.Subs, uc.Limits.Payload)
		}
	})

	t.Run("an explicit 0 is refused, and says why", func(t *testing.T) {
		for _, field := range roleLimitFields {
			code, resp := api.json(t, http.MethodPost, roles, token, roleBody("zero-"+field, map[string]any{field: 0}))
			if code != http.StatusBadRequest {
				t.Errorf("create with %s = 0: got %d %v, want 400", field, code, resp)
				continue
			}
			if got := fieldErrorCode(resp, field); got != "validation_nats_limit_zero" {
				t.Errorf("create with %s = 0: field error code %q, want validation_nats_limit_zero: %v", field, got, resp)
			}
			if msg, _ := resp["message"].(string); !strings.Contains(msg, field) {
				t.Errorf("the top-level message should name %s, for clients that show only it: %q", field, msg)
			}
			if n, _ := app.CountRecords("nats_roles", dbx.HashExp{"name": "zero-" + field}); n != 0 {
				t.Errorf("a refused create left %d role(s) named zero-%s", n, field)
			}
		}
		// "0" in a multipart form is just as explicit.
		code, resp := api.form(t, http.MethodPost, roles, token, map[string]string{
			"name": "zero-form", "organization": org.Id, "max_subscriptions": "0",
		})
		if code != http.StatusBadRequest {
			t.Errorf(`multipart create with max_subscriptions = "0": got %d %v, want 400`, code, resp)
		}
	})

	t.Run("update: 0 refused, null is unset, a value is kept", func(t *testing.T) {
		role := createRole(t, "update-me", map[string]any{"max_payload": 2048})
		path := roles + "/" + role.Id

		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"max_payload": 0}); code != http.StatusBadRequest {
			t.Errorf("PATCH max_payload = 0: got %d %v, want 400", code, resp)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), map[string]int{"max_payload": 2048})

		// Decrementing to exactly 0 is an explicit 0, not an unset field.
		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"max_payload-": 2048}); code != http.StatusBadRequest {
			t.Errorf("PATCH max_payload- = 2048 (down to 0): got %d %v, want 400", code, resp)
		}

		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"max_payload": 8192}); code != http.StatusOK {
			t.Fatalf("PATCH max_payload = 8192: %d %v", code, resp)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), map[string]int{"max_payload": 8192})

		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"max_payload": nil}); code != http.StatusOK {
			t.Fatalf("PATCH max_payload = null: %d %v", code, resp)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), map[string]int{"max_payload": -1})

		// A PATCH that does not mention the limits leaves them alone.
		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"description": "renamed"}); code != http.StatusOK {
			t.Fatalf("PATCH description: %d %v", code, resp)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), map[string]int{"max_subscriptions": -1, "max_payload": -1})
	})

	t.Run("the batch endpoint stone apply uses", func(t *testing.T) {
		app.Settings().Batch.Enabled = true
		t.Cleanup(func() { app.Settings().Batch.Enabled = false })

		batch := func(body map[string]any) (int, apiResponse) {
			return api.json(t, http.MethodPost, "/api/batch", token, map[string]any{
				"requests": []map[string]any{{"method": http.MethodPost, "url": roles, "body": body}},
			})
		}
		if code, resp := batch(roleBody("from-workspace", nil)); code != http.StatusOK {
			t.Fatalf("batch create: %d %v", code, resp)
		}
		role, err := app.FindFirstRecordByFilter("nats_roles", "name = {:n}", dbx.Params{"n": "from-workspace"})
		if err != nil {
			t.Fatalf("batch-created role not found: %v", err)
		}
		wantLimits(t, role, unsetLimits())

		if code, resp := batch(roleBody("zero-from-workspace", map[string]any{"max_subscriptions": 0})); code == http.StatusOK {
			t.Errorf("batch create with max_subscriptions = 0 should fail, got %d %v", code, resp)
		}
	})

	t.Run("app.Save without limits", func(t *testing.T) {
		col, err := app.FindCollectionByNameOrId("nats_roles")
		if err != nil {
			t.Fatal(err)
		}
		role := core.NewRecord(col)
		role.Set("name", "server-side")
		role.Set("organization", org.Id)
		if err := app.Save(role); err != nil {
			t.Fatalf("save role: %v", err)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), unsetLimits())

		// At the model layer a 0 and an unset field are the same value, so a 0
		// from Go code is stored as -1 too. That is the documented trade: only a
		// request can say "0" on purpose, and a request that does is refused.
		reloaded := reloadRecord(t, app, "nats_roles", role.Id)
		reloaded.Set("max_subscriptions", 0)
		if err := app.Save(reloaded); err != nil {
			t.Fatalf("save role with 0: %v", err)
		}
		wantLimits(t, reloadRecord(t, app, "nats_roles", role.Id), map[string]int{"max_subscriptions": -1})
	})
}

// Account limits are operator-only and have no create path through the API, so
// only the request layer applies: a write that sets a connection, subscription,
// payload or data limit to 0 is refused, blank included, since nothing turns an
// account's blank into -1. JetStream 0 is accepted, because it is a real setting
// (JetStream off). And a 0 already stored does not block later saves -- the
// console limits form resubmits all six fields every time.
func testAccountLimits(t *testing.T, app *pocketbase.PocketBase, api *recordAPI) {
	_, account := newOrgWithAccount(t, app, "acctlimits", "Account Limits", nil)
	path := "/api/collections/nats_accounts/records/" + account.Id

	su, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	superuser := core.NewRecord(su)
	superuser.Set("email", "operator@limits.test")
	superuser.Set("password", "operator-password-123")
	if err := app.Save(superuser); err != nil {
		t.Fatalf("create superuser: %v", err)
	}
	token := authToken(t, superuser)

	for _, field := range []string{"max_connections", "max_subscriptions", "max_data", "max_payload"} {
		for _, value := range []any{0, "", nil} {
			code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{field: value})
			if code != http.StatusBadRequest {
				t.Errorf("PATCH %s = %#v: got %d %v, want 400", field, value, code, resp)
			} else if got := fieldErrorCode(resp, field); got != "validation_nats_limit_zero" {
				t.Errorf("PATCH %s = %#v: field error code %q: %v", field, value, got, resp)
			}
		}
		if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{field: 4321}); code != http.StatusOK {
			t.Errorf("PATCH %s = 4321 (the paired allow): %d %v", field, code, resp)
		}
	}

	if code, resp := api.json(t, http.MethodPatch, path, token, map[string]any{"max_jetstream_memory_storage": 0}); code != http.StatusOK {
		t.Errorf("PATCH max_jetstream_memory_storage = 0 should be allowed (JetStream off): %d %v", code, resp)
	}

	// A 0 put there before this hook existed, by hand or by an operator.
	if _, err := app.DB().NewQuery("UPDATE {{nats_accounts}} SET [[max_connections]] = 0 WHERE [[id]] = {:id}").
		Bind(dbx.Params{"id": account.Id}).Execute(); err != nil {
		t.Fatal(err)
	}
	stored := reloadRecord(t, app, "nats_accounts", account.Id)
	form := map[string]any{"description": "edited from the limits form"}
	for _, field := range []string{"max_connections", "max_subscriptions", "max_data", "max_payload",
		"max_jetstream_disk_storage", "max_jetstream_memory_storage"} {
		form[field] = stored.GetInt(field)
	}
	if code, resp := api.json(t, http.MethodPatch, path, token, form); code != http.StatusOK {
		t.Errorf("resubmitting a 0 that is already stored should not be refused: %d %v", code, resp)
	}
}

func TestProvisioningRefusesZeroAccountDefaults(t *testing.T) {
	ok := hooks.OrgProvisioningOptions{
		NatsMaxConnections:            -1,
		NatsMaxSubscriptions:          5000,
		NatsMaxPayload:                1048576,
		NatsMaxJetStreamDiskStorage:   0, // JetStream off: a real setting, not a misreading
		NatsMaxJetStreamMemoryStorage: 0,
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid defaults refused: %v", err)
	}

	bad := ok
	bad.NatsMaxSubscriptions = 0
	bad.NatsMaxPayload = 0
	err := bad.Validate()
	if err == nil {
		t.Fatal("0 subscriptions and payload should be refused")
	}
	for _, key := range []string{"nats.default_limits.max_subscriptions", "nats.default_limits.max_payload"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should name %s: %v", key, err)
		}
	}
	if strings.Contains(err.Error(), "max_connections") {
		t.Errorf("error names a key that is not 0: %v", err)
	}
}

// operatorNATS runs a real operator-mode nats-server that trusts this database's
// operator, with the $SYS account and the given accounts preloaded into a MEMORY
// resolver: the arrangement `nats export` produces, built from the same records.
func operatorNATS(t *testing.T, app *pocketbase.PocketBase, accounts ...*core.Record) *natsserver.Server {
	t.Helper()
	operators, err := app.FindAllRecords("nats_system_operator")
	if err != nil || len(operators) == 0 {
		t.Fatalf("no system operator in the database: %v", err)
	}
	op := operators[0]
	sys := reloadRecord(t, app, "nats_accounts", op.GetString("system_account_id"))

	preload := ""
	for _, acc := range append([]*core.Record{sys}, accounts...) {
		acc = reloadRecord(t, app, "nats_accounts", acc.Id)
		if acc.GetString("jwt") == "" {
			t.Fatalf("account %q has no JWT", acc.GetString("name"))
		}
		preload += fmt.Sprintf("  %s: %q\n", acc.GetString("public_key"), acc.GetString("jwt"))
	}
	conf := fmt.Sprintf("operator: %q\nsystem_account: %s\nresolver: MEMORY\nresolver_preload: {\n%s}\n",
		op.GetString("jwt"), sys.GetString("public_key"), preload)

	path := filepath.Join(t.TempDir(), "nats.conf")
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	opts, err := natsserver.ProcessConfigFile(path)
	if err != nil {
		t.Fatalf("nats-server rejected the config built from the database: %v", err)
	}
	opts.Host = "127.0.0.1"
	opts.Port = -1
	opts.NoLog = true
	opts.NoSigs = true

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("create nats-server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		t.Fatal("nats-server never became ready")
	}
	t.Cleanup(func() {
		ns.Shutdown()
		ns.WaitForShutdown()
	})
	return ns
}

func credsFile(t *testing.T, user *core.Record) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), user.GetString("nats_username")+".creds")
	if err := os.WriteFile(path, []byte(user.GetString("creds_file")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The evidence for refusing 0 rather than keeping it as "publish-only", taken
// from a real server rather than from reading its source: a credential signed
// with Subs 0 is disconnected even though it never subscribes to anything, so it
// can never publish either. And the fix, end to end: a role created through the
// API without its limits now mints a credential that connects, subscribes and
// stays connected.
//
// Both credentials come out of the platform: same operator, same account, same
// pb-nats signing path. The Subs-0 one is the pre-fix role, reproduced by writing
// the 0 under the hooks and re-signing.
func testRoleLimitsOnARealServer(t *testing.T, app *pocketbase.PocketBase, api *recordAPI) {
	org, account := newOrgWithAccount(t, app, "realnats", "Real NATS", nil)
	owner := reloadRecord(t, app, "users", org.GetString("owner"))
	code, resp := api.json(t, http.MethodPost, "/api/collections/nats_roles/records", authToken(t, owner), map[string]any{
		"name":                  "no-limits-given",
		"organization":          org.Id,
		"publish_permissions":   []string{">"},
		"subscribe_permissions": []string{">"},
	})
	if code != http.StatusOK {
		t.Fatalf("create role: %d %v", code, resp)
	}
	role := reloadRecord(t, app, "nats_roles", resp["id"].(string))
	healthy := newNatsUser(t, app, org, account, role, "healthy")

	ns := operatorNATS(t, app, account)

	t.Run("a role created without limits connects, subscribes and stays up", func(t *testing.T) {
		nc, err := nats.Connect(ns.ClientURL(), nats.UserCredentials(credsFile(t, healthy)), nats.NoReconnect())
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer nc.Close()

		sub, err := nc.SubscribeSync("probe.>")
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		if err := nc.Publish("probe.one", []byte("hello")); err != nil {
			t.Fatalf("publish: %v", err)
		}
		if _, err := sub.NextMsg(5 * time.Second); err != nil {
			t.Fatalf("the published message never arrived: %v (last error: %v)", err, nc.LastError())
		}
		// The server closes a Subs-0 connection 20ms after CONNECT; be well past it.
		time.Sleep(300 * time.Millisecond)
		if !nc.IsConnected() {
			t.Fatalf("connection dropped: %v", nc.LastError())
		}
	})

	t.Run("a Subs-0 credential is disconnected without ever subscribing", func(t *testing.T) {
		zeroed := newNatsUser(t, app, org, account, role, "zeroed")
		if _, err := app.DB().NewQuery("UPDATE {{nats_roles}} SET [[max_subscriptions]] = 0 WHERE [[id]] = {:id}").
			Bind(dbx.Params{"id": role.Id}).Execute(); err != nil {
			t.Fatal(err)
		}
		zeroed.Set("regenerate", true)
		if err := app.Save(zeroed); err != nil {
			t.Fatalf("re-sign: %v", err)
		}
		zeroed = reloadRecord(t, app, "nats_users", zeroed.Id)
		if got := userClaims(t, zeroed).Limits.Subs; got != 0 {
			t.Fatalf("setup: re-signed JWT has Subs %d, want the pre-fix 0", got)
		}

		// Connect and do nothing else: no subscribe, no publish.
		//
		// The server answers CONNECT with -ERR 'maximum subscriptions exceeded'
		// and closes 20ms later, so there are three ways this ends, all of them
		// the same verdict: the -ERR lands inside the handshake and Connect
		// fails; or Connect returns and the -ERR closes the connection; or the
		// server's close wins and the client sees only EOF. Every report is
		// collected and one of them must name the limit.
		var reports []string
		closed := make(chan struct{})
		nc, err := nats.Connect(ns.ClientURL(),
			nats.UserCredentials(credsFile(t, zeroed)),
			nats.NoReconnect(),
			nats.ClosedHandler(func(*nats.Conn) { close(closed) }),
		)
		if err != nil {
			reports = append(reports, "connect: "+err.Error())
		} else {
			defer nc.Close()
			select {
			case <-closed:
				reports = append(reports, fmt.Sprintf("client: %v", nc.LastError()))
			case <-time.After(5 * time.Second):
				t.Fatal("a Subs-0 credential stayed connected; if nats-server now allows it, " +
					"revisit hooks/nats_limits.go, which refuses 0 on the strength of this test")
			}
		}

		// The server's own close reason, for the run where the client saw EOF. A
		// connection refused inside the handshake may not be recorded yet, which
		// is why this is one report among three rather than the assertion.
		connz, err := ns.Connz(&natsserver.ConnzOptions{State: natsserver.ConnClosed})
		if err != nil {
			t.Fatalf("connz: %v", err)
		}
		for _, ci := range connz.Conns {
			reports = append(reports, "server: "+ci.Reason)
		}
		named := false
		for _, r := range reports {
			named = named || strings.Contains(strings.ToLower(r), "maximum subscriptions exceeded")
		}
		if !named {
			t.Errorf("nothing says the connection ended for its subscription limit: %v", reports)
		}
	})
}
