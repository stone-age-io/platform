package demoseed

// Curated things, edge sites, overlay networks, and people.
//
// Nothing sets floorplan_position: it is pixel coordinates on an uploaded
// floorplan image, and this seed uploads none. Unplaced things show up in the
// positioning drawer instead.

type networkFixture struct {
	Org, Name, Description string
}

// One Nebula network per org; the CIDR comes from the org fixture, and the CA is
// the one RegisterOrgProvisioning created when the org was saved.
var networks = []networkFixture{
	{Org: "northwind", Name: "Northwind Overlay", Description: "Management path to both DCs and the trailer fleet."},
	{Org: "ironbridge", Name: "Ironbridge Overlay", Description: "Plant management path. Does not leave the site."},
	{Org: "galewind", Name: "Galewind Overlay", Description: "Reaches the unmanned substations over cellular, outbound only."},
}

type thingFixture struct {
	Org, Code, Name, Type, Location, Description string
	Metadata                                     map[string]any

	// Role overrides the thing type's default NATS role. Empty means the type's.
	Role string

	// NebulaIP, when set, mints a Nebula host at that overlay address. Most
	// field sensors get none: they reach the bus and nothing else.
	NebulaIP     string
	NebulaGroups []string

	// Inactive seeds a decommissioned device, so the inactive badge, the
	// `active` filter and stone_age_inactive_records have something to show.
	Inactive bool
}

