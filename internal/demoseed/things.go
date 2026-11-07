package demoseed

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Things are created the way POST /api/org/things creates them: the record, its
// NATS identity and, where the fixture asks, a Nebula host. Most field sensors
// get no Nebula host by design: the overlay is for the management path.

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

	thing, _, err := s.ensure("things", "organization = {:o} && code = {:c}",
		dbx.Params{"o": orgID, "c": t.Code}, func(r *core.Record) {
			r.Set("organization", orgID)
			r.Set("code", t.Code)
			r.Set("name", t.Name)
			r.Set("description", t.Description)
			r.Set("type", typeID)
			r.Set("location", locID)
			// things.authRule needs active = true, and bools have no schema
			// default. Always true here: deactivation is an update, below.
			r.Set("active", true)
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

	// Decommissioning is an update through hooks/active_flag.go: the hook forces
	// active = true on create, and its true->false edge refreshes the token key,
	// suspends the NATS identity and blocklists the Nebula host. Never set
	// nats_users.revoke instead: pb-nats checks revoke first, which rotates to a
	// fresh working credential and skips the suspend.
	//
	// The record is re-read first because the hook keys on Original(), and a
	// record just created in memory has an empty original, so it sees no edge.
	if t.Inactive {
		fresh, err := s.app.FindRecordById("things", thing.Id)
		if err != nil {
			return fmt.Errorf("reload thing %s before deactivating: %w", t.Code, err)
		}
		if fresh.GetBool("active") {
			fresh.Set("active", false)
			if err := s.app.Save(fresh); err != nil {
				return fmt.Errorf("deactivate thing %s: %w", t.Code, err)
			}
		}
	}
	return nil
}

// roleForThingType looks up a thing type's default NATS role from the fixtures,
// not the database, so a hand-edited record does not change what the next
// generated device gets.
func roleForThingType(org, code string) string {
	for _, tt := range thingTypes {
		if tt.Org == org && tt.Code == code {
			return tt.Role
		}
	}
	return ""
}

// fillThings tops the inventory up to Options.Things by generating devices for
// the thing types that declare a BulkPrefix. Types without one are single named
// participants (a WMS connector, a rule engine) and stay curated-only.
func (s *seeder) fillThings() error {
	target := s.opts.Things
	have, err := s.countThings()
	if err != nil {
		return err
	}
	if have >= target {
		return nil
	}

	// Fixture order, round-robin across types, so the fill spreads over every org.
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

// bulkMetadata produces plausible per-device metadata. It does not fill the
// whole metadata_schema: the schema is advisory.
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

// countByCodePrefix counts existing generated devices so sequence numbers resume.
// LIKE is safe: every prefix is a fixture literal with no wildcard characters.
func (s *seeder) countByCodePrefix(orgCode, prefix string) (int, error) {
	recs, err := s.app.FindAllRecords("things",
		dbx.NewExp("organization = {:o} AND code LIKE {:p}",
			dbx.Params{"o": s.orgs[orgCode], "p": prefix + "%"}))
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}
