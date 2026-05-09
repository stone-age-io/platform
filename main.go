package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/viper"

	pbaudit "github.com/skeeeon/pb-audit"
	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"
	pbtenancy "github.com/skeeeon/pb-tenancy"

	"platform/hooks"
	"platform/migrations"
)

//go:embed all:pb_public/*
var embeddedFS embed.FS

//go:embed schema.json
var schemaJSON []byte

// loadConfig handles the Viper initialization
func loadConfig() {
	// 1. Check for --config flag manually
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
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/stone-age/")
	}

	// 3. Environment Variables
	viper.SetEnvPrefix("STONE_AGE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 4. Set Defaults
	setDefaults()

	// 5. Read Config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
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
	viper.SetDefault("nats.default_limits.max_connections", 10)
	viper.SetDefault("nats.default_limits.max_subscriptions", 50)
	viper.SetDefault("nats.export_collection_name", "nats_account_exports")
	viper.SetDefault("nats.import_collection_name", "nats_account_imports")
	viper.SetDefault("nats.encryption_key", "")

	// Nebula
	viper.SetDefault("nebula.ca_collection_name", "nebula_ca")
	viper.SetDefault("nebula.network_collection_name", "nebula_networks")
	viper.SetDefault("nebula.host_collection_name", "nebula_hosts")
	viper.SetDefault("nebula.log_to_console", true)
	viper.SetDefault("nebula.encryption_key", "")

	// Audit
	viper.SetDefault("audit.collection_name", "audit_logs")
	viper.SetDefault("audit.retention.max_age", "")
	viper.SetDefault("audit.retention.max_records", 0)
	viper.SetDefault("audit.retention.interval", "0 2 * * *")

	// Branding (operator-level overrides for logo / theme / app name).
	// Empty disables overrides; the embedded default branding is used.
	viper.SetDefault("branding.dir", "")
}

func main() {
	loadConfig()

	app := pocketbase.New()

	// Pass embedded schema to initial migration
	migrations.SchemaJSON = schemaJSON

	// Register migrate command (auto-generates migration files in dev)
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	// Register the config flag with Cobra
	app.RootCmd.PersistentFlags().String("config", "", "Path to config file")

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
	natsOptions.ExportCollectionName = viper.GetString("nats.export_collection_name")
	natsOptions.ImportCollectionName = viper.GetString("nats.import_collection_name")
	natsOptions.EncryptionKey = viper.GetString("nats.encryption_key")

	// --- Nebula ---
	nebulaOptions := pbnebula.DefaultOptions()
	nebulaOptions.CACollectionName = viper.GetString("nebula.ca_collection_name")
	nebulaOptions.NetworkCollectionName = viper.GetString("nebula.network_collection_name")
	nebulaOptions.HostCollectionName = viper.GetString("nebula.host_collection_name")
	nebulaOptions.LogToConsole = viper.GetBool("nebula.log_to_console")
	if viper.IsSet("nebula.default_ca_validity_years") {
		nebulaOptions.DefaultCAValidityYears = viper.GetInt("nebula.default_ca_validity_years")
	}
	nebulaOptions.EncryptionKey = viper.GetString("nebula.encryption_key")

	// --- Audit ---
	auditOptions := pbaudit.DefaultOptions()
	auditOptions.CollectionName = viper.GetString("audit.collection_name")
	auditOptions.LogToConsole = viper.GetBool("audit.log_console")

	// Retention policy (optional)
	maxAgeStr := viper.GetString("audit.retention.max_age")
	maxRecords := viper.GetInt("audit.retention.max_records")
	if maxAgeStr != "" || maxRecords > 0 {
		retention := &pbaudit.RetentionPolicy{
			Interval: viper.GetString("audit.retention.interval"),
		}
		if maxAgeStr != "" {
			if d, err := time.ParseDuration(maxAgeStr); err == nil {
				retention.MaxAge = d
			}
		}
		if maxRecords > 0 {
			retention.MaxRecords = maxRecords
		}
		auditOptions.Retention = retention
	}

	// Setup Libraries (Hooks & APIs)
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

	// Register CLI Commands (for generating configs, keys, etc.)
	pbnats.RegisterCommands(app)

	// Platform-owned hooks: auto-provision NATS account + Nebula CA per new org.
	hooks.RegisterOrgProvisioning(app, hooks.OrgProvisioningOptions{
		OrgCollection:                tenancyOptions.OrganizationsCollection,
		NatsAccountCollection:        natsOptions.AccountCollectionName,
		NebulaCACollection:           nebulaOptions.CACollectionName,
		NatsMaxConnections:           viper.GetInt("nats.default_limits.max_connections"),
		NatsMaxSubscriptions:         viper.GetInt("nats.default_limits.max_subscriptions"),
		NatsMaxPayload:               viper.GetInt("nats.default_limits.max_payload"),
		NebulaDefaultCAValidityYears: viper.GetInt("nebula.default_ca_validity_years"),
	})

	// 6. Serve Embedded UI with SPA Support
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		subFS, err := fs.Sub(embeddedFS, "pb_public")
		if err != nil {
			return err
		}

		// Operator branding overlay: serves files (theme.css, logo.svg,
		// branding.json, etc.) from a host directory under /branding/*.
		// The route is always registered — index.html unconditionally <link>s
		// theme.css and the SPA fetches branding.json — so we serve silent
		// fallbacks for those two when no overlay directory is configured,
		// otherwise the browser console fills with 404s on stock deployments.
		var brandingFS fs.FS
		if brandingDir := viper.GetString("branding.dir"); brandingDir != "" {
			info, statErr := os.Stat(brandingDir)
			if statErr != nil || !info.IsDir() {
				log.Printf("⚠️  branding.dir is set but not a usable directory: %s", brandingDir)
			} else {
				brandingFS = os.DirFS(brandingDir)
				log.Printf("✅ Branding overlay serving from: %s", brandingDir)
			}
		}
		e.Router.GET("/branding/{path...}", func(e *core.RequestEvent) error {
			path := e.Request.PathValue("path")
			if path == "" || strings.Contains(path, "..") {
				return e.NotFoundError("Not found", nil)
			}
			if brandingFS != nil {
				if f, err := brandingFS.Open(path); err == nil {
					f.Close()
					return e.FileFS(brandingFS, path)
				}
			}
			switch path {
			case "theme.css":
				return e.Blob(200, "text/css; charset=utf-8", nil)
			case "branding.json":
				return e.Blob(200, "application/json", []byte("{}"))
			}
			return e.NotFoundError("Not found", nil)
		})

		e.Router.GET("/{path...}", func(e *core.RequestEvent) error {
			path := e.Request.PathValue("path")

			if path == "" || path == "/" {
				return e.FileFS(subFS, "index.html")
			}

			if f, err := subFS.Open(path); err == nil {
				f.Close()
				return e.FileFS(subFS, path)
			}

			if strings.Contains(path, ".") {
				return e.NotFoundError("File not found", nil)
			}

			return e.FileFS(subFS, "index.html")
		})

		return e.Next()
	})

	// Register Bootstrap Command
	addBootstrapCommand(app, tenancyOptions.OrganizationsCollection, tenancyOptions.MembershipsCollection, natsOptions)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
