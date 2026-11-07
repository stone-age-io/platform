package demoseed

// The Thing Type contract fixtures and the NATS permission templates.
//
// Subject prefixes use the reserved template variables from
// ui/src/utils/subjectResolver.ts ({org}, {location}, {thing},
// {thing_type_code}), not literal strings, because that is what the console
// renders and what a role pattern is derived from ({thing} collapses to "*").
//
// No prefix carries an organization token: the NATS account is the tenant
// boundary, and the managed-org export is the only place that adds one, on the
// way out of the account.

type operationFixture struct {
	Org, Name, Capability, SubjectSuffix, Description string
}

var operations = []operationFixture{
	// Northwind.
	{Org: "northwind", Name: "publish_temperature", Capability: "publish", SubjectSuffix: "temperature",
		Description: "Periodic temperature sample."},
	{Org: "northwind", Name: "publish_battery", Capability: "publish", SubjectSuffix: "battery",
		Description: "Battery state, sent hourly and on change."},
	{Org: "northwind", Name: "publish_door", Capability: "publish", SubjectSuffix: "door",
		Description: "Door open/closed transition."},
	{Org: "northwind", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon."},
	{Org: "northwind", Name: "subscribe_setpoint", Capability: "subscribe", SubjectSuffix: "setpoint",
		Description: "Accept a new temperature setpoint."},
	{Org: "northwind", Name: "publish_setpoint_echo", Capability: "publish", SubjectSuffix: "setpoint.echo",
		Description: "Echo the setpoint actually in force. This is the property a desired value should be paired with."},
	{Org: "northwind", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request. Payload is device-specific and deliberately unschemad."},
	{Org: "northwind", Name: "subscribe_render", Capability: "subscribe", SubjectSuffix: "render",
		Description: "Screen contents for an unattended display."},
	{Org: "northwind", Name: "publish_shipment", Capability: "publish", SubjectSuffix: "shipment",
		Description: "A shipment event lifted out of the warehouse management system."},
	{Org: "northwind", Name: "request_inventory", Capability: "request", SubjectSuffix: "inventory",
		Description: "Ask the WMS connector for on-hand inventory at a location."},

	// Northwind: stone-access.
	//
	// The suffixes are the real ones from access-control/internal/subjects; keep
	// them in step. `.tap` is the reader's input (a credential presentation);
	// `.evt.tap` is the decision the controller emits, and only it is audited.
	{Org: "northwind", Name: "publish_access_decision", Capability: "publish", SubjectSuffix: "evt.tap",
		Description: "The decision on a credential presentation."},
	{Org: "northwind", Name: "publish_access_alarm", Capability: "publish", SubjectSuffix: "evt.alarm",
		Description: "Door forced or held open past its threshold."},
	{Org: "northwind", Name: "publish_access_state", Capability: "publish", SubjectSuffix: "evt.state",
		Description: "Effective-posture change on a portal."},
	{Org: "northwind", Name: "subscribe_access_tap", Capability: "subscribe", SubjectSuffix: "tap",
		Description: "A credential presentation arriving from a reader. The controller's input, not its output."},
	{Org: "northwind", Name: "subscribe_access_grant", Capability: "subscribe", SubjectSuffix: "cmd.grant",
		Description: "Operator-initiated momentary unlock."},
	{Org: "northwind", Name: "subscribe_access_posture", Capability: "subscribe", SubjectSuffix: "cmd.posture",
		Description: "Set or clear a runtime posture override on a portal."},
	{Org: "northwind", Name: "publish_controller_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Controller liveness beat, outside the audited .evt subtree."},

	// Northwind: the tool crib (the kiosk app).
	//
	// The suffixes are the real ones from kiosk/internal/events/subjects.go; keep
	// them in step. The grammar is <prefix>.<kiosk_code>.<family>.<...>, and the
	// family decides transport: the controller's stream binds only
	// `kiosk.*.event.>`, so an `event.` subject is durable and nothing else is.
	//
	// Three item actions, not `event.item.>`: a publish operation must name a
	// subject something publishes on. An admin close publishes only
	// `event.checkout.admin_close`, so reports can tell a return from a write-off.
	{Org: "northwind", Name: "publish_kiosk_transaction", Capability: "publish", SubjectSuffix: "event.transaction.complete",
		Description: "A committed transaction. The ledger's unit of record, and the only event that registers a kiosk in the fleet."},
	{Org: "northwind", Name: "publish_kiosk_checkout", Capability: "publish", SubjectSuffix: "event.item.checkout",
		Description: "One line: a tool taken out."},
	{Org: "northwind", Name: "publish_kiosk_return", Capability: "publish", SubjectSuffix: "event.item.return",
		Description: "One line: a tool brought back."},
	{Org: "northwind", Name: "publish_kiosk_consume", Capability: "publish", SubjectSuffix: "event.item.consume",
		Description: "One line: a consumable drawn down. Never becomes an open checkout."},
	{Org: "northwind", Name: "publish_kiosk_admin_close", Capability: "publish", SubjectSuffix: "event.checkout.admin_close",
		Description: "An admin wrote off an open checkout as lost or damaged. One per row closed."},
	{Org: "northwind", Name: "publish_kiosk_inventory", Capability: "publish", SubjectSuffix: "event.inventory.adjust",
		Description: "A stock count changed by hand rather than by a transaction. Carries the adjustment id the controller dedupes on."},
	{Org: "northwind", Name: "publish_kiosk_instance", Capability: "publish", SubjectSuffix: "event.instance.lifecycle",
		Description: "A serialized unit was created or changed status: in_service, maintenance, retired."},
	{Org: "northwind", Name: "publish_kiosk_punch", Capability: "publish", SubjectSuffix: "event.timeclock.punch",
		Description: "An accepted clock-in or clock-out. Kiosks are the only writers of the punch ledger."},
	{Org: "northwind", Name: "publish_kiosk_receipt", Capability: "publish", SubjectSuffix: "event.receipt.transaction",
		Description: "Rendering context for a transaction receipt. The kiosk supplies the facts; the controller owns the template and the SMTP."},
	{Org: "northwind", Name: "publish_kiosk_lowstock", Capability: "publish", SubjectSuffix: "event.alert.lowstock",
		Description: "A SKU crossed its reorder threshold. The kiosk owns the detection because the kiosk owns the count."},
	{Org: "northwind", Name: "publish_kiosk_maintenance", Capability: "publish", SubjectSuffix: "event.alert.maintenance",
		Description: "A return routed one or more units into maintenance. Batched one per transaction."},
	{Org: "northwind", Name: "publish_kiosk_integrity", Capability: "publish", SubjectSuffix: "event.integrity.rebuild",
		Description: "An operator rebuilt the open-checkouts view from the ledger. Rare, destructive, and worth auditing."},
	{Org: "northwind", Name: "publish_kiosk_rfid_read", Capability: "publish", SubjectSuffix: "event.scan.rfid.observed",
		Description: "Every EPC seen in one RFID read window. Observability only — nothing projects it today."},
	{Org: "northwind", Name: "publish_kiosk_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "45s liveness beat, outside the stream. Last-write-wins on purpose: durability here would hide the very signal it carries."},
	// A wildcard suffix is right here: it is a subscription pattern. The kiosk
	// subscribes its whole command subtree and routes on the leaf name; listing
	// the leaves here would be a second registry that drifts from the kiosk's.
	{Org: "northwind", Name: "reply_kiosk_command", Capability: "reply", SubjectSuffix: "command.>",
		Description: "Answer any command the controller sends: inventory, instances, checkouts, timeclock, RFID. Single attempt, 5s budget — a handler that cannot do the work must still reply, or the controller renders the kiosk as offline."},
	{Org: "northwind", Name: "subscribe_kiosk_sighting", Capability: "subscribe", SubjectSuffix: "sighting.raw",
		Description: "Advisory tag sightings from an off-platform gateway. Raw: it carries a tag id, never a resolved unit, and the subscriber resolves it."},

	// Ironbridge.
	{Org: "ironbridge", Name: "publish_power", Capability: "publish", SubjectSuffix: "power",
		Description: "Three-phase panel measurement, every 5s."},
	{Org: "ironbridge", Name: "publish_vibration", Capability: "publish", SubjectSuffix: "vibration",
		Description: "Reduced bearing vibration summary."},
	{Org: "ironbridge", Name: "publish_cycle", Capability: "publish", SubjectSuffix: "cycle",
		Description: "One completed machine cycle."},
	{Org: "ironbridge", Name: "publish_alarm", Capability: "publish", SubjectSuffix: "alarm",
		Description: "Alarm raised or cleared."},
	{Org: "ironbridge", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon."},
	{Org: "ironbridge", Name: "subscribe_line_mode", Capability: "subscribe", SubjectSuffix: "mode",
		Description: "Accept a line running-mode change."},
	{Org: "ironbridge", Name: "publish_line_mode_echo", Capability: "publish", SubjectSuffix: "mode.echo",
		Description: "Echo the mode actually in force."},
	{Org: "ironbridge", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request."},
	{Org: "ironbridge", Name: "publish_oee", Capability: "publish", SubjectSuffix: "oee",
		Description: "Rolling OEE, computed by the analytics service from the cycle stream."},

	// Galewind.
	{Org: "galewind", Name: "publish_generation", Capability: "publish", SubjectSuffix: "generation",
		Description: "Turbine output sample."},
	{Org: "galewind", Name: "publish_feeder", Capability: "publish", SubjectSuffix: "feeder",
		Description: "Substation feeder measurement."},
	{Org: "galewind", Name: "publish_alarm", Capability: "publish", SubjectSuffix: "alarm",
		Description: "Protection or plant alarm."},
	{Org: "galewind", Name: "publish_heartbeat", Capability: "publish", SubjectSuffix: "heartbeat",
		Description: "Liveness beacon."},
	{Org: "galewind", Name: "subscribe_curtail", Capability: "subscribe", SubjectSuffix: "curtail",
		Description: "Accept a curtailment ceiling."},
	{Org: "galewind", Name: "publish_curtail_echo", Capability: "publish", SubjectSuffix: "curtail.echo",
		Description: "Echo the curtailment ceiling actually in force."},
	{Org: "galewind", Name: "reply_diagnostics", Capability: "reply", SubjectSuffix: "diag",
		Description: "Answer an on-demand diagnostics request."},
	{Org: "galewind", Name: "request_forecast", Capability: "request", SubjectSuffix: "forecast",
		Description: "Ask the SCADA bridge for the current generation forecast."},
	{Org: "galewind", Name: "publish_dispatch", Capability: "publish", SubjectSuffix: "dispatch",
		Description: "Dispatch instruction lifted from the ISO market feed."},
}

// thingKind groups thing types. The demo includes applications and appliances
// because an application is a first-class participant with its own signed
// identity, like a sensor.
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
	Operations                   []string // operation names, within the same org
	Role                         string   // nats_roles name to default to
	Kind                         thingKind
	Schema                       map[string]any

	// BulkPrefix and BulkLocations drive the generated fill. An empty BulkPrefix
	// means curated-only, for one-of-a-kind participants.
	BulkPrefix    string
	BulkLocations []string
}

var thingTypes = []thingTypeFixture{
	// Northwind.
	{Org: "northwind", Code: "temp-probe", Name: "Temperature Probe", Kind: kindDevice,
		Description:   "Wireless probe reporting one temperature and its own battery.",
		SubjectPrefix: "telemetry.{location}.{thing}",
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
		Operations:    []string{"publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":    str("Build"),
			"rule_count": intF("Loaded rules"),
		})},

	// Northwind: stone-access hardware.
	//
	// The prefixes are stone-access's real subject hierarchy:
	// acc.{location}.{type}.{thing} for a portal, acc.{location}.ctrl.{code} for
	// a controller (`ctrl` is reserved, not a portal type). The portal type is a
	// subject token, so door and gate need one thing type each.
	{Org: "northwind", Code: "access-controller", Name: "Access Controller", Kind: kindGateway,
		Description:   "A stone-access edge controller. Decides credential presentations locally against a mirrored policy graph, and keeps deciding when the WAN is down.",
		SubjectPrefix: "acc.{location}.ctrl.{thing}",
		Operations:    []string{"publish_controller_heartbeat", "publish_access_state"},
		Role:          "gateway",
		Schema: objSchema(map[string]any{
			"model":      enum("Board", "kincony-server-mini", "kincony-pi5r8"),
			"serial":     str("Serial number"),
			"reader_bus": str("RS485 device"),
			"relays":     intF("Relay outputs"),
			"inputs":     intF("Digital inputs"),
		}),
	},

	{Org: "northwind", Code: "access-door", Name: "Access-Controlled Door", Kind: kindDevice,
		Description:   "A door with a reader, a strike or maglock, and door-position monitoring. Its code is the portal code in stone-access.",
		SubjectPrefix: "acc.{location}.door.{thing}",
		Operations: []string{"publish_access_decision", "publish_access_alarm", "publish_access_state",
			"subscribe_access_tap", "subscribe_access_grant", "subscribe_access_posture"},
		Role: "device",
		Schema: objSchema(map[string]any{
			"lock_type":         enum("Lock", "strike", "maglock"),
			"reader_make":       str("Reader make"),
			"reader_protocol":   enum("Reader protocol", "osdp", "wiegand"),
			"held_open_seconds": intF("Held-open threshold (s)"),
			"installed":         date("Installed"),
		}),
	},

	{Org: "northwind", Code: "access-gate", Name: "Access-Controlled Gate", Kind: kindDevice,
		Description:   "A vehicle gate on the same controller as the doors, with a much longer held-open threshold.",
		SubjectPrefix: "acc.{location}.gate.{thing}",
		Operations: []string{"publish_access_decision", "publish_access_alarm",
			"subscribe_access_tap", "subscribe_access_grant"},
		Role: "device",
		Schema: objSchema(map[string]any{
			"operator_make":     str("Gate operator make"),
			"reader_protocol":   enum("Reader protocol", "osdp", "wiegand"),
			"held_open_seconds": intF("Held-open threshold (s)"),
			"loop_detector":     boolF("Vehicle loop fitted"),
		}),
	},

	{Org: "northwind", Code: "dock-display", Name: "Dock Display", Kind: kindAppliance,
		Description:   "Unattended screen above a dock door. Subscribes only.",
		SubjectPrefix: "display.{location}.{thing}",
		Operations:    []string{"subscribe_render"},
		Role:          "console-readonly",
		Schema: objSchema(map[string]any{
			"panel_size":  str("Panel size"),
			"orientation": enum("Orientation", "landscape", "portrait"),
		})},

	// Northwind: the tool crib.
	//
	// The prefix is the kiosk app's real subject hierarchy. It has no {location}
	// segment: the kiosk's code is the routing token (the stream binds
	// `kiosk.*.event.>`, commands address `kiosk.<code>.command.<name>`).
	//
	// The app that puts a code on the wire mints it, and the inventory follows:
	// kiosk codes are uppercase, stone-access codes lowercase. Site codes go the
	// other way; the platform is the system of record for sites.
	{Org: "northwind", Code: "tool-kiosk", Name: "Tool Crib Kiosk", Kind: kindGateway,
		Description:   "A self-service checkout node. Owns its own ledger and keeps transacting with the WAN down; the controller aggregates rather than authorizes.",
		SubjectPrefix: "kiosk.{thing}",
		Operations: []string{
			"publish_kiosk_transaction", "publish_kiosk_checkout", "publish_kiosk_return",
			"publish_kiosk_consume", "publish_kiosk_admin_close", "publish_kiosk_inventory",
			"publish_kiosk_instance", "publish_kiosk_punch", "publish_kiosk_receipt",
			"publish_kiosk_lowstock", "publish_kiosk_maintenance", "publish_kiosk_integrity",
			"publish_kiosk_rfid_read", "publish_kiosk_heartbeat", "reply_kiosk_command",
			"subscribe_kiosk_sighting",
		},
		// A kiosk mirrors its catalogue out of JetStream KV, and `device` has no
		// JetStream access.
		Role: "gateway",
		Schema: objSchema(map[string]any{
			"model":      str("Hardware model"),
			"os":         str("Operating system"),
			"port":       intF("HTTP port"),
			"scanner":    enum("Barcode scanner", "usb-hid", "none"),
			"rfid_mode":  enum("RFID", "counter_scan", "enclosure_diff", "mixed", "none"),
			"enclosures": intF("Cabinets driven"),
			"installed":  date("Installed"),
		})},

	{Org: "northwind", Code: "timeclock-terminal", Name: "Virtual Timeclock Terminal", Kind: kindAppliance,
		Description:   "A punch-only station with no checkout surface. Workers authenticate from their own phones, so the punched identity comes from the session and never from the request body.",
		SubjectPrefix: "kiosk.{thing}",
		Operations:    []string{"publish_kiosk_punch", "publish_kiosk_heartbeat"},
		// An appliance in form, an edge service on the bus: it mirrors KV and
		// publishes punches, and `console-readonly` can do neither.
		Role: "gateway",
		Schema: objSchema(map[string]any{
			"port":      intF("HTTP port"),
			"auth":      enum("Worker sign-in", "oauth2", "password", "both"),
			"installed": date("Installed"),
		})},

	// The controller declares no operations on purpose: every subject it touches
	// belongs to a kiosk, and an operation composes under its own type's prefix
	// (`app.kiosk.<code>.…`, which nothing uses). Cross-subtree participation
	// lives in the role, as for the access-controller;
	// TestApplicationRoleCanRunTheKioskController is the only check on it.
	{Org: "northwind", Code: "kiosk-controller", Name: "Kiosk Controller", Kind: kindApp,
		Description:   "Central aggregator for the kiosk fleet. Projects every node's ledger, publishes the catalogue into KV, and drives admin commands at remote nodes. Single instance.",
		SubjectPrefix: "app.kiosk.{thing}",
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":  str("Build"),
			"kiosks":   intF("Nodes managed"),
			"deployed": date("Deployed"),
		})},

	// Ironbridge.
	{Org: "ironbridge", Code: "power-meter", Name: "Panel Power Meter", Kind: kindDevice,
		Description:   "Three-phase meter on a distribution panel.",
		SubjectPrefix: "telemetry.{location}.{thing}",
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
		Operations:    []string{"publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":    str("Build"),
			"mes_vendor": str("MES vendor"),
		})},

	// Galewind.
	{Org: "galewind", Code: "turbine-ctl", Name: "Turbine Controller", Kind: kindDevice,
		Description:   "Per-turbine controller. Reports generation and accepts a curtailment ceiling.",
		SubjectPrefix: "turbine.{location}.{thing}",
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
		Operations:    []string{"publish_generation", "publish_heartbeat"},
		Role:          "device",
		Schema: objSchema(map[string]any{
			"height_m":   num("Instrument height (m)"),
			"anemometer": str("Anemometer model"),
		})},

	{Org: "galewind", Code: "edge-gateway", Name: "Edge Gateway", Kind: kindGateway,
		Description:   "Substation aggregator on cellular backhaul. Runs the leaf node so the site survives a WAN outage.",
		SubjectPrefix: "gateway.{location}.{thing}",
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
		Operations:    []string{"request_forecast", "publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":   str("Build"),
			"historian": str("Historian product"),
		})},

	{Org: "galewind", Code: "market-feed", Name: "ISO Market Feed", Kind: kindApp,
		Description:   "Software participant publishing dispatch instructions from the ISO.",
		SubjectPrefix: "app.market.{thing}",
		Operations:    []string{"publish_dispatch", "publish_heartbeat"},
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version": str("Build"),
			"iso":     enum("ISO", "ercot", "spp", "miso", "caiso"),
		})},

	{Org: "galewind", Code: "ops-wallboard", Name: "Operations Wallboard", Kind: kindAppliance,
		Description:   "Unattended screen in the operations centre. Subscribes only.",
		SubjectPrefix: "display.{location}.{thing}",
		Operations:    []string{},
		Role:          "console-readonly",
		Schema: objSchema(map[string]any{
			"panel_size":  str("Panel size"),
			"orientation": enum("Orientation", "landscape", "portrait"),
		})},
}

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
// A console role and a NATS role are independent: a member's capability on the
// bus is whatever memberships.nats_user points at, and nothing checks that it
// agrees with `viewer`. So `console-readonly` is a genuinely read-only NATS
// role, linked to the viewer and dashboard memberships. Its publish list is
// not empty: request/reply needs _INBOX, and reading KV means creating a
// consumer and fetching.
var roleTemplates = []roleFixture{
	// `acc.>` is on both lists here and on `gateway` for stone-access, whose
	// doors default to `device` and controllers to `gateway`. It is on
	// subscribe because a door listens on `acc.{location}.{type}.{thing}.cmd.grant`,
	// not `cmd.>`.
	//
	// Invariant: the publish and subscribe lists cover the same subtrees. A thing
	// type composes every operation, inbound too, under its own prefix (a reefer
	// takes its setpoint on `asset.{location}.{thing}.setpoint`), so a
	// subscribe list rooted at `cmd.` misses them.
	//
	// `_INBOX.>` is on publish because a `reply` operation answers to the
	// requester's inbox.
	//
	// `$SRV.>` is on subscribe for the edge agent, a NATS micro service named
	// stone-agent, so $SRV.PING.stone-agent discovers it. Both roles an agent
	// runs under carry it. Behind a leaf the gateway credential is also the
	// uplink, which filters it too:
	// TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt.
	//
	// TestEveryThingTypeCanSpeakItsOwnContract checks both directions for every
	// type against its role. Nothing at runtime does.
	//
	// The lists are subtree-coarse (`telemetry.>`): a role grants a kind of
	// participation, so a device can subscribe to a peer's telemetry.
	//
	// A per-user edit cannot narrow these roles. pb-nats unions the role's
	// lists with the user's own (`applyPermissions` in
	// internal/jwt/generator.go), for allow and deny alike, and NATS has no
	// specificity ranking or negation. Narrowing needs a role carrying only the
	// shared parts plus a per-user allow list, which needs a subject grammar
	// that the free-text `subject_prefix` does not give. The per-user fields are
	// owner/admin-only because writing them grants NATS permissions.
	{Name: "device", IsDefault: true,
		Description:      "A field device: publishes its own telemetry and status, listens for commands addressed to it.",
		Publish:          []string{"telemetry.>", "event.>", "asset.>", "line.>", "turbine.>", "status.>", "acc.>", "_INBOX.>"},
		Subscribe:        []string{"telemetry.>", "event.>", "asset.>", "line.>", "turbine.>", "status.>", "acc.>", "cmd.>", "config.>", "_INBOX.>", "$SRV.>"},
		MaxSubscriptions: 64, MaxPayload: 1048576},

	{Name: "gateway",
		Description: "A site aggregator: everything a device may do, plus the site's own subtree and JetStream access for the local mirror.",
		// `$KV.>` is not covered by `$JS.API.>`: reading KV goes through the
		// JetStream API, but a write is a plain publish to `$KV.{bucket}.{key}`.
		// Without it an edge service boots clean and fails on its first write.
		//
		// Not on `device`: a gateway runs the edge services that own buckets, and
		// `device` has no JetStream access, so `$KV.>` alone would be half a
		// permission.
		Publish:          []string{"telemetry.>", "event.>", "status.>", "gateway.>", "acc.>", "kiosk.>", "_INBOX.>", "$JS.API.>", "$KV.>"},
		Subscribe:        []string{"telemetry.>", "event.>", "status.>", "gateway.>", "acc.>", "kiosk.>", "cmd.>", "config.>", "_INBOX.>", "$SRV.>"},
		MaxSubscriptions: 512, MaxPayload: 4194304},

	{Name: "application",
		Description: "A software participant: publishes on its own app subtree and on the operator service contract, reads broadly.",
		// `kiosk.>` and `$KV.>` are for the kiosk controller, which works outside
		// its own subtree: it requests on `kiosk.<code>.command.<name>` and writes
		// the catalogue and fleet state into KV (`$KV.catalog_items.<kiosk>.<sku>`
		// is a plain publish, as on `gateway`).
		Publish:   []string{"app.>", "cmd.>", "helpdesk.>", "kiosk.>", "$JS.API.>", "$KV.>"},
		Subscribe: []string{">"},
		// No $SYS deny, deliberately; see console-readonly below. The operator's
		// endpoints live in the $SYS account, so `$SYS.REQ.SERVER.>` from this
		// account reaches no responder whatever this role permits.
		MaxSubscriptions: 1024, MaxPayload: 4194304},

	{Name: "console-operator",
		Description:      "A console session for an owner or admin: unrestricted within this account.",
		Publish:          []string{">"},
		Subscribe:        []string{">"},
		MaxSubscriptions: 256, MaxPayload: 4194304},

	{Name: "console-readonly",
		Description: "A console session that cannot change anything on the bus. Pair this with the viewer and dashboard roles — a console role is not a NATS role, and the two are set independently.",
		// Reads only, plus the publishes a read actually requires: an inbox for
		// request/reply, the JetStream API calls the KV browser issues to list
		// streams, create an ephemeral consumer and fetch values, and the
		// account-scoped monitoring endpoints.
		Publish: []string{
			"_INBOX.>",
			"$JS.API.INFO",
			"$JS.API.STREAM.NAMES",
			"$JS.API.STREAM.LIST",
			"$JS.API.STREAM.INFO.>",
			"$JS.API.CONSUMER.CREATE.>",
			"$JS.API.CONSUMER.MSG.NEXT.>",
			"$JS.API.DIRECT.GET.>",
			// Account monitoring: a Publisher or Button widget aimed at
			// `$SYS.REQ.ACCOUNT.PING.CONNZ` with `{}` lists this org's attached
			// leaf nodes, each named by the Thing code the agent sets as
			// `server_name`. A dashboard recipe, not a built-in view: nothing in
			// the schema marks which Things are gateways.
			"$SYS.REQ.ACCOUNT.PING.>",
		},
		Subscribe: []string{">"},
		// No $SYS publish deny, deliberately. Do not add one back.
		//
		// Server-level monitoring (`SERVER.PING.LEAFZ` answers for every tenant)
		// is the operator's alone, but the account enforces that, not a subject
		// rule: those endpoints are served inside the $SYS account, so a client
		// here reaches no responder with or without a deny.
		// TestTenantCannotReachServerEndpoints proves it with no deny list.
		//
		// A deny is also harmful: in NATS a publish deny beats a publish allow,
		// so a `$SYS.>` deny kills the account-scoped endpoints above, and the
		// symptom is a bare request timeout (the permissions error arrives only
		// on the connection's async error handler).
		// TestDenyingAllOfSysBlocksAccountMonitoring pins all four shapes
		// against a real server.
		MaxSubscriptions: 256, MaxPayload: 1048576},
}
