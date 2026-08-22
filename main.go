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
	"platform/internal/natsd"
	"platform/internal/version"
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

// setDefaults must define a value for EVERY key the program later reads.
// Anything read without a default silently becomes the zero value when
// config.yaml is absent — which for an int means 0, and 0 is a legal-looking
// but wrong limit (a Nebula CA valid for zero years, a zero-byte NATS payload
// cap). Keep this list in sync with config.yaml.
func setDefaults() {
	// Tenancy
	viper.SetDefault("tenancy.organizations_collection", "organizations")
	viper.SetDefault("tenancy.memberships_collection", "memberships")
	viper.SetDefault("tenancy.invites_collection", "invites")
	viper.SetDefault("tenancy.invite_expiry_days", 7)
	viper.SetDefault("tenancy.log_to_console", false)

	// NATS
	viper.SetDefault("nats.account_collection_name", "nats_accounts")
	viper.SetDefault("nats.user_collection_name", "nats_users")
	viper.SetDefault("nats.role_collection_name", "nats_roles")
	viper.SetDefault("nats.operator_name", "stone-age.io")
	// 4222 is the NATS default and the port `nats export` writes into nats.conf.
	// This default used to be 4422, which disagreed with the exported config,
	// the README, and the docs — the symptom was a Control Plane stuck in
	// bootstrap mode against a server that was running fine on another port.
	viper.SetDefault("nats.server_url", "nats://localhost:4222")
	// WebSocket listeners for the browser console, served at runtime by
	// GET /api/client-config. Empty by default: the UI then falls back to its
	// compiled-in ws://localhost:9222, which is the listener `nats export`
	// generates. Not derived from server_url — that is a TCP address for this
	// process, on a different port and often a different hostname.
	viper.SetDefault("nats.websocket_urls", []string{})
	viper.SetDefault("nats.log_to_console", false)
	viper.SetDefault("nats.default_limits.max_connections", 10)
	viper.SetDefault("nats.default_limits.max_subscriptions", 50)
	viper.SetDefault("nats.default_limits.max_payload", 1048576)
	viper.SetDefault("nats.export_collection_name", "nats_account_exports")
	viper.SetDefault("nats.import_collection_name", "nats_account_imports")
	// Run a NATS server inside this process instead of alongside it. Off by
	// default: the normal topology is a separate nats-server. See --nats.
	viper.SetDefault("nats.embedded", false)
	viper.SetDefault("nats.embedded_config", natsd.DefaultConfigFile)
	// At-rest encryption is OFF by default, deliberately: an empty key means the
	// private_key/seed columns are stored in plaintext. Set it to exactly 32
	// characters (preferably via STONE_AGE_NATS_ENCRYPTION_KEY) to turn it on,
	// and keep a backup — losing an enabled key loses the encrypted records.
	viper.SetDefault("nats.encryption_key", "")
	// Subject subtree exported from every managed org's account into the
	// operator hub account (import remaps it to "<prefix>.{orgId}.>").
	viper.SetDefault("nats.managed_export_subject", "helpdesk.>")

	// Nebula
	viper.SetDefault("nebula.ca_collection_name", "nebula_ca")
	viper.SetDefault("nebula.network_collection_name", "nebula_networks")
	viper.SetDefault("nebula.host_collection_name", "nebula_hosts")
	viper.SetDefault("nebula.log_to_console", false)
	viper.SetDefault("nebula.default_ca_validity_years", 10)
	// Off by default, same rationale as nats.encryption_key above.
	viper.SetDefault("nebula.encryption_key", "")

	// Audit
	viper.SetDefault("audit.collection_name", "audit_logs")
	viper.SetDefault("audit.log_to_console", false)
	viper.SetDefault("audit.retention.max_age", "")
	viper.SetDefault("audit.retention.max_records", 0)
	viper.SetDefault("audit.retention.interval", "0 2 * * *")

	// Branding (operator-level overrides for logo / theme / app name).
	// Empty disables overrides; the embedded default branding is used.
	viper.SetDefault("branding.dir", "")
}

