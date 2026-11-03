package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"platform/internal/natsd"
)

// addInitCommand registers `init`: the four first-run commands, in the one order
// that works, so nobody has to know the order.
//
//  1. superuser upsert  -- the PocketBase superuser
//  2. migrate up        -- import schema.json, which carries the API rules
//  3. bootstrap         -- the platform operator, the System org, the operator org
//  4. nats export       -- operator.jwt and nats.conf, which `serve --nats` reads
//
// `bootstrap` writes fields that only exist after `migrate up`, and PocketBase
// silently drops writes to fields that do not exist. `nats export` needs the
// operator that `bootstrap` links up. So the order is load-bearing, and before
// this command it lived in two places: docker-entrypoint.sh and the docs.
//
// Steps 2 to 4 run as child processes of this same binary, exactly as the
// entrypoint ran them. Each command then bootstraps the app fresh, so nothing
// here depends on whether this process's collection cache saw the schema import.
// Step 1 runs in-process instead, so the password never appears in a process
// list: `superuser upsert` only takes it as an argument.
//
// nats.conf is the "already done" marker, as it was in the entrypoint: it is
// written last, and an operator who edits it keeps their edits. A run that fails
// part-way writes no marker, and every step is idempotent, so running init again
// is the recovery.
func addInitCommand(app *pocketbase.PocketBase) {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "First-run setup: superuser, schema, operator and NATS config, in order",
		Long: `Runs the four first-run commands in order: superuser upsert, migrate up,
bootstrap and nats export. Does nothing if the NATS config already exists.

The password comes from STONE_AGE_BOOTSTRAP_PASSWORD, or a prompt. The NATS
config is written where --nats-config points, so 'serve --nats' with the same
--nats-config finds it.`,
		Example: `  stone-age init --email admin@example.com --operator-org "Acme MSP"
  stone-age serve --nats`,
		Run: func(cmd *cobra.Command, args []string) {
			natsConfig, _ := cmd.Flags().GetString("nats-config")
			if _, err := os.Stat(natsConfig); err == nil {
				log.Printf("ℹ️ %s exists, so this deployment is already initialized. Nothing to do.", natsConfig)
				return
			}

			email, _ := cmd.Flags().GetString("email")
			orgName, _ := cmd.Flags().GetString("org")
			operatorOrgName, _ := cmd.Flags().GetString("operator-org")

			// Every question is asked here, up front, so the child processes
			// never need a terminal.
			if email == "" {
				fmt.Print("Admin Email: ")
				fmt.Scanln(&email)
			}
			email = strings.TrimSpace(email)
			if !strings.Contains(email, "@") {
				log.Fatalf("❌ %q is not a valid email address", email)
			}
			if !cmd.Flags().Changed("operator-org") {
				fmt.Print("Operator Organization (blank to skip): ")
				fmt.Scanln(&operatorOrgName)
			}
			password := os.Getenv("STONE_AGE_BOOTSTRAP_PASSWORD")
			if password == "" {
				password = promptNewPassword()
			}
			if len(password) < minPasswordLen {
				log.Fatalf("❌ Password must be at least %d characters (got %d)", minPasswordLen, len(password))
			}

			forward := forwardedFlags(cmd)

			log.Printf("==> 1/4 superuser upsert %s", email)
			upsertSuperuser(app, email, password)

			log.Print("==> 2/4 migrate up")
			runSelf(forward, nil, "migrate", "up")

			log.Print("==> 3/4 bootstrap")
			// The password goes by environment, which bootstrap already reads.
			// --operator-org is always passed, even empty, so it never prompts.
			runSelf(forward, []string{"STONE_AGE_BOOTSTRAP_PASSWORD=" + password},
				"bootstrap", "--email", email, "--org", orgName, "--operator-org", strings.TrimSpace(operatorOrgName))

			log.Print("==> 4/4 nats export")
			runSelf(forward, nil, "nats", "export", "--output", filepath.Dir(natsConfig))

			serve := "stone-age serve --nats"
			if natsConfig != natsd.DefaultConfigFile {
				serve += " --nats-config " + natsConfig
			}
			fmt.Printf("\n🚀 Initialized. Start the platform with:\n\n   %s\n\n", serve)
			fmt.Println("   With a separate nats-server, start it on the exported nats.conf and run 'stone-age serve' instead.")
		},
	}

	cmd.Flags().String("email", "", "Email address for the superuser and the platform operator")
	cmd.Flags().String("org", "System", "Name of the System organization")
	cmd.Flags().String("operator-org", "", "Name of the platform operator's organization (empty to skip)")

	app.RootCmd.AddCommand(cmd)
}

// forwardedFlags returns the root flags this run set, such as --dir and
// --config, so each child command works on the same data directory with the
// same config. --nats and --nats-config are serve's, and init has used the
// latter already.
func forwardedFlags(cmd *cobra.Command) []string {
	var out []string
	// VisitAll and Changed, not Visit: InheritedFlags() builds a new FlagSet,
	// and Visit on it sees nothing as set.
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed || f.Name == "nats" || f.Name == "nats-config" {
			return
		}
		out = append(out, "--"+f.Name+"="+f.Value.String())
	})
	return out
}

// runSelf runs this binary with args followed by the forwarded flags, and
// stops init if the step fails.
func runSelf(forward, env []string, args ...string) {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("❌ Cannot find this binary to run %q: %v", args[0], err)
	}
	c := exec.Command(exe, append(args, forward...)...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = append(os.Environ(), env...)
	if err := c.Run(); err != nil {
		log.Fatalf("❌ '%s' failed: %v. Fix it and run init again; every step is safe to repeat.", strings.Join(args, " "), err)
	}
}

// upsertSuperuser is `superuser upsert` without the password on the command line.
func upsertSuperuser(app core.App, email, password string) {
	col, err := app.FindCachedCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		log.Fatalf("❌ Failed to find the superusers collection: %v", err)
	}
	su, err := app.FindAuthRecordByEmail(col, email)
	if err != nil {
		su = core.NewRecord(col)
	}
	su.SetEmail(email)
	su.SetPassword(password)
	if err := app.Save(su); err != nil {
		log.Fatalf("❌ Failed to save superuser %q: %v", email, err)
	}
}
