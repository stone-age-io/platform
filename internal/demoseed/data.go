package demoseed

// The fixtures. Everything here is fictional.
//
// The three organizations deliberately share their CODES with the three
// customers the helpdesk's own demo seed marks as platform organizations
// (northwind, ironbridge, galewind). That is not decoration: organizations.code
// is the ecosystem's one globally unique identifier (ADR 0002), and it is the
// token a managed org's exported `helpdesk.>` traffic is rewritten with. Seeding
// both sides with the same three codes means the two demos JOIN — a ticket in
// the helpdesk and a Thing here name the same tenant, which is the whole claim
// the code exists to support. Renaming an org here without renaming it there
// breaks a demo that is otherwise hard to assemble by hand.

// ---------------------------------------------------------------- shorthands

// objSchema is a JSON Schema shorthand. The seeder only ever writes flat object
// schemas, so these few helpers cover every metadata_schema and every
// message_schemas.schema below.
func objSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(title string) map[string]any   { return map[string]any{"type": "string", "title": title} }
func num(title string) map[string]any   { return map[string]any{"type": "number", "title": title} }
func intF(title string) map[string]any  { return map[string]any{"type": "integer", "title": title} }
func boolF(title string) map[string]any { return map[string]any{"type": "boolean", "title": title} }

func date(title string) map[string]any {
	return map[string]any{"type": "string", "format": "date", "title": title}
}

func stamp(title string) map[string]any {
	return map[string]any{"type": "string", "format": "date-time", "title": title}
}

func enum(title string, vals ...string) map[string]any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return map[string]any{"type": "string", "title": title, "enum": out}
}

// ------------------------------------------------------------- organizations

type orgFixture struct {
	Code, Name, Description string

	// Managed provisions the `helpdesk.>` stream export on this org's account
	// plus the hub-side import remapped to helpdesk.<code>.>. Two of the three
	// are managed so a reader sees both states side by side; a demo where every
	// org is managed teaches nothing about what the flag does.
	Managed bool

	// NebulaCIDR is the org's overlay range. Distinct per org, so a screenshot
	// of two organizations' hosts cannot be mistaken for one network.
	NebulaCIDR string
}

var orgs = []orgFixture{
	{
		Code: "northwind", Name: "Northwind Traders", Managed: true,
		Description: "Cold-chain distribution. Two distribution centres and a refrigerated trailer fleet.",
		NebulaCIDR:  "10.20.0.0/16",
	},
	{
		Code: "ironbridge", Name: "Ironbridge Manufacturing", Managed: false,
		Description: "Discrete manufacturing. One plant, three production lines, on-premise only.",
		NebulaCIDR:  "10.30.0.0/16",
	},
	{
		Code: "galewind", Name: "Galewind Energy", Managed: true,
		Description: "Wind generation and collection. Unmanned substations on cellular backhaul.",
		NebulaCIDR:  "10.40.0.0/16",
	},
}

// ------------------------------------------------------------- location types

type typeFixture struct {
	Org, Code, Name, Description string
	Schema                       map[string]any
}

var locationTypes = []typeFixture{
	// Northwind. The warehouse/office pair matches the helpdesk's own fixtures
	// for this customer, so someone comparing the two apps sees one taxonomy.
	{Org: "northwind", Code: "warehouse", Name: "Warehouse", Description: "Distribution centre.",
		Schema: objSchema(map[string]any{
			"dock_doors":     intF("Dock doors"),
			"sqft":           intF("Square feet"),
			"temp_class":     enum("Temperature class", "ambient", "chilled", "frozen"),
			"sprinklered":    boolF("Sprinklered"),
			"last_inspected": date("Last inspected"),
		})},
	{Org: "northwind", Code: "office", Name: "Office"},
	{Org: "northwind", Code: "zone", Name: "Storage Zone",
		Description: "A temperature-controlled area inside a warehouse.",
		Schema: objSchema(map[string]any{
			"setpoint_c":  num("Setpoint (C)"),
			"tolerance_c": num("Alarm tolerance (+/- C)"),
			"rack_count":  intF("Rack positions"),
		})},
	{Org: "northwind", Code: "trailer", Name: "Refrigerated Trailer",
		Description: "Mobile asset. Its location is where it was last seen.",
		Schema: objSchema(map[string]any{
			"vin":              str("VIN"),
			"reefer_make":      str("Reefer make"),
			"capacity_pallets": intF("Pallet capacity"),
		})},
	{Org: "northwind", Code: "cabinet", Name: "Equipment Cabinet"},

	// Ironbridge.
	{Org: "ironbridge", Code: "plant", Name: "Plant", Description: "Production facility.",
		Schema: objSchema(map[string]any{
			"shift_pattern": enum("Shift pattern", "1x8", "2x8", "3x8"),
			"hazmat":        boolF("Hazmat on site"),
			"union_site":    boolF("Union site"),
		})},
	{Org: "ironbridge", Code: "building", Name: "Building"},
	{Org: "ironbridge", Code: "line", Name: "Production Line",
		Schema: objSchema(map[string]any{
			"takt_seconds":   num("Takt time (s)"),
			"cell_count":     intF("Work cells"),
			"product_family": str("Product family"),
		})},
	{Org: "ironbridge", Code: "cabinet", Name: "Control Cabinet"},

	// Galewind.
	{Org: "galewind", Code: "substation", Name: "Substation",
		Description: "Collection or transmission substation.",
		Schema: objSchema(map[string]any{
			"voltage_kv":     num("Voltage (kV)"),
			"manned":         boolF("Manned"),
			"last_inspected": date("Last inspected"),
			"backhaul":       enum("Backhaul", "fibre", "cellular", "satellite"),
		})},
	{Org: "galewind", Code: "turbine", Name: "Turbine Pad",
		Schema: objSchema(map[string]any{
			"rated_kw":     intF("Rated output (kW)"),
			"hub_height_m": num("Hub height (m)"),
			"commissioned": date("Commissioned"),
		})},
	{Org: "galewind", Code: "control-room", Name: "Control Room"},
	{Org: "galewind", Code: "cabinet", Name: "Field Cabinet"},
}

