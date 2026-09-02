# Live demo telemetry (rule-router)

`stone-age demo-seed` fills the console with inventory: three organizations,
twenty-six locations, about a hundred and twenty things, their contracts and
their signed identities. What it cannot fill is a **dashboard** — a chart needs a
stream of readings, and the seed writes records, not traffic.

These are [rule-router](https://github.com/stone-age-io) scheduler rules that put
that traffic on the bus, on the exact subjects the seeded things declare.

```
northwind/telemetry.yaml    cold chain: probes, batteries, dock doors, reefers, WMS, twin state
ironbridge/telemetry.yaml   plant: panel power, bearing vibration, cycles, alarms, OEE
galewind/telemetry.yaml     wind: turbines, curtailment, feeders, ERCOT dispatch
```

Between them they speak for about **forty** of the seeded things, on **111**
rules. The rest stay silent on purpose: inventory is recorded long before every
device is commissioned, and a demo where all 120 reported would misrepresent both
the platform and the work. It is the reason the Things list has an `active`
filter, and the reason `stone_age_records` counts things *configured* rather than
things alive.

Every device type that declares `publish_heartbeat` now sends one. Until
recently only gateways and applications did, which left the console unable to
tell a commissioned device from a merely recorded one — exactly the distinction
`active` exists to make legible.

Nothing here ships in the binary and nothing here is required to run the
platform. It is demo tooling.

## One process per organization — this is not optional

In NATS operator mode the **account is the tenant boundary**. Each seeded
organization holds its own account, and a connection authenticated into one
cannot publish into another. So each directory above is its own rule-router
process with its own credential:

```bash
rule-router --config config/rule-scheduler.yaml --rules demo/rules/northwind
```

```bash
rule-router --config config/rule-scheduler.yaml --rules demo/rules/ironbridge
```

```bash
rule-router --config config/rule-scheduler.yaml --rules demo/rules/galewind
```

**Point `--rules` at the organization's directory, never at `demo/rules`.**
rule-router walks the path recursively, so the parent loads all three files into
one process — and two thirds of the rules then publish subjects the connection
has no permission for, filling the log with authorization errors for rules that
are perfectly correct.

`features.scheduler: true` is the only feature these need.

## The bus: use the platform's own embedded NATS

`serve --nats` runs a NATS server inside the Control Plane process, reading the
same `nats.conf` that `nats export` writes. It is an ordinary nats-server in
**operator mode** — which is the point: the account boundary above is real, and
every credential below is a signed JWT the platform minted.

```bash
./stone-age superuser upsert admin@demo.local 'demo-pass-1234'
./stone-age migrate up
./stone-age bootstrap --email admin@demo.local --org "System" --operator-org "816tech"
./stone-age demo-seed --confirm
./stone-age nats export -o ./nats-config
./stone-age serve --nats --nats-config ./nats-config/nats.conf
```

`nats export` writes absolute paths into the generated config, so `serve --nats`
runs from wherever you like. (It needs pb-nats **v0.2.1 or later**: in v0.2.0 the
export flag collided with the host's own `--config`, so `-o` printed to stdout and
wrote nothing at all.)

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
3. Point rule-router at it: `nats.credsFile: /path/to/user.creds`.

That credential is broad on purpose and it is still bounded by the account, so
the worst it can do is make a mess of one tenant's own bus. Do not reuse the
pattern for a real device — a device gets `device`.

> On Windows, quote the path in YAML with **single** quotes. A backslash path in
> double quotes is a YAML escape sequence, and `C:\Users\…` fails to parse with
> `did not find expected hexdecimal number`.

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

The top three fill in **under a minute** — they publish every five seconds.
Nearly everything else moves too, on a ten- to forty-five-second cron.

## What these files need from rule-router

A build with **six-field cron** and the **`{@random.*}` template functions**. On
an older one every rule fails to load, loudly, at startup — which is the right
failure, because a silently ignored seconds field would publish sixty times
slower than the files claim.

Both are recent, and the files were rewritten around them. What that changed:

**Cron takes an optional seconds field.** `*/5 * * * * *` is every five seconds.
One second is the floor — cron cannot express less. A rule that fires faster than
its action completes drops the overlap rather than stacking it, and the drop is
counted at `scheduler_job_runs_total{status="singleton_rescheduled"}` with the
first one per schedule logged.

**Payloads move.** `{@random.float(min,max,decimals)}`, `{@random.int(min,max)}`
and `{@random.choice(a,b)}` replaced the phase-shifted blocks each file used to
carry — six rules for one temperature probe, six for one turbine, four for one
power meter — with one rule each.

### Three things worth knowing before you copy the pattern

**It is bounded noise, not a shape.** The old six-rule blocks modelled a
compressor cycle and a wind ramp: repeating *shapes*. A random band never draws
one. For a probe in a working freezer that is arguably more honest; for anything
where the shape is the point, phase-shifted rules still work — they just shift on
seconds now.

**Independent draws lose correlation.** Galewind publishes `kw` and `wind_ms`
from two separate ranges, so each series looks right alone and the *pairing* is
wrong on any given sample. Fine for a chart, wrong for fitting a power curve.
Ironbridge's OEE has the same property: the published `oee` is not the product of
its three factors, because a template has no arithmetic. Each file says so where
it happens.

**Random goes on measurements, never on assertions.** Temperatures, power and
vibration are randomised. Setpoints, setpoint echoes, line modes and curtailment
ceilings are *not*, and must not be — an echo is the device repeating the
instruction it accepted, so a value that wanders by a tenth turns every
desired/reported pairing in the Digital Twin screen permanently red. Same reason
Ironbridge's scrap rate stays on its own cron instead of becoming a
`{@random.choice(true,false)}`: the cron sets the rate, random sets the value.

## Volume, and turning it down

Each org publishes on the order of a few hundred messages a minute — nothing for
NATS, but more than you want in a log you are reading. To calm it, widen the
crons or drop the seconds field to go back to minute granularity. Alarms,
excursions, market intervals and every KV write were deliberately **left on
minute crons**; each file says why where it does it.

## The Digital Twin section

Each file ends with a block that writes **reported** state into the `twin` KV
bucket. It is separable — delete it if you are not using it — and it needs the
bucket to exist first:

**NATS → KV Buckets → Initialize** in the console, or let `leaf-sync` create it.
The platform server *cannot*: it holds the NATS operator and has no user
credential inside any organization's account. From a shell it is
`nats kv add twin --history=10 --storage=file` with the same creds file.

You do not have to guess whether you got this right. rule-router names the
subjects it cannot reach **at startup**, before the first cron fires:

```
rules publish to JetStream subjects with no matching stream; these publishes
will time out waiting for an ack and then fail — add a stream covering them or
set 'mode: core' on the action
  subjects: 4
  examples: ["$KV.twin.thing.REEFER-1180.setpoint", ...]
```

Two things about that block are load-bearing:

- **It writes `twin` and never `twin_desired`.** One writer per bucket is the
  whole safety property — `twin` flows edge→hub and the device owns it,
  `twin_desired` flows hub→edge and the operator owns it. A bucket written from
  both ends does not pick a loser on a conflict, it oscillates. These rules stand
  in for devices, so they write the device's half.
- **It publishes to `$KV.{bucket}.{key}`.** rule-router has no KV action, and a
  KV bucket is a stream whose subjects are exactly that — it is what `nats kv
  put` does. `mode: jetstream` on those rules and nowhere else, because the ack
  is what tells you the bucket exists; published core, a missing bucket looks
  identical to a working demo with an empty screen.

To see drift: set `twin_desired` `thing.REEFER-1180.setpoint` to `-20` in the
console. The row shows `-18 → -20`. Then watch nothing happen — no hook, no
agent and no subscription in this platform applies a desired value to a device,
which is why the console says `differs` and never `pending`.

## Alongside the access-control demo

`stone-access` has its own rules directory (`demo/rules/` in that repo) driving
the same Northwind organization: real credential presentations at the ten seeded
doors, decided by running `access-controller` processes. The two are
complementary and share an account — run both and the Northwind bus carries
inventory telemetry on `telemetry.>`/`event.>`/`asset.>` and access traffic on
`acc.>` at the same time, which is what a tenant's account actually looks like.
