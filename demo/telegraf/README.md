# Querying the demo estate

[`northwind.conf`](northwind.conf) puts the Northwind demo's NATS traffic into
VictoriaMetrics. This file is what to type once it is running.

The companion config in the access-control repo
(`demo/telegraf/northwind-access.conf`) writes access events into the **same**
VictoriaMetrics from the **same** account, so the queries below cross both apps.
Neither config is required by the other — run one, or both.

Every query here was run against a live estate before being written down. The
numbers quoted are what it actually read at the time, not what it ought to read.

## What has to be running

VictoriaMetrics, Telegraf, and something publishing.

```bash
victoria-metrics -storageDataPath=./vmdata -retentionPeriod=30d
```

```bash
telegraf --config demo/telegraf/northwind.conf
```

The traffic comes from `rule-router --rules demo/rules/northwind` (see
[../rules/README.md](../rules/README.md)). The access half additionally needs the
four `access-controller` processes: the scheduler publishes taps and injects
alarms, but the *decisions*, the posture changes and the controller heartbeats
are emitted by the controllers themselves. Run the scheduler alone and only
`event_kind="alarm"` fills in — a correct reading of the bus, not a broken
config, and worth knowing before you debug an empty screen.

VictoriaMetrics ships a query UI at **http://localhost:8428/vmui/**. No Grafana
needed to work through this file.

## VictoriaMetrics is storage here, and nothing else

It holds history for charts, comparison and reporting. It decides nothing.
Thresholds and alarms belong on the bus, in rule-router, at the edge, where they
still work with the WAN down — and where they are not sitting behind a 10-second
flush, an ingest, and an evaluation interval. Do not put an alert path in front
of this. The access app already has one that runs beside the controllers.

---

## Open with one line

```
thing_celsius
```

Twelve probes at once — ten fixtures and both reefers' return-air probes.
Establishes that it is real in about two seconds.

## Cold chain

```
thing_celsius - thing_setpoint_c
```

The best query in the set. Freezers hold −18 and chillers +2, so raw temperature
cannot be compared across zones; drift from setpoint normalises the whole estate
onto one zero-centred line, and one panel then serves every zone type. Reads ±0.1
to ±1 on a healthy estate.

This works because the probes publish their setpoint alongside the reading, in
the same payload, on the same subject. That is a property of the thing type's
contract, not of this query.

```
avg_over_time(thing_celsius{thing="TP-001"}[10m])
```

Worth saying out loud while you show it: **the windowed average is a query, not a
pipeline.** Nothing computes it in advance, nothing accumulates state, and there
is no stream processor in the path. A window only needs to become a *message*
when something has to act on it — an alarm, or a downstream system that wants
15-minute records. A window that only needs to become a number on a chart is
this.

## The failing device

```
thing_percent < 25
```

Returns exactly one series: `TP-005`, around 19–20%, in `KC-DC1-FZ1`. One line,
one answer, no dashboard.

TP-005 is also the excursion probe — it reports about −9 against a −18 setpoint,
on a 47-minute cron, so leave the demo running and it eventually appears in the
drift query too. A sensor dying in a freezer that is warming, which is the
scenario the seed was built to tell.

## Liveness — what an inventory count cannot tell you

```
count by (thing) (count_over_time(thing_uptime[3m]))
```

Heartbeats per device over three minutes. A device that stops reporting drops out
of the result.

This is the query that earns the whole exercise. `stone_age_records{collection=
"leaf_nodes"}` counts things **configured** and can never fire on a device going
quiet — its HELP text says so. Availability is a property of traffic, and traffic
is on the bus.

```
count_over_time(access_count{event_kind="heartbeat"}[1m])
```

Per-controller beats at a 15-second cadence, so `< 3` is a box that has gone
quiet, named by site and code. `controller_heartbeats_received_total` on
accessd's own `/metrics` is labelled by status only — deliberately, because a
per-controller label there would size every deployment's label set to its
customer's estate. The subject carries what the label cannot.

Note what this is **not**: accessd's own sweep already flips a controller offline
after 45 seconds, and that is where the alarm belongs. This is the history of
that fact — how often, how long, which site, over months.

## Access decisions

```
sum by (reason) (count_over_time(access_count{event_kind="tap"}[15m]))
```

Read `allow_grant 15`, `deny_no_access 1`, `deny_revoked 1` on a quiet quarter
hour. These are the codes `policy.Decide` actually produced against seeded
credentials — edit an access group in the console and the next taps change the
graph. Nothing in the rules file types a reason string.

