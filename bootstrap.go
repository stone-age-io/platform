package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	pbnats "github.com/skeeeon/pb-nats"

	"platform/hooks"
)

// PocketBase's own minimum for auth record passwords, checked before the
// prompts finish.
const minPasswordLen = 8

// Fallback codes for the two infrastructure orgs (ADR 0002), used only when the
// name yields no valid code. Codes are derived from the name, never pinned, so
// bootstrap and migrations/schema_update_org_code.go agree on the same org.
// These strings are not reserved: nothing resolves an org by them.
const (
	systemOrgCode   = "system"
	operatorOrgCode = "operator"
)

// orgCodeFor derives an organization code from a display name, falling back to
// one of the constants above when the name yields nothing the field will accept
// (no alphanumerics, or a single character). It must stay the derivation the
// backfill migration uses.
func orgCodeFor(name, fallback string) string {
	if code := hooks.Slugify(name); hooks.OrgCodePattern.MatchString(code) {
		return code
	}
	return fallback
}

// addBootstrapCommand registers the `bootstrap` CLI command, which provisions
// the initial System organization, admin user, and links the pre-existing NATS
// System records (seeded by `pb-nats superuser upsert`) to it.
func addBootstrapCommand(app *pocketbase.PocketBase, orgColName, memberColName string, natsOpts pbnats.Options) {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Provision the initial System organization and admin user",
		Long: `Creates a default user, a 'System' organization, and links them.
Also links the pre-existing NATS System Account/User/Role (seeded by pb-nats/superuser upsert) to this organization.`,
		Run: func(cmd *cobra.Command, args []string) {
			email, _ := cmd.Flags().GetString("email")
			password, _ := cmd.Flags().GetString("password")
			orgName, _ := cmd.Flags().GetString("org")
			operatorOrgName, _ := cmd.Flags().GetString("operator-org")

			if email == "" {
				fmt.Print("Admin Email: ")
				fmt.Scanln(&email)
			}
			email = strings.TrimSpace(email)
			if email == "" {
				log.Fatal("❌ An admin email is required (pass --email or answer the prompt)")
			}
			if !strings.Contains(email, "@") {
				log.Fatalf("❌ %q is not a valid email address", email)
			}
			// Only when the flag was left out: an explicit --operator-org ""
			// means skip, which is how `init` runs this with no terminal.
			if operatorOrgName == "" && !cmd.Flags().Changed("operator-org") {
				fmt.Print("Operator Organization (blank to skip): ")
				fmt.Scanln(&operatorOrgName)
			}
			operatorOrgName = strings.TrimSpace(operatorOrgName)

			usersCol, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				log.Fatalf("❌ Failed to find users collection: %v", err)
			}

			// Refuse to run before the migrations. The libraries' collections
			// exist on a virgin DB, but the platform flags (is_operator, the org
			// flags) come from schema.json, and PocketBase silently drops a write
			// to a missing field: bootstrap would "succeed" with no operator.
			requireSchemaFields(app, hooks.PlatformSchemaFields(orgColName, memberColName))

			// Resolve the user first: on a re-run it exists and no password is
			// needed.
			var user *core.Record
			existingUser, _ := app.FindAuthRecordByEmail("users", email)
			if existingUser != nil {
				log.Printf("👤 User '%s' already exists, using existing record.", email)
				user = existingUser
			} else {
				if password == "" {
					password = os.Getenv("STONE_AGE_BOOTSTRAP_PASSWORD")
				}
				if password == "" {
					password = promptNewPassword()
				}
				if len(password) < minPasswordLen {
					log.Fatalf("❌ Password must be at least %d characters (got %d)", minPasswordLen, len(password))
				}
				user = core.NewRecord(usersCol)
				user.Set("email", email)
				user.Set("emailVisibility", true)
				// Created out-of-band by the server owner, so no verification mail.
				user.Set("verified", true)
				user.SetPassword(password)
				if err := app.Save(user); err != nil {
					log.Fatalf("❌ Failed to create user: %v", err)
				}
				log.Printf("✅ Created user '%s'", email)
			}

			if !user.GetBool("is_operator") {
				user.Set("is_operator", true)
				// Fatal, not a warning: the summary below prints is_operator = true,
				// and the API has no other way to grant it.
				if err := app.Save(user); err != nil {
					log.Fatalf("❌ Failed to set operator flag: %v", err)
				}
				log.Printf("✅ User '%s' set as operator", email)
			}

			orgCol, err := app.FindCollectionByNameOrId(orgColName)
			if err != nil {
				log.Fatalf("❌ Failed to find organizations collection '%s': %v", orgColName, err)
			}

			var org *core.Record
			existingOrg, _ := app.FindFirstRecordByFilter(orgColName, "name = {:name}", dbx.Params{"name": orgName})
			if existingOrg != nil {
				log.Printf("🏢 Organization '%s' already exists.", orgName)
				org = existingOrg
			} else {
				org = core.NewRecord(orgCol)
				org.Set("name", orgName)
				// Set here rather than by RegisterOrgCode so an underivable name
				// falls back to systemOrgCode instead of failing bootstrap.
				org.Set("code", orgCodeFor(orgName, systemOrgCode))
				org.Set("active", true)
				org.Set("owner", user.Id)
				// Provisioning skips system orgs: this one adopts the pre-seeded
				// $SYS NATS records below.
				org.Set("is_system_org", true)
				if err := app.Save(org); err != nil {
					log.Fatalf("❌ Failed to create organization: %v", err)
				}
				log.Printf("✅ Created organization '%s'", orgName)
			}

			// Backfill for DBs bootstrapped before the flag existed.
			if !org.GetBool("is_system_org") {
				org.Set("is_system_org", true)
				if err := app.Save(org); err != nil {
					log.Printf("⚠️ Failed to set system org flag: %v", err)
				}
			}

			// Same for the code (pre-ADR 0002). Only when empty: a code is frozen.
			if org.GetString("code") == "" {
				org.Set("code", orgCodeFor(orgName, systemOrgCode))
				if err := app.Save(org); err != nil {
					log.Printf("⚠️ Failed to set system org code: %v", err)
				}
			}

			// Collected rather than fatal, so the operator sees every gap in one run.
			var problems []string

			// Order is load-bearing: link the account and role before the user
			// that points at them. hooks/relation_tenancy.go re-checks relations
			// when a record's organization moves, so a user adopted ahead of its
			// role is refused. TestBootstrapLinksNatsTargetsBeforeTheUserThatPointsAtThem
			// pins the order.
			if !linkSingleton(app, natsOpts.AccountCollectionName, org.Id, "NATS Account", "name", orgName) {
				problems = append(problems, "NATS Account not linked — new organizations cannot be provisioned")
			}
			if !linkSingleton(app, natsOpts.RoleCollectionName, org.Id, "NATS Role", "name", orgName) {
				problems = append(problems, "NATS Role not linked — permission templates are unavailable")
			}
			if !linkSingleton(app, natsOpts.UserCollectionName, org.Id, "NATS User", "nats_username", orgName) {
				problems = append(problems, "NATS User not linked — the console cannot connect to NATS")
			}

			nebulaCol, _ := app.FindCollectionByNameOrId("nebula_ca")
			if nebulaCol != nil {
				existingCA, _ := app.FindFirstRecordByFilter(nebulaCol.Id, "organization = {:org}", dbx.Params{"org": org.Id})
				if existingCA == nil {
					ca := core.NewRecord(nebulaCol)
					ca.Set("name", orgName+" CA")
					ca.Set("organization", org.Id)
					// The configured default, so bootstrap and provisioning agree.
					ca.Set("validity_years", viper.GetInt("nebula.default_ca_validity_years"))
					if err := app.Save(ca); err == nil {
						log.Printf("✅ Provisioned Nebula CA for '%s'", orgName)
					} else {
						log.Printf("❌ Failed to provision Nebula CA: %v", err)
						problems = append(problems, fmt.Sprintf("Nebula CA not provisioned: %v", err))
					}
				}
			}

			memberCol, err := app.FindCollectionByNameOrId(memberColName)
			if err != nil {
				log.Fatalf("❌ Failed to find memberships collection '%s': %v", memberColName, err)
			}

			ensureOwnerMembership(app, memberCol, user, org)

			// The operator's own org, whose NATS account is the hub for
			// cross-account service imports.
			operatorOrg := ensureOperatorOrg(app, orgCol, user, operatorOrgName)
			if operatorOrg != nil {
				ensureOwnerMembership(app, memberCol, user, operatorOrg)
			}

			// The operator org is the day-to-day context; System is only for
			// cluster-level NATS operations.
			currentOrg := org
			if operatorOrg != nil {
				currentOrg = operatorOrg
			}
			user.Set("current_organization", currentOrg.Id)
			if err := app.Save(user); err != nil {
				log.Printf("⚠️ Failed to set user context: %v", err)
			}

			// A missing operator org is not a problem: --operator-org is optional.

			if len(problems) > 0 {
				fmt.Println("\n⚠️  Bootstrap finished, but the platform is only partly provisioned:")
				for _, p := range problems {
					fmt.Printf("   • %s\n", p)
				}
				fmt.Println("\n   Fix the above and re-run — bootstrap is idempotent.")
				fmt.Println("   Missing NATS records usually mean 'superuser upsert' has not been run yet.")
				os.Exit(1)
			}

			fmt.Println("\n🚀 Bootstrap complete!")
			fmt.Printf("   Operator user : %s  (is_operator = true)\n", email)
			fmt.Printf("   System org    : %s\n", org.GetString("name"))
			if operatorOrg != nil {
				fmt.Printf("   Operator org  : %s  (default working context)\n", operatorOrg.GetString("name"))
			}
			fmt.Println("\n   Next: start the server with 'serve' and sign in with the address above.")
		},
	}

	cmd.Flags().String("email", "", "Email address for the admin user")
	cmd.Flags().String("password", "", "Password for the admin user (insecure: visible in shell history and process list — prefer the interactive prompt or STONE_AGE_BOOTSTRAP_PASSWORD)")
	cmd.Flags().String("org", "System", "Name of the initial organization")
	cmd.Flags().String("operator-org", "", "Name of the platform operator's organization (hub for shared services)")

	app.RootCmd.AddCommand(cmd)
}