var things = []thingFixture{
	// Northwind.
	{Org: "northwind", Code: "GW-KC-01", Name: "KC Edge Gateway", Type: "edge-gateway",
		Location: "KC-DC1-MDF", Description: "Runs the KC leaf node and the cold-chain rules.",
		NebulaIP: "10.20.10.1", NebulaGroups: []string{"gateway", "kc"},
		Metadata: map[string]any{"serial": "GW-8841-KC", "model": "Advantech UNO-2484G", "os": "Debian 13", "wan_type": "fibre"}},
	{Org: "northwind", Code: "GW-SGF-01", Name: "Springfield Edge Gateway", Type: "edge-gateway",
		Location: "SGF-XD2-MDF", Description: "Runs the Springfield leaf node.",
		NebulaIP: "10.20.10.2", NebulaGroups: []string{"gateway", "sgf"},
		Metadata: map[string]any{"serial": "GW-8841-SGF", "model": "Advantech UNO-2484G", "os": "Debian 13", "wan_type": "cable"}},
	{Org: "northwind", Code: "WMS-CONN-01", Name: "WMS Connector", Type: "wms-connector",
		Location: "KC-OFFICE", Description: "Single instance. Lives beside the WMS, not at the edge.",
		NebulaIP: "10.20.20.1", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "2.4.1", "wms_vendor": "Manhattan", "deployed": "2026-02-11"}},
	{Org: "northwind", Code: "RULES-COLD-01", Name: "Cold Chain Rules", Type: "coldchain-rules",
		Location: "KC-DC1-MDF", Description: "rule-router. Watches the temperature stream for excursions.",
		NebulaIP: "10.20.20.2", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "1.9.0", "rule_count": 34}},
	{Org: "northwind", Code: "REEFER-1180", Name: "Reefer 1180", Type: "reefer-unit", Location: "TRL-1180",
		Metadata: map[string]any{"serial": "CR-118034", "make": "Carrier", "model": "Vector 8600", "engine_hrs": 14820}},
	{Org: "northwind", Code: "REEFER-1184", Name: "Reefer 1184", Type: "reefer-unit", Location: "TRL-1184",
		Metadata: map[string]any{"serial": "TK-118041", "make": "Thermo King", "model": "Precedent S-750", "engine_hrs": 9310}},
	{Org: "northwind", Code: "DISP-KC-01", Name: "KC Dock Display", Type: "dock-display", Location: "KC-DC1",
		Description: "Above dock doors 1-12.",
		Metadata:    map[string]any{"panel_size": "55in", "orientation": "landscape"}},
	// Northwind: the stone-access estate.
	//
	// Every code below is also a `controllers` or `portals` row in stone-access,
	// with the identical code; change both sides together. They are lowercase
	// because stone-access mints them as NATS subject tokens
	// (acc.{location}.{type}.{thing}). Site codes go the other way: stone-access
	// takes KC-DC1, KC-OFFICE and SGF-XD2 from here.
	//
	// Only the controllers get a Nebula host: a door is I/O on a controller's
	// terminal block, not a network participant.
	{Org: "northwind", Code: "ctrl-kc-dc1-1", Name: "KC DC1 - Dock Panel", Type: "access-controller",
		Location: "KC-DC1", Description: "Drives the DC main entrance, Dock A and the Freezer Zone 1 door.",
		NebulaIP: "10.20.40.1", NebulaGroups: []string{"access", "kc"},
		Metadata: map[string]any{"model": "kincony-server-mini", "serial": "KC-SM-00418",
			"reader_bus": "/dev/ttyAMA0", "relays": 8, "inputs": 8}},
	{Org: "northwind", Code: "ctrl-kc-dc1-2", Name: "KC DC1 - Interior Panel", Type: "access-controller",
		Location: "KC-DC1-MDF", Description: "Drives the comms cabinet and the yard gate.",
		NebulaIP: "10.20.40.2", NebulaGroups: []string{"access", "kc"},
		Metadata: map[string]any{"model": "kincony-pi5r8", "serial": "KC-P5-00092",
			"reader_bus": "/dev/ttyAMA2", "relays": 8, "inputs": 8}},
	{Org: "northwind", Code: "ctrl-kc-office-1", Name: "KC Office Panel", Type: "access-controller",
		Location: "KC-OFFICE", Description: "Drives the office lobby and the server room.",
		NebulaIP: "10.20.40.3", NebulaGroups: []string{"access", "kc"},
		Metadata: map[string]any{"model": "kincony-server-mini", "serial": "KC-SM-00421",
			"reader_bus": "/dev/ttyAMA0", "relays": 8, "inputs": 8}},
	{Org: "northwind", Code: "ctrl-sgf-xd2-1", Name: "Springfield Panel", Type: "access-controller",
		Location: "SGF-XD2-MDF", Description: "Drives all three Springfield doors.",
		NebulaIP: "10.20.40.4", NebulaGroups: []string{"access", "sgf"},
		Metadata: map[string]any{"model": "kincony-pi5r8", "serial": "SGF-P5-00107",
			"reader_bus": "/dev/ttyAMA2", "relays": 8, "inputs": 8}},

	{Org: "northwind", Code: "kc-dc1-main", Name: "DC Main Entrance", Type: "access-door", Location: "KC-DC1",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 30, "installed": "2024-03-12"}},
	{Org: "northwind", Code: "kc-dc1-dock-a", Name: "Dock A Personnel Door", Type: "access-door", Location: "KC-DC1",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 45, "installed": "2024-03-12"}},
	{Org: "northwind", Code: "kc-dc1-freezer-1", Name: "Freezer Zone 1 Door", Type: "access-door", Location: "KC-DC1-FZ1",
		Description: "Maglock, not a strike: fail-safe so the fire panel drops it.",
		Metadata: map[string]any{"lock_type": "maglock", "reader_make": "HID Signo 20K",
			"reader_protocol": "osdp", "held_open_seconds": 60, "installed": "2024-06-04"}},
	{Org: "northwind", Code: "kc-dc1-mdf", Name: "Comms Cabinet Door", Type: "access-door", Location: "KC-DC1-MDF",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 15, "installed": "2024-03-12"}},
	{Org: "northwind", Code: "kc-dc1-yard", Name: "Yard Gate", Type: "access-gate", Location: "KC-DC1",
		Metadata: map[string]any{"operator_make": "LiftMaster CSW24U", "reader_protocol": "osdp",
			"held_open_seconds": 180, "loop_detector": true}},
	{Org: "northwind", Code: "kc-office-lobby", Name: "Office Lobby Door", Type: "access-door", Location: "KC-OFFICE",
		Description: "Unlocked on a schedule during business hours.",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 30, "installed": "2023-11-20"}},
	{Org: "northwind", Code: "kc-office-server", Name: "Office Server Room Door", Type: "access-door", Location: "KC-OFFICE",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 15, "installed": "2023-11-20"}},
	{Org: "northwind", Code: "sgf-xd2-main", Name: "Cross-Dock Main Entrance", Type: "access-door", Location: "SGF-XD2",
		Description: "disarm_on_grant: the first valid badge of the morning disarms the dock.",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 30, "installed": "2025-02-18"}},
	{Org: "northwind", Code: "sgf-xd2-dock-b", Name: "Dock B Personnel Door", Type: "access-door", Location: "SGF-XD2",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 45, "installed": "2025-02-18"}},
	{Org: "northwind", Code: "sgf-xd2-mdf", Name: "Springfield Comms Cabinet Door", Type: "access-door", Location: "SGF-XD2-MDF",
		Metadata: map[string]any{"lock_type": "strike", "reader_make": "HID Signo 20",
			"reader_protocol": "osdp", "held_open_seconds": 15, "installed": "2025-02-18"}},

	// Northwind: the tool crib estate.
	//
	// Every code below is also a `kiosks` row in the kiosk app; change both
	// sides together. kiosk/internal/demoseed/data.go is the other side. They
	// are uppercase because the kiosk app mints them that way for
	// `kiosk.<code>.event.…`.
	//
	// Every node gets a Nebula host: a kiosk is a mini-PC with its own database
	// that an administrator must reach when the bus is down.
	{Org: "northwind", Code: "KC-DC1-CRIB", Name: "Main Tool Crib", Type: "tool-kiosk", Location: "KC-DC1",
		Description: "Counter scanner plus two RFID cabinets. The flagship node.",
		NebulaIP:    "10.20.50.1", NebulaGroups: []string{"kiosk", "kc"},
		Metadata: map[string]any{"model": "Intel NUC 13 Pro", "os": "Debian 13", "port": 8101,
			"scanner": "usb-hid", "rfid_mode": "mixed", "enclosures": 2, "installed": "2026-04-08"}},
	{Org: "northwind", Code: "KC-DC1-DOCK", Name: "Dock Consumables", Type: "tool-kiosk", Location: "KC-DC1",
		Description: "What most installs actually are: a badge, a barcode scanner, and a shelf of PPE.",
		NebulaIP:    "10.20.50.2", NebulaGroups: []string{"kiosk", "kc"},
		Metadata: map[string]any{"model": "Intel NUC 13 Pro", "os": "Debian 13", "port": 8102,
			"scanner": "usb-hid", "rfid_mode": "none", "enclosures": 0, "installed": "2026-04-08"}},
	{Org: "northwind", Code: "SGF-XD2-CRIB", Name: "Cross-Dock Crib", Type: "tool-kiosk", Location: "SGF-XD2",
		Description: "Second site, one cabinet, a deliberately thinner slice of the KC catalogue. This is what makes the fleet a fleet.",
		NebulaIP:    "10.20.50.3", NebulaGroups: []string{"kiosk", "sgf"},
		Metadata: map[string]any{"model": "Intel NUC 13 Pro", "os": "Debian 13", "port": 8103,
			"scanner": "usb-hid", "rfid_mode": "mixed", "enclosures": 1, "installed": "2026-05-20"}},
	// KC-OFFICE has no crib; staff punch from phones. It exercises the
	// cross-kiosk clock-out gate: no local checkouts, yet it must see a tool out
	// at Springfield.
	{Org: "northwind", Code: "KC-OFFICE-TC", Name: "Office Timeclock", Type: "timeclock-terminal", Location: "KC-OFFICE",
		Description: "Virtual timeclock terminal. No checkout surface is registered on this binary, so none can be exposed.",
		NebulaIP:    "10.20.50.4", NebulaGroups: []string{"kiosk", "kc"},
		Metadata: map[string]any{"port": 8092, "auth": "both", "installed": "2026-05-20"}},
	// No kiosk_code: the controller is not a node, so it sits in the application
	// overlay range beside the WMS connector.
	{Org: "northwind", Code: "KIOSK-CTRL-01", Name: "Kiosk Controller", Type: "kiosk-controller", Location: "KC-DC1-MDF",
		Description: "Single instance. Aggregates all three nodes' ledgers and publishes their catalogue.",
		NebulaIP:    "10.20.20.3", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "1.0.0", "kiosks": 3, "deployed": "2026-04-08"}},

	{Org: "northwind", Code: "TP-KC-LEGACY-07", Name: "Legacy Probe 07", Type: "temp-probe", Location: "KC-DC1-FZ1",
		Description: "Replaced during the 2026 refit. Kept for its history.", Inactive: true,
		Metadata: map[string]any{"serial": "TP-LEG-0007", "firmware": "1.2.9", "probe_type": "air", "calibrated_on": "2024-08-19"}},

	// Ironbridge.
	{Org: "ironbridge", Code: "GW-PIT-01", Name: "Plant Edge Gateway", Type: "edge-gateway",
		Location: "PIT-MCC1", Description: "Runs the plant leaf node and the OEE pipeline.",
		NebulaIP: "10.30.10.1", NebulaGroups: []string{"gateway", "plant"},
		Metadata: map[string]any{"serial": "GW-2210-PIT", "model": "Dell Edge 3200", "os": "Ubuntu 26.04", "wan_type": "fibre"}},
	{Org: "ironbridge", Code: "LC-LINE-A", Name: "Line A Controller", Type: "line-controller", Location: "LINE-A",
		Metadata: map[string]any{"plc_make": "Rockwell", "plc_model": "CompactLogix 5380", "rack_slot": "1/0"}},
	{Org: "ironbridge", Code: "LC-LINE-B", Name: "Line B Controller", Type: "line-controller", Location: "LINE-B",
		Metadata: map[string]any{"plc_make": "Rockwell", "plc_model": "CompactLogix 5380", "rack_slot": "1/0"}},
	{Org: "ironbridge", Code: "LC-LINE-C", Name: "Line C Controller", Type: "line-controller", Location: "LINE-C",
		Metadata: map[string]any{"plc_make": "Siemens", "plc_model": "S7-1500", "rack_slot": "0/1"}},
	{Org: "ironbridge", Code: "OEE-01", Name: "OEE Analytics", Type: "oee-analytics", Location: "PIT-MCC1",
		Description: "eKuiper. Windows the cycle stream; this is the Layer 2 job the rule engine deliberately does not do.",
		NebulaIP:    "10.30.20.1", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "2.0.3", "window_mins": 15, "engine": "ekuiper"}},
	{Org: "ironbridge", Code: "MES-CONN-01", Name: "MES Connector", Type: "mes-connector", Location: "PIT-PLANT",
		NebulaIP: "10.30.20.2", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "1.1.7", "mes_vendor": "Siemens Opcenter"}},

	// Galewind.
	{Org: "galewind", Code: "GW-N1-01", Name: "North Substation Gateway", Type: "edge-gateway",
		Location: "SUB-N1-CAB", Description: "Cellular backhaul. The site keeps running when the WAN does not.",
		NebulaIP: "10.40.10.1", NebulaGroups: []string{"gateway", "substation"},
		Metadata: map[string]any{"serial": "GW-5501-N1", "model": "Moxa UC-8580", "os": "Debian 13", "wan_type": "cellular", "apn": "iot.galewind"}},
	{Org: "galewind", Code: "GW-S2-01", Name: "South Substation Gateway", Type: "edge-gateway",
		Location: "SUB-S2", Description: "Cellular backhaul.",
		NebulaIP: "10.40.10.2", NebulaGroups: []string{"gateway", "substation"},
		Metadata: map[string]any{"serial": "GW-5501-S2", "model": "Moxa UC-8580", "os": "Debian 13", "wan_type": "cellular", "apn": "iot.galewind"}},
	{Org: "galewind", Code: "TC-114", Name: "Turbine 114 Controller", Type: "turbine-ctl", Location: "T-114",
		Metadata: map[string]any{"make": "Vestas", "model": "V112-3.0", "scada_id": "N1-114", "rated_kw": 2800, "commissioned": "2021-09-30"}},
	{Org: "galewind", Code: "TC-118", Name: "Turbine 118 Controller", Type: "turbine-ctl", Location: "T-118",
		Metadata: map[string]any{"make": "Vestas", "model": "V112-3.0", "scada_id": "N1-118", "rated_kw": 2800, "commissioned": "2021-10-14"}},
	{Org: "galewind", Code: "TC-207", Name: "Turbine 207 Controller", Type: "turbine-ctl", Location: "T-207",
		Metadata: map[string]any{"make": "Nordex", "model": "N133-3.6", "scada_id": "S2-207", "rated_kw": 3200, "commissioned": "2023-05-08"}},
	{Org: "galewind", Code: "TC-211", Name: "Turbine 211 Controller", Type: "turbine-ctl", Location: "T-211",
		Description: "Gearbox replacement scheduled; controller removed from service.", Inactive: true,
		Metadata: map[string]any{"make": "Nordex", "model": "N133-3.6", "scada_id": "S2-211", "rated_kw": 3200, "commissioned": "2023-05-22"}},
	{Org: "galewind", Code: "MET-N1", Name: "North Met Mast", Type: "met-mast", Location: "SUB-N1",
		Metadata: map[string]any{"height_m": 80.0, "anemometer": "Thies First Class"}},
	{Org: "galewind", Code: "SCADA-BR-01", Name: "SCADA Bridge", Type: "scada-bridge", Location: "SWT-OPS",
		NebulaIP: "10.40.20.1", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "3.2.0", "historian": "OSIsoft PI"}},
	{Org: "galewind", Code: "MARKET-01", Name: "ERCOT Market Feed", Type: "market-feed", Location: "SWT-OPS",
		NebulaIP: "10.40.20.2", NebulaGroups: []string{"application"},
		Metadata: map[string]any{"version": "1.0.4", "iso": "ercot"}},
	{Org: "galewind", Code: "WALL-OPS-01", Name: "Operations Wallboard", Type: "ops-wallboard", Location: "SWT-OPS",
		Metadata: map[string]any{"panel_size": "75in", "orientation": "landscape"}},
}

