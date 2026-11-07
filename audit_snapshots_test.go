package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// No collection on the audit snapshot list may hold a credential in an unhidden
// field. A snapshot copies every unhidden field, some credentials are unhidden
// on purpose (row scoping protects them), and audit_logs has no row scoping, so
// listing such a collection would silently archive credentials in plaintext.
// It reads schema.json, so a sensitive field added later is caught too.
func TestAuditSnapshotCollectionsHoldNoCredentials(t *testing.T) {
	// Matched against field names. A heuristic, which is fine in a test and
	// wrong for deciding what the binary does.
	sensitive := []string{
		"seed", "private", "creds", "config_yaml", "certificate",
		"jwt", "token", "secret", "signing", "password",
	}

	raw, err := os.ReadFile("schema.json")
	if err != nil {
		t.Fatalf("read schema.json: %v", err)
	}

	var collections []struct {
		Name   string `json:"name"`
		Fields []struct {
			Name   string `json:"name"`
			Hidden bool   `json:"hidden"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(raw, &collections); err != nil {
		t.Fatalf("parse schema.json: %v", err)
	}

	byName := make(map[string]int, len(collections))
	for i, c := range collections {
		byName[c.Name] = i
	}

	// The same arguments main.go passes.
	listed := auditSnapshotCollections("organizations", "memberships", "nats_roles", "nebula_networks")
	if len(listed) == 0 {
		t.Fatal("the snapshot list is empty; this test would pass vacuously")
	}

	for _, name := range listed {
		idx, ok := byName[name]
		if !ok {
			t.Errorf("%q is in the audit snapshot list but not in schema.json", name)
			continue
		}
		for _, f := range collections[idx].Fields {
			if f.Hidden {
				continue // stripped by PublicExport, so never reaches a snapshot
			}
			for _, needle := range sensitive {
				if strings.Contains(strings.ToLower(f.Name), needle) {
					t.Errorf("%s.%s is not hidden, so snapshotting %q would copy it into audit_logs; "+
						"remove the collection from auditSnapshotCollections or hide the field",
						name, f.Name, name)
				}
			}
		}
	}
}

// The paired "can": the collections kept off the list really would leak.
// Without this, hiding every field or emptying the list would pass the test
// above while destroying the audit trail.
func TestAuditSnapshotExclusionsWouldActuallyLeak(t *testing.T) {
	raw, err := os.ReadFile("schema.json")
	if err != nil {
		t.Fatalf("read schema.json: %v", err)
	}

	var collections []struct {
		Name   string `json:"name"`
		Fields []struct {
			Name   string `json:"name"`
			Hidden bool   `json:"hidden"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(raw, &collections); err != nil {
		t.Fatalf("parse schema.json: %v", err)
	}

	// Each excluded collection and the unhidden field that carries its secret.
	want := map[string]string{
		"nats_users":   "creds_file",
		"nebula_hosts": "config_yaml",
		"invites":      "token",
	}

	listed := auditSnapshotCollections("organizations", "memberships", "nats_roles", "nebula_networks")
	onList := make(map[string]bool, len(listed))
	for _, n := range listed {
		onList[n] = true
	}

	for collection, field := range want {
		if onList[collection] {
			t.Errorf("%s is on the audit snapshot list; %s would be copied into audit_logs", collection, field)
		}

		var found, hidden bool
		for _, c := range collections {
			if c.Name != collection {
				continue
			}
			for _, f := range c.Fields {
				if f.Name == field {
					found, hidden = true, f.Hidden
				}
			}
		}
		if !found {
			t.Errorf("%s.%s is gone from schema.json; this guard no longer describes the risk it was written for", collection, field)
			continue
		}
		if hidden {
			t.Errorf("%s.%s is now hidden — check whether that broke the identity that has to read it back, "+
				"and whether this guard is still needed", collection, field)
		}
	}
}
