package demoseed

// The contract layer and the NATS permission templates.
//
// Subject prefixes use the reserved template variables from
// ui/src/utils/subjectResolver.ts — {org}, {location}, {thing},
// {thing_type_code} — rather than literal strings, because that is what the
// console renders and what a role pattern is derived from ({thing} collapses to
// "*"). Fixtures written with literal subjects would look fine on the Thing Type
// screen and quietly teach the wrong thing.
//
// No prefix here carries an organization token. Inside NATS, the ACCOUNT is the
// tenant boundary — every subject below is already private to one org — so an
// org token in the subject would be redundant here and misleading next to the
// managed-org export, which adds exactly one such token on the way OUT of the
// account and is the only place it belongs.

// ------------------------------------------------------------ message schemas

type schemaFixture struct {
	Org, Namespace, Name, Version, Description string
	Schema                                     map[string]any
}

// schemaKey is how a fixture refers to a schema from an operation, and how the
// seeder keys them internally. It matches the composite leaf-sync mirrors these
// records under (namespace__name__version).
func schemaKey(org, namespace, name, version string) string {
	return org + ":" + namespace + "__" + name + "__" + version
}

// A small catalogue, seeded into each org that references it. message_schemas is
// org-scoped, so "shared" here means the same document is written per tenant —
// which is the honest shape: nothing in the platform lets two orgs read one
// schema record, and pretending otherwise in a demo would suggest a cross-tenant
// read that does not exist.
var messageSchemas = []schemaFixture{
	// ---- northwind
	{Org: "northwind", Namespace: "telemetry", Name: "temperature", Version: "1.0.0",
		Description: "A single temperature reading from a probe or a reefer unit.",
		Schema: objSchema(map[string]any{
			"ts":         stamp("Sampled at"),
			"celsius":    num("Temperature (C)"),
			"setpoint_c": num("Active setpoint (C)"),
			"probe":      str("Probe identifier"),
		}, "ts", "celsius")},
	{Org: "northwind", Namespace: "telemetry", Name: "battery", Version: "1.0.0",
		Description: "Battery state for a wireless sensor.",
		Schema: objSchema(map[string]any{
			"ts":      stamp("Sampled at"),
			"percent": intF("Charge (%)"),
			"volts":   num("Terminal voltage"),
		}, "ts", "percent")},
	{Org: "northwind", Namespace: "event", Name: "door_state", Version: "1.0.0",
		Description: "A dock or zone door changed state.",
		Schema: objSchema(map[string]any{
			"ts":       stamp("Observed at"),
			"state":    enum("State", "open", "closed"),
			"duration": intF("Seconds in previous state"),
		}, "ts", "state")},
	{Org: "northwind", Namespace: "command", Name: "setpoint", Version: "1.0.0",
		Description: "Instruct a unit to adopt a temperature setpoint. Paired with an echo, not a measurement.",
		Schema: objSchema(map[string]any{
			"celsius":    num("Requested setpoint (C)"),
			"issued_by":  str("Requesting identity"),
			"expires_at": stamp("Ignore after"),
		}, "celsius")},
	{Org: "northwind", Namespace: "status", Name: "heartbeat", Version: "1.0.0",
		Description: "Liveness beacon.",
		Schema: objSchema(map[string]any{
			"ts":     stamp("Sent at"),
			"uptime": intF("Uptime (s)"),
		}, "ts")},
	// A second version of the same document, so the Message Schemas screen shows
	// what versioning looks like before a reader has to create one.
	{Org: "northwind", Namespace: "status", Name: "heartbeat", Version: "1.1.0",
		Description: "Liveness beacon. Adds the reporting agent's build.",
		Schema: objSchema(map[string]any{
			"ts":      stamp("Sent at"),
			"uptime":  intF("Uptime (s)"),
			"version": str("Agent version"),
		}, "ts")},

	// ---- ironbridge
	{Org: "ironbridge", Namespace: "telemetry", Name: "power", Version: "1.0.0",
		Description: "Three-phase electrical measurement from a panel meter.",
		Schema: objSchema(map[string]any{
			"ts":         stamp("Sampled at"),
			"kw":         num("Real power (kW)"),
			"kvar":       num("Reactive power (kVAr)"),
			"pf":         num("Power factor"),
			"voltage_ll": num("Line-to-line volts"),
		}, "ts", "kw")},
	{Org: "ironbridge", Namespace: "telemetry", Name: "vibration", Version: "1.0.0",
		Description: "Bearing vibration summary, already reduced on the device.",
		Schema: objSchema(map[string]any{
			"ts":        stamp("Sampled at"),
			"rms_mm_s":  num("RMS velocity (mm/s)"),
			"peak_mm_s": num("Peak velocity (mm/s)"),
			"axis":      enum("Axis", "x", "y", "z"),
		}, "ts", "rms_mm_s")},
	{Org: "ironbridge", Namespace: "event", Name: "cycle", Version: "1.0.0",
		Description: "One completed machine cycle. The unit of OEE.",
		Schema: objSchema(map[string]any{
			"ts":       stamp("Completed at"),
			"seconds":  num("Cycle time (s)"),
			"good":     boolF("Passed inspection"),
			"part":     str("Part number"),
			"operator": str("Operator badge"),
		}, "ts", "seconds", "good")},
	{Org: "ironbridge", Namespace: "event", Name: "alarm", Version: "1.0.0",
		Description: "A machine or line alarm was raised or cleared.",
		Schema: objSchema(map[string]any{
			"ts":       stamp("Raised at"),
			"code":     str("Alarm code"),
			"severity": enum("Severity", "info", "warning", "critical"),
			"active":   boolF("Still active"),
			"message":  str("Human-readable text"),
		}, "ts", "code", "severity", "active")},
	{Org: "ironbridge", Namespace: "command", Name: "line_mode", Version: "1.0.0",
		Description: "Instruct a line controller to change running mode.",
		Schema: objSchema(map[string]any{
			"mode":      enum("Mode", "run", "hold", "changeover", "maintenance"),
			"issued_by": str("Requesting identity"),
		}, "mode")},
	{Org: "ironbridge", Namespace: "status", Name: "heartbeat", Version: "1.0.0",
		Description: "Liveness beacon.",
		Schema: objSchema(map[string]any{
			"ts":     stamp("Sent at"),
			"uptime": intF("Uptime (s)"),
		}, "ts")},

	// ---- galewind
	{Org: "galewind", Namespace: "telemetry", Name: "generation", Version: "1.0.0",
		Description: "Turbine output sample.",
		Schema: objSchema(map[string]any{
			"ts":          stamp("Sampled at"),
			"kw":          num("Output (kW)"),
			"wind_ms":     num("Wind speed (m/s)"),
			"nacelle_deg": num("Nacelle bearing (deg)"),
			"rpm":         num("Rotor speed (rpm)"),
		}, "ts", "kw")},
	{Org: "galewind", Namespace: "telemetry", Name: "feeder", Version: "1.0.0",
		Description: "Substation feeder measurement.",
		Schema: objSchema(map[string]any{
			"ts":     stamp("Sampled at"),
			"amps":   num("Current (A)"),
			"kv":     num("Voltage (kV)"),
			"feeder": str("Feeder identifier"),
		}, "ts", "amps", "kv")},
	{Org: "galewind", Namespace: "event", Name: "alarm", Version: "1.0.0",
		Description: "A protection or plant alarm.",
		Schema: objSchema(map[string]any{
			"ts":       stamp("Raised at"),
			"code":     str("Alarm code"),
			"severity": enum("Severity", "info", "warning", "critical"),
			"active":   boolF("Still active"),
			"message":  str("Human-readable text"),
		}, "ts", "code", "severity", "active")},
	{Org: "galewind", Namespace: "command", Name: "curtail", Version: "1.0.0",
		Description: "Curtail a turbine to a ceiling, expressed as a percentage of rating.",
		Schema: objSchema(map[string]any{
			"percent":    intF("Ceiling (% of rated)"),
			"reason":     enum("Reason", "grid", "noise", "wildlife", "maintenance"),
			"issued_by":  str("Requesting identity"),
			"expires_at": stamp("Ignore after"),
		}, "percent", "reason")},
	{Org: "galewind", Namespace: "status", Name: "heartbeat", Version: "1.0.0",
		Description: "Liveness beacon.",
		Schema: objSchema(map[string]any{
			"ts":     stamp("Sent at"),
			"uptime": intF("Uptime (s)"),
		}, "ts")},
}

