package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// OrgProvisioningOptions captures what new organizations get provisioned with.
// Kept as a plain struct so the caller (main.go) owns the config-loading story.
type OrgProvisioningOptions struct {
	OrgCollection         string
	NatsAccountCollection string
	NebulaCACollection    string

	NatsMaxConnections   int
	NatsMaxSubscriptions int
	NatsMaxPayload       int

	NebulaDefaultCAValidityYears int
}

// RegisterOrgProvisioning attaches a hook that auto-provisions a NATS account
// and Nebula CA for every newly-created organization (except the System org,
// which the bootstrap command handles explicitly).
func RegisterOrgProvisioning(app *pocketbase.PocketBase, opts OrgProvisioningOptions) {
	app.OnRecordAfterCreateSuccess(opts.OrgCollection).BindFunc(func(e *core.RecordEvent) error {
		// The System org adopts the pre-seeded $SYS records in bootstrap instead.
		// Flag is authoritative; the name check covers DBs bootstrapped before it existed.
		if e.Record.GetBool("is_system_org") || e.Record.GetString("name") == "System" {
			return e.Next()
		}

		orgName := e.Record.GetString("name")
		log.Printf("🔗 Organization '%s' created, provisioning infrastructure...", orgName)

		if natsCol, err := e.App.FindCollectionByNameOrId(opts.NatsAccountCollection); err == nil {
			rec := core.NewRecord(natsCol)
			rec.Set("name", orgName)
			rec.Set("organization", e.Record.Id)
			rec.Set("active", true)
			rec.Set("max_connections", opts.NatsMaxConnections)
			rec.Set("max_subscriptions", opts.NatsMaxSubscriptions)
			rec.Set("max_data", -1)
			rec.Set("max_payload", opts.NatsMaxPayload)
			rec.Set("max_jetstream_disk_storage", -1)
			rec.Set("max_jetstream_memory_storage", -1)

			if err := e.App.Save(rec); err != nil {
				log.Printf("❌ Failed to create NATS account: %v", err)
			} else {
				log.Printf("✅ Created NATS Account")
			}
		}

		if nebulaCol, err := e.App.FindCollectionByNameOrId(opts.NebulaCACollection); err == nil {
			rec := core.NewRecord(nebulaCol)
			rec.Set("name", orgName+" CA")
			rec.Set("organization", e.Record.Id)
			rec.Set("validity_years", opts.NebulaDefaultCAValidityYears)

			if err := e.App.Save(rec); err != nil {
				log.Printf("❌ Failed to create Nebula CA: %v", err)
			} else {
				log.Printf("✅ Created Nebula CA")
			}
		}

		return e.Next()
	})
}
