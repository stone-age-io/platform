package leafsync

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// openOrCreateKV returns an existing bucket untouched, creating it from cfg only
// if it is absent.
//
// Deliberately NOT CreateOrUpdateKeyValue: `twin` and `leaf_status` are shared
// with operators — the console can create and tune them too — and an agent that
// reasserts its own retention on every startup silently reverts whatever an
// admin set in the UI, with no error and no log line anywhere. Whoever creates
// the bucket first defines it; leaf-sync only fills in the gap.
//
// The per-collection config mirrors below are the exception: leaf-sync owns
// those outright, so it may keep enforcing their shape.
func openOrCreateKV(ctx context.Context, js jetstream.JetStream, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) {
	kv, err := js.KeyValue(ctx, cfg.Bucket)
	if err == nil {
		return kv, nil
	}
	if !errors.Is(err, jetstream.ErrBucketNotFound) {
		return nil, err
	}
	return js.CreateKeyValue(ctx, cfg)
}

// kvWriter wraps a JetStream context for writing mirrored records into local
// KV buckets. leaf-sync connects to the LOCAL leaf, so the default JetStream
// context targets the leaf's local domain.
type kvWriter struct {
	js jetstream.JetStream
}

func newKVWriter(nc *nats.Conn) (*kvWriter, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}
	return &kvWriter{js: js}, nil
}

// bucket returns (creating if needed) the KV bucket mirroring a collection.
// These buckets are leaf-sync's alone — nothing else writes them — so unlike
// the shared `twin`/`leaf_status` buckets it is safe to keep asserting config.
func (w *kvWriter) bucket(ctx context.Context, name string) (jetstream.KeyValue, error) {
	return w.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      name,
		Description: "leaf-sync mirror of PocketBase collection: " + name,
		History:     5,
		Storage:     jetstream.FileStorage,
	})
}
