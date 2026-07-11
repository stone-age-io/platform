package main

import (
	"fmt"
	"log"
	"syscall"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	pbnats "github.com/skeeeon/pb-nats"
)

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
			if password == "" {
				fmt.Print("Admin Password: ")
				bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
				password = string(bytePassword)
				fmt.Println()
			}
			if operatorOrgName == "" {
				fmt.Print("Operator Organization (blank to skip): ")
				fmt.Scanln(&operatorOrgName)
			}

			usersCol, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				log.Fatalf("❌ Failed to find users collection: %v", err)
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

			linkSingleton(app, natsOpts.AccountCollectionName, org.Id, "NATS Account", "name", orgName)
			linkSingleton(app, natsOpts.UserCollectionName, org.Id, "NATS User", "nats_username", orgName)
			linkSingleton(app, natsOpts.RoleCollectionName, org.Id, "NATS Role", "name", orgName)

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

			fmt.Println("\n🚀 Bootstrap complete!")
		},
	}

	cmd.Flags().String("email", "", "Email address for the admin user")
	cmd.Flags().String("password", "", "Password for the admin user")
	cmd.Flags().String("org", "System", "Name of the initial organization")
	cmd.Flags().String("operator-org", "", "Name of the platform operator's organization (hub for shared services)")

	app.RootCmd.AddCommand(cmd)
}

// linkSingleton adopts the single pre-existing NATS Account/User/Role
// (seeded by `pb-nats superuser upsert`) into the given organization.
// Only unlinked records are candidates — regular orgs provision their own
// records via hooks, so those must not make adoption look ambiguous.
// Fatal only when more than one unlinked record exists.
func linkSingleton(app *pocketbase.PocketBase, colName, orgID, label, nameField, orgName string) {
	col, _ := app.FindCollectionByNameOrId(colName)
	if col == nil {
		return
	}

	if linked, _ := app.FindFirstRecordByFilter(col.Id, "organization = {:org}", map[string]interface{}{"org": orgID}); linked != nil {
		log.Printf("ℹ️ %s already linked to this organization", label)
		return
	}

	unlinked, _ := app.FindRecordsByFilter(col.Id, "organization = ''", "", 2, 0)
	switch len(unlinked) {
	case 0:
		log.Printf("⚠️ Warning: No unlinked %s found. Did you run 'superuser upsert' first?", label)
	case 1:
		rec := unlinked[0]
		rec.Set("organization", orgID)
		if err := app.Save(rec); err != nil {
			log.Fatalf("❌ Failed to update %s: %v", label, err)
		}
		log.Printf("🔗 Linked %s '%s' to Organization '%s'", label, rec.GetString(nameField), orgName)
	default:
		log.Fatalf("❌ Ambiguous state: multiple unlinked %ss. Expected exactly 1 (System) for bootstrap.", label)
	}
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
	org.Set("is_operator_org", true)
	if err := app.Save(org); err != nil {
		log.Fatalf("❌ Failed to create operator organization: %v", err)
	}
	log.Printf("✅ Created operator organization '%s'", name)
	return org
}
