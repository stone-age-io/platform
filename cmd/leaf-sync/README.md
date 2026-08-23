# leaf-sync

`leaf-sync` is the Stone Age platform's **edge agent**. It runs on an edge box
alongside a stock NATS **leaf node** and keeps that node's local JetStream KV in
sync with its organization's config in the central PocketBase.

It is a small, separate binary built from this repo (`go build ./cmd/leaf-sync`)
— **not** the central-server binary. The edge never runs PocketBase.

## Where it fits

An edge is one org-scoped node made of three processes:

| Process | Role |
|---|---|
| stock `nats-server` (leaf node) | transport + local JetStream domain; bound to one org account |
| [`rule-router`](https://github.com/skeeeon/rule-router) | local automation + HTTP exposure over KV |
| **`leaf-sync`** | mirror central PocketBase config → local KV; bootstrap the leaf config; optionally wire up the twin buckets |

There are two distinct sync planes:

- **Config plane** — PocketBase records → `leaf-sync` (HTTPS pull) → local KV.
  Always on. It exists because central PocketBase is the NATS *operator* (SYSTEM
  account only) and cannot push into org account data planes.
- **Data plane** — the digital-twin buckets (`twin` up, `twin_desired` down),
  wired up by [twin sync](#twin-sync-data-plane). Off by default (`twin.enabled`).
- **Telemetry** — high-volume streams are still *not* `leaf-sync`'s job. Replicate
  those with cross-domain JetStream mirror/source, configured by the account's own
  users.

## Identity

Each edge is modeled as a **`leaf_nodes`** record in the platform — "a special
thing" with a single `nats_user`. When you create one (UI → *Leaf Nodes* → *New*),
a server-side hook provisions its NATS user automatically. The leaf node's own
PocketBase credentials (shown once) are what `leaf-sync` authenticates with.

## Configuration

Copy [`leaf-sync.example.yaml`](./leaf-sync.example.yaml) to `leaf-sync.yaml` and
set the PocketBase URL + the leaf node credentials. Everything else is pulled
from PocketBase. Pass a path with `--config`, or place `leaf-sync.yaml` in the
working directory or `/etc/leaf-sync/`.

| Key | Required | Notes |
|---|---|---|
| `pocketbase.url` | ✓ | Central platform base URL. |
| `pocketbase.email` / `pocketbase.password` | ✓ | The leaf node's own credentials (from the UI). |
| `nats.hub_leaf_url` | for `config` | Where the leaf remote dials the hub; written into `nats-leaf.conf`. |
| `nats.local_url` | | Local leaf the daemon connects to (default `nats://127.0.0.1:4222`). |
| `nats.creds_file` | | Creds filename (default `edge.creds`); written by `config`, read by `run`. |
| `nats.hub_domain` | | Hub's JetStream domain. When set, `run` writes a liveness heartbeat into the hub's `leaf_status` KV. Empty = heartbeat off, and the twin relay cannot run. |
| `nats.embedded` | | Run the leaf node inside this process — see [Running the leaf node in-process](#running-the-leaf-node-in-process). Same as `--nats` (default `false`). |
| `nats.embedded_config` | | The `nats-leaf.conf` `--nats` loads (default `<output.dir>/nats-leaf.conf`). |
| `nats.monitor_url` | | The leaf's own monitoring endpoint, the `http:` line `config` writes (default `http://127.0.0.1:8222`). Loopback and unauthenticated by design — how the edge reads its own server without a `$SYS` credential. Empty disables the checks and metrics that need it. |
| `output.dir` | | Where `config` writes files (default `.`). |
| `sync.interval` | | Full-reconcile cadence (default `30s`). |
| `observability.addr` | | Serve `/ready` + `/metrics` here (default empty = not served; the checks still run and still log). |
| `observability.metrics_token` | | Closes `/metrics`; accepted as Bearer or Basic-with-any-username (default empty = open). |
| `observability.interval` | | How often the readiness checks run (default `15s`). |
| `twin.enabled` | | Turn on [twin sync](#twin-sync-data-plane) (default `false`). Requires `nats.hub_domain`. |
| `reload_hook`, `jwt_refresh.enabled` | | Reserved — not yet active. |

## Commands

```sh
leaf-sync config     # one-shot: write nats-leaf.conf + edge.creds from PocketBase
leaf-sync run        # daemon: mirror config collections into local KV (+ twin sync)
leaf-sync run --nats # ...and run the leaf node itself, in this process
leaf-sync --version  # print the build version
```

- **`config`** authenticates to PocketBase as the leaf node and makes a single
  call to `GET /api/leaf/bootstrap`, which returns the values a leaf server
  needs: `domain`, `code`, `creds`, `account_jwt`, `account_pub`, `operator_jwt`,
  `sys_account_jwt`, `sys_account_pub`. It writes `nats-leaf.conf` (operator +
  `MEMORY` resolver_preload + JetStream domain + leaf remote + localhost
  monitoring) and the creds file. No NATS connection needed — run it before the
  leaf is up. A half-provisioned leaf node fails here naming the missing field,
  rather than producing a `nats-leaf.conf` with empty directives.

  The `$SYS` pair is not optional and is not a `$SYS` identity. The operator JWT
  names a system account, and a server running `resolver: MEMORY` has nowhere to
  fetch it from — without it preloaded, `nats-server` refuses to start at all
  ("error resolving system account: account missing"). Preloading an account JWT
  grants nothing on its own: connecting as `$SYS` needs a `$SYS` *user*
  credential, which is never served to a leaf node. Likewise the leaf remote
  carries an `account` key, which operator mode requires on every remote.
- **`run`** connects to the local leaf and, every `sync.interval`, performs a
  full reconcile of each allowed collection: upsert every record into KV bucket
  `<collection>`, then delete KV keys for records that no longer exist. Each
  record is keyed by the same handle `stone` uses — `message_schemas` by their
  `namespace__name__version`, everything else by `code`, then `name` — falling
  back to the PocketBase record id when that handle is absent, duplicated within
  the collection, or not a valid NATS KV key; the id always remains inside the
  stored JSON, so relation fields still resolve. Server-only noise fields
  (`collectionId`, `collectionName`, `expand`) are stripped from the value.
  Writes are changed-only: a record is re-`Put` into KV only when its content
  differs from what was last written **and** the bucket still actually holds the
  key. That second condition is what makes the mirror self-healing — a key
  removed out-of-band (`nats kv del`, a purged and recreated bucket, a lost file
  store) is rewritten on the next cycle rather than staying absent for the life
  of the process.

  Deletion is driven only by what PocketBase returned, never by whether a write
  succeeded, so a failed `Put` cannot escalate into the key being purged. Three
  layers of purge protection, in order of how often they matter:

  | Situation | Behaviour |
  |---|---|
  | A record's `Put` fails | Key kept; retried next cycle; collection reported as errored in the heartbeat |
  | Fetch succeeds but returns zero records while KV holds keys | Purge skipped entirely for that cycle |
  | Any PocketBase or NATS error (including a failed key listing) | Local KV left exactly as-is and retried |

  It never wipes local state, and stops cleanly on `SIGINT`/`SIGTERM` (cancelling
  any in-flight PocketBase/NATS call), so it's safe to run under systemd/Docker.

## Running the leaf node in-process

`leaf-sync run --nats` (or `nats.embedded: true`) starts the leaf's `nats-server`
inside the agent, from the same `nats-leaf.conf` that `config` writes. The edge
becomes two processes instead of three, and the "regenerated the config, forgot
to restart the server" failure stops being possible.

It is an ordinary `nats-server` reading an ordinary config file — the same
arrangement as the Control Plane's `serve --nats` — so turning it back off is a
flag, not a migration.

Two things to know:

- **`nats.local_url` must name the port the config listens on.** Startup refuses
  the pair when they disagree, because nothing in the process would ever reach
  the server and the symptom otherwise is a silent retry loop.
- **A PocketBase outage no longer exits the agent.** Without `--nats`, a failed
  login exits and the supervisor restarts — the bus is someone else's process, so
  that costs nothing. With `--nats`, exiting would take the leaf down too, and a
  supervisor cycling the pair through a WAN outage means devices reconnecting and
  JetStream recovering its store on a loop. So it retries with a 2s→60s backoff
  instead, serving the config it already has. Watch for `PocketBase auth failed
  (...); local NATS is up, retrying in ...` in the log.

**Off by default, and staying that way.** Where systemd or Docker is already
supervising services, a separate `nats-server` is the better shape: the bus
survives a leaf-sync restart, which is what you want when upgrading the agent on
a live site.

The convenience costs binary size — `leaf-sync` goes from ~12 MB to ~26 MB,
because `nats-server` is linked in whether or not `--nats` is used. Against a
separately installed `nats-server` binary, total edge footprint is roughly flat.

After each cycle, if `nats.hub_domain` is set, `run` writes a small liveness
**heartbeat** into the hub's `leaf_status` KV bucket (keyed by the leaf node's
`code`): agent version, timestamp, sync interval, per-collection record counts,
and any sync errors. The platform UI reads this to show each leaf node's
online/offline status. The write is best-effort — a heartbeat failure (e.g.
during a WAN outage) is logged and never disturbs the sync loop. The heartbeat
targets the *hub's* JetStream domain because `leaf-sync` is connected to the
local leaf (whose own domain is `edge-<code>`); the absence of a recent beat is
what the UI treats as "offline."

Any config key can be overridden by an environment variable: upper-case the key,
replace dots with underscores, and prefix with `LEAF_SYNC_` — e.g.
`LEAF_SYNC_POCKETBASE_PASSWORD`, `LEAF_SYNC_SYNC_INTERVAL=15s`.

## Readiness and metrics

A site's real health can only be measured on the site. Whether the config mirror
is fresh, whether the uplink to the hub is attached, how full JetStream is — all
of it lives inside the organization's NATS account and on this box. The Control
Plane holds the NATS operator and the `$SYS` account and has **no credential
inside any tenant's account**, so the most it can report is how many leaf nodes
are *configured*. That is why the edge exports its own.

The checks always run and always log a readiness transition. Setting an address
is what makes them reachable:

```yaml
observability:
  addr: "127.0.0.1:9100"    # 0.0.0.0:9100 to scrape from elsewhere
  metrics_token: ""         # empty = open; Bearer or Basic when set
  interval: 15s
```

```bash
curl -s localhost:9100/ready
curl -s localhost:9100/metrics
```

| Check | Fails when |
|---|---|
| `nats_local` | The agent is not connected to the local leaf |
| `sync_freshness` | No cycle completed in three intervals — the loop is wedged |
| `sync_errors` | *(warns)* A collection errored; its existing mirror is untouched and retried |
| `hub_uplink` | *(warns)* No outbound leaf connection — this site is **islanded** |

An islanded site warns rather than fails, and still answers 200. Local NATS keeps
working and devices keep running against the mirrored config; that autonomy is
the entire reason a leaf node exists, so reporting it as unready would invert the
design.

| Metric | What it says |
|---|---|
| `leaf_sync_last_cycle_timestamp_seconds` | Alert on this going stale — the mirror is frozen |
| `leaf_sync_last_cycle_duration_seconds` | Wall time of the last reconcile |
| `leaf_sync_cycles_total` | Cycles completed since this agent started |
| `leaf_sync_mirrored_records{collection}` | Records in local KV, per collection |
| `leaf_sync_last_cycle_errors` | Collections that failed last cycle — a stale mirror, not a lost one |
| `leaf_sync_nats_connected` | 1 when this agent is connected to the local leaf |
| `leaf_sync_hub_uplink_connected` | 0 = islanded |
| `leaf_sync_nats_connections` | Devices actually attached at this site |
| `leaf_sync_jetstream_bytes{tier}` | The number to watch on a small edge disk |

The server-derived rows come from the leaf's own monitoring port
(`nats.monitor_url`, the `http:` line `leaf-sync config` writes into
`nats-leaf.conf`). It is loopback and unauthenticated by design, which is how
the edge reads its own server without ever holding a `$SYS` user credential —
and it works the same whether the leaf runs embedded or as a separate process.
When it is unreachable those series are **omitted rather than reported as zero**:
zero would claim an islanded site with no devices, a much louder statement than
"not scraped".

### This is not the heartbeat

`leaf-sync` also writes a summary into the hub's `leaf_status` KV once per cycle,
and the console renders it as online/offline. That is a tenant-facing product
feature, not a monitoring channel. It is best-effort, one key per site with a
fixed shape, and — being delivered over the very link that breaks — it goes
quiet during exactly the outage you want detail about. These endpoints are
scraped locally and keep answering with the WAN down.

## Twin sync (data plane)

With `twin.enabled: true` and `nats.hub_domain` set, `run` also wires up the
digital twin. It is **two buckets with one direction each**, so no key ever has
two writers:

| Bucket | Written by | Flows | Mechanism |
|---|---|---|---|
| `twin` | the device, at the edge | edge → hub | relay (below) |
| `twin_desired` | operators, at the hub | hub → edge | JetStream **mirror** |

The point is edge autonomy: a site whose uplink drops keeps writing reported
state locally and catches the hub up when the link returns, while still reading
the last-known desired state from its local mirror.

### Why this shape

**One bucket written from both ends does not work.** It does not merely pick a
loser on a conflict — it *oscillates*: two concurrent values for one key swap
across the link, then swap back, each write generating the next event. Measured
at ~170,000 writes to a single key in 300 ms before the buckets were split.

**Encoding the owner in the key** (`thing.S01.state.temp`) buys the same safety,
but taxes every key in the system — firmware, rule-router configs, widgets, docs
— and leaves a mistyped segment silently unsynced with no error anywhere. Two
buckets makes the conflict unrepresentable and costs one noun.

**Desired state is a mirror, not a relay,** because it has exactly one origin.
Mirrors do forward writes to the origin transparently (nats.go routes `Put` via
`External.APIPrefix`) and that write would fail during a WAN outage — but the
edge never writes desired state, so it never comes up. Reads are served locally
from the last-known values, which is precisely what you want when the link is
down. The mirror is configured on the *receiving* side, so there is no hub-side
stream to mutate and no race between sites.

**Reported state can't be a source,** which would otherwise be the symmetric
answer. Aggregating N sites at the hub means N sources all named `KV_twin`;
same-named sources need the server's internal `iname`, which nats.go doesn't
expose, and the documented guidance is unique stream names for centrally
aggregated streams. That would mean `twin_<code>` at every edge, so rule-router
would read a different bucket name at each site. Not worth it — hence the relay
for this one direction.

### Relay mechanics

Boring on purpose:

- **One watcher**, edge → hub. There is no reverse pump, so there is no echo.
- **Compare before write.** A value already equal at the hub is skipped. An
  optimisation, not the safety property — `WatchAll` replays every current value
  on start, so without it each restart would rewrite the bucket and burn a
  revision per key.
- **That replay is also the resync.** Reconnecting after an outage walks every
  current value, so there's no separate catch-up path to get wrong.
- **Deletes are relayed explicitly.** A KV delete is a tombstone message, not an
  absence; dropping it would leave the key live at the hub forever, since the
  equality check only compares values that exist. (Purge is relayed as a delete —
  the key goes away either way, but history rollup is domain-bound.)
- **Upsert, never reconcile.** The relay only acts on what this site's bucket
  reports, so one site can never purge another site's keys from the hub.
- **The watcher is supervised** with 1s→30s backoff, since nats.go reconnects the
  connection but a dead watcher stays dead.
- **Failure is soft.** If any of it can't start — no hub domain, bucket
  unreachable, mirror rejected — it logs why and carries on with what it can;
  config sync runs regardless.

Buckets are created if absent and otherwise left alone. Unlike the per-collection
config mirrors, these are shared with the console and operators, so the agent does
not reassert retention over whatever they set. A leaf whose JetStream domain *is*
the hub's needs none of this and skips it.

> **Operational note:** enabling this makes `leaf-sync` load-bearing for reported
> state. Down, it no longer just means stale config — it means a frozen twin in
> the console while the site itself runs fine. The `leaf_status` heartbeat is what
> distinguishes the two, which is why the console shows it beside the data.

## Building

```sh
# Plain build (version reports "dev"):
go build -o leaf-sync ./cmd/leaf-sync

# Release build (stamp the version, surfaced by --version and in each heartbeat):
go build -ldflags "-X platform/internal/version.Version=$(git describe --tags --always --dirty)" \
  -o leaf-sync ./cmd/leaf-sync
```

## Deploy flow

1. In the platform UI, create a **Leaf Node**; copy the credentials from the
   success modal.
2. On the edge box: install `leaf-sync` + a stock `nats-server`, write
   `leaf-sync.yaml`.
3. `leaf-sync config` → produces `nats-leaf.conf` + `edge.creds`.
4. `nats-server -c nats-leaf.conf` (under your init system of choice).
5. `leaf-sync run` (under your init system of choice).
6. Point `rule-router` at `edge.creds` for local automation.

Steps 2, 4 and 5 collapse if you use the in-process server: install `leaf-sync`
alone, and run `leaf-sync run --nats` as the single service. See
[Running the leaf node in-process](#running-the-leaf-node-in-process) for what
you give up.

`leaf-sync` does not supervise the other processes — use systemd, Docker, or
whatever your platform provides.

## What gets synced

A hard allowlist, enforced both in the server's API rules and in `leaf-sync`:

```
things   locations   thing_types   location_types
thing_type_operations   message_schemas
```

`thing_type_operations` and `message_schemas` complete the
thing_type → operation → message_schema graph, so a consumer can resolve what a
thing's type can do — and, if it chooses, check the messages it exchanges
against their schemas — entirely offline. leaf-sync itself never validates; see
[Contract model](#contract-model) below.

These are the only collections a leaf node can read at all, and only within its
own organization. Secret-bearing collections (`nats_users`, `nats_accounts`,
`nebula_*`) are not exposed to a leaf-node identity — it holds no read grant on
any of them, so nothing there can be synced or browsed. The four values an edge
genuinely needs from them arrive through `GET /api/leaf/bootstrap` instead; see
[Security model](#security-model). A leaf node's `synced_collections` field (set
in the UI) selects which of the allowlist to mirror.

## Contract model

**leaf-sync publishes the contract; it does not enforce it.** The
`thing_type → operation → message_schema` graph is mirrored to local KV — and is
equally available from the central PocketBase API — as machine-readable
reference data. Nothing in the platform validates live traffic against it:
not leaf-sync, not the leaf node, and not `rule-router` (which routes by
*subject*, not payload).

This is deliberate. Enforcement that is built in but optional is worse than
none — consumers come to trust a gate that only half-closes. Instead the
platform is the *authority on the contract* and ships it to where consumers
are; each consumer decides whether to act on it. An application that wants to
reject malformed payloads pulls the relevant `message_schema` — from local KV at
the edge, or the PocketBase API centrally — and checks against it on its own
terms. Most consumers won't, and that is fine: the schema still earns its keep
as documentation.

The accepted tradeoff is drift: a published schema and the actual traffic can
diverge, and the platform will not notice. The schema is documentation that
*may* lie. leaf-sync's job is to keep that documentation present, well-formed,
and current wherever a consumer might want it — not to be the thing that acts on
it.

## Security model

- One NATS identity per edge, shared by the leaf remote, rule-router, and
  leaf-sync. The **edge box is the trust boundary**; tenant isolation is the NATS
  account boundary.
- The edge only ever holds public trust material (operator JWT, account JWT) plus
  its own user's creds. It cannot mint new account users.
- **A leaf-node identity has no read grant on any `nats_*` or `nebula_*`
  collection.** Everything it needs from them comes from one dedicated,
  leaf-node-authenticated route, `GET /api/leaf/bootstrap`, which reads those
  records with the server's own privileges and returns a fixed list of named
  fields. The `nats_system_operator` collection stays superuser-only.

  This is why the route exists rather than a read rule. It states the edge's
  blast radius as a fixed list — a leaked edge credential yields those values and
  nothing else — instead of "whatever the rules on those collections happen
  to match", which has to be re-derived every time an unrelated rule changes. It
  also decouples the agent from rule shape: `config` used to read `nats_users`
  and `nats_accounts` through the CRUD API, which made a correct tightening
  elsewhere capable of breaking every edge box's bootstrap.

  Account seeds and signing keys are never reachable: the handler returns named
  fields, not whole records.
- `GET /api/leaf/operator-jwt` still exists, superseded by `/api/leaf/bootstrap`.
  It is kept so that upgrading the server before the edge boxes cannot break an
  agent already in the field.

## Roadmap

- **v0 (current):** full-collection reconcile on an interval with changed-only KV
  writes (a static collection produces no writes), self-healing against
  out-of-band KV loss, purge protection independent of write success, and a
  best-effort liveness heartbeat.
- **v1:** incremental *fetch* (`updated > cursor` + PocketBase `/api/realtime` SSE)
  so a full page of records no longer crosses the wire each cycle — with a periodic
  full reconcile kept as the correctness backbone, since deletions and duplicate-
  `code` keying still need to see the whole set. Optional account-JWT refresh with
  `reload_hook`.

## Tests

One test is not a unit test and earns its place: `TestBuildLeafConfIsAcceptedBy
NATSServer` mints a real operator/`$SYS`/account set, runs the real generator,
and hands the output to `nats-server`'s own config loader and `NewServer`. No
ports, no network — operator-mode validation all happens while the server is
being constructed. It exists because the generator shipped for months producing
a config `nats-server` rejected outright (no `account` on the leaf remote, no
`$SYS` preload), and every string assertion around it passed the whole time.
Breaking either directive fails it with the exact message the edge would have
seen.

Unit tests cover the pure logic (config loading + defaults, the syncable-
collection allowlist, the KV deletion diff, the `nats-leaf.conf` generator), the
PocketBase REST client (auth, transparent re-auth on 401, pagination), and the
reconcile loop itself — `syncCollection` is driven through narrow `recordLister`
and `kvBucket` interfaces so a fake bucket can simulate a failed `Put`, a key
vanishing out-of-band, an empty fetch, and an unreadable bucket. No live NATS or
PocketBase needed:

```sh
go test ./internal/leafsync/...
```

The two purge-protection tests are regression tests for real data-loss bugs;
both fail if the guard they cover is removed. If you change the reconcile logic,
check they still fail when you break it deliberately — a purge guard that no
longer guards anything still passes a test that only asserts the happy path.
