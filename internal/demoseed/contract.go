package demoseed

// The Thing Type contract fixtures and the NATS permission templates.
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

// ----------------------------------------------------------------- operations

type operationFixture struct {
	Org, Name, Capability, SubjectSuffix, Description string
}

var operations = []operationFixture{
	// ---- northwind
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

	// ---- northwind: stone-access.
	//
	// The suffixes are the real ones from access-control/internal/subjects, so a
	// subject composed on the Thing Type screen is the subject a controller
	// actually publishes on. `evt.tap` rather than `tap` is not a typo: the bare
	// `.tap` subject is the READER's input (a credential presentation arriving at
	// the controller), while `.evt.tap` is the DECISION the controller emits. Two
	// different messages, and only the second is audited.
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

	// ---- northwind: the tool crib (the kiosk app).
	//
	// The suffixes are the real ones from kiosk/internal/events/subjects.go. The
	// grammar there is <prefix>.<kiosk_code>.<family>.<...>, and the FAMILY
	// segment is what decides transport: the controller's JetStream stream binds
	// `kiosk.*.event.>` and nothing else, so commands, heartbeats and sightings
	// are outside the stream by construction rather than by an exclusion list.
	// Every suffix below therefore leads with its family, and naming a new
	// `event.` subject is what makes it durable.
	//
	// Three item actions rather than one `event.item.>`: the last token is the
	// line's action, a publish operation resolves to a subject something actually
	// publishes ON, and a wildcard there is not one. `admin_close` is absent from
	// that trio on purpose — an admin close writes an admin_close LINE locally but
	// publishes only `event.checkout.admin_close`, which is what lets a report
	// separate "the worker brought it back" from "an admin wrote it off".
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
	// A wildcard suffix, and the one place it is right. This is a SUBSCRIPTION
	// pattern, not a publish target: the kiosk subscribes its whole command
	// subtree and its dispatcher routes on the leaf name. There are around twenty
	// of those leaves and they are registered in code, so enumerating them here
	// would stand up a second registry whose only guaranteed property is drifting
	// from the first.
	{Org: "northwind", Name: "reply_kiosk_command", Capability: "reply", SubjectSuffix: "command.>",
		Description: "Answer any command the controller sends: inventory, instances, checkouts, timeclock, RFID. Single attempt, 5s budget — a handler that cannot do the work must still reply, or the controller renders the kiosk as offline."},
	{Org: "northwind", Name: "subscribe_kiosk_sighting", Capability: "subscribe", SubjectSuffix: "sighting.raw",
		Description: "Advisory tag sightings from an off-platform gateway. Raw: it carries a tag id, never a resolved unit, and the subscriber resolves it."},

	// ---- ironbridge
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

	// ---- galewind
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

	// ---- northwind: stone-access hardware.
	//
	// These three carry the codes of records in the ACCESS-CONTROL app, and the
	// prefixes are its real subject hierarchy: acc.{location}.{type}.{thing} for a
	// portal, acc.{location}.ctrl.{code} for a controller. `ctrl` is a reserved
	// segment there — it is not a portal type — which is why the controller gets
	// its own thing type rather than a `{thing_type_code}` substitution.
	//
	// A door and a gate are separate types for the same reason: the second token
	// is the portal's TYPE, so one thing type per type is the only way a composed
	// subject on the Thing Type screen matches what a controller publishes.
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

	// ---- northwind: the tool crib.
	//
	// These carry the codes of `kiosks` rows in the KIOSK app, and the prefix is
	// its real subject hierarchy. Two things about it are worth reading twice.
	//
	// There is NO {location} segment. `acc.{location}.door.{thing}` names a portal
	// by where it hangs; `kiosk.{thing}` names a node by what it is, because a
	// kiosk's own code IS the routing token — the controller's stream binds
	// `kiosk.*.event.>` and its commands address `kiosk.<code>.command.<name>`.
	// Adding a site segment here would render a plausible subject on the Thing
	// Type screen that no kiosk publishes on and no controller listens to.
	//
	// And the codes are UPPERCASE where the stone-access ones are lowercase. Same
	// rule produced both: the app that puts a code on the wire is the one that
	// mints it, and the inventory follows. Site codes go the other way — KC-DC1,
	// KC-OFFICE and SGF-XD2 come from here, because the platform is the system of
	// record for sites.
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
		// kindGateway, and the role matches: a kiosk mirrors its catalogue out of
		// JetStream KV and answers commands on its own subtree. `device` has no
		// JetStream access at all, so a kiosk on it would come up, serve checkouts
		// off its local database, and never learn what it stocks.
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
		// An appliance in form, an edge service on the bus: it mirrors the worker
		// catalogue and the fleet punch replica out of KV, which is JetStream
		// access, which `console-readonly` (what the dock display takes) does not
		// have and could not use — that role cannot publish at all, and this one
		// publishes every punch it accepts.
		Role: "gateway",
		Schema: objSchema(map[string]any{
			"port":      intF("HTTP port"),
			"auth":      enum("Worker sign-in", "oauth2", "password", "both"),
			"installed": date("Installed"),
		})},

	// The controller declares NO operations, and that is the finding rather than
	// an omission.
	//
	// Every subject it touches belongs to some OTHER thing: it requests on each
	// kiosk's `command.` subtree, subscribes their `heartbeat` and `sighting.raw`,
	// and consumes their `event.` subtree through the stream. An operation's
	// suffix composes against its own type's prefix, so any entry here would
	// render `app.kiosk.<code>.…` on the Thing Type screen — a subject nothing
	// publishes on and nothing listens to.
	//
	// This is the same shape as the access-controller, whose subscriptions to door
	// subjects are likewise absent from its operation list. Cross-subtree
	// participation is expressed in the ROLE, not in operations, which is why
	// TestApplicationRoleCanRunTheKioskController exists: it is the only check
	// standing between this type and a credential that cannot do its job.
	{Org: "northwind", Code: "kiosk-controller", Name: "Kiosk Controller", Kind: kindApp,
		Description:   "Central aggregator for the kiosk fleet. Projects every node's ledger, publishes the catalogue into KV, and drives admin commands at remote nodes. Single instance.",
		SubjectPrefix: "app.kiosk.{thing}",
		Role:          "application",
		Schema: objSchema(map[string]any{
			"version":  str("Build"),
			"kiosks":   intF("Nodes managed"),
			"deployed": date("Deployed"),
		})},

	// ------------------------------------------------------------ ironbridge
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

	// -------------------------------------------------------------- galewind
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
	// `acc.>` is on both lists here and on `gateway` below because a sibling app
	// running inside this account owns that subtree, and its participants are
	// Things of these two kinds: an access-controlled door defaults to `device`
	// and a controller to `gateway`. Without it, a door's minted credential
	// cannot publish the decision its own thing type declares, and a controller
	// cannot receive the taps it exists to answer — and the failure would look
	// like an access-control bug rather than a permission the inventory forgot
	// to grant. It is on SUBSCRIBE as well as publish because a door listens on
	// `acc.{location}.{type}.{thing}.cmd.grant`, not on the generic `cmd.>`.
	//
	// The two lists below cover the SAME SUBTREES, and that is the invariant to
	// keep. It was not true before: subscribe carried only `cmd.>` and `config.>`,
	// on the assumption that anything inbound arrives on a command root. Nothing
	// in the fixture works that way — a reefer takes its setpoint on
	// `asset.{location}.{thing}.setpoint`, a line controller its mode on
	// `line.…mode`, a turbine its curtailment on `turbine.…curtail` — because a
	// thing type composes every operation under its OWN prefix, inbound and
	// outbound alike. A pattern rooted at `cmd.` matches none of them, so three
	// types shipped unable to receive the instructions they declare, and the
	// `.echo` half of each pair had nothing to echo.
	//
	// Mirroring is deliberately coarse: a device can subscribe to a peer's
	// telemetry, not just its own. That is the cost of a role template being a
	// KIND of participation rather than one identity's contract, and the reason
	// narrowing lives on the thing's own `publish_permissions` — owner/admin only,
	// because it is equivalent to granting NATS permissions.
	//
	// `_INBOX.>` is on PUBLISH for the same class of reason. A `reply` operation
	// means the thing answers requests, and an answer goes to the requester's
	// inbox — so a type declaring `reply_diagnostics` needs an inbox publish or
	// its request/reply half is dead. Three types declared one against a role
	// that had `_INBOX.>` on subscribe only, which is the requester's side of the
	// pair, not the responder's.
	//
	// TestEveryThingTypeCanSpeakItsOwnContract now checks both directions for
	// every type against the role it actually points at. Nothing at runtime does:
	// a role is a set of subject patterns, a thing type is a prefix plus a list
	// of operations, and the two are edited on different screens.
	//
	// The lists are subtree-coarse throughout (`telemetry.>`, not one subject per
	// device), so a role grants a KIND of participation rather than an identity's
	// exact contract. Narrowing per thing is a per-user `publish_permissions`
	// edit, which is owner/admin-only precisely because it is equivalent to
	// granting NATS permissions.
	{Name: "device", IsDefault: true,
		Description:      "A field device: publishes its own telemetry and status, listens for commands addressed to it.",
		Publish:          []string{"telemetry.>", "event.>", "asset.>", "line.>", "turbine.>", "status.>", "acc.>", "_INBOX.>"},
		Subscribe:        []string{"telemetry.>", "event.>", "asset.>", "line.>", "turbine.>", "status.>", "acc.>", "cmd.>", "config.>", "_INBOX.>"},
		MaxSubscriptions: 64, MaxPayload: 1048576},

	{Name: "gateway",
		Description: "A site aggregator: everything a device may do, plus the site's own subtree and JetStream access for the local mirror.",
		// `$KV.>` is NOT covered by `$JS.API.>`, and the gap is invisible until an
		// edge service tries to write a bucket. Reading a KV bucket goes through
		// the JetStream API (bind the stream, create a consumer, deliver to an
		// inbox), so a role with `$JS.API.>` + `_INBOX.>` watches KV perfectly
		// well — but a WRITE is a plain publish to `$KV.{bucket}.{key}`, which
		// matches neither. An access controller came up, synced its whole policy
		// graph, armed every portal, and then failed on the first status write
		// with `Permissions Violation for Publish to "$KV.ACC_STATUS.portal.…"`.
		// A box that boots clean and cannot report state is worse than one that
		// refuses to start.
		//
		// It is on `gateway` and deliberately not on `device`: a gateway runs the
		// edge services that own buckets (leaf node, rule engine, access
		// controller), while a field sensor publishes telemetry and lets the edge
		// relay it. `device` has no JetStream access at all, so `$KV.>` alone
		// would not even let it bind a bucket — half a permission is worse than
		// none, because it reads as support for something that cannot work.
		Publish:          []string{"telemetry.>", "event.>", "status.>", "gateway.>", "acc.>", "kiosk.>", "_INBOX.>", "$JS.API.>", "$KV.>"},
		Subscribe:        []string{"telemetry.>", "event.>", "status.>", "gateway.>", "acc.>", "kiosk.>", "cmd.>", "config.>", "_INBOX.>"},
		MaxSubscriptions: 512, MaxPayload: 4194304},

	{Name: "application",
		Description: "A software participant: publishes on its own app subtree and on the operator service contract, reads broadly.",
		// `kiosk.>` and `$KV.>` are the kiosk controller's half of the same lesson
		// the gateway role learned above, arrived at from the other direction.
		//
		// An application that AGGREGATES has no subtree of its own to work in. The
		// controller requests on each kiosk's `kiosk.<code>.command.<name>` and
		// writes the catalogue, the fleet punch state and the fleet open-checkout
		// state into KV. Subscribe is already `>`, so reading looked complete and
		// the gap showed up only on writes: catalogue fan-out is a plain publish
		// to `$KV.catalog_items.<kiosk>.<sku>`, which `$JS.API.>` does not cover.
		// Without it the controller starts, serves its console, aggregates every
		// event the fleet sends — and silently ships no catalogue, so the kiosks
		// stock nothing and the failure reads as a kiosk-side bug.
		Publish:   []string{"app.>", "cmd.>", "helpdesk.>", "kiosk.>", "$JS.API.>", "$KV.>"},
		Subscribe: []string{">"},
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
