//go:build e2e

package e2e

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stone-age-io/kit/credskeeper"
	"github.com/stone-age-io/kit/natsx"
)

// The kit's credskeeper against the real credential routes, in both key modes
// (ADR 0009 §1 and "Verification"). Every application on the platform keeps
// its creds file with this package, so a change to auth-refresh, the
// nats_users read rule, POST /api/me/keys or the trigger fields breaks a test
// here, not an application in the field.

// keeperThing is a Thing provisioned through POST /api/org/things, with the
// login the owner was shown once.
type keeperThing struct {
	ID, Email, Password, NatsUser string
}

func (tn *tenant) keeperThing(t *testing.T, deviceKey bool) *keeperThing {
	t.Helper()
	natsBody := map[string]any{"mode": "auto", "role_id": tn.role(t, []string{"probe.>"}, []string{"_INBOX.>"})}
	if deviceKey {
		natsBody["device_key"] = true
	}
	var created struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	platform.mustRequest(t, 200, "POST", "/org/things", tn.OwnerToken, map[string]any{
		"name": uniq("thing"), "nats": natsBody, "nebula": map[string]any{"mode": "none"},
	}, &created)
	var th struct {
		NatsUser string `json:"nats_user"`
	}
	platform.mustRequest(t, 200, "GET", "/collections/things/records/"+created.ID, platform.SUToken, nil, &th)
	return &keeperThing{ID: created.ID, Email: created.Email, Password: created.Password, NatsUser: th.NatsUser}
}

func (k *keeperThing) keeper(t *testing.T, dir, password string) *credskeeper.Keeper {
	t.Helper()
	kp, err := credskeeper.New(credskeeper.Config{
		URL:           strings.TrimSuffix(platform.API, "/api"),
		Identity:      k.Email,
		Password:      password,
		CredsFile:     filepath.Join(dir, "user.creds"),
		AllowInsecure: true,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), credskeeper.NewMetrics(prometheus.NewRegistry()))
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

// owner sets one trigger or flag as the tenant's owner, the way the console's
// Regenerate, Revoke and Deactivate buttons do.
func (tn *tenant) setNatsUser(t *testing.T, id string, body map[string]any) {
	t.Helper()
	platform.mustRequest(t, 200, "PATCH", "/collections/nats_users/records/"+id, tn.OwnerToken, body, nil)
}

func (tn *tenant) setThing(t *testing.T, id string, body map[string]any) {
	t.Helper()
	platform.mustRequest(t, 200, "PATCH", "/collections/things/records/"+id, tn.OwnerToken, body, nil)
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// syncUntil syncs until the creds file differs from before. pb-nats re-mints
// on the trigger's save, so one sync is normally enough; the loop absorbs the
// account JWT's debounce, which the following connect also waits out.
func syncUntilChanged(t *testing.T, k *credskeeper.Keeper, path, before string) {
	t.Helper()
	eventually(t, 15*time.Second, func() error {
		if err := k.Sync(context.Background()); err != nil {
			return err
		}
		if mustRead(t, path) == before {
			return errors.New("the creds file has not changed yet")
		}
		return nil
	})
}

// A platform-held key: an application's first start, a restart without the
// password, a Regenerate and a Revoke.
func TestCredsKeeperPlatformHeldKey(t *testing.T) {
	tn := newTenant(t)
	th := tn.keeperThing(t, false)
	dir := t.TempDir()
	creds := filepath.Join(dir, "user.creds")

	// First start, as an application does it (ADR 0009 §2): run the keeper,
	// wait for the first credential, then connect through natsx.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	k := th.keeper(t, dir, th.Password)
	runCtx, stopRun := context.WithCancel(ctx)
	go k.Run(runCtx)
	if err := k.WaitForCreds(ctx); err != nil {
		t.Fatal(err)
	}
	stopRun()
	var nc *nats.Conn
	eventually(t, 45*time.Second, func() error {
		var err error
		nc, err = natsx.Connect(natsx.Options{URLs: []string{platform.NatsURL}, Creds: creds, Name: "e2e-keeper"})
		return err
	})
	nc.Close()

	// Restart without the password: the session is enough.
	k = th.keeper(t, dir, "")
	if err := k.Sync(context.Background()); err != nil {
		t.Fatalf("restart without the password: %v", err)
	}

	// Regenerate: a new JWT reaches the file.
	before := mustRead(t, creds)
	tn.setNatsUser(t, th.NatsUser, map[string]any{"regenerate": true})
	syncUntilChanged(t, k, creds, before)
	connect(t, creds)

	// Revoke: the old key is locked out, and the new one reaches the file.
	old := filepath.Join(t.TempDir(), "old.creds")
	if err := os.WriteFile(old, []byte(mustRead(t, creds)), 0o600); err != nil {
		t.Fatal(err)
	}
	before = mustRead(t, creds)
	tn.setNatsUser(t, th.NatsUser, map[string]any{"revoke": true})
	syncUntilChanged(t, k, creds, before)
	eventually(t, 15*time.Second, refused(old))
	connect(t, creds)
}

// A device-held key: enrollment, a Revoke that empties the key and a new
// enrollment, and a lost seed stopping the keeper.
func TestCredsKeeperDeviceHeldKey(t *testing.T) {
	tn := newTenant(t)
	th := tn.keeperThing(t, true)
	dir := t.TempDir()
	creds, seed := filepath.Join(dir, "user.creds"), filepath.Join(dir, "user.nk")

	k := th.keeper(t, dir, th.Password)
	if err := k.Sync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	var u struct {
		CredsFile string `json:"creds_file"`
		PublicKey string `json:"public_key"`
	}
	platform.mustRequest(t, 200, "GET", "/collections/nats_users/records/"+th.NatsUser, platform.SUToken, nil, &u)
	if u.CredsFile != "" || u.PublicKey == "" {
		t.Fatalf("after enrollment the platform holds creds_file=%t public_key=%q", u.CredsFile != "", u.PublicKey)
	}
	connect(t, creds)

	// Revoke empties the key. The keeper makes and enrolls a new one.
	firstSeed := mustRead(t, seed)
	old := filepath.Join(t.TempDir(), "old.creds")
	if err := os.WriteFile(old, []byte(mustRead(t, creds)), 0o600); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, creds)
	tn.setNatsUser(t, th.NatsUser, map[string]any{"revoke": true})
	syncUntilChanged(t, k, creds, before)
	if mustRead(t, seed) == firstSeed {
		t.Fatal("the keeper kept the revoked seed")
	}
	eventually(t, 15*time.Second, refused(old))
	connect(t, creds)

	// A lost seed: the platform's key is not one this host holds.
	for _, f := range []string{seed, creds} {
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
	}
	err := k.Sync(context.Background())
	if !errors.Is(err, credskeeper.ErrForeignKey) {
		t.Fatalf("sync with the seed lost: %v, want ErrForeignKey", err)
	}
	if _, statErr := os.Stat(seed); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("the keeper made a seed over a key the platform holds")
	}
}