// requireSchemaFields aborts unless every listed field exists. The check is
// shared with the `schema` readiness check (hooks.MissingFields); here it is
// fatal, because bootstrap has nothing useful to do without the fields.
func requireSchemaFields(app *pocketbase.PocketBase, want []hooks.CollectionFields) {
	missing := hooks.MissingFields(app, want)
	if len(missing) == 0 {
		return
	}

	log.Printf("❌ Database schema is not initialized — missing: %s", strings.Join(missing, ", "))
	log.Fatal("   Apply the migrations first, then re-run bootstrap:\n" +
		"      stone-age migrate up\n" +
		"   (Starting the server once with 'serve' also applies them.)")
}

// promptNewPassword reads a password twice from the terminal and returns it only
// if both entries match. Recovering from a typo takes another CLI run.
func promptNewPassword() string {
	fmt.Print("Admin Password: ")
	first, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		log.Fatalf("❌ Failed to read password: %v", err)
	}

	fmt.Print("Confirm Password: ")
	second, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		log.Fatalf("❌ Failed to read password: %v", err)
	}

	if string(first) != string(second) {
		log.Fatal("❌ Passwords do not match")
	}
	return string(first)
}

// linkSingleton adopts the single unlinked NATS Account/User/Role (seeded by
// `superuser upsert`) into the given organization. Other orgs' records are
// already linked, so they do not count. Fatal when more than one is unlinked;
// false when there was nothing to adopt.
func linkSingleton(app *pocketbase.PocketBase, colName, orgID, label, nameField, orgName string) bool {
	col, _ := app.FindCollectionByNameOrId(colName)
	if col == nil {
		log.Printf("⚠️ Collection %q not found; cannot link %s", colName, label)
		return false
	}

	if linked, _ := app.FindFirstRecordByFilter(col.Id, "organization = {:org}", dbx.Params{"org": orgID}); linked != nil {
		log.Printf("ℹ️ %s already linked to this organization", label)
		return true
	}

	unlinked, _ := app.FindRecordsByFilter(col.Id, "organization = ''", "", 2, 0)
	switch len(unlinked) {
	case 0:
		log.Printf("⚠️ Warning: No unlinked %s found. Did you run 'superuser upsert' first?", label)
		return false
	case 1:
		rec := unlinked[0]
		rec.Set("organization", orgID)
		if err := app.Save(rec); err != nil {
			log.Fatalf("❌ Failed to update %s: %v", label, err)
		}
		log.Printf("🔗 Linked %s '%s' to Organization '%s'", label, rec.GetString(nameField), orgName)
		return true
	default:
		log.Fatalf("❌ Ambiguous state: multiple unlinked %ss. Expected exactly 1 (System) for bootstrap.", label)
	}
	return false
}

