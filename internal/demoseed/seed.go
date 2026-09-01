// Package demoseed populates a Control Plane with demo/showcase data.
//
// It runs IN-PROCESS against core.App rather than through an HTTP client, which
// buys three things an external script could not have:
//
//   - The provisioning hooks fire exactly as they do in production. Creating an
//     organization mints its NATS account and Nebula CA; creating a leaf node
//     mints that leaf's NATS user. The seed goes in THROUGH that machinery, not
//     around it, so a demo instance is indistinguishable from one built by hand
//     and a hook that breaks breaks the seed.
//   - No auth dance and no org switching. Every API rule here is scoped by
//     users.current_organization, so an HTTP seeder would have to log in as a
//     member of each tenant and PATCH its own context between phases.
//   - It cannot drift from the schema. A migration that breaks these fixtures
//     breaks `go test ./...`.
//
// It needs NO running NATS server. pb-nats generates account and user keys
// locally and signs the JWTs with the account seed; reaching the bus is a
// separate step that queues in nats_publish_queue and drains whenever a server
// appears. A demo seeded offline is complete the moment `serve --nats` starts.
//
// Seeding is IDEMPOTENT. Everything is found-or-created by a natural key —
// within an organization, the `code` that ADR 0002 already made unique — and
// generation is driven by a fixed-seed PRNG, so re-running converges rather than
// duplicating. Children reconcile individually rather than being gated on "was
// the parent new", because that gate leaves records permanently childless if a
// run dies partway.
package demoseed

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"sort"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Options controls the size of the generated bulk. The curated fixtures are
// always seeded in full; Things is a floor for the total, not a cap on them.
type Options struct {
	// Things is the total thing count to converge on across all organizations,
	// curated ones included.
	Things int

	// Seed makes generation reproducible. Keeping it stable is what makes
	// re-runs idempotent, so there is rarely a reason to change it.
	Seed int64

	// Log receives progress lines. Nil discards them.
	Log func(string, ...any)
}

// Result reports what the run did.
type Result struct {
	Created map[string]int
	Matched map[string]int
}

func (r *Result) mark(collection string, created bool) {
	if created {
		r.Created[collection]++
		return
	}
	r.Matched[collection]++
}

// Summary renders the per-collection counts in a stable order.
func (r *Result) Summary() string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]int{r.Created, r.Matched} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-24s %4d created  %4d existing\n", k, r.Created[k], r.Matched[k])
	}
	return b.String()
}

// seeder carries the resolved id maps between phases. Every map is keyed by the
// fixture's own natural key, never by a PocketBase id, so a phase reads like the
// fixture file it consumes.
type seeder struct {
	app  core.App
	opts Options
	rng  *mathrand.Rand
	res  *Result

	orgs      map[string]string // org code -> org id
	accounts  map[string]string // org code -> nats_accounts id
	cas       map[string]string // org code -> nebula_ca id
	networks  map[string]string // org code -> nebula_networks id
	users     map[string]string // person email -> users id
	roles     map[string]string // "<org>:<role name>" -> nats_roles id
	locTypes  map[string]string // "<org>:<code>" -> location_types id
	locations map[string]string // "<org>:<code>" -> locations id
	schemas   map[string]string // schemaKey() -> message_schemas id
	ops       map[string]string // "<org>:<name>" -> thing_type_operations id
	thingType map[string]string // "<org>:<code>" -> thing_types id
	things    map[string]string // "<org>:<code>" -> things id
}