// ----------------------------------------------------------------- locations

type locationFixture struct {
	Org, Code, Name, Type, Parent, Description string

	// Real coordinates. The Locations map and the map widget are two of the
	// console's best screens and both are blank without them.
	Lat, Lon float64

	Metadata map[string]any
}

// Two levels of nesting in each org, so the tree view has something to collapse.
// A parent is referenced by code and may appear later in this list, which is why
// seedLocations runs two passes.
var locations = []locationFixture{
	// -------------------------------------------------------------- northwind
	{Org: "northwind", Code: "KC-DC1", Name: "Kansas City Distribution Center", Type: "warehouse",
		Lat: 39.1141, Lon: -94.6275,
		Description: "Primary frozen and chilled DC.",
		Metadata: map[string]any{
			"dock_doors": 42, "sqft": 310000, "temp_class": "frozen",
			"sprinklered": true, "last_inspected": "2026-05-18",
		}},
	{Org: "northwind", Code: "KC-DC1-FZ1", Name: "Freezer Zone 1", Type: "zone", Parent: "KC-DC1",
		Lat: 39.1141, Lon: -94.6275,
		Metadata: map[string]any{"setpoint_c": -20.0, "tolerance_c": 2.0, "rack_count": 880}},
	{Org: "northwind", Code: "KC-DC1-FZ2", Name: "Freezer Zone 2", Type: "zone", Parent: "KC-DC1",
		Lat: 39.1141, Lon: -94.6275,
		Metadata: map[string]any{"setpoint_c": -20.0, "tolerance_c": 2.0, "rack_count": 640}},
	{Org: "northwind", Code: "KC-DC1-CH1", Name: "Chilled Zone 1", Type: "zone", Parent: "KC-DC1",
		Lat: 39.1141, Lon: -94.6275,
		Metadata: map[string]any{"setpoint_c": 3.0, "tolerance_c": 1.5, "rack_count": 520}},
	{Org: "northwind", Code: "KC-DC1-MDF", Name: "KC Comms Cabinet", Type: "cabinet", Parent: "KC-DC1",
		Lat: 39.1141, Lon: -94.6275},
	{Org: "northwind", Code: "KC-OFFICE", Name: "Kansas City Office", Type: "office", Parent: "KC-DC1",
		Lat: 39.1148, Lon: -94.6261},
	{Org: "northwind", Code: "SGF-XD2", Name: "Springfield Cross-Dock", Type: "warehouse",
		Lat: 37.2153, Lon: -93.2982,
		Description: "Ambient cross-dock, no long-term storage.",
		Metadata: map[string]any{
			"dock_doors": 18, "sqft": 74000, "temp_class": "ambient",
			"sprinklered": true, "last_inspected": "2026-03-02",
		}},
	{Org: "northwind", Code: "SGF-XD2-CH1", Name: "Springfield Chilled Bay", Type: "zone", Parent: "SGF-XD2",
		Lat: 37.2153, Lon: -93.2982,
		Metadata: map[string]any{"setpoint_c": 4.0, "tolerance_c": 2.0, "rack_count": 96}},
	{Org: "northwind", Code: "SGF-XD2-MDF", Name: "Springfield Comms Cabinet", Type: "cabinet", Parent: "SGF-XD2",
		Lat: 37.2153, Lon: -93.2982},
	{Org: "northwind", Code: "TRL-1180", Name: "Trailer 1180", Type: "trailer",
		Lat: 39.0997, Lon: -94.5786,
		Description: "In yard at KC-DC1.",
		Metadata:    map[string]any{"vin": "1NKDX4EX0PJ118034", "reefer_make": "Carrier", "capacity_pallets": 26}},
	{Org: "northwind", Code: "TRL-1184", Name: "Trailer 1184", Type: "trailer",
		Lat: 38.6270, Lon: -90.1994,
		Description: "In transit, last seen St. Louis.",
		Metadata:    map[string]any{"vin": "1NKDX4EX7PJ118041", "reefer_make": "Thermo King", "capacity_pallets": 26}},

	// ------------------------------------------------------------- ironbridge
	{Org: "ironbridge", Code: "PIT-PLANT", Name: "Pittsburgh Works", Type: "plant",
		Lat: 40.4406, Lon: -79.9959,
		Description: "Main production site.",
		Metadata:    map[string]any{"shift_pattern": "3x8", "hazmat": true, "union_site": true}},
	{Org: "ironbridge", Code: "PIT-B1", Name: "Building 1 - Fabrication", Type: "building", Parent: "PIT-PLANT",
		Lat: 40.4409, Lon: -79.9964},
	{Org: "ironbridge", Code: "PIT-B2", Name: "Building 2 - Assembly", Type: "building", Parent: "PIT-PLANT",
		Lat: 40.4402, Lon: -79.9951},
	{Org: "ironbridge", Code: "LINE-A", Name: "Line A - Press", Type: "line", Parent: "PIT-B1",
		Lat: 40.4409, Lon: -79.9964,
		Metadata: map[string]any{"takt_seconds": 42.0, "cell_count": 6, "product_family": "Stamped housings"}},
	{Org: "ironbridge", Code: "LINE-B", Name: "Line B - Weld", Type: "line", Parent: "PIT-B1",
		Lat: 40.4409, Lon: -79.9964,
		Metadata: map[string]any{"takt_seconds": 58.0, "cell_count": 4, "product_family": "Frame weldments"}},
	{Org: "ironbridge", Code: "LINE-C", Name: "Line C - Final Assembly", Type: "line", Parent: "PIT-B2",
		Lat: 40.4402, Lon: -79.9951,
		Metadata: map[string]any{"takt_seconds": 96.0, "cell_count": 9, "product_family": "Finished units"}},
	{Org: "ironbridge", Code: "PIT-MCC1", Name: "MCC Room 1", Type: "cabinet", Parent: "PIT-B1",
		Lat: 40.4409, Lon: -79.9964},

	// --------------------------------------------------------------- galewind
	{Org: "galewind", Code: "SWT-OPS", Name: "Sweetwater Operations Center", Type: "control-room",
		Lat: 32.4709, Lon: -100.4059,
		Description: "Staffed 24/7. The only site on fibre."},
	{Org: "galewind", Code: "SUB-N1", Name: "North Collection Substation", Type: "substation",
		Lat: 32.5731, Lon: -100.3620,
		Description: "Unmanned. Cellular backhaul.",
		Metadata:    map[string]any{"voltage_kv": 34.5, "manned": false, "last_inspected": "2026-06-11", "backhaul": "cellular"}},
	{Org: "galewind", Code: "SUB-S2", Name: "South Collection Substation", Type: "substation",
		Lat: 32.3688, Lon: -100.4712,
		Description: "Unmanned. Cellular backhaul.",
		Metadata:    map[string]any{"voltage_kv": 34.5, "manned": false, "last_inspected": "2026-04-27", "backhaul": "cellular"}},
	{Org: "galewind", Code: "SUB-N1-CAB", Name: "North Field Cabinet", Type: "cabinet", Parent: "SUB-N1",
		Lat: 32.5731, Lon: -100.3620},
	{Org: "galewind", Code: "T-114", Name: "Turbine 114", Type: "turbine", Parent: "SUB-N1",
		Lat: 32.5810, Lon: -100.3441,
		Metadata: map[string]any{"rated_kw": 2800, "hub_height_m": 89.0, "commissioned": "2021-09-30"}},
	{Org: "galewind", Code: "T-118", Name: "Turbine 118", Type: "turbine", Parent: "SUB-N1",
		Lat: 32.5904, Lon: -100.3517,
		Metadata: map[string]any{"rated_kw": 2800, "hub_height_m": 89.0, "commissioned": "2021-10-14"}},
	{Org: "galewind", Code: "T-207", Name: "Turbine 207", Type: "turbine", Parent: "SUB-S2",
		Lat: 32.3591, Lon: -100.4880,
		Metadata: map[string]any{"rated_kw": 3200, "hub_height_m": 94.0, "commissioned": "2023-05-08"}},
	{Org: "galewind", Code: "T-211", Name: "Turbine 211", Type: "turbine", Parent: "SUB-S2",
		Lat: 32.3502, Lon: -100.4761,
		Metadata: map[string]any{"rated_kw": 3200, "hub_height_m": 94.0, "commissioned": "2023-05-22"}},
}
