package demoseed

import (
	"fmt"
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Things and edge sites — the two phases that mint identities.
//
// A Thing here is created the way POST /api/org/things creates one: the record,
// its NATS identity, and (where the fixture asks for it) its Nebula host. Most
// field sensors get no Nebula host, which is the real design rather than an
// omission — a probe reaches the bus and nothing else, and the overlay exists
// for the management path that has to work when the bus does not.

// seedThings creates the curated fixtures, then generates enough additional
// devices to reach Options.Things.
func (s *seeder) seedThings() error {
	for _, t := range things {
		if err := s.thing(t); err != nil {
			return err
		}
	}
	return s.fillThings()
}

// thing is the found-or-create for one Thing plus its identities.
func (s *seeder) thing(t thingFixture) error {
	orgID, ok := s.orgs[t.Org]
	if !ok {
		return fmt.Errorf("thing %q references unknown org %q", t.Code, t.Org)
	}
	typeID, ok := s.thingType[t.Org+":"+t.Type]
	if !ok {
		return fmt.Errorf("thing %q references unknown thing type %q", t.Code, t.Type)
	}
	locID, ok := s.locations[t.Org+":"+t.Location]
	if !ok {
		return fmt.Errorf("thing %q references unknown location %q", t.Code, t.Location)
	}

	// The NATS role: the fixture's override, else the thing type's own.
	roleName := t.Role
	if roleName == "" {
		roleName = roleForThingType(t.Org, t.Type)
		if roleName == "" {
			return fmt.Errorf("thing type %q has no NATS role", t.Type)
		}
	}

	natsUserID, err := s.natsUser(t.Org, t.Code, roleName)
	if err != nil {
		return err
	}

	var nebulaHostID string
	if t.NebulaIP != "" {
		nebulaHostID, err = s.nebulaHost(t.Org, t.Code, t.NebulaIP, t.NebulaGroups, "")
		if err != nil {
			return err
		}
	}

	_, _, err = s.ensure("things", "organization = {:o} && code = {:c}",
		dbx.Params{"o": orgID, "c": t.Code}, func(r *core.Record) {
			r.Set("organization", orgID)
			r.Set("code", t.Code)
			r.Set("name", t.Name)
			r.Set("description", t.Description)
			r.Set("type", typeID)
			r.Set("location", locID)
			// The console used to omit this and every Thing it created was
			// locked out by things.authRule. Set explicitly for the same reason
			// the route sets it.
			r.Set("active", !t.Inactive)
			r.Set("email", fmt.Sprintf("%s@%s.thing.local", t.Code, t.Org))
			r.Set("emailVisibility", true)
			r.SetPassword(secret(16))
			r.Set("nats_user", natsUserID)
			if nebulaHostID != "" {
				r.Set("nebula_host", nebulaHostID)
			}
			if t.Metadata != nil {
				r.Set("metadata", t.Metadata)
			}
		})
	if err != nil {
		return err
	}

	// A decommissioned Thing is only decommissioned if its NATS identity is
	// revoked too — the flag on its own closes the console door and leaves the
	// device publishing. hooks/active_flag.go does this on the true->false flip,
	// which a create never is, so the seeder states it directly.
	if t.Inactive {
		if err := s.revokeNatsUser(natsUserID); err != nil {
			return err
		}
	}
	return nil
}

// roleForThingType looks up a thing type's default NATS role by name, from the
// fixtures rather than the database — the fixture is the source of truth for
// what the demo means, and a hand-edited role on the record should not silently
// change what the next generated device gets.
func roleForThingType(org, code string) string {
	for _, tt := range thingTypes {
		if tt.Org == org && tt.Code == code {
			return tt.Role
		}
	}
	return ""
}

// revokeNatsUser sets the revoke trigger pb-nats acts on. It adds the public key
// to the account's revocation list and re-signs the account JWT; `active = false`
// on the nats_users record alone is read by nothing and disconnects nobody.
func (s *seeder) revokeNatsUser(id string) error {
	rec, err := s.app.FindRecordById("nats_users", id)
	if err != nil {
		return err
	}
	// pb-nats clears the flag as it handles it, so a set flag means it has not
	// been processed yet and a clear one means either "already done" or "never
	// asked". Checking `active` instead would re-trigger on every run.
	if !rec.GetBool("active") {
		return nil
	}
	rec.Set("revoke", true)
	rec.Set("active", false)
	if err := s.app.Save(rec); err != nil {
		return fmt.Errorf("revoke nats user %s: %w", id, err)
	}
	return nil
}

// ------------------------------------------------------------- generated bulk

// fillThings tops the inventory up to Options.Things by generating devices for
// the thing types that declared a BulkPrefix. Types without one are curated-only:
// a WMS connector or a rule engine is a single named participant, and cloning it
// into a fleet of forty would misrepresent what an application Thing is.
func (s *seeder) fillThings() error {
	target := s.opts.Things
	have, err := s.countThings()
	if err != nil {
		return err
	}
	if have >= target {
		return nil
	}

	// Deterministic order: the fixture order, filtered. Round-robin across types
	// so the fill is spread over every org rather than exhausting the first.
	var bulk []thingTypeFixture
	for _, tt := range thingTypes {
		if tt.BulkPrefix != "" && len(tt.BulkLocations) > 0 {
			bulk = append(bulk, tt)
		}
	}
	if len(bulk) == 0 {
		return nil
	}

	// Per-type sequence numbers continue past whatever already exists, so a
	// re-run with a higher --things adds to the fleet instead of colliding.
	seq := map[string]int{}
	for i := range bulk {
		n, err := s.countByCodePrefix(bulk[i].Org, bulk[i].BulkPrefix+"-")
		if err != nil {
			return err
		}
		seq[bulk[i].Org+":"+bulk[i].Code] = n
	}

	for i := 0; have < target; i++ {
		tt := bulk[i%len(bulk)]
		key := tt.Org + ":" + tt.Code
		seq[key]++
		n := seq[key]

		code := fmt.Sprintf("%s-%03d", tt.BulkPrefix, n)
		loc := tt.BulkLocations[(n-1)%len(tt.BulkLocations)]

		if err := s.thing(thingFixture{
			Org: tt.Org, Code: code, Type: tt.Code, Location: loc,
			Name:     fmt.Sprintf("%s %03d", tt.Name, n),
			Metadata: s.bulkMetadata(tt, n),
		}); err != nil {
			return err
		}
		have++
	}
	return nil
}

// bulkMetadata produces plausible per-device metadata. It does not attempt to
// satisfy the type's whole metadata_schema: a schema here is advisory, and a
// half-filled metadata bag is what real inventory looks like.
func (s *seeder) bulkMetadata(tt thingTypeFixture, n int) map[string]any {
	m := map[string]any{
		"serial":   fmt.Sprintf("%s-%s-%05d", tt.BulkPrefix, tt.Org[:2], 10000+n*7),
		"firmware": fmt.Sprintf("%d.%d.%d", 2, s.pickInt(0, 4), s.pickInt(0, 9)),
	}
	switch tt.Code {
	case "temp-probe":
		m["probe_type"] = []string{"air", "product", "glycol"}[n%3]
		m["calibrated_on"] = fmt.Sprintf("2026-%02d-%02d", s.pickInt(1, 6), s.pickInt(1, 28))
	case "door-sensor":
		m["door_ref"] = fmt.Sprintf("D%02d", n)
		m["fail_safe"] = n%4 != 0
	case "power-meter":
		m["panel"] = fmt.Sprintf("MCC-%d", 1+(n%3))
		m["ct_ratio"] = []string{"200:5", "400:5", "800:5"}[n%3]
	case "vib-sensor":
		m["mount"] = []string{"stud", "magnet", "adhesive"}[n%3]
		m["machine_ref"] = fmt.Sprintf("MC-%03d", 100+n)
	case "feeder-relay":
		m["feeder"] = fmt.Sprintf("F%02d", n)
		m["protocol"] = []string{"dnp3", "iec61850", "modbus"}[n%3]
		m["make"] = []string{"SEL", "GE", "ABB"}[n%3]
	}
	return m
}

func (s *seeder) countThings() (int, error) {
	var total int
	for _, o := range orgs {
		recs, err := s.app.FindAllRecords("things",
			dbx.NewExp("organization = {:o}", dbx.Params{"o": s.orgs[o.Code]}))
		if err != nil {
			return 0, err
		}
		total += len(recs)
	}
	return total, nil
}

// countByCodePrefix counts existing generated devices so sequence numbers resume
// rather than restart. LIKE is safe here: every prefix is an ASCII literal from
// the fixtures, with no wildcard characters in it.
func (s *seeder) countByCodePrefix(orgCode, prefix string) (int, error) {
	recs, err := s.app.FindAllRecords("things",
		dbx.NewExp("organization = {:o} AND code LIKE {:p}",
			dbx.Params{"o": s.orgs[orgCode], "p": prefix + "%"}))
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

// ------------------------------------------------------------------ edge sites

func (s *seeder) seedLeafNodes() error {
	for _, l := range leafNodes {
		orgID, ok := s.orgs[l.Org]
		if !ok {
			return fmt.Errorf("leaf node %q references unknown org %q", l.Code, l.Org)
		}
		locID, ok := s.locations[l.Org+":"+l.Location]
		if !ok {
			return fmt.Errorf("leaf node %q references unknown location %q", l.Code, l.Location)
		}

		var nebulaHostID string
		if l.NebulaIP != "" {
			var err error
			nebulaHostID, err = s.nebulaHost(l.Org, "leaf-"+l.Code, l.NebulaIP, l.NebulaGroups, "")
			if err != nil {
				return err
			}
		}

		synced := l.Synced
		sort.Strings(synced)

		// No nats_user is set. RegisterLeafNodeProvisioning mints one on create
		// and links it back, which is the path `leaf-sync` authenticates over —
		// setting it here would bypass the hook and leave the demo unable to
		// demonstrate the thing it is demonstrating.
		if _, _, err := s.ensure("leaf_nodes", "organization = {:o} && code = {:c}",
			dbx.Params{"o": orgID, "c": l.Code}, func(r *core.Record) {
				r.Set("organization", orgID)
				r.Set("code", l.Code)
				r.Set("name", l.Name)
				r.Set("description", l.Description)
				r.Set("location", locID)
				// edge-<code>, matching what LeafNodeFormView derives.
				r.Set("domain", "edge-"+l.Code)
				r.Set("synced_collections", synced)
				r.Set("active", true)
				r.Set("email", fmt.Sprintf("%s@%s.leaf.local", l.Code, l.Org))
				r.Set("emailVisibility", true)
				r.SetPassword(secret(16))
				if nebulaHostID != "" {
					r.Set("nebula_host", nebulaHostID)
				}
			}); err != nil {
			return err
		}
	}
	return nil
}
