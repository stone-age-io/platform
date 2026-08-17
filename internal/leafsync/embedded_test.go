package leafsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platform/internal/leafsync/pbclient"
)

func TestStartEmbeddedNATSMissingConfigNamesLeafSync(t *testing.T) {
	cfg := &Config{EmbeddedConfig: filepath.Join(t.TempDir(), "nats-leaf.conf")}

	_, err := startEmbeddedNATS(cfg)
	if err == nil {
		t.Fatal("expected an error for a missing leaf config, got nil")
	}
	// The whole reason this check exists ahead of natsd's own: natsd tells the
	// reader to run `stone-age nats export`, which is the Control Plane's
	// command and does not produce nats-leaf.conf.
	if !strings.Contains(err.Error(), "leaf-sync config") {
		t.Errorf("error should point at `leaf-sync config`, got: %v", err)
	}
	if strings.Contains(err.Error(), "nats export") {
		t.Errorf("error leaked the Control Plane's bootstrap command: %v", err)
	}
}

// failingAuthServer stands in for a central PocketBase that is unreachable,
// then comes back after failUntil rejections. Returns the server and a counter
// of auth attempts.
func failingAuthServer(t *testing.T, failUntil int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) <= failUntil {
			http.Error(w, `{"message":"unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"t","record":{"id":"leaf1","code":"S01"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &attempts
}

func shrinkAuthBackoff(t *testing.T) {
	t.Helper()
	oldMin, oldMax := authRetryMin, authRetryMax
	authRetryMin, authRetryMax = time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() { authRetryMin, authRetryMax = oldMin, oldMax })
}

// Without --nats the bus belongs to another process, so exiting on a failed
// login is free and remains the behaviour.
func TestAuthenticateFailsFastWithoutEmbeddedNATS(t *testing.T) {
	srv, attempts := failingAuthServer(t, 5)

	cfg := &Config{PocketBaseURL: srv.URL, PocketBaseEmail: "e", PocketBasePassword: "p"}
	if _, err := authenticate(context.Background(), pbclient.New(srv.URL), cfg); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("expected exactly 1 attempt without EmbedNATS, got %d", got)
	}
}

// With --nats, exiting would take the leaf server down with it, so a central
// platform that is briefly unreachable must not end the process.
func TestAuthenticateRetriesWhenHostingTheBus(t *testing.T) {
	shrinkAuthBackoff(t)
	srv, attempts := failingAuthServer(t, 3)

	cfg := &Config{PocketBaseURL: srv.URL, PocketBaseEmail: "e", PocketBasePassword: "p", EmbedNATS: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaf, err := authenticate(ctx, pbclient.New(srv.URL), cfg)
	if err != nil {
		t.Fatalf("authenticate should have recovered: %v", err)
	}
	if leaf["code"] != "S01" {
		t.Errorf("did not return the authenticated record, got %v", leaf)
	}
	if got := attempts.Load(); got != 4 {
		t.Errorf("expected 3 failures then a success (4 attempts), got %d", got)
	}
}

// The retry must not outlive a SIGINT/SIGTERM.
func TestAuthenticateRetryStopsOnContextCancel(t *testing.T) {
	shrinkAuthBackoff(t)
	srv, _ := failingAuthServer(t, 1<<30) // never recovers

	cfg := &Config{PocketBaseURL: srv.URL, PocketBaseEmail: "e", PocketBasePassword: "p", EmbedNATS: true}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)

	done := make(chan error, 1)
	go func() {
		_, err := authenticate(ctx, pbclient.New(srv.URL), cfg)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error after cancellation, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("authenticate did not return after the context was cancelled")
	}
}
