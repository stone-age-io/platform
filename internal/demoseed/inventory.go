package demoseed

// Curated things, edge sites, overlay networks, and people.
//
// Nothing here sets floorplan_position, deliberately. Those are PIXEL
// coordinates on the location's uploaded floorplan image, and this seed uploads
// no images — a coordinate against an absent (or later, differently-sized) plan
// would place markers at meaningless spots. Unplaced is a real state the console
// models: the things below show up in the floor-plan positioning drawer, which
// is the workflow worth demonstrating anyway.

// ------------------------------------------------------------ overlay network

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

// ------------------------------------------------------------ curated things

type thingFixture struct {
	Org, Code, Name, Type, Location, Description string
	Metadata                                     map[string]any

	// Role overrides the thing type's default NATS role. Empty means "use the
	// type's". Set explicitly only where the point is that it differs.
	Role string

	// NebulaIP, when set, mints a Nebula host for this thing at that overlay
	// address. Most field sensors get no Nebula host at all: they reach the bus
	// and nothing else, which is the design, and giving every sensor a mesh
	// certificate in the demo would misrepresent it.
	NebulaIP     string
	NebulaGroups []string

	// Inactive seeds a decommissioned device. Two of these exist so the console's
	// inactive badge, the `active` filter and stone_age_inactive_records all have
	// something to show — and so the state is visibly normal rather than alarming.
	Inactive bool
}

var things = []thingFixture{
	// ------------------------------------------------------------- northwind
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
	{Org: "northwind", Code: "TP-KC-LEGACY-07", Name: "Legacy Probe 07", Type: "temp-probe", Location: "KC-DC1-FZ1",
		Description: "Replaced during the 2026 refit. Kept for its history.", Inactive: true,
		Metadata: map[string]any{"serial": "TP-LEG-0007", "firmware": "1.2.9", "probe_type": "air", "calibrated_on": "2024-08-19"}},

	// ------------------------------------------------------------ ironbridge
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

	// -------------------------------------------------------------- galewind
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

// ------------------------------------------------------------- lighthouses

// A lighthouse is a Nebula host, not a Thing — it is infrastructure rather than
// inventory, and giving it a Thing record would put a rendezvous server in the
// device list. Each org gets one so its network is actually usable.
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

// -------------------------------------------------------------- edge sites

type leafFixture struct {
	Org, Code, Name, Location, Description string

	// NebulaIP mints the leaf node's own overlay address. The leaf node's NATS
	// user is NOT set here: RegisterLeafNodeProvisioning mints it on create, and
	// the seed goes in through that hook rather than around it.
	NebulaIP     string
	NebulaGroups []string

	// Collections to mirror into the edge's local KV. Constrained server-side by
	// leafsync.allowedCollections regardless of what is written here.
	Synced []string
}

// domain is derived as edge-<code>, matching what LeafNodeFormView derives in
// the console. Left implicit in the fixture so the two cannot drift.
var leafNodes = []leafFixture{
	{Org: "northwind", Code: "kc-dc1", Name: "Kansas City Edge", Location: "KC-DC1-MDF",
		Description: "Leaf node for the KC distribution centre.",
		NebulaIP:    "10.20.30.1", NebulaGroups: []string{"leaf", "kc"},
		Synced: []string{"things", "locations", "thing_types", "location_types", "thing_type_operations", "message_schemas"}},
	{Org: "northwind", Code: "sgf-xd2", Name: "Springfield Edge", Location: "SGF-XD2-MDF",
		Description: "Leaf node for the Springfield cross-dock.",
		NebulaIP:    "10.20.30.2", NebulaGroups: []string{"leaf", "sgf"},
		Synced: []string{"things", "locations", "thing_types", "location_types"}},
	{Org: "ironbridge", Code: "pit-plant", Name: "Pittsburgh Works Edge", Location: "PIT-MCC1",
		Description: "Leaf node for the plant. Runs alongside the OEE pipeline.",
		NebulaIP:    "10.30.30.1", NebulaGroups: []string{"leaf", "plant"},
		Synced: []string{"things", "locations", "thing_types", "location_types", "thing_type_operations", "message_schemas"}},
	{Org: "galewind", Code: "sub-n1", Name: "North Substation Edge", Location: "SUB-N1-CAB",
		Description: "Leaf node on cellular. The reason the site keeps deciding locally during an outage.",
		NebulaIP:    "10.40.30.1", NebulaGroups: []string{"leaf", "substation"},
		Synced: []string{"things", "locations", "thing_types", "location_types", "thing_type_operations", "message_schemas"}},
	{Org: "galewind", Code: "sub-s2", Name: "South Substation Edge", Location: "SUB-S2",
		Description: "Leaf node on cellular.",
		NebulaIP:    "10.40.30.2", NebulaGroups: []string{"leaf", "substation"},
		Synced: []string{"things", "locations", "thing_types", "location_types"}},
}

// ------------------------------------------------------------------- people

// DemoPassword is shared by every seeded login. It is printed by the command and
// is the reason --confirm exists.
const DemoPassword = "demo1234"

type personFixture struct {
	Email, Name string

	// Operator marks a platform operator: someone who can manage organizations
	// themselves. One is seeded so the Organizations screen is reachable without
	// logging in as a superuser.
	Operator bool

	// Roles maps org code to membership role. Every one of the five roles is
	// represented at least once, because scripts/test-authz.sh is the only other
	// place they all appear and a person exploring the console should be able to
	// log in as each and see the difference.
	Roles map[string]string
}

// natsRoleForConsoleRole maps a console role to the NATS role a membership's
// identity gets.
//
// This is a FUNCTION rather than a field on personFixture, and that is the whole
// point. It was a field first, and the first person who held two different
// console roles in two organizations — Casey below, an admin in two tenants and
// a viewer in a third — got one NATS role across all three, so the demo shipped
// a "read-only" auditor who could publish anywhere. The pairing has to be
// per-membership because the console role is, and deriving it here makes that
// structural instead of something each fixture has to remember.
//
// A console role is still NOT a NATS role: nothing in the platform enforces this
// mapping, and an operator is free to link any identity to any membership. What
// this buys is that the seeded data demonstrates the correct pairing rather than
// the trap.
func natsRoleForConsoleRole(role string) string {
	switch role {
	case "viewer", "dashboard":
		return "console-readonly"
	default:
		// owner, admin and member all administer or edit inventory, and all
		// three legitimately publish from the console — testing an operation
		// with the Publisher widget is ordinary work for them.
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

	// One person in two tenants at different authority levels. This is what the
	// organization switcher exists for, and it is also the fastest way to see
	// that `current_organization` is server-side session state: switching in one
	// tab rescopes the other.
	{Email: "casey@msp.example", Name: "Casey Nakamura", Operator: true,
		Roles: map[string]string{"northwind": "admin", "ironbridge": "admin", "galewind": "viewer"}},
}