// A lighthouse is a Nebula host, not a Thing: infrastructure, not inventory.
// Each org gets one so its network is usable.
type lighthouseFixture struct {
	Org, Hostname, OverlayIP, PublicHostPort string
	Groups                                   []string
}

var lighthouses = []lighthouseFixture{
	{Org: "northwind", Hostname: "lighthouse-1", OverlayIP: "10.20.0.1",
		PublicHostPort: "lighthouse.northwind.example:4242", Groups: []string{"lighthouse"}},
	{Org: "ironbridge", Hostname: "lighthouse-1", OverlayIP: "10.30.0.1",
		PublicHostPort: "lighthouse.ironbridge.example:4242", Groups: []string{"lighthouse"}},
	{Org: "galewind", Hostname: "lighthouse-1", OverlayIP: "10.40.0.1",
		PublicHostPort: "lighthouse.galewind.example:4242", Groups: []string{"lighthouse"}},
}

// DemoPassword is shared by every seeded login. It is printed by the command and
// is the reason --confirm exists.
const DemoPassword = "demo1234"

type personFixture struct {
	Email, Name string

	// Operator marks a platform operator, so the Organizations screen is
	// reachable without a superuser login.
	Operator bool

	// Roles maps org code to membership role. All five roles appear at least
	// once, so each can be tried in the console.
	Roles map[string]string
}

