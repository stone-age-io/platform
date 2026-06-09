package leafsync

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// heartbeatBucket is the account KV bucket leaf-sync writes liveness into, keyed
// by leaf node code. It lives on the HUB's JetStream domain so the platform UI
// (connected to the hub) can read it; the digital-twin `twin` bucket lives here
// too. One key per site; the leaf node's `>` perms cover the cross-domain write.
const heartbeatBucket = "leaf_status"

// heartbeat is the liveness payload, written once per sync cycle. Deliberately
// flat: the absence of a recent beat is what signals "offline", so there is no
// need to also report hub-connection state (a report that couldn't be delivered
// during an outage anyway).
type heartbeat struct {
	Code     string         `json:"code"`
	Version  string         `json:"version"`
	TS       string         `json:"ts"`
	Interval string         `json:"interval"`
	Synced   map[string]int `json:"synced"`
	Errors   []string       `json:"errors"`
}

// heartbeater publishes liveness beats. A zero/nil heartbeater is valid and
// no-ops, so callers never need to branch on whether the heartbeat is enabled.
type heartbeater struct {
	kv   jetstream.KeyValue
	code string
}

// openHeartbeat opens (creating if needed) the leaf_status bucket on the hub's
// JetStream domain. It returns a no-op heartbeater (logging why) when the
// heartbeat is disabled or can't be set up — never an error, since liveness
// reporting must not gate the sync loop.
func openHeartbeat(ctx context.Context, nc *nats.Conn, hubDomain, code string) *heartbeater {
	if hubDomain == "" {
		log.Printf("leaf-sync: nats.hub_domain not set; heartbeat disabled")
		return &heartbeater{}
	}
	if code == "" {
		log.Printf("leaf-sync: leaf node has no code; heartbeat disabled")
		return &heartbeater{}
	}

	// Target the hub domain explicitly: leaf-sync is connected to the LOCAL leaf,
	// so the default JetStream context would write to the local domain (invisible
	// to the hub). The write then routes over the leaf remote.
	js, err := jetstream.NewWithDomain(nc, hubDomain)
	if err != nil {
		log.Printf("⚠️ leaf-sync: heartbeat disabled (JetStream on domain %q): %v", hubDomain, err)
		return &heartbeater{}
	}
	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      heartbeatBucket,
		Description: "leaf-sync liveness heartbeats, keyed by leaf node code",
		History:     1,
		Storage:     jetstream.FileStorage,
	})
	if err != nil {
		log.Printf("⚠️ leaf-sync: heartbeat disabled (open %q on domain %q): %v", heartbeatBucket, hubDomain, err)
		return &heartbeater{}
	}

	log.Printf("leaf-sync: heartbeat → %s/%s on hub domain %q", heartbeatBucket, code, hubDomain)
	return &heartbeater{kv: kv, code: code}
}

// publish writes one beat. Best-effort: errors are logged and swallowed so a
// heartbeat failure (e.g. during a WAN outage) never disturbs the sync loop.
func (h *heartbeater) publish(ctx context.Context, synced map[string]int, errs []string, interval time.Duration) {
	if h == nil || h.kv == nil {
		return
	}
	if errs == nil {
		errs = []string{}
	}
	payload, err := json.Marshal(heartbeat{
		Code:     h.code,
		Version:  Version,
		TS:       time.Now().UTC().Format(time.RFC3339),
		Interval: interval.String(),
		Synced:   synced,
		Errors:   errs,
	})
	if err != nil {
		log.Printf("leaf-sync: heartbeat marshal failed: %v", err)
		return
	}
	if _, err := h.kv.Put(ctx, h.code, payload); err != nil {
		log.Printf("leaf-sync: heartbeat publish failed (will retry next cycle): %v", err)
	}
}
