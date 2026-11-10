package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/spf13/viper"

	pbaudit "github.com/skeeeon/pb-audit"
	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"

	"platform/hooks"
	"platform/internal/demoseed"
	"platform/internal/natsd"
	"platform/internal/version"
	"platform/migrations"
)

//go:embed all:pb_public/*
var embeddedFS embed.FS

//go:embed schema.json
var schemaJSON []byte

// loadConfig initializes Viper.
func loadConfig() {
	// Read --config by hand: Cobra has not parsed flags yet.
	configPath := ""
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			break
		}
		// Cobra accepts --config=path too, so it must not silently fall
		// through to ./config.yaml here.
		if v, ok := strings.CutPrefix(arg, "--config="); ok {
			configPath = v
			break
		}
	}

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/stone-age/")
	}

	viper.SetEnvPrefix("STONE_AGE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	setDefaults()

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

// setDefaults must define a value for every key the program later reads.
// Without one, an absent config.yaml yields the zero value, and 0 is a
// legal-looking but wrong limit (a CA valid for zero years). Keep this list in
// sync with config.yaml.
func setDefaults() {
	// Tenancy
	viper.SetDefault("tenancy.invite_expiry_days", 7)

	// NATS
	viper.SetDefault("nats.operator_name", "stone-age.io")
	// The port `nats export` writes into nats.conf.
	viper.SetDefault("nats.server_url", "nats://localhost:4222")
	// Browser WebSocket URLs, served by GET /api/client-config. Empty means the
	// UI's compiled-in ws://localhost:9222, the listener `nats export` writes.
	// Never derived from server_url, which is this process's TCP address.
	viper.SetDefault("nats.websocket_urls", []string{})
	// Served by GET /api/me/leaf-config. hooks.LeafConfigRoutesOptions says why
	// neither is derived from nats.server_url.
	viper.SetDefault("nats.leaf_url", "")
	viper.SetDefault("nats.jetstream_domain", "hub")
	viper.SetDefault("nats.log_to_console", false)
	// Fair-use ceilings stamped into each new org's account JWT. config.yaml
	// carries the reasoning.
	viper.SetDefault("nats.default_limits.max_connections", 100)
	viper.SetDefault("nats.default_limits.max_subscriptions", 5000)
	viper.SetDefault("nats.default_limits.max_payload", 1048576)
	viper.SetDefault("nats.default_limits.max_jetstream_disk_storage", 5*1024*1024*1024)
	viper.SetDefault("nats.default_limits.max_jetstream_memory_storage", 64*1024*1024)
	// Off by default: the normal topology is a separate nats-server. See --nats.
	viper.SetDefault("nats.embedded", false)
	viper.SetDefault("nats.embedded_config", natsd.DefaultConfigFile)
	// Empty means at-rest encryption is off and the private_key/seed columns are
	// plaintext. When set it must be 32 characters; losing it loses the
	// encrypted records.
	viper.SetDefault("nats.encryption_key", "")
	// Exported from each managed org into the operator account, imported as
	// "<prefix>.{orgCode}.>".
	viper.SetDefault("nats.managed_export_subject", "helpdesk.>")
	// plans/inventory-events.md. The local subject is configurable so a
	// white-labelled deployment can drop "stone-age." from tenant subjects.
	viper.SetDefault("nats.inventory_events_enabled", false)
	viper.SetDefault("nats.inventory_local_subject", "stone-age.>")

	// Nebula
	viper.SetDefault("nebula.log_to_console", false)
	viper.SetDefault("nebula.default_ca_validity_years", 10)
	// Same as nats.encryption_key.
	viper.SetDefault("nebula.encryption_key", "")

	// Audit
	viper.SetDefault("audit.log_to_console", false)
	viper.SetDefault("audit.retention.max_age", "")
	viper.SetDefault("audit.retention.max_records", 0)
	viper.SetDefault("audit.retention.interval", "0 2 * * *")

	// Empty serves the embedded default branding.
	viper.SetDefault("branding.dir", "")

	// /api/ready cannot be switched off. /metrics is open by default (aggregate
	// counts, no customer identifiers); metrics.token closes it.
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.token", "")
	viper.SetDefault("readiness.interval", "15s")
	viper.SetDefault("readiness.timeout", "5s")
}

// validateEncryptionKey refuses to start on an at-rest key that is set but not
// 32 bytes (AES-256), rather than provision records that cannot be decrypted.
// Empty is valid: encryption is opt-in.
func validateEncryptionKey(key, name string) {
	if key == "" {
		return
	}
	if len(key) != 32 {
		log.Fatalf("❌ %s must be exactly 32 bytes when set (got %d). Leave it empty to disable at-rest encryption.", name, len(key))
	}
}

// requireDuration reads a duration setting and refuses one that does not parse.
// viper.GetDuration turns a typo into 0, which every caller reads as "not set".
// Empty is still "not set".
func requireDuration(key string) time.Duration {
	s := strings.TrimSpace(viper.GetString(key))
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Fatalf("❌ %s must be a Go duration such as \"15s\" or \"720h\" (got %q). Days are not a unit: 30 days is \"720h\".", key, s)
	}
	return d
}

