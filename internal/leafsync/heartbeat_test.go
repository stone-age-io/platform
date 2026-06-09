package leafsync

import (
	"context"
	"testing"
	"time"
)

// A disabled or zero-value heartbeater must be safe to publish on: heartbeat
// reporting is best-effort and must never panic or disturb the sync loop.
func TestHeartbeaterDisabledIsNoop(t *testing.T) {
	ctx := context.Background()
	synced := map[string]int{"things": 3}

	// nil pointer (defensive) and zero-value (kv == nil) both no-op.
	var nilHB *heartbeater
	nilHB.publish(ctx, synced, nil, time.Second)

	openHeartbeat(ctx, nil, "", "warehouse-a").publish(ctx, synced, nil, time.Second)
}
