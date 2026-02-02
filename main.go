package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	pbaudit "github.com/skeeeon/pb-audit"
	pbnats "github.com/skeeeon/pb-nats"
	pbnebula "github.com/skeeeon/pb-nebula"
	pbtenancy "github.com/skeeeon/pb-tenancy"
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

	// Nebula
	viper.SetDefault("nebula.ca_collection_name", "nebula_ca")
	viper.SetDefault("nebula.network_collection_name", "nebula_networks")
	viper.SetDefault("nebula.host_collection_name", "nebula_hosts")
	viper.SetDefault("nebula.log_to_console", true)

	// Audit
	viper.SetDefault("audit.collection_name", "audit_logs")
}

func main() {
	loadConfig()

	app := pocketbase.New()

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

	// Schema Import
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		if err := app.ImportCollectionsByMarshaledJSON(schemaJSON, false); err != nil {
			log.Printf("⚠️ Schema import warning: %v", err)
		} else {
			log.Println("✅ Schema imported from embedded schema.json")
		}

		return nil
	})

	// Infrastructure Provisioning Hook
	app.OnRecordAfterCreateSuccess(tenancyOptions.OrganizationsCollection).BindFunc(func(e *core.RecordEvent) error {
		// Prevent recursion if this is the System org we update manually in bootstrap
		if e.Record.GetString("name") == "System" {
			return e.Next()
		}

		log.Printf("🔗 Organization '%s' created, provisioning infrastructure...", e.Record.GetString("name"))

		// Create NATS Account
		natsCol, err := app.FindCollectionByNameOrId(natsOptions.AccountCollectionName)
		if err == nil {
			rec := core.NewRecord(natsCol)
			rec.Set("name", e.Record.GetString("name"))
			rec.Set("organization", e.Record.Id)
			rec.Set("active", true)
			rec.Set("max_connections", viper.GetInt("nats.default_limits.max_connections"))
			rec.Set("max_subscriptions", viper.GetInt("nats.default_limits.max_subscriptions"))
			rec.Set("max_data", -1)
			rec.Set("max_payload", viper.GetInt("nats.default_limits.max_payload"))
			rec.Set("max_jetstream_disk_storage", -1)
			rec.Set("max_jetstream_memory_storage", -1)

			if err := app.Save(rec); err != nil {
				log.Printf("❌ Failed to create NATS account: %v", err)
			} else {
				log.Printf("✅ Created NATS Account")
			}
		}

		// Create Nebula CA
		nebulaCol, err := app.FindCollectionByNameOrId(nebulaOptions.CACollectionName)
		if err == nil {
			rec := core.NewRecord(nebulaCol)
			rec.Set("name", e.Record.GetString("name")+" CA")
			rec.Set("organization", e.Record.Id)
			rec.Set("validity_years", viper.GetInt("nebula.default_ca_validity_years"))

			if err := app.Save(rec); err != nil {
				log.Printf("❌ Failed to create Nebula CA: %v", err)
			} else {
				log.Printf("✅ Created Nebula CA")
			}
		}

		return e.Next()
	})

	// 6. Serve Embedded UI with SPA Support
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		subFS, err := fs.Sub(embeddedFS, "pb_public")
		if err != nil {
			return err
		}

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

