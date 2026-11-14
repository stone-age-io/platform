//go:build e2e

package e2e

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// Fixtures set up the world as the superuser, as scripts/test-authz.sh does,
// except where the point is what a tenant role can do: then they act as that
// role. What a test asserts is what the bus or Nebula does with the result.

const testPassword = "Password123!"

type record struct {
	ID string `json:"id"`
}

var seq atomic.Int64

// uniq gives each fixture its own name, since every test shares one platform.
func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().Unix(), seq.Add(1))
}

// tenant is an organization with its owner signed in.
type tenant struct {
	OrgID      string
	AccountID  string
	OwnerToken string
}

func newTenant(t *testing.T) *tenant {
	t.Helper()
	ownerID, ownerEmail := createUser(t)
	var org record
	platform.mustRequest(t, 200, "POST", "/collections/organizations/records", platform.SUToken, map[string]any{
		"name": uniq("org"), "owner": ownerID, "active": true,
	}, &org)

	accounts := findByOrg(t, "nats_accounts", org.ID)
	if len(accounts) != 1 {
		t.Fatalf("org %s has %d NATS accounts, want 1", org.ID, len(accounts))
	}
	return &tenant{
		OrgID:      org.ID,
		AccountID:  accounts[0].ID,
		OwnerToken: signIn(t, ownerID, ownerEmail, org.ID),
	}
}

func createUser(t *testing.T) (id, email string) {
	t.Helper()
	email = uniq("user") + "@e2e.test"
	var r record
	platform.mustRequest(t, 200, "POST", "/collections/users/records", platform.SUToken, map[string]any{
		"email": email, "password": testPassword, "passwordConfirm": testPassword,
		"name": email, "verified": true,
	}, &r)
	return r.ID, email
}

// signIn points the user at org and returns their token.
func signIn(t *testing.T, userID, email, orgID string) string {
	t.Helper()
	platform.mustRequest(t, 200, "PATCH", "/collections/users/records/"+userID, platform.SUToken,
		map[string]any{"current_organization": orgID}, nil)
	var auth struct {
		Token string `json:"token"`
	}
	platform.mustRequest(t, 200, "POST", "/collections/users/auth-with-password", "",
		map[string]string{"identity": email, "password": testPassword}, &auth)
	return auth.Token
}

func findByOrg(t *testing.T, collection, orgID string) []record {
	t.Helper()
	var list struct {
		Items []record `json:"items"`
	}
	filter := url.QueryEscape(fmt.Sprintf("organization='%s'", orgID))
	platform.mustRequest(t, 200, "GET", "/collections/"+collection+"/records?filter="+filter, platform.SUToken, nil, &list)
	return list.Items
}

func (tn *tenant) role(t *testing.T, publish, subscribe []string) string {
	t.Helper()
	var r record
	platform.mustRequest(t, 200, "POST", "/collections/nats_roles/records", platform.SUToken, map[string]any{
		"name": uniq("role"), "organization": tn.OrgID,
		"publish_permissions": publish, "subscribe_permissions": subscribe,
	}, &r)
	return r.ID
}

// natsUser makes a NATS user and returns its id and the path of its issued
// creds file.
func (tn *tenant) natsUser(t *testing.T, roleID string) (id, credsPath string) {
	t.Helper()
	name := uniq("nu")
	var r record
	platform.mustRequest(t, 200, "POST", "/collections/nats_users/records", platform.SUToken, map[string]any{
		"email": name + "@nats.e2e.test", "password": testPassword, "passwordConfirm": testPassword,
		"nats_username": name, "account_id": tn.AccountID, "role_id": roleID,
		"organization": tn.OrgID, "active": true,
	}, &r)
	return r.ID, writeCreds(t, r.ID)
}

// member adds a `member` with a linked NATS user and returns their token.
func (tn *tenant) member(t *testing.T, roleID string) (token string) {
	t.Helper()
	userID, email := createUser(t)
	natsUserID, _ := tn.natsUser(t, roleID)
	platform.mustRequest(t, 200, "POST", "/collections/memberships/records", platform.SUToken, map[string]any{
		"user": userID, "organization": tn.OrgID, "role": "member", "nats_user": natsUserID,
	}, nil)
	return signIn(t, userID, email, tn.OrgID)
}

// thing creates a Thing through POST /api/org/things as the owner, the way the
// console does, and returns its id. body adds to the name.
func (tn *tenant) thing(t *testing.T, body map[string]any) string {
	t.Helper()
	body["name"] = uniq("thing")
	var r record
	platform.mustRequest(t, 200, "POST", "/org/things", tn.OwnerToken, body, &r)
	return r.ID
}

// thingCreds returns the path of a Thing's issued NATS creds file.
func thingCreds(t *testing.T, thingID string) string {
	t.Helper()
	var th struct {
		NatsUser string `json:"nats_user"`
	}
	platform.mustRequest(t, 200, "GET", "/collections/things/records/"+thingID, platform.SUToken, nil, &th)
	if th.NatsUser == "" {
		t.Fatalf("thing %s has no NATS user", thingID)
	}
	return writeCreds(t, th.NatsUser)
}

// writeCreds reads a NATS user's issued creds_file and writes it where nats.go
// can load it.
func writeCreds(t *testing.T, natsUserID string) string {
	t.Helper()
	var u struct {
		CredsFile string `json:"creds_file"`
	}
	eventually(t, 10*time.Second, func() error {
		platform.mustRequest(t, 200, "GET", "/collections/nats_users/records/"+natsUserID, platform.SUToken, nil, &u)
		if !strings.Contains(u.CredsFile, "NATS USER JWT") {
			return fmt.Errorf("nats user %s has no creds_file yet", natsUserID)
		}
		return nil
	})
	path := filepath.Join(t.TempDir(), natsUserID+".creds")
	if err := os.WriteFile(path, []byte(u.CredsFile), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// dial makes one connection attempt. opts add handlers.
func dial(credsPath string, opts ...nats.Option) (*nats.Conn, error) {
	return nats.Connect(platform.NatsURL,
		append([]nats.Option{nats.UserCredentials(credsPath), nats.NoReconnect()}, opts...)...)
}

// connect dials until it succeeds. The account JWT reaches the server late:
// after pb-nats's 3s debounce normally, but in the first ~30s after
// `serve --nats` starts, pb-nats has not yet connected to the embedded server
// ("bootstrap mode") and holds every publish until its 30s queue ticker.
// Hence 45s.
func connect(t *testing.T, credsPath string, opts ...nats.Option) *nats.Conn {
	t.Helper()
	var nc *nats.Conn
	eventually(t, 45*time.Second, func() error {
		var err error
		nc, err = dial(credsPath, opts...)
		return err
	})
	t.Cleanup(nc.Close)
	return nc
}

// refused is a check for eventually and holds: nil while the server refuses
// these creds.
func refused(credsPath string) func() error {
	return func() error {
		nc, err := dial(credsPath)
		if err != nil {
			return nil
		}
		nc.Close()
		return fmt.Errorf("the server accepted the connection")
	}
}

// closed is a check for eventually: nil once the server has dropped nc.
func closed(nc *nats.Conn) func() error {
	return func() error {
		if nc.IsClosed() {
			return nil
		}
		return fmt.Errorf("connection still open (status %v)", nc.Status())
	}
}