```
sum by (thing) (count_over_time(access_count{event_kind="alarm"}[1h]))
```

Alarms by portal. Forced, held, held_clear, intrusion.

```
sum(count_over_time(access_count{event_kind="tap", allow="true"}[1h]))
  / sum(count_over_time(access_count{event_kind="tap"}[1h]))
```

Allow rate. Useful as a single stat; useless as an alarm, because the right value
is a property of the site.

## Two apps, one system

```
{location="KC-DC1"}
```

A bare label selector, no join syntax, and it returns **badge decisions and
building telemetry side by side** — `access_count` next to `thing_percent`,
`thing_uptime`, `thing_rssi_dbm`, `thing_volts`.

That is the shared-account story in one line, and it only works because both
configs derive `location` and `thing` from the subject with the same tag names —
the names `internal/subjects/subjects.go` uses for the slots. Keep them in step
if you add a third app.

## Fleet

```
count by (version) (thing_uptime)
```

Firmware spread. Reads `{} 6`, `{version="0.4.0"} 2`, `{version="1.9.0"} 1` —
the empty group is the probes and door sensors, whose heartbeat payloads carry no
version. That is honest rather than tidy: a device that does not report its
firmware should not be counted as running any particular one.

---

## Metric reference

Everything from `northwind.conf` lands as measurement `thing`, and everything
from the access config as `access`. VictoriaMetrics names each series
`<measurement>_<field>`, so the field name is the metric name's second half.

| Metric | What it is | Labels |
|---|---|---|
| `thing_celsius` | probe / reefer return-air reading | `location` `thing` |
| `thing_setpoint_c` | the setpoint that reading is against | `location` `thing` |
| `thing_percent` | battery charge | `location` `thing` |
| `thing_volts` | battery voltage | `location` `thing` |
| `thing_uptime` | seconds since boot — presence is the liveness signal | `location` `thing` `version` `app_kind` |
| `thing_rssi_dbm` | radio signal on battery devices | `location` `thing` |
| `thing_engine_hrs` | reefer engine hours | `location` `thing` |
| `thing_load` | gateway load average | `location` `thing` `version` |
| `thing_rules` | rules loaded by the cold-chain engine | `app_kind` `thing` `version` |
| `thing_lines` | lines on a WMS shipment | `app_kind` `thing` `event` `site` |
| `thing_duration` | how long a dock door held its previous state | `location` `thing` `state` |
| `access_count` | constant 1 — one sample per access event | `location` `thing` `thing_type` `event_kind` `allow` `reason` `source` `type` `point` |

`org` is on everything. `event_kind` is `tap`, `alarm`, `state` or `heartbeat`.
`thing_type` is the portal type, or the literal `ctrl` for a controller — in
which case `thing` is the controller code.

**`location` vs `app_kind`.** A device is addressed `<root>.{location}.{thing}`;
an application is addressed `app.{kind}.{thing}`, where the second slot is a
subtree token (`wms`, `rules`) and not a place. Software has no site, so it gets
a different tag rather than a wrong one. `thing_lines` carries `site` instead,
which the WMS puts in the payload precisely because its subject has nowhere to
put one.

---

## Extending it — what happens when you add a message type

**Fields are automatic. Subjects are not.** That asymmetry is the whole answer,
and it is deliberate on both halves.

### Fields: every number, no config

Feed the parser a payload with one of everything and this is what comes out:

```json
{"ts":"…","celsius":-18.2,"setpoint_c":-18.0,"probe":"TP-011","new_metric":42.5,
 "new_label":"alpha","enabled":true,"door_count":7,
 "nested":{"depth":3,"name":"x"},"arr":[1,2,3]}
```

```
thing arr_0=1,arr_1=2,arr_2=3,celsius=-18.2,door_count=7,nested_depth=3,new_metric=42.5,setpoint_c=-18
```

- **Every JSON number becomes a field**, named after its key. Add `new_metric` to
  a payload a subject below already matches and `thing_new_metric` appears in
  VictoriaMetrics with no edit to anything.
- **Strings and booleans are dropped** unless named in `tag_keys`. `probe`,
  `new_label` and `enabled` vanished without a word.
- **Nested objects flatten** with `_`: `nested.depth` → `nested_depth`.
- **Arrays become indexed fields**: `arr_0`, `arr_1`, `arr_2`. A variable-length
  array therefore produces a growing set of metric names — avoid putting one in a
  payload you intend to measure.