// addBootstrapCommand creates the 'bootstrap' CLI command
func addBootstrapCommand(app *pocketbase.PocketBase, orgColName, memberColName string, natsOpts pbnats.Options) {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Provision the initial System organization and admin user",
		Long: `Creates a default user, a 'System' organization, and links them.
Also links the pre-existing NATS System Account/User/Role (seeded by pb-nats/superuser upsert) to this organization.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 1. Gather Input
			email, _ := cmd.Flags().GetString("email")
			password, _ := cmd.Flags().GetString("password")
			orgName, _ := cmd.Flags().GetString("org")

			if email == "" {
				fmt.Print("Admin Email: ")
				fmt.Scanln(&email)
			}
			if password == "" {
				fmt.Print("Admin Password: ")
				bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
				password = string(bytePassword)
				fmt.Println()
			}

			// 2. Create/Get User
			usersCol, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				log.Fatal(err)
			}

			var user *core.Record
			existingUser, _ := app.FindAuthRecordByEmail("users", email)
			if existingUser != nil {
				log.Printf("👤 User '%s' already exists, using existing record.", email)
				user = existingUser
			} else {
				user = core.NewRecord(usersCol)
				user.Set("email", email)
				user.Set("emailVisibility", true)
				user.SetPassword(password)
				if err := app.Save(user); err != nil {
					log.Fatalf("❌ Failed to create user: %v", err)
				}
				log.Printf("✅ Created user '%s'", email)
			}

			// 3. Create/Get Organization
			orgCol, err := app.FindCollectionByNameOrId(orgColName)
			if err != nil {
				log.Fatal(err)
			}

			var org *core.Record
			existingOrg, _ := app.FindFirstRecordByFilter(orgColName, "name = {:name}", map[string]interface{}{"name": orgName})
			if existingOrg != nil {
				log.Printf("🏢 Organization '%s' already exists.", orgName)
				org = existingOrg
			} else {
				org = core.NewRecord(orgCol)
				org.Set("name", orgName)
				org.Set("active", true)
				org.Set("owner", user.Id)
				// We rely on the hook check for "System" name to skip duplicate provisioning
				if err := app.Save(org); err != nil {
					log.Fatalf("❌ Failed to create organization: %v", err)
				}
				log.Printf("✅ Created organization '%s'", orgName)
			}

			// 4. Link NATS System Account to Organization
			natsAccCol, _ := app.FindCollectionByNameOrId(natsOpts.AccountCollectionName)
			if natsAccCol != nil {
				count, _ := app.CountRecords(natsAccCol)
				if count == 1 {
					sysAcc, err := app.FindFirstRecordByFilter(natsAccCol.Id, "1=1")
					if err == nil {
						if sysAcc.GetString("organization") == "" {
							sysAcc.Set("organization", org.Id)
							if err := app.Save(sysAcc); err == nil {
								log.Printf("🔗 Linked NATS Account '%s' to Organization '%s'", sysAcc.GetString("name"), orgName)
							} else {
								log.Fatalf("❌ Failed to update NATS Account: %v", err)
							}
						} else if sysAcc.GetString("organization") == org.Id {
							log.Printf("ℹ️ NATS Account already linked to this organization")
						} else {
							log.Printf("⚠️ NATS Account is linked to a different organization. Skipping.")
						}
					}
				} else if count == 0 {
					log.Printf("⚠️ Warning: No NATS Accounts found. Did you run 'superuser upsert' first?")
				} else {
					log.Fatalf("❌ Ambiguous state: Found %d NATS Accounts. Expected exactly 1 (System) account for bootstrap.", count)
				}
			}

			// 5. Link NATS System User to Organization
			natsUserCol, _ := app.FindCollectionByNameOrId(natsOpts.UserCollectionName)
			if natsUserCol != nil {
				count, _ := app.CountRecords(natsUserCol)
				if count == 1 {
					sysUser, err := app.FindFirstRecordByFilter(natsUserCol.Id, "1=1")
					if err == nil {
						if sysUser.GetString("organization") == "" {
							sysUser.Set("organization", org.Id)
							if err := app.Save(sysUser); err == nil {
								log.Printf("🔗 Linked NATS User '%s' to Organization '%s'", sysUser.GetString("nats_username"), orgName)
							} else {
								log.Fatalf("❌ Failed to update NATS User: %v", err)
							}
						} else if sysUser.GetString("organization") == org.Id {
							log.Printf("ℹ️ NATS User already linked to this organization")
						} else {
							log.Printf("⚠️ NATS User is linked to a different organization. Skipping.")
						}
					}
				} else if count == 0 {
					log.Printf("⚠️ Warning: No NATS Users found. Did you run 'superuser upsert' first?")
				} else {
					log.Fatalf("❌ Ambiguous state: Found %d NATS Users. Expected exactly 1 (System) user for bootstrap.", count)
				}
			}

			// 6. Link NATS System Role to Organization
			natsRoleCol, _ := app.FindCollectionByNameOrId(natsOpts.RoleCollectionName)
			if natsRoleCol != nil {
				count, _ := app.CountRecords(natsRoleCol)
				if count == 1 {
					sysRole, err := app.FindFirstRecordByFilter(natsRoleCol.Id, "1=1")
					if err == nil {
						if sysRole.GetString("organization") == "" {
							sysRole.Set("organization", org.Id)
							if err := app.Save(sysRole); err == nil {
								log.Printf("🔗 Linked NATS Role '%s' to Organization '%s'", sysRole.GetString("name"), orgName)
							} else {
								log.Fatalf("❌ Failed to update NATS Role: %v", err)
							}
						} else if sysRole.GetString("organization") == org.Id {
							log.Printf("ℹ️ NATS Role already linked to this organization")
						} else {
							log.Printf("⚠️ NATS Role is linked to a different organization. Skipping.")
						}
					}
				} else if count == 0 {
					log.Printf("⚠️ Warning: No NATS Roles found. Did you run 'superuser upsert' first?")
				} else {
					log.Fatalf("❌ Ambiguous state: Found %d NATS Roles. Expected exactly 1 (System) role for bootstrap.", count)
				}
			}

			// 7. Provision Nebula CA for System Org (if missing)
			nebulaCol, _ := app.FindCollectionByNameOrId("nebula_ca")
			if nebulaCol != nil {
				existingCA, _ := app.FindFirstRecordByFilter(nebulaCol.Id, "organization = {:org}", map[string]interface{}{"org": org.Id})
				if existingCA == nil {
					ca := core.NewRecord(nebulaCol)
					ca.Set("name", orgName+" CA")
					ca.Set("organization", org.Id)
					ca.Set("validity_years", 10)
					if err := app.Save(ca); err == nil {
						log.Printf("✅ Provisioned Nebula CA for '%s'", orgName)
					} else {
						log.Printf("❌ Failed to provision Nebula CA: %v", err)
					}
				}
			}

			// 8. Create Membership (Owner)
			memberCol, err := app.FindCollectionByNameOrId(memberColName)
			if err != nil {
				log.Fatal(err)
			}

			existingMember, _ := app.FindFirstRecordByFilter(memberColName, "user = {:user} && organization = {:org}", map[string]interface{}{
				"user": user.Id,
				"org":  org.Id,
			})

			if existingMember == nil {
				member := core.NewRecord(memberCol)
				member.Set("user", user.Id)
				member.Set("organization", org.Id)
				member.Set("role", "owner")
				if err := app.Save(member); err != nil {
					log.Fatalf("❌ Failed to create membership: %v", err)
				}
				log.Printf("✅ Linked user to organization as Owner")
			}

			// 9. Update User's Current Org Context
			user.Set("current_organization", org.Id)
			if err := app.Save(user); err != nil {
				log.Printf("⚠️ Failed to set user context: %v", err)
			}

			fmt.Println("\n🚀 Bootstrap complete!")
		},
	}

	cmd.Flags().String("email", "", "Email address for the admin user")
	cmd.Flags().String("password", "", "Password for the admin user")
	cmd.Flags().String("org", "System", "Name of the initial organization")

	app.RootCmd.AddCommand(cmd)
}