// ----------------------------------------------------------------- operations

type operationFixture struct {
	Org, Name, Capability, SubjectSuffix, Description string

	// SchemaNS/SchemaName/SchemaVersion name the message_schemas record to link,
	// or are empty for an operation whose payload is not described. Both states
	// exist on purpose: the console renders them differently, and an operator
	// should see that a schema is optional.
	SchemaNS, SchemaName, SchemaVersion string
}

var operations = []operationFixture{
	// ---- northwind
	{Org: "northwind", Name: "publish_temperature", Capability: "publish", SubjectSuffix: "temperature",
		Description: "Periodic temperature sample.",
		SchemaNS:    "telemetry", SchemaName: "temperature", SchemaVersion: "1.0.0"},
	{Org: "northwind", Name: "publish_battery", Capability: "publish", SubjectSuffix: "battery",
		Description: "Battery state, sent hourly and on change.",
		SchemaNS:    "telemetry", SchemaName: "battery", SchemaVersion: "1.0.0"},
	{Org: "northwind", Name: "publish_door", Capability: "publish", SubjectSuffix: "door",
		Description: "Door open/closed transition.",
		SchemaNS:    "event", SchemaName: "door_state", SchemaVersion: "1.0.0"},
	{Org: "northwind", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon.",
		SchemaNS:    "status", SchemaName: "heartbeat", SchemaVersion: "1.1.0"},
	{Org: "northwind", Name: "subscribe_setpoint", Capability: "subscribe", SubjectSuffix: "setpoint",
		Description: "Accept a new temperature setpoint.",
		SchemaNS:    "command", SchemaName: "setpoint", SchemaVersion: "1.0.0"},
	{Org: "northwind", Name: "publish_setpoint_echo", Capability: "publish", SubjectSuffix: "setpoint.echo",
		Description: "Echo the setpoint actually in force. This is the property a desired value should be paired with.",
		SchemaNS:    "command", SchemaName: "setpoint", SchemaVersion: "1.0.0"},
	{Org: "northwind", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request. Payload is device-specific and deliberately unschemad."},
	{Org: "northwind", Name: "subscribe_render", Capability: "subscribe", SubjectSuffix: "render",
		Description: "Screen contents for an unattended display."},
	{Org: "northwind", Name: "publish_shipment", Capability: "publish", SubjectSuffix: "shipment",
		Description: "A shipment event lifted out of the warehouse management system."},
	{Org: "northwind", Name: "request_inventory", Capability: "request", SubjectSuffix: "inventory",
		Description: "Ask the WMS connector for on-hand inventory at a location."},

	// ---- ironbridge
	{Org: "ironbridge", Name: "publish_power", Capability: "publish", SubjectSuffix: "power",
		Description: "Three-phase panel measurement, every 5s.",
		SchemaNS:    "telemetry", SchemaName: "power", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "publish_vibration", Capability: "publish", SubjectSuffix: "vibration",
		Description: "Reduced bearing vibration summary.",
		SchemaNS:    "telemetry", SchemaName: "vibration", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "publish_cycle", Capability: "publish", SubjectSuffix: "cycle",
		Description: "One completed machine cycle.",
		SchemaNS:    "event", SchemaName: "cycle", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "publish_alarm", Capability: "publish", SubjectSuffix: "alarm",
		Description: "Alarm raised or cleared.",
		SchemaNS:    "event", SchemaName: "alarm", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon.",
		SchemaNS:    "status", SchemaName: "heartbeat", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "subscribe_line_mode", Capability: "subscribe", SubjectSuffix: "mode",
		Description: "Accept a line running-mode change.",
		SchemaNS:    "command", SchemaName: "line_mode", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "publish_line_mode_echo", Capability: "publish", SubjectSuffix: "mode.echo",
		Description: "Echo the mode actually in force.",
		SchemaNS:    "command", SchemaName: "line_mode", SchemaVersion: "1.0.0"},
	{Org: "ironbridge", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request."},
	{Org: "ironbridge", Name: "publish_oee", Capability: "publish", SubjectSuffix: "oee",
		Description: "Rolling OEE, computed by the analytics service from the cycle stream."},

	// ---- galewind
	{Org: "galewind", Name: "publish_generation", Capability: "publish", SubjectSuffix: "generation",
		Description: "Turbine output sample.",
		SchemaNS:    "telemetry", SchemaName: "generation", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "publish_feeder", Capability: "publish", SubjectSuffix: "feeder",
		Description: "Substation feeder measurement.",
		SchemaNS:    "telemetry", SchemaName: "feeder", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "publish_alarm", Capability: "publish", SubjectSuffix: "alarm",
		Description: "Protection or plant alarm.",
		SchemaNS:    "event", SchemaName: "alarm", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon.",
		SchemaNS:    "status", SchemaName: "heartbeat", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "subscribe_curtail", Capability: "subscribe", SubjectSuffix: "curtail",
		Description: "Accept a curtailment ceiling.",
		SchemaNS:    "command", SchemaName: "curtail", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "publish_curtail_echo", Capability: "publish", SubjectSuffix: "curtail.echo",
		Description: "Echo the curtailment ceiling actually in force.",
		SchemaNS:    "command", SchemaName: "curtail", SchemaVersion: "1.0.0"},
	{Org: "galewind", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request."},
	{Org: "galewind", Name: "request_forecast", Capability: "request", SubjectSuffix: "forecast",
		Description: "Ask the SCADA bridge for the current generation forecast."},
	{Org: "galewind", Name: "publish_dispatch", Capability: "publish", SubjectSuffix: "dispatch",
		Description: "Dispatch instruction lifted from the ISO market feed."},
}

// ---------------------------------------------------------------- thing types

// thingKind groups thing types for the bulk generator and, more importantly, for
// the reader: this platform's claim is that an APPLICATION is a first-class
// participant with its own signed identity, exactly like a sensor. A demo with
// nothing but sensors in it quietly contradicts that.
type thingKind string

const (
	kindDevice    thingKind = "device"      // physical, in the field
	kindGateway   thingKind = "gateway"     // physical, aggregates for a site
	kindApp       thingKind = "application" // software participant
	kindAppliance thingKind = "appliance"   // unattended screen or reader
)

type thingTypeFixture struct {
	Org, Code, Name, Description string
	SubjectPrefix                string
	Capabilities                 []string
	Operations                   []string // operation names, within the same org
	Role                         string   // nats_roles name to default to
	Kind                         thingKind
	Schema                       map[string]any

	// BulkPrefix and BulkLocations drive the generated fill. An empty BulkPrefix
	// means this type is curated-only — one-of-a-kind participants (the WMS
	// connector, the rule engine) must not be duplicated into a fleet.
	BulkPrefix    string
	BulkLocations []string
}

var thingTypes = []thingTypeFixture{
	// ------------------------------------------------------------- northwind
	{Org: "northwind", Code: "temp-probe", Name: "Temperature Probe", Kind: kindDevice,
		Description:   "Wireless probe reporting one temperature and its own battery.",
		SubjectPrefix: "telemetry.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_temperature", "publish_battery", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"serial":        str("Serial number"),
			"firmware":      str("Firmware version"),
			"probe_type":    enum("Probe type", "air", "product", "glycol"),
			"calibrated_on": date("Last calibrated"),
		}),
		BulkPrefix:    "TP",
		BulkLocations: []string{"KC-DC1-FZ1", "KC-DC1-FZ2", "KC-DC1-CH1", "SGF-XD2-CH1"}},

	{Org: "northwind", Code: "door-sensor", Name: "Dock Door Sensor", Kind: kindDevice,
		Description:   "Magnetic contact on a dock or zone door.",
		SubjectPrefix: "event.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_door", "publish_battery", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"serial":    str("Serial number"),
			"door_ref":  str("Door number"),
			"fail_safe": boolF("Fail-safe wiring"),
		}),
		BulkPrefix:    "DS",
		BulkLocations: []string{"KC-DC1", "SGF-XD2"}},

	{Org: "northwind", Code: "reefer-unit", Name: "Reefer Controller", Kind: kindDevice,
		Description:   "Trailer refrigeration controller. Takes a setpoint and echoes the one in force.",
		SubjectPrefix: "asset.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations: []string{"publish_temperature", "subscribe_setpoint",
			"publish_setpoint_echo", "reply_diagnostics", "publish_heartbeat"},
		Role: "device",
		Schema: objSchema(map[string]any{
			"serial":     str("Serial number"),
			"make":       str("Make"),
			"model":      str("Model"),
			"engine_hrs": intF("Engine hours"),
		})},

	{Org: "northwind", Code: "edge-gateway", Name: "Edge Gateway", Kind: kindGateway,
		Description:   "Site aggregator. Runs the leaf node and rule-router.",
		SubjectPrefix: "gateway.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations:    []string{"publish_heartbeat", "reply_diagnostics"},
		Role:          "gateway",
		Schema: objSchema(map[string]any{
			"serial":   str("Serial number"),
			"model":    str("Hardware model"),
			"os":       str("Operating system"),
			"wan_type": enum("WAN", "fibre", "cable", "cellular"),
		})},

	{Org: "northwind", Code: "wms-connector", Name: "WMS Connector", Kind: kindApp,
		Description:   "Software participant. Lifts shipment events out of the warehouse management system and answers inventory requests.",
		SubjectPrefix: "app.wms.{thing}",
		Capabilities:  []string{"publish", "reply"},
		Operations:    []string{"publish_shipment", "request_inventory"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":    str("Build"),
			"wms_vendor": str("WMS vendor"),
			"deployed":   date("Deployed"),
		})},

	{Org: "northwind", Code: "coldchain-rules", Name: "Cold Chain Rule Engine", Kind: kindApp,
		Description:   "A rule-router instance watching the temperature stream and raising excursions.",
		SubjectPrefix: "app.rules.{thing}",
		Capabilities:  []string{"publish", "subscribe"},
		Operations:    []string{"publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":    str("Build"),
			"rule_count": intF("Loaded rules"),
		})},

	{Org: "northwind", Code: "dock-display", Name: "Dock Display", Kind: kindAppliance,
		Description:   "Unattended screen above a dock door. Subscribes only.",
		SubjectPrefix: "display.{location}.{thing}",
		Capabilities:  []string{"subscribe"},
		Operations:    []string{"subscribe_render"},
		Role:          "console-readonly",
		Schema: objSchema(map[string]any{
			"panel_size":  str("Panel size"),
			"orientation": enum("Orientation", "landscape", "portrait"),
		})},

	// ------------------------------------------------------------ ironbridge
	{Org: "ironbridge", Code: "power-meter", Name: "Panel Power Meter", Kind: kindDevice,
		Description:   "Three-phase meter on a distribution panel.",
		SubjectPrefix: "telemetry.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_power", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"serial":   str("Serial number"),
			"ct_ratio": str("CT ratio"),
			"panel":    str("Panel designation"),
		}),
		BulkPrefix:    "PM",
		BulkLocations: []string{"PIT-B1", "PIT-B2", "PIT-MCC1"}},

	{Org: "ironbridge", Code: "vib-sensor", Name: "Vibration Sensor", Kind: kindDevice,
		Description:   "Bearing vibration monitor. Reduces on-device and publishes a summary.",
		SubjectPrefix: "telemetry.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_vibration", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"serial":      str("Serial number"),
			"mount":       enum("Mount", "stud", "magnet", "adhesive"),
			"machine_ref": str("Machine tag"),
		}),
		BulkPrefix:    "VS",
		BulkLocations: []string{"LINE-A", "LINE-B", "LINE-C"}},

	{Org: "ironbridge", Code: "line-controller", Name: "Line Controller", Kind: kindDevice,
		Description:   "PLC front-end for one production line. Counts cycles, raises alarms, takes a mode.",
		SubjectPrefix: "line.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations: []string{"publish_cycle", "publish_alarm", "subscribe_line_mode",
			"publish_line_mode_echo", "reply_diagnostics", "publish_heartbeat"},
		Role: "device",
		Schema: objSchema(map[string]any{
			"plc_make":  str("PLC make"),
			"plc_model": str("PLC model"),
			"rack_slot": str("Rack/slot"),
		})},

	{Org: "ironbridge", Code: "edge-gateway", Name: "Edge Gateway", Kind: kindGateway,
		Description:   "Plant aggregator. Runs the leaf node and rule-router.",
		SubjectPrefix: "gateway.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations:    []string{"publish_heartbeat", "reply_diagnostics"},
		Role:          "gateway",
		Schema: objSchema(map[string]any{
			"serial":   str("Serial number"),
			"model":    str("Hardware model"),
			"os":       str("Operating system"),
			"wan_type": enum("WAN", "fibre", "cable", "cellular"),
		})},

	{Org: "ironbridge", Code: "oee-analytics", Name: "OEE Analytics", Kind: kindApp,
		Description:   "Stream processor. Windows the cycle stream into rolling availability, performance and quality.",
		SubjectPrefix: "app.oee.{thing}",
		Capabilities:  []string{"publish", "subscribe"},
		Operations:    []string{"publish_oee", "publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":     str("Build"),
			"window_mins": intF("Window (minutes)"),
			"engine":      enum("Engine", "ekuiper", "benthos", "custom"),
		})},

	{Org: "ironbridge", Code: "mes-connector", Name: "MES Connector", Kind: kindApp,
		Description:   "Software participant bridging the manufacturing execution system.",
		SubjectPrefix: "app.mes.{thing}",
		Capabilities:  []string{"publish", "request"},
		Operations:    []string{"publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":    str("Build"),
			"mes_vendor": str("MES vendor"),
		})},

	// -------------------------------------------------------------- galewind
	{Org: "galewind", Code: "turbine-ctl", Name: "Turbine Controller", Kind: kindDevice,
		Description:   "Per-turbine controller. Reports generation and accepts a curtailment ceiling.",
		SubjectPrefix: "turbine.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations: []string{"publish_generation", "publish_alarm", "subscribe_curtail",
			"publish_curtail_echo", "reply_diagnostics", "publish_heartbeat"},
		Role: "device",
		Schema: objSchema(map[string]any{
			"make":         str("Turbine make"),
			"model":        str("Turbine model"),
			"scada_id":     str("SCADA identifier"),
			"rated_kw":     intF("Rated output (kW)"),
			"commissioned": date("Commissioned"),
		})},

	{Org: "galewind", Code: "feeder-relay", Name: "Feeder Protection Relay", Kind: kindDevice,
		Description:   "Substation feeder relay reporting measurements and protection events.",
		SubjectPrefix: "telemetry.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_feeder", "publish_alarm", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"make":     str("Relay make"),
			"model":    str("Relay model"),
			"feeder":   str("Feeder identifier"),
			"protocol": enum("Protocol", "dnp3", "iec61850", "modbus"),
		}),
		BulkPrefix:    "FR",
		BulkLocations: []string{"SUB-N1", "SUB-S2"}},

	{Org: "galewind", Code: "met-mast", Name: "Met Mast", Kind: kindDevice,
		Description:   "Meteorological mast. Wind speed and bearing for the whole collection area.",
		SubjectPrefix: "telemetry.{location}.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_generation", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"height_m":   num("Instrument height (m)"),
			"anemometer": str("Anemometer model"),
		})},

	{Org: "galewind", Code: "edge-gateway", Name: "Edge Gateway", Kind: kindGateway,
		Description:   "Substation aggregator on cellular backhaul. Runs the leaf node so the site survives a WAN outage.",
		SubjectPrefix: "gateway.{location}.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations:    []string{"publish_heartbeat", "reply_diagnostics"},
		Role:          "gateway",
		Schema: objSchema(map[string]any{
			"serial":   str("Serial number"),
			"model":    str("Hardware model"),
			"os":       str("Operating system"),
			"wan_type": enum("WAN", "fibre", "cable", "cellular"),
			"apn":      str("Cellular APN"),
		})},

	{Org: "galewind", Code: "scada-bridge", Name: "SCADA Bridge", Kind: kindApp,
		Description:   "Software participant translating between the historian and the bus.",
		SubjectPrefix: "app.scada.{thing}",
		Capabilities:  []string{"publish", "subscribe", "reply"},
		Operations:    []string{"request_forecast", "publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":   str("Build"),
			"historian": str("Historian product"),
		})},

	{Org: "galewind", Code: "market-feed", Name: "ISO Market Feed", Kind: kindApp,
		Description:   "Software participant publishing dispatch instructions from the ISO.",
		SubjectPrefix: "app.market.{thing}",
		Capabilities:  []string{"publish"},
		Operations:    []string{"publish_dispatch", "publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version": str("Build"),
			"iso":     enum("ISO", "ercot", "spp", "miso", "caiso"),
		})},

	{Org: "galewind", Code: "ops-wallboard", Name: "Operations Wallboard", Kind: kindAppliance,
		Description:   "Unattended screen in the operations centre. Subscribes only.",
		SubjectPrefix: "display.{location}.{thing}",
		Capabilities:  []string{"subscribe"},
		Operations:    []string{},
		Role:          "console-readonly",
		Schema: objSchema(map[string]any{
			"panel_size":  str("Panel size"),
			"orientation": enum("Orientation", "landscape", "portrait"),
		})},
}

