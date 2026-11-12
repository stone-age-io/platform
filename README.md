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
log in as `admin@example.com` with the password you set. One container serves the
API, the console and the NATS server the console talks to.

`STONE_AGE_NATS_WEBSOCKET_URLS` is the address a **browser** dials; put the
host's real name there if anyone else will use it. For edge sites, also publish
`-p 7422:7422` and set `STONE_AGE_NATS_LEAF_URL`. The image runs as uid 10001,
so a bind mount must be writable by it. Every variable:
[Getting Started](https://github.com/stone-age-io/platform-docs/blob/main/docs/getting-started.md).

### From source

Needs Go 1.27+ and Node 20.19+ (or 22.12+).

```bash
cd ui && npm install && npm run build && cd ..
go build -o stone-age .
./stone-age init --email admin@example.com --operator-org "your-company"
./stone-age serve --nats
```

`init` asks for a password (or reads `STONE_AGE_BOOTSTRAP_PASSWORD`), runs the
first-run commands in the order they need, and does nothing once its
`nats.conf` exists. After that, `./stone-age serve --nats` is the whole thing.

### Something to look at

A fresh deployment is empty. One command fills it with three fictional tenants:
locations on a real map, a type taxonomy, devices, gateways, applications and
unattended screens, NATS roles and signed identities, a Nebula network, edge
sites, and members holding all five roles.

```bash
./stone-age demo-seed --confirm
```

Every login is `demo1234`; start with `dana@northwind.example`, or
`casey@msp.example` for the organization switcher. It is idempotent, and
`--things 400` gives a bigger fleet. The tenants share their codes with the
demos in [helpdesk](https://github.com/stone-age-io/helpdesk),
[access-control](https://github.com/stone-age-io/access-control) and
[kiosk](https://github.com/skeeeon/kiosk), so a door in one app and a Thing here
are the same door. `--confirm` is required because the command writes real
signed credentials into whatever deployment it runs against.

To make the dashboards move, [`demo/rules/`](demo/rules) holds rule-router rules
that publish on the subjects each seeded type declares.

---

## What it does

- **Multi-tenancy.** Organizations, memberships, invitations and five roles,
  enforced entirely by the API rules in `schema.json`.
- **NATS in operator mode.** An account per organization, users signed from
  roles, rotation, and revocation that actually disconnects.
- **Nebula overlay networks.** A certificate authority per organization, with
  networks, lighthouses and host certificates: the path that works when NATS
  does not.
- **Inventory as identity.** A Thing is one record that is an inventory entry,
  a login, a NATS identity and a mesh node. Deactivating it suspends all of them
  at once.
- **One name for a tenant, everywhere.** `organizations.code` is the one
  globally unique identifier: ids for storage, codes for addressing.
- **QR labels** whose payload is the bare code, never a URL.
- **A contract layer.** Thing Types and their operations say where each kind of
  participant publishes and subscribes, so subjects resolve from data.
- **Edge sites.** A site is a Thing whose
  [Agent](https://github.com/stone-age-io/agent) gets everything a NATS leaf
  needs from one route.
- **Digital twin.** Reported and desired state in two KV buckets with one
  writer each.
- **Dashboards.** 18 widget types on a live NATS WebSocket from the browser.
- **Audit logging** for platform operators, and an activity feed for tenants.

*This component* is one binary — Go backend, embedded console, SQLite, and with
`--nats` the message bus too. The *platform* is a few independent components
(this Control Plane, `nebula`, `rule-router`, the Agent) that find each other
over NATS.

## Documentation

The full documentation is in
[platform-docs](https://github.com/stone-age-io/platform-docs):

| For | Read |
|---|---|
| Installing and first steps | [Getting Started](https://github.com/stone-age-io/platform-docs/blob/main/docs/getting-started.md) |
| `config.yaml`, environment variables, encryption keys, branding | [Configuration](https://github.com/stone-age-io/platform-docs/blob/main/docs/configuration.md) |
| Running NATS separately, backups, upgrades, the production checklist | [Operations](https://github.com/stone-age-io/platform-docs/blob/main/docs/operations.md) |
| `/api/ready`, `/metrics` and alerts | [Health & Metrics](https://github.com/stone-age-io/platform-docs/blob/main/docs/health-metrics.md) |
| Roles and what each can do | [Authorization](https://github.com/stone-age-io/platform-docs/blob/main/docs/authorization.md) |
| The custom routes | [API Reference](https://github.com/stone-age-io/platform-docs/blob/main/docs/api-reference.md) |
| Design decisions | [Architecture](https://github.com/stone-age-io/platform-docs/blob/main/docs/architecture.md), [ADRs](https://github.com/stone-age-io/platform-docs/blob/main/docs/decisions.md) |

---

## Development

```bash
go run . serve          # backend on :8090
cd ui && npm run dev    # console on :5173, proxying /api to the backend
```

- `go test -short ./...` while working, `cd ui && npm test` for the console.
- **`./scripts/test-authz.sh` after any API-rule change.** The rules are the
  only tenancy enforcement and nothing else checks them. It builds the binary,
  uses a throwaway database and needs `go`, `curl` and `node`.
- **Changing the schema:** make the change in the admin panel at `/_/`, export
  the collections into `schema.json`, and add a new `migrations/schema_update_*.go`
  that re-imports it. Without the migration the change reaches new databases
  only.
- Contributor notes, including the dependencies pinned on purpose: `CLAUDE.md`,
  `AUTHORIZATION.md`, `TESTING.md` and the `CLAUDE.md` in `hooks/`,
  `migrations/` and `ui/`.

## Related

| | |
|---|---|
| [stone-age-io/agent](https://github.com/stone-age-io/agent) | The edge agent, and the edge deployment flow |
| [platform-docs](https://github.com/stone-age-io/platform-docs) | The documentation |
| [`CHANGELOG.md`](./CHANGELOG.md) | Releases |
| [`SECURITY.md`](./SECURITY.md) | Reporting a vulnerability, and what counts as one |

MIT — see [`LICENSE`](./LICENSE).
