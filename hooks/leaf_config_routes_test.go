package hooks

import (
	"sort"
	"strings"
	"testing"
)

// The field names GET /api/me/leaf-config puts on the wire are a cross-repo
// contract: the agent decodes them by name, and a rename would compile, deploy,
// and fail at an edge box with an empty directive in nats-leaf.conf.
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

// The probe carries exactly the two public account fields, under leaf-config's
// names, and in particular no `creds`.
func TestAccountJWTFieldNames(t *testing.T) {
	got := accountJWTResponse("JWT", "PUB")

	if len(got) != 2 || got["account_jwt"] != "JWT" || got["account_pub"] != "PUB" {
		t.Errorf("account-jwt returns %v, want exactly account_jwt and account_pub", got)
	}
	leaf := leafConfigResponse(leafConfig{AccountJWT: "JWT", AccountPub: "PUB"})
	for k, v := range got {
		if leaf[k] != v {
			t.Errorf("probe field %q = %q, but leaf-config names the same value %q", k, v, leaf[k])
		}
	}
}

// The JetStream domain is the Thing's code. `server_name` and
// `jetstream { domain }` in nats-leaf.conf depend on them agreeing, and the
// site-status widget matches a leaf's server_name (CONNZ) back to the code, so
// a divergence makes every gateway look offline.
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

// A platform not set up for edge sites says so, naming the setting and its
// environment variable, which the agent cannot.
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