// Run seeds the app. Safe to call repeatedly.
func Run(app core.App, opts Options) (*Result, error) {
	if opts.Things <= 0 {
		opts.Things = 120
	}
	if opts.Seed == 0 {
		opts.Seed = 20260831
	}
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}

	s := &seeder{
		app: app, opts: opts,
		rng: mathrand.New(mathrand.NewSource(opts.Seed)),
		res: &Result{Created: map[string]int{}, Matched: map[string]int{}},

		orgs: map[string]string{}, accounts: map[string]string{}, cas: map[string]string{},
		networks: map[string]string{}, users: map[string]string{}, roles: map[string]string{},
		locTypes: map[string]string{}, locations: map[string]string{}, schemas: map[string]string{},
		ops: map[string]string{}, thingType: map[string]string{}, things: map[string]string{},
	}

	// Order is load-bearing in three places, and only these three:
	//   people before organizations — organizations.owner is required.
	//   roles before memberships    — a membership links a console NATS identity.
	//   locations + types before things — a Thing points at both.
	steps := []struct {
		name string
		fn   func() error
	}{
		{"people", s.seedPeople},
		{"organizations", s.seedOrgs},
		{"NATS roles", s.seedNatsRoles},
		{"memberships", s.seedMemberships},
		{"location types", s.seedLocationTypes},
		{"locations", s.seedLocations},
		{"message schemas", s.seedMessageSchemas},
		{"operations", s.seedOperations},
		{"thing types", s.seedThingTypes},
		{"overlay networks", s.seedNetworks},
		{"things", s.seedThings},
		{"edge sites", s.seedLeafNodes},
	}
	for _, step := range steps {
		opts.Log("seeding %s...", step.name)
		if err := step.fn(); err != nil {
			return s.res, fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return s.res, nil
}

// ------------------------------------------------------------------ helpers

// ensure is the found-or-create primitive. `fill` runs only when the record is
// new, so hand-edits to demo data survive a re-run — someone who renames a Thing
// to try something out does not have it silently reverted by the next seed.
func (s *seeder) ensure(collection, filter string, params dbx.Params, fill func(*core.Record)) (*core.Record, bool, error) {
	if existing, err := s.app.FindFirstRecordByFilter(collection, filter, params); err == nil && existing != nil {
		s.res.mark(collection, false)
		return existing, false, nil
	}
	col, err := s.app.FindCollectionByNameOrId(collection)
	if err != nil {
		return nil, false, fmt.Errorf("find collection %s: %w", collection, err)
	}
	rec := core.NewRecord(col)
	fill(rec)
	if err := s.app.Save(rec); err != nil {
		return nil, false, fmt.Errorf("save %s: %w", collection, err)
	}
	s.res.mark(collection, true)
	return rec, true, nil
}

// secret returns a hex-encoded random string. Used for the passwords on auth
// records nobody logs into — a Thing, a NATS identity, a Nebula host. Their real
// credential is the signed JWT or certificate, never this.
func secret(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is not a condition a demo seeder can sensibly
		// recover from, and returning a predictable fallback would be worse.
		panic("demoseed: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// pickInt is a deterministic helper for generated metadata.
func (s *seeder) pickInt(lo, hi int) int { return lo + s.rng.Intn(hi-lo+1) }

// ------------------------------------------------------------------- people

func (s *seeder) seedPeople() error {
	for _, p := range people {
		rec, _, err := s.ensure("users", "email = {:e}", dbx.Params{"e": p.Email}, func(r *core.Record) {
			r.Set("email", p.Email)
			r.Set("name", p.Name)
			r.Set("emailVisibility", true)
			// No verification mail: these accounts are created out of band by
			// whoever ran the command, exactly like the bootstrap admin.
			r.Set("verified", true)
			r.SetPassword(DemoPassword)
			// is_operator is NOT grantable through the API (users.updateRule
			// freezes it). It is set here because app.Save bypasses rules — the
			// same privilege `bootstrap` uses, and the reason this command is
			// gated behind --confirm.
			if p.Operator {
				r.Set("is_operator", true)
			}
		})
		if err != nil {
			return err
		}
		s.users[p.Email] = rec.Id
	}
	return nil
}

// ------------------------------------------------------------ organizations

func (s *seeder) seedOrgs() error {
	for _, o := range orgs {
		ownerID, err := s.ownerFor(o.Code)
		if err != nil {
			return err
		}

		rec, _, err := s.ensure("organizations", "code = {:c}", dbx.Params{"c": o.Code}, func(r *core.Record) {
			r.Set("name", o.Name)
			// Set explicitly rather than left to RegisterOrgCode's derivation:
			// the code is the join key with the helpdesk's demo data, so it must
			// be what the fixture says and not what a slug of the name happens
			// to produce.
			r.Set("code", o.Code)
			r.Set("description", o.Description)
			r.Set("active", true)
			r.Set("owner", ownerID)
			r.Set("managed", o.Managed)
		})
		if err != nil {
			return err
		}
		s.orgs[o.Code] = rec.Id

		// RegisterOrgProvisioning created these on the save above (or on an
		// earlier run). Resolved here rather than in each consumer so a missing
		// one is reported once, against the org that is missing it.
		acct, err := s.app.FindFirstRecordByFilter("nats_accounts",
			"organization = {:o} && active = true", dbx.Params{"o": rec.Id})
		if err != nil || acct == nil {
			return fmt.Errorf("organization %q has no active NATS account; "+
				"the org-provisioning hook did not run (is this seeder wired into a binary that registers it?)", o.Code)
		}
		s.accounts[o.Code] = acct.Id

		ca, err := s.app.FindFirstRecordByFilter("nebula_ca",
			"organization = {:o}", dbx.Params{"o": rec.Id})
		if err != nil || ca == nil {
			return fmt.Errorf("organization %q has no Nebula CA; the org-provisioning hook did not run", o.Code)
		}
		s.cas[o.Code] = ca.Id
	}
	return nil
}

// ownerFor resolves the person the fixtures name as an org's owner. An org
// cannot be saved without one, so an org with no owner in `people` is a fixture
// bug and is reported as such rather than defaulting to whoever is handy.
func (s *seeder) ownerFor(orgCode string) (string, error) {
	for _, p := range people {
		if p.Roles[orgCode] == "owner" {
			id, ok := s.users[p.Email]
			if !ok {
				return "", fmt.Errorf("owner %q for org %q was not seeded", p.Email, orgCode)
			}
			return id, nil
		}
	}
	return "", fmt.Errorf("no person in the fixtures owns org %q", orgCode)
}

// --------------------------------------------------------------- NATS roles

func (s *seeder) seedNatsRoles() error {
	for _, o := range orgs {
		orgID := s.orgs[o.Code]
		for _, t := range roleTemplates {
			rec, _, err := s.ensure("nats_roles", "organization = {:o} && name = {:n}",
				dbx.Params{"o": orgID, "n": t.Name}, func(r *core.Record) {
					r.Set("name", t.Name)
					r.Set("description", t.Description)
					r.Set("organization", orgID)
					r.Set("is_default", t.IsDefault)
					r.Set("max_subscriptions", t.MaxSubscriptions)
					r.Set("max_payload", t.MaxPayload)
					r.Set("max_data", -1)
					r.Set("publish_permissions", t.Publish)
					r.Set("subscribe_permissions", t.Subscribe)
					r.Set("publish_deny_permissions", orEmpty(t.PublishDeny))
					r.Set("subscribe_deny_permissions", []string{})
				})
			if err != nil {
				return err
			}
			s.roles[o.Code+":"+t.Name] = rec.Id
		}
	}
	return nil
}

// orEmpty keeps a nil slice out of a JSON column, where it would serialize as
// null and read back differently from the empty list every other role carries.
func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// -------------------------------------------------------------- memberships

func (s *seeder) seedMemberships() error {
	for _, p := range people {
		// Deterministic order: map iteration is not, and a membership's NATS
		// identity is named after the org, so an unstable order would produce a
		// different (still valid) set of identities on each fresh run.
		codes := make([]string, 0, len(p.Roles))
		for c := range p.Roles {
			codes = append(codes, c)
		}
		sort.Strings(codes)

		for _, orgCode := range codes {
			role := p.Roles[orgCode]
			orgID, ok := s.orgs[orgCode]
			if !ok {
				return fmt.Errorf("person %q references unknown org %q", p.Email, orgCode)
			}
			userID := s.users[p.Email]

			// A console NATS identity per (person, org). Minted before the
			// membership so the relation can be set in one save. The NATS role
			// is derived from the CONSOLE role, per membership — see
			// natsRoleForConsoleRole.
			natsUserID, err := s.consoleNatsUser(p, orgCode, orgID, natsRoleForConsoleRole(role))
			if err != nil {
				return err
			}

			rec, _, err := s.ensure("memberships", "user = {:u} && organization = {:o}",
				dbx.Params{"u": userID, "o": orgID}, func(r *core.Record) {
					r.Set("user", userID)
					r.Set("organization", orgID)
					r.Set("role", role)
					r.Set("nats_user", natsUserID)
				})
			if err != nil {
				return err
			}

			// An owner's membership is NOT created here. pb-tenancy's
			// autoCreateOwnerMembership already made it when the organization was
			// saved, so `ensure` above finds it and its `fill` never runs — which
			// left every owner without the console NATS identity the rest of this
			// demo depends on.
			//
			// Reconciled only when EMPTY, not overwritten. An operator who points
			// their own membership at a different identity has made a choice, and
			// a re-seed should not undo it; a blank one is the hook's gap, not a
			// choice.
			if rec.GetString("nats_user") == "" {
				rec.Set("nats_user", natsUserID)
				if err := s.app.Save(rec); err != nil {
					return fmt.Errorf("link NATS identity to membership %s: %w", rec.Id, err)
				}
			}
		}

		// Land everyone somewhere, so a demo login is not met with an empty
		// console and an org switcher they have to find first.
		if len(codes) > 0 {
			if err := s.setCurrentOrg(s.users[p.Email], s.orgs[codes[0]]); err != nil {
				return err
			}
		}
	}
	return nil
}

// consoleNatsUser mints the NATS identity a person's browser connects with. The
// name mirrors the convention hooks/thing_routes.go uses for a Thing: the local
// half identifies the principal, the domain half is the org's immutable code.
func (s *seeder) consoleNatsUser(p personFixture, orgCode, orgID, natsRole string) (string, error) {
	local := strings.SplitN(p.Email, "@", 2)[0]
	username := "console-" + local
	email := fmt.Sprintf("%s@%s.nats.local", username, orgCode)

	roleID, ok := s.roles[orgCode+":"+natsRole]
	if !ok {
		return "", fmt.Errorf("person %q needs NATS role %q, which is not seeded in org %q",
			p.Email, natsRole, orgCode)
	}

	rec, _, err := s.ensure("nats_users", "email = {:e}", dbx.Params{"e": email}, func(r *core.Record) {
		r.Set("nats_username", username)
		r.Set("email", email)
		r.Set("emailVisibility", true)
		r.SetPassword(secret(32))
		r.Set("account_id", s.accounts[orgCode])
		r.Set("role_id", roleID)
		r.Set("organization", orgID)
		r.Set("active", true)
	})
	if err != nil {
		return "", err
	}
	return rec.Id, nil
}

// setCurrentOrg writes users.current_organization, which is the field every
// inventory read rule is scoped by. Skipped when already set, so a re-run does
// not yank a human out of the org they were last looking at.
func (s *seeder) setCurrentOrg(userID, orgID string) error {
	rec, err := s.app.FindRecordById("users", userID)
	if err != nil {
		return err
	}
	if rec.GetString("current_organization") != "" {
		return nil
	}
	rec.Set("current_organization", orgID)
	return s.app.Save(rec)
}

// ----------------------------------------------------------- location types

func (s *seeder) seedLocationTypes() error {
	for _, t := range locationTypes {
		orgID := s.orgs[t.Org]
		rec, _, err := s.ensure("location_types", "organization = {:o} && code = {:c}",
			dbx.Params{"o": orgID, "c": t.Code}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("code", t.Code)
				r.Set("name", t.Name)
				r.Set("description", t.Description)
				if t.Schema != nil {
					r.Set("metadata_schema", t.Schema)
				}
			})
		if err != nil {
			return err
		}
		s.locTypes[t.Org+":"+t.Code] = rec.Id
	}
	return nil
}

// ------------------------------------------------------------------ locations

func (s *seeder) seedLocations() error {
	// Pass 1: create without `parent`, because a parent may appear later in the
	// fixture list.
	for _, l := range locations {
		orgID := s.orgs[l.Org]
		typeID := s.locTypes[l.Org+":"+l.Type]
		if typeID == "" {
			return fmt.Errorf("location %q references unknown location type %q", l.Code, l.Type)
		}

		rec, _, err := s.ensure("locations", "organization = {:o} && code = {:c}",
			dbx.Params{"o": orgID, "c": l.Code}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("code", l.Code)
				r.Set("name", l.Name)
				r.Set("description", l.Description)
				r.Set("type", typeID)
				if l.Lat != 0 || l.Lon != 0 {
					r.Set("coordinates", types.GeoPoint{Lat: l.Lat, Lon: l.Lon})
				}
				if l.Metadata != nil {
					r.Set("metadata", l.Metadata)
				}
			})
		if err != nil {
			return err
		}
		s.locations[l.Org+":"+l.Code] = rec.Id
	}

	// Pass 2: link parents. Reconciled rather than set-on-create, so a fixture
	// that gains a parent later takes effect on the next run.
	for _, l := range locations {
		if l.Parent == "" {
			continue
		}
		parentID, ok := s.locations[l.Org+":"+l.Parent]
		if !ok {
			return fmt.Errorf("location %q references unknown parent %q", l.Code, l.Parent)
		}
		rec, err := s.app.FindRecordById("locations", s.locations[l.Org+":"+l.Code])
		if err != nil {
			return err
		}
		if rec.GetString("parent") == parentID {
			continue
		}
		rec.Set("parent", parentID)
		if err := s.app.Save(rec); err != nil {
			return fmt.Errorf("link parent of %s: %w", l.Code, err)
		}
	}
	return nil
}

// ------------------------------------------------------------ message schemas

func (s *seeder) seedMessageSchemas() error {
	for _, sc := range messageSchemas {
		orgID := s.orgs[sc.Org]
		rec, _, err := s.ensure("message_schemas",
			"organization = {:o} && namespace = {:ns} && name = {:n} && version = {:v}",
			dbx.Params{"o": orgID, "ns": sc.Namespace, "n": sc.Name, "v": sc.Version},
			func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("namespace", sc.Namespace)
				r.Set("name", sc.Name)
				r.Set("version", sc.Version)
				r.Set("format", "json_schema")
				r.Set("description", sc.Description)
				r.Set("schema", sc.Schema)
			})
		if err != nil {
			return err
		}
		s.schemas[schemaKey(sc.Org, sc.Namespace, sc.Name, sc.Version)] = rec.Id
	}
	return nil
}

