//go:build e2e

package e2e

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	nebulacert "github.com/slackhq/nebula/cert"
	"golang.org/x/crypto/curve25519"
)

// Device-held keys, as a device does them: sign in as the Thing, make the key
// pair, keep the private half, and enroll the public half through
// POST /api/me/keys -- the route the agent calls. What the platform issues for
// that key then goes to the real NATS server and Nebula's own config loader,
// the same as for a key the platform generated.

// deviceThing is a Thing provisioned through POST /api/org/things as the owner,
// signed in as itself.
type deviceThing struct {
	ID    string
	Token string
}

func (tn *tenant) deviceThing(t *testing.T, body map[string]any) *deviceThing {
	t.Helper()
	body["name"] = uniq("thing")
	var created struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	platform.mustRequest(t, 200, "POST", "/org/things", tn.OwnerToken, body, &created)
	if created.Password == "" {
		t.Fatal("POST /api/org/things returned no password to the owner")
	}
	var auth struct {
		Token string `json:"token"`
	}
	platform.mustRequest(t, 200, "POST", "/collections/things/auth-with-password", "",
		map[string]string{"identity": created.Email, "password": created.Password}, &auth)
	return &deviceThing{ID: created.ID, Token: auth.Token}
}

// enrollNATS makes a user key pair, enrolls it as the Thing, and builds the
// .creds from the JWT the platform signs for it and the seed, as the agent
// does. It returns the path of the .creds.
func (d *deviceThing) enrollNATS(t *testing.T) string {
	t.Helper()
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := kp.PublicKey()
	seed, _ := kp.Seed()
	platform.mustRequest(t, 200, "POST", "/me/keys", d.Token, map[string]string{"nats_public_key": pub}, nil)

	var th struct {
		NatsUser string `json:"nats_user"`
	}
	platform.mustRequest(t, 200, "GET", "/collections/things/records/"+d.ID, d.Token, nil, &th)

	// Read as the Thing itself: its own identity is all it can see.
	var u struct {
		JWT       string `json:"jwt"`
		CredsFile string `json:"creds_file"`
		PublicKey string `json:"public_key"`
	}
	eventually(t, 10*time.Second, func() error {
		platform.mustRequest(t, 200, "GET", "/collections/nats_users/records/"+th.NatsUser, d.Token, nil, &u)
		if u.JWT == "" {
			return fmt.Errorf("nats user %s has no JWT yet", th.NatsUser)
		}
		return nil
	})
	if u.CredsFile != "" {
		t.Fatal("a device-held identity has a creds_file on the platform")
	}
	if u.PublicKey != pub {
		t.Fatalf("the platform holds key %s, want the enrolled %s", u.PublicKey, pub)
	}

	creds, err := natsjwt.FormatUserConfig(u.JWT, seed)
	if err != nil {
		t.Fatalf("build .creds from the platform JWT and the device seed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "device.creds")
	if err := os.WriteFile(path, creds, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// enrollNebula makes an X25519 pair as `nebula-cert keygen` does, enrolls the
// public key as the Thing, and returns the path of the private key file.
func (d *deviceThing) enrollNebula(t *testing.T) string {
	t.Helper()
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(priv); err != nil {
		t.Fatal(err)
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := string(nebulacert.MarshalPublicKeyToPEM(nebulacert.Curve_CURVE25519, pub))
	platform.mustRequest(t, 200, "POST", "/me/keys", d.Token, map[string]string{"nebula_public_key": pubPEM}, nil)

	path := filepath.Join(t.TempDir(), "host.key")
	if err := os.WriteFile(path, nebulacert.MarshalPrivateKeyToPEM(nebulacert.Curve_CURVE25519, priv), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
