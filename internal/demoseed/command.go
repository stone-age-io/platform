package demoseed

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// RegisterCommand wires the `demo-seed` subcommand onto the root command,
// following the helpdesk sibling's `seed-demo` shape.
//
// --confirm must stay required: this ships in the production binary, and it is
// the only thing stopping a stray run from injecting three fictional tenants,
// with signed credentials, into a live deployment.
//
//	./stone-age demo-seed --confirm
//	./stone-age demo-seed --confirm --things 400
func RegisterCommand(app *pocketbase.PocketBase) {
	cmd := &cobra.Command{
		Use:   "demo-seed",
		Short: "Populate this deployment with demo/showcase data (idempotent)",
		Long: "Seeds three organizations — each with locations, a type taxonomy, message " +
			"schemas, operations, things spanning devices, gateways, applications and " +
			"appliances, NATS roles and identities, a Nebula network with a lighthouse, " +
			"edge sites, and members covering all five console roles.\n\n" +
			"Runs offline: NATS account and user JWTs are signed locally and the claim " +
			"publishes queue until a server is reachable.\n\n" +
			"Idempotent: re-running tops up rather than duplicating, and a run that fails " +
			"partway heals on the next one.\n\n" +
			"NOT for production deployments — every record is fictional and every seeded " +
			"login shares a well-known password.",
		// Every error here says what to do next; the usage dump would bury it.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			count, _ := cmd.Flags().GetInt("things")
			seed, _ := cmd.Flags().GetInt64("seed")

			if !confirm {
				return fmt.Errorf(
					"refusing to seed without --confirm\n\n"+
						"This writes three fictional organizations, their signed NATS credentials\n"+
						"and Nebula certificates, and %d users sharing the password %q,\n"+
						"into this deployment. Never run it against a production Control Plane.",
					len(people), DemoPassword)
			}

			if err := preflight(app); err != nil {
				return err
			}

			res, err := Run(app, Options{
				Things: count,
				Seed:   seed,
				Log:    func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
			})
			if res != nil {
				fmt.Print("\n", res.Summary())
			}
			if err != nil {
				return err
			}

			fmt.Printf("\ndone — every demo login uses password %q\n", DemoPassword)
			fmt.Println("try:  " + people[0].Email + "  (owner of " + orgs[0].Name + ")")
			fmt.Println("      casey@msp.example    (platform operator, member of all three)")
			return nil
		},
	}

	cmd.Flags().Bool("confirm", false, "required: acknowledge this writes fictional data into this deployment")
	cmd.Flags().Int("things", 120, "total things to converge on across all organizations, curated fixtures included")
	cmd.Flags().Int64("seed", 20260831, "PRNG seed; changing it regenerates different (but still stable) bulk")

	app.RootCmd.AddCommand(cmd)
}

// preflight refuses to run against a database the seed would silently corrupt,
// and says which step is missing.
//
// Like `bootstrap`, it must run after `migrate up`: PocketBase silently drops
// writes to missing fields, so the orgs would save without a `code`, the
// fixtures' join key, which is immutable once written.
func preflight(app core.App) error {
	required := []struct {
		collection string
		fields     []string
	}{
		{"organizations", []string{"code", "managed"}},
		{"things", []string{"code", "active"}},
		{"memberships", []string{"role"}},
	}
	for _, r := range required {
		col, err := app.FindCollectionByNameOrId(r.collection)
		if err != nil {
			return fmt.Errorf("collection %q does not exist — run `migrate up` first", r.collection)
		}
		for _, f := range r.fields {
			if col.Fields.GetByName(f) == nil {
				return fmt.Errorf(
					"%s.%s does not exist — schema.json has not been imported.\n"+
						"Run `migrate up` before seeding; PocketBase drops writes to missing\n"+
						"fields silently, so seeding now would report success and write nothing.",
					r.collection, f)
			}
		}
	}

	// pb-nats signs every JWT with the operator seed. Without one the records
	// still save, and the failure surfaces later as credentials no server accepts.
	// `superuser upsert` creates the operator, not `bootstrap`; the seed does not
	// need the System org.
	op, err := app.FindFirstRecordByFilter("nats_system_operator", "id != ''", nil)
	if err != nil || op == nil || op.GetString("jwt") == "" {
		return fmt.Errorf(
			"no NATS operator with a JWT — nothing has seeded the NATS chain of trust.\n" +
				"Run, in this order:\n" +
				"  ./stone-age superuser upsert <email> <password>\n" +
				"  ./stone-age migrate up\n" +
				"  ./stone-age bootstrap --email <email> --org \"System\" --operator-org \"<your company>\"\n" +
				"then seed. Without the operator, every credential this seeds would be unsigned.")
	}

	// A warning, not a refusal. The managed orgs import their `helpdesk.>` into
	// the operator org's account; with no operator org that hook logs a failure
	// per org and everything else seeds normally.
	hub, err := app.FindFirstRecordByFilter("organizations", "is_operator_org = true", nil)
	if err != nil || hub == nil {
		fmt.Println(
			"note: no operator organization exists, so the managed-org export/import pair\n" +
				"      will not be wired up for northwind and galewind. Everything else seeds\n" +
				"      normally. To get that half too, run `bootstrap --operator-org \"<your\n" +
				"      company>\"` first, then re-run this command — it is idempotent.")
	}

	return nil
}