// ----------------------------------------------------------------- operations

func (s *seeder) seedOperations() error {
	for _, op := range operations {
		orgID := s.orgs[op.Org]

		var schemaID string
		if op.SchemaName != "" {
			key := schemaKey(op.Org, op.SchemaNS, op.SchemaName, op.SchemaVersion)
			id, ok := s.schemas[key]
			if !ok {
				return fmt.Errorf("operation %q references unknown schema %q", op.Name, key)
			}
			schemaID = id
		}

		rec, _, err := s.ensure("thing_type_operations", "organization = {:o} && name = {:n}",
			dbx.Params{"o": orgID, "n": op.Name}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("name", op.Name)
				r.Set("capability", op.Capability)
				r.Set("subject_suffix", op.SubjectSuffix)
				r.Set("description", op.Description)
				if schemaID != "" {
					r.Set("schema", schemaID)
				}
			})
		if err != nil {
			return err
		}
		s.ops[op.Org+":"+op.Name] = rec.Id
	}
	return nil
}

// ---------------------------------------------------------------- thing types

func (s *seeder) seedThingTypes() error {
	for _, tt := range thingTypes {
		orgID := s.orgs[tt.Org]

		opIDs := make([]string, 0, len(tt.Operations))
		for _, name := range tt.Operations {
			id, ok := s.ops[tt.Org+":"+name]
			if !ok {
				return fmt.Errorf("thing type %q references unknown operation %q", tt.Code, name)
			}
			opIDs = append(opIDs, id)
		}

		roleID, ok := s.roles[tt.Org+":"+tt.Role]
		if !ok {
			return fmt.Errorf("thing type %q references unknown NATS role %q", tt.Code, tt.Role)
		}

		rec, _, err := s.ensure("thing_types", "organization = {:o} && code = {:c}",
			dbx.Params{"o": orgID, "c": tt.Code}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("code", tt.Code)
				r.Set("name", tt.Name)
				r.Set("description", tt.Description)
				r.Set("subject_prefix", tt.SubjectPrefix)
				r.Set("capabilities", tt.Capabilities)
				r.Set("operations", opIDs)
				r.Set("nats_role", roleID)
				if tt.Schema != nil {
					r.Set("metadata_schema", tt.Schema)
				}
			})
		if err != nil {
			return err
		}
		s.thingType[tt.Org+":"+tt.Code] = rec.Id
	}
	return nil
}

