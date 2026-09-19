package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The one thing that must stay true about the audit snapshot list: no
// collection on it holds a credential in a field that a snapshot would copy.
//
// WHY THIS IS A TEST AND NOT A COMMENT. A pb-audit snapshot is PublicExport(),
// so it copies every field a collection does not mark `hidden`. This platform
// deliberately leaves two credential fields unhidden because the identity that
// owns them has to read them back — `nats_users.creds_file` holds the NKEY seed
// and `nebula_hosts.config_yaml` holds the Nebula private key inline — and
// relies on ROW SCOPING to decide who sees which row. audit_logs has no row
// scoping. Adding one of those collections to the list would therefore restore,
// silently and in one line, the plaintext credential archive the list exists to
// remove. The failure has no symptom: the console looks identical and the rows
// are only read by an operator.
//
// It reads schema.json rather than naming the offending collections, so a field
// added to a listed collection tomorrow is caught by the same assertion.
func TestAuditSnapshotCollectionsHoldNoCredentials(t *testing.T) {
	// Substrings that mark a field as carrying credential or trust material.
	// Matched against field NAMES, which is why this is a test: the list is a
	// heuristic, and a heuristic is fine for raising a hand and wrong for
	// deciding what the binary does.
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

	// The same arguments main.go passes, which are config.yaml's defaults.
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

// The paired "can": the collections deliberately kept OFF the list really are
// the ones a snapshot would leak. Without this, hiding every field or emptying
// the list would pass the test above while destroying the audit trail.
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

	// The two the whole change is about, plus the field that carries the secret.
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