The string/number asymmetry is a **safety feature, not a limitation**. A number
is a value: ingesting one adds a series. A string is a label: ingesting one
*multiplies* series, and a single unbounded string field — an order number, a
trace id, a cardholder — is a cardinality bomb that is painful to undo after the
fact. Numbers opt out, strings opt in. That is the right default and it is the
same rule the tag lists in the configs are already following.

### Subjects: one line per new operation, on purpose

A new **thing of an existing type** needs nothing: `telemetry.*.*.temperature`
already covers an eleventh probe. A new **operation** — a new subject suffix —
is not ingested at all, because nothing subscribes to it. Nothing warns you
either; the series simply never appears.

The tempting fix is to widen the patterns to `telemetry.*.*.*` and be done. Do
not, and the reason is concrete: `asset.*.*.*` also matches
`asset.TRL-1180.REEFER-1180.setpoint`, a *subscribe*-side subject with no `ts`
field, and every message on it then logs

```
E! Error in plugin: could not parse: 'json_time_key' could not be found
```

one line per message, forever. It also matches `.diag` **requests**, whose body
is whatever the requester sent — frequently empty. Widening trades one curation
decision for permanent log spam and junk series.

The real reason the list is written out longhand is that **it is an editorial
decision, not a mechanical one.** This config deliberately ingests 32 of the 44
subjects the rules file publishes and skips the other 12 — setpoint echoes
because an echo is an assertion rather than a measurement, `.diag` because
request/reply has no feed, `$KV.twin.>` because that state is already durable
with history, `.render` because it is a screen's contents. No wildcard can
express *"this is a measurement"*.

### If you want it generated

The platform already holds the answer. Thing types carry a `subject_prefix`,
operations carry a `subject_suffix` **and a `capability`** — `publish`,
`subscribe`, `request` or `reply` — and the console composes exactly these
strings on the Thing Type screen. So the `subjects` array is derivable: every
`{subject_prefix}.{subject_suffix}` whose capability is `publish`, with the
location and thing slots wildcarded.

One caveat keeps the editorial point alive: `publish_setpoint_echo` is
`capability: "publish"` and is excluded here anyway. Capability gets you most of
the list; measurement-versus-assertion is still a human call. A generator should
print the candidate list for someone to strike lines from, not write the file
unattended.

### Checklist

| Change | What you do |
|---|---|
| New numeric field on a subject already matched | nothing — it appears as `thing_<field>` |
| New string field you want to group by | add it to `tag_keys`, after checking it is bounded |
| New string field you do not add | nothing appears, and nothing warns you |
| New subject suffix / new operation | add one line to `subjects` |
| New thing of an existing type | nothing |

Row three is the operational risk worth naming: the failure mode is **silence**,
not an error. The cheapest check after adding a type is

```bash
curl -s localhost:8428/api/v1/label/__name__/values
```

and confirming the field you expected is in the list.

---

## Things that will look broken and are not

**`thing_duration` is empty most of the day.** The dock door rules are gated
`5-21` in `America/Chicago`, so dwell times appear during warehouse shift and
nowhere else. Same for `event_kind="state"`, which needs a posture change — the
nightly gate lockdown fires it at 20:00.

**Instant queries return nothing for the first half-minute.** VictoriaMetrics
evaluates an instant query at `now - 30s` by default, so until there is more than
that much history the evaluation point predates all the data. Wait, or use a
range.

**`location` does not join across the two apps.** Telemetry is tagged at zone
codes (`KC-DC1-FZ1`) and access at site codes (`KC-DC1`), because that is where
each thing actually sits in the inventory. `{location="KC-DC1"}` works only
because the door sensors happen to be at site level. To roll zones up:

```
label_replace(avg by (location) (thing_celsius), "site", "$1", "location", "^([A-Z]+-[A-Z0-9]+).*")
```

Fine for a demo. If you want it properly it should be a real `site` tag, not a
regex repeated in every query — the location hierarchy is in the platform, and
the subject only carries the leaf.

## Do not build a prediction panel on this data

`predict_linear(thing_percent[1h], 86400*30)` will render, look impressive, and
mean nothing. The generator emits **bounded noise, not shapes** — batteries here
jitter across a couple of tenths of ADC noise, they do not drain, and the rules
file says so where it sets the range. Every `random` band in
`demo/rules/northwind/telemetry.yaml` is a claim about physics, not a trend.

Trend and prediction queries are legitimate against real devices. Against this
seed they fabricate a slope out of noise, which is the one way a demo can lie
about the product rather than about the data.
