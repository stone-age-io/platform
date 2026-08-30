package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	pbnats "github.com/skeeeon/pb-nats"

	"platform/hooks"
)

// PocketBase's own minimum for auth record passwords. Checked up front so a typo
// surfaces as a clear message instead of a validation error after the prompts.
const minPasswordLen = 8

// FALLBACK organization codes for the two infrastructure orgs (ADR 0002).
//
// These used to be pinned unconditionally, on the reasoning that a
// deterministic code is what reserves the two strings: the unique index on
// organizations.code makes them unavailable to every other org, so no
// reserved-word list is needed. The cost was not worth it. Both orgs are
// renameable -- --org-name and --operator-org -- and pinning meant bootstrap
// and migrations/schema_update_org_code.go DISAGREED about the same org: a
// fresh install of `--operator-org "816tech"` produced `operator`, while
// running the backfill migration over an existing database produced `816tech`.
// One deployment, two codes, decided by install history -- for a value that is
// immutable and gets printed on physical labels.
//
// Codes are now derived from the name like every other org's, and these are
// used only when the name yields nothing valid. So the two strings are no
// longer reserved by construction: an org named "Operator" can take
// `operator`. That is acceptable because nothing resolves an org by these
// values -- they are written here and never read anywhere -- and the unique
// index still stops two orgs sharing one code.
const (
	systemOrgCode   = "system"
	operatorOrgCode = "operator"
)