// ensureOwnerMembership creates the admin user's Owner membership in the
// given organization if it doesn't already exist.
func ensureOwnerMembership(app *pocketbase.PocketBase, memberCol *core.Collection, user, org *core.Record) {
	existing, _ := app.FindFirstRecordByFilter(memberCol.Id, "user = {:user} && organization = {:org}", dbx.Params{
		"user": user.Id,
		"org":  org.Id,
	})
	if existing != nil {
		return
	}
	member := core.NewRecord(memberCol)
	member.Set("user", user.Id)
	member.Set("organization", org.Id)
	member.Set("role", "owner")
	if err := app.Save(member); err != nil {
		log.Fatalf("❌ Failed to create membership for '%s': %v", org.GetString("name"), err)
	}
	log.Printf("✅ Linked user to organization '%s' as Owner", org.GetString("name"))
}

// ensureOperatorOrg finds or creates the single operator organization, which
// the provisioning hook gives a NATS account and Nebula CA like any other.
// Returns nil when none exists and no name was given.
func ensureOperatorOrg(app *pocketbase.PocketBase, orgCol *core.Collection, user *core.Record, name string) *core.Record {
	if existing, _ := app.FindFirstRecordByFilter(orgCol.Id, "is_operator_org = true"); existing != nil {
		log.Printf("🏢 Operator organization '%s' already exists.", existing.GetString("name"))
		return existing
	}
	if name == "" {
		log.Printf("⚠️ No operator organization configured — cross-account service imports (helpdesk etc.) need one. Re-run bootstrap with --operator-org to add it.")
		return nil
	}

	// Adopt a same-named org instead of tripping the unique name index.
	org, _ := app.FindFirstRecordByFilter(orgCol.Id, "name = {:name}", dbx.Params{"name": name})
	if org == nil {
		org = core.NewRecord(orgCol)
		org.Set("name", name)
		org.Set("active", true)
		org.Set("owner", user.Id)
	}
	// Only when empty: an existing code is frozen (ADR 0002).
	if org.GetString("code") == "" {
		org.Set("code", orgCodeFor(name, operatorOrgCode))
	}
	org.Set("is_operator_org", true)
	if err := app.Save(org); err != nil {
		log.Fatalf("❌ Failed to create operator organization: %v", err)
	}
	log.Printf("✅ Created operator organization '%s'", name)
	return org
}
