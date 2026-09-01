# Live demo telemetry (rule-router)

`stone-age demo-seed` fills the console with inventory: three organizations,
twenty-six locations, about a hundred and twenty things, their contracts and
their signed identities. What it cannot fill is a **dashboard** — a chart needs a
stream of readings, and the seed writes records, not traffic.

These are [rule-router](https://github.com/stone-age-io) scheduler rules that put
that traffic on the bus, on the exact subjects the seeded things declare.

```
northwind/telemetry.yaml    cold chain: probes, dock doors, reefers, WMS, twin state
ironbridge/telemetry.yaml   plant: panel power, bearing vibration, cycles, alarms, OEE
galewind/telemetry.yaml     wind: turbines, curtailment, feeders, ERCOT dispatch
```

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
cd nats-config && ../stone-age serve --nats --nats-config ./nats.conf
```

Two things about that last pair of lines:

- **`nats export -o <dir>` currently writes nothing** — it ignores the flag and
  prints to stdout instead, in the pb-nats version vendored here. Until that is
  fixed, redirect the three artifacts by hand:
  `nats export --config > nats-config/nats.conf`, `--operator-conf >
  nats-config/operator.conf`, `--operator-jwt > nats-config/operator.jwt`.
- **Run `serve --nats` from the directory holding `nats.conf`.** The generated
  config references `operator.jwt`, the `jwt` resolver directory and
  `./storage/jetstream` by relative path, resolved against the process working
  directory rather than the config file. Start it from elsewhere and it dies with
  `error parsing operator JWT: open operator.jwt`.

An external `nats-server` works exactly the same way — same config, same
credentials. Only the process boundary differs.

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
| Console | `event.KC-DC1.DS-001.door` | — |
| Console | `app.wms.WMS-CONN-01.shipment` | — |

The three chart rows are the three signals that actually move. Everything else
publishes a single plausible value, for a reason worth knowing before you write
your own rules.

## Two constraints you will hit immediately

**Cron floors at one minute.** rule-router's scheduler is 5-field and validates
it, so one rule publishes at most once a minute. A real press cycles faster than
that; these files sample rather than reproduce.

**A rule's payload is fixed.** There is no random function and no arithmetic on
`{@time.*}`, so one rule publishing every minute draws a *flat line*. The
workaround, used once per file and commented where it happens, is to give the one
signal you actually want to watch a handful of phase-shifted rules — different
minute, different value — so the series repeats as a waveform. It is verbose, and
it is the honest cost of simulating a device with a scheduler. If rule-router
ever grows a value generator, those blocks each collapse to one rule.

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