// validateEncryptionKey rejects a malformed at-rest encryption key at startup.
//
// An empty key is valid and is the default: at-rest encryption is opt-in, and
// leaving it off stores the private_key/seed columns in plaintext. A key of any
// other length than 32 bytes is never valid though — AES-256 needs exactly that
// — so it is a typo or a truncated secret, and the only safe response is to
// refuse to start rather than provision records that cannot be decrypted.
func validateEncryptionKey(key, name string) {
	if key == "" {
		return // encryption disabled — the default
	}
	if len(key) != 32 {
		log.Fatalf("❌ %s must be exactly 32 bytes when set (got %d). Leave it empty to disable at-rest encryption.", name, len(key))
	}
}

func main() {
	loadConfig()

	app := pocketbase.New()

	// Pass embedded schema to initial migration
	migrations.SchemaJSON = schemaJSON

	// Register migrate command. Automigrate writes a new migration file whenever a
	// collection is changed through the admin UI — helpful during development,
	// wrong in production, where the migrations directory is not part of the
	// deployed artifact and schema drift should arrive as reviewed code. Detect
	// `go run` the way PocketBase's own docs do: its throwaway binary lands under
	// the OS temp dir, whereas a released build does not.
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isGoRun,
	})

	// Register the config flag with Cobra
	app.RootCmd.PersistentFlags().String("config", "", "Path to config file")

	// `--version` on the root command. PocketBase sets this to its OWN version,
	// which is "(untracked)" in a module build -- so before this the platform
	// binary answered `--version` with a string that named neither the platform
	// nor PocketBase usefully. A bug report needs both: the API rules are the
	// entire authorization layer here, and their semantics are PocketBase's.
	app.RootCmd.Version = version.Version + " (PocketBase " +
		version.Dependency("github.com/pocketbase/pocketbase") + ")"

	// Embedded NATS server. Registered as persistent flags on the root command,
	// the same way --config above is, because PocketBase does not add the
	// `serve` subcommand until app.Start() — which registers and executes in one
	// call, leaving no point where a serve-only flag could be attached.
	//
	// Only `serve` acts on these; the other commands open the database directly
	// and have no bus to talk to.
	//
	// Bound to viper so the setting can equally come from config.yaml or
	// STONE_AGE_NATS_EMBEDDED. An unset flag falls through to those.
	app.RootCmd.PersistentFlags().Bool("nats", false,
		"serve: run a NATS server in this process, using the config from `nats export`")
	app.RootCmd.PersistentFlags().String("nats-config", natsd.DefaultConfigFile,
		"serve: path to the nats.conf used by --nats")
	if err := viper.BindPFlag("nats.embedded", app.RootCmd.PersistentFlags().Lookup("nats")); err != nil {
		log.Fatalf("❌ Failed to bind --nats: %v", err)
	}
	if err := viper.BindPFlag("nats.embedded_config", app.RootCmd.PersistentFlags().Lookup("nats-config")); err != nil {
		log.Fatalf("❌ Failed to bind --nats-config: %v", err)
	}

	// --- Tenancy ---
	tenancyOptions := pbtenancy.DefaultOptions()
	tenancyOptions.OrganizationsCollection = viper.GetString("tenancy.organizations_collection")
	tenancyOptions.MembershipsCollection = viper.GetString("tenancy.memberships_collection")
	tenancyOptions.InvitesCollection = viper.GetString("tenancy.invites_collection")
	tenancyOptions.LogToConsole = viper.GetBool("tenancy.log_to_console")
	// Assigned unconditionally: setDefaults guarantees a sane value, so the old
	// viper.IsSet guard only obscured whether the key could be missing.
	tenancyOptions.InviteExpiryDays = viper.GetInt("tenancy.invite_expiry_days")

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
	nebulaOptions.DefaultCAValidityYears = viper.GetInt("nebula.default_ca_validity_years")
	nebulaOptions.EncryptionKey = viper.GetString("nebula.encryption_key")

	// At-rest encryption is opt-in, but a key of the wrong length is always a
	// misconfiguration. Fail at startup rather than at the first key write, which
	// would leave an org half-provisioned.
	validateEncryptionKey(natsOptions.EncryptionKey, "nats.encryption_key")
	validateEncryptionKey(nebulaOptions.EncryptionKey, "nebula.encryption_key")

	// --- Audit ---
	auditOptions := pbaudit.DefaultOptions()
	auditOptions.CollectionName = viper.GetString("audit.collection_name")
	// Canonical key is audit.log_to_console, matching the tenancy/nats/nebula
	// sections. The legacy audit.log_console spelling is still honoured so an
	// existing config.yaml does not silently lose the setting.
	auditOptions.LogToConsole = viper.GetBool("audit.log_to_console") || viper.GetBool("audit.log_console")

	// Never audit the NATS root operator record. pb-audit snapshots the whole
	// record into before_changes/after_changes via PublicExport(), so anything
	// not flagged hidden in schema.json ends up copied into audit_logs. That
	// record holds the root of the entire NATS chain of trust — its seed signs
	// every account and user JWT on the platform — and there is no reason to
	// keep a second copy of it in a queryable collection. The hidden flags on
	// those fields are the primary defence; this is belt and braces.
	auditOptions.EventFilter = func(collectionName, eventType string) bool {
		return collectionName != "nats_system_operator"
	}

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

	// Platform-owned hooks: managed orgs export their service-event subtree
	// into the operator hub account with an org-prefixed local subject.
	managedExportSubject := viper.GetString("nats.managed_export_subject")
	if !strings.HasSuffix(managedExportSubject, ".>") {
		log.Fatalf("❌ nats.managed_export_subject must end in '.>' (got %q)", managedExportSubject)
	}
	hooks.RegisterManagedOrgExports(app, hooks.ManagedOrgExportsOptions{
		OrgCollection:     tenancyOptions.OrganizationsCollection,
		AccountCollection: natsOptions.AccountCollectionName,
		ExportCollection:  natsOptions.ExportCollectionName,
		ImportCollection:  natsOptions.ImportCollectionName,
		ExportSubject:     managedExportSubject,
	})

	// Platform-owned hooks: auto-provision a single NATS user per new leaf node.
	hooks.RegisterLeafNodeProvisioning(app, hooks.LeafNodeProvisioningOptions{
		LeafNodeCollection:    "leaf_nodes",
		NatsAccountCollection: natsOptions.AccountCollectionName,
		NatsUserCollection:    natsOptions.UserCollectionName,
		NatsRoleCollection:    natsOptions.RoleCollectionName,
	})

	// Makes `active` on things/leaf_nodes mean something. The authRule only stops
	// new logins; this invalidates outstanding tokens and revokes the device's
	// NATS identity, which is where its real capability lives.
	hooks.RegisterActiveFlag(app, hooks.ActiveFlagOptions{
		ThingCollection:    "things",
		LeafNodeCollection: "leaf_nodes",
		NatsUserCollection: natsOptions.UserCollectionName,
	})

	// Closes a departing member's tenant context. The inventory read rules trust
	// users.current_organization on its own, so a membership delete that leaves
	// it pointing at the old org leaves the reader inside it.
	hooks.RegisterMembershipLifecycle(app, hooks.MembershipLifecycleOptions{
		MembershipCollection: tenancyOptions.MembershipsCollection,
		UserCollection:       "users",
	})

	// Leaf-node-authenticated bootstrap routes. These serve the operator JWT, the
	// org account JWT, and the leaf's own creds, so a leaf-node identity needs no
	// read grant on nats_users or nats_accounts at all.
	hooks.RegisterLeafNodeRoutes(app, hooks.LeafNodeRoutesOptions{
		LeafNodeCollection:    "leaf_nodes",
		NatsUserCollection:    natsOptions.UserCollectionName,
		NatsAccountCollection: natsOptions.AccountCollectionName,
	})

	// Self-service credential rotation. Reading credentials needs no route (the
	// nats_users rules are row-scoped to the caller's own identity); rotation does,
	// because it must permit a write to exactly one field.
	hooks.RegisterCredentialRoutes(app, hooks.CredentialRoutesOptions{
		NatsUserCollection:   natsOptions.UserCollectionName,
		MembershipCollection: tenancyOptions.MembershipsCollection,
		ThingCollection:      "things",
		LeafNodeCollection:   "leaf_nodes",
	})

	// Signing-key operations on an org's own NATS account. Same reason as above:
	// the update rule cannot permit three trigger fields and forbid the limits,
	// so nats_accounts.updateRule is operator-only and these live in a route.
	hooks.RegisterNatsAccountRoutes(app, hooks.NatsAccountRoutesOptions{
		NatsAccountCollection: natsOptions.AccountCollectionName,
		MembershipCollection:  tenancyOptions.MembershipsCollection,
	})

	// Thing creation with optional identity provisioning, in one transaction. The
	// console used to do this in three unguarded calls, so a late failure orphaned
	// a signed NATS credential; it also never set `active`, so every Thing it made
	// was locked out of the API by things.authRule.
	hooks.RegisterThingRoutes(app, hooks.ThingRoutesOptions{
		ThingCollection:         "things",
		OrgCollection:           tenancyOptions.OrganizationsCollection,
		MembershipCollection:    tenancyOptions.MembershipsCollection,
		NatsUserCollection:      natsOptions.UserCollectionName,
		NatsAccountCollection:   natsOptions.AccountCollectionName,
		NatsRoleCollection:      natsOptions.RoleCollectionName,
		NebulaHostCollection:    nebulaOptions.HostCollectionName,
		NebulaNetworkCollection: nebulaOptions.NetworkCollectionName,
	})

	// Deployment facts the SPA needs at runtime but cannot be compiled with —
	// currently just the browser-facing NATS WebSocket URLs. A build-time
	// constant would mean a frontend rebuild per operator, which is the same
	// problem the branding overlay avoids.
	//
	// Validated at startup rather than shipped to the browser to fail there.
	// The env-var override is the reason this is worth a check: viper splits a
	// string env value on WHITESPACE, so STONE_AGE_NATS_WEBSOCKET_URLS with
	// comma-separated URLs yields one entry containing both, and the only
	// symptom would be a console that cannot connect for no stated reason.
	natsWebsocketURLs := viper.GetStringSlice("nats.websocket_urls")
	for _, u := range natsWebsocketURLs {
		// Checked before the scheme, because a comma-joined list still starts
		// with a valid scheme and would otherwise sail through as one URL.
		if strings.Contains(u, ",") {
			log.Fatalf("❌ nats.websocket_urls entry %q contains a comma, so it is one malformed URL rather than a list.\n"+
				"       In config.yaml write a YAML list: websocket_urls: [\"wss://a:9222\", \"wss://b:9222\"]\n"+
				"       In STONE_AGE_NATS_WEBSOCKET_URLS separate them with SPACES — viper splits that value on whitespace.", u)
		}
		if !strings.HasPrefix(u, "ws://") && !strings.HasPrefix(u, "wss://") {
			log.Fatalf("❌ nats.websocket_urls entries must start with ws:// or wss:// (got %q).\n"+
				"       These are browser WebSocket URLs, not the nats:// address in nats.server_url.", u)
		}
	}
	hooks.RegisterClientConfigRoutes(app, hooks.ClientConfigRoutesOptions{
		NatsWebsocketURLs: natsWebsocketURLs,
	})

	// Embedded NATS server. Bound to OnServe rather than OnBootstrap on purpose:
	// the support library calls e.Next() *before* it seeds the NATS operator, so
	// a bootstrap handler registered after its Setup actually runs earlier, when
	// the operator JWT does not exist yet and there is nothing to serve.
	//
	// A consequence is that the JWT publisher spends its first few seconds in
	// bootstrap mode before its retry ticker finds the server. That is the
	// normal, already-handled path — anything it queues drains on connect.
	var embeddedNATS *natsd.Server
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if !viper.GetBool("nats.embedded") {
			return e.Next()
		}
		srv, err := natsd.Start(
			viper.GetString("nats.embedded_config"),
			viper.GetString("nats.server_url"),
			"nats.server_url",
		)
		if err != nil {
			// Refuse to serve. Running on would mean issuing credentials for a
			// bus that isn't there, which looks like it worked until a device
			// tries to connect.
			return err
		}
		embeddedNATS = srv
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		embeddedNATS.Stop() // no-op when nil
		return e.Next()
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
