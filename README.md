# Stone Age Platform

The **Control Plane** for Stone-Age.io: one HTTP API for the things and places
you manage, which also mints the credentials that let them talk. Multi-tenant
inventory, NATS account and credential management, and Nebula overlay networks,
in one Go binary with the console compiled into it.

Used at its shallowest it is a REST inventory of Things and Locations — a device
needs no NATS user and no Nebula host to exist as a record. Used at its fullest
it provisions and keeps in sync the identities an entire event-driven fabric runs
on.

---

## Run it

### Container — nothing to install

```bash
docker run -d --name stone-age \
  -p 8090:8090 -p 4222:4222 -p 9222:9222 \
  -v stone-age-data:/data \
  -e STONE_AGE_BOOTSTRAP_PASSWORD='change-me-8-chars-min' \
  -e STONE_AGE_NATS_WEBSOCKET_URLS='ws://localhost:9222' \
  ghcr.io/stone-age-io/platform:latest
```

Console at <http://localhost:8090>, admin panel at <http://localhost:8090/_/>,
log in as `admin@example.com` with the password you set. One container: the same
process serves the API, the console, and the NATS server the console talks to.

`STONE_AGE_NATS_WEBSOCKET_URLS` is the address a **browser** dials, which the
container cannot work out for itself — put the host's real name there rather than
`localhost` if anyone else will use it.

### From source — five commands

Needs Go 1.26+ and Node 20.19+ (or 22.12+, whichever your distribution has).

```bash
cd ui && npm install && npm run build && cd ..
go build -o stone-age .
./stone-age superuser upsert admin@example.com 'change-me-8-chars-min'
./stone-age migrate up
./stone-age bootstrap --email admin@example.com --org "System" --operator-org "your-company"
./stone-age nats export --output ./nats-config/
./stone-age serve --nats
```

The first line builds the console into `pb_public/`; the second embeds it.

**The order of the middle three is load-bearing.** `bootstrap` writes
`is_operator`, `is_system_org` and `is_operator_org`, which only exist after
`schema.json` has been imported by `migrate up`. PocketBase silently discards a
write to a field that does not exist, so running `bootstrap` first would print
success while leaving you with no platform operator at all. It now refuses to run
in that state and says so. `superuser upsert` has to come first as well: it seeds
the NATS operator and `$SYS` records the other two link up.

Everything except the last line is once per deployment. After the first run,
`./stone-age serve --nats` is the whole thing.

Measured on a clean clone: about two minutes of machine time from `git clone` to
a console serving with the bus up, of which roughly ninety seconds is `npm
install` and the Vite build. The Go build is nine seconds and the four seeding
commands are eleven.

Granting operator status is deliberately impossible through the API. This command
or the admin panel, nothing else.

---

## What it does

**Multi-tenancy.** Organizations, memberships, invitations, and five roles.
Isolation is enforced *entirely* by the PocketBase API rules in `schema.json` —
the NATS and Nebula libraries contain no tenancy logic and never reference
`organization`. The console's capability map is navigation convenience, not a
boundary.

**NATS in operator mode.** Creating an organization provisions its account.
Roles are permission templates; a user JWT is signed from one. Credentials
rotate, revocation actually disconnects, and account signing keys are manageable.
Nothing about a device's *console* session gives it bus access — the signed
credential is the capability.

**Nebula overlay networks.** A certificate authority per organization, plus
networks, lighthouses and host certificates. This is the out-of-band path: SSH
and management traffic that has to work when NATS does not.

**Inventory as identity.** A Thing is one record that is simultaneously an
inventory entry, a login, a NATS identity and a mesh node. Deactivating it takes
all four away in one operation — the API rule blocks new logins, outstanding
tokens are invalidated immediately, and the NATS credential is revoked.

