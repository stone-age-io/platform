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
| **`leaf-sync`** | mirror central PocketBase config → local KV; bootstrap the leaf config |

There are two distinct sync planes:

- **Config plane** — PocketBase records → `leaf-sync` (HTTPS pull) → local KV.
  This is what `leaf-sync` does. It exists because central PocketBase is the NATS
  *operator* (SYSTEM account only) and cannot push into org account data planes.
- **Data plane** — data already in NATS (digital twin, telemetry) replicates via
  cross-domain JetStream **mirror/source**, configured by the account's own users.
  This is *not* `leaf-sync`'s job.

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
| `nats.hub_domain` | | Hub's JetStream domain. When set, `run` writes a liveness heartbeat into the hub's `leaf_status` KV. Empty = heartbeat off. |
| `output.dir` | | Where `config` writes files (default `.`). |
| `sync.interval` | | Full-reconcile cadence (default `30s`). |
| `reload_hook`, `jwt_refresh.enabled` | | Reserved — not yet active. |

## Commands

```sh
leaf-sync config    # one-shot: write nats-leaf.conf + edge.creds from PocketBase
leaf-sync run       # daemon: mirror config collections into local KV
leaf-sync --version # print the build version
```

- **`config`** authenticates to PocketBase as the leaf node and fetches its own
  record (domain, synced collections, nats_user), its NATS user's `creds_file`,
  the org account JWT, and the operator JWT (via `GET /api/leaf/operator-jwt`).
  It writes `nats-leaf.conf` (operator + `MEMORY` resolver_preload + JetStream
  domain + leaf remote + localhost monitoring) and the creds file. No NATS
  connection needed — run it before the leaf is up.
- **`run`** connects to the local leaf and, every `sync.interval`, performs a
  full reconcile of each allowed collection: upsert every record into KV bucket
  `<collection>`, then delete KV keys for records that no longer exist. Each
  record is keyed by the same handle `stone` uses — `message_schemas` by their
  `namespace__name__version`, everything else by `code`, then `name` — falling
  back to the PocketBase record id when that handle is absent, duplicated within
  the collection, or not a valid NATS KV key; the id always remains inside the
  stored JSON, so relation fields still resolve. Server-only noise fields
  (`collectionId`, `collectionName`, `expand`) are stripped from the value.
  Fail-soft: on any PocketBase/NATS error it keeps local KV as-is and retries —
  it never wipes local state. A *successful but empty* fetch is also guarded: if
  a collection returns zero records while local KV still holds keys, the purge is
  skipped for that cycle (a transient auth/scoping glitch can't wipe the mirror).
  It stops cleanly on `SIGINT`/`SIGTERM` (cancelling any in-flight PocketBase/NATS
  call), so it's safe to run under systemd/Docker.

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

## Building

```sh
# Plain build (version reports "dev"):
go build -o leaf-sync ./cmd/leaf-sync

# Release build (stamp the version, surfaced by --version and in each heartbeat):
go build -ldflags "-X platform/internal/leafsync.Version=$(git describe --tags --always --dirty)" \
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

`leaf-sync` does not supervise the other processes — use systemd, Docker, or
whatever your platform provides.

## What gets synced

A hard allowlist, enforced both in the server's API rules and in `leaf-sync`:

```
things   locations   thing_types   location_types
thing_type_operations   message_schemas
```

`thing_type_operations` and `message_schemas` complete the
thing_type → operation → message_schema graph, so an edge node can resolve what
a thing's type can do — and validate the messages it exchanges — entirely
offline.

A leaf node can only read these (and only within its own organization). Secret-
bearing collections (`nats_users`, `nats_accounts`, `nebula_*`) are never
exposed to a leaf node identity and can never be synced. A leaf node's
`synced_collections` field (set in the UI) selects which of the allowlist to
mirror.

## Security model

- One NATS identity per edge, shared by the leaf remote, rule-router, and
  leaf-sync. The **edge box is the trust boundary**; tenant isolation is the NATS
  account boundary.
- The edge only ever holds public trust material (operator JWT, account JWT) plus
  its own user's creds. It cannot mint new account users.
- The operator JWT is exposed only through the dedicated, leaf-node-authenticated
  route `GET /api/leaf/operator-jwt`; the `nats_system_operator` collection stays
  superuser-only.

## Roadmap

- **v0 (current):** full-collection reconcile on an interval, with an
  empty-fetch purge guard and a best-effort liveness heartbeat.
- **v1:** incremental sync (`updated > cursor` + PocketBase `/api/realtime` SSE)
  with periodic full reconcile as the correctness backbone; optional account-JWT
  refresh with `reload_hook`.

## Tests

Unit tests cover the pure logic (config loading + defaults, the syncable-
collection allowlist, the KV deletion diff, the `nats-leaf.conf` generator) and
the PocketBase REST client (auth, transparent re-auth on 401, pagination) via an
`httptest` server — no live NATS or PocketBase needed:

```sh
go test ./internal/leafsync/...
```
