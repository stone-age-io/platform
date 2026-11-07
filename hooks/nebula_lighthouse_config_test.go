package hooks_test

import (
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"gopkg.in/yaml.v3"
)

// A DEPENDENCY CONTRACT test, like those in nebula_rotation_test.go.
//
// The contract is three column NAMES: additional_host_ports, advertise_addrs
// and local_allow_list, declared by pb-nebula and by schema.json and written by
// the console form. PocketBase discards a write to a missing field in silence,
// so a spelling drift would store nothing and change no config, with no error.
//
// It also pins the split between the two halves, which are not duplicates:
//
//   - a lighthouse's addresses are BOOTSTRAP data and belong in every OTHER
//     host's static_host_map
//   - advertise_addrs and local_allow_list are REPORTING data about one machine
//     and belong only in that machine's own config
//
// Requires pb-nebula v0.4.0.
func TestLighthouseAddressColumnsReachTheGeneratedConfig(t *testing.T) {
	app, ca := nebulaOrg(t)
	org := ca.GetString("organization")

	newRecord := func(collection string) *core.Record {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			t.Fatal(err)
		}
		return core.NewRecord(col)
	}

	network := newRecord("nebula_networks")
	network.Set("name", "lighthouse-addr-net")
	network.Set("cidr_range", "10.43.0.0/16")
	network.Set("ca_id", ca.Id)
	network.Set("organization", org)
	network.Set("active", true)
	if err := app.Save(network); err != nil {
		t.Fatal(err)
	}

	const (
		lighthouseIP = "10.43.0.1"
		primaryAddr  = "203.0.113.10:4242"
		v6Addr       = "[2001:db8::10]:4242"
		lanAddr      = "10.0.0.10:4242"
	)

	// Every column under test, set the way the console form sets them.
	lh := newRecord("nebula_hosts")
	lh.Set("email", "lh@lighthouse-addr.test")
	lh.Set("emailVisibility", true)
	lh.Set("password", "Password123!")
	lh.Set("hostname", "lh-1")
	lh.Set("overlay_ip", lighthouseIP)
	lh.Set("network_id", network.Id)
	lh.Set("organization", org)
	lh.Set("is_lighthouse", true)
	lh.Set("public_host_port", primaryAddr)
	lh.Set("additional_host_ports", []string{v6Addr, lanAddr})
	lh.Set("advertise_addrs", []string{"203.0.113.10:0"})
	lh.Set("local_allow_list", map[string]any{
		"interfaces": map[string]any{"docker.*": false},
	})
	if err := app.Save(lh); err != nil {
		t.Fatalf("saving a lighthouse with the three new columns: %v", err)
	}

	// An ordinary peer, and the negative control in one record: it sets none of
	// the three, so its own config must carry neither reporting key.
	peer := newRecord("nebula_hosts")
	peer.Set("email", "peer@lighthouse-addr.test")
	peer.Set("emailVisibility", true)
	peer.Set("password", "Password123!")
	peer.Set("hostname", "peer-1")
	peer.Set("overlay_ip", "10.43.0.20")
	peer.Set("network_id", network.Id)
	peer.Set("organization", org)
	if err := app.Save(peer); err != nil {
		t.Fatal(err)
	}

	// --- the columns survived the save at all ---
	lh = reload(t, app, lh)
	if got := lh.GetString("additional_host_ports"); got == "" || got == "null" {
		t.Fatal("additional_host_ports stored nothing -- schema.json and pb-nebula " +
			"disagree on the column name, and PocketBase dropped the write in silence")
	}

	// --- bootstrap data reaches the PEER ---
	peerCfg := parseGeneratedConfig(t, reload(t, app, peer), "peer")

	entry, ok := peerCfg.StaticHostMap[lighthouseIP]
	if !ok {
		t.Fatalf("peer's static_host_map has no entry for the lighthouse at %s; got %v",
			lighthouseIP, peerCfg.StaticHostMap)
	}
	// Nebula tries the list in order, so order is asserted, not just membership.
	want := []string{primaryAddr, v6Addr, lanAddr}
	if len(entry) != len(want) {
		t.Fatalf("peer's static_host_map carries %d addresses, want %d: %v", len(entry), len(want), entry)
	}
	for i := range want {
		if entry[i] != want[i] {
			t.Errorf("static_host_map[%d] = %q, want %q (primary first, then the column's order): %v",
				i, entry[i], want[i], entry)
		}
	}

	// --- reporting data stays on the LIGHTHOUSE and does not leak to the peer ---
	if len(peerCfg.Lighthouse.AdvertiseAddrs) != 0 {
		t.Errorf("the peer's config carries advertise_addrs %v; that is a statement "+
			"about one machine and belongs only in that machine's own config",
			peerCfg.Lighthouse.AdvertiseAddrs)
	}
	if len(peerCfg.Lighthouse.LocalAllowList) != 0 {
		t.Errorf("the peer's config carries local_allow_list %v; it sets none, and a "+
			"host that sets none must render the config it always did",
			peerCfg.Lighthouse.LocalAllowList)
	}

	lhCfg := parseGeneratedConfig(t, lh, "lighthouse")

	if len(lhCfg.Lighthouse.AdvertiseAddrs) != 1 || lhCfg.Lighthouse.AdvertiseAddrs[0] != "203.0.113.10:0" {
		t.Errorf("lighthouse.advertise_addrs = %v, want [203.0.113.10:0] -- behind 1:1 NAT "+
			"this is the only thing that stops the host reporting its private address",
			lhCfg.Lighthouse.AdvertiseAddrs)
	}

	ifaces, ok := lhCfg.Lighthouse.LocalAllowList["interfaces"].(map[string]any)
	if !ok {
		t.Fatalf("lighthouse.local_allow_list has no interfaces map: %#v", lhCfg.Lighthouse.LocalAllowList)
	}
	if blocked, ok := ifaces["docker.*"]; !ok || blocked != false {
		t.Errorf(`local_allow_list.interfaces["docker.*"] = %v (present=%t), want false -- `+
			"without it a containerised host advertises docker0 to every peer", blocked, ok)
	}

	// listen.port comes from public_host_port alone; the extra entries are other
	// ADDRESSES, and a v6 entry split at the last colon would land here.
	if lhCfg.Listen.Port != 4242 {
		t.Errorf("listen.port = %d, want 4242 (from public_host_port, not from the address list)",
			lhCfg.Listen.Port)
	}
}

// generatedConfig is the slice of a Nebula config this test reads. Parsed, not
// string-matched, which would pass on a commented-out or misplaced key.
type generatedConfig struct {
	StaticHostMap map[string][]string `yaml:"static_host_map"`
	Lighthouse    struct {
		AmLighthouse   bool           `yaml:"am_lighthouse"`
		AdvertiseAddrs []string       `yaml:"advertise_addrs"`
		LocalAllowList map[string]any `yaml:"local_allow_list"`
	} `yaml:"lighthouse"`
	Listen struct {
		Port int `yaml:"port"`
	} `yaml:"listen"`
}

func parseGeneratedConfig(t *testing.T, host *core.Record, label string) generatedConfig {
	t.Helper()
	raw := host.GetString("config_yaml")
	if raw == "" {
		t.Fatalf("%s has no config_yaml", label)
	}
	var cfg generatedConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("%s config_yaml is not valid YAML: %v\n%s", label, err, raw)
	}
	return cfg
}