// ----------------------------------------------------------------- NATS roles

type roleFixture struct {
	Org, Name, Description string
	IsDefault              bool
	Publish, Subscribe     []string
	PublishDeny            []string
	MaxSubscriptions       int
	MaxPayload             int
}

// One set per org, written out for each of the three by seedNatsRoles.
//
// `console-readonly` is the one worth reading carefully. A console role and a
// NATS role are two INDEPENDENT authorization systems here: `viewer` is
// read-only against the PocketBase API, but a member's real capability on the
// bus is whatever memberships.nats_user points at, and nothing checks that the
// two agree. Seeding a genuinely read-only NATS role and linking it to the
// viewer and dashboard memberships is what makes this demo teach the correct
// pairing instead of quietly demonstrating the trap — a "read-only" auditor
// holding publish ">" can drive the Publisher and Button widgets.
//
// Its publish list is not empty, because a subscriber is not a passive party in
// NATS: request/reply needs _INBOX, and reading a KV bucket in the console means
// creating an ephemeral consumer and fetching from the stream. Those are the
// minimum publishes a read-only console session actually issues.
var roleTemplates = []roleFixture{
	{Name: "device", IsDefault: true,
		Description:      "A field device: publishes its own telemetry and status, listens for commands addressed to it.",
		Publish:          []string{"telemetry.>", "event.>", "asset.>", "line.>", "turbine.>", "status.>"},
		Subscribe:        []string{"cmd.>", "config.>", "_INBOX.>"},
		MaxSubscriptions: 64, MaxPayload: 1048576},

	{Name: "gateway",
		Description:      "A site aggregator: everything a device may do, plus the site's own subtree and JetStream access for the local mirror.",
		Publish:          []string{"telemetry.>", "event.>", "status.>", "gateway.>", "$JS.API.>"},
		Subscribe:        []string{"cmd.>", "config.>", "gateway.>", "_INBOX.>"},
		MaxSubscriptions: 512, MaxPayload: 4194304},

	{Name: "application",
		Description: "A software participant: publishes on its own app subtree and on the operator service contract, reads broadly.",
		Publish:     []string{"app.>", "cmd.>", "helpdesk.>", "$JS.API.>"},
		Subscribe:   []string{">"},
		// $SYS belongs to the operator, never to a tenant's application. Denied
		// explicitly rather than left to the account boundary, so the intent is
		// legible on the role screen.
		PublishDeny:      []string{"$SYS.>"},
		MaxSubscriptions: 1024, MaxPayload: 4194304},

	{Name: "console-operator",
		Description:      "A console session for an owner or admin: unrestricted within this account.",
		Publish:          []string{">"},
		Subscribe:        []string{">"},
		MaxSubscriptions: 256, MaxPayload: 4194304},

	{Name: "console-readonly",
		Description: "A console session that cannot change anything on the bus. Pair this with the viewer and dashboard roles — a console role is not a NATS role, and the two are set independently.",
		// Reads only, plus the publishes a read actually requires: an inbox for
		// request/reply, and the JetStream API calls the KV browser issues to
		// list streams, create an ephemeral consumer and fetch values.
		Publish: []string{
			"_INBOX.>",
			"$JS.API.INFO",
			"$JS.API.STREAM.NAMES",
			"$JS.API.STREAM.LIST",
			"$JS.API.STREAM.INFO.>",
			"$JS.API.CONSUMER.CREATE.>",
			"$JS.API.CONSUMER.MSG.NEXT.>",
			"$JS.API.DIRECT.GET.>",
		},
		Subscribe:        []string{">"},
		PublishDeny:      []string{"$SYS.>"},
		MaxSubscriptions: 256, MaxPayload: 1048576},
}
