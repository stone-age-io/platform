package leafsync

import (
	"strings"
	"testing"
)

func TestBuildLeafConf(t *testing.T) {
	conf := buildLeafConf(leafConfParams{
		OperatorJWT: "OPJWT",
		AccountPub:  "ACCOUNTPUB",
		AccountJWT:  "ACCJWT",
		Domain:      "edge-warehouse",
		HubLeafURL:  "nats-leaf://hub.example.com:7422",
		CredsName:   "edge.creds",
	})

	// Each directive a stock nats-server needs to come up as an offline,
	// operator-mode leaf bound to one preloaded account.
	wants := []string{
		`server_name: "edge-warehouse"`,
		`domain: "edge-warehouse"`,    // JetStream domain isolates the edge from the hub
		`operator: "OPJWT"`,           // operator trust anchor
		`resolver: MEMORY`,            // no dynamic account resolver => no $SYS bridge
		`ACCOUNTPUB: "ACCJWT"`,        // resolver_preload: the single org account
		`url: "nats-leaf://hub.example.com:7422"`,
		`credentials: "edge.creds"`,
		`http: "127.0.0.1:8222"`,      // localhost monitoring only
	}
	for _, w := range wants {
		if !strings.Contains(conf, w) {
			t.Errorf("nats-leaf.conf missing %q\n---\n%s", w, conf)
		}
	}
}
