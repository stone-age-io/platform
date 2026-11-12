# Querying the demo estate

[`northwind.conf`](northwind.conf) puts the Northwind demo's NATS traffic into
VictoriaMetrics. This file is what to type once it is running.

The companion config in the access-control repo
(`demo/telegraf/northwind-access.conf`) writes access events into the **same**
VictoriaMetrics from the **same** account, so the queries below cross both apps.
Neither needs the other.

## What has to be running

VictoriaMetrics, Telegraf, and something publishing.

```bash
victoria-metrics -storageDataPath=./vmdata -retentionPeriod=30d
telegraf --config demo/telegraf/northwind.conf
```

- The traffic comes from the Northwind rules ([../rules](../rules/README.md)).
- The access half also needs the four `access-controller` processes, which emit
  the decisions, posture changes and heartbeats. With the scheduler alone, only
  `event_kind="alarm"` fills in.
- [Where things are](#where-things-are) also needs the inventory feed in
  [../inventory](../inventory/README.md).

VictoriaMetrics has a query UI at **http://localhost:8428/vmui/**. It is storage
for history and decides nothing: alarms belong on the bus, in rule-router.

---

## Open with one line

```
thing_celsius
```

Twelve probes at once: ten fixtures and both reefers' return-air probes.

## Cold chain

```
thing_celsius - thing_setpoint_c
```

Drift from setpoint puts freezers (−18) and chillers (+2) on one zero-centred
line. It works because each probe publishes its setpoint in the same payload.

```
avg_over_time(thing_celsius{thing="TP-001"}[10m])
```

A windowed average is a query, not a pipeline: nothing computes it in advance.

## The failing device

```
thing_percent < 25
```

One series: `TP-005`, around 19–20%, in `KC-DC1-FZ1`. It is also the excursion
probe (about −9 against −18, on a 47-minute cron), so it eventually shows in the
drift query too.

## Liveness

```
count by (thing) (count_over_time(thing_uptime[3m]))
```

Heartbeats per device over three minutes; a device that stops reporting drops
out. `stone_age_records` counts devices configured and cannot tell you this.

```
count_over_time(access_count{event_kind="heartbeat"}[1m])
```

Per-controller beats at a 15-second cadence, so `< 3` is a quiet box, named by
site and code. accessd already alarms on this after 45 seconds; this is the
history of it.

## Access decisions

```
sum by (reason) (count_over_time(access_count{event_kind="tap"}[15m]))
```

The reason codes `policy.Decide` produced, such as `allow_grant`,
`deny_no_access` and `deny_revoked`. Edit an access group in the console and the
next taps change the graph.

```
sum by (thing) (count_over_time(access_count{event_kind="alarm"}[1h]))
```

Alarms by portal: forced, held, held_clear, intrusion.

```
sum(count_over_time(access_count{event_kind="tap", allow="true"}[1h]))
  / sum(count_over_time(access_count{event_kind="tap"}[1h]))
```

Allow rate, as a single stat.

## Fleet

```
count by (version) (thing_uptime)
```

Firmware spread. The empty group is the probes and door sensors, whose
heartbeats carry no version.

---

## Where things are

**Not yet run against a live estate**: the inventory feed was run on the bus,
but not through Telegraf and VictoriaMetrics. Treat these as intended queries.
How the join works, and its limits:
[Observability §5](https://github.com/stone-age-io/platform-docs/blob/main/docs/observability.md).

Readings carry `thing` only. `stone_thing_info` carries the rest, one series per
Thing, refreshed every minute. Join on `(org, thing)`, since a code is unique
only inside its organization:

```
avg by (location) (
  thing_celsius
    * on(org, thing) group_left(name, location, location_path)
      (stone_thing_info @ end())
)
```

Everything under one place, by path:

```
thing_celsius
  * on(org, thing) group_left(name, location)
    (stone_thing_info{location_path=~".*/KC-DC1/.*"} @ end())
```

Sites side by side, as a Grafana variable for a repeated panel:

```
label_values(stone_location_info{location_type="warehouse"}, location)
```

The access config still tags `location` from its subject, so its events work
with a bare selector; to put them beside readings by current place, join
through `stone_thing_info` as above.

---

## Metric reference

Readings land as measurement `thing` and access events as `access`, so each
series is `<measurement>_<field>`.

| Metric | What it is | Labels |
|---|---|---|
| `thing_celsius` | probe / reefer return-air reading | `thing` |
| `thing_setpoint_c` | the setpoint that reading is against | `thing` |
| `thing_percent` | battery charge | `thing` |
| `thing_volts` | battery voltage | `thing` |
| `thing_uptime` | seconds since boot — presence is the liveness signal | `thing` `version` |
| `thing_rssi_dbm` | radio signal on battery devices | `thing` |
| `thing_engine_hrs` | reefer engine hours | `thing` |
| `thing_load` | gateway load average | `thing` `version` |
| `thing_rules` | rules loaded by the cold-chain engine | `thing` `version` |
| `thing_lines` | lines on a WMS shipment | `thing` `event` `site` |
| `thing_duration` | how long a dock door held its previous state | `thing` `state` |
| `stone_thing_info` | constant 1 — one series per Thing, from the inventory feed | `thing` `name` `thing_type` `location` `location_path` |
| `stone_location_info` | constant 1 — one series per Location | `location` `name` `location_type` `location_path` |
| `access_count` | constant 1 — one sample per access event | `location` `thing` `thing_type` `event_kind` `allow` `reason` `source` `type` `point` |

`org` is on everything. `event_kind` is `tap`, `alarm`, `state` or `heartbeat`.
`thing_type` is the portal type, or `ctrl` for a controller. Readings carry no
`location`: place comes from `stone_thing_info`, where it is current.

## Adding a message type

| Change | What you do |
|---|---|
| New numeric field on a subject already matched | nothing — it appears as `thing_<field>` |
| New string field you want to group by | add it to `tag_keys`, after checking it is bounded |
| New string field you do not add | nothing appears, and nothing warns you |
| New subject suffix (a new operation) | add one line to `subjects` |
| New thing of an existing type | nothing |

The subject list is written out by hand on purpose: it ingests measurements and
skips echoes, `.diag` requests, `$KV.twin.>` and screen contents, which no
wildcard can tell apart. A wider pattern also matches subjects with no `ts`
field and logs a parse error per message. After adding a type, check the field
arrived:

```bash
curl -s localhost:8428/api/v1/label/__name__/values
```

## Things that will look broken and are not

- **`thing_duration` is empty most of the day.** The dock door rules run
  `5-21` in `America/Chicago`. `event_kind="state"` needs a posture change; the
  nightly gate lockdown fires one at 20:00.
- **Instant queries return nothing for the first half-minute**, because
  VictoriaMetrics evaluates them at `now - 30s`.
- **Do not build a prediction panel.** The generator emits bounded noise, not
  trends, so `predict_linear` fabricates a slope.