func main() {
	loadConfig()

	app := pocketbase.New()

	migrations.SchemaJSON = schemaJSON

	// Automigrate only under `go run` (its binary lands in the OS temp dir, the
	// check PocketBase's docs use). In production schema changes arrive as
	// reviewed migrations, not admin UI edits.
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isGoRun,
	})

	app.RootCmd.PersistentFlags().String("config", "", "Path to config file")

	// PocketBase sets --version to its own, which is "(untracked)" in a module
	// build. A bug report needs the platform version plus PocketBase (whose
	// semantics the API rules use) and the pb-* libraries (credential minting,
	// CA issuance, audit). Library versions come from build info, so an untagged
	// one reads back as a pseudo-version.
	app.RootCmd.Version = version.Version + "\n" + version.DependencyLines(
		"ganigeorgiev.com/pocketbase/pocketbase",
		"github.com/skeeeon/pb-nats",
		"github.com/skeeeon/pb-nebula",
		"github.com/skeeeon/pb-audit",
	)

	// Root persistent flags because PocketBase adds `serve` inside app.Start(),
	// leaving nowhere to attach a serve-only flag. Only `serve` acts on them.
	// Bound to viper, so an unset flag falls through to config or env.
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

	// Collection names are constants, not config: schema.json names collections
	// literally in its API rules, so renaming one only breaks authorization.
	const (
		orgCollection        = "organizations"
		membershipCollection = "memberships"
		inviteCollection     = "invites"
	)
	// No IsSet guard: setDefaults guarantees a value.
	inviteExpiryDays := viper.GetInt("tenancy.invite_expiry_days")

	natsOptions := pbnats.DefaultOptions()
	natsOptions.OperatorName = viper.GetString("nats.operator_name")
	natsOptions.NATSServerURL = viper.GetString("nats.server_url")
	natsOptions.LogToConsole = viper.GetBool("nats.log_to_console")
	natsOptions.EncryptionKey = viper.GetString("nats.encryption_key")

	nebulaOptions := pbnebula.DefaultOptions()
	nebulaOptions.LogToConsole = viper.GetBool("nebula.log_to_console")
	nebulaOptions.DefaultCAValidityYears = viper.GetInt("nebula.default_ca_validity_years")
	nebulaOptions.EncryptionKey = viper.GetString("nebula.encryption_key")

	// At startup, not at the first key write, which would half-provision an org.
	validateEncryptionKey(natsOptions.EncryptionKey, "nats.encryption_key")
	validateEncryptionKey(nebulaOptions.EncryptionKey, "nebula.encryption_key")

	auditOptions := pbaudit.DefaultOptions()
	// audit.log_console is the legacy spelling, still honoured.
	auditOptions.LogToConsole = viper.GetBool("audit.log_to_console") || viper.GetBool("audit.log_console")

	// Never audit the root operator: its seed signs every JWT on the platform,
	// and hidden fields are the primary defence. The activity feed is derived
	// from audited writes, so auditing it would only double the volume.
	auditOptions.EventFilter = func(collectionName, eventType string) bool {
		return collectionName != "nats_system_operator" &&
			collectionName != hooks.ActivityCollection
	}

	auditOptions.SnapshotCollections = auditSnapshotCollections(
		orgCollection, membershipCollection,
		natsOptions.RoleCollectionName, nebulaOptions.NetworkCollectionName,
	)

	maxAgeStr := viper.GetString("audit.retention.max_age")
	maxRecords := viper.GetInt("audit.retention.max_records")
	if maxAgeStr != "" || maxRecords > 0 {
		retention := &pbaudit.RetentionPolicy{
			Interval: viper.GetString("audit.retention.interval"),
		}
		retention.MaxAge = requireDuration("audit.retention.max_age")
		if maxRecords > 0 {
			retention.MaxRecords = maxRecords
		}
		auditOptions.Retention = retention
	}

	// Before the libraries, so a cross-tenant relation is refused before pb-nats
	// signs a JWT from it.
	hooks.RegisterRelationTenancy(app)

	if err := pbaudit.Setup(app, auditOptions); err != nil {
		log.Fatalf("Failed to register audit setup: %v", err)
	}
	if err := pbnats.Setup(app, natsOptions); err != nil {
		log.Fatalf("Failed to register NATS setup: %v", err)
	}
	if err := pbnebula.Setup(app, nebulaOptions); err != nil {
		log.Fatalf("Failed to register Nebula setup: %v", err)
	}

	// `nats export` writes a leafnode listener and the hub's JetStream domain,
	// which edge sites need. The domain comes from nats.jetstream_domain, the
	// key GET /api/me/leaf-config serves as hub_domain: keep both on one key.
	natsCommandOptions := pbnats.DefaultCommandOptions()
	natsCommandOptions.AccountCollectionName = natsOptions.AccountCollectionName
	natsCommandOptions.EncryptionKey = natsOptions.EncryptionKey
	natsCommandOptions.DefaultLeafnodePort = 7422
	natsCommandOptions.DefaultJetstreamDomain = viper.GetString("nats.jetstream_domain")
	pbnats.RegisterCommandsWithOptions(app, natsCommandOptions)

	// Org codes (ADR 0002). Before provisioning, so the code exists before
	// anything else reads the record.
	hooks.RegisterOrgCode(app, orgCollection)

	// Thing and Location codes (ADR 0003).
	hooks.RegisterCodes(app)

	// locations.path (ADR 0004). After RegisterCodes, so a generated code is in
	// place before the path is built.
	hooks.RegisterLocationPath(app)

	// An unset NATS limit is -1, never 0, which a JWT reads as "none".
	hooks.RegisterNatsLimits(app, hooks.NatsLimitsOptions{
		RoleCollection:    natsOptions.RoleCollectionName,
		AccountCollection: natsOptions.AccountCollectionName,
	})

	// NATS account and Nebula CA for each new org.
	provisioningOptions := hooks.OrgProvisioningOptions{
		OrgCollection:                 orgCollection,
		NatsAccountCollection:         natsOptions.AccountCollectionName,
		NebulaCACollection:            nebulaOptions.CACollectionName,
		NatsMaxConnections:            viper.GetInt("nats.default_limits.max_connections"),
		NatsMaxSubscriptions:          viper.GetInt("nats.default_limits.max_subscriptions"),
		NatsMaxPayload:                viper.GetInt("nats.default_limits.max_payload"),
		NatsMaxJetStreamDiskStorage:   viper.GetInt64("nats.default_limits.max_jetstream_disk_storage"),
		NatsMaxJetStreamMemoryStorage: viper.GetInt64("nats.default_limits.max_jetstream_memory_storage"),
		NebulaDefaultCAValidityYears:  viper.GetInt("nebula.default_ca_validity_years"),
	}
	if err := provisioningOptions.Validate(); err != nil {
		log.Fatalf("❌ %v", err)
	}
	hooks.RegisterOrgProvisioning(app, provisioningOptions)

	// organizations.active withdraws the NATS account and nothing else. Must
	// follow RegisterOrgProvisioning: both bind AfterUpdateSuccess, and
	// provisioning (create-if-missing, before e.Next()) must run first so a
	// failed account exists by the time this looks for it.
	hooks.RegisterOrgActiveFlag(app, hooks.OrgActiveFlagOptions{
		OrgCollection:         orgCollection,
		NatsAccountCollection: natsOptions.AccountCollectionName,
	})

	// Managed orgs export a subtree into the operator account, org-prefixed.
	managedExportSubject := viper.GetString("nats.managed_export_subject")
	if !strings.HasSuffix(managedExportSubject, ".>") {
		log.Fatalf("❌ nats.managed_export_subject must end in '.>' (got %q)", managedExportSubject)
	}
	hooks.RegisterManagedOrgExports(app, hooks.ManagedOrgExportsOptions{
		OrgCollection:     orgCollection,
		AccountCollection: natsOptions.AccountCollectionName,
		ExportCollection:  natsOptions.ExportCollectionName,
		ImportCollection:  natsOptions.ImportCollectionName,
		ExportSubject:     managedExportSubject,
	})

	// Inventory events, published in the operator account and mapped into each
	// org by an export/import pair. After provisioning, which creates the
	// account. See hooks/inventory_events.go.
	inventoryLocalSubject := viper.GetString("nats.inventory_local_subject")
	if !strings.HasSuffix(inventoryLocalSubject, ".>") {
		log.Fatalf("❌ nats.inventory_local_subject must end in '.>' (got %q)", inventoryLocalSubject)
	}
	// Both subtrees live in the operator account as <prefix>.<code>.>. A shared
	// first token lets them overlap, exporting managed orgs' helpdesk traffic to
	// whichever tenant's code lined up.
	firstToken := func(s string) string { return strings.SplitN(s, ".", 2)[0] }
	if firstToken(inventoryLocalSubject) == firstToken(managedExportSubject) {
		log.Fatalf("❌ nats.inventory_local_subject (%q) and nats.managed_export_subject (%q) must start with different tokens",
			inventoryLocalSubject, managedExportSubject)
	}
	hooks.RegisterInventoryEvents(app, hooks.InventoryEventsOptions{
		Enabled:            viper.GetBool("nats.inventory_events_enabled"),
		OrgCollection:      orgCollection,
		AccountCollection:  natsOptions.AccountCollectionName,
		UserCollection:     natsOptions.UserCollectionName,
		RoleCollection:     natsOptions.RoleCollectionName,
		ExportCollection:   natsOptions.ExportCollectionName,
		ImportCollection:   natsOptions.ImportCollectionName,
		ThingCollection:    "things",
		LocationCollection: "locations",
		LocalSubject:       inventoryLocalSubject,
		ServerURL:          viper.GetString("nats.server_url"),
	})

	// Owner membership and invitations (hooks/org_membership.go,
	// hooks/invites.go). Registered after provisioning and managed exports so
	// they run first on org create. It calls e.Next(), so later binds still run.
	hooks.RegisterOrgMembership(app, hooks.OrgMembershipOptions{
		OrgCollection:        orgCollection,
		MembershipCollection: membershipCollection,
		UserCollection:       "users",
	})
	hooks.RegisterInvites(app, hooks.InviteOptions{
		MembershipCollection: membershipCollection,
		InviteCollection:     inviteCollection,
		UserCollection:       "users",
		ExpiryDays:           inviteExpiryDays,
	})

	// Operator-editable mail templates, seeded on first serve if missing and
	// never overwritten. Add one by appending it here (hooks/email_templates.go).
	hooks.RegisterEmailTemplates(app, hooks.OrgInviteEmail)

	// Tenant activity feed. See hooks/activity.go.
	hooks.RegisterActivity(app)

	// things.active: the authRule only stops new logins; this also invalidates
	// tokens and revokes the NATS identity, where the real capability lives.
	hooks.RegisterActiveFlag(app, hooks.ActiveFlagOptions{
		ThingCollection:      "things",
		NatsUserCollection:   natsOptions.UserCollectionName,
		NebulaHostCollection: nebulaOptions.HostCollectionName,
	})

	// The inventory read rules trust users.current_organization alone, so a
	// removed member's must be cleared.
	hooks.RegisterMembershipLifecycle(app, hooks.MembershipLifecycleOptions{
		MembershipCollection: membershipCollection,
		UserCollection:       "users",
	})

	// Credential rotation is a route because a rule cannot permit a write to
	// exactly one field. Reads need no route: the rules are row-scoped.
	hooks.RegisterCredentialRoutes(app, hooks.CredentialRoutesOptions{
		NatsUserCollection:   natsOptions.UserCollectionName,
		MembershipCollection: membershipCollection,
		ThingCollection:      "things",
	})

	// Signing-key operations. A rule cannot permit the trigger fields and forbid
	// the limits, so nats_accounts.updateRule is operator-only.
	hooks.RegisterNatsAccountRoutes(app, hooks.NatsAccountRoutesOptions{
		NatsAccountCollection: natsOptions.AccountCollectionName,
		MembershipCollection:  membershipCollection,
	})

	// CA rotation (nebula_ca.updateRule is operator-only) and the host
	// certificate audit, which must parse Nebula certificates server-side.
	hooks.RegisterNebulaRoutes(app, hooks.NebulaRoutesOptions{
		NebulaCACollection:      nebulaOptions.CACollectionName,
		NebulaNetworkCollection: nebulaOptions.NetworkCollectionName,
		NebulaHostCollection:    nebulaOptions.HostCollectionName,
		MembershipCollection:    membershipCollection,
	})

	// A Thing and its optional identities in one transaction, so a late failure
	// cannot orphan a signed credential.
	hooks.RegisterThingRoutes(app, hooks.ThingRoutesOptions{
		ThingCollection:         "things",
		OrgCollection:           orgCollection,
		MembershipCollection:    membershipCollection,
		NatsUserCollection:      natsOptions.UserCollectionName,
		NatsAccountCollection:   natsOptions.AccountCollectionName,
		NatsRoleCollection:      natsOptions.RoleCollectionName,
		NebulaHostCollection:    nebulaOptions.HostCollectionName,
		NebulaNetworkCollection: nebulaOptions.NetworkCollectionName,
	})

	// Browser NATS URLs, served at runtime so the embedded UI needs no rebuild
	// per operator. Validated here because viper splits an env value on
	// whitespace, so comma-separated URLs arrive as one entry.
	natsWebsocketURLs := viper.GetStringSlice("nats.websocket_urls")
	for _, u := range natsWebsocketURLs {
		// Before the scheme check: a comma-joined list still starts with one.
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

	// A bad leaf URL gives an edge box a remote that dials nowhere, and the only
	// symptom is a site that never appears.
	natsLeafURL := viper.GetString("nats.leaf_url")
	if natsLeafURL != "" &&
		!strings.HasPrefix(natsLeafURL, "nats-leaf://") &&
		!strings.HasPrefix(natsLeafURL, "tls://") {
		log.Fatalf("❌ nats.leaf_url must start with nats-leaf:// or tls:// (got %q).\n"+
			"       This is the address an EDGE box dials to reach this hub's leafnode port,\n"+
			"       not the nats:// address in nats.server_url.", natsLeafURL)
	}
	hooks.RegisterLeafConfigRoutes(app, hooks.LeafConfigRoutesOptions{
		ThingCollection:       "things",
		NatsUserCollection:    natsOptions.UserCollectionName,
		NatsAccountCollection: natsOptions.AccountCollectionName,
		HubLeafURL:            natsLeafURL,
		HubDomain:             viper.GetString("nats.jetstream_domain"),
	})

	// OnServe, not OnBootstrap: pb-nats calls e.Next() before it seeds the
	// operator, so a later bootstrap handler would run with no operator JWT.
	// The JWT publisher then spends a few seconds in bootstrap mode; its queue
	// drains on connect.
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
			// Refuse to serve credentials for a bus that is not there.
			return err
		}
		embeddedNATS = srv
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		embeddedNATS.Stop() // no-op when nil
		return e.Next()
	})

	// GET /api/ready and /metrics. PocketBase's /api/health serves 200 through
	// every failure that matters here (an untrusted operator, say). Nothing reads
	// inside an org's NATS account: this process holds no credential in one.
	hooks.RegisterObservability(app, hooks.ObservabilityOptions{
		Version:               version.Version,
		OrgCollection:         orgCollection,
		MembershipCollection:  membershipCollection,
		NatsAccountCollection: natsOptions.AccountCollectionName,
		NatsUserCollection:    natsOptions.UserCollectionName,
		NebulaHostCollection:  nebulaOptions.HostCollectionName,
		NebulaCACollection:    nebulaOptions.CACollectionName,
		AuditCollection:       auditOptions.CollectionName,
		NatsServerURL:         natsOptions.NATSServerURL,
		NatsWebsocketURLs:     natsWebsocketURLs,
		NatsEncryptionKey:     natsOptions.EncryptionKey,
		NebulaEncryptionKey:   nebulaOptions.EncryptionKey,
		// A closure: embeddedNATS is assigned later, in OnServe.
		EmbeddedNATS:   func() *natsd.Server { return embeddedNATS },
		ProbeInterval:  requireDuration("readiness.interval"),
		ProbeTimeout:   requireDuration("readiness.timeout"),
		MetricsEnabled: viper.GetBool("metrics.enabled"),
		MetricsToken:   viper.GetString("metrics.token"),
	})

	// Embedded UI with SPA fallback.
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		subFS, err := fs.Sub(embeddedFS, "pb_public")
		if err != nil {
			return err
		}

		// Branding overlay under /branding/*. Always registered, with empty
		// fallbacks for theme.css and branding.json, because index.html and the
		// SPA always request those two.
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
				// Vite puts a content hash in every name under assets/, so a
				// file there never changes and can be cached for good. Nothing
				// else gets the header: index.html and sw.js keep their names
				// across releases. Compression is the proxy's job, not ours.
				if strings.HasPrefix(path, "assets/") {
					e.Response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				return e.FileFS(subFS, path)
			}

			if strings.Contains(path, ".") {
				return e.NotFoundError("File not found", nil)
			}

			return e.FileFS(subFS, "index.html")
		})

		return e.Next()
	})

	addBootstrapCommand(app, orgCollection, membershipCollection, natsOptions)
	addInitCommand(app)

	// After every hook above, so the seed goes through the platform's own
	// provisioning, exactly as a console-created org does.
	demoseed.RegisterCommand(app)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// auditSnapshotCollections names the collections whose before/after values are
