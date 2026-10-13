package hooks

import (
	"sort"
	"strings"
	"testing"
)

// The field names GET /api/me/leaf-config puts on the wire are a CROSS-REPO
// contract: the agent decodes them by name from a different module, and a
// rename here would compile, deploy, and fail at an edge box with an empty
// directive in a generated nats-leaf.conf.
//
// The route this replaced had no server-side test at all -- its field list was
// pinned only by a hand-written fixture inside the consumer, which is exactly
// the arrangement that lets a server-side rename ship green.
func TestLeafConfigFieldNames(t *testing.T) {
	got := leafConfigResponse(leafConfig{})

	want := []string{
		"account_jwt",
		"account_pub",
		"code",
		"creds",
		"domain",
		"hub_domain",
		"hub_leaf_url",
		"operator_jwt",
		"sys_account_jwt",
		"sys_account_pub",
	}

	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) != len(want) {
		t.Fatalf("leaf-config returns %d fields %v, want %d %v", len(keys), keys, len(want), want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("field %d = %q, want %q (full set: %v)", i, k, want[i], keys)
		}
	}
}

// The JetStream domain is the Thing's code, computed rather than stored. A
// stored domain would be a second name for an identifier the Thing already
// carries, free to drift from it. Two directives in nats-leaf.conf depend on
// them agreeing -- `server_name` and `jetstream { domain }` -- and a tenant's
// site-status dashboard widget matches a leaf's reported server_name (CONNZ)
// back to a Thing's code, so a divergence here makes every gateway look offline.
func TestLeafConfigDomainIsTheCode(t *testing.T) {
	got := leafConfigResponse(leafConfig{Code: "s01"})

	if got["domain"] != "s01" {
		t.Errorf("domain = %q, want the code %q", got["domain"], "s01")
	}
	if got["code"] != got["domain"] {
		t.Errorf("code %q and domain %q must be identical", got["code"], got["domain"])
	}
}

// Nothing here is per-caller, so the hub values must arrive verbatim.
func TestLeafConfigCarriesTheHubValuesVerbatim(t *testing.T) {
	got := leafConfigResponse(leafConfig{
		Code:       "s01",
		HubLeafURL: "nats-leaf://hub.example.com:7422",
		HubDomain:  "hub",
	})

	if got["hub_leaf_url"] != "nats-leaf://hub.example.com:7422" {
		t.Errorf("hub_leaf_url = %q", got["hub_leaf_url"])
	}
	if got["hub_domain"] != "hub" {
		t.Errorf("hub_domain = %q", got["hub_domain"])
	}
}

// A platform that is not set up for edge sites says so, naming the setting and
// its environment variable. The agent would reject the empty value anyway, but
// it can only name the field, which does not tell anyone what to change.
func TestLeafConfigNamesTheMissingHubSetting(t *testing.T) {
	tests := []struct {
		leafURL, domain string
		want            string // "" = set up
	}{
		{"nats-leaf://hub.example.com:7422", "hub", ""},
		{"", "hub", "STONE_AGE_NATS_LEAF_URL"},
		{"nats-leaf://hub.example.com:7422", "", "STONE_AGE_NATS_JETSTREAM_DOMAIN"},
	}
	for _, tt := range tests {
		got := hubNotSetUp(LeafConfigRoutesOptions{HubLeafURL: tt.leafURL, HubDomain: tt.domain})
		if tt.want == "" && got != "" {
			t.Errorf("leaf_url %q, domain %q: refused with %q, want served", tt.leafURL, tt.domain, got)
		}
		if tt.want != "" && !strings.Contains(got, tt.want) {
			t.Errorf("leaf_url %q, domain %q: got %q, want it to name %s", tt.leafURL, tt.domain, got, tt.want)
		}
	}
}