// natsRoleForConsoleRole maps a console role to the NATS role a membership's
// identity gets.
//
// It is derived per membership, not a field on personFixture, because one
// person can hold different console roles in different orgs (Casey below), and
// a per-person field gave a read-only viewer publish rights. Nothing in the
// platform enforces this pairing; the seed only demonstrates the correct one.
func natsRoleForConsoleRole(role string) string {
	switch role {
	case "viewer", "dashboard":
		return "console-readonly"
	default:
		// owner, admin and member all publish from the console (the Publisher
		// widget).
		return "console-operator"
	}
}

var people = []personFixture{
	{Email: "dana@northwind.example", Name: "Dana Whitfield",
		Roles: map[string]string{"northwind": "owner"}},
	{Email: "raj@northwind.example", Name: "Raj Malhotra",
		Roles: map[string]string{"northwind": "admin"}},
	{Email: "elena@northwind.example", Name: "Elena Sokolova",
		Roles: map[string]string{"northwind": "member"}},
	{Email: "audit@northwind.example", Name: "Northwind Compliance",
		Roles: map[string]string{"northwind": "viewer"}},
	{Email: "dock-display@northwind.example", Name: "KC Dock Screen",
		Roles: map[string]string{"northwind": "dashboard"}},

	{Email: "marcus@ironbridge.example", Name: "Marcus Adeyemi",
		Roles: map[string]string{"ironbridge": "owner"}},
	{Email: "lin@ironbridge.example", Name: "Lin Chen",
		Roles: map[string]string{"ironbridge": "member"}},
	{Email: "wallboard@ironbridge.example", Name: "Line A Wallboard",
		Roles: map[string]string{"ironbridge": "dashboard"}},

	{Email: "sofia@galewind.example", Name: "Sofia Rendon",
		Roles: map[string]string{"galewind": "owner"}},
	{Email: "tom@galewind.example", Name: "Tom Blackwood",
		Roles: map[string]string{"galewind": "admin"}},
	{Email: "iso@galewind.example", Name: "Grid Compliance",
		Roles: map[string]string{"galewind": "viewer"}},

	// Service logins for the inventory feed (ADR 0004 in platform-docs;
	// demo/inventory): nats-auth-manager keeps the token in the org's `tokens`
	// KV bucket and rule-router polls the list API with it. Viewer only, because
	// anything with `$KV.tokens.>` can read that token. Its NATS identity
	// (console-inventory-feed) is minted like everyone else's and unused.
	{Email: "inventory-feed@northwind.example", Name: "Northwind Inventory Feed",
		Roles: map[string]string{"northwind": "viewer"}},
	{Email: "inventory-feed@ironbridge.example", Name: "Ironbridge Inventory Feed",
		Roles: map[string]string{"ironbridge": "viewer"}},
	{Email: "inventory-feed@galewind.example", Name: "Galewind Inventory Feed",
		Roles: map[string]string{"galewind": "viewer"}},

	// One person in three tenants at different authority levels, for the org
	// switcher. `current_organization` is server-side: switching in one tab
	// rescopes the other.
	{Email: "casey@msp.example", Name: "Casey Nakamura", Operator: true,
		Roles: map[string]string{"northwind": "admin", "ironbridge": "admin", "galewind": "viewer"}},
}
