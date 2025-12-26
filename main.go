package main

import (
	"log"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/spf13/viper"

	pbaudit "github.com/skeeeon/pb-audit"
	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"
	pbtenancy "github.com/skeeeon/pb-tenancy"
)

// loadConfig handles the Viper initialization
func loadConfig() {
	// 1. Check for --config flag manually (before PocketBase parses flags)
	configPath := ""
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			break
		}
	}

	// 2. Set Configuration Source
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Default search locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/stone-age/")
	}

	// 3. Environment Variables (e.g. STONE_AGE_NATS_SERVER_URL)
	viper.SetEnvPrefix("STONE_AGE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 4. Set Defaults (Matching your libraries' defaults)
	setDefaults()

	// 5. Read Config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Only log if we explicitly asked for a specific file and it's missing
			if configPath != "" {
				log.Fatalf("❌ Explicit config file not found at: %s", configPath)
			}
			log.Println("ℹ️ No config file found, using defaults/environment variables")
		} else {
			log.Fatalf("❌ Error reading config file: %s", err)
		}
	} else {
		log.Printf("✅ Loaded configuration from: %s", viper.ConfigFileUsed())
	}
}

func setDefaults() {
	// Tenancy
	viper.SetDefault("tenancy.organizations_collection", "organizations")
	viper.SetDefault("tenancy.memberships_collection", "memberships")
	viper.SetDefault("tenancy.invites_collection", "invites")

	// NATS
	viper.SetDefault("nats.account_collection_name", "nats_accounts")
	viper.SetDefault("nats.user_collection_name", "nats_users")
	viper.SetDefault("nats.role_collection_name", "nats_roles")
	viper.SetDefault("nats.operator_name", "stone-age.io")
	viper.SetDefault("nats.server_url", "nats://localhost:4222")
	
	// NATS Default Limits (For the glue logic)
	viper.SetDefault("nats.default_limits.max_connections", 10)
	viper.SetDefault("nats.default_limits.max_subscriptions", 50)

	// Nebula
	viper.SetDefault("nebula.ca_collection_name", "nebula_ca")
	viper.SetDefault("nebula.network_collection_name", "nebula_networks")
	viper.SetDefault("nebula.host_collection_name", "nebula_hosts")
	viper.SetDefault("nebula.log_to_console", true)

	// Audit
	viper.SetDefault("audit.collection_name", "audit_logs")
}

