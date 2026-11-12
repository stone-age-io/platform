# Live demo telemetry (rule-router)

`stone-age demo-seed` fills the console with inventory, but a dashboard needs a
stream of readings. These [rule-router](https://github.com/skeeeon/rule-router)
rules put that traffic on the bus, on the exact subjects the seeded things
declare.

```
northwind/telemetry.yaml    cold chain: probes, batteries, dock doors, reefers, WMS, diagnostics, twin state
ironbridge/telemetry.yaml   plant: panel power, bearing vibration, cycles, alarms, OEE, diagnostics
galewind/telemetry.yaml     wind: turbines, curtailment, feeders, ERCOT dispatch, diagnostics
```

They speak for about forty of the seeded things, on 124 rules: 111 on a
schedule and 13 that answer requests. The rest stay silent on purpose, because
inventory is recorded before every device is commissioned. This is demo tooling;
nothing here ships in the binary. The inventory feed for the TSDB is separate,
in [../inventory](../inventory/README.md).

## One process per organization — this is not optional

In NATS operator mode the **account is the tenant boundary**. Each seeded
organization holds its own account, and a connection authenticated into one
cannot publish into another. So each directory above is its own rule-router
process with its own credential, all sharing [`rule-router.yaml`](rule-router.yaml):

```bash
RR_NATS_CREDSFILE=/path/to/console-dana.creds   rule-router --config demo/rules/rule-router.yaml --rules demo/rules/northwind
RR_NATS_CREDSFILE=/path/to/console-marcus.creds rule-router --config demo/rules/rule-router.yaml --rules demo/rules/ironbridge
RR_NATS_CREDSFILE=/path/to/console-sofia.creds  rule-router --config demo/rules/rule-router.yaml --rules demo/rules/galewind
```

Run them from the platform repository root. **Point `--rules` at one
organization's directory, never at `demo/rules`**: the parent loads all three
files into one process, and two thirds of the rules then publish subjects the
connection has no permission for.

## The bus: use the platform's own embedded NATS

`serve --nats` runs a NATS server inside the Control Plane process, reading the
same `nats.conf` that `nats export` writes. It is an ordinary nats-server in
**operator mode** — which is the point: the account boundary above is real, and
every credential below is a signed JWT the platform minted.

```bash
./stone-age init --email admin@demo.local --operator-org "816tech"
./stone-age demo-seed --confirm
./stone-age serve --nats
```

An external `nats-server -c nats-config/nats.conf` works exactly the same way —
same config, same credentials. Only the process boundary differs.

## Which credential

Any NATS identity in the org whose role permits the publishes. The seeded
`device` and `gateway` roles each cover part of the subject space and nothing
covers all of it — which is correct, and is why a simulator is not a device.

Use a **`console-operator`** identity, which publishes `>` within its own
account and nowhere else:

1. Sign in as the org owner — `dana@northwind.example`,
   `marcus@ironbridge.example`, `sofia@galewind.example`, password `demo1234`.
2. **NATS → Users**, open the user linked to that membership (`console-dana`,
   `console-marcus`, `console-sofia`), **Download credentials**.
3. Pass its path as `RR_NATS_CREDSFILE` (above).

That credential is broad on purpose and it is still bounded by the account, so
the worst it can do is make a mess of one tenant's own bus. Do not reuse the
pattern for a real device — a device gets `device`.


## What you should see

Point a dashboard widget at any of these:

| Widget | Subject | jsonPath |
|---|---|---|
| Chart | `telemetry.KC-DC1-FZ1.TP-001.temperature` | `$.celsius` |
| Chart | `turbine.T-114.TC-114.generation` | `$.kw` |
| Chart | `telemetry.PIT-B1.PM-001.power` | `$.kw` |
| Stat | `telemetry.LINE-B.VS-002.vibration` | `$.rms_mm_s` |
| Gauge | `telemetry.KC-DC1-FZ1.TP-005.battery` | `$.percent` |
| Chart | `app.market.MARKET-01.dispatch` | `$.price_mwh` |
| Console | `event.KC-DC1.DS-001.door` | — |
| Console | `app.wms.WMS-CONN-01.shipment` | — |
| Console | `telemetry.KC-DC1-FZ1.TP-001.heartbeat` | — |

The top three publish every five seconds; nearly everything else moves on a
ten- to forty-five-second cron.

## Diagnostics

Six seeded thing types declare `reply_diagnostics`: the device subscribes to
its own `<subject_prefix>.diag` and answers on the requester's inbox. Thirteen
rules stand in for it, one per commissioned reefer, line controller, turbine and
gateway:

| Org | Subject | Answers for |
|---|---|---|
| northwind | `asset.TRL-1180.REEFER-1180.diag` | reefer: setpoint, supply/return, defrost, fuel |
| northwind | `gateway.KC-DC1-MDF.GW-KC-01.diag` | gateway: leaf link, sync lag, disk, clients |
| ironbridge | `line.LINE-A.LC-LINE-A.diag` | PLC: mode, scan time, I/O, faults |
| galewind | `turbine.T-114.TC-114.diag` | turbine: curtail ceiling, rotor, gearbox temp |

```bash
nats req --creds console-dana.creds asset.TRL-1180.REEFER-1180.diag ''
```

Or from the console: a **Publisher** widget on the subject has a **Request**
button, and a **Button** widget set to `request` does it in one click.

- **No `conditions`**, so an empty request still gets an answer rather than a
  timeout that looks like a dead device.
- **Core transport**: `reply: true` implies it, and no stream is needed.
- **Assertions stay fixed**: a reply's setpoint, mode and curtailment match the
  `.echo` rules and `twin` keys.
- **TC-211 does not answer**, because the seed marks that turbine inactive.

The two `request` operations (`request_inventory`, `request_forecast`) are not
simulated: a scheduled rule has nowhere to receive an answer.

## What these files need from rule-router

Six-field cron, the `{@random.*}` template functions, and `reply: true`
triggers with a `respond` action. A build missing any of them refuses the rules
at load. Three things to know before copying the pattern:

- **It is bounded noise, not a shape.** Batteries jitter rather than drain, so
  do not build trend or prediction panels on it.
- **Independent draws lose correlation.** Galewind's `kw` and `wind_ms` each
  look right alone, but not as a pair, and Ironbridge's published OEE is not
  the product of its factors.
- **Random goes on measurements, never on assertions.** Setpoints, echoes,
  modes and curtailment ceilings stay fixed, or every desired/reported pair in
  the twin screen differs.

## Volume

Each org publishes a few hundred messages a minute. To calm it, widen the crons
or drop the seconds field. Alarms, excursions, market intervals and every KV
write already run on minute crons.

## The Digital Twin section

Each file ends with rules that write **reported** state into the `twin` KV
bucket. It is separable, and the bucket must exist first: **NATS → KV Buckets →
Initialize** in the console, or `nats kv add twin --history=10 --storage=file`
with the same creds. The platform server cannot create it. rule-router names
any subject it cannot reach at startup.

- **It writes `twin`, never `twin_desired`.** One writer per bucket; these rules
  stand in for devices.
- **It publishes to `$KV.twin.{key}`** with `mode: jetstream`, so a missing
  bucket fails on the ack rather than looking like an empty screen.

To see drift, set `twin_desired` `thing.REEFER-1180.setpoint` to `-20` in the
console. The row shows `-18 → -20`, and nothing changes it: no part of the
platform applies a desired value to a device.

## Alongside the access-control demo

`stone-access` has its own rules (`demo/rules/` in that repo) driving the same
Northwind organization with real credential presentations at the ten seeded
doors. Run both and the Northwind account carries inventory telemetry and
access traffic at once.