// orgCodeFor derives an organization code from a display name, falling back to
// one of the constants above when the name yields nothing the field will accept
// (no alphanumerics at all, or a single character). Deliberately the same
// derivation the backfill migration uses, which is the whole point: see above.
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
			if operatorOrgName == "" {
				fmt.Print("Operator Organization (blank to skip): ")
				fmt.Scanln(&operatorOrgName)
			}
			operatorOrgName = strings.TrimSpace(operatorOrgName)

			usersCol, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				log.Fatalf("❌ Failed to find users collection: %v", err)
			}

			// The platform's own fields (is_operator, the org flags) come from
			// schema.json via the migrations. The embedded libraries create their
			// collections during OnBootstrap, so those exist even on a virgin DB —
			// which makes it perfectly possible to run bootstrap before any
			// migration has been applied. Doing so used to "succeed" while losing
			// every platform flag, because record.Set() on a field that does not
			// exist is a silent no-op in PocketBase: no operator, no is_system_org,
			// no is_operator_org, and no error anywhere.
			requireSchemaFields(app, []collectionFields{
				{"users", []string{"is_operator"}},
				{orgColName, []string{"is_system_org", "is_operator_org"}},
				{memberColName, []string{"role"}},
			})

			// Resolve the user before asking for a password: on a re-run the
			// account already exists and the password is never used, so there is
			// no reason to make the operator type one.
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
				// No verification email is sent for the bootstrap account — it is
				// created out-of-band by whoever owns the server.
				user.Set("verified", true)
				user.SetPassword(password)
				if err := app.Save(user); err != nil {
					log.Fatalf("❌ Failed to create user: %v", err)
				}
				log.Printf("✅ Created user '%s'", email)
			}

			if !user.GetBool("is_operator") {
				user.Set("is_operator", true)
				if err := app.Save(user); err != nil {
					log.Printf("⚠️ Failed to set operator flag: %v", err)
				} else {
					log.Printf("✅ User '%s' set as operator", email)
				}
			}

			orgCol, err := app.FindCollectionByNameOrId(orgColName)
			if err != nil {
				log.Fatalf("❌ Failed to find organizations collection '%s': %v", orgColName, err)
			}

			var org *core.Record
			existingOrg, _ := app.FindFirstRecordByFilter(orgColName, "name = {:name}", map[string]interface{}{"name": orgName})
			if existingOrg != nil {
				log.Printf("🏢 Organization '%s' already exists.", orgName)
				org = existingOrg
			} else {
				org = core.NewRecord(orgCol)
				org.Set("name", orgName)
				// Derived from the name, same as every other org and same as the
				// backfill migration. Set explicitly rather than left to
				// RegisterOrgCode so that a name nothing can be derived from
				// falls back to systemOrgCode instead of failing the bootstrap.
				org.Set("code", orgCodeFor(orgName, systemOrgCode))
				org.Set("active", true)
				org.Set("owner", user.Id)
				// The org-provisioning hook skips system orgs, avoiding duplicate
				// provisioning — this org adopts the pre-seeded $SYS NATS records below.
				org.Set("is_system_org", true)
				if err := app.Save(org); err != nil {
					log.Fatalf("❌ Failed to create organization: %v", err)
				}
				log.Printf("✅ Created organization '%s'", orgName)
			}

			// Backfill the flag on re-runs against DBs bootstrapped before it existed.
			if !org.GetBool("is_system_org") {
				org.Set("is_system_org", true)
				if err := app.Save(org); err != nil {
					log.Printf("⚠️ Failed to set system org flag: %v", err)
				}
			}

			// Same backfill for the code, on a DB bootstrapped before ADR 0002.
			// Only when empty: a code that already exists is frozen. It agrees
			// with the migration now, so reaching here after one is a no-op.
			if org.GetString("code") == "" {
				org.Set("code", orgCodeFor(orgName, systemOrgCode))
				if err := app.Save(org); err != nil {
					log.Printf("⚠️ Failed to set system org code: %v", err)
				}
			}

			// Collected rather than fatal: each of these leaves the platform usable
			// but incompletely provisioned, and the operator needs to see all of
			// them at once instead of fixing one per run.
			var problems []string

			if !linkSingleton(app, natsOpts.AccountCollectionName, org.Id, "NATS Account", "name", orgName) {
				problems = append(problems, "NATS Account not linked — new organizations cannot be provisioned")
			}
			if !linkSingleton(app, natsOpts.UserCollectionName, org.Id, "NATS User", "nats_username", orgName) {
				problems = append(problems, "NATS User not linked — the console cannot connect to NATS")
			}
			if !linkSingleton(app, natsOpts.RoleCollectionName, org.Id, "NATS Role", "name", orgName) {
				problems = append(problems, "NATS Role not linked — permission templates are unavailable")
			}

			nebulaCol, _ := app.FindCollectionByNameOrId("nebula_ca")
			if nebulaCol != nil {
				existingCA, _ := app.FindFirstRecordByFilter(nebulaCol.Id, "organization = {:org}", map[string]interface{}{"org": org.Id})
				if existingCA == nil {
					ca := core.NewRecord(nebulaCol)
					ca.Set("name", orgName+" CA")
					ca.Set("organization", org.Id)
					// Was hardcoded to 10; use the configured default so bootstrap
					// and the org-provisioning hook agree.
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

			// Operator organization: the platform operator's own org, whose NATS
			// account acts as the hub for cross-account service imports (helpdesk
			// etc.). Provisioned like any regular org via the org-provisioning hook.
			operatorOrg := ensureOperatorOrg(app, orgCol, user, operatorOrgName)
			if operatorOrg != nil {
				ensureOwnerMembership(app, memberCol, user, operatorOrg)
			}

			// Default working context: the operator org is the day-to-day org;
			// the System org exists only for cluster-level NATS operations.
			currentOrg := org
			if operatorOrg != nil {
				currentOrg = operatorOrg
			}
			user.Set("current_organization", currentOrg.Id)
			if err := app.Save(user); err != nil {
				log.Printf("⚠️ Failed to set user context: %v", err)
			}

			// A missing operator org is NOT a problem here: --operator-org is
			// documented as optional and ensureOperatorOrg already warns.

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

// collectionFields names the fields bootstrap requires on one collection.
type collectionFields struct {
	collection string
	fields     []string
}

// requireSchemaFields aborts unless every listed field exists. Bootstrap writes
// platform flags that only exist once schema.json has been imported, and a write
// to a missing field is silently discarded — so without this check the operator
// gets a "Bootstrap complete!" that produced no platform operator at all.
func requireSchemaFields(app *pocketbase.PocketBase, want []collectionFields) {
	var missing []string
	for _, w := range want {
		col, err := app.FindCollectionByNameOrId(w.collection)
		if err != nil {
			missing = append(missing, fmt.Sprintf("collection %q", w.collection))
			continue
		}
		for _, f := range w.fields {
			if col.Fields.GetByName(f) == nil {
				missing = append(missing, w.collection+"."+f)
			}
		}
	}
	if len(missing) == 0 {
		return
	}

	log.Printf("❌ Database schema is not initialized — missing: %s", strings.Join(missing, ", "))
	log.Fatal("   Apply the migrations first, then re-run bootstrap:\n" +
		"      stone-age migrate up\n" +
		"   (Starting the server once with 'serve' also applies them.)")
}

// promptNewPassword reads a password twice from the terminal and returns it only
// if both entries match. The bootstrap password cannot be recovered or reset
// without another CLI run, so a silent typo here is expensive.
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

// linkSingleton adopts the single pre-existing NATS Account/User/Role
// (seeded by `pb-nats superuser upsert`) into the given organization.
// Only unlinked records are candidates — regular orgs provision their own
// records via hooks, so those must not make adoption look ambiguous.
// Fatal only when more than one unlinked record exists.
//
// Returns false when there was nothing to adopt, so the caller can report an
// incompletely provisioned platform instead of printing unqualified success.
func linkSingleton(app *pocketbase.PocketBase, colName, orgID, label, nameField, orgName string) bool {
	col, _ := app.FindCollectionByNameOrId(colName)
	if col == nil {
		log.Printf("⚠️ Collection %q not found; cannot link %s", colName, label)
		return false
	}

	if linked, _ := app.FindFirstRecordByFilter(col.Id, "organization = {:org}", map[string]interface{}{"org": orgID}); linked != nil {
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
	existing, _ := app.FindFirstRecordByFilter(memberCol.Id, "user = {:user} && organization = {:org}", map[string]interface{}{
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

// ensureOperatorOrg finds or creates the single operator organization.
// Saving a new org fires the org-provisioning hook, which creates its NATS
// account (the services hub) and Nebula CA like any regular organization.
// Returns nil when no operator org exists and no name was given.
func ensureOperatorOrg(app *pocketbase.PocketBase, orgCol *core.Collection, user *core.Record, name string) *core.Record {
	if existing, _ := app.FindFirstRecordByFilter(orgCol.Id, "is_operator_org = true"); existing != nil {
		log.Printf("🏢 Operator organization '%s' already exists.", existing.GetString("name"))
		return existing
	}
	if name == "" {
		log.Printf("⚠️ No operator organization configured — cross-account service imports (helpdesk etc.) need one. Re-run bootstrap with --operator-org to add it.")
		return nil
	}

	// A same-named org may predate the flag (e.g. created via the UI).
	// Adopt it instead of tripping the unique name index.
	org, _ := app.FindFirstRecordByFilter(orgCol.Id, "name = {:name}", map[string]interface{}{"name": name})
	if org == nil {
		org = core.NewRecord(orgCol)
		org.Set("name", name)
		org.Set("active", true)
		org.Set("owner", user.Id)
	}
	// Derived from --operator-org rather than pinned, so a fresh bootstrap and a
	// `migrate up` over an existing database land on the same code (ADR 0002).
	// Only when empty -- an existing code is frozen.
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
