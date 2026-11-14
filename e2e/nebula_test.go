//go:build e2e

package e2e

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/slackhq/nebula"
	"github.com/slackhq/nebula/config"
	"github.com/slackhq/nebula/logging"
)

// The config_yaml the platform hands a lighthouse and a host is one Nebula will
// start on. nebula.Main with configTest loads the PKI (refusing a key that does
// not pair with its certificate), the firewall and the lighthouse settings, and
// stops before it needs a TUN device. Until v0.19.1 every host key failed the
// pair check, so no host the platform issued could start.
func TestNebulaStartsOnTheIssuedConfigs(t *testing.T) {
	tn := newTenant(t)
	cas := findByOrg(t, "nebula_ca", tn.OrgID)
	if len(cas) != 1 {
		t.Fatalf("org has %d Nebula CAs, want 1", len(cas))
	}

	var network record
	platform.mustRequest(t, 200, "POST", "/collections/nebula_networks/records", platform.SUToken, map[string]any{
		"name": uniq("net"), "cidr_range": "10.42.0.0/24", "ca_id": cas[0].ID,
		"organization": tn.OrgID, "active": true,
	}, &network)

	var lighthouse record
	platform.mustRequest(t, 200, "POST", "/collections/nebula_hosts/records", platform.SUToken, map[string]any{
		"email": uniq("lh") + "@nebula.e2e.test", "emailVisibility": true,
		"password": testPassword, "passwordConfirm": testPassword,
		"hostname": "lighthouse", "overlay_ip": "10.42.0.1", "is_lighthouse": true,
		"public_host_port": "127.0.0.1:4242", "network_id": network.ID,
		"organization": tn.OrgID, "active": true,
	}, &lighthouse)

	// The host goes through POST /api/org/things, the path the console uses.
	thingID := tn.thing(t, map[string]any{
		"nats":   map[string]any{"mode": "none"},
		"nebula": map[string]any{"mode": "auto", "network_id": network.ID, "overlay_ip": "10.42.0.10"},
	})
	var th struct {
		NebulaHost string `json:"nebula_host"`
	}
	platform.mustRequest(t, 200, "GET", "/collections/things/records/"+thingID, platform.SUToken, nil, &th)
	if th.NebulaHost == "" {
		t.Fatal("the Thing has no Nebula host")
	}

	for name, hostID := range map[string]string{"lighthouse": lighthouse.ID, "host": th.NebulaHost} {
		t.Run(name, func(t *testing.T) {
			yaml := configYAML(t, hostID)
			var c config.C
			if err := c.LoadString(yaml); err != nil {
				t.Fatalf("config_yaml does not parse: %v\n%s", err, yaml)
			}
			if _, err := nebula.Main(&c, true, "e2e", logging.NewLogger(io.Discard), nil); err != nil {
				t.Fatalf("Nebula refuses to start on the issued config: %v", err)
			}
		})
	}
}

// configYAML waits for a Nebula host's generated config.
func configYAML(t *testing.T, hostID string) string {
	t.Helper()
	var h struct {
		ConfigYAML string `json:"config_yaml"`
	}
	eventually(t, 10*time.Second, func() error {
		platform.mustRequest(t, 200, "GET", "/collections/nebula_hosts/records/"+hostID, platform.SUToken, nil, &h)
		if h.ConfigYAML == "" {
			return fmt.Errorf("host %s has no config_yaml yet", hostID)
		}
		return nil
	})
	return h.ConfigYAML
}