func main() {
	// 1. Load Configuration
	loadConfig()

	app := pocketbase.New()

	// 2. Configure Libraries from Viper
	// We map the config keys directly to the Options structs

	// --- Tenancy ---
	tenancyOptions := pbtenancy.DefaultOptions()
	tenancyOptions.OrganizationsCollection = viper.GetString("tenancy.organizations_collection")
	tenancyOptions.MembershipsCollection = viper.GetString("tenancy.memberships_collection")
	tenancyOptions.InvitesCollection = viper.GetString("tenancy.invites_collection")
	tenancyOptions.LogToConsole = viper.GetBool("tenancy.log_to_console")
	if viper.IsSet("tenancy.invite_expiry_days") {
		tenancyOptions.InviteExpiryDays = viper.GetInt("tenancy.invite_expiry_days")
	}

	// --- NATS ---
	natsOptions := pbnats.DefaultOptions()
	natsOptions.AccountCollectionName = viper.GetString("nats.account_collection_name")
	natsOptions.UserCollectionName = viper.GetString("nats.user_collection_name")
	natsOptions.RoleCollectionName = viper.GetString("nats.role_collection_name")
	natsOptions.OperatorName = viper.GetString("nats.operator_name")
	natsOptions.NATSServerURL = viper.GetString("nats.server_url")
	natsOptions.LogToConsole = viper.GetBool("nats.log_to_console")
	// (Add backup URLs here if needed from config)

	// --- Nebula ---
	nebulaOptions := pbnebula.DefaultOptions()
	nebulaOptions.CACollectionName = viper.GetString("nebula.ca_collection_name")
	nebulaOptions.NetworkCollectionName = viper.GetString("nebula.network_collection_name")
	nebulaOptions.HostCollectionName = viper.GetString("nebula.host_collection_name")
	nebulaOptions.LogToConsole = viper.GetBool("nebula.log_to_console")
	if viper.IsSet("nebula.default_ca_validity_years") {
		nebulaOptions.DefaultCAValidityYears = viper.GetInt("nebula.default_ca_validity_years")
	}

	// --- Audit ---
	auditOptions := pbaudit.DefaultOptions()
	auditOptions.CollectionName = viper.GetString("audit.collection_name")
	auditOptions.LogToConsole = viper.GetBool("audit.log_console")

	// 3. Setup Libraries
	if err := pbaudit.Setup(app, auditOptions); err != nil {
		log.Fatalf("Failed to register audit setup: %v", err)
	}
	if err := pbtenancy.Setup(app, tenancyOptions); err != nil {
		log.Fatalf("Failed to register tenancy setup: %v", err)
	}
	if err := pbnats.Setup(app, natsOptions); err != nil {
		log.Fatalf("Failed to register NATS setup: %v", err)
	}
	if err := pbnebula.Setup(app, nebulaOptions); err != nil {
		log.Fatalf("Failed to register Nebula setup: %v", err)
	}

	// 4. Schema Injection (Add 'organization' field)
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil { return err }

		orgsCollection, err := app.FindCollectionByNameOrId(tenancyOptions.OrganizationsCollection)
		if err != nil { return nil } // Tenancy not initialized yet

		collectionsToUpdate := []string{
			natsOptions.AccountCollectionName,
			natsOptions.UserCollectionName,
			natsOptions.RoleCollectionName,
			nebulaOptions.CACollectionName,
			nebulaOptions.NetworkCollectionName,
			nebulaOptions.HostCollectionName,
		}

		for _, name := range collectionsToUpdate {
			col, err := app.FindCollectionByNameOrId(name)
			if err != nil { continue }

			if col.Fields.GetByName("organization") == nil {
				log.Printf("➕ Injecting organization field into '%s'...", name)
				col.Fields.Add(&core.RelationField{
					Name:         "organization",
					CollectionId: orgsCollection.Id,
					MaxSelect:    1,
				})
				// Silent fail on save, log if debug needed
				app.Save(col)
			}
		}
		return nil
	})

	// 5. Register Glue Hooks (Org Created -> Infrastructure Created)
	app.OnRecordAfterCreateSuccess(tenancyOptions.OrganizationsCollection).BindFunc(func(e *core.RecordEvent) error {
		log.Printf("🔗 Organization '%s' created, provisioning infrastructure...", e.Record.GetString("name"))

		// A. Create NATS Account
		natsCol, err := app.FindCollectionByNameOrId(natsOptions.AccountCollectionName)
		if err == nil {
			rec := core.NewRecord(natsCol)
			form := forms.NewRecordUpsert(app, rec)
			form.Load(map[string]any{
				"name":                         e.Record.GetString("name"),
				"organization":                 e.Record.Id,
				"active":                       true,
				// Load default limits from Viper
				"max_connections":              viper.GetInt("nats.default_limits.max_connections"),
				"max_subscriptions":            viper.GetInt("nats.default_limits.max_subscriptions"),
				"max_data":                     -1,
				"max_payload":                  viper.GetInt("nats.default_limits.max_payload"),
				"max_jetstream_disk_storage":   -1,
				"max_jetstream_memory_storage": -1,
			})
			if err := form.Submit(); err != nil {
				log.Printf("❌ Failed to create NATS account: %v", err)
			} else {
				log.Printf("✅ Created NATS Account")
			}
		}

		// B. Create Nebula CA
		nebulaCol, err := app.FindCollectionByNameOrId(nebulaOptions.CACollectionName)
		if err == nil {
			rec := core.NewRecord(nebulaCol)
			form := forms.NewRecordUpsert(app, rec)
			form.Load(map[string]any{
				"name":           e.Record.GetString("name") + " CA",
				"organization":   e.Record.Id,
				"validity_years": viper.GetInt("nebula.default_ca_validity_years"),
			})
			if err := form.Submit(); err != nil {
				log.Printf("❌ Failed to create Nebula CA: %v", err)
			} else {
				log.Printf("✅ Created Nebula CA")
			}
		}

		return e.Next()
	})

	// 6. Start Application
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