// ADR 0009 §6: whoever holds the Thing's session outlives a Revoke, and
// deactivating the Thing is what locks them out.
func TestCredsKeeperStolenSessionNeedsDeactivation(t *testing.T) {
	tn := newTenant(t)
	th := tn.keeperThing(t, true)
	hostDir, thiefDir := t.TempDir(), t.TempDir()

	host := th.keeper(t, hostDir, th.Password)
	if err := host.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The thief copies the session file out of the compromised host.
	session := mustRead(t, filepath.Join(hostDir, "platform-session.json"))
	if err := os.WriteFile(filepath.Join(thiefDir, "platform-session.json"), []byte(session), 0o600); err != nil {
		t.Fatal(err)
	}
	thief := th.keeper(t, thiefDir, "")

	// A Revoke alone: the thief enrolls first and gets working creds, and
	// the real host stops on a key it did not make.
	tn.setNatsUser(t, th.NatsUser, map[string]any{"revoke": true})
	if err := thief.Sync(context.Background()); err != nil {
		t.Fatalf("thief after Revoke: %v", err)
	}
	connect(t, filepath.Join(thiefDir, "user.creds"))
	if err := host.Sync(context.Background()); !errors.Is(err, credskeeper.ErrForeignKey) {
		t.Fatalf("host after the thief enrolled: %v, want ErrForeignKey", err)
	}

	// The recovery in §6: deactivate, set a new password, reactivate, Revoke.
	tn.setThing(t, th.ID, map[string]any{"active": false})
	eventually(t, 15*time.Second, refused(filepath.Join(thiefDir, "user.creds")))
	newPassword := "Recovered-" + testPassword
	tn.setThing(t, th.ID, map[string]any{"password": newPassword, "passwordConfirm": newPassword})
	tn.setThing(t, th.ID, map[string]any{"active": true})
	tn.setNatsUser(t, th.NatsUser, map[string]any{"revoke": true})

	if err := thief.Sync(context.Background()); !errors.Is(err, credskeeper.ErrNeedPassword) {
		t.Fatalf("thief after deactivation: %v, want ErrNeedPassword", err)
	}
	host = th.keeper(t, hostDir, newPassword)
	if err := host.Sync(context.Background()); err != nil {
		t.Fatalf("host with the new password: %v", err)
	}
	connect(t, filepath.Join(hostDir, "user.creds"))
	eventually(t, 15*time.Second, refused(filepath.Join(thiefDir, "user.creds")))
}