// written into audit_logs. Every other collection records only
// `changed_fields`, the names of the fields that moved.
//
// A snapshot copies every unhidden field, and some credentials are unhidden on
// purpose (nats_users.creds_file, nebula_hosts.config_yaml): row scoping
// protects them, and audit_logs has no row scoping. So list only collections
// whose diffs a human reads, and never one holding a credential, JWT,
// certificate or bearer token. It is an allowlist because a redaction list
// silently misses the next sensitive field. AUTHORIZATION.md has the long form.
//
// Left out: nats_users, nebula_hosts, nats_accounts, nebula_ca, invites
// (`token`), nats_account_exports/imports (account tokens), and
// nats_publish_queue. nats_roles is in: it holds no secret, and its diff
// records a change to what an identity may do on the bus.
//
// TestAuditSnapshotCollectionsHoldNoCredentials guards it against schema.json.
func auditSnapshotCollections(orgCollection, membershipCollection, natsRoleCollection, nebulaNetworkCollection string) []string {
	return []string{
		orgCollection,
		membershipCollection,
		"users",
		"things",
		"locations",
		"thing_types",
		"location_types",
		"thing_type_operations",
		natsRoleCollection,
		nebulaNetworkCollection,
		hooks.EmailTemplatesCollection,
	}
}
