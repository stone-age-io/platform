package leafsync

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

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
func (w *kvWriter) bucket(ctx context.Context, name string) (jetstream.KeyValue, error) {
	return w.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      name,
		Description: "leaf-sync mirror of PocketBase collection: " + name,
		History:     5,
		Storage:     jetstream.FileStorage,
	})
}
