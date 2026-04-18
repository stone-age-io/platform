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
				// The org-provisioning hook skips "System" by name, avoiding duplicate provisioning.
				if err := app.Save(org); err != nil {
					log.Fatalf("❌ Failed to create organization: %v", err)
				}
				log.Printf("✅ Created organization '%s'", orgName)
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

// linkSingleton adopts the single pre-existing NATS Account/User/Role
// (seeded by `pb-nats superuser upsert`) into the given organization.
// Fatal on ambiguous state — more than one record of the seed collection
// means bootstrap cannot safely pick one.
func linkSingleton(app *pocketbase.PocketBase, colName, orgID, label, nameField, orgName string) {
	col, _ := app.FindCollectionByNameOrId(colName)
	if col == nil {
		return
	}
	count, _ := app.CountRecords(col)
	switch {
	case count == 0:
		log.Printf("⚠️ Warning: No %ss found. Did you run 'superuser upsert' first?", label)
		return
	case count > 1:
		log.Fatalf("❌ Ambiguous state: Found %d %ss. Expected exactly 1 (System) for bootstrap.", count, label)
	}

	rec, err := app.FindFirstRecordByFilter(col.Id, "1=1")
	if err != nil {
		return
	}
	switch rec.GetString("organization") {
	case "":
		rec.Set("organization", orgID)
		if err := app.Save(rec); err != nil {
			log.Fatalf("❌ Failed to update %s: %v", label, err)
		}
		log.Printf("🔗 Linked %s '%s' to Organization '%s'", label, rec.GetString(nameField), orgName)
	case orgID:
		log.Printf("ℹ️ %s already linked to this organization", label)
	default:
		log.Printf("⚠️ %s is linked to a different organization. Skipping.", label)
	}
}