// ----------------------------------------------------------- overlay networks

func (s *seeder) seedNetworks() error {
	byOrg := map[string]orgFixture{}
	for _, o := range orgs {
		byOrg[o.Code] = o
	}

	for _, n := range networks {
		orgID := s.orgs[n.Org]
		rec, _, err := s.ensure("nebula_networks", "organization = {:o} && name = {:n}",
			dbx.Params{"o": orgID, "n": n.Name}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("name", n.Name)
				r.Set("description", n.Description)
				r.Set("cidr_range", byOrg[n.Org].NebulaCIDR)
				r.Set("ca_id", s.cas[n.Org])
				r.Set("active", true)
			})
		if err != nil {
			return err
		}
		s.networks[n.Org] = rec.Id
	}

	// Lighthouses last: they are hosts on the networks just created.
	for _, lh := range lighthouses {
		if _, err := s.nebulaHost(lh.Org, lh.Hostname, lh.OverlayIP, lh.Groups, lh.PublicHostPort); err != nil {
			return err
		}
	}
	return nil
}

// nebulaHost is the found-or-create for a Nebula host, mirroring the naming
// hooks/thing_routes.go uses so a seeded host and a console-created one are
// indistinguishable.
func (s *seeder) nebulaHost(orgCode, hostname, overlayIP string, groups []string, publicHostPort string) (string, error) {
	orgID := s.orgs[orgCode]
	networkID := s.networks[orgCode]
	if networkID == "" {
		return "", fmt.Errorf("org %q has no overlay network", orgCode)
	}

	rec, _, err := s.ensure("nebula_hosts", "network_id = {:n} && hostname = {:h}",
		dbx.Params{"n": networkID, "h": hostname}, func(r *core.Record) {
			r.Set("hostname", hostname)
			r.Set("email", fmt.Sprintf("%s@%s.nebula.local", hostname, orgCode))
			r.Set("emailVisibility", true)
			r.SetPassword(secret(32))
			r.Set("network_id", networkID)
			r.Set("overlay_ip", overlayIP)
			r.Set("organization", orgID)
			r.Set("groups", orEmpty(groups))
			r.Set("active", true)
			if publicHostPort != "" {
				r.Set("is_lighthouse", true)
				r.Set("public_host_port", publicHostPort)
			}
		})
	if err != nil {
		return "", err
	}
	return rec.Id, nil
}

// natsUser is the found-or-create for a device or application NATS identity,
// using the same convention as POST /api/org/things: nats_username is the
// Thing's code, and the email's domain half is the org's immutable code.
func (s *seeder) natsUser(orgCode, code, roleName string) (string, error) {
	orgID := s.orgs[orgCode]
	roleID, ok := s.roles[orgCode+":"+roleName]
	if !ok {
		return "", fmt.Errorf("NATS role %q is not seeded in org %q", roleName, orgCode)
	}
	email := fmt.Sprintf("%s@%s.nats.local", code, orgCode)

	rec, _, err := s.ensure("nats_users", "email = {:e}", dbx.Params{"e": email}, func(r *core.Record) {
		r.Set("nats_username", code)
		r.Set("email", email)
		r.Set("emailVisibility", true)
		r.SetPassword(secret(32))
		r.Set("account_id", s.accounts[orgCode])
		r.Set("role_id", roleID)
		r.Set("organization", orgID)
		r.Set("active", true)
	})
	if err != nil {
		return "", err
	}
	return rec.Id, nil
}