**One name for a tenant, everywhere.** `organizations.code` is the ecosystem's
single globally unique identifier; everything under it is unique only within its
organization. The rule is **ids for storage, codes for addressing** — relation
columns stay PocketBase ids, while anything that has to survive leaving this
database (a NATS subject, a URL, a sticker on a wall) travels by code. The
managed-org subject rewrite roots at it, so a sibling app reading
`helpdesk.{code}.>` and a person scanning a label are naming the same tenant.
Codes are optional but **immutable**: mutability, not optionality, was what
disqualified the alternative. See ADR 0002 in
[platform-docs](https://github.com/stone-age-io/platform-docs).

**QR labels.** Print an operator-branded label for any thing or location that
has a code, sized in millimetres to real stock (2″ × 1″ and 4″ × 2″, both
reserving the centred RFID inlay keep-out so one layout prints on plain or RFID
media). The payload is the **bare code** — no host, no organization, no kind
token — because a sticker in a public hallway is something a stranger can
replace, and a URL payload would let a forged label send a person to arbitrary
content. Scanning happens inside an app (the `scanner` widget here, `/staff/scan`
in the helpdesk); nothing ever fetches the decoded string as a destination.

**The contract layer.** Declarative device contracts describing *where* a
participant speaks and *what shape* its messages take, so a consumer can resolve
both from data alone:

- **Thing Types** (`thing_types`) define a subject prefix and a set of operations.
- **Operations** (`thing_type_operations`) declare a capability (`publish` /
  `subscribe` / `request` / `reply`), a subject suffix, and an optional schema.
- **Message Schemas** (`message_schemas`) are versioned JSON Schema documents.
  The console has a visual builder and an infer-from-sample tool.

**Edge sites.** Each is a `leaf_nodes` record — a special kind of thing, with one
server-provisioned NATS user. The separate [`leaf-sync`](./cmd/leaf-sync/README.md)
agent runs on the edge, authenticates as the leaf node, and mirrors its
organization's configuration into a NATS leaf node's local JetStream KV. It can
host that leaf node itself (`leaf-sync run --nats`), so an edge site is one
service rather than two.

**Digital twin.** Two KV buckets per organization with one writer each: `twin`
for reported state flowing edge-to-hub, `twin_desired` for desired state flowing
hub-to-edge. Drift is shown as the values themselves, not a status word.

**Dashboards.** Grid layout, 17 widget types, three data-source kinds
(subscription, consumer, KV), variable substitution, and a live NATS WebSocket
connection straight from the browser.

**Audit logging** across every create, update, delete and auth event, with a
searchable viewer.

### Scope note on "single binary"

*This component* is one binary — Go backend, embedded Vue console, SQLite, no
runtime dependencies, and with `--nats` the message bus too. The **platform** is
not one binary: it is a small set of independent single-binary components (this
Control Plane, `nebula`, `rule-router`, the Agent, `leaf-sync`) that find each
other over NATS. Deploy each where it belongs.

---

## Configuration

A `config.yaml` in the working directory or `/etc/stone-age/`, or environment
variables prefixed `STONE_AGE_` (`STONE_AGE_NATS_SERVER_URL`,
`STONE_AGE_TENANCY_LOG_TO_CONSOLE`). Environment variables win. Every key has a
default, so the file is optional.

Two keys are worth understanding before a production deployment.

**`nats.encryption_key` and `nebula.encryption_key`** encrypt the secret columns
at rest — NATS account and user seeds, Nebula CA and host private keys. They are
empty by default, which means plaintext in SQLite. Set them before creating
anything real: a row cannot be read back without the key it was written with, so
losing the key loses the material.

**`nats.server_url` and `nats.websocket_urls` are not the same address.** The
first is what *this process* dials to publish account claims. The second is what
a *browser* dials — a different port, often a different host — and it is served
to the console at runtime by `GET /api/client-config`, so changing it needs no
frontend rebuild:

```yaml
nats:
  server_url: "nats://localhost:4222"
  websocket_urls: ["ws://localhost:9222"]
```

Behind HTTPS use `wss://`; browsers refuse a plaintext socket from a secure page.
Multiple entries are peers of one cluster, not a fallback order. A device that
should talk to a local leaf node instead sets a per-device override in the
console's settings, which *replaces* this list rather than adding to it.

Full reference: the [platform docs](https://github.com/stone-age-io/platform-docs).

### Running the bus separately

`serve --nats` is off by default. It is an ordinary `nats-server` reading the
ordinary config file `nats export` writes, so moving to a separate process is a
config change rather than a migration:

```bash
./stone-age nats export --output ./nats-config/
nats-server -c ./nats-config/nats.conf
./stone-age serve
```

That sameness is not a slogan: the parsed config goes straight to
`nats-server`, so **a `cluster` block works like any other directive** and the
Control Plane can be one node of a cluster whose other nodes are plain
`nats-server` processes. `stone_age_nats_cluster_routes` reports the peers.

The trade-off to weigh before choosing that: while they share a process,
restarting the Control Plane restarts the bus — and in a cluster that means
taking a node down with it. See ADR 0001 in the platform docs.

### Readiness and metrics

`GET /api/ready` answers whether the deployment actually *works*, which
PocketBase's `/api/health` does not — that one reports the HTTP server is
listening, and it is true of every failure worth catching here:

```bash
curl -s localhost:8090/api/ready
```

| Check | Catches |
| --- | --- |
| `database` | SQLite unreadable — `pb_data/` not writable, or held by another running instance |
| `schema` | `schema.json` was never imported, so the platform's own fields do not exist — and PocketBase drops writes to missing fields silently, which is how `bootstrap` "succeeds" while writing nothing |
| `schema_version` | This `pb_data` was written by a **newer** build. Migrations do not roll back, so `serve` cannot repair it |
| `bootstrap` | No user has `is_operator`, or no organization has `is_system_org` — nobody can create an organization |
| `nats_operator` | No NATS operator record, or no JWT on it. Nothing can sign an account or user credential |
| `nats_reachable` | Nothing listening at `nats.server_url`. *Warns* if it answers but JetStream is off — KV buckets need it |
| `nats_trust` | The NATS server **rejects this platform's `$SYS` credential**: its `nats.conf` carries a different operator, so every account claim fails, no organization reaches the bus, and the console looks fine |
| `nats_websocket_urls` | *Warns* — unset, so the console falls back to `ws://localhost:9222` and no browser on another machine can reach the bus |
| `encryption_at_rest` | *Warns* — NATS seeds and Nebula private keys are stored in plaintext |

Only a failure returns 503. Warnings answer 200: a probe's only lever is to
restart or de-register, and neither fixes "you have not set an encryption key".
A check that could not look at all (no `$SYS` credential to test with) reports
`skipped` rather than passing.

Every check carries a `detail` and, when unhappy, a suggested `fix`; the same
text goes to the log. The endpoint is unauthenticated, because the callers are
orchestrators and uptime checkers that hold no session — put it behind your
proxy if the deployment needs it closed. The container image wires it to
`HEALTHCHECK`.

`GET /metrics` is Prometheus text, open by default:

```yaml
scrape_configs:
  - job_name: stone-age
    static_configs: [{ targets: ["platform.example.com:8090"] }]
```

Set `metrics.token` to close it; the same value works as `Authorization: Bearer`
or as HTTP Basic with any username, covering every scraper in common use.

| Metric | What it says |
| --- | --- |
| `stone_age_ready` | 1 when the last probe passed. Warnings do not clear it |
| `stone_age_check_state{name,state}` | Each check's current state, as a complete state set |
| `stone_age_check_timestamp_seconds` | Last probe time — alert on this going stale, it means the prober itself is wedged |
| `stone_age_build_info{version}` | Always 1; read the label |
| `stone_age_http_requests_total{route,method,status}` | Traffic by matched route *pattern*, status by class |
| `stone_age_http_request_duration_seconds` | Request latency histogram, same route label |
| `stone_age_records{collection}` | Rows **configured** — inventory, not availability |
| `stone_age_inactive_records{collection}` | Decommissioned things and leaf nodes (`active = false`) |
| `stone_age_nats_users_revoked` | Credentials on their account's revocation list |
| `stone_age_database_size_bytes` | SQLite plus its WAL, for disk growth |
| `stone_age_collector_errors` | Collectors that failed *this scrape* — the affected series are missing, not zero |
| `stone_age_nats_*` | The embedded bus: `_embedded_up`, connections, cluster routes, leaf-node connections, slow consumers, msgs/bytes, JetStream bytes. **Only with `serve --nats`** — absent entirely with an external server, rather than reported as zero |

The standard `go_*` and `process_*` collectors are included too.

**Nothing here is per-organization, and that is structural rather than a
simplification.** This process holds the NATS operator and the `$SYS` account
and has no user credential inside any organization's account, so it cannot read
what is in one — not the `twin` buckets, and not the `leaf_status` heartbeats
`leaf-sync` writes. `stone_age_records{collection="leaf_nodes"}` therefore
counts leaf nodes *configured*; an alert on it can never fire.

Per-site health is exported by `leaf-sync` on the edge box, which is inside the
account and keeps answering when the WAN is down — see
[cmd/leaf-sync/README.md](cmd/leaf-sync/README.md). The console shows the same
thing for humans, because a browser connects with the logged-in user's own
in-account credential.

With an external `nats-server`, scrape it with
[prometheus-nats-exporter](https://github.com/nats-io/prometheus-nats-exporter):
it reads the server's own monitoring port and reports far more than this could.

### Branding overlay

Operators can re-skin the console without rebuilding the binary. Point
`branding.dir` at a host directory containing any of:

| File            | Purpose                                                         |
| --------------- | --------------------------------------------------------------- |
| `branding.json` | `{ "appName": "...", "logo": "logo.svg" }`                       |
| `logo.svg`      | Brand mark on login, sidebar and mobile header                   |
| `theme.css`     | Override the daisyUI v4 custom properties for light and dark     |

```bash
cp -r branding.example /etc/stone-age/branding
# edit the three files, then in config.yaml:
#   branding:
#     dir: /etc/stone-age/branding
```

Missing files fall back to the embedded defaults. For a different logo per theme,
the example `theme.css` documents a CSS-only swap using the `.brand-logo-img`
class hook.

---

## Development

Backend and frontend separately, for hot module replacement:

```bash
go run main.go serve
```

```bash
cd ui && npm run dev
```

Frontend on `:5173`, proxying `/api` to the backend on `:8090`.

Two frontend dependencies are **deliberately held back**, and a routine "bump
everything" pass must not drag them along: Tailwind 3 / daisyUI 4, because
upgrading is a design-system migration rather than a version bump — 376
references use the v4 `oklch(var(--b1))` form, which v5 breaks silently — and
TypeScript 6, because TS 7 drops the `./lib/tsc` subpath `vue-tsc` resolves at
startup, so the build dies before it type-checks anything. The full reasoning is
at the top of `ui/tailwind.config.js`. CI asserts both majors.

### Authorization tests

Tenant isolation and privilege boundaries are enforced *entirely* by the API
rules in `schema.json`, which are plain strings in a JSON file with no compiler
behind them. One script exercises them against a real server:

```bash
./scripts/test-authz.sh
```

It builds the binary, stands up a throwaway database, runs the checks and tears
everything down; `pb_data/` is never touched. The check count lives in
`EXPECTED_CHECKS` at the top of the script, where it guards against a suite that
exits early — bump it when you add a check. Requires `go`, `curl` and `node`. CI
runs it on every pull request.

Two things to know when reading a failure. PocketBase answers **404, not 403**,
when an update rule rejects a request — deliberately, so it does not confirm the
record exists. And every "cannot do X" check is paired with a "can still do Y"
check on the same record, so a rule that denies everything fails the suite rather
than passing it.

### Schema management

`schema.json` is the single source of truth for every collection, embedded in the
binary. It is applied by the files in `migrations/`, each of which calls
`ImportCollectionsByMarshaledJSON(SchemaJSON, false)` and runs exactly once per
database. The import is additive (`deleteMissing=false`), so collections created
by the libraries survive.

To change it:

1. Make the change in the admin panel at `/_/`.
2. **Settings → Export collections**, and replace `schema.json`.
3. **Add a new file to `migrations/`** that re-imports the schema — copy an
   existing `schema_update_*.go`, rename it, and describe the change in its doc
   comment.
4. Commit both, and run `./scripts/test-authz.sh` if you touched a rule.

> Step 3 is not optional. Every existing deployment has already run the existing
> migrations, so editing `schema.json` alone reaches *fresh* databases only and is
> a silent no-op everywhere else. A new non-null column with a live rule over it
> also needs its backfill in the same migration.

### Hooks

`main.go` registers the platform's own hooks and routes. The ones worth knowing:

- **Organization created** → provisions that organization's NATS account and
  Nebula CA.
- **Leaf node created** → mints the edge node's NATS user, so `leaf-sync` can
  authenticate as the leaf node (`hooks/leaf_node_provisioning.go`).
- **Membership deleted** → clears the departing member's organization context,
  which is what the inventory read rules are scoped by
  (`hooks/membership_lifecycle.go`).
- **`active` flipped on a thing or leaf node** → invalidates outstanding tokens
  and revokes the linked NATS identity (`hooks/active_flag.go`).
- **`GET /api/leaf/bootstrap`** → everything `leaf-sync config` needs, in eight
  named fields, so a leaf-node identity needs no read grant on any `nats_*` or
  `nebula_*` collection (`hooks/leaf_node_routes.go`).
  `GET /api/leaf/operator-jwt` remains as a superseded alias.
- **`POST /api/org/things`** → a Thing plus an optional NATS or Nebula identity in
  one transaction: member-level for the inventory half, owner/admin for the
  identity half (`hooks/thing_routes.go`).
- **`GET /api/client-config`** → the deployment facts the console cannot be
  compiled with, chiefly the browser-facing WebSocket URLs
  (`hooks/client_config_routes.go`).
- **`OnServe`** → serves the embedded `pb_public` filesystem with SPA history
  fallback, plus the branding overlay.

---

## Related

| | |
|---|---|
| [`cmd/leaf-sync/`](./cmd/leaf-sync/README.md) | The edge agent, and the edge deployment flow |
| [platform-docs](https://github.com/stone-age-io/platform-docs) | Configuration reference, operations, architecture decisions |
| [`CHANGELOG.md`](./CHANGELOG.md) | Releases |
| [`SECURITY.md`](./SECURITY.md) | Reporting a vulnerability, and what counts as one |

MIT — see [`LICENSE`](./LICENSE).
