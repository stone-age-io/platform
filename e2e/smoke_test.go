//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// A fresh install comes up through init and serve --nats, and an organization's
// NATS user connects with the creds the platform issued and round-trips a
// message. If this fails, nothing else in the package means anything.
func TestSmokeIssuedCredsConnectAndRoundTrip(t *testing.T) {
	tn := newTenant(t)
	_, creds := tn.natsUser(t, tn.role(t, []string{"smoke.>"}, []string{"smoke.>", "_INBOX.>"}))

	nc := connect(t, creds)
	sub, err := nc.SubscribeSync("smoke.ping")
	if err != nil {
		t.Fatal(err)
	}
	if err := nc.Publish("smoke.ping", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("no message back on smoke.ping: %v", err)
	}
	if string(msg.Data) != "hello" {
		t.Fatalf("got %q, want %q", msg.Data, "hello")
	}
}
