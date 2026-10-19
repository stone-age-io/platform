package hooks_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"gopkg.in/yaml.v3"
)

// WHY THIS TEST EXISTS
//
// A DEPENDENCY CONTRACT test, like the two beside it in nebula_rotation_test.go:
// it asserts a pb-nebula behaviour rather than platform code, so an upstream
// regression or a downgrade fails here instead of in production.
//
// The contract is three column NAMES. pb-nebula declares additional_host_ports,
// advertise_addrs and local_allow_list in its own collections manager and reads
// them back off the record; schema.json declares the same three, and the console
// form writes them. Nothing checks that the two spellings agree -- and PocketBase
// DISCARDS A WRITE TO A FIELD THAT DOES NOT EXIST, in silence. So a drift here
// does not error anywhere: the operator fills in a second lighthouse address, the
// form saves, the record stores nothing, and config_yaml comes back exactly as it
// was. The same failure shape as the managed-export names, the re-import that
// keeps the live field, and the flag PocketBase discarded.
//
// It also pins the split between the two halves of the feature, which is the part
// most likely to be "simplified" by someone who reads them as duplicates:
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

	// Every column under test, set through the ordinary record API -- the same
	// path the console form takes.
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
	// Nebula tries the list in order and the primary is the operator's stated
	// first choice, so order is asserted, not just membership.
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

	// listen.port still comes from public_host_port alone. A host listens on one
	// port; the extra entries name other ADDRESSES it is reachable at, and a v6
	// entry parsed with the old last-colon split would have landed here as 10.
	if lhCfg.Listen.Port != 4242 {
		t.Errorf("listen.port = %d, want 4242 (from public_host_port, not from the address list)",
			lhCfg.Listen.Port)
	}
}

// generatedConfig is the slice of a Nebula config this test reads. Parsed rather
// than string-matched: `grep advertise_addrs` would pass on a commented-out key
// and on one rendered into the wrong section.
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
